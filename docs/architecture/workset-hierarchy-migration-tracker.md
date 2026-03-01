# Workset Hierarchy Migration Tracker

Last updated: 2026-02-27

## Goal

Transition from legacy `template`-centric workspace grouping to `workset` as the primary hierarchy concept, without a full frontend rewrite.

## Current State (Verified)

- Dev mode uses isolated config at `~/.workset/dev/config.yaml` via `wails-ui/workset/dev_paths_dev.go`.
- App startup calls `ensureDevConfig()` and `GetConfig()`, so config migrations on `loadGlobal()` run in dev.
- Before this update, there was no migration that backfilled missing `workspaces[*].workset`.
- Result: existing entries without `workset` remained legacy-shaped and frontend fell back inconsistently.

## Implemented in This Pass

- Config schema now persists as `worksets:` (legacy `workspaces:` still read for migration compatibility).
  - `internal/config/types.go`
  - `internal/config/defaults.go`
  - `internal/config/global.go`
  - `internal/config/global_update.go`
- Added `workset_catalog` as explicit hierarchy data:
  - workset -> repos + feature threads.
  - populated from existing workspace refs/workspace configs during migration.
- Added global-config migration for workspace workset backfill:
  - `pkg/worksetapi/migrations.go`
  - For each workspace with empty `workset`:
    - use `template` when present;
    - otherwise derive from workspace repos (`repo-a`, `repo-a + repo-b`, etc.);
    - fallback to workspace name if repo config is unavailable.
  - Persists changes through `ConfigUpdater` (`UpdateGlobal`) when available.
- Wired migration into config load path:
  - `pkg/worksetapi/helpers.go` (`Service.loadGlobal`).
  - Legacy `workspaces:` key now triggers a write-back to canonical `worksets:` on load.
  - This runs through the same `GetConfig()` path Wails startup already calls.
- Legacy `groups` is dropped during the legacy-key rewrite path so migrated config no longer shows template groups.
- Ensured newly created workspaces always get `workset`:
  - `pkg/worksetapi/helpers.go` (`registerWorkspace`).
  - If `template` provided, `workset=template`; else `workset=workspace name`.
- Legacy `template` is now migration-only:
  - Migrated into `workset` and stripped on save/update.
- Workspace snapshot identity now prefers explicit `workset` over repo heuristics:
  - `pkg/worksetapi/workspace_snapshots.go`
  - `workset_key` now derives from normalized `workset` when present (`workset:<slug>`).
  - Fixes incorrect top-bar workset options when multiple threads belong to one workset.
- Terminal cockpit duplicate selector reduction:
  - `wails-ui/workset/frontend/src/App.svelte`
  - top context-bar workset switcher is hidden in `terminal-cockpit` (thread navigation remains in the left panel).
- Workset hub reactivity hardening:
  - `wails-ui/workset/frontend/src/lib/components/views/WorksetHubView.svelte`
  - prop-sync for `groupMode`/`layoutMode` now only applies when controlled props are explicitly provided.
  - avoids uncontrolled-mode resets and reduces risk of effect depth loops.
- Settings terminology surface update:
  - `wails-ui/workset/frontend/src/lib/components/settings/SettingsSidebar.svelte`
  - `wails-ui/workset/frontend/src/lib/components/SettingsPanel.svelte`
  - Removed Templates (`groups`) from visible Settings navigation and section routing.
- Create workset flow update (repo-catalog-first):
  - `wails-ui/workset/frontend/src/lib/components/workspace-action/WorkspaceActionCreateForm.svelte`
  - `wails-ui/workset/frontend/src/lib/components/workspace-action/WorkspaceActionFormContent.svelte`
  - `wails-ui/workset/frontend/src/lib/components/WorkspaceActionModal.svelte`
  - `wails-ui/workset/frontend/src/App.svelte`
  - New workset path now opens modal create flow with searchable repo-catalog multi-select instead of onboarding entry from rail.
- Create workset flow refinement (template-free + add-repo inline):
  - `wails-ui/workset/frontend/src/lib/services/workspaceActionContextService.ts`
  - `wails-ui/workset/frontend/src/lib/services/workspaceActionService.ts`
  - `wails-ui/workset/frontend/src/lib/components/workspace-action/WorkspaceActionCreateForm.svelte`
  - Create flow no longer loads template groups; it supports:
    - repo-catalog selection
    - inline add of direct repository sources
    - optional “save to catalog” per direct source.
- New thread entrypoint under workset Threads:
  - `wails-ui/workset/frontend/src/lib/components/chrome/ExplorerPanel.svelte`
  - `wails-ui/workset/frontend/src/lib/components/views/SpacesWorkbenchView.svelte`
  - `wails-ui/workset/frontend/src/App.svelte`
  - Added per-workset “+” action in Threads sections that opens a thread-scoped create modal (`create-thread`) with:
    - thread name input
    - seeded repo selection from the selected workset’s repos.
- Cockpit hierarchy alignment with global explorer + global changes drawer:
  - `wails-ui/workset/frontend/src/lib/components/views/SpacesWorkbenchView.svelte`
  - In main-window (`useGlobalExplorer=true`) mode:
    - suppresses the legacy cockpit header strip (thread/branch meta) so context is owned by the top context bar.
    - suppresses the legacy inline diff-summary sidebar so diff context is owned by the global right-side changes drawer.
  - Popout mode keeps the local header/diff affordances (no global explorer/chrome there).

## Migration Lifecycle Contract

To make migration removal deliberate (instead of ad hoc), global config migrations now run through an explicit ordered plan in `pkg/worksetapi/migrations.go`:

1. `2026-02-workspaces-to-worksets`
2. `2026-02-group-remotes-to-aliases`

Each migration now carries:
- a stable ID
- a summary of what it normalizes
- a `remove_after` guidance string documenting the deletion trigger

### Removal trigger

Delete a migration only when all are true:

1. Configs loaded in the supported upgrade window no longer contain the legacy key/shape.
2. Migration tests remain green with the migration code removed or no-op’d.
3. Two full minor releases have shipped with no regressions attributable to that legacy shape.

### Test coverage

`pkg/worksetapi/migrations_workset_test.go` now asserts:
- migration plan order is stable
- each migration includes removal metadata
- legacy key rewrite is idempotent across repeated loads

## Tests Added/Updated

- Added:
  - `pkg/worksetapi/migrations_workset_test.go`
    - migrates missing `workset` from workspace repo config.
    - migrates missing `workset` from legacy `template`.
  - `pkg/worksetapi/workspace_snapshots_test.go`
    - `TestListWorkspaceSnapshotsUsesExplicitWorksetIdentity` validates explicit workset key/label behavior.
- Updated:
  - `pkg/worksetapi/service_workspaces_test.go`
    - asserts create flow writes `workset` by default.
    - asserts template input also sets `workset`.

## Next Phases

1. Frontend terminology cleanup
   - Remove primary-nav dependence on `template` labels.
   - Keep backend compatibility fields for two minor releases.
2. Workspace switch UX
   - Keep one selector in top context bar once a workset is selected.
   - Avoid duplicate selector in left rail.
3. Thread/feature hierarchy
   - Define where feature threads are sourced and stored (workspace state vs global config).
4. Final deprecation
   - Remove `template` write paths and keep read-only compatibility migration.
