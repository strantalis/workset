# Workset Desktop (Tauri) Product Specification (Working)

This is a working product specification. We are drafting it one section at a time.

---

## Section 1: Product Mental Model (What This App Is)

### The problem we’re solving
Workset is a desktop app for doing one piece of work across many repositories without losing track of what belongs together. The core pain is that a “feature” or “thread” of work almost never lives in one repo, but git and most UIs force you to think repo-by-repo.

This product treats a multi-repo feature as a first-class thing you can create, switch, and review as a single unit.

### The three key concepts (and only these are “real”)

#### 1) Workset (app concept, not a CLI capability)
A **Workset** is a saved configuration: “these are the repos that belong together for this product/project.”

- It is durable and user-named.
- It contains a curated list of repos (and optional defaults).
- It is not a work thread by itself.
- Creating a Workset is an app responsibility (UI + persistence). The CLI does not “create worksets”.

You can think of Workset as: a profile / recipe / config bundle.

#### 2) Workspace (CLI-backed concept: the work thread)
A **Workspace** is a named work thread inside a Workset: “feature/search-ranking”, “bugfix/login-timeout”, etc.

- A Workspace is what you create when you want to start a new multi-repo thread.
- A Workspace is always scoped to exactly one Workset (because it uses that Workset’s repo set).
- A Workspace has one name, and that name is used to coordinate work across repos.

You can think of Workspace as: a feature branch identity applied to the whole repo set.

#### 3) Branch (git concept, per repo)
A **branch** is still a per-repository git branch. The product goal is to make the user not have to manage these manually per repo.

### The key semantic: what “Create Workspace” actually does
When the user creates a Workspace named `X` under the active Workset, the system provisions the Workset’s repositories so that:

- Each repo ends up on a branch named `X` (or whatever exact naming/mapping the CLI defines).
- The user now has a coherent cross-repo “thread” they can switch to later by selecting Workspace `X`.
- The “unit of switching” in the UI is Workspace, not repository.

If one repo fails to provision, the Workspace creation is not “done”; it is a partial failure state with a repair path.

### Global selection rules (the invariants)
- There is exactly one active Workset at a time (selected in the top chrome).
- Everything the user sees is scoped to that Workset (repos, workspaces, diffs, agents).
- There is exactly one active Workspace at a time within that Workset.
- Workspace selection is not a second top-chrome selector; it is controlled within the Workset-scoped UI.

### What the UI must never do
- Show Workspaces from multiple Worksets at the same time.
- Treat “branch” and “workspace” as separate user-facing primary concepts. Branches are an implementation detail unless the user explicitly drills into repo details.
- Implicitly change the active Workset as a side effect of creating/selecting a Workspace.

### Role of the CLI (what the app delegates)
- The CLI defines and performs the actual multi-repo git operations for Workspace lifecycle (create/switch/provision/status).
- The app wraps those operations with:
  - UX (progress, per-repo status, retries)
  - persistence of Workset config
  - safe error reporting and recovery
  - higher-level views like “what changed in this Workspace across repos”

---

## Section 2 (Revised): Information Architecture (Native Agent Tabs + No Review Page)

This section removes the “Review” page and replaces it with an in-Space diff + diff-as-tab model, and clarifies that Spaces main content is primarily the terminal/agent surface.

### 2.1 Global chrome (top bar)
Top chrome is reserved for global context only.

Top chrome contains:
- **Workset selector** (the only persistent selector)
  - pick/switch Workset
  - “Create Workset…” (app-level)
  - “Manage Worksets…” (rename/delete/export/import)
- Lightweight global items (optional):
  - search/command palette
  - app settings shortcut
  - connection/health indicator

Top chrome explicitly does **not** contain:
- a Workspace selector
- repo selection controls
- branch controls
- diff controls

### 2.2 Primary navigation (left sidebar)
Primary nav is stable and Workset-scoped.

Primary nav items:
1. **Command Center**
2. **Spaces**
3. **Settings**

Removed:
- **Review** (no separate page)

Rationale: review/diff is an always-available right panel capability of the active Space, and detailed diffs become tabs in the main terminal area.

### 2.3 Page layout model
All pages share:

`[Primary Sidebar] [Secondary Sidebar] [Main Content] [Right Panel]`

For **Spaces** specifically:
- **Main Content** = the tabbed terminal/agent environment (like iTerm)
- **Right Panel** = lightweight diff summary + file list (not a full review app)

### 2.4 Command Center (Workset config + health)
Command Center is for Workset configuration and diagnostics, not daily work.

Secondary sidebar:
- Overview
- Repositories (add/remove repos in this Workset config)
- Diagnostics (CLI/env/auth checks, logs, debug bundle)

Main content:
- Workset overview cards (repo count, workspace count, warnings)
- Primary CTAs: “Add repositories”, “Go to Spaces”, “Create workspace”

