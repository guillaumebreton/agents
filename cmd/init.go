package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"notb.re/agents/internal/coding"
	"notb.re/agents/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the agents configuration and install hooks",
	Long: `Create the default configuration file if it doesn't exist and install
coding agent hooks for status reporting.

Sets the current directory as the workspace.
The config is written to ~/.config/agents/config.json.

Hooks are installed for all supported coding agents (currently: opencode).
Re-run init after upgrading agents to update hooks to the latest version.`,
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

		// Install hooks for all registered coding agents.
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

			// Check if hook needs updating by comparing version.
			if existing, err := os.ReadFile(hookPath); err == nil {
				if strings.Contains(string(existing), "Version: "+coding.HookVersion) {
					fmt.Printf("%s hook is up to date (%s)\n", name, hookPath)
					continue
				}
			}

			// Install or update the hook.
			if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
				return fmt.Errorf("creating hook directory: %w", err)
			}
			if err := os.WriteFile(hookPath, []byte(hookContent), 0o644); err != nil {
				return fmt.Errorf("writing %s hook: %w", name, err)
			}
			fmt.Printf("%s hook installed at %s (version: %s)\n", name, hookPath, coding.HookVersion)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
