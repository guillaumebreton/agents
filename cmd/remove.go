package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <repo>",
	Short: "Remove an agent, its worktree, and clean up state",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("remove: not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
