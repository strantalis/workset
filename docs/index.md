# Workset

![Workset](assets/workset.png)

Workset is a Go CLI for managing multi-repo workspaces with linked worktrees by default. It keeps your main clones where they are and spins up feature worktrees under `worktrees/<feature>`.

## What you get

- Workspaces that track intent across repos.
- Linked worktrees for isolation without duplication.
- URL repos cloned into `~/.workset/repos` (configurable).
- Explicit base/write remotes for forks.
- Templates for repeatable repo bundles.

## Next up

Branch create/checkout and worktree lifecycle commands are next in the roadmap.
