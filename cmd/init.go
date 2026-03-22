package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"notb.re/agents/internal/coding"
	"notb.re/agents/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the agents configuration and install hooks",
	Long: `Create the default configuration file if it doesn't exist and
(re)install coding agent hooks for status reporting.

Sets the current directory as the workspace.
The config is written to ~/.config/agents/config.json.

Hooks are always overwritten, so re-running init after upgrading agents
is all that is needed to pick up the latest hook content.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Config.
		if config.Exists() {
			fmt.Printf("config already exists at %s\n", config.Path())
		} else {
			if err := config.Init(); err != nil {
				return err
			}
			fmt.Printf("config created at %s\n", config.Path())
		}

		// Resolve the agents binary path for hooks.
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolving executable path: %w", err)
		}

		// (Re)install hooks for all registered coding agents.
		for _, name := range coding.List() {
			ca, _ := coding.Get(name)

			hookPath := ca.HookPath()
			if hookPath == "" {
				continue
			}
			hookContent := ca.Hook(exe)
			if hookContent == "" {
				continue
			}

			if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
				return fmt.Errorf("creating hook directory: %w", err)
			}
			if err := os.WriteFile(hookPath, []byte(hookContent), 0o644); err != nil {
				return fmt.Errorf("writing %s hook: %w", name, err)
			}
			fmt.Printf("%s hook installed at %s\n", name, hookPath)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
