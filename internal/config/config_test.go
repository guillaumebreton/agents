package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempConfig(t *testing.T) func() {
	t.Helper()
	originalPath := configPath
	originalCache := current
	current = nil
	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.json")
	return func() {
		configPath = originalPath
		current = originalCache
	}
}

func TestLoadMissingFile(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Workspace != "" {
		t.Errorf("expected empty workspace, got %q", cfg.Workspace)
	}
}

func TestSaveAndLoad(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	cfg := Config{Workspace: "/some/path"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Workspace != "/some/path" {
		t.Errorf("expected workspace '/some/path', got %q", loaded.Workspace)
	}
}

func TestExistsAndPath(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	if Exists() {
		t.Error("expected config to not exist")
	}

	Save(Config{Workspace: "/test"})

	if !Exists() {
		t.Error("expected config to exist after save")
	}
}

func TestInit(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	if err := Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cwd, _ := os.Getwd()
	if cfg.Workspace != cwd {
		t.Errorf("expected workspace %q, got %q", cwd, cfg.Workspace)
	}
}

func TestDefaultAgentName(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	// Falls back to "opencode" when not configured.
	if got := DefaultAgentName(); got != "opencode" {
		t.Errorf("expected default agent 'opencode', got %q", got)
	}

	// Returns the configured value when set.
	if err := Save(Config{Workspace: "/ws", DefaultAgent: "pi"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if got := DefaultAgentName(); got != "pi" {
		t.Errorf("expected default agent 'pi', got %q", got)
	}
}

func TestInitAlreadyExists(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	Save(Config{Workspace: "/existing"})

	err := Init()
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
}

func TestWorkspace(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	// No config file yet — workspace should be empty, not an error.
	ws, err := Workspace()
	if err != nil {
		t.Fatalf("Workspace (no config): %v", err)
	}
	if ws != "" {
		t.Errorf("expected empty workspace before init, got %q", ws)
	}

	// After saving, Workspace should return the persisted value.
	if err := Save(Config{Workspace: "/my/workspace"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	ws2, err := Workspace()
	if err != nil {
		t.Fatalf("Workspace (after save): %v", err)
	}
	if ws2 != "/my/workspace" {
		t.Errorf("expected workspace '/my/workspace', got %q", ws2)
	}
}
