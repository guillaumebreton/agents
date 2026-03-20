package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"notb.re/agents/internal/agentctl"
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
	table         table.Model
	agentNames    []string // canonical agent names indexed by row, used for all lookups
	width         int
	height        int
	err           error
	confirming    bool   // true when showing delete confirmation
	confirmName   string // name of the agent to delete
	adding        bool   // true when the add popup is open
	addForm       *huh.Form
	addRepo       *string // heap-allocated so huh can write back through the pointer
	addBranch     *string // heap-allocated so huh can write back through the pointer
	notif         *notification
	staleWorktree *agentctl.ErrStaleWorktree // non-nil when awaiting stale worktree confirmation
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
	agentTypeW := 12
	statusW := 12
	fixed := agentTypeW + statusW
	remaining := width - fixed - 8 // padding allowance (2 per column × 4 extra columns)
	remaining = max(remaining, 20)
	nameW := remaining * 25 / 100
	repoW := remaining * 25 / 100
	worktreeW := remaining - nameW - repoW

	return []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Agent", Width: agentTypeW},
		{Title: "Repository", Width: repoW},
		{Title: "Worktree", Width: worktreeW},
		{Title: "Status", Width: statusW},
	}
}

func (m *watchModel) refreshRows() {
	agents, err := dataStore.List()
	if err != nil {
		m.err = err
		return
	}

	// Stable ordering by name.
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})

	cursor := m.table.Cursor()

	rows := make([]table.Row, 0, len(agents))
	names := make([]string, 0, len(agents))
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
			} else {
				// Window is dead — clean up stale window state.
				a.WindowID = ""
				a.PanePID = ""
				a.Status = "exited"
				dataStore.Save(a)
				status = "✕ exited"
			}
		}
		names = append(names, a.Name)
		repo, wt := splitWorktree(a.WorktreePath)
		rows = append(rows, table.Row{a.Name, a.AgentType, repo, wt, status})
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
func newAddForm(repo *string, branch *string) *huh.Form {
	repos := listRepos()
	opts := make([]huh.Option[string], len(repos))
	for i, r := range repos {
		opts[i] = huh.NewOption(r, r)
	}
	if len(opts) == 0 {
		opts = []huh.Option[string]{huh.NewOption("(no repos found)", "")}
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Repository").
				Description("Select a repository in the workspace").
				Options(opts...).
				Value(repo),
			huh.NewInput().
				Title("Branch name").
				Description("Enter the branch to work on (new or existing)").
				Value(branch).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("branch name cannot be empty")
					}
					return nil
				}),
		),
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
			repo := *m.addRepo
			branch := *m.addBranch
			m.addForm = nil
			m.table.Focus()
			if repo != "" && branch != "" {
				notifCmd := m.notify("Starting agent…", notifInfo)
				startCmd := func() tea.Msg {
					return agentStartedMsg{err: ctl.Start(repo, branch, "opencode")}
				}
				return m, tea.Batch(tickCmd(), notifCmd, startCmd, waitProgressCmd())
			}
			return m, tickCmd()
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
		if m.staleWorktree != nil {
			switch msg.String() {
			case "y", "enter":
				stale := m.staleWorktree
				m.staleWorktree = nil
				notifCmd := m.notify("Pruning stale worktree, retrying…", notifInfo)
				startCmd := func() tea.Msg {
					return agentStartedMsg{err: ctl.ForceStart(stale)}
				}
				return m, tea.Batch(notifCmd, startCmd, waitProgressCmd())
			case "n", "esc":
				m.staleWorktree = nil
				return m, m.notify("Cancelled", notifInfo)
			}
			return m, nil
		}

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
					if err == nil && a.WindowID != "" {
						if alive, err := mux.WindowExists(a.WindowID); err == nil && alive {
							mux.SelectWindow(a.WindowID)
						}
					}
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
			repo := ""
			branch := ""
			m.addRepo = &repo
			m.addBranch = &branch
			m.addForm = newAddForm(m.addRepo, m.addBranch)
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
			// Check for stale worktree — show a confirmation popup instead of an error.
			if stale, ok := msg.err.(*agentctl.ErrStaleWorktree); ok {
				m.staleWorktree = stale
				return m, nil
			}
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
			"↑/↓ navigate • enter switch • s start • d remove • q quit • %d agent(s)",
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

	if m.staleWorktree != nil {
		popup := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorRed).
			Padding(1, 3).
			Bold(true).
			Foreground(colorRed).
			Render(fmt.Sprintf("Stale worktree registration found:\n%s\n\nPrune and recreate?\n\n  [y] Yes    [n] No", m.staleWorktree.WorktreePath))

		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			popup,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(colorBg),
		)
	}

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

// splitWorktree extracts the repository name and the worktree branch directory
// from an absolute worktree path.  The repo is derived by stripping the
// "_worktrees" suffix from the parent directory name.
func splitWorktree(worktreePath string) (repo, worktree string) {
	workspace, err := config.Workspace()
	if err != nil {
		return "", worktreePath
	}
	rel, ok := strings.CutPrefix(worktreePath, workspace)
	if !ok {
		return "", worktreePath
	}
	rel = strings.TrimPrefix(rel, "/")

	parts := strings.SplitN(rel, "/", 2)
	if len(parts) != 2 {
		return "", rel
	}
	repo = strings.TrimSuffix(parts[0], "_worktrees")
	worktree = parts[1]
	return repo, worktree
}

// listRepos returns the names of git repositories found directly inside the workspace.
func listRepos() []string {
	workspace, err := config.Workspace()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return nil
	}
	var repos []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Skip worktree sibling directories (name ends with _worktrees).
		if strings.HasSuffix(e.Name(), "_worktrees") {
			continue
		}
		// Accept only directories that contain a .git entry.
		gitPath := filepath.Join(workspace, e.Name(), ".git")
		if _, err := os.Stat(gitPath); err == nil {
			repos = append(repos, e.Name())
		}
	}
	sort.Strings(repos)
	return repos
}

func init() {
	rootCmd.AddCommand(watchCmd)
}
