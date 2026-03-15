package cmd

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
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
	table  table.Model
	width  int
	height int
	err    error
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
		Bold(true).
		Padding(0, 1)
	return s
}

func buildColumns(width int) []table.Column {
	// Reserve space: 4 columns with 2 padding each = 8 chars padding per col
	// Window (10) + Status (10) = 20 fixed
	// Remaining split: Name gets 30%, Worktree gets 70%
	windowW := 12
	statusW := 12
	fixed := windowW + statusW
	remaining := width - fixed - 8 // padding allowance
	if remaining < 20 {
		remaining = 20
	}
	nameW := remaining * 30 / 100
	worktreeW := remaining - nameW

	return []table.Column{
		{Title: "Name", Width: nameW},
		{Title: "Worktree", Width: worktreeW},
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

	cursor := m.table.Cursor()

	rows := make([]table.Row, 0, len(agents))
	for _, a := range agents {
		status := "○ stopped"
		windowID := "-"
		if a.WindowID != "" {
			windowID = a.WindowID
			alive, err := mux.WindowExists(a.WindowID)
			if err == nil && alive {
				status = "● running"
			} else {
				status = "✕ dead"
			}
		}
		rows = append(rows, table.Row{a.Name, a.WorktreePath, windowID, status})
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
	if tableHeight < 3 {
		tableHeight = 3
	}
	m.table.SetHeight(tableHeight)
}

func (m watchModel) Init() tea.Cmd {
	return tickCmd()
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		return m, nil
	case tickMsg:
		m.refreshRows()
		return m, tickCmd()
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m watchModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPurple).
		MarginBottom(1).
		Render("◆ Agents")

	helpStyle := lipgloss.NewStyle().
		Foreground(colorDim).
		MarginTop(1)

	agentCount := len(m.table.Rows())
	help := helpStyle.Render(fmt.Sprintf(
		"↑/↓ navigate • q quit • %d agent(s) • refreshes every 2s",
		agentCount,
	))

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		m.table.View(),
		help,
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Background(colorBg).
		Render(content)
}

func init() {
	rootCmd.AddCommand(watchCmd)
}
