<script lang="ts">
  import TerminalPane from './TerminalPane.svelte'
  import Self from './TerminalLayoutNode.svelte'

  type DragState = {
    tabId: string
    sourcePaneId: string
    sourceIndex: number
  } | null

  type DropZone = 'left' | 'right' | 'top' | 'bottom' | 'center' | null

  let {
    node,
    workspaceId,
    workspaceName,
    active = true,
    focusedPaneId,
    dragState = null,
    onFocusPane,
    onSelectTab,
    onAddTab,
    onSplitPane,
    onCloseTab,
    onClosePane,
    onResizeSplit,
    onTabDragStart,
    onTabDragEnd,
    onTabDrop,
    onTabSplitDrop
  }: {
    node: any
    workspaceId: string
    workspaceName: string
    active?: boolean
    focusedPaneId?: string
    dragState?: DragState
    onFocusPane: (paneId: string) => void
    onSelectTab: (paneId: string, tabId: string) => void
    onAddTab: (paneId: string) => void
    onSplitPane: (paneId: string, direction: 'row' | 'column') => void
    onCloseTab: (paneId: string, tabId: string) => void
    onClosePane: (paneId: string) => void
    onResizeSplit?: (splitId: string, ratio: number) => void
    onTabDragStart?: (paneId: string, tabId: string, index: number) => void
    onTabDragEnd?: () => void
    onTabDrop?: (targetPaneId: string, targetIndex: number) => void
    onTabSplitDrop?: (targetPaneId: string, direction: 'row' | 'column', position: 'before' | 'after') => void
  } = $props()

  const isPane = (value: any): boolean => value?.kind === 'pane'
  const isSplit = (value: any): boolean => value?.kind === 'split'

  // Divider drag state
  let isDraggingDivider = $state(false)
  let dividerRef: HTMLDivElement | null = null
  let splitContainerRef = $state<HTMLDivElement | null>(null)

  const handleDividerPointerDown = (event: PointerEvent): void => {
    if (!onResizeSplit || !isSplit(node)) return
    event.preventDefault()
    isDraggingDivider = true
    dividerRef = event.currentTarget as HTMLDivElement
    dividerRef.setPointerCapture(event.pointerId)
  }

  const handleDividerPointerMove = (event: PointerEvent): void => {
    if (!isDraggingDivider || !splitContainerRef || !onResizeSplit || !isSplit(node)) return

    const rect = splitContainerRef.getBoundingClientRect()
    let ratio: number

    if (node.direction === 'row') {
      ratio = (event.clientX - rect.left) / rect.width
    } else {
      ratio = (event.clientY - rect.top) / rect.height
    }

    onResizeSplit(node.id, ratio)
  }

  const handleDividerPointerUp = (event: PointerEvent): void => {
    if (!isDraggingDivider) return
    isDraggingDivider = false
    if (dividerRef) {
      dividerRef.releasePointerCapture(event.pointerId)
      dividerRef = null
    }
  }

  const handleDividerKeyDown = (event: KeyboardEvent): void => {
    if (!onResizeSplit || !isSplit(node)) return

    const step = event.shiftKey ? 0.1 : 0.02
    let delta = 0

    if (node.direction === 'row') {
      if (event.key === 'ArrowLeft') delta = -step
      else if (event.key === 'ArrowRight') delta = step
    } else {
      if (event.key === 'ArrowUp') delta = -step
      else if (event.key === 'ArrowDown') delta = step
    }

    if (delta !== 0) {
      event.preventDefault()
      onResizeSplit(node.id, node.ratio + delta)
    }
  }

  // Tab drag handlers
  let dropTargetIndex = $state<number | null>(null)
  let activeDropZone = $state<DropZone>(null)
  let paneBodyRef = $state<HTMLDivElement | null>(null)

  const EDGE_THRESHOLD = 0.25 // 25% from edge triggers split zone

  const getDropZone = (event: DragEvent, element: HTMLElement): DropZone => {
    const rect = element.getBoundingClientRect()
    const x = (event.clientX - rect.left) / rect.width
    const y = (event.clientY - rect.top) / rect.height

    // Check edges (25% threshold)
    if (x < EDGE_THRESHOLD) return 'left'
    if (x > 1 - EDGE_THRESHOLD) return 'right'
    if (y < EDGE_THRESHOLD) return 'top'
    if (y > 1 - EDGE_THRESHOLD) return 'bottom'
    return 'center'
  }

  const handleTabDragStart = (event: DragEvent, tabId: string, index: number): void => {
    if (!onTabDragStart || !isPane(node)) return
    event.dataTransfer?.setData('text/plain', tabId)
    event.dataTransfer!.effectAllowed = 'move'
    onTabDragStart(node.id, tabId, index)
  }

  const handleTabDragEnd = (): void => {
    dropTargetIndex = null
    activeDropZone = null
    onTabDragEnd?.()
  }

  const handleTabDragOver = (event: DragEvent, index: number): void => {
    if (!dragState || !isPane(node)) return
    event.preventDefault()
    event.dataTransfer!.dropEffect = 'move'
    dropTargetIndex = index
  }

  const handleHeaderDragOver = (event: DragEvent): void => {
    if (!dragState || !isPane(node)) return
    event.preventDefault()
    event.dataTransfer!.dropEffect = 'move'
    dropTargetIndex = node.tabs.length
    activeDropZone = 'center'
  }

  const handleHeaderDrop = (event: DragEvent): void => {
    event.preventDefault()
    if (!dragState || !onTabDrop || !isPane(node)) return
    onTabDrop(node.id, node.tabs.length)
    dropTargetIndex = null
    activeDropZone = null
  }

  const handleBodyDragOver = (event: DragEvent): void => {
    if (!dragState || !isPane(node) || !paneBodyRef) return
    event.preventDefault()
    event.dataTransfer!.dropEffect = 'move'
    activeDropZone = getDropZone(event, paneBodyRef)
  }

  const handleBodyDrop = (event: DragEvent): void => {
    event.preventDefault()
    if (!dragState || !isPane(node)) return

    if (activeDropZone === 'center' || !activeDropZone) {
      // Drop as tab
      onTabDrop?.(node.id, node.tabs.length)
    } else if (onTabSplitDrop) {
      // Split drop
      const direction: 'row' | 'column' =
        activeDropZone === 'left' || activeDropZone === 'right' ? 'row' : 'column'
      const position: 'before' | 'after' =
        activeDropZone === 'left' || activeDropZone === 'top' ? 'before' : 'after'
      onTabSplitDrop(node.id, direction, position)
    }

    dropTargetIndex = null
    activeDropZone = null
  }

  const handleTabDrop = (event: DragEvent, index: number): void => {
    event.preventDefault()
    if (!dragState || !onTabDrop || !isPane(node)) return
    onTabDrop(node.id, index)
    dropTargetIndex = null
    activeDropZone = null
  }

  const handleDragLeave = (event: DragEvent): void => {
    // Only clear if leaving the pane entirely
    const relatedTarget = event.relatedTarget as HTMLElement | null
    const pane = (event.currentTarget as HTMLElement)
    if (!relatedTarget || !pane.contains(relatedTarget)) {
      dropTargetIndex = null
      activeDropZone = null
    }
  }
