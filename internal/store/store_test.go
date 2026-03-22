package store

import (
	"os"
	"path/filepath"
	"testing"

	"notb.re/agents/internal/agent"
)

func testStore(t *testing.T) *JSONStore {
	t.Helper()
	dir := t.TempDir()
	return &JSONStore{path: filepath.Join(dir, "state.json")}
}

func TestSaveAndGet(t *testing.T) {
	s := testStore(t)

	a := agent.Agent{Name: "myrepo", WorkdirPath: "/tmp/myrepo", AgentType: "opencode"}
	if err := s.Save(a); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Get("myrepo")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "myrepo" {
		t.Errorf("expected name 'myrepo', got %q", got.Name)
	}
	if got.WorkdirPath != "/tmp/myrepo" {
		t.Errorf("expected workdir '/tmp/myrepo', got %q", got.WorkdirPath)
	}
	if got.AgentType != "opencode" {
		t.Errorf("expected agent type 'opencode', got %q", got.AgentType)
	}
}

func TestGetNotFound(t *testing.T) {
	s := testStore(t)

	_, err := s.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestList(t *testing.T) {
	s := testStore(t)

	// Empty store.
	agents, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}

	// Add two agents.
	s.Save(agent.Agent{Name: "a", AgentType: "opencode"})
	s.Save(agent.Agent{Name: "b", AgentType: "opencode"})

	agents, err = s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestDelete(t *testing.T) {
	s := testStore(t)

	s.Save(agent.Agent{Name: "myrepo", AgentType: "opencode"})

	if err := s.Delete("myrepo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.Get("myrepo")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	s := testStore(t)

	err := s.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestSaveUpdate(t *testing.T) {
	s := testStore(t)

	s.Save(agent.Agent{Name: "myrepo", WorkdirPath: "/old"})
	s.Save(agent.Agent{Name: "myrepo", WorkdirPath: "/new"})

	got, _ := s.Get("myrepo")
	if got.WorkdirPath != "/new" {
		t.Errorf("expected updated workdir '/new', got %q", got.WorkdirPath)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s1 := &JSONStore{path: path}
	s1.Save(agent.Agent{Name: "persisted", AgentType: "opencode"})

	// New store instance reading the same file.
	s2 := &JSONStore{path: path}
	got, err := s2.Get("persisted")
	if err != nil {
		t.Fatalf("Get from new store: %v", err)
	}
	if got.Name != "persisted" {
		t.Errorf("expected 'persisted', got %q", got.Name)
	}
}

func TestLoadMissingFile(t *testing.T) {
	s := &JSONStore{path: "/nonexistent/path/state.json"}
	agents, err := s.List()
	if err != nil {
		t.Fatalf("List on missing file: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(agents))
	}
}

func TestLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	os.WriteFile(path, []byte("not json"), 0o644)

	s := &JSONStore{path: path}
	_, err := s.List()
	if err == nil {
		t.Fatal("expected error for corrupt file")
	}
}
