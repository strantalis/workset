# Workset

[![test](https://github.com/strantalis/workset/actions/workflows/test.yml/badge.svg)](https://github.com/strantalis/workset/actions/workflows/test.yml)
[![lint](https://github.com/strantalis/workset/actions/workflows/lint.yml/badge.svg)](https://github.com/strantalis/workset/actions/workflows/lint.yml)
[![release](https://github.com/strantalis/workset/actions/workflows/release.yml/badge.svg)](https://github.com/strantalis/workset/actions/workflows/release.yml)
[![docs](https://github.com/strantalis/workset/actions/workflows/docs.yml/badge.svg)](https://github.com/strantalis/workset/actions/workflows/docs.yml)

<p align="center">
  <img src="docs/assets/workset.png" alt="Workset" width="560">
</p>

Workset is a Go CLI for managing **multi-repo workspaces** with **linked Git worktrees** by default. It captures intent ("these repos move together") and keeps multi-repo work safe, explicit, and predictable.

## Why Workset

- **Workspaces first**: treat related repos as a single unit of work.
- **Linked worktrees by default**: branch work happens in isolated directories without duplicating repos.
- **Repo defaults**: remote + default branch come from aliases or global defaults.
- **Templates**: reusable repo sets that expand into workspace config.
- **Safe defaults**: no destructive actions without explicit flags.

## Status

Workset is in active development. Current commands focus on workspace creation, repo add, status, and groups/templates. Branch/worktree workflows are next.

> [!WARNING]
> This project is under active development; interfaces and behavior may change without notice.

## Quickstart

Install (recommended):

```bash
brew tap strantalis/homebrew-tap
brew install workset
```

Upgrade (Homebrew):

```bash
brew update
brew upgrade --cask workset
```

Install (npm):

```bash
npm install -g @strantalis/workset@latest
```

Alternative (Go install):

```bash
go install github.com/strantalis/workset/cmd/workset@latest
```

Create a workspace and add repos:

```bash
workset new demo
workset repo add git@github.com:your/org-repo.git -w demo
workset status -w demo
```

Templates:

```bash
workset group create platform
workset group add platform repo-alias
workset group apply platform -w demo
```

Sessions (tmux/screen/exec):

```bash
workset session start demo -- zsh
workset session attach demo
workset session show demo
workset session stop demo
```

## Concepts

- **Workspace**: a directory with `workset.yaml` and `.workset/` state.
- **Repo sources**: local paths stay put; URL clones land in `~/.workset/repos` (configurable).
- **Worktrees**: worktrees live under `<workspace>/<repo>` by default.
- **Repo defaults**: aliases and global defaults supply the remote + default branch.
- **Templates**: global repo sets applied into a workspace.

## Docs

Docs are built with **MkDocs + Material**. The site config is `mkdocs.yml`, markdown content lives in `docs/`, and the published site is `workset.dev`.

Local dev (requires `uv`):

```bash
make docs-serve
```

## Roadmap

- Branch create/checkout + worktree management
- Fetch/pull/exec across repos
- Scoped status and JSON output