Right panel (optional):
- Recent operations / failures / retry shortcuts

### 2.5 Spaces (the daily driver)
Spaces is where you create/select the active Workspace and then live inside it (agents + terminals + diffs).

#### 2.5.1 Secondary Sidebar (Spaces): Workspace list + lifecycle
- Header: “Workspaces”
- Action: “New Workspace”
- List: Workspaces (scoped to active Workset)
  - each shows: name, status (Ready / Partial / Error), last used
  - selecting sets active workspace

This is the canonical place to switch work threads.

#### 2.5.2 Main Content (Spaces): Tabbed “iTerm-like” surface
Main content is a tab strip plus active tab content.

Tab types (minimum):
- **Agent tab**: a native coding agent session attached to this Workspace context
- **Terminal tab**: plain shell terminal (tests/builds/manual git, etc.)
- **Diff tab**: full diff viewer for a selected file (opened from right panel)

Tab strip behavior:
- Create tab buttons: “New Agent”, “New Terminal”
- Tabs are per-workspace (when you switch workspace, you see that workspace’s tabs)
- Tabs can be renamed (“agent-1”, “tests”, “diff: server/api.go”, etc.)
- Tabs can be closed; closing should stop/cleanup that underlying session (with confirm if running)

#### 2.5.3 Right Panel (Spaces): Lightweight diff summary
Right panel is always visible while in Spaces (unless user collapses it).

It shows a compact, scannable summary:
- Grouped by repo:
  - repo name
  - changed files under it
  - each file shows `+adds/-deletes` (green/red) and status (M/A/D)
- Optional top summary: total files changed, total `+/-`

Interaction:
- Clicking a file opens a Diff tab in main content (does not navigate away)

Constraints:
- This right panel is not a PR tool.
- It is meant to keep you oriented (“what changed in this workspace?”) and to open diffs quickly as tabs.

### 2.6 Empty states + failure states (updated)
1) No Worksets:
- onboarding: create first Workset (top chrome flow)

2) Workset has no repos:
- Command Center pushes “Add repositories”

3) Workset has repos but no workspaces:
- Spaces shows “Create your first workspace” CTA

4) Workspace exists but provisioning is partial:
- Workspace list marks it “Needs repair”
- Right panel shows only available diff data (or a clear “diff unavailable until provisioned” message)
- Main content shows repair CTA and per-repo failure details (in a “Workspace status” tab or a panel)

---

## Section 3: Runtime Model (Tabs, Processes, and State Ownership)

This section is grounded in what the existing Wails app already proves works well: a dedicated PTY daemon (`workset-sessiond`), per-workspace persisted terminal layout (tabs + splits), and a background diff watcher that keeps a repo-grouped `+/-` file list fresh in the right panel.

### 3.1 The core runtime principle
In **Spaces**, the main content is a terminal surface that can host:
- interactive agent sessions (Claude Code running in a terminal)
- plain terminals (tests/builds/manual commands)
- full file diffs (opened as tabs)

Runtime is “many terminal-like sessions + lightweight diff data,” all scoped to the active Workspace.

### 3.2 Reuse the proven PTY architecture (sessiond)
Do not implement PTYs ad-hoc in the UI layer. The Wails app uses a daemon (`workset-sessiond`) that provides:
- attach/stream semantics (session stays alive even if UI detaches)
- backlog + snapshot (so the UI can recover after refresh/restart)
- backpressure/credit accounting (UI ACKs bytes to prevent runaway memory)
- terminal mode signals (alt-screen, mouse tracking)
- optional Kitty graphics event stream

Spec requirements (Tauri):
- The Tauri backend must manage terminal/agent sessions via `workset-sessiond` (or an equivalent daemon with the same features).
- A “Terminal tab” and an “Agent tab” are the same underlying thing: a PTY session with a different startup command.

### 3.3 What a “tab” is (concrete definition)
A tab is a view over a single underlying runtime session or a diff artifact.

Tab types:
1. **Agent tab**
   - Backed by a sessiond PTY
   - Starts by launching the agent command (Claude Code) inside the workspace root
2. **Terminal tab**
   - Backed by a sessiond PTY
   - Starts a shell (or configured command) inside the workspace root
3. **Diff tab**
   - Not a PTY
   - Displays a patch for one file (or optionally a repo-level patch)
   - Opened from the right-panel diff navigator

Non-negotiable UX behavior:
- Tabs are per-workspace. Switching workspace swaps the whole tab set.
- Closing a PTY-backed tab stops that underlying session (with confirmation if needed).
- Diff tabs are disposable and should restore quickly from cached patch data (or refetch).

