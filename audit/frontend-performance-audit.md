# Frontend Performance Audit

Date: 2026-03-22

## Scope

This audit covers two fronts:

1. Startup and bundle cost in `wails-ui/workset/frontend`
2. Runtime scaling for repo-heavy and terminal-heavy sessions

This is an audit and prioritization pass, not a patch set.

## Baseline

Commands run:

- `cd wails-ui/workset/frontend && npm run check`
- `cd wails-ui/workset/frontend && npm run build`
- `cd wails-ui/workset/frontend && du -sh dist && find dist/assets -type f | wc -l`

Measured baseline:

- `svelte-check`: `0 errors`, `0 warnings`
- Production build: `6245 modules transformed`
- Main entry chunk: `dist/assets/index-dA4EPxY8.js` at `2,020.96 kB`, `619.93 kB gzip`
- Main CSS: `dist/assets/index-DucJTur0.css` at `199.30 kB`, `29.67 kB gzip`
- Built output size: `15M dist`
- Built asset count: `380`

Notable build warning:

- Vite reports that [`terminalService.ts`](../wails-ui/workset/frontend/src/lib/terminal/terminalService.ts) cannot be moved to a separate chunk because it is both statically and dynamically imported.

## Executive Summary

The frontend currently has two real performance risks:

1. Repo updates fan out too broadly. We watch and recompute against more repo state than the user is actively looking at.
2. The startup path is too heavy. The main bundle is already above the point where "we can optimize later" is a safe posture.

The repo-side risk is the more urgent one because it compounds as Sean adds more repos and as backend diff/status events get noisier. The terminal side is less dire than it first sounds: current code appears to scale mostly with mounted panes in the active tab, not with all terminals ever created across hidden tabs. That said, hundreds of simultaneously visible panes would still be expensive.

## Scaling Answers

### How the app scales with many repos

Today the app scales worse with "all loaded repos" than with "currently visible repos."

- [`App.svelte`](../wails-ui/workset/frontend/src/App.svelte) creates app-level repo watchers and syncs them from the full `$workspaces` collection on every relevant store update.
- [`createRepoStatusWatchers.ts`](../wails-ui/workset/frontend/src/lib/composables/createRepoStatusWatchers.ts) walks every non-archived, non-placeholder repo and starts either a local watch or a full PR diff watch for it.
- [`state.ts`](../wails-ui/workset/frontend/src/lib/state.ts) pushes repo events back through the top-level `workspaces` store, which causes downstream regrouping and resorting in explorer and workset views.

For a workspace with 10 repos this is probably still acceptable on a modern machine, but the design trend is wrong. More repos means more watcher lifecycles, more Wails events, more whole-store updates, and more repeated tree/summarization work in the UI.

### How the app scales with many terminals

Today terminal cost looks closer to "active tab pane count" than "total terminal count across tabs."

- [`TerminalWorkspace.svelte`](../wails-ui/workset/frontend/src/lib/components/TerminalWorkspace.svelte) renders `workspaceTabs`, but only mounts the active tab root through a single [`TerminalLayoutNode`](../wails-ui/workset/frontend/src/lib/components/TerminalLayoutNode.svelte) subtree.
- Hidden tabs still exist in layout state and may be prewarmed, but they are not all rendered as live terminal panes at once.
- Each mounted [`TerminalPane.svelte`](../wails-ui/workset/frontend/src/lib/components/TerminalPane.svelte) still brings store subscriptions, sync work, resize handling, and optional `requestAnimationFrame` sampling when debug mode is enabled.

That means:

- `100` terminals spread across inactive tabs is materially less scary than `100` visible panes in the active tab.
- `10` to `25` visible panes is the more relevant runtime stress case.

## Findings

### High: App-level repo watcher fanout scales with loaded repos, not visible repos

Evidence:

