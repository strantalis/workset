---
description: Quick lookup table of Workset CLI commands and their purpose.
---

# Command Index

Quick lookup for CLI commands and intent.

| Command | Purpose |
| --- | --- |
| `workset new` | Create a workspace. |
| `workset ls` | List workspaces. |
| `workset exec` | Run a command in a workspace. |
| `workset hooks run` | Run repo hooks for an event. |
| `workset session start` | Start a persistent session. |
| `workset session attach` | Attach to a session. |
| `workset session stop` | Stop a session. |
| `workset session show` | Show session details. |
| `workset session ls` | List sessions for a workspace. |
| `workset version` | Print version. |
| `workset config show` | Show config values. |
| `workset config set` | Set config values. |
| `workset repo alias ls` | List repo aliases. |
| `workset repo alias add` | Add a repo alias. |
| `workset repo alias set` | Update a repo alias. |
| `workset repo alias rm` | Remove a repo alias. |
| `workset repo add` | Add a repo to a workspace. |
| `workset repo ls` | List repos in a workspace. |
| `workset repo rm` | Remove a repo from a workspace. |
| `workset status` | Show workspace status. |
| `workset group ls` | List templates (groups). |
| `workset group show` | Show template contents. |
| `workset group create` | Create a template. |
| `workset group rm` | Remove a template. |
| `workset group add` | Add a repo alias to a template. |
| `workset group remove` | Remove a repo alias from a template. |
| `workset group apply` | Apply a template to a workspace. |
| `workset rm` | Remove a workspace (with `--delete`, stop sessions and delete on disk). |

!!! note
    Commands that operate on a workspace require `-w <workspace>` unless `defaults.workspace` is set.
