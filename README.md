## Agents

Simple, straightforward agent watcher in golang.

## What it does

It works in a directory where all repositories are cloned. The workspace path is set on first run and stored in `~/.config/agents/config.json`.

- Running `agents` with no subcommand ensures a tmux session named `agents` exists and attaches to it.
- `agents start <repo> <branch>` creates a git worktree in `<repo>_worktrees/`, opens a tmux window, and launches opencode.
- `agents start <repo>` (no branch) restarts a previously tracked agent in its existing worktree. If the window is still alive, it's a no-op.
- `agents start all` starts all tracked agents.
- `agents remove <repo>` kills the tmux window, removes the worktree, and deletes the agent from state.
- `agents watch` lists all tracked agents with their tmux window info.
- Agent state (name, worktree path, session ID, window ID) is persisted to `~/.config/agents/state.json`.
- Abstractions for the terminal multiplexer and store allow swapping implementations later.

## Plan

- [x] Project structure with abstractions (Agent, Multiplexer interface, Store interface)
- [x] Cobra CLI scaffolding
- [x] JSON file-based store (`~/.config/agents/state.json`)
- [x] Tmux multiplexer implementation
- [x] Workspace config (`~/.config/agents/config.json`, set on first run)
- [x] `agents start <repo> <branch>` — create worktree, open tmux window, launch opencode, persist state
- [x] `agents start <repo>` — restart a previously tracked agent in its worktree
- [x] `agents start all` — start all tracked agents
- [ ] `agents remove <repo>` — kill window, remove worktree, clean up state
- [ ] `agents watch` — list all agents with status and tmux window info
- [ ] Configuration (opencode binary path)

## For later

- Add hooks into the agent to detect if it's working or pending
- The watch needs to be also a web ui, so we can access if from anywhere
- Use charmbracelet TUI libraries for richer watch display