### 3.4 Terminal layout model (tabs + splits) and persistence
The Wails app uses a layout tree with:
- panes containing tabs
- split nodes (row/column) with ratios
- active tab per pane
- focused pane

Spec requirements (Tauri):
- Persist a layout per workspace context (keyed by workspace path, not just a name).
- On app launch:
  - load the saved layouts
  - opportunistically restore PTY sessions referenced by the layout (start/attach)
  - show tabs as “reconnected” vs “fresh” if that concept exists

### 3.5 Workspace root and repo worktrees (no guessing)
The Wails app resolves:
- workspace root path from the underlying service (workspace list includes `Path`)
- repo worktree paths as `workspacePath + repo.RepoDir` (with fallback to a local path)

Spec requirements (Tauri):
- The backend must obtain the workspace root path and repo worktree paths from the same source of truth the CLI uses (service/CLI output).
- Terminals and agents must start with `cwd = workspace root path`.
- Repo diffs must be computed against the repo worktree path.

### 3.6 Right-panel diffs: simple, always-on, event-updated
The right panel is a compact diff navigator:
- grouped by repo
- list of files with `+added/-removed`
- clicking a file opens a Diff tab

Spec requirements (Tauri):
- Maintain a background “diff watcher” per (workspace, repo) while that workspace is active.
- The watcher emits updates that drive the right panel (so it’s “live” without manual refresh).
- The right panel is not a PR tool; it’s navigation and awareness.

### 3.7 Diff tabs: file-level patch as the primitive
The existing backend provides:
- repo diff summary (files + totals)
- file diff patch (tracked + untracked)

Spec requirements (Tauri):
- Clicking a file opens a Diff tab for that exact file entry (path + status + prevPath if rename).
- The Diff tab fetches a patch using the same approach:
  - tracked: `git diff` staged + unstaged for that file
  - untracked: `--no-index` diff against empty baseline
- Optional: “next/prev changed file” navigation driven by the current right-panel list.

### 3.8 Auth/runtime environment is a first-class dependency
Spec requirements (Tauri):
- Provide an explicit “Reload login environment” capability.
- All spawned PTY sessions must inherit:
  - PATH resolution consistent with login shell
  - SSH agent socket (for git SSH)
  - `gh` auth context (if used)
  - git credential helper behavior

### 3.9 Failure handling: make it boring and recoverable
Minimum recovery requirements:
- If sessiond is down: show clear status + “restart sessiond”; terminals show “disconnected” not “loading forever”.
- If a repo is missing/unavailable: right panel shows “diff unavailable” with reason.
- If workspace provisioning is partial: Spaces shows “needs repair” and per-repo remediation.

---

## Section 4: Backend Contract + Persistence (Tauri Commands, Events, and Local Storage)

This section defines the interface between the Tauri frontend and backend, plus what is persisted locally vs derived from Workset (CLI/service) state. The goal is to make Sections 1–3 implementable without ambiguity.

### 4.1 Design goals for the contract
- Deterministic UI state transitions (no infinite loading).
- Long-running operations report progress (workspace create, repo membership changes).
- Streaming IO for PTY sessions (agent/terminal tabs) with backpressure and replay.
- Background diff updates for the right panel (watcher/event model).
- Structured errors with remediation hints.

The frontend must not parse raw CLI output strings; it calls typed backend commands and consumes typed payloads/events.

### 4.2 Sources of truth
There are four “engines” in the product:

1) App-local Workset profile store (app-owned)
- Workset profiles (name + repo sources + defaults).
- Active selection state.
- UI layout/tab persistence keyed by workspace path.

2) Workset workspace state (CLI-owned)
- A workspace is a directory with `<workspace>/workset.yaml` and `<workspace>/.workset/`.
- Repo membership in a workspace is modified by `workset repo add/rm`.
- Worktrees live under `<workspace>/<repo_dir>` by default.

3) `workset-sessiond` (PTY daemon, runtime-owned)
- Terminal/agent sessions: attach/stream/bootstrap/ack, snapshot/backlog, modes.

4) Derived git data (computed against repo worktrees)
- Diff summaries and file patches are computed against repo worktree paths for the active workspace.

### 4.3 Local persistence (what the app stores)
The app persists:

1) Workset profiles (app concept)
- `id` (uuid)
- `name`
- `repos[]` (repo sources as entered, plus display metadata)
- optional defaults (remote/base branch, workspace root, etc.)
- timestamps

2) UI context
- active workset id
- last active workspace per workset
- panel collapsed state, etc.

3) Workspace-scoped layout state (keyed by workspace path)
- split/pane layout tree + tab list + active tab id + focused pane id
- tab titles

4) Workset membership migration history (recommended)
- records of “repo added/removed” events and per-workspace results (success/failure + error envelope)

