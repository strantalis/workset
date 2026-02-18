# Workset UI Figma Make Migration Tracker

Owner: Sean + Codex  
Last updated: 2026-02-07

## Objective

Implement the new Figma Make UI concepts in the Wails Svelte app with in-place replacement, centralized styling, and Playwright validation against `wails dev`.

## Scope

- App shell + navigation rail
- Context bar + command palette
- Workset Hub
- Command Center
- Terminal Cockpit
- PR Orchestration
- Onboarding
- Skill Registry
- Shared tokens + reusable UI/chrome primitives

## Constraints

- Keep existing backend APIs and Wails bindings unchanged.
- Avoid style scatter: no ad-hoc utility sprawl, no repeated one-off CSS.
- Use real state/service data, not static demo data in production views.

## Parallel Task Matrix

| Task ID | Lane | Task | Depends On | Status | Files |
| --- | --- | --- | --- | --- | --- |
| UI-001 | A | Create migration tracker + execution gates | - | completed | `docs/architecture/workset-ui-figma-make-migration-tracker.md` |
| UI-002 | A | Add view-model adapter layer | - | completed | `wails-ui/workset/frontend/src/lib/view-models/*` |
| UI-003 | A | Add reusable chrome primitives (ContextBar + CommandPalette) | UI-002 | completed | `wails-ui/workset/frontend/src/lib/components/chrome/*` |
| UI-004 | B | Build Workset Hub view | UI-002, UI-003 | completed | `wails-ui/workset/frontend/src/lib/components/views/WorksetHubView.svelte` |
| UI-005 | B | Build Command Center view | UI-002, UI-003 | completed | `wails-ui/workset/frontend/src/lib/components/views/CommandCenterView.svelte` |
| UI-006 | C | Build Terminal Cockpit view | UI-002, UI-003 | completed | `wails-ui/workset/frontend/src/lib/components/views/TerminalCockpitView.svelte` |
| UI-007 | C | Build PR Orchestration view | UI-002, UI-003 | completed | `wails-ui/workset/frontend/src/lib/components/views/PROrchestrationView.svelte` |
| UI-008 | D | Build Onboarding view | UI-002, UI-003 | completed | `wails-ui/workset/frontend/src/lib/components/views/OnboardingView.svelte` |
| UI-009 | D | Build Skill Registry view | UI-002, UI-003 | completed | `wails-ui/workset/frontend/src/lib/components/views/SkillRegistryView.svelte` |
| UI-010 | E | Replace `App.svelte` shell + wire all views | UI-004..UI-009 | completed | `wails-ui/workset/frontend/src/App.svelte` |
| UI-011 | F | Run Svelte autofixer on all touched `.svelte` files | UI-010 | completed | touched `.svelte` files |
| UI-012 | F | Frontend verification gates (`format:check`, `lint`, `check`, `test`, `build`) | UI-011 | completed | frontend workspace |
| UI-013 | F | Playwright validation against `wails dev` browser URL | UI-012 | completed | Playwright run log |
| UI-014 | F | Fix `RepoDiff` callback binding regression found in browser validation | UI-013 | completed | `wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte` |
| UI-015 | G | Remove pre-existing `no-console` lint warning in `RepoDiff` | UI-012 | pending | `wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte` |

## Style Conformance Checklist

- [x] All new color/spacing/typography uses map to semantic tokens.
- [x] No repeated style blocks for the same UI pattern across views.
- [x] Shared patterns extracted to `components/ui` or `components/chrome`.
- [x] No hardcoded palette values in feature views unless documented exception.

## View Parity Checklist

- [x] Workset Hub
- [x] Command Center
- [x] Terminal Cockpit
- [x] PR Orchestration
- [x] Onboarding
- [x] Skill Registry
- [x] Context bar
- [x] Command palette

## Verification Log

| Timestamp | Command | Result | Notes |
| --- | --- | --- | --- |
| 2026-02-07 | Planning + environment audit | pass | Confirmed Wails/Svelte plumbing and Figma Make access. |
| 2026-02-07 | `npx @sveltejs/mcp svelte-autofixer` on `App.svelte`, chrome components, view components | pass | Fixed invalid nested button structure and command palette a11y issues from autofixer findings. |
| 2026-02-07 | `npm run format:check` | pass | Prettier clean after formatting touched files. |
| 2026-02-07 | `npm run lint` | pass (warnings) | No lint errors. One pre-existing warning remains in `RepoDiff.svelte` (`no-console`). |
| 2026-02-07 | `npm run check` | pass | Fixed initial type errors in `CommandCenterView.svelte` and icon import in `TerminalCockpitView.svelte`. |
| 2026-02-07 | `npm run test` | pass | 71 test files / 328 tests passed. |
| 2026-02-07 | `npm run build` | pass | Build succeeds; existing chunk-size warnings remain. |

## Playwright Validation Log

| Timestamp | Target URL | Scenario | Result | Notes |
| --- | --- | --- | --- | --- |
| 2026-02-07 | `http://localhost:34115` | Screen navigation across all six views | pass | Each rail target renders expected view text and state. |
| 2026-02-07 | `http://localhost:34115` | Context bar visibility rules | pass | Visible for scoped views after selecting a workspace; hidden on Hub/Onboarding. |
| 2026-02-07 | `http://localhost:34115` | Command palette (`Meta+K`) open + navigate | pass | Palette opens and routes to selected view. |
| 2026-02-07 | `http://localhost:34115` | PR view repo action opens `RepoDiff` | pass | `section.diff` renders after clicking `Open Repo`/`Inspect`. |
| 2026-02-07 | `http://localhost:34115` | Runtime console health after interactions | pass | Fixed `Illegal invocation` by binding animation callbacks in `RepoDiff.svelte`. |

## Risks / Blockers

- `RepoDiff.svelte` still has one pre-existing lint warning (`console.error`) tracked in UI-015.
- Build emits chunk-size warnings (pre-existing), not blocking this migration.
