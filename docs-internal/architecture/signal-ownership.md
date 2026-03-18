---
description: Ownership boundaries for process, terminal protocol, transport, and UI health signals.
---

# Signal Ownership

This document defines which layer owns each class of signal in the desktop app.

## Purpose

Prevent cross-layer signal handling and reduce lifecycle bugs by making ownership explicit.

## Signal Classes

## 1) OS Process Signals

Examples:

- `SIGTERM`
- `SIGINT`
- app shutdown callbacks

Owner:

- `cmd/workset-sessiond/main.go`
- Wails app lifecycle hooks (`wails-ui/workset/app.go`)

Rules:

- Only process lifecycle code handles OS signals.
- Terminal/session modules must not handle OS signals directly.

## 2) Terminal Protocol Signals

Examples:

- OSC/CSI/DSR sequences
- alt-screen enter/exit
- mouse protocol mode changes

Owner:

- `pkg/sessiond/session.go`

Rules:

- Daemon parses protocol bytes and computes mode state.
- Wails backend forwards typed events only.
- Frontend can perform renderer-level callbacks required by xterm integration (for example OSC color query responses), but this does not replace daemon protocol ownership.

## 3) Stream/Transport Lifecycle Signals

Examples:

- attach/bootstrap/bootstrap_done/data
- lifecycle started/closed/error
- stream credit/ack

Owner:

- Backend stream bridge (`wails-ui/workset/terminal_stream.go`)
- Daemon stream protocol (`pkg/sessiond/server.go`, `pkg/sessiond/session.go`)

Rules:

- Stream setup/teardown paths must always release stream resources.
- Lifecycle event names are treated as API contract and must use shared constants.

## 4) UI Health and Recovery Signals

Examples:

- startup timeout
- render health checks
- reconnect/retry timers

Owner:

- Frontend terminal state machine (`wails-ui/workset/frontend/src/lib/terminal/terminalService.ts`)

Rules:

- Health signals do not parse protocol bytes.
- Recovery actions trigger backend lifecycle APIs; they do not mutate daemon state directly.

## Shared Event Contract

Backend event names are defined in:

- `wails-ui/workset/events.go`

Frontend event names are defined in:

- `wails-ui/workset/frontend/src/lib/events.ts`

Changes to event names must be made in both files in one change set.

## Current Signal Path Map

The table below maps active paths to the owning layer and expected boundary behavior.

| Signal / Event | Producer | Owner | Consumer | Notes |
| --- | --- | --- | --- | --- |
| `SIGTERM`, `SIGINT` | OS | `cmd/workset-sessiond/main.go` | daemon shutdown | No terminal/session module should trap these directly. |
| App startup/shutdown | Wails runtime | `wails-ui/workset/app.go` | service/session bootstrap | Startup triggers session restore; shutdown persists state and closes sessions. |
| PTY byte stream | child process via PTY | `pkg/sessiond/session.go` | `pkg/sessiond/server.go` attach stream | Protocol parsing happens in daemon only. |
| `terminal:bootstrap` | daemon stream bridge | `wails-ui/workset/terminal_stream.go` | `terminalService.ts` | Frontend receives typed bootstrap payload only. |
| `terminal:data` | daemon stream bridge | `wails-ui/workset/terminal_stream.go` | `terminalService.ts` renderer path | Frontend never parses ownership-level protocol state from raw bytes. |
| `terminal:modes` | daemon mode tracker | `pkg/sessiond/session.go` + `wails-ui/workset/terminal_stream.go` | `terminalService.ts` | Mode state (alt-screen/mouse) is emitted as explicit state, not inferred in UI. |
| `terminal:lifecycle` | backend lifecycle control | `wails-ui/workset/terminal_manager.go`, `wails-ui/workset/terminal_stream.go` | `terminalService.ts` | UI reacts to lifecycle only; no direct daemon mutation. |
| `sessiond:restarted` | backend restart manager | `wails-ui/workset/app_sessiond.go` | `terminalService.ts` restart handler | UI resets local state and requests restart through backend APIs. |

## Cross-Layer Cleanup Applied

- Wails event subscriptions now route through one shared frontend registry helper (`wailsEventRegistry.ts`) including terminal service listeners.
- Terminal restore now follows one canonical backend merge flow (layout + daemon running sessions + persisted state) to avoid layer-specific restore branching.