### 4.4 Non-negotiable behavior: Workset membership converges all workspaces
You decided:
- Adding a repo to a Workset profile applies to all existing workspaces under that workset immediately.
- Removing a repo from a Workset profile applies to all existing workspaces under that workset immediately.

This means repo membership changes are not passive config edits; they start a long-running “migration job”:

- Add repo: run `workset repo add -w <workspace> <source> ...` for each workspace.
- Remove repo: run `workset repo rm -w <workspace> <name> ...` for each workspace.

There is no stable “Not provisioned” state. Instead, per workspace:
- `updating` (pending/running)
- `up_to_date` (succeeded)
- `update_failed` (failed; repairable with retry)

### 4.5 Tauri backend command surface (proposed)
The contract is grouped by capability. Command names are placeholders; the important part is payload shape and event semantics.

#### Group 1: Workset profiles (app-owned)
- `worksets.list() -> WorksetProfile[]`
- `worksets.create({ name, defaults? }) -> WorksetProfile`
- `worksets.update({ id, name?, defaults? }) -> WorksetProfile`
- `worksets.delete({ id }) -> void`

Repo membership changes:
- `worksets.repos.add({ workset_id, source, display_name? }) -> { workset: WorksetProfile, job: MigrationJobRef }`
- `worksets.repos.remove({ workset_id, repo_id, remove_options }) -> { workset: WorksetProfile, job: MigrationJobRef }`

`remove_options` must include:
- `delete_worktrees: boolean` (default true)
- `delete_local: boolean` (default false; only if the repo is managed)

#### Group 2: Active context
- `context.get() -> { active_workset_id?: string, active_workspace?: string }`
- `context.set_active_workset({ workset_id }) -> void`
- `context.set_active_workspace({ workspace_name }) -> void`

#### Group 3: Workspaces (CLI-backed)
- `workspaces.list({ workset_id }) -> WorkspaceSummary[]`
- `workspaces.create({ workset_id, name, path? }) -> WorkspaceCreateJobRef`
- `workspaces.create.status({ job_id }) -> WorkspaceCreateProgress`
- `workspaces.delete({ workset_id, workspace_name, delete?: boolean }) -> void` (optional)

Workspace creation should internally use `workset new` and then converge workspace repos to the workset profile.

#### Group 4: Migration jobs (repo membership apply-to-all)
- `migration.status({ job_id }) -> MigrationProgress`
- `migration.cancel({ job_id }) -> void`
- `migration.retry_failed({ job_id }) -> MigrationJobRef`
- `migration.retry_workspace({ job_id, workspace_name }) -> MigrationJobRef`

Migration progress must include:
- job state: `queued|running|done|failed|canceled`
- per-workspace state: `pending|running|success|failed` + error envelope

#### Group 5: Workspace repo resolution (CLI/service-backed)
- `workspace.repos.list({ workspace_name }) -> RepoInstance[]`

RepoInstance must include:
- repo identity (name or stable id)
- `worktree_path`
- `repo_dir`
- missing/statusKnown flags
- default branch/remote if available

#### Group 6: Diff summary + file patch
- `diff.summary({ workspace_name, repo }) -> DiffSummary`
- `diff.file_patch({ workspace_name, repo, path, prev_path?, status }) -> FilePatch`

#### Group 7: Diff watcher (background)
- `diff.watch.start({ workspace_name, repo }) -> void`
- `diff.watch.stop({ workspace_name, repo }) -> void`

Events:
- `diff:summary` `{ workspace_name, repo, summary }`
- `diff:status` `{ workspace_name, repo, statusKnown, missing, dirty }`

#### Group 8: PTY sessions (sessiond-backed)
- `pty.create({ workspace_name }) -> { terminal_id }`
- `pty.start({ workspace_name, terminal_id, kind, command? }) -> void` where `kind` is `terminal|agent`
- `pty.write({ workspace_name, terminal_id, data }) -> void`
- `pty.resize({ workspace_name, terminal_id, cols, rows }) -> void`
- `pty.ack({ workspace_name, terminal_id, bytes }) -> void`
- `pty.bootstrap({ workspace_name, terminal_id }) -> BootstrapPayload`
- `pty.stop({ workspace_name, terminal_id }) -> void`

Events:
- `pty:data`
- `pty:bootstrap`
- `pty:bootstrap_done`
- `pty:modes`
- `pty:lifecycle`
- `pty:kitty` (optional)

#### Group 9: Diagnostics
- `diagnostics.env_snapshot() -> EnvSnapshot`
- `diagnostics.reload_login_env() -> EnvSnapshot`
- `diagnostics.sessiond.status() -> SessiondStatus`
- `diagnostics.sessiond.restart({ reason }) -> SessiondStatus`

### 4.6 Error envelope (required)
Every failed operation must include:
- `category`: `auth | network | git | config | runtime | unknown`
- `operation`
- `message`
- `details`
- `retryable`
- `suggested_actions[]` (optional)

