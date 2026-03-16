package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"notb.re/agents/internal/agent"
)

// JSONStore persists agent state as a single JSON file on disk.
type JSONStore struct {
	path string
	mu   sync.Mutex
}

// state is the on-disk representation.
type state struct {
	Agents map[string]agent.Agent `json:"agents"`
}

// NewJSONStore creates a new JSONStore that reads/writes to
// ~/.config/agent/state.json, creating the directory if needed.
func NewJSONStore() (*JSONStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("unable to determine config directory: %w", err)
	}
	dir := filepath.Join(configDir, "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("unable to create config directory: %w", err)
	}
	return &JSONStore{path: filepath.Join(dir, "state.json")}, nil
}

func (s *JSONStore) load() (state, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state{Agents: make(map[string]agent.Agent)}, nil
		}
		return state{}, fmt.Errorf("reading state file: %w", err)
	}
	var st state
	if err := json.Unmarshal(data, &st); err != nil {
		return state{}, fmt.Errorf("parsing state file: %w", err)
	}
	if st.Agents == nil {
		st.Agents = make(map[string]agent.Agent)
	}
	return st, nil
}

func (s *JSONStore) save(st state) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}
	return nil
}

func (s *JSONStore) List() ([]agent.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load()
	if err != nil {
		return nil, err
	}
	agents := make([]agent.Agent, 0, len(st.Agents))
	for _, a := range st.Agents {
		agents = append(agents, a)
	}
	return agents, nil
}

func (s *JSONStore) Get(name string) (agent.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load()
	if err != nil {
		return agent.Agent{}, err
	}
	a, ok := st.Agents[name]
	if !ok {
		return agent.Agent{}, fmt.Errorf("agent %q not found", name)
	}
	return a, nil
}

func (s *JSONStore) GetByWorktree(worktree string) (agent.Agent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load()
	if err != nil {
		return agent.Agent{}, err
	}
	for _, a := range st.Agents {
		if a.WorktreePath == worktree {
			return a, nil
		}
	}
	return agent.Agent{}, fmt.Errorf("no agent found for worktree %q", worktree)
}

func (s *JSONStore) Save(a agent.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load()
	if err != nil {
		return err
	}
	st.Agents[a.Name] = a
	return s.save(st)
}

func (s *JSONStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := st.Agents[name]; !ok {
		return fmt.Errorf("agent %q not found", name)
	}
	delete(st.Agents, name)
	return s.save(st)
}
