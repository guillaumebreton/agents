package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"notb.re/agents/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the agents configuration",
	Long: `Create the default configuration file if it doesn't exist.

Sets the current directory as the workspace and seeds default agent
definitions. The config is written to ~/.config/agents/config.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if config.Exists() {
			fmt.Printf("config already exists at %s\n", config.Path())
			return nil
		}
		if err := config.Init(); err != nil {
			return err
		}
		fmt.Printf("config created at %s\n", config.Path())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
