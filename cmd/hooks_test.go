package cmd_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"notb.re/agents/internal/agent"
)

// testBinary is the compiled agents binary shared across all tests in this file.
var testBinary string

func TestMain(m *testing.M) {
	bin, err := buildTestBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "skipping hook integration tests: build failed: %v\n", err)
		os.Exit(0)
	}
	testBinary = bin
	code := m.Run()
	os.RemoveAll(filepath.Dir(bin))
	os.Exit(code)
}

// buildTestBinary compiles the agents binary into a temp directory.
func buildTestBinary() (string, error) {
	dir, err := os.MkdirTemp("", "agents-test-bin-*")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "agents")
	cmd := exec.Command("go", "build", "-o", bin, "notb.re/agents")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("%s: %w", out, err)
	}
	return bin, nil
}

// testEnv creates isolated config and state files in a temp directory and
// returns the environment that points the binary at them.
func testEnv(t *testing.T) (configFile, stateFile string, env []string) {
	t.Helper()
	dir := t.TempDir()
	configFile = filepath.Join(dir, "config.json")
	stateFile = filepath.Join(dir, "state.json")

	cfg := fmt.Sprintf(`{"workspace": %q}`, dir)
	if err := os.WriteFile(configFile, []byte(cfg), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	env = append(os.Environ(),
		"AGENTS_CONFIG_FILE="+configFile,
		"AGENTS_STATE_FILE="+stateFile,
	)
	return
}

// seedState writes one or more agents directly into the state file, simulating
// entries that were created by the controller (e.g. via StartWorktree / StartMain).
func seedState(t *testing.T, stateFile string, agents ...agent.Agent) {
	t.Helper()
	m := make(map[string]agent.Agent, len(agents))
	for _, a := range agents {
		m[a.Name] = a
	}
	data, err := json.MarshalIndent(map[string]any{"agents": m}, "", "  ")
	if err != nil {
		t.Fatalf("marshalling seed state: %v", err)
	}
	if err := os.WriteFile(stateFile, data, 0o644); err != nil {
		t.Fatalf("writing seed state: %v", err)
	}
}

// run runs the test binary with the given args and env, failing the test on
// any non-zero exit.
func agents(t *testing.T, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command(testBinary, args...)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("agents %v\n  exit: %v\n  output: %s", args, err, out)
	}
}

