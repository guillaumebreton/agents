package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"notb.re/agents/internal/agentctl"
	"notb.re/agents/internal/config"
	"notb.re/agents/internal/multiplexer"
	"notb.re/agents/internal/store"
)

const sessionName = "agents"

var debug bool
var mux multiplexer.Multiplexer = multiplexer.NewTmux()
var dataStore store.Store
var ctl *agentctl.Controller

func init() {
	s, err := store.NewJSONStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dataStore = s
	ctl = &agentctl.Controller{
		Store:       s,
		Mux:         mux,
		SessionName: sessionName,
		Progress:    func(msg string) { progressCh <- msg },
	}
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// runSetup interactively asks the user to configure the workspace on first run.
func runSetup() error {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}

	const optCurrent = "current"
	const optCustom = "custom"

	choice := optCurrent
	customPath := ""
	branchPrefix := ""
	defaultAgent := "opencode"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("No configuration found. Where is your workspace?").
				Description("The workspace is the directory that contains your repositories.").
				Options(
					huh.NewOption(fmt.Sprintf("Use current directory (%s)", cwd), optCurrent),
					huh.NewOption("Type a custom path", optCustom),
				).
				Value(&choice),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Workspace path").
				Description("Enter the absolute path to your workspace directory.").
				Value(&customPath).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("path cannot be empty")
					}
					expanded := expandTilde(s)
					info, err := os.Stat(expanded)
					if err != nil {
						return fmt.Errorf("directory not found: %s", expanded)
					}
					if !info.IsDir() {
						return fmt.Errorf("not a directory: %s", expanded)
					}
					return nil
				}),
		).WithHideFunc(func() bool { return choice != optCustom }),
		huh.NewGroup(
			huh.NewInput().
				Title("Branch prefix (optional)").
				Description(`Prefix added to every created branch, e.g. "agent/".`).
				Value(&branchPrefix),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default coding agent").
				Description("Used when --agent is not specified on the command line.").
				Options(
					huh.NewOption("opencode", "opencode"),
					huh.NewOption("pi", "pi"),
				).
				Value(&defaultAgent),
		),
	)

	if err := form.Run(); err != nil {
		if err == huh.ErrUserAborted {
			return fmt.Errorf("setup cancelled")
		}
		// Any other error (e.g. no TTY when launched non-interactively) is ignored so
		// that subcommands launched via tmux SendCommand are not blocked.
		return nil
	}

	workspace := cwd
	if choice == optCustom {
		workspace = expandTilde(customPath)
	}

	if err := config.SaveConfig(workspace, branchPrefix, defaultAgent); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("Workspace set to %s\n", workspace)
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "agents",
	Short: "Simple agent watcher for coding agents",
	Long:  "A CLI tool that manages coding agents in terminal multiplexer sessions, handling git worktrees and agent lifecycle.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if debug {
			log.SetLevel(log.DebugLevel)
		} else {
			log.SetLevel(log.InfoLevel)
		}
		if !config.Exists() {
			log.Info("Config doesn't exist")
			if err := runSetup(); err != nil {
				return err
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Already inside the tmux session — just run the watch TUI directly.
		if os.Getenv("TMUX") != "" {
			p := tea.NewProgram(newWatchModel(), tea.WithAltScreen())
			_, err := p.Run()
			return err
		}

		// Outside tmux — ensure the session exists, start watch on window 1, attach.
		workspace, err := config.Workspace()
		if err != nil {
			return err
		}
		exists, err := mux.SessionExists(sessionName)
		if err != nil {
			return err
		}
		if !exists {
			if err := mux.CreateSession(sessionName, workspace); err != nil {
				return err
			}
			windows, err := mux.ListWindows(sessionName)
			if err != nil {
				return fmt.Errorf("listing windows: %w", err)
			}
			if len(windows) > 0 {
				exe, err := os.Executable()
				if err != nil {
					return fmt.Errorf("resolving executable path: %w", err)
				}
				if err := mux.SendCommand(windows[0].ID, exe+" watch"); err != nil {
					return fmt.Errorf("launching watch: %w", err)
				}
			}
		}
		return mux.AttachSession(sessionName)
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
