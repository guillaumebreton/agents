package agent

// Agent represents a coding agent tied to a worktree and a
// terminal multiplexer window.
type Agent struct {
	// Name is a human-friendly identifier, typically the repo name.
	Name string `json:"name"`

	// WorktreePath is the absolute path to the git worktree.
	WorktreePath string `json:"worktree_path"`

	// SessionID is the opencode (or other agent) session identifier.
	SessionID string `json:"session_id,omitempty"`

	// WindowID is the multiplexer window/pane identifier.
	// Empty means no window is currently open.
	WindowID string `json:"window_id,omitempty"`
}
