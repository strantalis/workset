---
description: Global and workspace configuration reference for Workset.
---

# Config

Global config lives at `~/.workset/config.yaml` and stores defaults, repo aliases, templates, and workspace registry entries.

Workspace config lives at `<workspace>/workset.yaml` and is the source of truth for a workspace.
If you have an existing config at `~/.config/workset/config.yaml`, Workset migrates it to `~/.workset/config.yaml` on next load.

`defaults.workspace` can point to a registered workspace name or a path. When set, workspace commands use it if `-w` is omitted.
`defaults.repo_store_root` is where URL-based repos are cloned when added to a workspace.
Remote names and default branches come from repo aliases (`repos.<name>.remote` / `repos.<name>.default_branch`) or from `defaults.remote` / `defaults.base_branch`. For local repos, the configured remote must exist or Workset errors.

## Global config (`~/.workset/config.yaml`)

### Top-level keys

| Key | Description |
| --- | --- |
| `defaults` | Global defaults for commands and workspace behavior. |
| `hooks` | Hook execution defaults and repo trust list. |
| `repos` | Named repo aliases for URL or local path. |
| `workspaces` | Registry of named workspaces and their paths. |

### `defaults`

| Field | Description |
| --- | --- |
| `remote` | Default remote name for repos (used when aliases omit one). |
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

### `hooks`

| Field | Description |
| --- | --- |
| `enabled` | Enable hook execution (default `true`). |
| `on_error` | Default hook error handling (`fail` or `warn`). |
| `repo_hooks.trusted_repos` | Repo names whose `.workset/hooks.yaml` can run without prompting. |

### Session themes

Session theming is opt-in. Set `defaults.session_theme` to `workset` to apply the built-in status line to `tmux`/`screen` sessions. Use the override fields to customize the tmux or screen values.

For screen, the `session_screen_hardstatus` value is passed to `screen -X hardstatus` and split on whitespace, so keep it in the same format you would use in a `hardstatus` line.

### `repos` entries

| Field | Description |
| --- | --- |
| `url` | Git URL to clone. |
| `path` | Local repo path (saved as absolute). |
| `remote` | Remote name for this repo alias (defaults to `defaults.remote`). |
| `default_branch` | Default branch for this repo alias. |

### `workspaces` entries

| Field | Description |
| --- | --- |
| `path` | Workspace path. |

## Example (global)

```yaml
defaults:
  remote: origin
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

hooks:
  enabled: true
  on_error: fail
  repo_hooks:
    trusted_repos: [platform]

repos:
  platform:
    url: git@github.com:org/platform.git
    remote: origin
    default_branch: main

# local repos use "path" (relative paths are resolved to absolute on save)
  local-repo:
    path: /Users/sean/src/local-repo
    remote: origin
    default_branch: main

workspaces:
  core:
    path: ~/.workset/workspaces/core

## Repo hooks (`.workset/hooks.yaml`)

Each repo worktree can define hooks under `.workset/hooks.yaml`. These run when workset creates a new worktree (event: `worktree.created`).

```yaml
hooks:
  - id: bootstrap
    on: [worktree.created]
    run: ["npm", "ci"]
    cwd: "{repo.path}"
    on_error: fail
```
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
| `remote` | Derived from the repo alias or defaults (not stored in workspace config). |
| `default_branch` | Derived from the repo alias or defaults (not stored in workspace config). |

## Example (workspace)

```yaml
name: feature-policy-eval

repos:
  - name: platform
    repo_dir: platform
    local_path: /Users/sean/src/platform
    managed: false
    # remote + default_branch are derived from alias/defaults
```

`local_path` points at the repo's main working copy. When a repo is added from a URL, Workset clones it into `defaults.repo_store_root` and marks it `managed: true`.

## Next steps

- [CLI](cli.md)
- [Templates](templates.md)
