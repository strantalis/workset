# Desktop Hardening Execution Tracker

Owner: Codex + Sean  
Last updated: 2026-02-06

## Goal

Stabilize and simplify the desktop architecture so terminal/session reliability is testable and maintainable.

## Success Criteria

- No recurring `too many open files` failures during terminal churn.
- Deterministic terminal lifecycle (start/attach/bootstrap/live/closed/error).
- Clear signal ownership boundaries (OS signals vs terminal protocol vs transport events vs UI health).
- Shared event contract constants across Go backend and frontend listeners.
- Concurrency-safe service initialization in Wails backend.
- Repeatable verification commands documented and runnable.

## Constraints / Current Blockers

- No open blocker currently.
- Historical note: `go1.25.5` toolchain failed during stdlib package discovery with `too many open files in system`; moving module/workspace declarations to `go 1.25.6` resolved this locally.

## Work Plan

### Phase 0: Immediate Stabilization (critical)

- [x] Fix stream/socket release path in terminal stream teardown (`wails-ui/workset/terminal_stream.go`, `wails-ui/workset/terminal_types.go`).
- [x] Add regression tests for stream release semantics (`wails-ui/workset/terminal_stream_lifecycle_test.go`).
- [x] Close parent-side daemon log FD after spawn (`pkg/sessiond/client.go`).
- [x] Verify stress path with repeated start/stop/attach once local FD pressure is cleared.

### Phase 1: Boundary Hardening (in progress)

- [x] Make Wails backend service initialization concurrency-safe (`wails-ui/workset/service_helpers.go`).
- [x] Introduce shared backend event name constants and replace inline strings.
- [x] Introduce shared frontend event name constants and replace inline strings.
- [x] Deduplicate frontend Wails event subscription registry helper.

### Phase 2: Signal Ownership

- [x] Add explicit signal ownership spec doc:
  - OS process signals owner.
  - Terminal protocol signal owner.
  - Stream/lifecycle event owner.
  - UI health/retry timer owner.
- [x] Map each existing signal path to the owning layer and remove cross-layer handling.

### Phase 3: Terminal Lifecycle Simplification

- [x] Unify terminal restore order into one canonical flow (`wails-ui/workset/terminal_state.go`).
- [x] Extract stream transport I/O boundary into `terminalTransport.ts`.
- [x] Extract terminal store registry boundary into `terminalStateStore.ts`.
- [x] Extract terminal renderer factory/web-links boundary into `terminalRenderer.ts`.
- [x] Split `terminalService.ts` into:
  - stream transport module,
  - state machine/store module,
  - xterm UI/rendering module.
- [x] Keep external API stable for `TerminalController.svelte` and `TerminalWorkspace.svelte`.

### Phase 4: Verification and Release Readiness

- [x] Go verification:
  - `go test ./...`
- [x] Frontend verification:
  - `cd wails-ui/workset/frontend && npm run format:check`
  - `cd wails-ui/workset/frontend && npm run lint`
  - `cd wails-ui/workset/frontend && npm run check`
  - `cd wails-ui/workset/frontend && npm run test`
- [x] Repo parity:
  - `make check`
- [x] Add targeted reliability test notes:
  - terminal reconnect after daemon restart,
  - idle timeout close,
  - repeated tab create/close churn,
  - bootstrap replay correctness.

## Detailed Task Checklist

### A. Service Init Safety

- [x] Add `sync.Once` (or equivalent lock) to protect service singleton initialization.
- [x] Ensure all callers still use `a.ensureService()` safely from goroutines.
- [x] Add test for concurrent access if practical.

### B. Event Contract Hygiene

- [x] Add Go constants for:
  - `hooks:progress`
  - `github:operation`
  - `sessiond:restarted`
  - `terminal:*`
  - `repodiff:*`
- [x] Replace hardcoded backend emit strings with constants.
- [x] Add TS constants for the same events.
- [x] Replace hardcoded frontend subscribe strings with constants.

### C. Frontend Event Bus Dedup

- [x] Create one shared `subscribeWailsEvent` helper.
- [x] Migrate:
  - `hookEventService.ts`
  - `githubOperationService.ts`
  - `repoDiffService.ts`
- [x] Keep unsubscribe semantics unchanged.

### D. Testing/Operations Notes

- [x] Add temporary troubleshooting section for local FD cleanup/testing readiness.
- [x] Record exact commands to reproduce terminal churn scenario.

## Temporary FD Troubleshooting (Local)

If terminal starts fail with `open /dev/null: too many open files`, use this sequence before rerunning checks:

1. Inspect limits:
   - `ulimit -n`
   - `launchctl limit maxfiles` (macOS)
2. Find descriptor-heavy processes:
   - `lsof -nP | awk '{print $2}' | sort | uniq -c | sort -nr | head -20`
