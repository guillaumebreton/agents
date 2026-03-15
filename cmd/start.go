package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"notb.re/agents/internal/agent"
	"notb.re/agents/internal/config"
)

var startCmd = &cobra.Command{
	Use:   "start <repo> [branch]",
	Short: "Start an agent for a repository",
	Long: `Start an agent for the given repository.

On the first call, a branch is required to create the git worktree.
On subsequent calls, the branch is optional — the existing worktree is reused.

If a window already exists, checks it is still running. Otherwise opens a new
tmux window in the worktree and launches the coding agent.

Use 'agents start all' to start all tracked agents.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo := args[0]

		// Handle "agents start all"
		if repo == "all" {
			return startAll()
		}

		var branch string
		if len(args) == 2 {
			branch = args[1]
		}

		return startAgent(repo, branch)
	},
}

func startAgent(repo string, branch string) error {
	// Check if agent is already tracked.
	a, err := dataStore.Get(repo)
	if err != nil {
		// Agent not found — this is a new agent, branch is required.
		if branch == "" {
			return fmt.Errorf("agent %q not found, a branch is required: agents start %s <branch>", repo, repo)
		}
		return startNewAgent(repo, branch)
	}

	return startExistingAgent(a)
}

func startNewAgent(repo string, branch string) error {
	workspace, err := config.Workspace()
	if err != nil {
		return err
	}

	repoPath := filepath.Join(workspace, repo)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository directory %q not found", repoPath)
	}

	// Create worktree directory.
	worktreeDir := filepath.Join(workspace, repo+"_worktrees")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		return fmt.Errorf("creating worktree directory: %w", err)
	}

	worktreePath := filepath.Join(worktreeDir, branch)

	// Create git worktree.
	gitCmd := exec.Command("git", "worktree", "add", worktreePath, branch)
	gitCmd.Dir = repoPath
	if out, err := gitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating worktree: %s: %w", strings.TrimSpace(string(out)), err)
	}

	fmt.Printf("created worktree at %s\n", worktreePath)

	// Create the agent and start it.
	a := agent.Agent{
		Name:         repo,
		WorktreePath: worktreePath,
	}

	return openWindowAndSave(a)
}

func startExistingAgent(a agent.Agent) error {
	// If a window is already tracked, check if it's still alive.
	if a.WindowID != "" {
		alive, err := mux.WindowExists(a.WindowID)
		if err != nil {
			return err
		}
		if alive {
			fmt.Printf("agent %q is already running in window %s\n", a.Name, a.WindowID)
			return nil
		}
		// Window is gone, clear it.
		a.WindowID = ""
	}

	return openWindowAndSave(a)
}

func openWindowAndSave(a agent.Agent) error {
	// Ensure session exists.
	workspace, err := config.Workspace()
	if err != nil {
		return err
	}
	exists, err := mux.SessionExists(sessionName)
	if err != nil {
		return err
	}
	if !exists {
		if err := mux.CreateSession(sessionName, workspace); err != nil {
			return err
		}
	}

	// Create a new window in the session.
	win, err := mux.CreateWindow(sessionName, a.Name, a.WorktreePath)
	if err != nil {
		return err
	}
	a.WindowID = win.ID

	// Launch opencode in the window.
	if err := mux.SendCommand(win.ID, "opencode"); err != nil {
		return fmt.Errorf("launching opencode: %w", err)
	}

	// Persist agent state.
	if err := dataStore.Save(a); err != nil {
		return err
	}

	fmt.Printf("started agent %q in window %s\n", a.Name, win.ID)
	return nil
}

func startAll() error {
	agents, err := dataStore.List()
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		fmt.Println("no tracked agents to start")
		return nil
	}
	for _, a := range agents {
		fmt.Printf("starting %q...\n", a.Name)
		if err := startExistingAgent(a); err != nil {
			fmt.Fprintf(os.Stderr, "error starting %q: %v\n", a.Name, err)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(startCmd)
}
