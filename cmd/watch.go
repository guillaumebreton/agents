package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch and display all tracked agents",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("watch: not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
}
