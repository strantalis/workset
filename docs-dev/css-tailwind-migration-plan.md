# CSS to Tailwind Migration Plan (Workset Frontend)

- Status: Executed (phase rollout complete for shared-class dedupe pass)
- Date: 2026-02-19
- Scope root: `wails-ui/workset/frontend/src`
- Planning owner: Documentation-only plan

## Goals

1. Reduce duplicated style logic while preserving current behavior and visual output.
2. Consolidate styling onto Tailwind v4 + a small shared component layer instead of large view-specific CSS files.
3. Migrate high-churn screens first, then shared components, then cleanup.
4. Keep every migration step reversible with low-risk rollback.
5. Enforce measurable quality gates so migration does not regress UX, accessibility, or test stability.

## Execution Summary (2026-02-19)

### What was completed

1. Tailwind v4 foundation was wired and normalized:
   - `wails-ui/workset/frontend/package.json`
   - `wails-ui/workset/frontend/vite.config.ts`
   - `wails-ui/workset/frontend/src/style.css`
2. Shared component class layer expanded in:
   - `wails-ui/workset/frontend/src/styles/components.css`
3. High-churn component families were migrated to shared classes:
   - workspace actions + workspace manager
   - settings sections
   - chrome/context components
   - repo-diff header/list surfaces
   - onboarding + hook status surfaces
   - workset hub + command center view surfaces
4. Repeated selector groups (dot/status chips, diff stats, section titles, keyboard chips, pending-hook actions/errors, empty-state primitives) were consolidated and local duplicates removed.

### Validation results

1. `npm run format:check` ✅
2. `npm run lint` ✅
3. `npm run test` ✅ (`75` files, `379` tests)
4. `npm run build` ✅
5. `npm run check` ❌ due pre-existing type-contract drift in API/test files unrelated to this CSS migration:
   - `src/lib/api/github/pull-request.ts`
   - `src/lib/api/repo-diff.ts`
   - `src/lib/api/updates.ts`
   - `src/lib/api.settings.test.ts`

### Post-migration metrics snapshot

Compared to the initial baseline from this document:

- Total style LOC: `13,953` -> `13,441` (`-512`)
- Inline style LOC: `8,381` -> `7,654` (`-727`)
- Style units (`<style>` blocks + css files): `70` -> `62`
- Repeated selectors: `203` -> `159`
- Exact duplicate selector+rule combos: `78` -> `42`
- Excess duplicate copies: `110` -> `61`

## Non-goals

1. No backend/API changes.
2. No redesign pass; visual parity is the default target.
3. No rewrite of test strategy; reuse current test suite and add deltas only where migration introduces risk.

## Current-state metrics (baseline)

Snapshot source: `wails-ui/workset/frontend/src` as of 2026-02-19.

### Styling footprint

- External CSS files: `8`
- External CSS LOC: `5,572`
- Svelte files: `65`
- Svelte style blocks: `63` (`57` inline, `6` external `src=...`)
- Inline `<style>` LOC in Svelte: `8,381`
- Total style LOC (external + inline): `13,953`
- Unique class selectors in external CSS: `482`
- `var(--token)` references in external CSS: `1,120`

### Largest external CSS files

1. `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.css` (`1,340` LOC)
2. `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.css` (`1,151` LOC)
3. `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.css` (`1,059` LOC)
4. `wails-ui/workset/frontend/src/lib/components/views/WorksetHubView.css` (`837` LOC)
5. `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.css` (`677` LOC)

### Largest inline style hotspots (Svelte)

1. `wails-ui/workset/frontend/src/lib/components/views/TerminalCockpitView.svelte` (`466` style LOC)
2. `wails-ui/workset/frontend/src/lib/components/workspace-action/WorkspaceActionAddRepoForm.svelte` (`397`)
3. `wails-ui/workset/frontend/src/lib/components/settings/sections/GroupManager.svelte` (`375`)
4. `wails-ui/workset/frontend/src/lib/components/settings/sections/AliasManager.svelte` (`357`)
5. `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiffAnnotationStyles.svelte` (`339`)
6. `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiffPrPanel.svelte` (`300`)