</script>

{#if !node}
  <div class="pane-empty">No terminals</div>
{:else if !isPane(node)}
  <div
    class="split {node.direction}"
    class:dragging-divider={isDraggingDivider}
    bind:this={splitContainerRef}
  >
    <div class="split-child" style={`flex:${node.ratio} 1 0%`}>
      <Self
        node={node.first}
        {workspaceId}
        {workspaceName}
        {active}
        {focusedPaneId}
        {dragState}
        {onFocusPane}
        {onSelectTab}
        {onAddTab}
        {onSplitPane}
        {onCloseTab}
        {onClosePane}
        {onResizeSplit}
        {onTabDragStart}
        {onTabDragEnd}
        {onTabDrop}
        {onTabSplitDrop}
      />
    </div>
    <div
      class="split-divider"
      class:active={isDraggingDivider}
      role="separator"
      tabindex="0"
      aria-orientation={node.direction === 'row' ? 'vertical' : 'horizontal'}
      aria-valuenow={Math.round(node.ratio * 100)}
      aria-valuemin={15}
      aria-valuemax={85}
      onpointerdown={handleDividerPointerDown}
      onpointermove={handleDividerPointerMove}
      onpointerup={handleDividerPointerUp}
      onpointercancel={handleDividerPointerUp}
      onkeydown={handleDividerKeyDown}
    ></div>
    <div class="split-child" style={`flex:${1 - node.ratio} 1 0%`}>
      <Self
        node={node.second}
        {workspaceId}
        {workspaceName}
        {active}
        {focusedPaneId}
        {dragState}
        {onFocusPane}
        {onSelectTab}
        {onAddTab}
        {onSplitPane}
        {onCloseTab}
        {onClosePane}
        {onResizeSplit}
        {onTabDragStart}
        {onTabDragEnd}
        {onTabDrop}
        {onTabSplitDrop}
      />
    </div>
  </div>
{:else}
  {#if node.tabs.length === 0}
    <div class="pane-empty">No terminals</div>
  {:else}
    {@const activeTab = node.tabs.find((tab: any) => tab.id === node.activeTabId) ?? node.tabs[0]}
    {@const isFocused = focusedPaneId === node.id}
    {@const isDragTarget = dragState && dragState.sourcePaneId !== node.id}
    <div
      class="pane"
      class:focused={isFocused}
      class:drag-active={isDragTarget && activeDropZone}
      data-pane-id={node.id}
      role="button"
      tabindex="0"
      onclick={() => onFocusPane(node.id)}
      onkeydown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          onFocusPane(node.id)
        }
      }}
      ondragleave={handleDragLeave}
    >
      <div
        class="pane-header"
        class:drop-target={isDragTarget}
        role="tablist"
        tabindex="-1"
        ondragover={handleHeaderDragOver}
        ondrop={handleHeaderDrop}
      >
        <div class="pane-tabs">
          {#each node.tabs as tab, index}
            <div
              class="pane-tab"
              class:active={tab.id === activeTab.id}
              class:dragging={dragState?.tabId === tab.id}
              class:drop-before={dropTargetIndex === index}
              role="button"
              tabindex="0"
              draggable="true"
              ondragstart={(e) => handleTabDragStart(e, tab.id, index)}
              ondragend={handleTabDragEnd}
              ondragover={(e) => {
                handleTabDragOver(e, index)
                e.stopPropagation()
              }}
              ondrop={(e) => {
                handleTabDrop(e, index)
                e.stopPropagation()
              }}
              onclick={() => onSelectTab(node.id, tab.id)}
              onkeydown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault()
                  onSelectTab(node.id, tab.id)
                }
              }}
            >
              <span class="tab-label">{tab.title}</span>
              {#if node.tabs.length > 1}
                <button
                  type="button"
                  class="tab-close"
                  title="Close tab"
                  onclick={(event) => {
                    event.stopPropagation()
                    onCloseTab(node.id, tab.id)
                  }}
                >
                  <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                    <path d="M3 3L9 9M9 3L3 9" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
                  </svg>
                </button>
              {/if}
            </div>
          {/each}
        </div>
      </div>
      <div
        class="pane-body"
        role="region"
        bind:this={paneBodyRef}
        ondragover={handleBodyDragOver}
        ondrop={handleBodyDrop}
      >
        <TerminalPane
          {workspaceId}
          {workspaceName}
          terminalId={activeTab.terminalId}
          active={isFocused && active}
          compact={true}
        />

        {#if dragState && dragState.sourcePaneId !== node.id}
          <div class="drop-zones">
            <div class="drop-zone left" class:active={activeDropZone === 'left'}></div>
            <div class="drop-zone right" class:active={activeDropZone === 'right'}></div>
            <div class="drop-zone top" class:active={activeDropZone === 'top'}></div>
            <div class="drop-zone bottom" class:active={activeDropZone === 'bottom'}></div>
            <div class="drop-zone center" class:active={activeDropZone === 'center'}></div>
          </div>
        {/if}
      </div>
    </div>
  {/if}
{/if}

<style>
  .split {
    display: flex;
    flex: 1;
    min-height: 0;
    min-width: 0;
  }

  .split.row {
    flex-direction: row;
  }

  .split.column {
    flex-direction: column;
  }

  .split.dragging-divider {
    user-select: none;
  }

  .split-child {
    min-height: 0;
    min-width: 0;
    display: flex;
  }

  .split-divider {
    flex-shrink: 0;
    background: var(--border);
    opacity: 0.5;
    transition: opacity 0.15s ease, background 0.15s ease;
    touch-action: none;
  }

  .split.row > .split-divider {
    width: 1px;
    cursor: col-resize;
    margin: 0 4px;
  }

  .split.column > .split-divider {
    height: 1px;
    cursor: row-resize;
    margin: 4px 0;
  }

  .split-divider:hover,
  .split-divider:focus,
  .split-divider.active {
    opacity: 1;
    background: var(--accent);
  }

  .split-divider:focus {
    outline: none;
  }

  .pane {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
    min-width: 0;
    background: var(--panel);
    border-radius: 8px;
    overflow: hidden;
    border: 1px solid transparent;
    transition: opacity 0.15s ease, border-color 0.15s ease;
  }

  .pane.focused {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 30%, transparent);
  }

  .pane:not(.focused) {
    opacity: 0.6;
    border-color: var(--border);
  }

  .pane:not(.focused):hover,
  .pane.drag-active {
    opacity: 0.8;
  }

  .pane-header {
    display: flex;
    align-items: center;
    padding: 0 8px;
    border-bottom: 1px solid var(--border);
    background: color-mix(in srgb, var(--bg) 50%, var(--panel));
    transition: background 0.15s ease;
  }

  .pane-header.drop-target {
    background: color-mix(in srgb, var(--accent) 10%, var(--panel));
  }

  .pane-tabs {
    display: flex;
    align-items: stretch;
    gap: 0;
    flex: 1;
    min-width: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .pane-tabs::-webkit-scrollbar {
    display: none;
  }

  .pane-tab {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 12px;
    font-size: 12px;
    background: transparent;
    color: var(--muted);
    cursor: grab;
    border-bottom: 2px solid transparent;
    margin-bottom: -1px;
    transition: color 0.15s ease, border-color 0.15s ease;
    white-space: nowrap;
    position: relative;
  }

  .pane-tab:hover {
    color: var(--text);
  }

  .pane-tab:active {
    cursor: grabbing;
  }

  .pane-tab.active {
    color: var(--text);
    border-bottom-color: var(--accent);
  }

  .pane-tab.dragging {
    opacity: 0.4;
  }

  .pane-tab.drop-before::before {
    content: '';
    position: absolute;
    left: 0;
    top: 6px;
    bottom: 6px;
    width: 2px;
    background: var(--accent);
    border-radius: 1px;
  }

  .tab-label {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .tab-close {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 18px;
    height: 18px;
    color: var(--muted);
    border: none;
    background: transparent;
    border-radius: 4px;
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.15s ease, background 0.15s ease, color 0.15s ease;
  }

  .pane-tab:hover .tab-close,
  .pane-tab.active .tab-close {
    opacity: 1;
  }

  .tab-close:hover {
    background: color-mix(in srgb, var(--warning) 20%, transparent);
    color: var(--warning);
  }

  .pane-body {
    flex: 1;
    min-height: 0;
    padding: 4px;
    position: relative;
  }

  .pane-empty {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--muted);
    font-size: 12px;
  }

  /* Drop zone overlays */
  .drop-zones {
    position: absolute;
    inset: 0;
    pointer-events: none;
    z-index: 10;
  }

  .drop-zone {
    position: absolute;
    background: color-mix(in srgb, var(--accent) 20%, transparent);
    border: 2px solid transparent;
    border-radius: 6px;
    opacity: 0;
    transition: opacity 0.1s ease, border-color 0.1s ease;
  }

  .drop-zone.active {
    opacity: 1;
    border-color: var(--accent);
    background: color-mix(in srgb, var(--accent) 30%, transparent);
  }

  .drop-zone.left {
    left: 4px;
    top: 4px;
    bottom: 4px;
    width: calc(25% - 4px);
  }

  .drop-zone.right {
    right: 4px;
    top: 4px;
    bottom: 4px;
    width: calc(25% - 4px);
  }

  .drop-zone.top {
    left: 4px;
    right: 4px;
    top: 4px;
    height: calc(25% - 4px);
  }

  .drop-zone.bottom {
    left: 4px;
    right: 4px;
    bottom: 4px;
    height: calc(25% - 4px);
  }

  .drop-zone.center {
    left: 30%;
    right: 30%;
    top: 30%;
    bottom: 30%;
  }
</style>
