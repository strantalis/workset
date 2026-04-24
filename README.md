# Creating a PR!!!
# Workset

[![test](https://github.com/strantalis/workset/actions/workflows/test.yml/badge.svg)](https://github.com/strantalis/workset/actions/workflows/test.yml)
[![lint](https://github.com/strantalis/workset/actions/workflows/lint.yml/badge.svg)](https://github.com/strantalis/workset/actions/workflows/lint.yml)
[![release](https://github.com/strantalis/workset/actions/workflows/release.yml/badge.svg)](https://github.com/strantalis/workset/actions/workflows/release.yml)
[![docs](https://github.com/strantalis/workset/actions/workflows/docs.yml/badge.svg)](https://github.com/strantalis/workset/actions/workflows/docs.yml)

All your repos. One place.

A **workset** is a collection of repos that belong together. When you start a feature, you create a **thread** that spins up linked worktrees across all of them. No duplicate clones, no losing track of which branches go together. Desktop app and CLI included.

<p align="center">
  <img src=".github/workset-model.svg" alt="Workset model: worksets group repos, threads create linked worktrees across them" width="680">
</p>

- **Desktop app**: embedded terminals, diff views, PR management, and GitHub workflows in one window.
- **GitHub workflows**: create PRs, view status, read review comments, and generate PR descriptions.
- **AI-powered**: generate PR text and commit messages with pluggable agents (Codex, Claude).
- **Safe defaults**: no destructive actions without explicit flags.

> [!NOTE]
> Workset is under active development; interfaces and behavior may change without notice.

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

Create a thread and add repos:

```bash
workset new demo
workset repo add git@github.com:your/org-repo.git -t demo
workset status -t demo
```

Worksets (reusable repo bundles):

```bash
workset repo registry add platform git@github.com:org/platform.git
workset repo registry add api git@github.com:org/api.git
workset new auth-spike --workset platform-core --repo platform --repo api
```

## Docs

Docs are built with **Astro + Starlight**. The site config is `docs-site/astro.config.mjs`, content lives in `docs-site/src/content/docs/`, and the published site is [workset.dev](https://workset.dev).

Local dev:

```bash
make docs-serve
```

