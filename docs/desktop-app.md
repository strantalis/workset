---
description: Desktop UI for Workset with terminals, thread management, and GitHub workflows.
---

# Desktop App

Workset includes a native desktop app built with Wails (Go backend + Svelte frontend). It shares the same config, thread state, and worksetapi service as the CLI.

<img class="desktop-app-hero-shot" src="../assets/screenshots/desktop-overview.png" alt="Workset desktop app with explorer, terminals, and document viewer">

---

<div class="ws-feature-row" markdown>
<div class="ws-feature-row__text" markdown>

## Thread Management

Create, rename, archive, and delete threads. Add and remove repos, inspect status, and browse local changes — all from the cockpit.

</div>
<div class="ws-feature-row__media">
  <img class="desktop-app-feature-shot" src="../assets/screenshots/thread-management.png" alt="Thread management view with explorer and split terminals">
</div>
</div>

<div class="ws-feature-row ws-feature-row--reverse" markdown>
<div class="ws-feature-row__text" markdown>

## Integrated Terminals

Embedded terminals per thread powered by Ghostty Web. Clickable links, clipboard support, and proper emoji rendering included.

</div>
<div class="ws-feature-row__media">
  <img class="desktop-app-feature-shot" src="../assets/screenshots/integrated-terminals.png" alt="Integrated terminal view with OpenCode running inside Workset">
</div>
</div>

<div class="ws-feature-row" markdown>
<div class="ws-feature-row__text" markdown>

## GitHub Workflows

Authenticate via GitHub CLI or personal access token. Create PRs, view status, read review comments, and generate PR descriptions with AI — directly from the app.

</div>
<div class="ws-feature-row__media">
  <img class="desktop-app-feature-shot" src="../assets/screenshots/github-workflows.png" alt="GitHub workflow view for creating a pull request">
</div>
</div>

---

## Install

Download the latest macOS build from [GitHub Releases](https://github.com/strantalis/workset/releases).

!!! tip "Looking for the CLI?"
    The CLI is a separate binary. Install it via Homebrew (`brew install workset`) or `go install`. See [Getting Started](getting-started.md) for details.

## In-app updates (macOS)

The desktop app supports a custom updater flow from **Settings → About**:

- Choose an update channel (`stable` or `alpha`).
- Click **Check for Updates**.
- If a newer version exists, click **Update and Restart**.

Update manifests are fetched from `WORKSET_UPDATES_BASE_URL` when set, otherwise from `https://strantalis.github.io/workset/updates`. The updater requires both SHA256 match and matching macOS signing Team ID from the manifest.

<details markdown>
<summary>Terminal settings</summary>

- `defaults.terminal_idle_timeout` controls idle shutdown for GUI terminals.
- `defaults.terminal_protocol_log` enables sessiond protocol logging (restart daemon to apply).
- `defaults.terminal_debug_overlay` shows the terminal debug overlay.
- `defaults.agent` controls the generator used for PR/commit text and the default coding agent for terminals (supported values: `codex`, `claude`).
- `defaults.agent_model` optionally overrides the model used for PR/commit text generation (terminal launcher is unaffected).

See [Config](config.md#defaults) for the full field reference.

</details>
