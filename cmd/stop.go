package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <repo>",
	Short: "Stop a running agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo := args[0]

		a, err := dataStore.Get(repo)
		if err != nil {
			return fmt.Errorf("agent %q not found", repo)
		}

		if a.WindowID == "" {
			fmt.Printf("agent %q has no active window\n", repo)
			return nil
		}

		// Check if the window is still alive before killing.
		alive, err := mux.WindowExists(a.WindowID)
		if err != nil {
			return err
		}
		if alive {
			if err := mux.KillWindow(a.WindowID); err != nil {
				return fmt.Errorf("killing window for agent %q: %w", repo, err)
			}
		}

		// Clear window ID and persist.
		a.WindowID = ""
		if err := dataStore.Save(a); err != nil {
			return err
		}

		fmt.Printf("stopped agent %q\n", repo)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
