package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"notb.re/agents/internal/config"
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
	Use:   "watch",
	Short: "Watch and display all tracked agents",
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

type watchModel struct {
	table       table.Model
	width       int
	height      int
	err         error
	confirming  bool   // true when showing delete confirmation
	confirmName string // name of the agent to delete
}

func newWatchModel() watchModel {
	t := table.New(
		table.WithColumns(buildColumns(80)),
		table.WithFocused(true),
		table.WithHeight(20),
	)
	t.SetStyles(tableStyles())

	m := watchModel{table: t, width: 80, height: 24}
	m.refreshRows()
	return m
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPurple).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorSubtle).
		BorderBottom(true).
		Padding(0, 1)
	s.Cell = lipgloss.NewStyle().
		Foreground(colorWhite).
		Padding(0, 1)
	s.Selected = lipgloss.NewStyle().
		Foreground(colorCream).
		Background(colorSelectBg).
		Bold(true)
	return s
}

func buildColumns(width int) []table.Column {
	// Fixed-width columns.
	agentTypeW := 12
	pidW := 10
	windowW := 10
	statusW := 12
	fixed := agentTypeW + pidW + windowW + statusW
	remaining := width - fixed - 12 // padding allowance
	remaining = max(remaining, 20)
	nameW := remaining * 30 / 100
	worktreeW := remaining - nameW

	return []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Agent", Width: agentTypeW},
		{Title: "Worktree", Width: worktreeW},
		{Title: "PID", Width: pidW},
		{Title: "Window", Width: windowW},
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
	for _, a := range agents {
		status := "○ stopped"
		windowID := "-"
		panePID := "-"
		if a.WindowID != "" {
			windowID = a.WindowID
			if a.PanePID != "" {
				panePID = a.PanePID
			}
			alive, err := mux.WindowExists(a.WindowID)
			if err == nil && alive {
				// Use the reported status if available.
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
				windowID = "-"
				panePID = "-"
				status = "✕ exited"
			}
		}
		rows = append(rows, table.Row{a.Name, a.AgentType, shortenWorktree(a.WorktreePath), panePID, windowID, status})
	}

	m.table.SetRows(rows)

	// Preserve cursor position across refreshes.
	if cursor < len(rows) {
		m.table.SetCursor(cursor)
	}
}

func (m *watchModel) recalcLayout() {
	m.table.SetColumns(buildColumns(m.width))
	// Title (1) + margin (1) + help (1) + margin (1) + header border (1) = 5 lines overhead
	tableHeight := m.height - 5
	tableHeight = max(tableHeight, 3)
	m.table.SetHeight(tableHeight)
}

func (m watchModel) Init() tea.Cmd {
	return tickCmd()
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirming {
			switch msg.String() {
			case "y":
				// Perform the removal.
				a, err := dataStore.Get(m.confirmName)
				if err == nil {
					removeAgent(a)
				}
				m.confirming = false
				m.confirmName = ""
				m.refreshRows()
				return m, nil
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
		case "d":
			row := m.table.SelectedRow()
			if row != nil {
				m.confirming = true
				m.confirmName = row[0]
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		return m, nil
	case tickMsg:
		if !m.confirming {
			m.refreshRows()
		}
		return m, tickCmd()
	}

	if !m.confirming {
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

	title := renderGradientTitle("Agents")

	helpStyle := lipgloss.NewStyle().
		Foreground(colorDim).
		MarginTop(1)

	var helpText string
	if m.confirming {
		helpText = lipgloss.NewStyle().
			Foreground(colorRed).
			MarginTop(1).
			Bold(true).
			Render(fmt.Sprintf("Remove agent %q? y/n", m.confirmName))
	} else {
		agentCount := len(m.table.Rows())
		helpText = helpStyle.Render(fmt.Sprintf(
			"↑/↓ navigate • d remove • q quit • %d agent(s)",
			agentCount,
		))
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		m.table.View(),
		helpText,
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(content)
}

// renderGradientTitle renders "///// Title" with a purple gradient on the slashes.
func renderGradientTitle(text string) string {
	// Purple gradient from dim to bright.
	gradientColors := []lipgloss.Color{
		lipgloss.Color("53"),
		lipgloss.Color("55"),
		lipgloss.Color("57"),
		lipgloss.Color("63"),
		lipgloss.Color("99"),
	}

	var b strings.Builder
	for i, c := range gradientColors {
		style := lipgloss.NewStyle().Foreground(c).Bold(true)
		_ = i
		b.WriteString(style.Render("/"))
	}
	b.WriteString(" ")
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Render(text))

	return lipgloss.NewStyle().MarginBottom(1).Render(b.String())
}

// shortenWorktree returns the worktree path relative to the workspace.
func shortenWorktree(worktreePath string) string {
	workspace, err := config.Workspace()
	if err != nil {
		return worktreePath
	}
	rel, ok := strings.CutPrefix(worktreePath, workspace)
	if !ok {
		return worktreePath
	}
	return strings.TrimPrefix(rel, "/")
}

func init() {
	rootCmd.AddCommand(watchCmd)
}
