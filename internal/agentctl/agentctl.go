// Package agentctl implements the core agent lifecycle operations:
// starting (worktree creation, tmux window, agent launch) and removal.
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

// StartWorktree starts an agent in a git worktree for the given repo and branch.
// The worktree is created under <workspace>/<repo>_worktrees/<branch> if it
// does not already exist. If an agent is already running for the same
// repo+branch, it is returned to focus instead of starting a duplicate.
// Branch is required.
func (c *Controller) StartWorktree(repoPath, branch, agentType string) error {
	if repoPath == "" {
		return fmt.Errorf("repo path is required")
	}
	if branch == "" {
		return fmt.Errorf("branch is required for worktree mode")
	}

	repoName := filepath.Base(repoPath)

	// Check for an existing agent on the same repo+branch.
	existing, _ := c.Store.List()
	for _, a := range existing {
		if a.Kind == agent.KindWorktree && a.RepoPath == repoPath && a.Branch == branch {
			return c.startExisting(a)
		}
	}

	// Resolve the worktree path.
	worktreeDir := filepath.Join(filepath.Dir(repoPath), repoName+"_worktrees")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		return fmt.Errorf("creating worktree directory: %w", err)
	}
	worktreePath := filepath.Join(worktreeDir, strings.ReplaceAll(branch, "/", "-"))

	// Create the worktree if it doesn't exist yet.
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		c.progress("Creating worktree…")
		if err := createWorktree(repoPath, worktreePath, branch); err != nil {
			return err
		}
	}

	a := agent.Agent{
		Name:        repoName + "/" + branch,
		Kind:        agent.KindWorktree,
		WorkdirPath: worktreePath,
		RepoPath:    repoPath,
		Branch:      branch,
		AgentType:   agentType,
	}
	return c.openWindowAndSave(a)
}

// StartMain starts an agent in the main checkout of a git repository.
// Only one agent is allowed per main checkout at a time. If branch is non-empty
// it is created from the current HEAD and checked out before launching.
func (c *Controller) StartMain(repoPath, branch, agentType string) error {
	if repoPath == "" {
		return fmt.Errorf("repo path is required")
	}

	// Only one agent is allowed on the main checkout.
	existing, _ := c.Store.List()
	for _, a := range existing {
		if a.Kind == agent.KindMain && a.RepoPath == repoPath {
			return c.startExisting(a)
		}
	}

	if branch != "" {
		c.progress("Checking out branch…")
		if err := checkoutBranch(repoPath, branch); err != nil {
			return fmt.Errorf("checking out branch %q: %w", branch, err)
		}
	}

	currentBranch := gitBranch(repoPath)
	repoName := filepath.Base(repoPath)
	name := repoName
	if currentBranch != "" {
		name = repoName + "/" + currentBranch
	}

	a := agent.Agent{
		Name:        name,
		Kind:        agent.KindMain,
		WorkdirPath: repoPath,
		RepoPath:    repoPath,
		Branch:      currentBranch,
		AgentType:   agentType,
	}
	return c.openWindowAndSave(a)
}

// Start starts an agent on a plain directory (no git management).
// If name is empty the base name of dir is used. If the same directory is
// already tracked and its window is alive, it returns an error.
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

	// Check for an existing agent on the same directory.
	existing, _ := c.Store.List()
	for _, a := range existing {
		if a.Kind == agent.KindPlain && a.WorkdirPath == abs {
			return c.startExisting(a)
		}
	}

	if name == "" {
		name = filepath.Base(abs)
		// Disambiguate if the basename is already taken by a different directory.
		if _, err := c.Store.Get(name); err == nil {
			name = name + "-" + abs[len(abs)-4:]
		}
	}

	a := agent.Agent{
		Name:        name,
		Kind:        agent.KindPlain,
		WorkdirPath: abs,
		AgentType:   agentType,
		Branch:      gitBranch(abs), // non-empty if the plain dir happens to be a git repo
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
		// Window is gone — clear stale IDs and reopen.
		a.WindowID = ""
		a.WindowIndex = ""
		a.WindowName = ""
		a.PanePID = ""
	}
	return c.openWindowAndSave(a)
}

// Reopen creates a new tmux window and relaunches the agent for an entry that
// is already in the store but whose window is no longer alive.
// It is a no-op if the agent is still running.
func (c *Controller) Reopen(a agent.Agent) error {
	return c.startExisting(a)
}

