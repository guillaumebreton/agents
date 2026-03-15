package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <repo>",
	Short: "Remove an agent, its worktree, and clean up state",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo := args[0]

		a, err := dataStore.Get(repo)
		if err != nil {
			return fmt.Errorf("agent %q not found", repo)
		}

		// Kill the tmux window if it's still alive.
		if a.WindowID != "" {
			alive, err := mux.WindowExists(a.WindowID)
			if err != nil {
				return err
			}
			if alive {
				if err := mux.KillWindow(a.WindowID); err != nil {
					return fmt.Errorf("killing window for agent %q: %w", repo, err)
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
					// Try removing the directory directly as a fallback.
					if rmErr := os.RemoveAll(a.WorktreePath); rmErr != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to remove directory: %v\n", rmErr)
					}
				}
				fmt.Printf("removed worktree %s\n", a.WorktreePath)
			}
		}

		// Delete from store.
		if err := dataStore.Delete(repo); err != nil {
			return err
		}

		fmt.Printf("removed agent %q\n", repo)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