---

## Section 5: Workspace Provisioning UX + Progress Model (Create, Partial Failure, Repair)

This section defines how “Create Workspace” works in Spaces, and how we handle partial failures. It also defines the repo membership migration model (apply-to-all) for add/remove of repos in a Workset profile.

### 5.1 User intent
When a user creates a workspace named `X` under the active workset, they expect:
- a coherent multi-repo thread named `X`
- all repos in the workset profile are present in that workspace directory and usable
- they can immediately open agent/terminal tabs and start work

### 5.2 Where it lives
Spaces is the canonical home for:
- create workspace
- list/select workspace
- see provisioning/migration status
- repair failures (retry)

### 5.3 Create Workspace modal
Inputs:
- workspace name (required)
- optional base branch override (only if supported)

Preview:
- list of repos in the workset profile that will be included

Primary action:
- Create

### 5.4 Workspace creation is a job
Workspace creation must be non-blocking and progress-reporting.

Job states:
- `queued -> running -> succeeded | partial | failed`

Per-repo states:
- `pending | running | succeeded | failed`

Per-repo step labels (UI-facing):
- `preflight`
- `fetch/clone`
- `worktree/checkout`
- `verify`

### 5.5 When the new workspace appears
Immediately after job start:
- it appears in the workspace list as `Provisioning`
- it can be selected (UI shows provisioning state)

### 5.6 Tabs availability during provisioning
Default rule:
- allow agent/terminal tabs once the workspace path exists and at least one repo worktree is available
- otherwise show disabled state with a clear reason

### 5.7 Completion semantics
Outcomes:
- `Ready`: all repos succeeded
- `Needs repair`: at least one repo failed (workspace is partial)
- `Failed`: provisioning did not establish a usable workspace context

### 5.8 Repair model
Repair is a first-class action:
- Retry failed repos (bulk)
- Retry a specific repo
- View details (error envelope + logs)
- Open diagnostics (auth/runtime)

Repair must not require deleting/recreating a workspace.

### 5.9 Workset repo membership changes are immediate migrations (apply-to-all)
When a repo is added/removed from the active workset profile, the app must converge all existing workspaces under that workset.

Add repo:
- start a migration job that runs `workset repo add -w <workspace> <source> ...` for each workspace

Remove repo:
- start a migration job that runs `workset repo rm -w <workspace> <name> [--delete-worktrees] [--delete-local]` for each workspace
- UI must require a confirmation and expose the destructive options:
  - delete worktrees (default true)
  - delete local clones (default false; only for managed repos)

Per-workspace migration states (no stable “Not provisioned”):
- `updating` (pending/running)
- `up_to_date` (succeeded)
- `update_failed` (failed; retryable per workspace)

### 5.10 Error categories and remediation
Errors must map cleanly to actions:
- `auth`: show steps (ssh agent, gh auth, credential helper)
- `network`: retry + offline hints
- `git`: conflicts, dirty state, worktree conflicts
- `config`: invalid repo source, repo missing
- `runtime`: CLI/sessiond unavailable, PATH issues

### 5.11 Success UX
On success:
- auto-select the new workspace (recommended)
- open default tabs (1 agent, 1 terminal)
- start diff watchers for all repos so the right panel populates quickly

---

## Section 6: Diff UX + Diff Tab Rendering (Right Panel + File Diffs as Tabs)

This section defines the diff experience inside Spaces:
- the right panel is a compact “diff navigator”
- clicking a file opens a full diff as a tab in the main terminal area

There is no separate Review page.

### 6.1 Scope: what we are diffing (v1)
In v1, diffs represent local workspace changes in each repo worktree:
- unstaged changes
- staged changes
- untracked files (rendered via no-index diff against an empty baseline)

We are not building PR creation/review tooling in this diff surface.

### 6.2 Right panel: information density and hierarchy
The right panel is always visible in Spaces (collapsible).

Structure:
- Panel header: `Diff` + totals (files changed, total `+/-`)
- Repo groups (one per repo in the active workspace):
  - repo name
  - repo total `+/-`
  - optional repo status badges: `missing`, `dirty`, `status unknown`
  - file list (changed files only)

File row fields:
- path (display relative to repo root)
- status: `M/A/D/R` (icon or short label)
- `+added/-removed` (green/red)

### 6.3 Right panel: interactions
Required behaviors:
- Clicking a repo header expands/collapses its file list.
- Clicking a file row opens a Diff tab for that file (see 6.5).
- If a Diff tab for that file is already open, focus it instead of opening a duplicate.

Optional behaviors (nice to have):
- Search filter in the diff panel (substring match on path).
- Toggle: “Only changed repos” (default on).

