<script lang="ts">
  import TerminalLayoutNode from './TerminalLayoutNode.svelte'
  import {createWorkspaceTerminal} from '../api'
  import {StartWorkspaceTerminal} from '../../../wailsjs/go/main/App'

  interface Props {
    workspaceId: string
    workspaceName: string
    active?: boolean
  }

  let { workspaceId, workspaceName, active = true }: Props = $props()

  type TerminalTab = {
    id: string
    terminalId: string
    title: string
  }

  type PaneNode = {
    id: string
    kind: 'pane'
    tabs: TerminalTab[]
    activeTabId: string
  }

  type SplitNode = {
    id: string
    kind: 'split'
    direction: 'row' | 'column'
    ratio: number
    first: LayoutNode
    second: LayoutNode
  }

  type LayoutNode = PaneNode | SplitNode

  type TerminalLayout = {
    version: number
    root: LayoutNode
    focusedPaneId?: string
  }

  const STORAGE_VERSION = 1
  const STORAGE_PREFIX = 'workset:terminal-layout:'

  let layout = $state<TerminalLayout | null>(null)
  let initError = $state('')
  let loading = $state(false)

  const newId = (): string => {
    if (typeof crypto !== 'undefined' && crypto.randomUUID) {
      return crypto.randomUUID()
    }
    return `term-${Math.random().toString(36).slice(2)}`
  }

  const storageKey = (id: string): string => `${STORAGE_PREFIX}${id}`

  const saveLayout = (next: TerminalLayout): void => {
    if (typeof localStorage === 'undefined') return
    try {
      localStorage.setItem(storageKey(workspaceId), JSON.stringify(next))
    } catch {
      // Ignore storage failures.
    }
  }

  const loadLayout = (): TerminalLayout | null => {
    if (typeof localStorage === 'undefined') return null
    try {
      const raw = localStorage.getItem(storageKey(workspaceId))
      if (!raw) return null
      const parsed = JSON.parse(raw) as TerminalLayout
      if (!parsed || parsed.version !== STORAGE_VERSION || !parsed.root) {
        return null
      }
      return parsed
    } catch {
      return null
    }
  }

  const collectTabs = (node: LayoutNode, tabs: TerminalTab[] = []): TerminalTab[] => {
    if (node.kind === 'pane') {
      return tabs.concat(node.tabs)
    }
    collectTabs(node.first, tabs)
    collectTabs(node.second, tabs)
    return tabs
  }

  const collectPaneIds = (node: LayoutNode, ids: string[] = []): string[] => {
    if (node.kind === 'pane') {
      ids.push(node.id)
      return ids
    }
    collectPaneIds(node.first, ids)
    collectPaneIds(node.second, ids)
    return ids
  }

  const findPane = (node: LayoutNode, paneId: string): PaneNode | null => {
    if (node.kind === 'pane') {
      return node.id === paneId ? node : null
    }
    return findPane(node.first, paneId) || findPane(node.second, paneId)
  }

  const updatePane = (node: LayoutNode, paneId: string, updater: (pane: PaneNode) => PaneNode): LayoutNode => {
    if (node.kind === 'pane') {
      return node.id === paneId ? updater(node) : node
    }
    const first = updatePane(node.first, paneId, updater)
    const second = updatePane(node.second, paneId, updater)
    if (first === node.first && second === node.second) {
      return node
    }
    return {...node, first, second}
  }

  const splitPane = (node: LayoutNode, paneId: string, direction: 'row' | 'column', pane: PaneNode): LayoutNode => {
    if (node.kind === 'pane') {
      if (node.id !== paneId) {
        return node
      }
      return {
        id: newId(),
        kind: 'split',
        direction,
        ratio: 0.5,
        first: node,
        second: pane
      }
    }
    const first = splitPane(node.first, paneId, direction, pane)
    const second = splitPane(node.second, paneId, direction, pane)
    if (first === node.first && second === node.second) {
      return node
    }
    return {...node, first, second}
  }

  const removePane = (node: LayoutNode, paneId: string): LayoutNode | null => {
    if (node.kind === 'pane') {
      return node.id === paneId ? null : node
    }
    const first = removePane(node.first, paneId)
    const second = removePane(node.second, paneId)
    if (!first && !second) return null
    if (!first) return second
    if (!second) return first
    return {...node, first, second}
  }

  const updateSplitRatio = (node: LayoutNode, splitId: string, ratio: number): LayoutNode => {
    if (node.kind === 'pane') return node
    if (node.id === splitId) {
      return {...node, ratio}
    }
    const first = updateSplitRatio(node.first, splitId, ratio)
    const second = updateSplitRatio(node.second, splitId, ratio)
    if (first === node.first && second === node.second) return node
    return {...node, first, second}
  }

  const moveTab = (
    node: LayoutNode,
    sourcePaneId: string,
    targetPaneId: string,
    tabId: string,
    targetIndex: number
  ): LayoutNode => {
    // Find the tab in source pane
    const sourcePane = findPane(node, sourcePaneId)
    if (!sourcePane) return node
    const tab = sourcePane.tabs.find((t) => t.id === tabId)
    if (!tab) return node

    // If same pane, just reorder
    if (sourcePaneId === targetPaneId) {
      return updatePane(node, sourcePaneId, (pane) => {
        const tabs = pane.tabs.filter((t) => t.id !== tabId)
        tabs.splice(targetIndex, 0, tab)
        return {...pane, tabs}
      })
    }

    // Remove from source
    let updated = updatePane(node, sourcePaneId, (pane) => ({
      ...pane,
      tabs: pane.tabs.filter((t) => t.id !== tabId),
      activeTabId: pane.activeTabId === tabId
        ? (pane.tabs.find((t) => t.id !== tabId)?.id ?? pane.activeTabId)
        : pane.activeTabId
    }))

    // Add to target
    updated = updatePane(updated, targetPaneId, (pane) => {
      const tabs = [...pane.tabs]
      tabs.splice(targetIndex, 0, tab)
      return {...pane, tabs, activeTabId: tab.id}
    })

    return updated
  }

  const firstPaneId = (node: LayoutNode): string => {
    if (node.kind === 'pane') return node.id
    return firstPaneId(node.first)
  }

  const nextTitle = (node: LayoutNode): string => {
    const count = collectTabs(node).length
    return `Terminal ${count + 1}`
  }

  const buildTab = (terminalId: string, title: string): TerminalTab => ({
    id: newId(),
    terminalId,
    title
  })

  const buildPane = (tab: TerminalTab): PaneNode => ({
    id: newId(),
    kind: 'pane',
    tabs: [tab],
    activeTabId: tab.id
  })

  const ensureFocusedPane = (next: TerminalLayout): TerminalLayout => {
    if (!next.focusedPaneId) {
      return {...next, focusedPaneId: firstPaneId(next.root)}
    }
    if (collectPaneIds(next.root).includes(next.focusedPaneId)) {
      return next
    }
    return {...next, focusedPaneId: firstPaneId(next.root)}
  }

  const updateLayout = (next: TerminalLayout): void => {
    const normalized = ensureFocusedPane(next)
    layout = normalized
    saveLayout(normalized)
  }

  const initWorkspace = async (): Promise<void> => {
    if (!workspaceId) return
    loading = true
    initError = ''
    try {
      const stored = loadLayout()
      if (stored) {
        updateLayout(stored)
        await restoreTerminals(stored)
        return
      }
      const created = await createWorkspaceTerminal(workspaceId)
      const tab = buildTab(created.terminalId, 'Terminal 1')
      const pane = buildPane(tab)
      updateLayout({version: STORAGE_VERSION, root: pane, focusedPaneId: pane.id})
    } catch (error) {
      initError = String(error)
    } finally {
      loading = false
    }
  }

  const restoreTerminals = async (state: TerminalLayout): Promise<void> => {
    const terminals = collectTabs(state.root).map((tab) => tab.terminalId)
    await Promise.allSettled(
      terminals.map((terminalId) => StartWorkspaceTerminal(workspaceId, terminalId))
    )
  }

  const handleFocusPane = (paneId: string): void => {
    if (!layout) return
    if (layout.focusedPaneId === paneId) return
    updateLayout({...layout, focusedPaneId: paneId})
  }

  const handleSelectTab = (paneId: string, tabId: string): void => {
    if (!layout) return
    const nextRoot = updatePane(layout.root, paneId, (pane) => ({
      ...pane,
      activeTabId: tabId
    }))
    updateLayout({...layout, root: nextRoot, focusedPaneId: paneId})
  }

  const handleAddTab = async (paneId: string): Promise<void> => {
    if (!layout) return
    try {
      const created = await createWorkspaceTerminal(workspaceId)
      const title = nextTitle(layout.root)
      const tab = buildTab(created.terminalId, title)
      const nextRoot = updatePane(layout.root, paneId, (pane) => ({
        ...pane,
        tabs: [...pane.tabs, tab],
        activeTabId: tab.id
      }))
      updateLayout({...layout, root: nextRoot, focusedPaneId: paneId})
    } catch (error) {
      initError = String(error)
    }
  }

  const handleSplitPane = async (paneId: string, direction: 'row' | 'column'): Promise<void> => {
    if (!layout) return
    try {
      const created = await createWorkspaceTerminal(workspaceId)
      const title = nextTitle(layout.root)
      const tab = buildTab(created.terminalId, title)
      const pane = buildPane(tab)
      const nextRoot = splitPane(layout.root, paneId, direction, pane)
      updateLayout({...layout, root: nextRoot, focusedPaneId: paneId})
    } catch (error) {
      initError = String(error)
    }
  }

  const handleCloseTab = (paneId: string, tabId: string): void => {
    if (!layout) return
    const pane = findPane(layout.root, paneId)
    if (!pane) return
    const remaining = pane.tabs.filter((tab) => tab.id !== tabId)
    if (remaining.length === 0) {
      handleClosePane(paneId)
      return
    }
    const nextActive = pane.activeTabId === tabId ? remaining[0].id : pane.activeTabId
    const nextRoot = updatePane(layout.root, paneId, (existing) => ({
      ...existing,
      tabs: remaining,
      activeTabId: nextActive
    }))
    updateLayout({...layout, root: nextRoot, focusedPaneId: paneId})
  }

  const handleClosePane = (paneId: string): void => {
    if (!layout) return
    const nextRoot = removePane(layout.root, paneId)
    if (!nextRoot) {
      void initWorkspace()
      return
    }
    updateLayout({...layout, root: nextRoot})
  }

  const MIN_RATIO = 0.15
  const MAX_RATIO = 0.85

  const handleResizeSplit = (splitId: string, ratio: number): void => {
    if (!layout) return
    const clampedRatio = Math.max(MIN_RATIO, Math.min(MAX_RATIO, ratio))
    const nextRoot = updateSplitRatio(layout.root, splitId, clampedRatio)
    updateLayout({...layout, root: nextRoot})
  }

  // Drag state for tab reordering
  type DragState = {
    tabId: string
    sourcePaneId: string
    sourceIndex: number
  } | null

  let dragState = $state<DragState>(null)

  const handleTabDragStart = (paneId: string, tabId: string, index: number): void => {
    dragState = {tabId, sourcePaneId: paneId, sourceIndex: index}
  }

  const handleTabDragEnd = (): void => {
    dragState = null
  }

  const handleTabDrop = (targetPaneId: string, targetIndex: number): void => {
    if (!layout || !dragState) return
    const {tabId, sourcePaneId} = dragState

    // Move the tab
    let nextRoot = moveTab(layout.root, sourcePaneId, targetPaneId, tabId, targetIndex)

    // Check if source pane is now empty and needs to be removed
    const sourcePane = findPane(nextRoot, sourcePaneId)
    if (sourcePane && sourcePane.tabs.length === 0) {
      const removed = removePane(nextRoot, sourcePaneId)
      if (removed) {
        nextRoot = removed
      }
    }

    updateLayout({...layout, root: nextRoot, focusedPaneId: targetPaneId})
    dragState = null
  }

  const handleTabSplitDrop = (
    targetPaneId: string,
    direction: 'row' | 'column',
    position: 'before' | 'after'
  ): void => {
    if (!layout || !dragState) return
    const {tabId, sourcePaneId} = dragState

    // Find the tab in source pane
    const sourcePane = findPane(layout.root, sourcePaneId)
    if (!sourcePane) return
    const tab = sourcePane.tabs.find((t) => t.id === tabId)
    if (!tab) return

    // Create a new pane with the tab
    const newPane: PaneNode = {
      id: newId(),
      kind: 'pane',
      tabs: [tab],
      activeTabId: tab.id
    }

    // Remove tab from source pane
    let nextRoot = updatePane(layout.root, sourcePaneId, (pane) => ({
      ...pane,
      tabs: pane.tabs.filter((t) => t.id !== tabId),
      activeTabId: pane.activeTabId === tabId
        ? (pane.tabs.find((t) => t.id !== tabId)?.id ?? pane.activeTabId)
        : pane.activeTabId
    }))

    // Check if source pane is now empty and needs to be removed
    const updatedSourcePane = findPane(nextRoot, sourcePaneId)
    if (updatedSourcePane && updatedSourcePane.tabs.length === 0) {
      const removed = removePane(nextRoot, sourcePaneId)
      if (removed) {
        nextRoot = removed
      }
    }

    // Split the target pane with the new pane
    // position 'before' means new pane goes first, 'after' means new pane goes second
    const splitWithNewPane = (node: LayoutNode): LayoutNode => {
      if (node.kind === 'pane') {
        if (node.id !== targetPaneId) return node
        return {
          id: newId(),
          kind: 'split',
          direction,
          ratio: 0.5,
          first: position === 'before' ? newPane : node,
          second: position === 'before' ? node : newPane
        }
      }
      const first = splitWithNewPane(node.first)
      const second = splitWithNewPane(node.second)
      if (first === node.first && second === node.second) return node
      return {...node, first, second}
    }

    nextRoot = splitWithNewPane(nextRoot)
    updateLayout({...layout, root: nextRoot, focusedPaneId: newPane.id})
    dragState = null
  }

  // Keyboard navigation helpers
  type PanePosition = {
    id: string
    x: number
    y: number
    w: number
    h: number
  }

  const buildPanePositions = (
    node: LayoutNode,
    x = 0,
    y = 0,
    w = 1,
    h = 1,
    positions: PanePosition[] = []
  ): PanePosition[] => {
    if (node.kind === 'pane') {
      positions.push({id: node.id, x, y, w, h})
      return positions
    }
    const {direction, ratio, first, second} = node
    if (direction === 'row') {
      buildPanePositions(first, x, y, w * ratio, h, positions)
      buildPanePositions(second, x + w * ratio, y, w * (1 - ratio), h, positions)
    } else {
      buildPanePositions(first, x, y, w, h * ratio, positions)
      buildPanePositions(second, x, y + h * ratio, w, h * (1 - ratio), positions)
    }
    return positions
  }

  const findAdjacentPane = (
    currentId: string,
    direction: 'up' | 'down' | 'left' | 'right',
    positions: PanePosition[]
  ): string | null => {
    const current = positions.find((p) => p.id === currentId)
    if (!current) return null

    const cx = current.x + current.w / 2
    const cy = current.y + current.h / 2

    let candidates = positions.filter((p) => p.id !== currentId)

    // Filter by direction
    if (direction === 'left') {
      candidates = candidates.filter((p) => p.x + p.w <= current.x + 0.01)
    } else if (direction === 'right') {
      candidates = candidates.filter((p) => p.x >= current.x + current.w - 0.01)
    } else if (direction === 'up') {
      candidates = candidates.filter((p) => p.y + p.h <= current.y + 0.01)
    } else {
      candidates = candidates.filter((p) => p.y >= current.y + current.h - 0.01)
    }

    if (candidates.length === 0) return null

    // Find closest by center distance
    const axis = direction === 'left' || direction === 'right' ? 'y' : 'x'
    const center = axis === 'x' ? cx : cy
    candidates.sort((a, b) => {
      const aCtr = axis === 'x' ? a.x + a.w / 2 : a.y + a.h / 2
      const bCtr = axis === 'x' ? b.x + b.w / 2 : b.y + b.h / 2
      return Math.abs(aCtr - center) - Math.abs(bCtr - center)
    })

    return candidates[0].id
  }

  const handleWorkspaceKeydown = (event: KeyboardEvent): void => {
    if (!layout) return

    // Alt+Arrow: Navigate between panes
    if (event.altKey && !event.ctrlKey && !event.metaKey) {
      let direction: 'up' | 'down' | 'left' | 'right' | null = null
      if (event.key === 'ArrowUp') direction = 'up'
      else if (event.key === 'ArrowDown') direction = 'down'
      else if (event.key === 'ArrowLeft') direction = 'left'
      else if (event.key === 'ArrowRight') direction = 'right'

      if (direction && layout.focusedPaneId) {
        event.preventDefault()
        const positions = buildPanePositions(layout.root)
        const nextPaneId = findAdjacentPane(layout.focusedPaneId, direction, positions)
        if (nextPaneId) {
          handleFocusPane(nextPaneId)
        }
      }
    }

    // Ctrl+Tab: Cycle tabs in current pane
    if (event.ctrlKey && event.key === 'Tab' && layout.focusedPaneId) {
      event.preventDefault()
      const pane = findPane(layout.root, layout.focusedPaneId)
      if (!pane || pane.tabs.length <= 1) return

      const currentIndex = pane.tabs.findIndex((t) => t.id === pane.activeTabId)
      const delta = event.shiftKey ? -1 : 1
      const nextIndex = (currentIndex + delta + pane.tabs.length) % pane.tabs.length
      handleSelectTab(layout.focusedPaneId, pane.tabs[nextIndex].id)
    }
  }

  $effect(() => {
    if (!workspaceId) return
    void initWorkspace()
  })
