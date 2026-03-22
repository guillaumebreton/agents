package multiplexer

// Window represents a single window/pane in a terminal multiplexer.
type Window struct {
	// ID is the multiplexer-specific identifier for the window (e.g. "@1").
	ID string

	// Index is the numeric index shown in the tmux status bar (e.g. "1").
	Index string

	// Name is the display name of the window.
	Name string

	// PanePID is the process ID of the shell running in the pane.
	PanePID string
}

// PaneDetails holds the tmux metadata resolved from a pane ID.
type PaneDetails struct {
	WindowID    string
	WindowIndex string
	WindowName  string
	PanePID     string
}

// Multiplexer is an abstraction over terminal multiplexers (tmux, zellij, etc.).
type Multiplexer interface {
	// SessionExists checks whether the named session already exists.
	SessionExists(session string) (bool, error)

	// CreateSession creates a new session with the given name and working directory.
	CreateSession(session string, workdir string) error

	// AttachSession attaches to an existing session. This replaces the
	// current process (exec) so it does not return on success.
	AttachSession(session string) error

	// CreateWindow creates a new window inside the session and returns it.
	CreateWindow(session string, name string, workdir string) (Window, error)

	// WindowExists checks whether the given window is still alive.
	WindowExists(windowID string) (bool, error)

	// KillWindow destroys a window by its ID.
	KillWindow(windowID string) error

	// ListWindows returns all windows in the given session.
	ListWindows(session string) ([]Window, error)

	// WindowIDForPane returns the window ID that contains the given pane ID.
	WindowIDForPane(paneID string) (string, error)

	// SendCommand sends a shell command to the given window.
	SendCommand(windowID string, command string) error

	// SelectWindow switches focus to the given window.
	SelectWindow(windowID string) error

	// PaneInfo resolves a tmux pane ID (e.g. "%3") to window metadata.
	// Used by the register command.
	PaneInfo(paneID string) (PaneDetails, error)
}