### Duplication signals

- Most repeated declaration lines across CSS:
  - `display: flex;` (`152`)
  - `align-items: center;` (`140`)
  - `color: var(--muted);` (`91`)
  - `color: var(--text);` (`86`)
  - `border: 1px solid var(--border);` (`59`)
- Non-token hardcoded color literals outside `wails-ui/workset/frontend/src/style.css`:
  - `32` occurrences across `20` unique literal values/patterns

### Tailwind readiness

- Tailwind is already wired:
  - dependency: `tailwindcss@4.2.0`
  - Vite plugin: `@tailwindcss/vite` in `wails-ui/workset/frontend/vite.config.ts`
  - import: `@import 'tailwindcss';` in `wails-ui/workset/frontend/src/style.css`
- Existing shared layer exists: `wails-ui/workset/frontend/src/styles/components.css` (`@layer components`)
- Unit/integration coverage surface for main views already exists:
  - `wails-ui/workset/frontend/src/lib/components/views/*.spec.ts` and `*.svelte.test.ts` (`5` files)
  - Playwright e2e specs: `wails-ui/workset/frontend/e2e/*.spec.ts` (`7` files)

## File priority order

Priority is based on style volume, churn risk, and user-visible impact.

1. `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.svelte`
1. `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.css`
1. `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.svelte`
1. `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.css`
1. `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.svelte`
1. `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.css`
1. `wails-ui/workset/frontend/src/lib/components/views/WorksetHubView.svelte`
1. `wails-ui/workset/frontend/src/lib/components/views/WorksetHubView.css`
1. `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.svelte`
1. `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.css`
1. `wails-ui/workset/frontend/src/App.svelte`
1. `wails-ui/workset/frontend/src/App.css`
1. `wails-ui/workset/frontend/src/lib/components/views/TerminalCockpitView.svelte`
1. `wails-ui/workset/frontend/src/lib/components/workspace-action/WorkspaceActionAddRepoForm.svelte`
1. `wails-ui/workset/frontend/src/lib/components/settings/sections/GroupManager.svelte`
1. `wails-ui/workset/frontend/src/lib/components/settings/sections/AliasManager.svelte`
1. `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiffAnnotationStyles.svelte`
1. `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiffPrPanel.svelte`
1. `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiffChecksSidebar.svelte`
1. `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiffFileListSidebar.svelte`
1. `wails-ui/workset/frontend/src/lib/components/WorkspaceTree.svelte`
1. `wails-ui/workset/frontend/src/lib/components/WorkspaceItem.svelte`
1. `wails-ui/workset/frontend/src/styles/components.css`
1. `wails-ui/workset/frontend/src/style.css`

## Phased rollout

## Phase 0: Baseline + guardrails (no behavior change)

### Objective

Create migration safety rails and baseline measurements before style moves.

### Work

1. Capture and store baseline metrics from this plan into a tracked migration checklist.
2. Define migration conventions:
   - Tailwind utilities for layout/spacing/typography.
   - keep semantic class names only where component API/readability needs it.
   - `@layer components` for reusable patterns currently duplicated 3+ times.
3. Freeze new large CSS additions in:
   - `wails-ui/workset/frontend/src/lib/components/views/*.css`
   - inline `<style>` blocks in large Svelte files.

### Exit criteria

1. Team agrees on migration conventions and acceptance criteria.
2. Baseline metrics are documented and referenced by phase gates.

## Phase 1: Foundation and token mapping

### Objective

Map existing design tokens to Tailwind theme aliases and stabilize shared primitives first.

### Work

1. Centralize token mapping from `wails-ui/workset/frontend/src/style.css`.
2. Expand `wails-ui/workset/frontend/src/styles/components.css` only for reusable semantic primitives that cannot be represented cleanly as utilities.
3. Migrate app-shell-level primitives:
   - `wails-ui/workset/frontend/src/App.svelte`
   - `wails-ui/workset/frontend/src/App.css`

