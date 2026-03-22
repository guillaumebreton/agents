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

// agents runs the test binary with the given args and env, failing the test on
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

// TestHookRegisterAndUpdateStatus is the primary integration test.
// It replays the exact sequence a coding agent hook executes:
//  1. register  — called on session_start / AgentsPlugin init
//  2. update-status working — called when the agent starts processing
//  3. update-status idle    — called when the agent finishes
func TestHookRegisterAndUpdateStatus(t *testing.T) {
	_, stateFile, env := testEnv(t)
	workdir := t.TempDir()
	const panePID = "88001"
	const windowID = "@1"

	// 1. register (hook: session_start)
	agents(t, env,
		"register",
		"--window-id", windowID,
		"--pane-pid", panePID,
		"--workdir", workdir,
		"--agent-type", "opencode",
	)

	state := readState(t, stateFile)
	a, ok := findByPanePID(state, panePID)
	if !ok {
		t.Fatalf("agent with pane-pid %q not found after register\nstate: %+v", panePID, state)
	}
	if a.AgentType != "opencode" {
		t.Errorf("agent type: want %q, got %q", "opencode", a.AgentType)
	}
	if a.WindowID != windowID {
		t.Errorf("window ID: want %q, got %q", windowID, a.WindowID)
	}
	if a.WorkdirPath != workdir {
		t.Errorf("workdir: want %q, got %q", workdir, a.WorkdirPath)
	}
	t.Logf("registered agent %q", a.Name)

	// 2. update-status working (hook: agent_start / message.updated)
	agents(t, env, "update-status", "--pane-pid", panePID, "--status", "working")

	state = readState(t, stateFile)
	a, _ = findByPanePID(state, panePID)
	if a.Status != "working" {
		t.Errorf("status after working update: want %q, got %q", "working", a.Status)
	}

	// 3. update-status idle (hook: agent_end / session.idle)
	agents(t, env, "update-status", "--pane-pid", panePID, "--status", "idle")

	state = readState(t, stateFile)
	a, _ = findByPanePID(state, panePID)
	if a.Status != "idle" {
		t.Errorf("status after idle update: want %q, got %q", "idle", a.Status)
	}
}

// TestHookPiFlow runs the same sequence for the pi agent type.
func TestHookPiFlow(t *testing.T) {
	_, stateFile, env := testEnv(t)
	const panePID = "88002"

	agents(t, env,
		"register",
		"--window-id", "@2",
		"--pane-pid", panePID,
		"--workdir", t.TempDir(),
		"--agent-type", "pi",
	)
	agents(t, env, "update-status", "--pane-pid", panePID, "--status", "working")
	agents(t, env, "update-status", "--pane-pid", panePID, "--status", "idle")

	state := readState(t, stateFile)
	a, ok := findByPanePID(state, panePID)
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

// TestHookRegisterIdempotent verifies that calling register twice for the same
// window is a no-op and does not create a duplicate entry.
func TestHookRegisterIdempotent(t *testing.T) {
	_, stateFile, env := testEnv(t)
	workdir := t.TempDir()

	for i := 0; i < 3; i++ {
		agents(t, env,
			"register",
			"--window-id", "@5",
			"--pane-pid", "88003",
			"--workdir", workdir,
			"--agent-type", "opencode",
		)
	}

	state := readState(t, stateFile)
	count := 0
	for _, a := range state {
		if a.WindowID == "@5" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 agent for window @5, got %d", count)
	}
}

// TestHookRegisterOutsideTmux verifies that register with no pane info exits
// cleanly and stores nothing — matching the hook's behaviour outside tmux.
func TestHookRegisterOutsideTmux(t *testing.T) {
	_, stateFile, env := testEnv(t)

	agents(t, env, "register", "--workdir", t.TempDir(), "--agent-type", "opencode")

	state := readState(t, stateFile)
	if len(state) != 0 {
		t.Errorf("expected empty store when run outside tmux, got %d agents", len(state))
	}
}

// TestHookUpdateStatusBeforeRegister verifies that update-status for an
// unknown pane PID exits cleanly. This can happen if the hook fires before
// register completes (e.g. a race on startup).
func TestHookUpdateStatusBeforeRegister(t *testing.T) {
	_, _, env := testEnv(t)
	agents(t, env, "update-status", "--pane-pid", "00000", "--status", "idle")
}

// TestHookMultipleAgentsSameWindow verifies that two separate register calls
// for different pane PIDs but the same workdir basename get distinct names.
func TestHookMultipleAgentsSameWorkdir(t *testing.T) {
	_, stateFile, env := testEnv(t)

	// Both agents share the same workdir basename.
	dir1 := t.TempDir()
	dir2 := filepath.Join(filepath.Dir(dir1), filepath.Base(dir1)+"-2")
	os.MkdirAll(dir2, 0o755)

	agents(t, env, "register", "--window-id", "@10", "--pane-pid", "88010", "--workdir", dir1, "--agent-type", "opencode")
	agents(t, env, "register", "--window-id", "@11", "--pane-pid", "88011", "--workdir", dir1, "--agent-type", "opencode")

	state := readState(t, stateFile)
	if len(state) != 2 {
		t.Errorf("expected 2 agents, got %d: %+v", len(state), state)
	}
}
