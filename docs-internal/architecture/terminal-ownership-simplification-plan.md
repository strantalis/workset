---
description: Execution plan to simplify terminal ownership and streaming during the Wails v3 migration.
---

# Terminal Ownership Simplification Plan

## Objective

Move to a simpler and more reliable terminal model where `workset-sessiond` is the single authority for terminal lifecycle, stream state, and input ownership, while the desktop app and frontend stay as transport + rendering layers.

## Current pain points

- App-layer ownership checks reject calls during popout/handoff and generate noisy binding errors.
- Frontend and backend both hold parts of replay/attach/ack state, which creates race conditions.
- Multiple orchestration layers make terminal recovery hard to reason about.

## Scope

- In scope:
  - Session ownership and stream lifecycle.
  - Workspace popout terminal handoff behavior.
  - Replay/bootstrap/ack simplification.
  - Go/TS tests and e2e coverage for handoff and reconnect flows.
- Out of scope:
  - Typography and visual redesign changes unrelated to terminal behavior.

## Milestones

### M0. Baseline and reproducibility

- [ ] Capture reproducible steps for:
  - [ ] Popout handoff.
  - [ ] Session restart/reconnect.
  - [ ] Terminal corruption cases.
- [ ] Capture baseline logs and expected user-visible behavior.

Exit criteria:
- A deterministic local repro script and baseline pass/fail matrix exists.

### M1. Contract and state-machine definition

- [ ] Document single state machine for terminal session lifecycle:
  - `idle -> bootstrapping -> live -> closed`.
- [ ] Define ownership contract:
  - viewer attach identity.
  - input lease owner.
  - lease transfer semantics.
- [ ] Define stream contract:
  - monotonic sequencing.
  - bootstrap snapshot + backlog behavior.
  - ack/credit rules.

Exit criteria:
- Contract documented and mapped to protocol fields before implementation.

### M2. Sessiond ownership authority

- [x] Add ownership/lease state to `sessiond` sessions.
- [ ] Enforce input writes through lease checks in `sessiond`.
- [x] Add explicit lease transfer API (main <-> popout).
- [ ] Keep owner-independent read-only attach for viewers.

Exit criteria:
- Ownership policy is enforced by `sessiond`, not by app-layer guards.

### M3. App-layer simplification (Wails backend)

- [x] Make app-layer terminal ownership checks advisory (no hard rejection).
- [ ] Remove redundant app-layer ownership gating once M2 is complete.
- [ ] Keep app-layer owner info only for UI hints/telemetry.
- [ ] Ensure window close/open events call lease transfer APIs instead of local owner map updates.

Exit criteria:
- App backend no longer blocks terminal operations based on local owner map.

### M4. Frontend orchestration simplification

- [ ] Collapse replay/bootstrap transitions into one coordinator.
- [ ] Keep exactly one output queue into xterm.
- [ ] Keep byte-safe payload path only.
- [ ] Remove legacy buffering branches that exist only to recover ownership races.

Exit criteria:
- Frontend terminal flow has one deterministic bootstrap path and one live path.

### M5. Workspace popout behavior

- [ ] Popout from worksets view becomes first-class workspace surface.
- [ ] Transfer input lease to popout on open.
- [ ] Transfer input lease back to main on popout close.
- [ ] Remove cockpit-only assumptions for popout controls.

Exit criteria:
- A workspace can be popped out with full interaction and stable terminal behavior.

### M6. Validation and rollout

- [ ] Unit tests:
  - [ ] `pkg/sessiond` ownership/lease and stream tests.
  - [ ] `wails-ui/workset` backend ownership/bridge tests.
  - [ ] frontend terminal orchestration tests.
- [ ] E2E tests:
  - [ ] main view terminal.
  - [ ] open popout.
  - [ ] input lease handoff both directions.
  - [ ] reconnect after session restart.
- [ ] Add feature flag + fallback path for staged rollout.

Exit criteria:
- Full test suite coverage for handoff/reconnect and no known regressions.

## Acceptance criteria

- No recurring `workspace terminal is owned by window ...` binding errors in normal flows.
- No recurring `session not found` / `terminal not started` loops during popout handoff.
- Popout close/open cycles preserve terminal continuity and interactive control.
- Terminal output remains byte-accurate with no parser corruption caused by transport.

## Execution order

1. M0 baseline capture.
2. M1 contract.
3. M2 sessiond ownership authority.
4. M3 app simplification.
5. M4 frontend simplification.
6. M5 UX completion.
7. M6 test hardening + rollout.

## Tracking notes

- Keep this document updated as the source of truth for migration status.
- Keep each milestone to small, reversible PRs.
- Prefer explicit behavior flags for rollout rather than big-bang replacement.