### Exit criteria

1. App shell renders with parity in all main routes.
2. No net increase in external CSS LOC.
3. `npm run check`, `npm run test`, and route smoke e2e pass.

## Phase 2: High-volume view migration

### Objective

Retire large view-scoped CSS files in priority order.

### Work order

1. Onboarding
   - `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.svelte`
   - `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.css`
2. PR orchestration
   - `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.svelte`
   - `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.css`
3. Skill registry
   - `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.svelte`
   - `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.css`
4. Workset hub
   - `wails-ui/workset/frontend/src/lib/components/views/WorksetHubView.svelte`
   - `wails-ui/workset/frontend/src/lib/components/views/WorksetHubView.css`
5. Command center
   - `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.svelte`
   - `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.css`

### Exit criteria per view

1. View-specific unit/spec tests pass.
2. Relevant e2e flow passes.
3. View CSS file is reduced to near-zero or removed.
4. Hardcoded color literals in migrated view become token-based or justified exceptions.

## Phase 3: Inline style block migration (component families)

### Objective

Convert large inline `<style>` blocks to utility-first patterns and shared component classes.

### Work clusters

1. Terminal cluster
   - `wails-ui/workset/frontend/src/lib/components/views/TerminalCockpitView.svelte`
   - `wails-ui/workset/frontend/src/lib/components/TerminalWorkspace.svelte`
   - `wails-ui/workset/frontend/src/lib/components/TerminalPane.svelte`
   - `wails-ui/workset/frontend/src/lib/components/TerminalLayoutNode.svelte`
2. Workspace actions/settings cluster
   - `wails-ui/workset/frontend/src/lib/components/workspace-action/WorkspaceActionAddRepoForm.svelte`
   - `wails-ui/workset/frontend/src/lib/components/settings/sections/GroupManager.svelte`
   - `wails-ui/workset/frontend/src/lib/components/settings/sections/AliasManager.svelte`
   - `wails-ui/workset/frontend/src/lib/components/WorkspaceManager.svelte`
3. Repo diff cluster
   - `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiffAnnotationStyles.svelte`
   - `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiffPrPanel.svelte`
   - `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiffChecksSidebar.svelte`
   - `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiffFileListSidebar.svelte`

### Exit criteria

1. Inline style LOC reduced by at least 40% from baseline (`8,381` -> `<=5,028`).
2. Shared primitives avoid duplicated declaration lines in new code.
3. No e2e regressions in terminal, PR, and workspace flows.

## Phase 4: Cleanup + enforcement

### Objective

Finalize migration and prevent CSS duplication regressions.

### Work

1. Remove fully migrated legacy CSS files.
2. Add style governance checks (CI and reviewer checklist):
   - reject new large view-level `.css` files unless explicitly approved.
   - reject repeated one-off utility aliases when existing shared primitives cover it.
3. Update docs with final token/utilities conventions and migration outcomes.

### Exit criteria

1. External CSS LOC reduced by at least 50% from baseline (`5,572` -> `<=2,786`).
2. Inline style LOC reduced by at least 50% from baseline (`8,381` -> `<=4,190`).
3. Duplicated declaration concentration reduced (top 5 repeated declaration counts down by >=30%).
4. Full frontend verification gate green.

## Explicit parallelization opportunities (sub-agent tracks)

Use independent sub-agent tracks with strict file ownership to avoid merge conflicts.

