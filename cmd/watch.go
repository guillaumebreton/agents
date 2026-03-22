package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"notb.re/agents/internal/agent"
	"notb.re/agents/internal/config"
)

// Layout constants — adjust these to change margins consistently everywhere.
const (
	marginX = 2 // horizontal padding (columns) on each side
	marginY = 1 // vertical padding (lines) on top and bottom
)

// Notification display durations.
const (
	notifDurationInfo  = 4 * time.Second // how long info messages stay visible
	notifDurationError = 8 * time.Second // how long error messages stay visible
)

// Charm-inspired color palette.
var (
	colorPurple   = lipgloss.Color("63")
	colorCream    = lipgloss.Color("229")
	colorGreen    = lipgloss.Color("42")
	colorRed      = lipgloss.Color("196")
	colorDim      = lipgloss.Color("240")
	colorSubtle   = lipgloss.Color("236")
	colorWhite    = lipgloss.Color("252")
	colorBg       = lipgloss.Color("235")
	colorSelectBg = lipgloss.Color("57")
)

var watchCmd = &cobra.Command{
	Use:    "watch",
	Short:  "Watch and display all tracked agents",
	Hidden: true, // invoked internally via tmux SendCommand; not for direct use
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(newWatchModel(), tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

// tickMsg triggers a periodic refresh of agent data.
type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// agentStartedMsg is sent after startAgent completes (success or failure).
type agentStartedMsg struct{ err error }

// agentRemovedMsg is sent after agent removal completes.
type agentRemovedMsg struct{ err error }

// agentProgressMsg carries a status string from agentctl during agent creation.
type agentProgressMsg struct{ status string }

// progressCh is the channel agentctl writes progress strings into.
// It is buffered so the goroutine never blocks.
var progressCh = make(chan string, 8)

// waitProgressCmd returns a tea.Cmd that waits for the next progress message.
func waitProgressCmd() tea.Cmd {
	return func() tea.Msg {
		return agentProgressMsg{status: <-progressCh}
	}
}

// notifClearMsg is sent when a notification's auto-dismiss timer fires.
type notifClearMsg struct{}

type notifKind int

const (
	notifInfo  notifKind = iota
	notifError notifKind = iota
)

type notification struct {
	msg  string
	kind notifKind
}

func notifClearCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return notifClearMsg{}
	})
}

type watchModel struct {
	table       table.Model
	agentNames  []string // canonical agent names indexed by row, used for all lookups
	width       int
	height      int
	err         error
	confirming  bool   // true when showing delete confirmation
	confirmName string // name of the agent to delete
	adding      bool   // true when the add popup is open
	addForm     *huh.Form
	addMode     *string // "repo" or "path"
	addRepo     *string // heap-allocated so huh can write back through the pointer
	addBranch   *string // optional branch; non-empty → worktree mode, empty → main mode
	addDir      *string // heap-allocated so huh can write back through the pointer
	notif       *notification
}

func newWatchModel() watchModel {
	t := table.New(
		table.WithFocused(true),
	)
	return watchModel{table: t}
}

func tableStyles(width int) table.Styles {
	s := table.DefaultStyles()
	s.Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPurple).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorSubtle).
		BorderBottom(true).
		Padding(0, 1)
	s.Cell = lipgloss.NewStyle().
		Padding(0, 1)
	s.Selected = lipgloss.NewStyle().
		Foreground(colorCream).
		Background(colorSelectBg).
		Width(width)
	return s
}

func buildColumns(width int) []table.Column {
	indexW := 4
	agentTypeW := 12
	statusW := 12
	branchW := 18
	fixed := indexW + agentTypeW + statusW + branchW
	remaining := width - fixed - 12 // padding allowance
	remaining = max(remaining, 20)

	return []table.Column{
		{Title: "#", Width: indexW},
		{Title: "Agent", Width: agentTypeW},
		{Title: "Repo", Width: remaining},
		{Title: "Branch", Width: branchW},
		{Title: "Status", Width: statusW},
	}
}

