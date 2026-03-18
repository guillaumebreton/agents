package coding

import (
	"slices"
	"testing"
)

func TestOpenCodeRegistered(t *testing.T) {
	ca, err := Get("opencode")
	if err != nil {
		t.Fatalf("expected opencode to be registered: %v", err)
	}
	if ca.Name() != "opencode" {
		t.Errorf("expected name 'opencode', got %q", ca.Name())
	}
	if ca.Command() != "opencode" {
		t.Errorf("expected command 'opencode', got %q", ca.Command())
	}
}

func TestGetUnknownAgent(t *testing.T) {
	_, err := Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestListContainsOpenCode(t *testing.T) {
	names := List()
	found := slices.Contains(names, "opencode")
	if !found {
		t.Error("expected List() to contain 'opencode'")
	}
}

func TestDefaultIsOpenCode(t *testing.T) {
	if Default != "opencode" {
		t.Errorf("expected default to be 'opencode', got %q", Default)
	}
}
