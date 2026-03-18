---
description: Core Workset concepts including worksets, threads, repo sources, and linked worktrees.
---

# Concepts

## Workset

A workset is the canonical grouping for related threads and their repo bundle. By default, new threads live under:

```text
<workset_root>/worksets/<workset>/<thread>
```

Worksets let you keep several focused threads under one higher-level stream of work.

### Creating workset-backed threads

Register repos globally, then create threads with `--workset` to group them:

```bash
workset repo registry add platform git@github.com:org/platform.git
workset repo registry add api git@github.com:org/api.git
workset new auth-spike --workset platform-core --repo platform --repo api
```

The thread is created under `<workset_root>/worksets/platform-core/auth-spike`. To reuse the same repo bundle, create new threads with the same `--workset` and repo selection.

## Thread

A thread is a concrete directory with `workset.yaml` and `.workset/` state. It captures intent for one active branch of work.

```
<thread>/
  workset.yaml
  .workset/
  <repo>/
```

## Repo sources

- **Local paths** stay put and are referenced by absolute path.
- **URL repos** are cloned into `~/.workset/repos` (configurable) and marked as `managed: true`.

## Worktrees

Worktrees live under `<thread>/<repo>` by default, keeping your main clones clean and stable.

!!! tip
    This makes branch work fast and isolated without duplicating repositories.

## Registered repos

Registered repos define the remote name and default branch for a repo. If an entry omits them, Workset falls back to `defaults.remote` and `defaults.base_branch`.

You manage them with `workset repo registry ...` in the CLI and from the Repo Catalog in the desktop app.

## Next steps

- [Getting Started](getting-started.md)
- [Config](config.md)