### 6.4 Right panel: ordering rules (v1 defaults)
Repo group order:
- repos with changes first
- then alphabetical by display name

File order within a repo:
- directories first, then files (path sort) OR plain path sort
- stable ordering is more important than clever ordering

### 6.5 Diff tabs: what opens when a file is clicked
A Diff tab is created with:
- repo identity
- file identity: `path`, `prev_path` (if rename), `status`
- a snapshot of the file’s `+/-` stats at open time

Tab title:
- default: `diff: <path>` (truncate middle if long)
- tab subtitle/tooltip shows `<repo> • <path>`

The tab content shows:
- repo + path header with status badge
- `+/-` summary
- unified diff view (v1)

### 6.6 Diff rendering component (frontend)
Use a dedicated diff renderer component that can:
- render unified diff with syntax highlighting
- handle renames (show old/new path)
- handle deletes (no new content)
- handle binary (no textual diff)
- display “truncated” state when backend truncates patch

Implementation note:
- The existing Wails app already uses `@pierre/diffs` for rendering; a Tauri rewrite can reuse the same renderer strategy.

### 6.7 Patch source of truth (backend)
The backend must provide two primitives per (workspace, repo):

1) Summary:
- changed file list + totals (for right panel)

2) File patch:
- patch for a single file (for diff tabs)

Rules:
- Summary is produced by a background watcher while workspace is active.
- File patches are fetched on-demand when a diff tab is opened (and on refresh).

### 6.8 Keeping an open diff tab consistent with watcher updates
When watcher updates arrive for a repo:
- update the right panel immediately
- do not forcibly re-render any open Diff tab content (avoid scroll jumps)

If the currently open file’s stats changed since the tab opened:
- show a small “Out of date” indicator in the tab header
- provide a `Refresh diff` action to refetch and rerender the patch

### 6.9 Large diffs and truncation policy
Backends often need to truncate large patches. The UI must make this obvious and recoverable.

If the backend reports `truncated: true`:
- show “Diff truncated” warning
- provide actions:
  - `Open file in editor`
  - `Open repo in Finder`
  - optional: `Copy patch`

The right panel should also cap file lists to a reasonable limit:
- show first N files and display “+ N more” (to avoid UI stalls).

### 6.10 Binary files
If the backend reports `binary: true`:
- do not attempt to render a textual diff
- show a “Binary file” state and provide actions:
  - `Open file`
  - `Reveal in Finder`

### 6.11 Missing repos and unknown status
If a repo worktree path is missing/unavailable:
- right panel shows the repo with a `missing` badge
- file list is empty with a clear message (“Repo path unavailable in this workspace”)
- clicking the repo shows actions: `Open diagnostics`, `Retry migration/provisioning` (if applicable)

### 6.12 Staged vs unstaged (v1 presentation)
In v1, file patches may combine staged + unstaged into a single unified patch (simpler).

If we later want more detail:
- add a toggle in Diff tab: `All | Staged | Unstaged`
- but keep the right panel summary aggregated (still one line per file).

---

## Section 7: Settings + Diagnostics (Make Runtime + Auth Boring)

This section exists because the app’s “real work” depends on the user’s local environment being correct:
- git authentication (SSH keys, credential helpers, `gh` auth)
- CLI availability and versioning
- `workset-sessiond` health for PTYs
- file system access for workspaces, repo worktrees, and diff watchers

The UI must make these dependencies visible and fixable without guesswork.

### 7.1 Settings scope and IA
Settings is a first-class page (primary nav), but it should be intentionally small.

Settings is split into two categories:
1) **App Settings** (global)
2) **Workset Settings** (per workset profile, app-owned config)

Diagnostics is accessible in two places:
- Settings (global diagnostics and tools)
- Command Center → Diagnostics (workset-scoped diagnostics and quick fixes)

### 7.2 App Settings (global)
Minimum global settings:
- Default shell / command for “New Terminal” tabs
- Default agent command for “New Agent” tabs (e.g. `claude`)
- Workspace root location (where workspace directories are created, if the app chooses the path)
- Diff defaults:
  - max patch bytes (soft cap, UI-friendly)
  - file list cap per repo in diff navigator
- Telemetry (if any) with explicit opt-in/out

Important: These settings should not contain “workset repo list” or workspace lifecycle. Those belong to Workset config and Spaces.

### 7.3 Workset Settings (per-workset profile)
Minimum per-workset settings:
- Workset name (rename)
- Repo list (add/remove; also available via Command Center)
- Defaults that affect workspace creation/provisioning:
  - optional “base branch” preference (if supported by CLI semantics)
  - optional “default remote” preference

Hard rule: Workset creation/editing is app-owned and must not pretend to be a CLI feature.

