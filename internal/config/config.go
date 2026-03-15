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

	// First run — set workspace to cwd.
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	cfg.Workspace = cwd
	if err := Save(cfg); err != nil {
		return "", fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("workspace set to %s\n", cwd)
	return cwd, nil
}
