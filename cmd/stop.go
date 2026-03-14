package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <repo>",
	Short: "Stop a running agent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("stop: not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
