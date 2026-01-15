# Config

Global config lives at `~/.config/workset/config.yaml` and stores defaults, repo aliases, templates, and workspace registry entries.

Workspace config lives at `<workspace>/workset.yaml` and is the source of truth for a workspace.

`defaults.workspace` can point to a registered workspace name or a path. When set, workspace commands use it if `-w` is omitted.
`defaults.repo_store_root` is where URL-based repos are cloned when added to a workspace.

## Example (global)

```yaml
defaults:
  base_branch: main
  workspace: core
  workspace_root: ~/.workset/workspaces
  repo_store_root: ~/.workset/repos
  remotes:
    base: upstream
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

## Example (workspace)

```yaml
name: feature-policy-eval

repos:
  - name: platform
    repo_dir: platform
    local_path: /Users/sean/src/platform
    managed: false
    remotes:
      base:  { name: upstream, default_branch: main }
      write: { name: origin }
```

`local_path` points at the repo's main working copy. When a repo is added from a URL, Workset clones it into `defaults.repo_store_root` and marks it `managed: true`.
