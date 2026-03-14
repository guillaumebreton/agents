package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "agents",
	Short: "Simple agent watcher for coding agents",
	Long:  "A CLI tool that manages coding agents in terminal multiplexer sessions, handling git worktrees and agent lifecycle.",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
