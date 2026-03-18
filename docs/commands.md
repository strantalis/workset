---
description: Quick lookup table of Workset CLI commands and their purpose.
---

# Command Index

Quick lookup for CLI commands and intent.

| Command | Purpose |
| --- | --- |
| `workset new` | Create a thread, optionally under an explicit workset with `--workset` and seeded repos via `--repo`. |
| `workset ls` | List registered threads. |
| `workset hooks run` | Run repo hooks for an event. |
| `workset version` | Print version information. |
| `workset config show` | Show the canonical global config. |
| `workset config set` | Set a supported `defaults.*` value. |
| `workset config recover` | Rebuild config registrations by scanning `workset_root/worksets`. |
| `workset repo registry ls` | List registered repos. |
| `workset repo registry add` | Register a repo from a Git URL or local path. |
| `workset repo registry set` | Update a registered repo. |
| `workset repo registry rm` | Unregister a repo. |
| `workset repo add` | Add a repo to a thread. |
| `workset repo ls` | List repos in a thread. |
| `workset repo rm` | Remove a repo from a thread. |
| `workset status` | Show thread status. |
| `workset rm` | Remove a thread and, with `--delete`, delete it on disk. |

!!! note
    Commands that operate on a thread require `-t <thread>` unless `defaults.thread` is set.