### 7.4 Diagnostics: what we must show (always)
Diagnostics must have a single “Status” view that answers:
- Can the backend run `workset` successfully?
- Can the backend reach `workset-sessiond` and create/attach PTYs?
- Does the process environment look like a login environment (PATH, HOME, SHELL)?
- Can git authenticate to a private GitHub repo using the user’s normal setup?

Displayed as:
- Green/Yellow/Red rows with a one-line explanation and “Fix” action(s).

### 7.5 Diagnostics: environment snapshot + reload
The app must support:
- `View environment snapshot`
- `Reload login environment`

This is not a “nice-to-have”: it is the primary escape hatch when the app is launched in an environment that doesn’t match the user’s shell (common on macOS).

Environment snapshot should show (read-only):
- `PATH`
- `SHELL`
- `HOME`
- `SSH_AUTH_SOCK` (presence and value)
- `GIT_SSH_COMMAND` (if set)
- `GIT_ASKPASS` (if set)
- `GH_CONFIG_DIR` (if set) and `gh auth status` summary (redacted)

Reload should:
- rehydrate env from the login shell (backend responsibility)
- restart any newly created PTY sessions with the updated env
- not silently mutate running tabs; show a banner: “New sessions use updated environment.”

### 7.6 Git/GitHub auth checks (opinionated, actionable)
Diagnostics should include explicit checks and fixes for:
- SSH:
  - key present (do not display private key paths)
  - agent socket present (`SSH_AUTH_SOCK`)
  - quick test: `ssh -T git@github.com` (run as an explicit diagnostic action, not on every launch)
- `gh`:
  - `gh auth status` (for github.com)
  - `gh` present in PATH
- Git credential helper:
  - show configured helper (`git config --global credential.helper` and/or system helper on macOS)

When operations fail with auth errors, surface remediation suggestions that match the error category:
- “Repo URL uses HTTPS; switch to SSH URL” (if the repo source is HTTPS and user expects SSH)
- “Reload login environment” (if PATH/SSH_AUTH_SOCK mismatch)
- “Open GitHub auth” (if gh is missing auth)

### 7.7 workset CLI health + versioning
Diagnostics should show:
- `workset` binary path and version
- ability to run `workset ls` (or equivalent non-destructive command)
- ability to run workspace-scoped commands against the selected workspace

If the CLI is missing or outdated:
- show clear action: “Install/Update Workset CLI” with a link/instructions (do not auto-install)

### 7.8 sessiond health + recovery
Because PTY tabs depend on `workset-sessiond`, diagnostics must provide:
- status: running/not running, version (if available), socket path (if applicable)
- `Restart sessiond` action
- last error and last restart time (if tracked)

If sessiond is down:
- terminal/agent tabs show a disconnected state with a “Reconnect” and “Restart sessiond” CTA
- never show “loading forever”

### 7.9 Workspace-scoped diagnostics (in Command Center)
When a specific workset/workspace is selected, Command Center diagnostics should add:
- workspace path exists + is writable
- repo worktree paths exist for each repo
- migration status (if a repo add/rm migration is running or failed)
- diff watcher status per repo

Provide “Fix” actions:
- Retry failed migration for a workspace
- Restart diff watchers
- Reveal workspace folder

### 7.10 Debug bundle (supportability)
Add “Export debug bundle” that produces a zip containing:
- redacted diagnostics snapshot (no tokens, no private keys)
- recent backend logs
- recent sessiond logs (if accessible)
- migration job history (app-local) and last error envelopes
- UI layout state (for reproduction)

This bundle is for humans and future agents to debug issues quickly.

---

## Section 8: Scope Boundaries + Phased Delivery Plan (Build the Core First)

This section prevents scope creep and defines an implementation sequence that de-risks the two hardest parts early:
- durable terminals/agents (PTY + sessiond)
- workspace/repo provisioning and migrations (CLI-backed, long-running jobs)

### 8.1 Non-negotiable outcomes (what “v1 works” means)
The product is “real” when a user can:
1) Create a Workset (app-owned config) and add a few repos.
2) Create a Workspace (CLI-backed) for that Workset, see progress, recover from partial failure.
3) Open multiple terminal/agent tabs within that Workspace and have them survive detach/reattach.
4) See a per-repo diff navigator and open file diffs as tabs.
5) Add/remove repos on the Workset and have that apply to all existing Workspaces via migration jobs.

### 8.2 Explicit non-goals (out of scope for v1)
To avoid turning this into “GitHub Desktop + an IDE + a PR tool”, v1 does not include:
- PR creation, PR review, inline comments, approvals, or GitHub API integration.
- “Branch management UI” beyond Workspace semantics; user is not managing per-repo branches directly.
- Merge/conflict resolution UI. (If `workset`/git reports conflicts, we surface the error and provide a terminal.)
- Repo browsing, file tree editing, or a built-in code editor.
- LSP features, symbol search, or deep IDE-like navigation.
- Multi-user collaboration, sharing worksets, or remote sync of app-owned configs.
- Secrets storage: no storing SSH keys, GitHub tokens, or credential helper secrets inside the app.

