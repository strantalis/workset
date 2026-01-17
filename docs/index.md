---
description: Workset is a Go CLI for managing multi-repo workspaces with linked Git worktrees and explicit remotes.
---

# Workset

<p align="center">
  <img src="assets/workset.png" alt="Workset" width="640">
</p>

Workset is a Go CLI for managing multi-repo workspaces with linked worktrees by default. It keeps your main clones where they are and spins up workspace worktrees under the workspace directory.

## Quickstart

```bash
go build ./cmd/workset
./workset new demo
./workset repo add git@github.com:your/org-repo.git -w demo
./workset status -w demo
```

!!! tip
    Set `defaults.workspace` in your global config to skip `-w` for most commands.

## What you get

- Workspaces that track intent across repos.
- Linked worktrees for isolation without duplication.
- URL repos cloned into `~/.workset/repos` (configurable).
- Explicit base/write remotes for forks.
- Templates for repeatable repo bundles.

## Next up

Branch create/checkout and worktree lifecycle commands are next in the roadmap.

## Learn more

- [Getting Started](getting-started.md)
- [Concepts](concepts.md)
- [Config](config.md)


