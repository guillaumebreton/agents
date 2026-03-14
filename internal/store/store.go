package store

import "notb.re/agent/internal/agent"

// Store is an abstraction for persisting agent state.
type Store interface {
	// List returns all tracked agents.
	List() ([]agent.Agent, error)

	// Get returns a single agent by name.
	Get(name string) (agent.Agent, error)

	// Save persists an agent (creates or updates).
	Save(a agent.Agent) error

	// Delete removes an agent from the store.
	Delete(name string) error
}
