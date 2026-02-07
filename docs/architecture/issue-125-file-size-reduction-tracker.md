# Issue #125 File-Size Reduction Tracker

Owner: Sean + Codex  
Source issue: `https://github.com/strantalis/workset/issues/125`  
Last updated: 2026-02-07 (subagent pass 9)

## Goal

Reduce architecture risk from oversized files by splitting high-complexity modules into stable boundaries with tests and CI guardrails.

## Program Targets

- No production source file exceeds 700 LOC unless explicitly allowlisted with justification.
- CI blocks regressions on file-size and test/lint failures.
- Refactors are behavior-preserving and land in small, reversible commits.

## Current Baseline (2026-02-07)

Largest files by LOC right now:

- `wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte` (4181)
- `wails-ui/workset/frontend/src/lib/components/WorkspaceActionModal.svelte` (2651)
- `wails-ui/workset/frontend/src/lib/terminal/terminalService.ts` (2294)
- `pkg/termemu/termemu.go` (1714)
- `wails-ui/workset/frontend/src/lib/api.ts` (1312)
- `pkg/worksetapi/github_service.go` (1295)
- `wails-ui/workset/app_updates.go` (1114)
- `wails-ui/workset/frontend/src/lib/components/TerminalWorkspace.svelte` (1061)

## Parallel Tracks (Issue Map)

- [x] `#124` Guardrails (must start first)
- [ ] `#115` FE-DIFF (slice 1 landed)
- [ ] `#116` FE-WORKSPACE (slice 4 landed)
- [ ] `#117` FE-TERMINAL (slice 2 landed)
- [ ] `#118` FE-PLATFORM
- [x] `#119` BE-SESSIOND (structural splits complete)
- [ ] `#120` BE-GITHUB (slice 4 landed)
- [ ] `#121` BE-TERMEMU
- [ ] `#122` BE-UPDATER
- [ ] `#123` TEST-E2E

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

- [ ] Split monolithic e2e suite into scenario files + shared fixtures.
- [ ] Align test package/module boundaries with refactored production modules.

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
- [ ] Extract summary loader/store module.
- [ ] Extract PR status/reviews controller.
- [ ] Extract render queue/selection/file-fetch controller.
- [ ] Extract annotation/reply/edit/delete actions module.
- [ ] Keep current public props/events unchanged.

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
- [ ] Separate modal state transitions from UI markup.
- [ ] Extract workspace mutations into dedicated service.
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
- [ ] Remove remaining renderer/transport coupling from service shell.
- [ ] Add service-level tests for reconnect/attach/detach/stream-release.
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

- [ ] Split monolithic API module into domain entrypoints.
- [ ] Keep backward-compatible imports through adapter layer during migration.
- [ ] Remove adapter after callsites are migrated.

Verification:

- [ ] `cd wails-ui/workset/frontend && npm run test`
- [ ] `cd wails-ui/workset/frontend && npm run check`

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
- [ ] Add unit tests for each extracted service boundary.

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

- [ ] Extract parser package and fixtures.
- [ ] Extract state transition engine.
- [ ] Extract snapshot renderer/serializer.
- [ ] Backfill regression tests for escape-sequence edge cases.

Verification:

- [ ] `go test ./pkg/termemu -count=1`

## `#122` BE-UPDATER

Primary file:

- `wails-ui/workset/app_updates.go`

Target architecture:

- updater client
- update state machine
- preferences persistence
- Wails binding adapter

Tasks:

- [ ] Split update check/start orchestration from app binding layer.
- [ ] Isolate signing/asset selection logic.
- [ ] Add tests for channel preference + state transitions.

Verification:

- [ ] `go test ./wails-ui/workset -run "Test.*Update.*" -count=1`

## `#123` TEST-E2E

Primary file:

- `internal/e2e/e2e_test.go`

Target architecture:

- scenario-based test files
- shared fixture/bootstrap package
- reusable helpers for workspace/repo/session setup

Tasks:

- [ ] Split by scenario domain (workspace, repos, sessions, github).
- [ ] Extract fixture manager.
- [ ] Keep runtime equivalent or faster vs baseline.

Verification:

- [ ] `go test ./internal/e2e -count=1`

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

1. Run `#120` slice 5: move GraphQL/thread mapping helpers out of `github_service.go`.
2. Run `#116` slice 5: move context-loading/derivation out of `WorkspaceActionModal.svelte`.
3. Run `#117` slice 3: remove remaining transport/renderer coupling from `terminalService.ts`.