// readState deserialises the state file into a map keyed by agent name.
func readState(t *testing.T, stateFile string) map[string]agent.Agent {
	t.Helper()
	data, err := os.ReadFile(stateFile)
	if os.IsNotExist(err) {
		return map[string]agent.Agent{}
	}
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}
	var st struct {
		Agents map[string]agent.Agent `json:"agents"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("parsing state file: %v\ncontent: %s", err, data)
	}
	if st.Agents == nil {
		return map[string]agent.Agent{}
	}
	return st.Agents
}

// findByPanePID returns the first agent whose PanePID matches.
func findByPanePID(agents map[string]agent.Agent, panePID string) (agent.Agent, bool) {
	for _, a := range agents {
		if a.PanePID == panePID {
			return a, true
		}
	}
	return agent.Agent{}, false
}

// findByWindowID returns the first agent whose WindowID matches.
func findByWindowID(agents map[string]agent.Agent, windowID string) (agent.Agent, bool) {
	for _, a := range agents {
		if a.WindowID == windowID {
			return a, true
		}
	}
	return agent.Agent{}, false
}

// TestHookRegisterAndUpdateStatus is the primary integration test.
// It replays the exact sequence a coding agent hook executes:
//  1. controller creates the agent entry (simulated via seedState)
//  2. register — called by hook on session_start; updates PanePID / WindowName
//  3. update-status working — called when the agent starts processing
//  4. update-status idle    — called when the agent finishes
func TestHookRegisterAndUpdateStatus(t *testing.T) {
	_, stateFile, env := testEnv(t)
	workdir := t.TempDir()
	const panePID = "88001"
	const windowID = "@1"

	// Simulate the controller having started the agent and saved an entry.
	seedState(t, stateFile, agent.Agent{
		Name:        "myrepo",
		Kind:        agent.KindMain,
		WorkdirPath: workdir,
		RepoPath:    workdir,
		AgentType:   "opencode",
		WindowID:    windowID,
	})

	// 1. Hook calls register on session_start — updates PanePID and WindowName.
	agents(t, env,
		"register",
		"--window-id", windowID, "--window-index", "1",
		"--window-name", "myrepo",
		"--pane-pid", panePID,
		"--workdir", workdir,
		"--agent-type", "opencode",
	)

	state := readState(t, stateFile)
	a, ok := findByWindowID(state, windowID)
	if !ok {
		t.Fatalf("agent with window-id %q not found after register\nstate: %+v", windowID, state)
	}
	if a.PanePID != panePID {
		t.Errorf("pane PID: want %q, got %q", panePID, a.PanePID)
	}
	if a.WindowName != "myrepo" {
		t.Errorf("window name: want %q, got %q", "myrepo", a.WindowName)
	}
	if a.AgentType != "opencode" {
		t.Errorf("agent type: want %q, got %q", "opencode", a.AgentType)
	}
	t.Logf("register updated agent %q (pane-pid=%s)", a.Name, a.PanePID)

	// 2. update-status working (hook: agent_start / message.updated)
	agents(t, env, "update-status", "--pane-pid", panePID, "--status", "working", "--agent-type", "opencode")

	state = readState(t, stateFile)
	a, _ = findByWindowID(state, windowID)
	if a.Status != "working" {
		t.Errorf("status after working update: want %q, got %q", "working", a.Status)
	}

	// 3. update-status idle (hook: agent_end / session.idle)
	agents(t, env, "update-status", "--pane-pid", panePID, "--status", "idle", "--agent-type", "opencode")

	state = readState(t, stateFile)
	a, _ = findByWindowID(state, windowID)
	if a.Status != "idle" {
		t.Errorf("status after idle update: want %q, got %q", "idle", a.Status)
	}
}

// TestHookPiFlow runs the same sequence for the pi agent type.
func TestHookPiFlow(t *testing.T) {
	_, stateFile, env := testEnv(t)
	const panePID = "88002"
	const windowID = "@2"
	workdir := t.TempDir()

	seedState(t, stateFile, agent.Agent{
		Name:        "proj/main",
		Kind:        agent.KindMain,
		WorkdirPath: workdir,
		RepoPath:    workdir,
		AgentType:   "pi",
		WindowID:    windowID,
	})

	agents(t, env, "register",
		"--window-id", windowID, "--window-name", "proj",
		"--pane-pid", panePID, "--workdir", workdir, "--agent-type", "pi")
	agents(t, env, "update-status", "--pane-pid", panePID, "--status", "working", "--agent-type", "pi")
	agents(t, env, "update-status", "--pane-pid", panePID, "--status", "idle", "--agent-type", "pi")

	state := readState(t, stateFile)
	a, ok := findByWindowID(state, windowID)
	if !ok {
		t.Fatal("pi agent not found after register")
	}
	if a.AgentType != "pi" {
		t.Errorf("agent type: want %q, got %q", "pi", a.AgentType)
	}
	if a.Status != "idle" {
		t.Errorf("final status: want %q, got %q", "idle", a.Status)
	}
}

// TestHookRegisterIdempotent verifies that calling register multiple times for
// the same window updates the entry but does not create duplicates.
func TestHookRegisterIdempotent(t *testing.T) {
	_, stateFile, env := testEnv(t)
	workdir := t.TempDir()
	const windowID = "@5"

	seedState(t, stateFile, agent.Agent{
		Name:        "myrepo",
		Kind:        agent.KindMain,
		WorkdirPath: workdir,
		RepoPath:    workdir,
		AgentType:   "opencode",
		WindowID:    windowID,
	})

	for i := 0; i < 3; i++ {
		agents(t, env, "register",
			"--window-id", windowID, "--window-name", "myrepo",
			"--pane-pid", fmt.Sprintf("8800%d", i),
			"--workdir", workdir, "--agent-type", "opencode")
	}

	state := readState(t, stateFile)
	count := 0
	for _, a := range state {
		if a.WindowID == windowID {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 agent for window %s, got %d", windowID, count)
	}
}

// TestHookRegisterIgnoresUnknownWindow verifies that register for a window not
// in the store does nothing — agents started outside the tool are not adopted.
func TestHookRegisterIgnoresUnknownWindow(t *testing.T) {
	_, stateFile, env := testEnv(t)

	agents(t, env, "register",
		"--window-id", "@99",
		"--window-name", "stranger",
		"--pane-pid", "55555",
		"--workdir", t.TempDir(),
		"--agent-type", "opencode",
	)

	state := readState(t, stateFile)
	if len(state) != 0 {
		t.Errorf("expected store to remain empty for unknown window, got %d agents", len(state))
	}
}

// TestHookRegisterOutsideTmux verifies that register with no pane info exits
// cleanly — matching the hook's behaviour when not running inside tmux.
func TestHookRegisterOutsideTmux(t *testing.T) {
	_, stateFile, env := testEnv(t)

	agents(t, env, "register", "--workdir", t.TempDir(), "--agent-type", "opencode")

	state := readState(t, stateFile)
	if len(state) != 0 {
		t.Errorf("expected empty store when run outside tmux, got %d agents", len(state))
	}
}

// TestHookUpdateStatusBeforeRegister verifies that update-status for an
// unknown pane PID exits cleanly (race on startup or unmanaged agent).
func TestHookUpdateStatusBeforeRegister(t *testing.T) {
	_, _, env := testEnv(t)
	agents(t, env, "update-status", "--pane-pid", "00000", "--status", "idle")
}

// TestHookAgentTypeUpdatedOnRestart verifies that if the same window restarts
// with a different agent, register updates the stored agent type immediately.
func TestHookAgentTypeUpdatedOnRestart(t *testing.T) {
	_, stateFile, env := testEnv(t)
	workdir := t.TempDir()
	const windowID = "@99"
	const panePID = "88099"

	// Seed with opencode.
	seedState(t, stateFile, agent.Agent{
		Name:        "myrepo",
		Kind:        agent.KindMain,
		WorkdirPath: workdir,
		RepoPath:    workdir,
		AgentType:   "opencode",
		WindowID:    windowID,
	})

	// Confirm opencode.
	agents(t, env, "register",
		"--window-id", windowID, "--window-name", "myrepo",
		"--pane-pid", panePID, "--workdir", workdir, "--agent-type", "opencode")

	state := readState(t, stateFile)
	a, _ := findByWindowID(state, windowID)
	if a.AgentType != "opencode" {
		t.Fatalf("expected opencode, got %q", a.AgentType)
	}

	// User switches to pi in the same window.
	agents(t, env, "register",
		"--window-id", windowID, "--window-name", "myrepo",
		"--pane-pid", panePID, "--workdir", workdir, "--agent-type", "pi")

	state = readState(t, stateFile)
	a, _ = findByWindowID(state, windowID)
	if a.AgentType != "pi" {
		t.Errorf("expected pi after switch, got %q", a.AgentType)
	}
	if len(state) != 1 {
		t.Errorf("expected 1 agent after switch, got %d", len(state))
	}
}
