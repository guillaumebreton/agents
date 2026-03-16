// Package coding defines the interface for coding agent implementations
// and provides a registry for looking them up by name.
package coding

import "fmt"

// CodingAgent is the abstraction over different coding agent tools
// (opencode, claude, aider, etc.). Each implementation knows how to
// build the command to launch itself in a terminal.
type CodingAgent interface {
	// Name returns the shorthand identifier (e.g. "opencode").
	Name() string

	// Command returns the shell command to launch the agent.
	Command() string
}

var registry = map[string]CodingAgent{}

// Register adds a coding agent to the global registry.
func Register(a CodingAgent) {
	registry[a.Name()] = a
}

// Get returns a coding agent by name from the registry.
func Get(name string) (CodingAgent, error) {
	a, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown coding agent %q", name)
	}
	return a, nil
}

// List returns all registered coding agent names.
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// Default is the name of the default coding agent.
const Default = "opencode"
