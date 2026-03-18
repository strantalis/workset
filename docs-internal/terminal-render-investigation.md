# Terminal Render Investigation Log

## Repro Baseline

- Environment: Workset desktop app (WKWebView/macOS)
- Repro:
1. Open 2 panes.
2. In one pane, open 2 tabs.
3. Switch tabs in that pane.
4. Click/focus back into the other pane.
5. Observe glyph corruption/partial redraw. Scrolling or selection often restores rendering.

## Experiment Log

| ID | Change | Status | Result |
| --- | --- | --- | --- |
| 001 | Removed inactive pane opacity/scrim variants | Complete | No meaningful change in corruption rate. |
| 002 | Ordered stream chunk flush (prevent stale/out-of-order writes) | Complete | Helped stale stream safety, did not eliminate pane-switch corruption. |
| 003 | Active-flip attach path (instead of focus-only) | Complete | Did not eliminate corruption. |
| 004 | KISS pass: simplify `terminalSyncController` (remove mount generation/pending microtask queue) | Complete | Reduced complexity, corruption still reproducible. |
| 005 | KISS pass: simplify renderer nudge to single fit+refresh | Complete | Corruption still reproducible in interactive testing. |
| 006 | KISS pass: simplify attach lifecycle (single host mount via `replaceChildren`, remove deferred fit retries) | Complete | Pending user validation after latest cut. |
| 007 | KISS pass: enforce single terminal owner per container (deterministic displacement detach in sync controller) | Complete | Pending user validation after latest cut. |
| 008 | KISS pass: skip inactive active-flip attach work and skip fit/nudge for non-renderable hosts (0-size/disconnected) | Rolled back (partial) | Inactive-flip attach skip caused input regression (`frontend_input_write_failed` bursts after pane switches). Reverted inactive-skip; kept renderability fit/nudge guard. |
| 009 | Reassert stream on active flip (`controller.active_change`) | Complete | Active flip now requests stream sync (`reason: active_flip`); reduced owner/input flaps but did not remove all lockups. |
| 010 | Ordered-stream deadlock recovery on forced flush (gap-tolerant recovery + blocked-flush diagnostics) | In progress | Logs showed `frontend_output_enqueued_ordered` growth with no `frontend_output_flush_ordered` (output arriving but never draining). Forced flush now recovers across missing seq gaps and logs `frontend_output_flush_ordered_blocked` when queued output cannot flush. |
| 011 | Simplified renderer baseline: disable WebGL addon and image addon in orchestration | In progress | Establishes a minimal xterm core-renderer path for WKWebView triage and removes two addon-specific artifact vectors. Pending interactive validation. |
| 012 | Restore xterm default `rescaleOverlappingGlyphs` (`false`) in core renderer | In progress | Targets horizontal seam/scanline artifacts seen on block/pixel-heavy terminal output in non-WebGL mode. Pending interactive validation. |
| 013 | Terminal-specific fallback font stack (`SF Mono`/`Menlo`) for non-WebGL raster path | In progress | Targets block/box glyph seam artifacts that vary by font metrics and rasterization behavior. Pending interactive validation. |
| 014 | Upstream beta bump: `@xterm/xterm` `6.1.0-beta.166` + `@xterm/addon-image` `0.10.0-beta.166` | In progress | Isolates upstream renderer/addon fixes from local lifecycle logic changes. Pending interactive validation. |
| 015 | WebGL re-enabled + `rebuildAtlas` extended to `!wasActive` transitions | Partial | Corruption is at the **cell-metric level**, not atlas level. Scattered characters with large horizontal gaps on pane switch — WebGL renderer has wrong `cellWidth`/canvas pixel dimensions after DOM move. Root cause: synchronous `fit()` runs before the browser commits new container layout. `clearTextureAtlas()` alone does not fix this. |
| 016 | WebGL re-enabled + single RAF-deferred `fit()+refresh()` after synchronous nudge | Failed | RAF fit is a no-op: if cols/rows haven't changed, `terminal.resize()` doesn't fire and the WebGL canvas pixel dimensions are never corrected. |
| 017 | WebGL re-enabled + dispose+recreate WebGL addon on `rebuildAtlas` transitions | In progress | Root cause: WebGL canvas pixel dimensions go stale on DOM moves and cannot be corrected by `clearTextureAtlas()` or `fit()` alone (resize is a no-op when cols/rows are unchanged). Fix: dispose and recreate the WebGL addon (`reinitWebgl`) on every inactive→active transition, forcing the renderer to re-initialize against the current container geometry. Added `reinitWebgl` to `terminalInstanceManager`; wired via forward ref into `nudgeRenderer`. Pending interactive validation. |

## Current Working Hypothesis

- Main failure includes ordered-stream deadlock when a seq gap appears: output keeps arriving and queuing, but strict in-order flush blocked all rendering.
- WebGL cell-metric corruption on pane switch is a **layout timing problem**: the WebGL renderer recalculates `cellWidth`/canvas pixel dimensions during `terminal.resize()` (triggered by `fitAddon.fit()`). If `fit()` runs before the browser has committed the container's new layout position, the renderer captures stale dimensions. One RAF-deferred `fit()+refresh()` after the synchronous pass is the minimal targeted fix.

## Next Controlled Tests

1. Validate experiment `016`: reproduce the 2-pane / 2-tab switching sequence and confirm scattered-glyph corruption is gone with WebGL enabled.
2. Validate that ASCII art / block char row seams are eliminated under WebGL path.
3. Validate experiment `010` still prevents unbounded `frontend_output_enqueued_ordered` growth.
