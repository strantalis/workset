---
description: Global and workspace configuration reference for Workset.
---

# Config

Global config lives at `~/.workset/config.yaml` and stores defaults, repo aliases, templates, and workspace registry entries.

Workspace config lives at `<workspace>/workset.yaml` and is the source of truth for a workspace.
If you have an existing config at `~/.config/workset/config.yaml`, Workset migrates it to `~/.workset/config.yaml` on next load.

`defaults.workspace` can point to a registered workspace name or a path. When set, workspace commands use it if `-w` is omitted.
`defaults.repo_store_root` is where URL-based repos are cloned when added to a workspace.
Remote names for local repos are derived from the repo itself; URL-based repos default to `origin`. Use `workset repo remotes set` to override per workspace repo.

## Global config (`~/.workset/config.yaml`)

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
| `session_theme` | Optional session theme for `tmux`/`screen` (`workset` to enable built-in theme). |
| `session_tmux_status_style` | Override tmux `status-style` when a session theme is enabled. |
| `session_tmux_status_left` | Override tmux `status-left` when a session theme is enabled. |
| `session_tmux_status_right` | Override tmux `status-right` when a session theme is enabled. |
| `session_screen_hardstatus` | Override screen `hardstatus` when a session theme is enabled. |

### Session themes

Session theming is opt-in. Set `defaults.session_theme` to `workset` to apply the built-in status line to `tmux`/`screen` sessions. Use the override fields to customize the tmux or screen values.

For screen, the `session_screen_hardstatus` value is passed to `screen -X hardstatus` and split on whitespace, so keep it in the same format you would use in a `hardstatus` line.

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
  session_name_format: workset-{workspace}
  # optional: enable built-in session theme for tmux/screen
  session_theme: workset
  # optional: override tmux or screen status lines
  # session_tmux_status_style: "bg=colour235,fg=colour250"
  # session_tmux_status_left: " #[fg=colour39]workset #[fg=colour250]#S "
  # session_tmux_status_right: " #[fg=colour244]%Y-%m-%d %H:%M "
  # session_screen_hardstatus: "alwayslastline workset %n %t %=%H:%M %d-%b-%y"

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