func (m *watchModel) refreshRows() {
	agents, err := dataStore.List()
	if err != nil {
		m.err = err
		return
	}

	// Sort by window index numerically so rows appear in tmux status-bar order.
	// Fall back to name comparison for agents without an index yet.
	sort.Slice(agents, func(i, j int) bool {
		xi, erri := strconv.Atoi(agents[i].WindowIndex)
		xj, errj := strconv.Atoi(agents[j].WindowIndex)
		if erri == nil && errj == nil {
			if xi != xj {
				return xi < xj
			}
		} else if erri == nil {
			return true
		} else if errj == nil {
			return false
		}
		return agents[i].Name < agents[j].Name
	})

	cursor := m.table.Cursor()

	rows := make([]table.Row, 0, len(agents))
	names := make([]string, 0, len(agents))
	prevWindowID := "\x00" // sentinel so the first window always prints
	for _, a := range agents {
		status := "○ stopped"
		if a.WindowID != "" {
			alive, err := mux.WindowExists(a.WindowID)
			if err == nil && alive {
				switch a.Status {
				case "idle":
					status = "● idle"
				case "working":
					status = "◆ working"
				case "waiting":
					status = "◇ waiting"
				case "exited":
					status = "✕ exited"
				default:
					status = "● running"
				}
				// Fetch window index once (stable for the window's lifetime).
				if a.WindowIndex == "" {
					if idx := liveWindowIndex(a.WindowID); idx != "" {
						a.WindowIndex = idx
						dataStore.Save(a)
					}
				}
				// Refresh branch on every tick — user may have switched branches.
				if br := liveBranch(a.WorkdirPath); br != a.Branch {
					a.Branch = br
					dataStore.Save(a)
				}
			} else {
				// Window is dead — clean up stale window state.
				a.WindowID = ""
				a.WindowIndex = ""
				a.PanePID = ""
				a.Status = "exited"
				dataStore.Save(a)
				status = "✕ exited"
			}
		}
		// Show the window index only for the first agent in each window group.
		indexLabel := ""
		if a.WindowID != prevWindowID {
			indexLabel = a.WindowIndex
			prevWindowID = a.WindowID
		}

		names = append(names, a.Name)
		rows = append(rows, table.Row{indexLabel, a.AgentType, repoLabel(a), a.Branch, status})
	}

	m.agentNames = names
	m.table.SetRows(rows)
	m.table.SetWidth(m.width - marginX*2)
	m.table.SetStyles(tableStyles(m.width - marginX*2))

	// Preserve cursor position across refreshes.
	if cursor < len(rows) {
		m.table.SetCursor(cursor)
	}
}

func (m *watchModel) recalcLayout() {
	innerW := m.width - marginX*2
	cols := buildColumns(innerW)
	m.table.SetColumns(cols)
	m.table.SetStyles(tableStyles(innerW))
	// title(1) + title margin(1) + header border(1) + notif(1) + help(1) + marginY top only
	overhead := 5 + marginY
	tableHeight := max(m.height-overhead, 3)
	m.table.SetHeight(tableHeight)
}

// notify sets a notification and schedules an auto-clear after the appropriate duration.
func (m *watchModel) notify(msg string, kind notifKind) tea.Cmd {
	m.notif = &notification{msg: msg, kind: kind}
	d := notifDurationInfo
	if kind == notifError {
		d = notifDurationError
	}
	return notifClearCmd(d)
}

// notifyProgress sets a notification without scheduling an auto-clear.
// The message stays until replaced by the next progress step or a notify() call.
func (m *watchModel) notifyProgress(msg string) {
	m.notif = &notification{msg: msg, kind: notifInfo}
}