If a feature is proposed that overlaps these, it must be justified against the v1 outcomes above.

### 8.3 Source-of-truth boundaries (what owns what)
This is a boundary contract, not a suggestion:
- Workset profiles (workset name + repo sources) are **app-owned** and persisted by the app.
- Workspace lifecycle and repo membership inside a workspace are **CLI-owned** operations.
- Terminals/agents are **sessiond-owned** runtime sessions that the app attaches to.
- Diffs are **derived** from repo worktrees and updated via watcher events.

If UI behavior and CLI behavior appear to conflict, treat it as a design issue:
- surface the mismatch explicitly
- decide whether we change UI semantics or add a new CLI affordance
- do not “paper over” with silent heuristics

### 8.4 App structure (Tauri shape, at a product level)
The Tauri app is split into three cooperating systems:
1) **Frontend** (web UI): Workset selector, Spaces, tabs, diff navigator, settings/diagnostics.
2) **Backend** (Tauri Rust): typed command/event API, job engine, file watchers, orchestrates CLI + sessiond.
3) **External tools**:
   - `workset` CLI for workspace/repo operations
   - `workset-sessiond` for PTYs (terminals/agents)
   - `git` for diff computation (or backend wrappers over git)

The backend is responsible for ensuring the UI never “hangs loading forever”.

### 8.5 Phased delivery plan (recommended)
The goal is to get a working vertical slice quickly, then harden and expand.

#### Phase 0: Skeleton + persistence
Deliverables:
- Workset selector in top chrome (list/create/rename/delete).
- Local persistence for workset profiles and active workset selection.
- Empty states for “no worksets”, “no repos”, “no workspaces”.

Acceptance criteria:
- App can restart and preserve active workset selection and workset list.

#### Phase 1: Workspace list + create job (CLI-backed)
Deliverables:
- Spaces: workspace list for active workset.
- “New Workspace” flow triggers a create job with progress and per-repo status.
- Partial failure state + retry.

Acceptance criteria:
- A workspace can be created successfully with multiple repos.
- If one repo fails, the UI shows a repair path and does not lose the workspace.

#### Phase 2: sessiond integration (PTY tabs)
Deliverables:
- Tab strip in Spaces with “New Terminal” and “New Agent”.
- PTY session lifecycle and streaming via sessiond attach/bootstrap/ack.
- Layout persistence keyed by workspace path (tabs at minimum; splits optional in v1).

Acceptance criteria:
- Open 2+ agent/terminal tabs; switch away and back; tabs reconnect without losing output.
- No more “WebSocket HMR” dependency (native app runtime must be stable).

#### Phase 3: Diff navigator + diff tabs (derived state)
Deliverables:
- Right panel diff navigator: repo-grouped file list with `+/-`.
- Background diff watchers for active workspace repos.
- Clicking a file opens a diff tab; diff tabs support refresh and large/binary states.

Acceptance criteria:
- Editing a file updates the right panel shortly after.
- Clicking a file shows a readable diff and does not freeze the UI for large repos.

#### Phase 4: Workset repo membership migrations (apply-to-all)
Deliverables:
- Add repo to workset profile triggers migration job across all workspaces.
- Remove repo triggers migration job across all workspaces (with confirmation + options).
- Per-workspace migration status and retry tooling.

Acceptance criteria:
- Add a repo and observe it appear in every existing workspace (or show per-workspace failure + retry).
- Remove a repo and observe it removed from every existing workspace (or show per-workspace failure + retry).

#### Phase 5: Diagnostics + debug bundle (operational polish)
Deliverables:
- Settings + Diagnostics views per Section 7.
- “Reload login environment”, sessiond status/restart, CLI status/version, auth checks.
- Export debug bundle (redacted).

Acceptance criteria:
- Common auth/env failures produce actionable diagnostics rather than vague errors.

### 8.6 Performance + safety constraints (must be enforced)
Constraints:
- Do not compute full diffs for all repos on every UI render; use watchers and caching.
- Cap patch size and file list sizes to avoid UI stalls; show truncation explicitly.
- Never store secrets; redact tokens from logs and debug bundles.
- All destructive operations (repo removal with deletions) require explicit confirmation and defaults must be conservative.

### 8.7 Migration strategy (from the existing Wails app)
This rewrite is a product restart, but it should still reuse what we already know:
- session durability semantics from `workset-sessiond`
- layout persistence keyed by workspace path
- diff watcher event model
- error envelopes and “loading never” avoidance

The goal is not to reproduce the old UI; it is to preserve the proven runtime behaviors while implementing the new product IA.
