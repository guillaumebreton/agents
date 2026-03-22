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

// Kind describes how the agent's working directory was set up.
type Kind string

const (
	// KindWorktree means the agent is running in a git worktree created by
	// agents. Removal will run "git worktree remove" before closing the window.
	KindWorktree Kind = "worktree"

	// KindMain means the agent is running in the main checkout of a git
	// repository. Removal closes the window but leaves the repo untouched.
	KindMain Kind = "main"

	// KindPlain means the agent is running in a plain directory with no git
	// worktree management. Removal only closes the window.
	KindPlain Kind = "plain"
)

// Agent represents a coding agent tied to a working directory and a
// terminal multiplexer window.
type Agent struct {
	// Name is a human-friendly identifier for the agent.
	Name string `json:"name"`

	// Kind describes how the working directory was set up.
	Kind Kind `json:"kind,omitempty"`

	// WorkdirPath is the absolute path to the directory the agent works in.
	// The JSON key is kept as "worktree_path" for backward compatibility with
	// existing state files.
	WorkdirPath string `json:"worktree_path"`

	// RepoPath is the absolute path to the git repository root.
	// For KindWorktree it is the main repo (parent of the worktree).
	// For KindMain it equals WorkdirPath.
	// Empty for KindPlain.
	RepoPath string `json:"repo_path,omitempty"`

	// AgentType is the shorthand name of the coding agent (e.g. "opencode").
	AgentType string `json:"agent_type"`

	// Branch is the git branch the agent is working on.
	// Empty when the directory is not a git repository.
	Branch string `json:"branch,omitempty"`

	// WindowIndex is the numeric index of the tmux window (e.g. "1", "2").
	// This is the number shown in the tmux status bar.
	WindowIndex string `json:"window_index,omitempty"`

	// WindowName is the display name of the tmux window that hosts this agent.
	WindowName string `json:"window_name,omitempty"`

	// Status is the last reported status of the coding agent.
	Status Status `json:"status,omitempty"`

	// SessionID is the coding agent session identifier.
	SessionID string `json:"session_id,omitempty"`

	// WindowID is the multiplexer window/pane identifier.
	// Empty means no window is currently open.
	WindowID string `json:"window_id,omitempty"`

	// PanePID is the process ID of the shell running in the pane.
	PanePID string `json:"pane_pid,omitempty"`
}
