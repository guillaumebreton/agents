## Agents

Simple, straight forward agent watcher in golang.

## What it does

It work in a directory where all repositories are clone. 
- It works in a tmux session name agent. 
- In the first window, it starts agent watch - which for now list the agent and the session
- When you do agent start <repostiory_name> branch, it creates a worktree
for the agent, with the branch, in repository_name_worktrees. it stores the link of the window with the repository name in his cache
- I should be able to stop and restart easily. 
- When you do agent start it check if an opencode (configurable) system, and then start the agent in a new tmux window
- It should have an abstraction of the agent and an abstraction of terminal multiplexer, so we can swap the implementation later. 
- when I do agent remove
- It should keep track of the session if it can so I can to agent start and it restart in the right place
- It should keep track of the agent, session, worktree somewhere.
- I should be able to do agent start all 
- It should use cobra (or SOTA lib)
- The watch for now only display a list of agents, with pane number.
- It shoul duse charmcli later


## For later

- Depending on the agent it should able to add a hook into the agent to detect if it's working or pending. either by using hooks or something else
-
