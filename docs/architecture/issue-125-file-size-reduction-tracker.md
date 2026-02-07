# Issue #125 File-Size Reduction Tracker

Owner: Sean + Codex  
Source issue: `https://github.com/strantalis/workset/issues/125`  
Last updated: 2026-02-08 (subagent pass 27)

## Goal

Reduce architecture risk from oversized files by splitting high-complexity modules into stable boundaries with tests and CI guardrails.

## Program Targets

- No production source file exceeds 700 LOC unless explicitly allowlisted with justification.
- CI blocks regressions on file-size and test/lint failures.
- Refactors are behavior-preserving and land in small, reversible commits.

## Current Baseline (2026-02-08)

Largest files by LOC right now:

- `wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte` (2986)
- `wails-ui/workset/frontend/src/lib/components/WorkspaceActionModal.svelte` (2420)
- `wails-ui/workset/frontend/src/lib/terminal/terminalService.ts` (1293)
- `wails-ui/workset/frontend/src/lib/components/TerminalWorkspace.svelte` (1061)
- `wails-ui/workset/frontend/src/lib/components/WorkspaceManager.svelte` (1022)
- `wails-ui/workset/frontend/src/lib/components/SettingsPanel.svelte` (987)
- `pkg/termemu/termemu.go` (972)
- `wails-ui/workset/frontend/src/lib/components/settings/sections/SkillManager.svelte` (956)
- `wails-ui/workset/app_diffs.go` (926)

## Parallel Tracks (Issue Map)

- [x] `#124` Guardrails (must start first)
- [x] `#115` FE-DIFF (slice 13 landed)
- [ ] `#116` FE-WORKSPACE (slice 6 landed)
- [ ] `#117` FE-TERMINAL (slice 16 landed)
- [x] `#118` FE-PLATFORM (slice 5 landed; adapter removed)
- [x] `#119` BE-SESSIOND (structural splits complete)
- [x] `#120` BE-GITHUB (slice 5 + tests tranche 2 landed)
- [x] `#121` BE-TERMEMU (slice 4 landed)
- [x] `#122` BE-UPDATER (slice 3 landed + orchestration tests)
- [x] `#123` TEST-E2E (scenario split + fixtures landed)

## Execution Strategy

### Phase 0: Guardrails First (`#124`)

- [x] Add file-size policy script (production files only; tests/docs excluded).
- [x] Add allowlist file with reason + owner + expiry.
- [x] Wire policy check into CI and `make check` parity path.
- [x] Fail PRs on new violations or LOC growth above threshold.
- [x] Add docs for override workflow.

Exit criteria:

- PR cannot merge when policy fails.
- Existing exceptions are explicit and justified.

### Phase 1: Reliability-Critical Backend Splits (`#119`, `#120`, `#121`, `#122`)

Order:

1. `#119` sessiond (highest runtime risk)
2. `#120` github service
3. `#121` termemu
4. `#122` updater

### Phase 2: Frontend Feature Splits (`#115`, `#116`, `#117`, `#118`)

Order:

1. `#115` RepoDiff
2. `#116` WorkspaceActionModal/workspace flows
3. `#117` terminal service completion
4. `#118` API/settings orchestration

### Phase 3: Test Architecture (`#123`)

- [x] Split monolithic e2e suite into scenario files + shared fixtures.
- [x] Align test package/module boundaries with refactored production modules.

## Detailed Task Plan by Issue

## `#124` Guardrails

Scope:

- `scripts/` + CI workflow + docs.

Tasks:

- [x] Implement `scripts/check-file-size.sh` (or Go equivalent) with path filters.
- [x] Add config file for thresholds/allowlist.
- [x] Add CI job step for policy enforcement.
- [x] Add local command (`go run ./scripts/guardrails --config guardrails.yml --head-sha "$(git rev-parse HEAD)"`).

Verification:

- [x] `go run ./scripts/guardrails --config guardrails.yml --head-sha "$(git rev-parse HEAD)"`
- [x] CI run shows failing sample + passing sample.

## `#115` FE-DIFF

Primary file:

- `wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte`

Target architecture:

- `repo-diff/summary/` (summary load + state)
- `repo-diff/pr/` (PR status/reviews actions)
- `repo-diff/render/` (diff rendering bridge)
- `repo-diff/comments/` (annotation/review interactions)
- Keep `RepoDiff.svelte` as composition shell.

Tasks:

- [x] Extract watcher lifecycle/start-stop-update orchestration module (`repo-diff/watcherLifecycle.ts`).
- [x] Extract summary loader/store controller module (`repo-diff/summaryController.ts`).
- [x] Extract PR status/reviews controller (`repo-diff/prStatusController.ts`).
- [x] Extract render queue/selection/file-fetch controller (`repo-diff/fileDiffController.ts`).
- [x] Extract annotation/reply/edit/delete actions module (`repo-diff/reviewAnnotationActions.ts`).
- [x] Extract check-sidebar grouping/filtering/summary state into `repo-diff/checkSidebarController.ts`.
- [x] Extract GitHub operation/auth orchestration into `repo-diff/githubOperationsController.ts`.
- [x] Extract mount/subscription/cleanup orchestration into `repo-diff/repoDiffLifecycle.ts`.
- [x] Extract sidebar resize/persistence lifecycle into `repo-diff/sidebarResizeController.ts`.
- [x] Keep current public props/events unchanged.
- [x] Extract diff rendering + scroll/highlight orchestration into `repo-diff/diffRenderController.ts`.
- [x] Extract PR/status/create orchestration state surface into `repo-diff/prOrchestrationSurface.ts`.
- [x] Extract summary/local/branch diff source switching orchestration into a dedicated helper.
  Slice landed: extracted source-switch + branch-ref reload orchestration into `repo-diff/summarySourceController.ts`.
- [x] Extract checks sidebar rendering state surface from `RepoDiff.svelte` template.
  Slice landed: extracted checks tab UI + interactions into `repo-diff/RepoDiffChecksSidebar.svelte`.

Verification:

- [x] `cd wails-ui/workset/frontend && npm run test -- src/lib/components/RepoDiff.spec.ts`
- [x] `cd wails-ui/workset/frontend && npm run check`

## `#116` FE-WORKSPACE

Primary files:

- `wails-ui/workset/frontend/src/lib/components/WorkspaceActionModal.svelte`
- `wails-ui/workset/frontend/src/lib/components/WorkspaceManager.svelte`
- `wails-ui/workset/frontend/src/lib/components/WorkspaceItem.svelte`

Target architecture:

- modal state machine + action handlers extracted to module(s)
- per-action panes as small components
- shared workspace mutation service for create/remove/archive/pin/color

Tasks:

- [x] Extract hook-results phase UI into `workspace-action/WorkspaceActionHookResults.svelte`.
- [x] Extract removal overlay UI into `workspace-action/RemovalOverlay.svelte`.
- [x] Extract create/add mutation + hook transition logic into `services/workspaceActionService.ts`.
- [x] Extract rename/archive/remove mutation runners into `services/workspaceActionService.ts`.
- [x] Extract hook tracking + pending-hook action core into `services/workspaceActionHooks.ts`.
- [x] Extract context loading/derivation into `services/workspaceActionContextService.ts`.
- [x] Separate modal state transitions from UI markup.
  Slice landed: extracted modal phase/title/subtitle/size and hook-transition decisions into `services/workspaceActionModalController.ts`.
- [x] Extract workspace mutations into dedicated service.
  Slice landed: added `workspaceActionMutations` gateway/service boundary in `services/workspaceActionService.ts` and routed modal mutations through it.
- [ ] Split large modal sections into components.
- [x] Add tests for action-state transitions and failure paths.

Verification:

- [x] `cd wails-ui/workset/frontend && npm run test -- src/lib/components/workspace-action/*.test.ts`
- [x] `cd wails-ui/workset/frontend && npm run test -- src/lib/components/Workspace*.spec.ts`
- [x] `cd wails-ui/workset/frontend && npm run lint`

## `#117` FE-TERMINAL

Primary file:

- `wails-ui/workset/frontend/src/lib/terminal/terminalService.ts`

Status:

- Partial modularization already landed (transport/state/renderer helpers exist).

Remaining tasks:

- [x] Move lifecycle FSM to standalone module with explicit state graph.
- [x] Move renderer addon wiring (WebGL + web-links sync) into `terminalRenderer.ts`.
- [x] Move web-links transport/renderer adapter wiring into `terminalWebLinks.ts`.
- [x] Extract reconnect/attach/detach stream orchestration into `terminalStreamOrchestrator.ts`.
- [ ] Remove remaining renderer/transport coupling from service shell.
  Slices landed: extracted resize/transport coupling into `terminalResizeBridge.ts`; extracted render-health/recovery orchestration into `terminalRenderHealth.ts`; extracted attach/dispose + renderer-addon state handling into `terminalAttachRendererState.ts`.
  Latest slice landed: extracted Xterm instance attach/dispose wiring into `terminalInstanceManager.ts`.
  Latest slice landed: extracted viewport/resize/focus lifecycle into `terminalViewportResizeController.ts`.
  Latest slice landed: extracted output queue + backlog flush policy into `terminalOutputBuffer.ts`.
