---
description: Global and thread configuration reference for Workset.
---

# Config

Global config lives at `~/.workset/config.yaml` and stores defaults, registered repos, and the workset registry.

Thread config lives at `<thread>/workset.yaml` and is the source of truth for a thread.
Workset only reads the canonical config path above; legacy config locations and legacy keys are not part of the supported contract.

`defaults.thread` can point to a registered thread name or a path. When set, thread commands use it if `-t` is omitted.
`defaults.repo_store_root` is where URL-based repos are cloned when added to a thread.
Remote names and default branches come from registered repos (`repos.<name>.remote` / `repos.<name>.default_branch`) or from `defaults.remote` / `defaults.base_branch`. For local repos, the configured remote must exist or Workset errors.

## Global config (`~/.workset/config.yaml`)

### Top-level keys

| Key        | Description                                                                 |
| ---------- | --------------------------------------------------------------------------- |
| `defaults` | Global defaults for commands and thread/workset behavior.                   |
| `github`   | GitHub auth defaults and overrides.                                         |
| `hooks`    | Hook execution defaults and repo trust list.                                |
| `repos`    | Registered repos for URL or local path sources.                             |
| `worksets` | Registry of named worksets, each with optional `repos` and a `threads` map. |

### `defaults`

| Field                    | Description                                                                                                                                                           |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `remote`                 | Default remote name for repos (used when aliases omit one).                                                                                                           |
| `base_branch`            | Default branch for new worktrees.                                                                                                                                     |
| `thread`                 | Default thread name or absolute path.                                                                                                                                 |
| `workset_root`           | Base directory used for generated workset and thread paths. Default thread paths are `<workset_root>/worksets/<workset>/<thread>`.                                    |
| `repo_store_root`        | Where URL-based repos are cloned.                                                                                                                                     |
| `agent`                  | Default agent for PR text generation, commit messages, and the GUI terminal launcher. Supported values: `codex`, `claude`.                                            |
| `agent_model`            | Optional model override for PR text generation and commit messages (does not affect the terminal launcher). Examples: `gpt-5.1-codex-mini` (Codex), `haiku` (Claude). |
| `terminal_idle_timeout`  | Idle timeout for GUI terminals/sessiond (duration like `30m`; use `0` to disable). Default is `0`.                                                                    |
| `terminal_protocol_log`  | Enable sessiond protocol logging (`on`/`off`). Requires daemon restart.                                                                                               |
| `terminal_debug_overlay` | Show the terminal debug overlay (`on`/`off`).                                                                                                                         |
| `terminal_font_size`     | Terminal text size in pixels. Must be an integer between `8` and `28`. Default is `13`.                                                                               |
| `terminal_cursor_blink`  | Whether the terminal cursor blinks (`on`/`off`). Default is `on`.                                                                                                     |

### `hooks`

| Field                      | Description                                                       |
| -------------------------- | ----------------------------------------------------------------- |
| `enabled`                  | Enable hook execution (default `true`).                           |
| `on_error`                 | Default hook error handling (`fail` or `warn`).                   |
| `repo_hooks.trusted_repos` | Repo names whose `.workset/hooks.yaml` can run without prompting. |

### `github`

| Field      | Description                                                                  |
| ---------- | ---------------------------------------------------------------------------- |
| `cli_path` | Optional override for the `gh` CLI path (useful for Nix or custom installs). |

### `repos` entries

| Field            | Description                                                           |
| ---------------- | --------------------------------------------------------------------- |
| `url`            | Git URL to clone.                                                     |
| `path`           | Local repo path (saved as absolute).                                  |
| `remote`         | Remote name for this registered repo (defaults to `defaults.remote`). |
| `default_branch` | Default branch for this registered repo.                              |

### `worksets` entries

`worksets` is keyed by workset name. Each entry can define the workset's repo bundle and its registered threads.

| Field     | Description                                                                       |
| --------- | --------------------------------------------------------------------------------- |
| `repos`   | Registered repo names associated with the workset.                                |
| `threads` | Map of thread names to thread refs (`path`, metadata, optional `repo_overrides`). |

## Example (global)

```yaml
defaults:
  remote: origin
  base_branch: main
  thread: core
  workset_root: ~/.workset
  repo_store_root: ~/.workset/repos
  agent: codex
  # agent_model: gpt-5.1-codex-mini
  terminal_idle_timeout: "0"
  terminal_protocol_log: off
  terminal_debug_overlay: off
  terminal_font_size: "13"
  terminal_cursor_blink: on

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

worksets:
  core:
    repos: [platform]
    threads:
      feature-policy-eval:
        path: ~/.workset/worksets/core/feature-policy-eval
        workset: core
```

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

## Thread config (`<thread>/workset.yaml`)

### Top-level fields

| Field   | Description                         |
| ------- | ----------------------------------- |
| `name`  | Thread display name.                |
| `repos` | List of repo entries in the thread. |

### `repos` entries

| Field            | Description                                                                 |
| ---------------- | --------------------------------------------------------------------------- |
| `name`           | Repo alias name.                                                            |
| `repo_dir`       | Directory name under the thread.                                            |
| `local_path`     | Path to the repo's main working copy.                                       |
| `managed`        | `true` if Workset owns the clone.                                           |
| `remote`         | Derived from the registered repo or defaults (not stored in thread config). |
| `default_branch` | Derived from the registered repo or defaults (not stored in thread config). |

## Example (thread)

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
- [Concepts](concepts.md)
