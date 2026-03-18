package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"notb.re/agents/internal/agent"
)

var removeCmd = &cobra.Command{
	Use:   "remove [repo/branch]",
	Short: "Remove an agent, its worktree, and clean up state",
	Long: `Remove an agent by name (repo/branch), or auto-detect from the current tmux window.

If no name is given, detects which agent owns the current tmux window
and removes it.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var a agent.Agent
		var err error

		if len(args) == 1 {
			a, err = dataStore.Get(args[0])
			if err != nil {
				return fmt.Errorf("agent %q not found", args[0])
			}
		} else {
			// Auto-detect from current tmux pane.
			a, err = detectCurrentAgent()
			if err != nil {
				return err
			}
			fmt.Printf("detected agent %q from current window\n", a.Name)
		}

		if !removeForceFlag {
			var confirm bool
			err = huh.NewConfirm().
				Title(fmt.Sprintf("Remove agent %q?", a.Name)).
				Description("This will kill the window and remove the worktree.").
				Affirmative("Yes").
				Negative("No").
				Value(&confirm).
				Run()
			if err != nil {
				return err
			}
			if !confirm {
				fmt.Println("aborted")
				return nil
			}
		}

		return removeAgent(a)
	},
}

func detectCurrentAgent() (agent.Agent, error) {
	paneID := os.Getenv("TMUX_PANE")
	if paneID == "" {
		return agent.Agent{}, fmt.Errorf("not inside a tmux pane; specify the repo name explicitly")
	}

	windowID, err := mux.WindowIDForPane(paneID)
	if err != nil {
		return agent.Agent{}, err
	}

	// Find the agent that owns this window.
	agents, err := dataStore.List()
	if err != nil {
		return agent.Agent{}, err
	}
	for _, a := range agents {
		if a.WindowID == windowID {
			return a, nil
		}
	}
	return agent.Agent{}, fmt.Errorf("no tracked agent found for current window %s", windowID)
}

func removeAgent(a agent.Agent) error {
	// Kill the tmux window if it's still alive.
	if a.WindowID != "" {
		alive, err := mux.WindowExists(a.WindowID)
		if err != nil {
			return err
		}
		if alive {
			if err := mux.KillWindow(a.WindowID); err != nil {
				return fmt.Errorf("killing window for agent %q: %w", a.Name, err)
			}
			fmt.Printf("killed window %s\n", a.WindowID)
		}
	}

	// Remove the git worktree.
	if a.WorktreePath != "" {
		if _, err := os.Stat(a.WorktreePath); err == nil {
			gitCmd := exec.Command("git", "worktree", "remove", a.WorktreePath, "--force")
			if out, err := gitCmd.CombinedOutput(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to remove worktree: %s: %v\n", strings.TrimSpace(string(out)), err)
				if rmErr := os.RemoveAll(a.WorktreePath); rmErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to remove directory: %v\n", rmErr)
				}
			}
			fmt.Printf("removed worktree %s\n", a.WorktreePath)
		}
	}

	// Delete from store.
	if err := dataStore.Delete(a.Name); err != nil {
		return err
	}

	fmt.Printf("removed agent %q\n", a.Name)
	return nil
}

func init() {
	removeCmd.Flags().BoolVarP(&removeForceFlag, "force", "f", false, "skip confirmation prompt")
	rootCmd.AddCommand(removeCmd)
}

var removeForceFlag bool
