---
description: Workset is a Go CLI and desktop app for managing multi-repo workspaces with linked Git worktrees and consistent repo defaults.
---

# Workset

<p align="center">
  <img src="assets/workset.png" alt="Workset" width="640">
</p>

Workset is a Go CLI plus a Wails desktop app for managing multi-repo workspaces with linked worktrees by default. It keeps your main clones where they are and spins up workspace worktrees under the workspace directory.

## Quickstart

```bash
brew tap strantalis/homebrew-tap
brew install workset
workset new demo
workset repo add git@github.com:your/org-repo.git -w demo
workset status -w demo
```

!!! tip
    Set `defaults.workspace` in your global config to skip `-w` for most commands.

## What you get

- Workspaces that track intent across repos.
- Linked worktrees for isolation without duplication.
- URL repos cloned into `~/.workset/repos` (configurable).
- Repo defaults (remote + default branch) via aliases.
- Templates for repeatable repo bundles.
- A desktop app for workspace management, terminals, and GitHub workflows.

## Next up

Branch create/checkout and worktree lifecycle commands are next in the roadmap.

## Learn more

- [Getting Started](getting-started.md)
- [Concepts](concepts.md)
- [Config](config.md)
- [Desktop App](desktop-app.md)
