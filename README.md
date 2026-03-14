## Agents

Simple, straightforward agent watcher in golang.

## What it does

It works in a directory where all repositories are cloned.

- It works in a tmux session named `agents`.
- In the first window, `agents watch` lists all tracked agents with their tmux window/pane info.
- `agents start <repo> <branch>` creates a git worktree in `<repo>_worktrees/`, opens a tmux window, detects opencode config and launches the agent.
- `agents start <repo>` (no branch) restarts a previously tracked agent in its existing worktree.
- `agents start all` starts all tracked agents.
- `agents stop <repo>` stops an agent by killing its tmux window.
- `agents remove <repo>` stops the agent, removes the worktree, and cleans up state.
- Agent state (name, worktree, session ID, window ID) is persisted to `~/.config/agents/state.json`.
- Abstractions for the terminal multiplexer and store allow swapping implementations later.

## Plan

- [x] Project structure with abstractions (Agent, Multiplexer interface, Store interface)
- [x] Cobra CLI scaffolding (start, stop, remove, watch commands)
- [x] JSON file-based store (`~/.config/agents/state.json`)
- [x] Tmux multiplexer implementation
- [ ] `agents start <repo> <branch>` — create worktree, open tmux window, detect & launch opencode, persist state
- [ ] `agents start <repo>` — restart a previously tracked agent in its worktree
- [ ] `agents start all` — start all tracked agents
- [ ] `agents stop <repo>` — kill tmux window, update state
- [ ] `agents remove <repo>` — stop agent, remove worktree, clean up state
- [ ] `agents watch` — list all agents with status and tmux window info
- [ ] Configuration (opencode binary path)

## For later

- Add hooks into the agent to detect if it's working or pending
- Use charmbracelet TUI libraries for richer watch display
