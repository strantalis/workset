---
description: Global and workspace configuration reference for Workset.
---

# Config

Global config lives at `~/.config/workset/config.yaml` and stores defaults, repo aliases, templates, and workspace registry entries.

Workspace config lives at `<workspace>/workset.yaml` and is the source of truth for a workspace.

`defaults.workspace` can point to a registered workspace name or a path. When set, workspace commands use it if `-w` is omitted.
`defaults.repo_store_root` is where URL-based repos are cloned when added to a workspace.

## Global config (`~/.config/workset/config.yaml`)

### Top-level keys

| Key | Description |
| --- | --- |
| `defaults` | Global defaults for commands and workspace behavior. |
| `repos` | Named repo aliases for URL or local path. |
| `workspaces` | Registry of named workspaces and their paths. |

### `defaults`

| Field | Description |
| --- | --- |
| `base_branch` | Default branch for new worktrees. |
| `workspace` | Default workspace name or absolute path. |
| `workspace_root` | Base directory for new workspaces. |
| `repo_store_root` | Where URL-based repos are cloned. |
| `session_backend` | Default session backend (`auto`, `tmux`, `screen`, `exec`). |
| `session_name_format` | Format string for session names (supports `{workspace}`). |
| `remotes.base` | Default base remote name. |
| `remotes.write` | Default write remote name. |
| `parallelism` | Max parallel operations. |

### `repos` entries

| Field | Description |
| --- | --- |
| `url` | Git URL to clone. |
| `path` | Local repo path (saved as absolute). |
| `default_branch` | Default branch for this repo alias. |

### `workspaces` entries

| Field | Description |
| --- | --- |
| `path` | Workspace path. |

## Example (global)

```yaml
defaults:
  base_branch: main
  workspace: core
  workspace_root: ~/.workset/workspaces
  repo_store_root: ~/.workset/repos
  session_backend: auto
  session_name_format: workset:{workspace}
  remotes:
    base: origin
    write: origin
  parallelism: 8

repos:
  platform:
    url: git@github.com:org/platform.git
    default_branch: main

# local repos use "path" (relative paths are resolved to absolute on save)
  local-repo:
    path: /Users/sean/src/local-repo
    default_branch: main

workspaces:
  core:
    path: ~/.workset/workspaces/core
```

## Workspace config (`<workspace>/workset.yaml`)

### Top-level fields

| Field | Description |
| --- | --- |
| `name` | Workspace display name. |
| `repos` | List of repo entries in the workspace. |

### `repos` entries

| Field | Description |
| --- | --- |
| `name` | Repo alias name. |
| `repo_dir` | Directory name under the workspace. |
| `local_path` | Path to the repo's main working copy. |
| `managed` | `true` if Workset owns the clone. |
| `remotes.base` | Base remote name + default branch. |
| `remotes.write` | Write remote name. |

## Example (workspace)

```yaml
name: feature-policy-eval

repos:
  - name: platform
    repo_dir: platform
    local_path: /Users/sean/src/platform
    managed: false
    remotes:
      base:  { name: origin, default_branch: main }
      write: { name: origin }
```

`local_path` points at the repo's main working copy. When a repo is added from a URL, Workset clones it into `defaults.repo_store_root` and marks it `managed: true`.

## Next steps

- [CLI](cli.md)
- [Templates](templates.md)
