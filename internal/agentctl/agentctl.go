// Package agentctl implements the core agent lifecycle operations:
// starting (tmux window creation, agent launch) and removal.
package agentctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"notb.re/agents/internal/agent"
	"notb.re/agents/internal/coding"
	"notb.re/agents/internal/multiplexer"
	"notb.re/agents/internal/store"
)

// Controller handles agent lifecycle using the provided store and multiplexer.
type Controller struct {
	Store       store.Store
	Mux         multiplexer.Multiplexer
	SessionName string
	// Progress is called with a status string at each step of agent creation.
	// It is optional — if nil, progress updates are silently dropped.
	Progress func(string)
}

func (c *Controller) progress(msg string) {
	if c.Progress != nil {
		c.Progress(msg)
	}
}

// Start starts an agent on dir, opening a tmux window if needed.
// If name is empty, the base name of dir is used as the agent name.
// Returns an error if an agent with that name is already running.
func (c *Controller) Start(name, dir, agentType string) error {
	if dir == "" {
		return fmt.Errorf("dir is required")
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving directory: %w", err)
	}
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		return fmt.Errorf("directory %q does not exist", abs)
	}
	if name == "" {
		name = filepath.Base(abs)
	}

	// Re-attach to an existing tracked agent if one is found.
	a, err := c.Store.Get(name)
	if err == nil {
		return c.startExisting(a)
	}

	c.progress("Detecting repository…")
	a = agent.Agent{
		Name:        name,
		WorkdirPath: abs,
		AgentType:   agentType,
		IsGitRepo:   isGitRepo(abs),
	}
	return c.openWindowAndSave(a)
}

func (c *Controller) startExisting(a agent.Agent) error {
	if a.WindowID != "" {
		alive, err := c.Mux.WindowExists(a.WindowID)
		if err != nil {
			return err
		}
		if alive {
			return fmt.Errorf("agent %q is already running", a.Name)
		}
		a.WindowID = ""
	}
	return c.openWindowAndSave(a)
}

func (c *Controller) openWindowAndSave(a agent.Agent) error {
	exists, err := c.Mux.SessionExists(c.SessionName)
	if err != nil {
		return err
	}
	if !exists {
		workspace := a.WorkdirPath
		if err := c.Mux.CreateSession(c.SessionName, workspace); err != nil {
			return err
		}
	}

	c.progress("Opening tmux window…")
	win, err := c.Mux.CreateWindow(c.SessionName, a.Name, a.WorkdirPath)
	if err != nil {
		return err
	}
	a.WindowID = win.ID
	a.PanePID = win.PanePID

	c.progress("Launching agent…")
	ca, err := coding.Get(a.AgentType)
	if err != nil {
		return fmt.Errorf("resolving agent type: %w", err)
	}
	if err := c.Mux.SendCommand(win.ID, ca.Command()); err != nil {
		return fmt.Errorf("launching %s: %w", a.AgentType, err)
	}

	return c.Store.Save(a)
}

// Adopt registers a pane that is already running an agent but was not started
// through the controller. It is safe to call repeatedly for the same pane —
// if the window ID is already tracked the call is a no-op.
func (c *Controller) Adopt(windowID, panePID, workdir, agentType string) error {
	// Skip if this window is already tracked under any name.
	existing, _ := c.Store.List()
	for _, a := range existing {
		if a.WindowID == windowID {
			return nil
		}
	}

	// Derive a name from the workdir basename, disambiguating with the
	// window ID if another agent already owns that name.
	name := filepath.Base(workdir)
	if _, err := c.Store.Get(name); err == nil {
		name = name + "-" + windowID
	}

	a := agent.Agent{
		Name:        name,
		WorkdirPath: workdir,
		AgentType:   agentType,
		IsGitRepo:   isGitRepo(workdir),
		WindowID:    windowID,
		PanePID:     panePID,
	}
	return c.Store.Save(a)
}

// Remove kills the tmux window and deletes the agent from the store.
// The working directory itself is never touched.
func (c *Controller) Remove(a agent.Agent) error {
	if a.WindowID != "" {
		alive, err := c.Mux.WindowExists(a.WindowID)
		if err != nil {
			return err
		}
		if alive {
			if err := c.Mux.KillWindow(a.WindowID); err != nil {
				return fmt.Errorf("killing window: %w", err)
			}
		}
	}

	return c.Store.Delete(a.Name)
}

// isGitRepo reports whether dir is inside a git repository by running
// "git rev-parse --is-inside-work-tree".
func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}
