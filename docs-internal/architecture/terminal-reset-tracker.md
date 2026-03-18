# Terminal Simplification Plan

## Goal

- Keep terminal runtime minimal and deterministic.
- Remove tmux-style restart/replay orchestration from app/frontend.
- Preserve only one advanced behavior: handoff between main window and popout window.

## Target Architecture

### Backend (`sessiond` + Wails app)

- `sessiond` is source of truth for PTY session lifecycle and input lease ownership.
- Wails app forwards start/write/resize/ack with window identity; no local restart logic.
- Ownership map in app layer is advisory for UX only (who currently controls input).

### Frontend (`terminalService`)

- Single data path:
  - subscribe `terminal:data`
  - write bytes directly to xterm
  - ack bytes immediately
- No bootstrap replay coordinator in active rendering path.
- No app-side restart manager or synthetic reattach state machine.

### Popout/Main Handoff

- On popout open:
  - transfer input lease to popout window.
- On popout close/return:
  - transfer input lease back to main window.
- Stream stays live; only input owner changes.

## xterm.js Add-ons

- `@xterm/addon-fit`: required for responsive layout and resize -> PTY size sync.
- `@xterm/addon-serialize`: for optional visual handoff snapshot during popout/main transition only.
- `@xterm/addon-web-links`: keep enabled for clickable links when mouse reporting is not active.
- Avoid adding more addons unless they solve a proven terminal-specific problem.

## Execution Phases

### Phase 1 (done in progress)

- Remove persisted terminal restore/restart logic in backend.
- Enforce session lease checks in `sessiond` writes.
- Route ownership-sensitive writes through window-aware APIs.

### Phase 2 (done in progress)

- Collapse frontend to direct data handling.
- Remove bootstrap/replay event wiring from `terminalService`.
- Keep session attach/start flows and ownership-aware transport only.

### Phase 3 (next)

- Add explicit handoff snapshot path using `SerializeAddon`:
  - capture buffer from current owner before transfer.
  - hydrate target terminal immediately.
  - continue with live stream.
- Add focused e2e coverage for:
  - main -> popout transfer
  - popout -> main transfer
  - owner mismatch rejection and recovery.

## Guardrails

- No hidden restart behavior in frontend or app layer.
- One owner can write at a time; mismatched owner writes are rejected.
- Keep changes small and reversible; remove dead code as soon as equivalent path is stable.
