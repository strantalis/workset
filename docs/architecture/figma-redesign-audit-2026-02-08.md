# Figma Redesign Audit (2026-02-08)

## Scope

- Branch: `frontend-overhaul`
- Diff size: `32 files changed, 2382 insertions(+), 1589 deletions(-)` (tracked)
- Untracked files: `61` (many design screenshots + new frontend directories)
- Focus: regressions, mock-data leftovers, dead code, localization/LOC issues, runtime validation

## Validation Executed

### Backend checks

- `go test ./...` ✅ pass
- `golangci-lint run` ✅ pass (`0 issues`)

### Frontend checks

- `npm run lint` ❌ fail (`15 errors`, `3 warnings`)
- `npm run check` ✅ pass (`0 errors`, `13 warnings`)
- `npm run test` ❌ fail (`4 failed`, `324 passed`)
- `npm run build` ✅ pass (with warnings)

### Runtime validation (Playwright + running Wails dev app)

- App loaded at `http://127.0.0.1:34115`
- Navigated through:
  - Workset Hub
  - Command Center
  - Engineering Cockpit
  - PR Orchestration
  - Skill Registry
  - Settings sections (Workspace, Agent, GitHub, Repo Catalog, Templates)
  - Command Palette (`Meta+K`)
- Console/runtime errors observed:
  - `404` on `favicon.ico` (low severity)
  - Command palette button click blocked by top drag overlay (see High-4)

## Findings (Severity Ordered)

## High

1. Onboarding flow is still mock-wired and drops user intent.
- Evidence:
  - Simulated progress via `setTimeout` instead of backend orchestration: `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.svelte:165`
  - Draft/start path does not fully wire to real create flow: `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.svelte:181`, `wails-ui/workset/frontend/src/App.svelte:210`
  - Seed data is hardcoded catalog/registry: `wails-ui/workset/frontend/src/lib/view-models/onboardingViewModel.ts:23`, `wails-ui/workset/frontend/src/lib/view-models/onboardingViewModel.ts:68`
- Impact: redesign UI appears production-ready but behaves like demo scaffolding.

2. Skill markdown rendering is XSS-prone.
- Evidence:
  - Raw `{@html}` sinks flagged by eslint rule: `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.svelte:570`, `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.svelte:584`
  - Markdown converted by `marked` and rendered directly: `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.svelte:160`
- Impact: malicious/unsafe markdown can execute script in renderer context.

3. Repo diff flow is functionally regressed from Command Center.
- Evidence:
  - `onSelectRepo` declared but never used: `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.svelte:44`
  - App still expects repo selection to drive detail view: `wails-ui/workset/frontend/src/App.svelte:380`
- Impact: App-level RepoDiff route is effectively unreachable via redesigned Command Center interactions.

4. Context action button is blocked by draggable titlebar overlay.
- Evidence:
  - Playwright click failure: `.titlebar-drag intercepts pointer events`
  - Overlay definition: `wails-ui/workset/frontend/src/App.svelte:276`
  - Overlay styling (`position: fixed`, high z-index): `wails-ui/workset/frontend/src/App.svelte:507`
  - Blocked control lives in ContextBar: `wails-ui/workset/frontend/src/lib/components/chrome/ContextBar.svelte:58`
- Impact: visible command palette button is not reliably clickable (keyboard shortcut still works).

## Medium

1. PR orchestration remains partly mock-derived from repo state.
- Evidence:
  - PR titles/status synthesized from repo data: `wails-ui/workset/frontend/src/lib/view-models/prViewModel.ts:40`, `wails-ui/workset/frontend/src/lib/view-models/prViewModel.ts:47`
  - PR action hooks are dead/no-op: `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.svelte:78`, `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.svelte:85`
- Impact: PR surface can mislead users and hides missing backend wiring.

2. Open PR metrics are placeholder values.
- Evidence:
  - Hardcoded `openPrs: 0`: `wails-ui/workset/frontend/src/lib/view-models/worksetViewModel.ts:96`
  - CommandCenter stat placeholder: `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.svelte:79`
  - WorksetHub aggregates this into visible stats: `wails-ui/workset/frontend/src/lib/components/views/WorksetHubView.svelte:165`
- Impact: incorrect dashboard stats.

3. Dead controls remain in active UI.
- Evidence:
  - `View Logs` button without handler: `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.svelte:1165`
  - CommandCenter locals unused (`hasSummary`, `files`): `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.svelte:385`
- Impact: maintenance overhead + misleading affordances.