- [`App.svelte:115`](../wails-ui/workset/frontend/src/App.svelte#L115) creates `repoStatusWatchers`
- [`App.svelte:402-461`](../wails-ui/workset/frontend/src/App.svelte#L402) subscribes to global repo diff/status/PR event streams
- [`App.svelte:491-493`](../wails-ui/workset/frontend/src/App.svelte#L491) syncs watchers from `$workspaces`
- [`createRepoStatusWatchers.ts:58-121`](../wails-ui/workset/frontend/src/lib/composables/createRepoStatusWatchers.ts#L58) iterates all eligible repos and starts or updates watches

Impact:

- Repo watcher count grows with all loaded workspaces, even when the user only has one workset or one repo surface open.
- PR-open repos are more expensive because they upgrade into full diff watches.
- This creates avoidable background churn before the repo surface is even opened.

Recommendation:

- Change the watcher contract to accept a filtered target set derived from the active workspace, expanded repos, visible repo surfaces, and any PR drawer that is actually open.

### High: Repo events trigger whole-store remaps

Evidence:

- [`state.ts:272-335`](../wails-ui/workset/frontend/src/lib/state.ts#L272) applies repo patches by mapping the full `workspaces` array
- [`state.ts:338-380`](../wails-ui/workset/frontend/src/lib/state.ts#L338) routes diff/status/PR events through `applyRepoPatch`
- [`state.ts:382-395`](../wails-ui/workset/frontend/src/lib/state.ts#L382) review-comment updates do another full-store walk

Impact:

- A single repo diff event can cause explorer summaries, workset summaries, active selection lookups, and shortcut maps to re-run even if only one repo changed.
- This design gets more expensive as the number of workspaces and repos grows.

Recommendation:

- Stop storing high-churn repo runtime state exclusively inside the top-level `workspaces` array.
- Split repo runtime projections into a dedicated keyed store or cache so repo events update one repo entry without remapping unrelated workspaces.

### High: `UnifiedRepoView` does repeated whole-map derivation work and eager multi-repo search indexing

Evidence:

- [`UnifiedRepoView.svelte:398-513`](../wails-ui/workset/frontend/src/lib/components/views/UnifiedRepoView.svelte#L398) rebuilds `activeDiffMap`, `changedFileSet`, repo stats, directory change counts, and comment counts from whole maps
- [`UnifiedRepoView.svelte:533-548`](../wails-ui/workset/frontend/src/lib/components/views/UnifiedRepoView.svelte#L533) loads up to `5000` indexed files per expanded repo when search begins
- [`UnifiedRepoView.svelte:762-776`](../wails-ui/workset/frontend/src/lib/components/views/UnifiedRepoView.svelte#L762) also starts per-repo local status watches for expanded repos
- [`UnifiedRepoView.svelte:1065-1077`](../wails-ui/workset/frontend/src/lib/components/views/UnifiedRepoView.svelte#L1065) polls PR review comments every `10s` while the PR lifecycle drawer is open
- [`UnifiedRepoView.svelte:1149-1189`](../wails-ui/workset/frontend/src/lib/components/views/UnifiedRepoView.svelte#L1149) subscribes to local summary events and refreshes caches and the current file on each event

Impact:

- Search cost scales with expanded repo count, not selected repo count.
- Tree badge work scales with all diff/comment entries every time the source maps change.
- Repo view performance gets worse exactly in the sessions where Sean is doing the most active code review and branch work.

Recommendation:

- Stop eager full-index loading across every expanded repo when search starts.
- Segment the file tree, diff stats, and comment badge pipelines so they only recompute for the repo that changed.
- De-duplicate repo watch ownership between the app shell and `UnifiedRepoView`.

### High: Startup bundle is oversized and heavy features are still on the hot path

Evidence:

- Production build emitted a `2,020.96 kB` main JS chunk and `15M` total `dist`
- [`SpacesWorkbenchView.svelte:5-6`](../wails-ui/workset/frontend/src/lib/components/views/SpacesWorkbenchView.svelte#L5) statically imports both `TerminalWorkspace` and `UnifiedRepoView`
- [`UnifiedRepoView.svelte:81-85`](../wails-ui/workset/frontend/src/lib/components/views/UnifiedRepoView.svelte#L81) statically imports markdown rendering, editor, and diff components
- [`documentRender.ts:1-3`](../wails-ui/workset/frontend/src/lib/documentRender.ts#L1) statically imports `shiki`
- [`documentRender.ts:174-255`](../wails-ui/workset/frontend/src/lib/documentRender.ts#L174) dynamically loads Mermaid, but highlighted markdown still pays the Shiki cost immediately
- [`vite.config.ts:17-43`](../wails-ui/workset/frontend/vite.config.ts#L17) has no explicit chunking strategy

Impact:

- Users are paying for code-viewing, diffing, markdown highlighting, diagram rendering, and other heavy UI capabilities before they ask for them.
- The large entry chunk also makes every future feature cost more because the baseline is already high.

Recommendation:

- Split workbench surfaces by intent, not just by component boundary.
- Move editor, diff, and markdown rendering behind dynamic imports or async component boundaries.
- Add explicit Vite chunking for heavy editor/rendering/diagram groups rather than relying on default Rollup behavior.

### Medium: Terminal runtime cost is bounded better than repo cost, but visible-pane scaling is still real

Evidence:

- [`TerminalWorkspace.svelte:76-80`](../wails-ui/workset/frontend/src/lib/components/TerminalWorkspace.svelte#L76) tracks tabs and active tab state
- [`TerminalWorkspace.svelte:555-660`](../wails-ui/workset/frontend/src/lib/components/TerminalWorkspace.svelte#L555) renders only `currentWorkspaceTab?.root`
- [`TerminalPane.svelte:43-52`](../wails-ui/workset/frontend/src/lib/components/TerminalPane.svelte#L43) uses `requestAnimationFrame` for scroll-state refresh when hover-driven affordances are active
- [`TerminalPane.svelte:144-160`](../wails-ui/workset/frontend/src/lib/components/TerminalPane.svelte#L144) starts another `requestAnimationFrame` loop for performance sampling when debug mode is enabled

Impact:

- The design is meaningfully safer than "mount every terminal everywhere."
- But a large active split layout still multiplies controller subscriptions, resize work, sync work, and optional RAF loops.

Recommendation:

- Keep the current "active tab only" render boundary.
- Treat `10` to `25` visible panes as the practical scaling target.
- Avoid adding any new per-pane polling or per-pane document/window listeners.

### Medium: Explorer and workset summaries still regroup and resort from the top

Evidence:

- [`App.svelte:160-179`](../wails-ui/workset/frontend/src/App.svelte#L160) rebuilds visible workspaces, summaries, shortcuts, and context state from `$workspaces`
- [`ExplorerPanel.svelte`](../wails-ui/workset/frontend/src/lib/components/chrome/ExplorerPanel.svelte) groups worksets, deduplicates repos, and aggregates repo health from whole workspace input
- [`worksetViewModel.ts`](../wails-ui/workset/frontend/src/lib/view-models/worksetViewModel.ts) maps and sorts workspaces into summary models on demand

Impact:

- The explorer likely stays fine with small data, but it still rides the same whole-store churn created by repo events.

Recommendation:

- Once repo runtime state is split out, keep explorer/workset summaries on slower-moving structural workspace state and patch in repo status separately.

## Ranked Fix List

1. Narrow app-level repo watcher scope to visible and active repos only.
2. Move high-churn repo runtime state out of the top-level `workspaces` array.
3. Break `UnifiedRepoView` derivations into per-repo segments and stop eager multi-repo search indexing.
4. Chunk the workbench surfaces so editor, diff, markdown, and diagram code stop inflating the startup path.
5. Keep terminal rendering scoped to the active tab and validate visible-pane stress limits before adding more terminal UI overlays.
6. Add a lightweight perf budget command to keep bundle regressions visible in normal dev flow.

## Recommended Acceptance Criteria For The Follow-up Patch Set

- Repo watcher count scales with visible and active repos, not all loaded repos.
- A single repo diff/status event does not remap every workspace record.
- Starting file search does not index every expanded repo by default.
- The startup path no longer loads editor, diff, markdown highlighter, and diagram code up front.
- The main entry chunk falls well below the current `2.02 MB` raw baseline.

## How To Refresh This Audit

- `cd wails-ui/workset/frontend && npm run check`
- `cd wails-ui/workset/frontend && npm run perf:audit:build`

The second command rebuilds the frontend and prints a repeatable dist summary using the helper added in this change.
