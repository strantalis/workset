---
description: Core Workset concepts including workspaces, repo sources, worktrees, remotes, and templates.
---

# Concepts

## Workspace

A workspace is a directory with `workset.yaml` and `.workset/` state. It captures intent: “these repos move together.”

```
<workspace>/
  workset.yaml
  .workset/
  <repo>/
```

## Repo sources

- **Local paths** stay put and are referenced by absolute path.
- **URL repos** are cloned into `~/.workset/repos` (configurable) and marked as `managed: true`.

## Worktrees

Worktrees live under `<workspace>/<repo>` by default, keeping your main clones clean and stable.

!!! tip
    This makes branch work fast and isolated without duplicating repositories.

## Remotes

Workset treats remotes as explicit intent:

- **base**: the authoritative upstream
- **write**: the fork or repo you can push to

## Templates (groups)

Templates are reusable repo sets stored in global config. Apply a template to a workspace to bring in a known set of repos quickly.

## Next steps

- [Getting Started](getting-started.md)
- [Config](config.md)
