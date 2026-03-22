package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"notb.re/agents/internal/agent"
	"notb.re/agents/internal/notify"
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
		agentType, _ := cmd.Flags().GetString("agent-type")

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
			return nil
		}

		oldStatus := a.Status
		a.Status = agent.Status(status)
		// Keep the agent type up to date in case the agent was restarted.
		if agentType != "" {
			a.AgentType = agentType
		}
		if err := dataStore.Save(a); err != nil {
			return err
		}

		// Fire a desktop notification on transitions that need human attention.
		newStatus := agent.Status(status)
		switch {
		case oldStatus == agent.StatusWorking && newStatus == agent.StatusIdle:
			notify.Send("Agents", fmt.Sprintf("%s finished", a.Name))
		case newStatus == agent.StatusWaiting && oldStatus != agent.StatusWaiting:
			notify.Send("Agents", fmt.Sprintf("%s is waiting for input", a.Name))
		}

		fmt.Printf("updated %q status to %s\n", a.Name, status)
		return nil
	},
}

func init() {
	updateStatusCmd.Flags().String("pane-pid", "", "pane PID of the agent")
	updateStatusCmd.Flags().String("status", "", "new status (idle, working, waiting, exited)")
	updateStatusCmd.Flags().String("agent-type", "", "coding agent type (e.g. opencode, pi); kept in sync on each call")
	rootCmd.AddCommand(updateStatusCmd)
}
