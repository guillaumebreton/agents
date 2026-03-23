## Agents

A terminal multiplexer-based watcher for coding agents (opencode, pi, etc.).

## What it does

Agents manages coding agent sessions inside tmux. It tracks each agent's working directory, git branch, and live status, and displays everything in a full-screen TUI grouped by tmux window number.

The workspace path is set on first run and stored in `~/.config/agents/config.json`.

### Commands

- `agents` — ensure the `agents` tmux session exists and attach to it (launches `agents watch` in the first window).
- `agents init` — create the config and (re)install hooks for all supported coding agents.
- `agents watch` — full-screen TUI showing all tracked agents.
- `agents register` — called automatically by coding-agent hooks on startup; updates the store entry for the running window.
- `agents update-status` — called automatically by coding-agent hooks on status transitions.
- `agents version` — print version, commit, and build date.
- `--debug` — enable debug logging on any command.

### TUI

The watch TUI shows a table with columns:

| # | Agent | Repo | Branch | Status |
|---|-------|------|--------|--------|
| 1 | opencode | myrepo | main | ● idle |
| 2 | pi | otherrepo | feat/x | ◆ working |

- **#** — tmux window index (matches the number in the tmux status bar).
- **Agent** — coding agent type.
- **Repo** — repository or directory name.
- **Branch** — current git branch (refreshed live every tick).
- **Status** — last reported status from the agent hook.

Rows are sorted by window index and grouped: the `#` is shown only once per window.

Key bindings: `↑/↓` navigate · `enter` switch to window or reopen stopped agent · `s` start new agent · `d` remove agent · `q` quit.

### Starting agents

Press `s` in the watch TUI. Choose between a **workspace directory** (listed from the workspace, with `⎇` marking git repos) or a **custom path**.

For git repositories:
- Leave branch empty → open the agent in the **main checkout** (one agent per repo).
- Enter a branch name → create or reuse a **git worktree** on that branch.

For plain directories (no `.git`): the agent opens directly in that directory, no git management.

### Agent lifecycle by kind

| Kind | On start | On remove |
|------|----------|-----------|
| Worktree | Create `<repo>_worktrees/<branch>` via `git worktree add` | Remove worktree + close window |
| Main | Optionally checkout branch, open in repo root | Close window (branch kept) |
| Plain | Open in directory as-is | Close window |

Pressing `enter` on a stopped agent reopens it in a new window without any git operations.

### Self-registration via hooks

Run `agents init` to install hooks into your coding agents:

- **opencode** — `~/.config/opencode/plugins/agents-hook.js`
- **pi** — `~/.pi/agent/extensions/agents-hook.ts`

On agent startup the hook calls `agents register`, which links the running tmux pane to the store entry. On every status transition the hook calls `agents update-status`. This enables live status in the table without any polling.

Agents started outside of the `agents` tool are silently ignored — only sessions explicitly started through the TUI appear in the store.

## Architecture

- **`internal/agent`** — `Agent` struct with `Kind` (worktree / main / plain), `RepoPath`, `Branch`, `WindowIndex`, and status fields.
- **`internal/agentctl`** — `Controller` with `StartWorktree`, `StartMain`, `Start` (plain), `Reopen`, `Adopt`, and `Remove`. Git operations (worktree creation, branch checkout) live here.
- **`internal/coding`** — `CodingAgent` interface; opencode and pi implementations with hook templates.
- **`internal/multiplexer`** — `Multiplexer` interface with a tmux implementation. Swappable for other multiplexers.
- **`internal/store`** — JSON file store at `~/.config/agents/state.json`. Override with `AGENTS_STATE_FILE`.
- **`internal/config`** — workspace path and default agent at `~/.config/agents/config.json`. Override with `AGENTS_CONFIG_FILE`.
- **`cmd/watch`** — Bubble Tea TUI.

## Testing

```sh
make test               # unit tests
make test-integration   # hook integration tests (builds binary, simulates full register→update-status flow)
```

The integration tests use `AGENTS_CONFIG_FILE` and `AGENTS_STATE_FILE` env vars for full isolation — no real tmux or running agents needed.