4. Settings redesign broke existing tests and naming contracts.
- Evidence:
  - Missing `Built With` + `Wails` text in About tests:
    - `src/lib/components/settings/SettingsPanel.about.spec.ts` failures
  - Sidebar label expectation mismatch (`Repo Registry` vs `Repo Catalog`):
    - `src/lib/components/settings/SettingsSidebar.spec.ts:59`
- Impact: test suite failing; behavioral/text contract drift not reconciled.

## Low

1. Workspace description persistence lacks normalization semantics.
- Evidence: `pkg/worksetapi/workspaces_ui.go:80` stores raw input, including whitespace-only values.
- Impact: inconsistent values in UI/state.

2. `SetWorkspaceDescription` may not update activity timestamp.
- Evidence: no `LastUsed` update in description setter: `pkg/worksetapi/workspaces_ui.go:80`
- Impact: stale activity ordering if `LastUsed` is intended as mutation activity.

3. Localization gaps (l10n).
- Evidence:
  - Hardcoded English relative time labels: `wails-ui/workset/frontend/src/lib/view-models/worksetViewModel.ts:30`, `wails-ui/workset/frontend/src/lib/view-models/prViewModel.ts:30`
  - Hardcoded bucket/group labels (`Today`, `This Week`): `wails-ui/workset/frontend/src/lib/view-models/worksetViewModel.ts:134`
- Impact: not localizable; pluralization/time-grammar correctness risk.

4. Repo contains large untracked screenshot artifacts.
- Evidence: 61 untracked files, mostly `.png` captures in project root.
- Impact: high noise floor for review, risk of accidental commits.

## Lint/Test Failure Inventory

### `npm run lint` key failures

- Unused/dead vars:
  - `wails-ui/workset/frontend/src/lib/components/SettingsPanel.svelte:26`
  - `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.svelte:42`
  - `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.svelte:27`
- Unsafe HTML rendering:
  - `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.svelte:570`
  - `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.svelte:584`
- Debug logging left in production paths:
  - `wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte:397`
  - `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.svelte:159`
  - `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.svelte:289`

### `npm run test` failing suites

- `src/lib/components/settings/SettingsPanel.about.spec.ts`
  - Missing expected text: `Built With`, `Wails`
- `src/lib/components/settings/SettingsSidebar.spec.ts`
  - Missing expected text: `Repo Registry`

## Cleanup Task Breakdown (Prioritized)

## Track A: Blockers (merge gate)

1. Replace mock onboarding init path with real backend create pipeline.
- Wire `OnboardingDraft` from `OnboardingView` through `App.svelte` to create API call.
- Remove `setTimeout` progress simulation; consume real progress state.
- Include description/template/repo selections in persisted create payload.

2. Eliminate unsafe markdown rendering in Skill Registry.
- Add markdown sanitization (DOMPurify or equivalent) before `{@html}` sinks.
- Keep lint rule `svelte/no-at-html-tags` green.

3. Restore repo-detail navigation from Command Center.
- Wire `onSelectRepo(activeWorkspaceId, repo.id)` from repo list row/card action.
- Add component test for selecting a repo from Command Center and showing detail view.

4. Fix click interception from drag overlay.
- Preferred: make ContextBar draggable and explicitly `no-drag` interactive buttons.
- Alternative: set `pointer-events: none` on `.titlebar-drag`.
- Add Playwright check for clickable command palette trigger.

## Track B: Correctness + fidelity

1. Replace placeholder PR stats and PR list synthesis with real data source.
2. Remove no-op action props and dead buttons (`View Logs`, unused handlers).
3. Normalize/validate workspace description input; define `LastUsed` semantics and test it.
4. Resolve Settings text contract changes by either:
- updating specs to the new UX language, or
- restoring previous labels where compatibility matters.

## Track C: Quality and maintainability

1. Clear all lint errors/warnings in redesign scope.
2. Address Svelte warnings (unused CSS selectors, a11y label association).
3. Convert relative-time and grouping labels to i18n-ready format (`Intl.RelativeTimeFormat` + strings catalog).
4. Move screenshot artifacts into a dedicated docs/assets folder or remove from worktree.

## Suggested Execution Order

1. Track A (A1-A4), then rerun frontend checks.
2. Track B (B1-B4), then update/add focused tests.
3. Track C cleanup before merge freeze.

## Exit Criteria

- `npm run lint` passes with zero errors in redesign files.
- `npm run test` passes (including settings suites).
- `npm run check` has no new warnings in touched files.
- Playwright flow confirms:
  - command palette button click works,
  - onboarding creates a real workset from selected draft,
  - command center opens repo details,
  - PR metrics reflect actual data source.