| Track | Scope                           | Files (primary)                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Can run in parallel with                | Depends on                                       |
| ----- | ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------- | ------------------------------------------------ |
| A     | Foundation/token mapping        | `wails-ui/workset/frontend/src/style.css`, `wails-ui/workset/frontend/src/styles/components.css`, `wails-ui/workset/frontend/src/App.svelte`, `wails-ui/workset/frontend/src/App.css`                                                                                                                                                                                                                                                                                         | B, C (after token contracts are stable) | Phase 0                                          |
| B     | View migration lane 1           | `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.svelte`, `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.css`, `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.svelte`, `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.css`                                                                                                                                                                | C, D                                    | A token contracts                                |
| C     | View migration lane 2           | `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.svelte`, `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.css`, `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.svelte`, `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.css`, `wails-ui/workset/frontend/src/lib/components/views/WorksetHubView.svelte`, `wails-ui/workset/frontend/src/lib/components/views/WorksetHubView.css` | B, D                                    | A token contracts                                |
| D     | Inline-style component clusters | `wails-ui/workset/frontend/src/lib/components/views/TerminalCockpitView.svelte`, `wails-ui/workset/frontend/src/lib/components/workspace-action/WorkspaceActionAddRepoForm.svelte`, `wails-ui/workset/frontend/src/lib/components/settings/sections/GroupManager.svelte`, `wails-ui/workset/frontend/src/lib/components/settings/sections/AliasManager.svelte`, `wails-ui/workset/frontend/src/lib/components/repo-diff/RepoDiff*.svelte`                                     | B, C                                    | A token contracts; view-level conflicts resolved |
| E     | Verification and release safety | `wails-ui/workset/frontend/e2e/*.spec.ts`, `wails-ui/workset/frontend/src/lib/components/views/*.spec.ts`, `wails-ui/workset/frontend/src/lib/components/views/*.svelte.test.ts`                                                                                                                                                                                                                                                                                              | All tracks                              | none                                             |

### Sub-agent operating rules

1. One track owns a file at a time.
2. Track E runs continuously and reports regressions before merge.
3. Merge sequence per batch: A -> (B and C) -> D -> E full gate.
4. No cross-track refactors without explicit handoff note.

## Risk controls

1. **Visual parity gates**
   - For each migrated view, validate key states in Playwright:
     - `wails-ui/workset/frontend/e2e/redesign-flows.spec.ts`
     - `wails-ui/workset/frontend/e2e/workset-hub-command-center.spec.ts`
     - `wails-ui/workset/frontend/e2e/pr-orchestration-flows.spec.ts`
     - `wails-ui/workset/frontend/e2e/cockpit-settings.spec.ts`
2. **Behavior parity gates**
   - Run and keep green:
     - `npm run format:check`
     - `npm run lint`
     - `npm run check`
     - `npm run test`
     - `npm run build`
3. **Scope control**
   - Migrate one view/cluster per change set; avoid mixed structural + styling rewrites in the same change.
4. **Token discipline**
   - New color/spacing/typography must map to `var(--...)`-backed theme values or explicit Tailwind theme aliases.
5. **Conflict control**
   - File-level ownership matrix per sub-agent track to avoid overlapping edits in high-churn files.

## Rollback strategy

1. **Per-phase rollback**
   - Keep migration changes phase-scoped so each batch can be reverted with a single `git revert`.
2. **Per-view rollback**
   - During active migration, keep legacy CSS file and import path until acceptance is complete, then remove in a follow-up cleanup commit.
3. **Break-glass rollback trigger**
   - If any of `npm run check`, `npm run test`, `npm run build`, or critical e2e specs fail after migration and cannot be resolved quickly, revert the latest migration batch.
4. **Operational rollback order**
   - Revert newest migration batch first.
   - Re-run frontend gate and smoke e2e before continuing.
5. **Artifact safety**
   - Do not mix styling migration with backend logic changes; rollback remains styling-only.

## Execution checklist (for implementation phase)

1. Confirm baseline metrics have not drifted significantly before starting.
2. Start Track A and stabilize token/shared layer contracts.
3. Run Track B and Track C in parallel on non-overlapping files.
4. Run Track D after high-volume view merge conflicts are clear.
5. Run Track E continuously plus a full gate before each merge wave.
6. At phase close, update metrics against baseline and decide continue/rollback.