</script>

<section
  class="terminal-workspace"
  role="application"
  tabindex="-1"
  onkeydown={handleWorkspaceKeydown}
>
  <header class="workspace-header">
    <span class="title">Terminals</span>
    <div class="workspace-actions">
      <button
        type="button"
        class="action"
        title="New tab (in focused pane)"
        onclick={() => layout?.focusedPaneId && handleAddTab(layout.focusedPaneId)}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <path d="M7 2v10M2 7h10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
      </button>
      <button
        type="button"
        class="action"
        title="Split vertical"
        onclick={() => layout?.focusedPaneId && handleSplitPane(layout.focusedPaneId, 'row')}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <rect x="1" y="2" width="12" height="10" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
          <path d="M7 2v10" stroke="currentColor" stroke-width="1.2"/>
        </svg>
      </button>
      <button
        type="button"
        class="action"
        title="Split horizontal"
        onclick={() => layout?.focusedPaneId && handleSplitPane(layout.focusedPaneId, 'column')}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <rect x="1" y="2" width="12" height="10" rx="1.5" stroke="currentColor" stroke-width="1.2"/>
          <path d="M1 7h12" stroke="currentColor" stroke-width="1.2"/>
        </svg>
      </button>
    </div>
  </header>

  <div class="workspace-container">
    {#if initError}
      <div class="terminal-error">
        <div class="status-text">Terminal startup error.</div>
        <div class="status-message">{initError}</div>
        <button type="button" class="retry-action" onclick={() => initWorkspace()}>Retry</button>
      </div>
    {:else if loading || !layout}
      <div class="terminal-loading">Preparing terminals…</div>
    {:else}
      <TerminalLayoutNode
        node={layout.root}
        {workspaceId}
        {workspaceName}
        {active}
        focusedPaneId={layout.focusedPaneId}
        {dragState}
        onFocusPane={handleFocusPane}
        onSelectTab={handleSelectTab}
        onAddTab={handleAddTab}
        onSplitPane={handleSplitPane}
        onCloseTab={handleCloseTab}
        onClosePane={handleClosePane}
        onResizeSplit={handleResizeSplit}
        onTabDragStart={handleTabDragStart}
        onTabDragEnd={handleTabDragEnd}
        onTabDrop={handleTabDrop}
        onTabSplitDrop={handleTabSplitDrop}
      />
    {/if}
  </div>
</section>

<style>
  .terminal-workspace {
    display: flex;
    flex-direction: column;
    gap: 8px;
    height: 100%;
  }

  .workspace-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    padding: 0 4px;
  }

  .title {
    font-size: 12px;
    font-weight: 500;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .workspace-actions {
    display: flex;
    align-items: center;
    gap: 2px;
  }

  .action {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--muted);
    cursor: pointer;
    transition: background 0.15s ease, color 0.15s ease;
  }

  .action:hover {
    background: var(--border);
    color: var(--text);
  }

  .workspace-container {
    flex: 1;
    min-height: 0;
    display: flex;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--panel);
    overflow: hidden;
  }

  .terminal-loading,
  .terminal-error {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 16px;
    color: var(--muted);
    font-size: 12px;
  }

  .terminal-error {
    color: var(--text);
  }

  .terminal-error .status-message {
    color: var(--muted);
  }

  .retry-action {
    margin-top: 8px;
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 6px 12px;
    font-size: 12px;
    background: transparent;
    color: var(--text);
    cursor: pointer;
  }

  .retry-action:hover {
    border-color: var(--accent);
  }
</style>
