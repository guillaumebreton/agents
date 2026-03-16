package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"notb.re/agents/internal/agent"
)

var updateStatusCmd = &cobra.Command{
	Use:   "update-status",
	Short: "Update the status of a tracked agent",
	Long: `Update the status of a tracked agent by its worktree path.

This command is meant to be called by coding agent hooks (e.g. opencode plugins),
not directly by the user.

Valid statuses: idle, working, waiting, exited`,
	RunE: func(cmd *cobra.Command, args []string) error {
		worktree, _ := cmd.Flags().GetString("worktree")
		status, _ := cmd.Flags().GetString("status")

		if worktree == "" {
			return fmt.Errorf("--worktree is required")
		}
		if status == "" {
			return fmt.Errorf("--status is required")
		}
		if !agent.ValidStatus(status) {
			return fmt.Errorf("invalid status %q (valid: idle, working, waiting, exited)", status)
		}

		a, err := dataStore.GetByWorktree(worktree)
		if err != nil {
			return err
		}

		a.Status = agent.Status(status)
		if err := dataStore.Save(a); err != nil {
			return err
		}

		fmt.Printf("updated %q status to %s\n", a.Name, status)
		return nil
	},
}

func init() {
	updateStatusCmd.Flags().String("worktree", "", "worktree path of the agent")
	updateStatusCmd.Flags().String("status", "", "new status (idle, working, waiting, exited)")
	rootCmd.AddCommand(updateStatusCmd)
}