// newAddForm builds a fresh huh form for adding an agent.
// mode must point to either "repo" (workspace repository) or "path" (custom directory).
// For "repo" mode, branch controls whether a git worktree is created:
//   - empty  → open the agent in the main checkout (KindMain)
//   - filled → create / reuse a worktree on that branch (KindWorktree)
func newAddForm(mode *string, repo *string, branch *string, dir *string) *huh.Form {
	workspace, _ := config.Workspace()

	dirs := listWorkspaceDirs()
	opts := make([]huh.Option[string], len(dirs))
	for i, d := range dirs {
		label := "   " + d // plain directory — indented, no icon
		if isMainGitRepo(filepath.Join(workspace, d)) {
			label = "⎇  " + d // git repository
		}
		opts[i] = huh.NewOption(label, d)
	}
	if len(opts) == 0 {
		opts = []huh.Option[string]{huh.NewOption("(no directories found)", "")}
	}

	// selectedIsGit reports whether the currently selected workspace entry is
	// a main git repository. Used to show/hide the branch page.
	selectedIsGit := func() bool {
		if *repo == "" {
			return false
		}
		return isMainGitRepo(filepath.Join(workspace, *repo))
	}

	return huh.NewForm(
		// Page 1: choose source.
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Workdir source").
				Description("Where should the agent work?").
				Options(
					huh.NewOption("Workspace directory", "repo"),
					huh.NewOption("Custom path", "path"),
				).
				Value(mode),
		),
		// Page 2a: workspace directory selector (hidden when mode=path).
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Directory").
				Description("⎇  = git repository   •   plain = regular directory").
				Options(opts...).
				Value(repo),
		).WithHideFunc(func() bool { return *mode != "repo" }),
		// Page 2b: custom path input (hidden when mode=repo).
		huh.NewGroup(
			huh.NewInput().
				Title("Directory path").
				Description("Absolute (or ~-prefixed) path to an existing directory").
				Value(dir).
				Validate(func(s string) error {
					if *mode != "path" {
						return nil
					}
					expanded := expandTilde(strings.TrimSpace(s))
					if expanded == "" {
						return fmt.Errorf("path cannot be empty")
					}
					info, err := os.Stat(expanded)
					if err != nil {
						return fmt.Errorf("directory not found: %s", expanded)
					}
					if !info.IsDir() {
						return fmt.Errorf("not a directory: %s", expanded)
					}
					return nil
				}),
		).WithHideFunc(func() bool { return *mode != "path" }),
		// Page 3: branch input — only shown for git repositories.
		huh.NewGroup(
			huh.NewInput().
				Title("Branch (optional)").
				Description("Empty = main checkout  •  filled = worktree").
				Value(branch),
		).WithHideFunc(func() bool { return *mode != "repo" || !selectedIsGit() }),
	).WithTheme(huh.ThemeDracula())
}

