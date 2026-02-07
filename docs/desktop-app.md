---
description: Desktop UI for Workset with terminals, workspace management, and GitHub workflows.
---

# Desktop App

Workset includes a desktop UI built with Wails (Go backend + Svelte frontend). Production builds use the same worksetapi service, config, and workspace state as the CLI. In `wails dev`, app state is isolated under `~/.workset/dev`.

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

Note: `wails dev` reads/writes config, workspaces, repo store, and UI state under `~/.workset/dev`.

## GitHub auth

The app defaults to GitHub CLI. Open Settings -> GitHub to connect via CLI or a personal access token. Tokens are stored in your OS keychain.

## In-app updates (macOS)

The desktop app now supports a custom updater flow from **Settings -> About**:

- Choose an update channel (`stable` or `alpha`).
- Click **Check for Updates**.
- If a newer version exists, click **Update and Restart**.

Update manifests are fetched from `WORKSET_UPDATES_BASE_URL` when set, otherwise from:

```text
https://strantalis.github.io/workset/updates
```

The updater requires both:

- SHA256 match for the update archive.
- Matching macOS signing Team ID from the manifest.

Security and rollback boundaries:

- Remote update assets must use `https://` (dev-only exception: loopback `http://` with `WORKSET_UPDATES_ALLOW_INSECURE_HTTP=1`).
- Archive extraction rejects path traversal and symlink targets that escape the staging root.
- Validation happens before apply (checksum + signing Team ID checks).
- On update failure, the app records a failed update state and leaves the current installed bundle unchanged.

### Manifest helper script

Use `scripts/generate_update_manifest.sh` to create channel manifests:

```bash
scripts/generate_update_manifest.sh \
  --channel stable \
  --version 0.3.0 \
  --asset-url https://github.com/strantalis/workset/releases/download/v0.3.0/workset-v0.3.0-macos-update.zip \
  --sha256 <sha256> \
  --team-id <apple-team-id> \
  --notes-url https://github.com/strantalis/workset/releases/tag/v0.3.0 \
  --output updates/stable.json
```

## Terminal settings

- `defaults.terminal_idle_timeout` controls idle shutdown for GUI terminals.
- `defaults.terminal_protocol_log` enables sessiond protocol logging (restart daemon to apply).
- `defaults.terminal_debug_overlay` shows the terminal debug overlay.
- Terminal links open with Cmd/Ctrl+click for `https://` URLs (disabled while TUI mouse mode is active).
- OSC 52 clipboard writes are supported (clipboard reads are blocked).
- Unicode 11 width handling is enabled for improved emoji alignment.
- `defaults.agent` controls the generator used for PR/commit text and the default coding agent for terminals.
- `defaults.agent_model` optionally overrides the model used for PR/commit text generation (terminal launcher is unaffected).
- `defaults.agent_launch` controls whether agent commands run via a shell (`auto`) or require an agent path with directory separators (`strict`).

For the full terminal architecture, see [Terminal Architecture](architecture/terminal.md).
