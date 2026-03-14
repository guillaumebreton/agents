package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <repo> [branch]",
	Short: "Start an agent for a repository",
	Long: `Start an agent for the given repository.

On the first call, a branch is required to create the git worktree.
On subsequent calls, the branch is optional — the existing worktree is reused.

If a window already exists, checks it is still running. Otherwise opens a new
tmux window in the worktree and launches the coding agent.

Use 'agents start all' to start all tracked agents.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("start: not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
