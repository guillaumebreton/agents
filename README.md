## Agents

Simple, straight forward agent watcher in golang.

## What it does

It work in a directory where all repositories are clone. 
- It works in a tmux session name agent. 
- It works in a tmux session named agents. 
- In the first window, it starts agents watch - which for now lists the agents and the session
- When you do agents start <repository_name> <branch>, it creates a worktree for the agent, with the branch, in repository_name_worktrees. It stores the link of the window with the repository name in its cache
- I should be able to stop and restart easily. 
- When you do agents start it checks if an opencode (configurable) system, and then starts the agent in a new tmux window
- It should have an abstraction of the agent and an abstraction of terminal multiplexer, so we can swap the implementation later. 
- When I do agents remove
- It should keep track of the session if it can so I can do agents start and it restarts in the right place
- It should keep track of the agent, session, worktree somewhere.
- I should be able to do agents start all 
- It should use cobra (or SOTA lib)
- The watch for now only displays a list of agents, with pane number.
- It should use charmcli later


## For later

- Depending on the agent it should able to add a hook into the agent to detect if it's working or pending. either by using hooks or something else
-