func (c *Controller) openWindowAndSave(a agent.Agent) error {
	exists, err := c.Mux.SessionExists(c.SessionName)
	if err != nil {
		return err
	}
	if !exists {
		if err := c.Mux.CreateSession(c.SessionName, a.WorkdirPath); err != nil {
			return err
		}
	}

	c.progress("Opening tmux window…")
	win, err := c.Mux.CreateWindow(c.SessionName, a.Name, a.WorkdirPath)
	if err != nil {
		return err
	}
	a.WindowID = win.ID
	a.WindowIndex = win.Index
	a.WindowName = win.Name
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

// Adopt updates an existing tracked agent when its hook calls register.
// It only modifies entries that are already in the store (agents started
// through this controller). Unrecognised window IDs are silently ignored so
// that agents launched outside of agents never pollute the store.
func (c *Controller) Adopt(windowID, panePID, windowIndex, windowName, workdir, agentType string) error {
	existing, _ := c.Store.List()
	for _, a := range existing {
		if a.WindowID == windowID {
			a.PanePID = panePID
			if windowIndex != "" {
				a.WindowIndex = windowIndex
			}
			if windowName != "" {
				a.WindowName = windowName
			}
			if agentType != "" {
				a.AgentType = agentType
			}
			return c.Store.Save(a)
		}
	}
	// Window not started by agents — silently ignore.
	return nil
}

// Remove tears down an agent according to its kind:
//   - KindWorktree: kill window + remove the git worktree
//   - KindMain:     kill window only (branch is preserved)
//   - KindPlain:    kill window only
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

	if a.Kind == agent.KindWorktree && a.WorkdirPath != "" && a.RepoPath != "" {
		if _, err := os.Stat(a.WorkdirPath); err == nil {
			gitCmd := exec.Command("git", "worktree", "remove", "--force", a.WorkdirPath)
			gitCmd.Dir = a.RepoPath
			if out, err := gitCmd.CombinedOutput(); err != nil {
				if rmErr := os.RemoveAll(a.WorkdirPath); rmErr != nil {
					return fmt.Errorf("removing worktree: %s", strings.TrimSpace(string(out)))
				}
			}
		}
	}

	return c.Store.Delete(a.Name)
}

// ── git helpers ──────────────────────────────────────────────────────────────

// gitBranch returns the current branch name in dir, or an empty string if dir
// is not a git repository or is in detached HEAD state.
func gitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if branch == "HEAD" {
		return ""
	}
	return branch
}

// branchExistsInRepo returns true if branch exists locally or on origin.
func branchExistsInRepo(repoPath, branch string) bool {
	check := func(ref string) bool {
		cmd := exec.Command("git", "rev-parse", "--verify", ref)
		cmd.Dir = repoPath
		cmd.Stderr = &strings.Builder{}
		return cmd.Run() == nil
	}
	localCh := make(chan bool, 1)
	remoteCh := make(chan bool, 1)
	go func() { localCh <- check(branch) }()
	go func() { remoteCh <- check("origin/" + branch) }()
	return <-localCh || <-remoteCh
}

// createWorktree adds a git worktree at worktreePath on branch.
// It creates the branch if it does not already exist.
// Stale registrations are pruned automatically before a single retry.
func createWorktree(repoPath, worktreePath, branch string) error {
	try := func() error {
		var cmd *exec.Cmd
		if branchExistsInRepo(repoPath, branch) {
			cmd = exec.Command("git", "worktree", "add", worktreePath, branch)
		} else {
			cmd = exec.Command("git", "worktree", "add", "-b", branch, worktreePath)
		}
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git worktree add: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := try(); err != nil {
		if strings.Contains(err.Error(), "already registered worktree") {
			// Prune stale registration and retry once.
			pruneCmd := exec.Command("git", "worktree", "prune")
			pruneCmd.Dir = repoPath
			pruneCmd.CombinedOutput()
			return try()
		}
		return err
	}
	return nil
}

// checkoutBranch creates branch from HEAD (if it doesn't exist) and checks
// it out in repoPath.
func checkoutBranch(repoPath, branch string) error {
	var cmd *exec.Cmd
	if branchExistsInRepo(repoPath, branch) {
		cmd = exec.Command("git", "checkout", branch)
	} else {
		cmd = exec.Command("git", "checkout", "-b", branch)
	}
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
