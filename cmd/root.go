package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"notb.re/agent/internal/multiplexer"
)

const sessionName = "agents"

var mux multiplexer.Multiplexer = multiplexer.NewTmux()

var rootCmd = &cobra.Command{
	Use:   "agents",
	Short: "Simple agent watcher for coding agents",
	Long:  "A CLI tool that manages coding agents in terminal multiplexer sessions, handling git worktrees and agent lifecycle.",
	RunE: func(cmd *cobra.Command, args []string) error {
		exists, err := mux.SessionExists(sessionName)
		if err != nil {
			return err
		}
		if !exists {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting working directory: %w", err)
			}
			if err := mux.CreateSession(sessionName, cwd); err != nil {
				return err
			}
		}
		return mux.AttachSession(sessionName)
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
