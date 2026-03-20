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
	"notb.re/agents/internal/config"
	"notb.re/agents/internal/multiplexer"
	"notb.re/agents/internal/store"
)

// ErrStaleWorktree is returned by Start when git reports a stale worktree
// registration. The caller can prompt the user and retry with ForceStart.
type ErrStaleWorktree struct {
	Repo         string
	Branch       string
	AgentType    string
	WorktreePath string
	RepoPath     string
}

func (e *ErrStaleWorktree) Error() string {
	return fmt.Sprintf("stale worktree registration found for %q — prune and recreate?", e.WorktreePath)
}

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

// AgentName returns the canonical identifier for an agent: repo/branch.
func AgentName(repo, branch string) string {
	return repo + "/" + branch
}

// Start starts an agent for the given repo and branch, creating a worktree and
// tmux window if needed. If the agent is already tracked and its window is alive,
// it returns an error.
func (c *Controller) Start(repo, branch, agentType string) error {
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	if branch == "" {
		return fmt.Errorf("branch is required")
	}

	// Resolve the full branch name (with prefix) before any lookup or creation,
	// so the store key is always consistent.
	prefix := config.BranchPrefix()
	fullBranch := branch
	if prefix != "" && !strings.HasPrefix(branch, prefix) {
		fullBranch = prefix + branch
	}

	name := AgentName(repo, fullBranch)
	a, err := c.Store.Get(name)
	if err != nil {
		return c.startNew(repo, fullBranch, agentType)
	}
	return c.startExisting(a)
}

func (c *Controller) startNew(repo, branch, agentType string) error {
	workspace, err := config.Workspace()
	if err != nil {
		return err
	}

	// branch is already fully qualified (prefix applied by Start).
	fullBranch := branch

	// 1. Check the repo exists.
	repoPath := filepath.Join(workspace, repo)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository %q not found in workspace", repo)
	}

	// 2. Check whether the branch already exists locally or remotely.
	c.progress("Checking branch…")
	branchExists := branchExistsInRepo(repoPath, fullBranch)

	// 3. Resolve the worktree path.
	worktreeDir := filepath.Join(workspace, repo+"_worktrees")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		return fmt.Errorf("creating worktree directory: %w", err)
	}
	worktreePath := filepath.Join(worktreeDir, strings.ReplaceAll(fullBranch, "/", "-"))

	// 4. Create the worktree if it doesn't exist.
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		c.progress("Creating worktree…")
		if err := createWorktree(repoPath, worktreePath, fullBranch, branchExists); err != nil {
			if strings.Contains(err.Error(), "already registered worktree") {
				return &ErrStaleWorktree{
					Repo:         repo,
					Branch:       branch,
					AgentType:    agentType,
					WorktreePath: worktreePath,
					RepoPath:     repoPath,
				}
			}
			return err
		}
	}

	a := agent.Agent{
		Name:         AgentName(repo, fullBranch),
		WorktreePath: worktreePath,
		AgentType:    agentType,
	}
	return c.openWindowAndSave(a)
}

// ForceStart is called after the user confirms they want to prune a stale
// worktree registration and retry. It receives the ErrStaleWorktree returned
// by a previous Start call.
func (c *Controller) ForceStart(e *ErrStaleWorktree) error {
	pruneCmd := exec.Command("git", "worktree", "prune")
	pruneCmd.Dir = e.RepoPath
	pruneCmd.CombinedOutput()

	branchExists := branchExistsInRepo(e.RepoPath, e.WorktreePath)
	if err := createWorktree(e.RepoPath, e.WorktreePath, e.Branch, branchExists); err != nil {
		return err
	}

	a := agent.Agent{
		Name:         AgentName(e.Repo, e.Branch),
		WorktreePath: e.WorktreePath,
		AgentType:    e.AgentType,
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
	workspace, err := config.Workspace()
	if err != nil {
		return err
	}

	exists, err := c.Mux.SessionExists(c.SessionName)
	if err != nil {
		return err
	}
	if !exists {
		if err := c.Mux.CreateSession(c.SessionName, workspace); err != nil {
			return err
		}
	}

	c.progress("Opening tmux window…")
	win, err := c.Mux.CreateWindow(c.SessionName, a.Name, a.WorktreePath)
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

// Remove kills the tmux window, removes the git worktree, and deletes the agent
// from the store.
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

	if a.WorktreePath != "" {
		if _, err := os.Stat(a.WorktreePath); err == nil {
			// git worktree remove must run from the main repo directory.
			// Derive it: WorktreePath is <workspace>/<repo>_worktrees/<branch>,
			// so the repo is the parent of the _worktrees dir.
			repoPath := filepath.Dir(filepath.Dir(a.WorktreePath))
			// Strip the _worktrees suffix to get the actual repo dir name.
			worktreesDir := filepath.Dir(a.WorktreePath)
			repoName := strings.TrimSuffix(filepath.Base(worktreesDir), "_worktrees")
			repoPath = filepath.Join(filepath.Dir(worktreesDir), repoName)

			gitCmd := exec.Command("git", "worktree", "remove", "--force", a.WorktreePath)
			gitCmd.Dir = repoPath
			if out, err := gitCmd.CombinedOutput(); err != nil {
				// Fall back to plain directory removal.
				if rmErr := os.RemoveAll(a.WorktreePath); rmErr != nil {
					return fmt.Errorf("removing worktree: %s", strings.TrimSpace(string(out)))
				}
			}
		}
	}

	return c.Store.Delete(a.Name)
}

// branchExistsInRepo checks whether branch exists locally or on origin,
// running both checks in parallel.
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

func createWorktree(repoPath, worktreePath, branch string, branchExists bool) error {
	var cmd *exec.Cmd
	// TODO we don't want to reuse worktree??
	if branchExists {
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
