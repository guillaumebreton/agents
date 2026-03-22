package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"notb.re/agents/internal/coding"
)

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a running agent session with the watcher",
	Long: `Register the current coding agent session so it appears in the agents watcher.

Intended to be called automatically by coding agent hooks on startup.
Silently exits when not running inside tmux or when agents is not configured.

Example (from a hook):
  agents register --pane-id $TMUX_PANE --workdir /path/to/project --agent-type opencode`,
	RunE: func(cmd *cobra.Command, args []string) error {
		paneID, _ := cmd.Flags().GetString("pane-id")
		workdir, _ := cmd.Flags().GetString("workdir")
		agentType, _ := cmd.Flags().GetString("agent-type")
		windowID, _ := cmd.Flags().GetString("window-id")
		panePID, _ := cmd.Flags().GetString("pane-pid")

		if workdir == "" {
			return fmt.Errorf("--workdir is required")
		}
		if agentType == "" {
			return fmt.Errorf("--agent-type is required")
		}
		if !isKnownAgentType(agentType) {
			return fmt.Errorf("unknown agent type %q (known: %v)", agentType, coding.List())
		}

		// If window-id and pane-pid are given directly, skip the tmux lookup.
		// This is used by tests and non-tmux environments.
		windowName, _ := cmd.Flags().GetString("window-name")
		if windowID == "" || panePID == "" {
			// Fall back to resolving from the tmux pane ID.
			if paneID == "" {
				// Not inside tmux — silently do nothing.
				return nil
			}
			var err error
			windowID, panePID, windowName, err = mux.PaneInfo(paneID)
			if err != nil {
				// Pane lookup failed (e.g. tmux not reachable) — silently do nothing.
				return nil
			}
		}

		return ctl.Adopt(windowID, panePID, windowName, workdir, agentType)
	},
}

func isKnownAgentType(name string) bool {
	for _, n := range coding.List() {
		if n == name {
			return true
		}
	}
	return false
}

func init() {
	registerCmd.Flags().String("pane-id", "", "tmux pane ID (e.g. %3); typically $TMUX_PANE")
	registerCmd.Flags().String("workdir", "", "working directory of the agent")
	registerCmd.Flags().String("agent-type", "", "coding agent type (e.g. opencode, pi)")
	registerCmd.Flags().String("window-id", "", "tmux window ID — bypasses pane-id lookup (used for testing)")
	registerCmd.Flags().String("pane-pid", "", "shell PID of the pane — bypasses pane-id lookup (used for testing)")
	registerCmd.Flags().String("window-name", "", "tmux window name — used alongside window-id/pane-pid overrides")
	rootCmd.AddCommand(registerCmd)
}
