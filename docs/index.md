---
title: Workset — Multi-repo threads
description: Workset is a Go CLI and desktop app for managing multi-repo threads with linked Git worktrees.
---

# Workset

Workset links Git worktrees across repos so you can spin up isolated threads without duplicating clones. CLI and desktop app included.

## Why this exists

Every dev has that one side project that started as "I'll just write a quick script" and somehow ended up with a build pipeline. This is mine.

The problem was simple: I work across multiple repos and kept losing track of which branches went together. The reasonable fix was a shell alias. What I built instead has a CLI, a desktop app with embedded terminals, AI-generated pull requests, an in-process terminal service that manages PTY sessions, and a skill marketplace. At no point did anyone ask for this.

If you're here, you either have the same multi-repo problem or you're morbidly curious about what happens when scope creep goes unsupervised. Either way, welcome.

## Quickstart

```bash
brew tap strantalis/homebrew-tap
brew install workset
workset new demo
workset repo add git@github.com:your/org-repo.git -t demo
workset status -t demo
```

## What it does

- **Linked worktrees** — Branch work stays isolated. Main clones stay clean. No duplicate repos on disk.
- **Multi-repo threads** — Group repos into threads that track intent. Add, remove, and inspect from one place.
- **Worksets** — Define repeatable repo bundles. Spin up new threads from the same workset with a single command.
- **Desktop app** — Native Wails app with embedded terminals, diff views, and GitHub workflow integration.
- **AI-powered** — Generate PR text and commit messages with pluggable AI agents. Codex and Claude supported.

## Next steps

- [Getting Started](getting-started.md) — Install, create a thread, and run your first command.
- [Concepts](concepts.md) — Understand worksets, threads, and registered repos.
- [Desktop App](desktop-app.md) — The native GUI with terminals and GitHub integration.
- [CLI Reference](cli.md) — Full command index.
