package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the application configuration.
type Config struct {
	// Workspace is the root directory where all repositories live.
	Workspace string `json:"workspace"`

	// DefaultAgent is the coding agent used when none is specified with --agent.
	// Defaults to "opencode" when empty.
	DefaultAgent string `json:"default_agent,omitempty"`
}

var configPath string

// current is the in-memory cache. Nil means not yet loaded.
var current *Config

func init() {
	// Allow tests (and power users) to override the config file location.
	if custom := os.Getenv("AGENTS_CONFIG_FILE"); custom != "" {
		configPath = custom
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("unable to determine home directory: %v", err))
	}
	configPath = filepath.Join(home, ".config", "agents", "config.json")
}

// Path returns the path to the config file.
func Path() string {
	return configPath
}

// Exists returns true if the config file already exists on disk.
func Exists() bool {
	_, err := os.Stat(configPath)
	return err == nil
}

// Get returns the cached config, loading from disk on the first call.
func Get() (Config, error) {
	if current != nil {
		return *current, nil
	}
	cfg, err := load()
	if err != nil {
		return Config{}, err
	}
	current = &cfg
	return cfg, nil
}

// Workspace returns the configured workspace path.
func Workspace() (string, error) {
	cfg, err := Get()
	if err != nil {
		return "", err
	}
	return cfg.Workspace, nil
}

// DefaultAgentName returns the configured default coding agent name.
// Falls back to "opencode" when no default has been set.
func DefaultAgentName() string {
	cfg, err := Get()
	if err != nil || cfg.DefaultAgent == "" {
		return "opencode"
	}
	return cfg.DefaultAgent
}

// Load reads the config from disk without using the in-memory cache.
// Useful for tests and one-shot reads that must not be influenced by
// a previously cached value.
func Load() (Config, error) {
	return load()
}

// Save writes cfg to disk and updates the in-memory cache.
func Save(cfg Config) error {
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return err
	}
	current = &cfg
	return nil
}

// SaveConfig persists the workspace and default agent together.
func SaveConfig(workspace, defaultAgent string) error {
	cfg, err := Get()
	if err != nil {
		return err
	}
	cfg.Workspace = workspace
	cfg.DefaultAgent = defaultAgent
	return Save(cfg)
}

// SaveWorkspace persists the given directory as the workspace.
func SaveWorkspace(dir string) error {
	cfg, err := Get()
	if err != nil {
		return err
	}
	cfg.Workspace = dir
	return Save(cfg)
}

// Init creates a default config file using cwd as the workspace.
// Returns an error if the config already exists.
func Init() error {
	if Exists() {
		return fmt.Errorf("config already exists at %s", configPath)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	return Save(Config{Workspace: cwd})
}

// load reads the config from disk. Returns an empty Config if the file
// does not exist yet.
func load() (Config, error) {
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
