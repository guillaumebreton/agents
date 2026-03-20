## Agents

Simple, straightforward agent watcher in golang.

## What it does

It works in a directory where all repositories are cloned. The workspace path is set on first run and stored in `~/.config/agents/config.json`.

- Running `agents` with no subcommand ensures a tmux session named `agents` exists, launches `agents watch` in the first window, and attaches to it.
- `agents init` creates the config file with the current directory as workspace.
- `agents start <repo> <branch>` creates a git worktree in `<repo>_worktrees/`, opens a tmux window, and launches the coding agent.
- `agents start <repo> <branch> --agent=opencode` specifies which coding agent to use.
- `agents start <repo>` (no branch) restarts a previously tracked agent in its existing worktree. If the window is still alive, it's a no-op.
- `agents start all` starts all tracked agents.
- `agents remove <repo>` kills the tmux window, removes the worktree, and deletes the agent from state.
- `agents watch` full-screen TUI showing all tracked agents with live status.
- `agents version` prints the version.
- `--debug` flag enables debug logging on any command.

## Architecture

- **Coding agents** are code-based abstractions (`internal/coding`). Each implementation (e.g. OpenCode) registers itself and provides its launch command. New agents are added as code, not config.
- **Multiplexer** is an interface (`internal/multiplexer`) with a tmux implementation. Swappable for zellij etc.
- **Store** persists agent state to `~/.config/agents/state.json`.
- **Config** stores workspace path in `~/.config/agents/config.json`.

## Plan

- [x] Project structure with abstractions
- [x] Cobra CLI scaffolding
- [x] JSON file-based store
- [x] Tmux multiplexer implementation
- [x] Workspace config
- [x] `agents start` (new, existing, all)
- [x] `agents remove`
- [x] `agents watch` with charmbracelet TUI
- [x] `agents init`
- [x] `agents version`
- [x] Code-based coding agent abstraction with OpenCode implementation
- [x] `--agent` flag
- [x] `--debug` flag
- [x] CI (GitHub Actions: build, test, vet)
- [x] Release pipeline (goreleaser)
- [x] Tests (config, store, coding registry)

## For later

- Add more coding agent implementations (claude, aider, etc.)
- Add hooks into the agent to detect if it's working or pending
- Navigation actions in watch (enter to jump to window, r to remove, s to start)
- Web UI for watch

## Known issues

- Long delay between popup closing and agent creation
- Error message when removing a worktree
- Error message display breaks the layout
- Stale worktree confirmation breaks layout — replace stdin prompt with TUI popup
