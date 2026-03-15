package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// AgentConfig defines a coding agent implementation.
type AgentConfig struct {
	// Command is the binary/command to run in the tmux window.
	Command string `json:"command"`
}

// Config holds the application configuration.
type Config struct {
	// Workspace is the root directory where all repositories live.
	Workspace string `json:"workspace"`

	// DefaultAgent is the agent shorthand used when --agent is not specified.
	// Defaults to "opencode".
	DefaultAgent string `json:"default_agent"`

	// Agents maps shorthand names to agent configurations.
	Agents map[string]AgentConfig `json:"agents"`
}

const defaultAgentName = "opencode"

// DefaultAgents returns the built-in agent definitions.
func DefaultAgents() map[string]AgentConfig {
	return map[string]AgentConfig{
		"opencode": {Command: "opencode"},
	}
}

// AgentCommand returns the shell command for the given agent shorthand.
// Falls back to using the name directly as a command if not in the registry.
func AgentCommand(name string) (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	if cfg.Agents != nil {
		if ac, ok := cfg.Agents[name]; ok {
			return ac.Command, nil
		}
	}
	// Check defaults.
	defaults := DefaultAgents()
	if ac, ok := defaults[name]; ok {
		return ac.Command, nil
	}
	// Treat the name itself as the command.
	return name, nil
}

var configPath string

func init() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		panic(fmt.Sprintf("unable to determine config directory: %v", err))
	}
	configPath = filepath.Join(configDir, "agents", "config.json")
}

// Load reads the config from disk. Returns an empty Config if the file
// does not exist yet.
func Load() (Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

// Save writes the config to disk.
func Save(cfg Config) error {
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	return os.WriteFile(configPath, data, 0o644)
}

// DefaultAgentName returns the configured default agent shorthand.
func DefaultAgentName() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	if cfg.DefaultAgent != "" {
		return cfg.DefaultAgent, nil
	}
	return defaultAgentName, nil
}

// Workspace returns the configured workspace path. If not set, it
// initializes it to cwd and persists it.
func Workspace() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	if cfg.Workspace != "" {
		return cfg.Workspace, nil
	}

	// First run — set workspace to cwd and seed defaults.
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	cfg.Workspace = cwd
	cfg.DefaultAgent = defaultAgentName
	cfg.Agents = DefaultAgents()
	if err := Save(cfg); err != nil {
		return "", fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("workspace set to %s\n", cwd)
	return cwd, nil
}
