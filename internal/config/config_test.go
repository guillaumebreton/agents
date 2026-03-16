package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempConfig(t *testing.T) func() {
	t.Helper()
	original := configPath
	dir := t.TempDir()
	configPath = filepath.Join(dir, "config.json")
	return func() {
		configPath = original
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

func TestInitAlreadyExists(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	Save(Config{Workspace: "/existing"})

	err := Init()
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
}

func TestWorkspaceAutoInit(t *testing.T) {
	cleanup := withTempConfig(t)
	defer cleanup()

	ws, err := Workspace()
	if err != nil {
		t.Fatalf("Workspace: %v", err)
	}
	cwd, _ := os.Getwd()
	if ws != cwd {
		t.Errorf("expected workspace %q, got %q", cwd, ws)
	}

	// Second call should return persisted value.
	ws2, err := Workspace()
	if err != nil {
		t.Fatalf("Workspace (second call): %v", err)
	}
	if ws2 != cwd {
		t.Errorf("expected workspace %q, got %q", cwd, ws2)
	}
}