func (m watchModel) Init() tea.Cmd {
	return tickCmd()
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle the add form popup.
	if m.adding {
		// Don't forward tick or window-size messages to huh — only key/mouse events.
		switch msg := msg.(type) {
		case tickMsg:
			return m, tickCmd()
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.recalcLayout()
			return m, nil
		case tea.KeyMsg:
			if msg.String() == "esc" {
				m.adding = false
				m.addForm = nil
				m.table.Focus()
				return m, tickCmd()
			}
		}

		form, cmd := m.addForm.Update(msg)
		m.addForm = form.(*huh.Form)

		if m.addForm.State == huh.StateCompleted {
			m.adding = false
			mode := *m.addMode
			m.addForm = nil
			m.table.Focus()

			agentType := config.DefaultAgentName()
			notifCmd := m.notify("Starting agent…", notifInfo)

			var startCmd tea.Cmd
			if mode == "repo" {
				selected := *m.addRepo
				if selected == "" {
					return m, tickCmd()
				}
				workspace, _ := config.Workspace()
				dirPath := filepath.Join(workspace, selected)
				branch := strings.TrimSpace(*m.addBranch)

				if isMainGitRepo(dirPath) {
					if branch != "" {
						// Git repo + branch → worktree.
						startCmd = func() tea.Msg {
							return agentStartedMsg{err: ctl.StartWorktree(dirPath, branch, agentType)}
						}
					} else {
						// Git repo, no branch → main checkout.
						startCmd = func() tea.Msg {
							return agentStartedMsg{err: ctl.StartMain(dirPath, "", agentType)}
						}
					}
				} else {
					// Plain directory — branch input is ignored.
					startCmd = func() tea.Msg {
						return agentStartedMsg{err: ctl.Start("", dirPath, agentType)}
					}
				}
			} else {
				dir := expandTilde(strings.TrimSpace(*m.addDir))
				if dir == "" {
					return m, tickCmd()
				}
				startCmd = func() tea.Msg {
					return agentStartedMsg{err: ctl.Start("", dir, agentType)}
				}
			}

			return m, tea.Batch(tickCmd(), notifCmd, startCmd, waitProgressCmd())
		}
		if m.addForm.State == huh.StateAborted {
			m.adding = false
			m.addForm = nil
			m.table.Focus()
			return m, tickCmd()
		}

		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirming {
			switch msg.String() {
			case "y", "enter":
				name := m.confirmName
				m.confirming = false
				m.confirmName = ""
				notifCmd := m.notify("Removing agent…", notifInfo)
				removeCmd := func() tea.Msg {
					a, err := dataStore.Get(name)
					if err != nil {
						return agentRemovedMsg{err: err}
					}
					return agentRemovedMsg{err: ctl.Remove(a)}
				}
				return m, tea.Batch(notifCmd, removeCmd)
			case "n", "esc":
				m.confirming = false
				m.confirmName = ""
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if len(m.agentNames) > 0 {
				cursor := m.table.Cursor()
				if cursor < len(m.agentNames) {
					a, err := dataStore.Get(m.agentNames[cursor])
					if err != nil {
						return m, nil
					}
					// If the window is alive, switch to it.
					if a.WindowID != "" {
						if alive, err := mux.WindowExists(a.WindowID); err == nil && alive {
							mux.SelectWindow(a.WindowID)
							return m, nil
						}
					}
					// Window is dead or was never opened — reopen the agent.
					notifCmd := m.notify("Reopening agent…", notifInfo)
					startCmd := func() tea.Msg {
						return agentStartedMsg{err: ctl.Reopen(a)}
					}
					return m, tea.Batch(notifCmd, startCmd, waitProgressCmd())
				}
			}
			return m, nil
		case "d":
			if len(m.agentNames) > 0 {
				cursor := m.table.Cursor()
				if cursor < len(m.agentNames) {
					m.confirming = true
					m.confirmName = m.agentNames[cursor]
				}
			}
			return m, nil
		case "s":
			mode := "repo"
			repo := ""
			branch := ""
			dir := ""
			m.addMode = &mode
			m.addRepo = &repo
			m.addBranch = &branch
			m.addDir = &dir
			m.addForm = newAddForm(m.addMode, m.addRepo, m.addBranch, m.addDir)
			m.adding = true
			m.table.Blur()
			return m, m.addForm.Init()
		}
	case agentRemovedMsg:
		if msg.err != nil {
			return m, m.notify(msg.err.Error(), notifError)
		}
		m.refreshRows()
		return m, m.notify("Agent removed", notifInfo)
	case agentProgressMsg:
		m.notifyProgress(msg.status)
		return m, waitProgressCmd()
	case agentStartedMsg:
		if msg.err != nil {
			return m, m.notify(msg.err.Error(), notifError)
		}
		m.refreshRows()
		return m, m.notify("Agent started", notifInfo)
	case notifClearMsg:
		m.notif = nil
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		m.refreshRows()
		return m, nil
	case tickMsg:
		if !m.confirming && !m.adding {
			m.refreshRows()
		}
		return m, tickCmd()
	}

	if !m.confirming && !m.adding && len(m.table.Rows()) > 0 {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m watchModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	innerW := m.width - marginX*2
	title := renderGradientTitle("Agents", innerW)

	agentCount := len(m.table.Rows())
	helpText := lipgloss.NewStyle().
		Foreground(colorDim).
		Render(fmt.Sprintf(
			"↑/↓ navigate • enter switch/reopen • s start • d remove • q quit • %d agent(s)",
			agentCount,
		))

	// Notification — rendered on the right side of the help bar, truncated to fit.
	helpW := lipgloss.Width(helpText)
	maxNotifW := max(0, innerW-helpW-1) // 1 for the separator space
	var notifText string
	if m.notif != nil {
		color := colorGreen
		prefix := ""
		if m.notif.kind == notifError {
			color = colorRed
			prefix = "error: "
		}
		raw := prefix + m.notif.msg
		// Truncate with ellipsis if the message is too wide.
		if len([]rune(raw)) > maxNotifW && maxNotifW > 1 {
			raw = string([]rune(raw)[:maxNotifW-1]) + "…"
		}
		notifText = lipgloss.NewStyle().Foreground(color).Render(raw)
	}

	// Place help on the left and notification on the right on a single line.
	notifW := lipgloss.Width(notifText)
	gap := max(0, innerW-helpW-notifW)
	footer := helpText + strings.Repeat(" ", gap) + notifText
	top := lipgloss.JoinVertical(lipgloss.Left, title, m.table.View())

	// Manually build the layout so the footer is truly pinned to the bottom.
	// No bottom margin — the terminal edge is enough.
	innerH := m.height - marginY
	vgap := strings.Repeat("\n", max(0, innerH-lipgloss.Height(top)-lipgloss.Height(footer)))
	hPad := strings.Repeat(" ", marginX)

	var sb strings.Builder
	sb.WriteString(strings.Repeat("\n", marginY))
	for i, line := range strings.Split(top+vgap+footer, "\n") {
		// Don't add a trailing newline after the last line.
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(hPad + line)
	}
	base := sb.String()

	if m.confirming {
		popup := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorRed).
			Padding(1, 3).
			Bold(true).
			Foreground(colorRed).
			Render(fmt.Sprintf("Remove agent %q?\n\n  [y] Yes    [n] No", m.confirmName))

		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			popup,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(colorBg),
		)
	}

	if m.adding && m.addForm != nil {
		popupW := max(60, min(m.width*2/3, 120))
		formView := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPurple).
			Padding(1, 3).
			Width(popupW).
			Render(m.addForm.View())

		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			formView,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(colorBg),
		)
	}

	return base
}

