package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"notb.re/agents/internal/config"
	"notb.re/agents/internal/multiplexer"
	"notb.re/agents/internal/store"
)

const sessionName = "agents"

var mux multiplexer.Multiplexer = multiplexer.NewTmux()
var dataStore store.Store

func init() {
	s, err := store.NewJSONStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dataStore = s
}

var rootCmd = &cobra.Command{
	Use:   "agents",
	Short: "Simple agent watcher for coding agents",
	Long:  "A CLI tool that manages coding agents in terminal multiplexer sessions, handling git worktrees and agent lifecycle.",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			// Launch watch in the first window.
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

// Execute runs the root command.
func Execute() {
	log.SetLevel(log.DebugLevel)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
