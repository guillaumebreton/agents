package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"notb.re/agents/internal/agent"
)

var updateStatusCmd = &cobra.Command{
	Use:   "update-status",
	Short: "Update the status of a tracked agent",
	Long: `Update the status of a tracked agent by its pane PID.

This command is meant to be called by coding agent hooks (e.g. opencode plugins),
not directly by the user.

Valid statuses: idle, working, waiting, exited`,
	RunE: func(cmd *cobra.Command, args []string) error {
		panePID, _ := cmd.Flags().GetString("pane-pid")
		status, _ := cmd.Flags().GetString("status")

		if panePID == "" {
			return fmt.Errorf("--pane-pid is required")
		}
		if status == "" {
			return fmt.Errorf("--status is required")
		}
		if !agent.ValidStatus(status) {
			return fmt.Errorf("invalid status %q (valid: idle, working, waiting, exited)", status)
		}

		a, err := dataStore.GetByPanePID(panePID)
		if err != nil {
			// Silently exit when no tracked agent matches this pane PID.
			// This happens when opencode runs outside a managed worktree
			// (e.g. directly in a repo, not via "agents start").
			return nil
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
	updateStatusCmd.Flags().String("pane-pid", "", "pane PID of the agent")
	updateStatusCmd.Flags().String("status", "", "new status (idle, working, waiting, exited)")
	rootCmd.AddCommand(updateStatusCmd)
}