// renderGradientTitle renders "///// Agents ////..." filling the width,
// with the slashes in a purple gradient and the title in solid purple.
func renderGradientTitle(text string, width int) string {
	gradientColors := []lipgloss.Color{
		lipgloss.Color("63"),
		lipgloss.Color("99"),
		lipgloss.Color("135"),
		lipgloss.Color("171"),
		lipgloss.Color("207"),
		lipgloss.Color("200"),
		lipgloss.Color("199"),
	}

	label := " " + text + " "
	labelW := len([]rune(label))
	leftCount := 5
	rightCount := max(0, width-leftCount-labelW)
	totalSlashes := leftCount + rightCount

	var b strings.Builder
	// Left slashes.
	for i := 0; i < leftCount; i++ {
		ci := i * (len(gradientColors) - 1) / max(totalSlashes-1, 1)
		b.WriteString(lipgloss.NewStyle().Foreground(gradientColors[ci]).Render("/"))
	}
	// Title in purple.
	b.WriteString(lipgloss.NewStyle().Foreground(colorPurple).Bold(true).Render(label))
	// Right slashes.
	for i := 0; i < rightCount; i++ {
		ci := (leftCount + i) * (len(gradientColors) - 1) / max(totalSlashes-1, 1)
		b.WriteString(lipgloss.NewStyle().Foreground(gradientColors[ci]).Render("/"))
	}

	return lipgloss.NewStyle().MarginBottom(1).Render(b.String())
}

// repoLabel returns the repository or directory name to display in the table.
// For git agents it uses the repo root basename; for plain directories it uses
// the workdir basename.
func repoLabel(a agent.Agent) string {
	if a.RepoPath != "" {
		return filepath.Base(a.RepoPath)
	}
	return filepath.Base(a.WorkdirPath)
}

// liveWindowIndex queries tmux for the current numeric index of windowID.
func liveWindowIndex(windowID string) string {
	cmd := exec.Command("tmux", "display-message", "-t", windowID, "-p", "#{window_index}")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// liveBranch returns the current git branch for the given directory, or an
// empty string if the directory is not a git repo or is in detached HEAD state.
func liveBranch(dir string) string {
	if dir == "" {
		return ""
	}
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return ""
	}
	return branch
}

// listWorkspaceDirs returns all directory names directly inside the workspace.
// Both plain directories and git repositories are included.
func listWorkspaceDirs() []string {
	workspace, err := config.Workspace()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

// isMainGitRepo reports whether dir is a main git repository
// (i.e. contains a .git directory, not a .git file as linked worktrees do).
func isMainGitRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

func init() {
	rootCmd.AddCommand(watchCmd)
}
