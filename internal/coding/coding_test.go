package coding

import (
	"slices"
	"strings"
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

func TestPiRegistered(t *testing.T) {
	ca, err := Get("pi")
	if err != nil {
		t.Fatalf("expected pi to be registered: %v", err)
	}
	if ca.Name() != "pi" {
		t.Errorf("expected name 'pi', got %q", ca.Name())
	}
	if ca.Command() != "pi" {
		t.Errorf("expected command 'pi', got %q", ca.Command())
	}
}

func TestPiHookPath(t *testing.T) {
	ca, err := Get("pi")
	if err != nil {
		t.Fatalf("expected pi to be registered: %v", err)
	}
	hookPath := ca.HookPath()
	if hookPath == "" {
		t.Fatal("expected non-empty hook path for pi")
	}
	if !strings.HasSuffix(hookPath, "agents-hook.ts") {
		t.Errorf("expected hook path to end with agents-hook.ts, got %q", hookPath)
	}
}

func TestPiHookContent(t *testing.T) {
	ca, err := Get("pi")
	if err != nil {
		t.Fatalf("expected pi to be registered: %v", err)
	}
	hook := ca.Hook("/usr/local/bin/agents")
	if hook == "" {
		t.Fatal("expected non-empty hook content for pi")
	}
	// Must contain the version marker so init can detect stale hooks.
	if !strings.Contains(hook, "Version: "+HookVersion) {
		t.Errorf("hook content missing version marker %q", HookVersion)
	}
	// Must reference the binary path.
	if !strings.Contains(hook, "/usr/local/bin/agents") {
		t.Error("hook content missing agents binary path")
	}
	// Must target pi's extension API.
	if !strings.Contains(hook, `@mariozechner/pi-coding-agent`) {
		t.Error("hook content missing pi extension API import")
	}
	// Must call register on startup.
	if !strings.Contains(hook, `"register"`) {
		t.Error("pi hook missing register call")
	}
	// Must pass agent-type in update-status so the watcher stays in sync.
	if !strings.Contains(hook, "agent-type") {
		t.Error("pi hook missing --agent-type in update-status call")
	}
	// Must pass TMUX_PANE for window resolution.
	if !strings.Contains(hook, "TMUX_PANE") {
		t.Error("pi hook missing TMUX_PANE reference")
	}
	// Must pass workdir (cwd) to register.
	if !strings.Contains(hook, "workdir") {
		t.Error("pi hook missing workdir argument to register")
	}
}

func TestOpenCodeHookContent(t *testing.T) {
	ca, err := Get("opencode")
	if err != nil {
		t.Fatalf("expected opencode to be registered: %v", err)
	}
	hook := ca.Hook("/usr/local/bin/agents")
	if hook == "" {
		t.Fatal("expected non-empty hook content for opencode")
	}
	if !strings.Contains(hook, "Version: "+HookVersion) {
		t.Errorf("hook content missing version marker %q", HookVersion)
	}
	if !strings.Contains(hook, "/usr/local/bin/agents") {
		t.Error("opencode hook missing agents binary path")
	}
	// Must call register on startup.
	if !strings.Contains(hook, "register") {
		t.Error("opencode hook missing register call")
	}
	// Must pass TMUX_PANE for window resolution.
	if !strings.Contains(hook, "TMUX_PANE") {
		t.Error("opencode hook missing TMUX_PANE reference")
	}
	// Must pass workdir (cwd) to register.
	if !strings.Contains(hook, "workdir") {
		t.Error("opencode hook missing workdir argument to register")
	}
	// Must call update-status for status reporting.
	if !strings.Contains(hook, "update-status") {
		t.Error("opencode hook missing update-status call")
	}
	// Must pass agent-type in update-status so the watcher stays in sync.
	if !strings.Contains(hook, "agent-type") {
		t.Error("opencode hook missing --agent-type in update-status call")
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

func TestListContainsPi(t *testing.T) {
	names := List()
	found := slices.Contains(names, "pi")
	if !found {
		t.Error("expected List() to contain 'pi'")
	}
}

func TestDefaultIsOpenCode(t *testing.T) {
	if Default != "opencode" {
		t.Errorf("expected default to be 'opencode', got %q", Default)
	}
}
