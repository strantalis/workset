# Figma Redesign Cleanup Report (2026-02-09)

## Scope

- Repo: `workset`
- Focus: redesign surfaces in Wails frontend and API-to-UI data trust
- Audit goals:
  - eliminate synthetic/mock-like UI data where users expect real API state
  - identify regressions, dead code, and UX trust gaps
  - produce a concrete cleanup backlog with implementation order

## Implemented In This Pass

1. Workspace snapshots now preserve workspace entries when `workset.yaml` or state is unhealthy.
- Behavior:
  - missing `workset.yaml`: workspace still returned with empty repo list
  - corrupt state: workspace and repos still returned; tracked PR state omitted
- Files:
  - `pkg/worksetapi/workspace_snapshots.go`
  - `pkg/worksetapi/workspace_snapshots_test.go`

2. Reorder API now fails on unknown workspace IDs to prevent frontend/backend drift.
- Behavior:
  - request returns `NotFoundError` when payload contains stale workspace IDs
- Files:
  - `pkg/worksetapi/workspaces_ui.go`
  - `pkg/worksetapi/workspaces_ui_test.go`

3. Snapshot payload now includes real repo branch/ahead/behind/diff/file data directly from backend APIs.
- Behavior:
  - `ListWorkspaceSnapshots(includeStatus=true)` now includes `currentBranch`, `ahead`, `behind`, `diff`, and `files`
  - values are sourced from real local git status and diff summary APIs
  - frontend no longer performs per-repo fan-out hydration calls
- Files:
  - `wails-ui/workset/app_workspaces.go`
  - `wails-ui/workset/frontend/src/lib/api/workspaces.ts`
  - `wails-ui/workset/frontend/src/lib/api.workspace-terminal.test.ts`

4. Synthetic recency fallback removed.
- Behavior:
  - no longer forces `lastUsed` to “now” when missing
  - unknown timestamps remain empty and sort as oldest in local derived ordering
- Files:
  - `wails-ui/workset/frontend/src/lib/api/workspaces.ts`
  - `wails-ui/workset/frontend/src/lib/state.ts`

5. UX trustfulness fixes for controls and actions.
- Behavior:
  - PR checks action now says `Refresh checks` (truthful semantics)
  - cockpit controls that have no backend behavior are disabled and marked coming soon
- Files:
  - `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.svelte`
  - `wails-ui/workset/frontend/src/lib/components/views/TerminalCockpitView.svelte`

6. Locale-aware relative time formatting introduced.
- Behavior:
  - shared formatter now uses `Intl.RelativeTimeFormat`
  - PR/workset view-models share one implementation
- Files:
  - `wails-ui/workset/frontend/src/lib/view-models/relativeTime.ts`
  - `wails-ui/workset/frontend/src/lib/view-models/prViewModel.ts`
  - `wails-ui/workset/frontend/src/lib/view-models/worksetViewModel.ts`
  - `wails-ui/workset/frontend/src/lib/view-models/prViewModel.test.ts`

7. Warning burn-down completed for a11y/css/no-console issues identified in redesign surfaces.
- Behavior:
  - fixed label associations in settings and skill registry
  - removed unused selector warnings tied to icon classes
  - removed production `console.warn/error` paths in the touched redesign surfaces
- Files:
  - `wails-ui/workset/frontend/src/lib/components/settings/sections/AliasManager.svelte`
  - `wails-ui/workset/frontend/src/lib/components/settings/sections/GroupManager.svelte`
  - `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.svelte`
  - `wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte`

8. Playwright coverage expanded to validate API-backed repo metadata rendering.
- Added assertions:
  - command center branch/ahead/behind values match API snapshot payload
- File:
  - `wails-ui/workset/frontend/e2e/redesign-flows.spec.ts`

9. Comprehensive Playwright suite added for redesign stabilization and regression prevention.
- Added:
  - shared app/API harness utilities for Wails API reads and stable navigation setup
  - shell navigation coverage (rail views, command palette shortcut, settings modal lifecycle)
  - workset hub stat parity coverage against live snapshots
  - command center stat and repo-metadata parity coverage
  - PR orchestration count parity and checks-copy trustfulness coverage
  - cockpit + settings behavior and API-backed library-count coverage