3. Inspect workset/sessiond specifically:
   - `pgrep -fl "workset|workset-sessiond"`
   - `lsof -nP -p <PID> | wc -l`
4. Clear stale daemon state if needed:
   - `pkill -f workset-sessiond`
   - `rm -f ~/.workset/sessiond.sock`
5. Re-run targeted tests before full suite.

## Terminal Churn Repro Commands

Use these targeted commands once local FD pressure is healthy:

- `go test ./pkg/sessiond -run "TestRepeatedCreateStop|TestSessionCloseWithReasonReapsProcess" -count=1`
- `go test ./wails-ui/workset -run "TestTerminalSessionReleaseStream|TestEnsureServiceConcurrent" -count=1`
- `go test ./pkg/sessiond -run "TestSessiondSnapshotAndBacklog|TestIdleCloseRecreateSession" -count=1`
- `go test ./...`

## Targeted Reliability Notes

1. Terminal reconnect after daemon restart
   - Start a terminal, run a long command (`while true; do date; sleep 1; done`), trigger `RestartSessiondWithReason`.
   - Expect lifecycle `error` then `started`, stream resumes without duplicate listeners.
2. Idle timeout close
   - Set a short terminal idle timeout in settings and wait without interaction.
   - Expect lifecycle `idle` and no stale stream FD retention.
3. Repeated tab create/close churn
   - Repeatedly create/close the same terminal tab and alternate workspace focus.
   - Expect no growth trend in `lsof -nP -p <workset-sessiond-pid> | wc -l`.
4. Bootstrap replay correctness
   - Start terminal, emit buffered output, detach/reattach.
   - Expect single ordered replay (`bootstrap` then `bootstrap_done`) without duplicate backlog chunks.

## Verification Log

- 2026-02-06: `go test ./...` -> failed due to `too many open files in system`.
- 2026-02-06: `make check` -> failed because frontend `prettier` missing.
- 2026-02-06: `make test` -> failed because frontend `@testing-library/svelte` missing.
- 2026-02-06: `go test ./pkg/sessiond -run "TestSessionCloseWithReasonReapsProcess|TestRepeatedCreateStop" -count=1` -> failed due to `too many open files in system`.
- 2026-02-06: `go test ./wails-ui/workset -run "TestEnsureServiceConcurrent|TestTerminalSessionReleaseStream" -count=1` -> failed due to `too many open files in system`.
- 2026-02-06: `npm run format:check` -> failed (`prettier: command not found`).
- 2026-02-06: `npm run lint` -> failed (`eslint: command not found`).
- 2026-02-06: `npm run check` -> failed (missing generated `wailsjs` bindings and frontend deps such as `@testing-library/svelte`, `@xterm/addon-*`, `@lucide/svelte`).
- 2026-02-06: `npm run check` (after transport/state-store extraction) -> still failed with dependency/binding gaps; error count moved from 139 to 137 after typed callback cleanup.
- 2026-02-06: `npm run check` (after renderer extraction) -> still failed with dependency/binding gaps; error count now 138 (includes one renderer option typing issue that was subsequently corrected).
- 2026-02-06: `npm run check | tail -n 5` -> confirms current count `138 errors`.
- 2026-02-06: `npm run test -- repoDiffService.test.ts githubOperationService.test.ts hookEventService.test.ts` -> failed (missing `@testing-library/svelte` in `vite.config.ts` setup).
- 2026-02-06: `make check` -> failed (`prettier: command not found`).
- 2026-02-06: set `go.mod`, `go.work`, `wails-ui/workset/go.mod` to `go 1.25.6`; verified `go list std` and `go build ./cmd/workset-sessiond` pass.
- 2026-02-06: `go test ./...` -> passed.
- 2026-02-06: `go test ./pkg/sessiond -run "TestRepeatedCreateStop|TestSessionCloseWithReasonReapsProcess" -count=1` -> passed.
- 2026-02-06: `go test ./pkg/sessiond -run "TestSessiondSnapshotAndBacklog|TestIdleCloseRecreateSession" -count=1` -> passed.
- 2026-02-06: `go test ./wails-ui/workset -run "TestTerminalSessionReleaseStream|TestEnsureServiceConcurrent" -count=1` -> passed.
- 2026-02-06: `cd wails-ui/workset/frontend && npm run format` -> applied formatting changes.
- 2026-02-06: `cd wails-ui/workset/frontend && npm run format:check` -> passed.
- 2026-02-06: `cd wails-ui/workset/frontend && npm run lint` -> passed.
- 2026-02-06: `cd wails-ui/workset/frontend && npm run check` -> passed (`0 errors`, `0 warnings`).
- 2026-02-06: `cd wails-ui/workset/frontend && npm run test` -> passed (`29` files, `165` tests).
- 2026-02-06: `make check` -> passed.