- [x] Extract attach/open lifecycle sequencing into a standalone module.
  Slice landed: extracted open/create/connect + retry sequencing into `terminalAttachOpenLifecycle.ts`.
- [x] Extract event subscription wiring into a standalone module.
  Slice landed: extracted event registration/routing/cleanup into `terminalEventSubscriptions.ts`.
- [x] Extract mode/bootstrap coordination into a standalone module.
  Slice landed: extracted mode/bootstrap handling + mismatch guard into `terminalModeBootstrapCoordinator.ts`.
- [x] Extract kitty image/overlay rendering and event application into a standalone module.
  Slice landed: extracted kitty state + overlay + event controller into `terminalKittyImageController.ts`.
- [x] Extract input/filter/retry/session-recovery write path into a standalone module.
  Slice landed: extracted send-input orchestration into `terminalInputOrchestrator.ts`.
- [x] Extract replay/ack buffering orchestration into a standalone module.
  Slice landed: extracted replay/ack coordination into `terminalReplayAckOrchestrator.ts`.
- [x] Add service-level tests for reconnect/attach/detach/stream-release (`terminalStreamOrchestrator.test.ts`).
- [ ] Shrink `terminalService.ts` to orchestration-only facade.

Verification:

- [x] `cd wails-ui/workset/frontend && npm run test -- src/lib/terminal/*.test.ts`
- [x] `go test ./wails-ui/workset -run "TestTerminalSessionReleaseStream|TestEnsureServiceConcurrent" -count=1`

## `#118` FE-PLATFORM

Primary files:

- `wails-ui/workset/frontend/src/lib/api.ts`
- `wails-ui/workset/frontend/src/lib/components/settings/*`

Target architecture:

- `api/` domain clients (`workspaces`, `repos`, `terminal`, `github`, `updates`)
- settings orchestrators separated by domain

Tasks:

- [x] Split monolithic API module into domain entrypoints.
  Slices landed: extracted updates/app-version domain into `api/updates.ts`; extracted GitHub operations into `api/github.ts`; extracted repo-diff watch/diff APIs into `api/repo-diff.ts`; extracted settings/session/group/alias APIs into `api/settings.ts`; extracted workspace APIs into `api/workspaces.ts`; extracted terminal/layout APIs into `api/terminal-layout.ts`, all with compatibility re-exports from `api.ts`.
- [x] Keep backward-compatible imports through adapter layer during migration.
- [x] Remove adapter after callsites are migrated.
  Slice landed: migrated remaining frontend/test callsites to domain API modules, moved skills API into `api/skills.ts`, and removed the `src/lib/api.ts` compatibility barrel.

Verification:

- [x] `cd wails-ui/workset/frontend && npm run test`
- [x] `cd wails-ui/workset/frontend && npm run check`

## `#119` BE-SESSIOND

Primary file:

- `pkg/sessiond/session.go`

Target architecture:

- protocol parser/encoder
- stream/backlog subsystem
- lifecycle/session control subsystem
- persistence/snapshot subsystem

Tasks:

- [x] Extract terminal filter + protocol parsing/logging block into `pkg/sessiond/terminal_filter.go`.
- [x] Extract stream/subscriber fanout + credit handling into `pkg/sessiond/stream.go`.
- [x] Extract persistence/snapshot + transcript/recording subsystem into `pkg/sessiond/session_persist.go`.
- [x] Extract protocol message handling package.
- [x] Extract backlog/snapshot response shaping into `pkg/sessiond/session_response.go`.
- [x] Extract lifecycle + process supervision package.
- [x] Keep public session behavior and API unchanged.
- [x] Add churn tests around create/stop/restore.

Verification:

- [x] `go test ./pkg/sessiond -count=1`
- [x] `go test ./pkg/sessiond -run "TestRepeatedCreateStop|TestSessionCloseWithReasonReapsProcess|TestSessiondSnapshotAndBacklog|TestIdleCloseRecreateSession" -count=1`

