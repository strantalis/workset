---
description: Desktop UI for Workset with terminals, workspace management, and GitHub workflows.
---

# Desktop App

Workset includes a desktop UI built with Wails (Go backend + Svelte frontend). It uses the same worksetapi service, config, and workspace state as the CLI.

## What it does

- Manage workspaces (create, rename, archive, delete).
- Add/remove repos and inspect status.
- Embedded terminals per workspace (xterm.js renderer).
- Diffs for local changes and PR branches.
- GitHub workflows: auth, PR status, create PRs, review comments, and generated PR text.
- Settings for defaults, aliases, groups, and GitHub auth.

## Run locally

```bash
cd wails-ui/workset
wails dev
wails build
```

You'll need the Wails CLI plus Go and Node.js installed locally.

## GitHub auth

The app defaults to GitHub CLI. Open Settings -> GitHub to connect via CLI or a personal access token. Tokens are stored in your OS keychain.

## Terminal settings

- `defaults.terminal_renderer` controls the xterm.js renderer (`webgl`).
- `defaults.terminal_idle_timeout` controls idle shutdown for GUI terminals.
- `defaults.agent` controls the generator used for PR/commit text and the default coding agent for terminals.

For the full terminal architecture, see [Terminal Architecture](architecture/terminal.md).
