# Config

Global config lives at `~/.config/workset/config.yaml` and stores defaults, repo aliases, templates, and workspace registry entries.

Workspace config lives at `<workspace>/workset.yaml` and is the source of truth for a workspace.

## Example (global)

```yaml
defaults:
  base_branch: main
  remotes:
    base: upstream
    write: origin
  parallelism: 8

repos:
  platform:
    url: git@github.com:org/platform.git
    default_branch: main
```

## Example (workspace)

```yaml
name: feature-policy-eval

repos:
  - name: platform
    repo_dir: repos/platform
    editable: true
    remotes:
      base:  { name: upstream, default_branch: main }
      write: { name: origin }
```
