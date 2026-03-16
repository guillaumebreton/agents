package agent

// Status represents the reported state of a coding agent.
type Status string

const (
	StatusIdle    Status = "idle"
	StatusWorking Status = "working"
	StatusWaiting Status = "waiting"
	StatusExited  Status = "exited"
)

// ValidStatus returns true if the given string is a valid status.
func ValidStatus(s string) bool {
	switch Status(s) {
	case StatusIdle, StatusWorking, StatusWaiting, StatusExited:
		return true
	}
	return false
}

// Agent represents a coding agent tied to a worktree and a
// terminal multiplexer window.
type Agent struct {
	// Name is a human-friendly identifier, typically the repo name.
	Name string `json:"name"`

	// WorktreePath is the absolute path to the git worktree.
	WorktreePath string `json:"worktree_path"`

	// AgentType is the shorthand name of the coding agent (e.g. "opencode").
	AgentType string `json:"agent_type"`

	// Status is the last reported status of the coding agent.
	Status Status `json:"status,omitempty"`

	// SessionID is the coding agent session identifier.
	SessionID string `json:"session_id,omitempty"`

	// WindowID is the multiplexer window/pane identifier.
	// Empty means no window is currently open.
	WindowID string `json:"window_id,omitempty"`
}