- Files:
  - `wails-ui/workset/frontend/e2e/helpers/app-harness.ts`
  - `wails-ui/workset/frontend/e2e/navigation-shell.spec.ts`
  - `wails-ui/workset/frontend/e2e/workset-hub-command-center.spec.ts`
  - `wails-ui/workset/frontend/e2e/pr-orchestration-flows.spec.ts`
  - `wails-ui/workset/frontend/e2e/cockpit-settings.spec.ts`

## Validation Executed

### Go

- `go test ./...` (pass)
- `golangci-lint run` (pass, 0 issues)

### Frontend

- `npm run lint` (pass)
- `npm run check` (pass, 0 warnings)
- `npm run test` (pass, 74 files / 332 tests)
- `npm run build` (pass, bundle-size warnings remain)
- `npm run test:e2e` (pass, 15/15)

### Playwright (live Wails app)

- `wails-ui/workset/frontend/e2e/redesign-flows.spec.ts`
- `wails-ui/workset/frontend/e2e/navigation-shell.spec.ts`
- `wails-ui/workset/frontend/e2e/workset-hub-command-center.spec.ts`
- `wails-ui/workset/frontend/e2e/pr-orchestration-flows.spec.ts`
- `wails-ui/workset/frontend/e2e/cockpit-settings.spec.ts`
- Passed flows:
  - command palette open from context bar
  - rail navigation across all primary redesign surfaces
  - command palette keyboard shortcut behavior
  - settings modal open/close lifecycle
  - workset hub stat-pill parity with API snapshot totals
  - command center repo-details navigation + API-backed stat assertions
  - command center branch/ahead/behind values match API snapshot
  - cockpit workspace/repo context parity and no-op control disabled-state assertions
  - settings section switching and API-backed catalog/template count assertions
  - onboarding templates from API-backed catalog
  - onboarding workspace-name default safety check
  - PR orchestration sidebar counts vs API-backed tracked PR data
  - PR orchestration checks action wording trustfulness (`Refresh checks`)

## Current Findings (Severity Ordered)

## High

1. None currently open in redesigned data path after this pass.

## Medium

1. Build remains heavy due syntax-highlighting chunks and one mixed static+dynamic import warning.
- Evidence: `npm run build` chunk-size output and dynamic import warning for terminal service.
- Impact: slower cold-load/bundle churn; no functional correctness impact.
- Files:
  - `wails-ui/workset/frontend/vite.config.ts`
  - `wails-ui/workset/frontend/src/lib/components/SettingsPanel.svelte`
  - `wails-ui/workset/frontend/src/lib/components/TerminalWorkspace.svelte`

## Low

1. Some view-level error paths still degrade silently by design (non-blocking UI behavior).
- Candidate follow-up: add lightweight toast plumbing for non-fatal action failures.

## Task Breakdown (Next Cleanup)

## Track A: Bundle Performance and Build Noise

1. Audit syntax-highlighting dependencies (`@pierre/diffs`) and trim language/theme payloads where feasible.
2. Evaluate Vite v7 chunking strategy (`rolldownOptions` / chunk policies) for large generated language chunks.
3. Remove mixed dynamic+static import duplication around terminal service loading.

## Track B: UX Reliability Hardening

1. Add non-fatal error toast channel for review/check actions that currently fail quietly.
2. Add mutation-safe e2e coverage for pin/archive/settings save paths in an isolated fixture workspace.

## Coverage Added In This Pass

- Backend:
  - `pkg/worksetapi/workspace_snapshots_test.go`
  - `pkg/worksetapi/workspaces_ui_test.go`
- Frontend unit:
  - `wails-ui/workset/frontend/src/lib/api.workspace-terminal.test.ts`
  - `wails-ui/workset/frontend/src/lib/view-models/onboardingViewModel.test.ts`
  - `wails-ui/workset/frontend/src/lib/view-models/prViewModel.test.ts`
- Frontend e2e:
  - `wails-ui/workset/frontend/e2e/redesign-flows.spec.ts`
  - `wails-ui/workset/frontend/e2e/navigation-shell.spec.ts`
  - `wails-ui/workset/frontend/e2e/workset-hub-command-center.spec.ts`
  - `wails-ui/workset/frontend/e2e/pr-orchestration-flows.spec.ts`
  - `wails-ui/workset/frontend/e2e/cockpit-settings.spec.ts`
  - `wails-ui/workset/frontend/e2e/helpers/app-harness.ts`

## Recommended Cleanup Order

1. Track A (bundle/chunk performance)
2. Track B (error-surfacing polish)