## `#120` BE-GITHUB

Primary file:

- `pkg/worksetapi/github_service.go`

Target architecture:

- PR lifecycle service
- review/comment service
- operation orchestration service
- provider integration adapters

Tasks:

- [x] Move pure git command/diff helpers into `pkg/worksetapi/github_git_helpers.go`.
- [x] Separate read vs write helper use-cases into dedicated modules (`github_service_read_helpers.go`, `github_service_write_helpers.go`).
- [x] Separate synchronous status fetch paths into `github_service_status.go`.
- [x] Extract mutating operation orchestration entrypoints into `github_service_write.go`.
- [x] Extract GraphQL thread-mapping helpers into `github_service_thread_graphql_helpers.go`.
- [ ] Add unit tests for each extracted service boundary.
  Tranche 1 landed: read-helper and GraphQL thread helper tests (`github_service_read_helpers_test.go`, `github_service_thread_graphql_helpers_test.go`).
  Tranche 2 landed: read/status/write helper boundary tests (`github_service_read_helpers_test.go`, `github_service_status_test.go`, `github_service_write_helpers_test.go`).

Verification:

- [x] `go test ./pkg/worksetapi -count=1`

## `#121` BE-TERMEMU

Primary file:

- `pkg/termemu/termemu.go`

Target architecture:

- parser
- state machine
- snapshot encoding/output

Tasks:

- [x] Extract parser state machine boundary into `pkg/termemu/parser.go`.
- [x] Extract state transition engine into `pkg/termemu/state_engine.go`.
- [x] Extract snapshot renderer/serializer into `pkg/termemu/snapshot_ansi.go` and `pkg/termemu/snapshot_state.go`.
- [x] Backfill regression tests for escape-sequence edge cases in `pkg/termemu/termemu_test.go`.

Verification:

- [x] `go test ./pkg/termemu -count=1`

## `#122` BE-UPDATER

Primary file:

- `wails-ui/workset/app_updates.go`

Target architecture:

- updater client
- update state machine
- preferences persistence
- Wails binding adapter

Tasks:

- [x] Extract update manifest/asset client helpers into `app_update_client.go`.
- [x] Extract update package/signing helpers into `app_update_package.go`.
- [x] Split update check/start orchestration from app binding layer.
  Slice landed: moved update lifecycle orchestration into `app_update_orchestrator.go` and kept `app_updates.go` as Wails binding surface.
- [x] Isolate signing/asset selection logic.
  Slice landed: added `selectUpdatePackage` validation/selection helper in `app_update_package.go`.
- [x] Add tests for channel preference + state transitions.
  Slice landed: added `app_update_orchestrator_test.go` covering channel preference and check/start phase transitions.

Verification:

- [x] `go test ./wails-ui/workset -run "Test.*Update.*" -count=1`

## `#123` TEST-E2E

Primary file:

- `internal/e2e/e2e_test.go`

Target architecture:

- scenario-based test files
- shared fixture/bootstrap package
- reusable helpers for workspace/repo/session setup

Tasks:

- [x] Split by scenario domain (workspace, repos, sessions, github).
  Slice landed: replaced `e2e_test.go` with scenario files (`repo_test.go`, `workspace_test.go`, `template_group_test.go`, `status_test.go`, `cli_test.go`).
- [x] Extract fixture manager.
  Slice landed: extracted shared runner/git/bootstrap helpers into `main_test.go`, `bootstrap_test.go`, and `helpers_test.go`.
- [x] Keep runtime equivalent or faster vs baseline.
  Current runtime: `go test ./internal/e2e -count=1` ~14-18s in local runs.

Verification:

- [x] `go test ./internal/e2e -count=1`

## Program-Level Verification Gate (Every PR)

- [x] `make check`
- [x] `go test ./...`
- [x] File-size policy check passes.
- [x] No new production file >700 LOC (unless allowlisted).

## Tracking Notes

- Use this file as source of truth for phase/order/checklist status.
- Update checkboxes in the same PR that completes work.
- If a track is blocked, add a short blocker note under that track with the unblock action.

## Immediate Next Actions

1. Run `#117` next slice: extract OSC/theme response handling and clipboard/runtime clipboard helpers out of `terminalService.ts`.
2. Run `#116` next slice: split large `WorkspaceActionModal.svelte` sections into dedicated per-mode components.
3. Run issue closeout pass: verify no regression in `internal/e2e` signing assumptions and finalize `#125` completion checklist.
