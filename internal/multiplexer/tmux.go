package multiplexer

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"

	"github.com/charmbracelet/log"
)

// Tmux implements the Multiplexer interface using tmux.
type Tmux struct{}

// NewTmux returns a new Tmux multiplexer.
func NewTmux() *Tmux {
	return &Tmux{}
}

func (t *Tmux) SessionExists(session string) (bool, error) {
	log.Debug("checking if session exists", "session", session)
	cmd := exec.Command("tmux", "has-session", "-t", session)
	// Suppress stderr so tmux error messages don't leak to the terminal.
	cmd.Stdout = nil
	cmd.Stderr = &strings.Builder{}
	err := cmd.Run()
	if err != nil {
		// tmux returns exit code 1 when the session does not exist.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("checking session %q: %w", session, err)
	}
	return true, nil
}

func (t *Tmux) AttachSession(session string) error {
	log.Debug("attaching to session", "session", session)
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found: %w", err)
	}
	return syscall.Exec(tmuxPath, []string{"tmux", "attach-session", "-t", session}, os.Environ())
}

func (t *Tmux) CreateSession(session string, workdir string) error {
	log.Debug("creating session", "session", session, "workdir", workdir)
	cmd := exec.Command("tmux", "new-session", "-d", "-s", session, "-c", workdir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating session %q: %s: %w", session, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (t *Tmux) CreateWindow(session string, name string, workdir string) (Window, error) {
	// -P prints the window info, -F specifies the format.
	cmd := exec.Command("tmux", "new-window",
		"-t", session,
		"-n", name,
		"-c", workdir,
		"-P", "-F", "#{window_id}",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Window{}, fmt.Errorf("creating window %q in session %q: %s: %w", name, session, strings.TrimSpace(string(out)), err)
	}
	id := strings.TrimSpace(string(out))
	return Window{ID: id, Name: name}, nil
}

func (t *Tmux) WindowExists(windowID string) (bool, error) {
	cmd := exec.Command("tmux", "list-windows", "-a", "-F", "#{window_id}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("listing windows: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return slices.Contains(strings.Split(strings.TrimSpace(string(out)), "\n"), windowID), nil
}

func (t *Tmux) KillWindow(windowID string) error {
	cmd := exec.Command("tmux", "kill-window", "-t", windowID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("killing window %q: %s: %w", windowID, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (t *Tmux) ListWindows(session string) ([]Window, error) {
	cmd := exec.Command("tmux", "list-windows",
		"-t", session,
		"-F", "#{window_id}\t#{window_name}",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("listing windows in session %q: %s: %w", session, strings.TrimSpace(string(out)), err)
	}
	var windows []Window
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		windows = append(windows, Window{ID: parts[0], Name: parts[1]})
	}
	return windows, nil
}

func (t *Tmux) SendCommand(windowID string, command string) error {
	log.Debug("sending command", "window", windowID, "command", command)
	cmd := exec.Command("tmux", "send-keys", "-t", windowID, command, "Enter")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sending command to window %q: %s: %w", windowID, strings.TrimSpace(string(out)), err)
	}
	return nil
}
