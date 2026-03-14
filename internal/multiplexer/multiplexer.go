package multiplexer

// Window represents a single window/pane in a terminal multiplexer.
type Window struct {
	// ID is the multiplexer-specific identifier for the window.
	ID string

	// Name is the display name of the window.
	Name string
}

// Multiplexer is an abstraction over terminal multiplexers (tmux, zellij, etc.).
type Multiplexer interface {
	// SessionExists checks whether the named session already exists.
	SessionExists(session string) (bool, error)

	// CreateSession creates a new session with the given name and working directory.
	CreateSession(session string, workdir string) error

	// CreateWindow creates a new window inside the session and returns it.
	CreateWindow(session string, name string, workdir string) (Window, error)

	// KillWindow destroys a window by its ID.
	KillWindow(windowID string) error

	// ListWindows returns all windows in the given session.
	ListWindows(session string) ([]Window, error)

	// SendCommand sends a shell command to the given window.
	SendCommand(windowID string, command string) error
}
