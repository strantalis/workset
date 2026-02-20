<script lang="ts">
	import TerminalDropZones from './TerminalDropZones.svelte';
	import TerminalPane from './TerminalPane.svelte';
	import TerminalPaneActions from './TerminalPaneActions.svelte';
	import TerminalPaneTab from './TerminalPaneTab.svelte';
	import Self from './TerminalLayoutNode.svelte';
	import { shouldHandlePaneKeydown } from './terminalLayoutKeydown';

	type DragState = {
		tabId: string;
		sourcePaneId: string;
		sourceIndex: number;
	} | null;

	type DropZone = 'left' | 'right' | 'top' | 'bottom' | 'center' | null;

	// Props must use 'let' for Svelte 5 reactivity ($props() pattern)
	let {
		// eslint-disable-next-line prefer-const
		node,
		// eslint-disable-next-line prefer-const
		workspaceId,
		// eslint-disable-next-line prefer-const
		workspaceName,
		// eslint-disable-next-line prefer-const
		active = true,
		// eslint-disable-next-line prefer-const
		focusedPaneId,
		// eslint-disable-next-line prefer-const
		totalPaneCount,
		// eslint-disable-next-line prefer-const
		dragState = null,
		// eslint-disable-next-line prefer-const
		onFocusPane,
		// eslint-disable-next-line prefer-const
		onSelectTab,
		// eslint-disable-next-line prefer-const
		onAddTab,
		// eslint-disable-next-line prefer-const
		onSplitPane,
		// eslint-disable-next-line prefer-const
		onCloseTab,
		// eslint-disable-next-line prefer-const
		onClosePane,
		// eslint-disable-next-line prefer-const
		onResizeSplit,
		// eslint-disable-next-line prefer-const
		onTabDragStart,
		// eslint-disable-next-line prefer-const
		onTabDragEnd,
		// eslint-disable-next-line prefer-const
		onTabDrop,
		// eslint-disable-next-line prefer-const
		onTabSplitDrop,
	}: {
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		node: any;
		workspaceId: string;
		workspaceName: string;
		active?: boolean;
		focusedPaneId?: string;
		totalPaneCount: number;
		dragState?: DragState;
		onFocusPane: (paneId: string) => void;
		onSelectTab: (paneId: string, tabId: string) => void;
		onAddTab: (paneId: string) => void;
		onSplitPane: (paneId: string, direction: 'row' | 'column') => void;
		onCloseTab: (paneId: string, tabId: string) => void;
		onClosePane: (paneId: string) => void;
		onResizeSplit?: (splitId: string, ratio: number) => void;
		onTabDragStart?: (paneId: string, tabId: string, index: number) => void;
		onTabDragEnd?: () => void;
		onTabDrop?: (targetPaneId: string, targetIndex: number) => void;
		onTabSplitDrop?: (
			targetPaneId: string,
			direction: 'row' | 'column',
			position: 'before' | 'after',
		) => void;
	} = $props();

	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	const isPane = (value: any): boolean => value?.kind === 'pane';
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	const isSplit = (value: any): boolean => value?.kind === 'split';

	// Divider drag state
	let isDraggingDivider = $state(false);
	let dividerRef: HTMLDivElement | null = null;
	let splitContainerRef = $state<HTMLDivElement | null>(null);

	const handleDividerPointerDown = (event: PointerEvent): void => {
		if (!onResizeSplit || !isSplit(node)) return;
		event.preventDefault();
		isDraggingDivider = true;
		dividerRef = event.currentTarget as HTMLDivElement;
		dividerRef.setPointerCapture(event.pointerId);
	};

	const handleDividerPointerMove = (event: PointerEvent): void => {
		if (!isDraggingDivider || !splitContainerRef || !onResizeSplit || !isSplit(node)) return;

		const rect = splitContainerRef.getBoundingClientRect();
		let ratio: number;

		if (node.direction === 'row') {
			ratio = (event.clientX - rect.left) / rect.width;
		} else {
			ratio = (event.clientY - rect.top) / rect.height;
		}

		onResizeSplit(node.id, ratio);
	};

	const handleDividerPointerUp = (event: PointerEvent): void => {
		if (!isDraggingDivider) return;
		isDraggingDivider = false;
		if (dividerRef) {
			dividerRef.releasePointerCapture(event.pointerId);
			dividerRef = null;
		}
	};

	const handleDividerKeyDown = (event: KeyboardEvent): void => {
		if (!onResizeSplit || !isSplit(node)) return;

		const step = event.shiftKey ? 0.1 : 0.02;
		let delta = 0;

		if (node.direction === 'row') {
			if (event.key === 'ArrowLeft') delta = -step;
			else if (event.key === 'ArrowRight') delta = step;
		} else {
			if (event.key === 'ArrowUp') delta = -step;
			else if (event.key === 'ArrowDown') delta = step;
		}

		if (delta !== 0) {
			event.preventDefault();
			onResizeSplit(node.id, node.ratio + delta);
		}
	};

	// Tab drag handlers
	let dropTargetIndex = $state<number | null>(null);
	let activeDropZone = $state<DropZone>(null);
	let paneBodyRef = $state<HTMLDivElement | null>(null);

	const EDGE_THRESHOLD = 0.25; // 25% from edge triggers split zone

	const getDropZone = (event: DragEvent, element: HTMLElement): DropZone => {
		const rect = element.getBoundingClientRect();
		const x = (event.clientX - rect.left) / rect.width;
		const y = (event.clientY - rect.top) / rect.height;

		// Check edges (25% threshold)
		if (x < EDGE_THRESHOLD) return 'left';
		if (x > 1 - EDGE_THRESHOLD) return 'right';
		if (y < EDGE_THRESHOLD) return 'top';
		if (y > 1 - EDGE_THRESHOLD) return 'bottom';
		return 'center';
	};

	const handleTabDragStart = (event: DragEvent, tabId: string, index: number): void => {
		if (!onTabDragStart || !isPane(node)) return;
		const paneId = node?.id ?? '';
		if (!paneId) return;
		event.dataTransfer?.setData('text/plain', tabId);
		event.dataTransfer!.effectAllowed = 'move';
		onTabDragStart(paneId, tabId, index);
	};

	const handleTabDragEnd = (): void => {
		dropTargetIndex = null;
		activeDropZone = null;
		onTabDragEnd?.();
	};

	const handleTabDragOver = (event: DragEvent, index: number): void => {
		if (!dragState || !isPane(node)) return;
		event.preventDefault();
		event.dataTransfer!.dropEffect = 'move';
		dropTargetIndex = index;
	};

	const handleHeaderDragOver = (event: DragEvent): void => {
		if (!dragState || !isPane(node)) return;
		event.preventDefault();
		event.dataTransfer!.dropEffect = 'move';
		dropTargetIndex = node?.tabs?.length ?? 0;
		activeDropZone = 'center';
	};

	const handleHeaderDrop = (event: DragEvent): void => {
		event.preventDefault();
		if (!dragState || !onTabDrop || !isPane(node)) return;
		const paneId = node?.id ?? '';
		if (!paneId) return;
		onTabDrop(paneId, node?.tabs?.length ?? 0);
		dropTargetIndex = null;
		activeDropZone = null;
	};

	const handleBodyDragOver = (event: DragEvent): void => {
		if (!dragState || !isPane(node) || !paneBodyRef) return;
		event.preventDefault();
		event.dataTransfer!.dropEffect = 'move';
		activeDropZone = getDropZone(event, paneBodyRef);
	};

	const handleBodyDrop = (event: DragEvent): void => {
		event.preventDefault();
		if (!dragState || !isPane(node)) return;
		const paneId = node?.id ?? '';
		if (!paneId) return;

		if (activeDropZone === 'center' || !activeDropZone) {
			// Drop as tab
			onTabDrop?.(paneId, node?.tabs?.length ?? 0);
		} else if (onTabSplitDrop) {
			// Split drop
			const direction: 'row' | 'column' =
				activeDropZone === 'left' || activeDropZone === 'right' ? 'row' : 'column';
			const position: 'before' | 'after' =
				activeDropZone === 'left' || activeDropZone === 'top' ? 'before' : 'after';
			onTabSplitDrop(paneId, direction, position);
		}

		dropTargetIndex = null;
		activeDropZone = null;
	};

	const handleTabDrop = (event: DragEvent, index: number): void => {
		event.preventDefault();
		if (!dragState || !onTabDrop || !isPane(node)) return;
		const paneId = node?.id ?? '';
		if (!paneId) return;
		onTabDrop(paneId, index);
		dropTargetIndex = null;
		activeDropZone = null;
	};

	const handleDragLeave = (event: DragEvent): void => {
		// Only clear if leaving the pane entirely
		const relatedTarget = event.relatedTarget as HTMLElement | null;
		const pane = event.currentTarget as HTMLElement;
		if (!relatedTarget || !pane.contains(relatedTarget)) {
			dropTargetIndex = null;
			activeDropZone = null;
		}
	};
</script>

{#if !node}
	<div class="pane-empty">No terminals</div>
{:else if isSplit(node)}
	<div
		class="split {node?.direction ?? 'row'}"
		class:dragging-divider={isDraggingDivider}
		bind:this={splitContainerRef}
	>
		<div class="split-child" style={`flex:${node?.ratio ?? 0.5} 1 0%`}>
			<Self
				node={node?.first ?? null}
				{workspaceId}
				{workspaceName}
				{active}
				{focusedPaneId}
				{totalPaneCount}
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
		<!-- role="separator" with tabindex makes this an interactive widget per WAI-ARIA -->
		<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
		<div
			class="split-divider"
			class:active={isDraggingDivider}
			role="separator"
			tabindex="0"
			aria-orientation={(node?.direction ?? 'row') === 'row' ? 'vertical' : 'horizontal'}
			aria-valuenow={Math.round((node?.ratio ?? 0.5) * 100)}
			aria-valuemin={15}
			aria-valuemax={85}
			onpointerdown={handleDividerPointerDown}
			onpointermove={handleDividerPointerMove}
			onpointerup={handleDividerPointerUp}
			onpointercancel={handleDividerPointerUp}
			onkeydown={handleDividerKeyDown}
		></div>
		<div class="split-child" style={`flex:${1 - (node?.ratio ?? 0.5)} 1 0%`}>
			<Self
				node={node?.second ?? null}
				{workspaceId}
				{workspaceName}
				{active}
				{focusedPaneId}
				{totalPaneCount}
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
{:else if (node?.tabs?.length ?? 0) === 0}
	<div class="pane-empty">No terminals</div>
{:else if !isPane(node)}
	<div class="pane-empty">Terminal layout unavailable</div>
{:else}
	{@const paneTabs = node?.tabs ?? []}
	{@const paneId = node?.id ?? ''}
	{@const activeTab =
		paneTabs.find((tab: { id: string }) => tab.id === node?.activeTabId) ?? paneTabs[0]}
	{@const activeTabId = activeTab?.id ?? ''}
	{@const activeTerminalId = activeTab?.terminalId ?? ''}
	{@const isFocused = focusedPaneId === paneId}
	{@const isDragTarget = dragState && dragState.sourcePaneId !== paneId}
	<div
		class="pane"
		class:focused={isFocused}
		class:drag-active={isDragTarget && activeDropZone}
		data-pane-id={paneId}
		role="button"
		tabindex="0"
		onclick={() => paneId && onFocusPane(paneId)}
		onkeydown={(event) => {
			if (!shouldHandlePaneKeydown(event)) {
				return;
			}
			event.preventDefault();
			if (paneId) {
				onFocusPane(paneId);
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
				{#each paneTabs as tab, index (tab.id)}
					<TerminalPaneTab
						{tab}
						{paneId}
						{index}
						isActive={tab.id === activeTabId}
						isDragging={dragState?.tabId === tab.id}
						isDropBefore={dropTargetIndex === index}
						showClose={totalPaneCount > 1 || paneTabs.length > 1}
						{onSelectTab}
						{onCloseTab}
						onTabDragStart={(event, idx) => handleTabDragStart(event, tab.id, idx)}
						onTabDragEnd={handleTabDragEnd}
						onTabDragOver={handleTabDragOver}
						onTabDrop={handleTabDrop}
					/>
				{/each}
			</div>
			<TerminalPaneActions {paneId} {onAddTab} {onSplitPane} />
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
				terminalId={activeTerminalId}
				active={isFocused && active}
				compact={true}
			/>

			<TerminalDropZones
				show={Boolean(dragState && dragState.sourcePaneId !== paneId)}
				{activeDropZone}
			/>
		</div>
	</div>
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
		transition:
			opacity 0.15s ease,
			background 0.15s ease;
		touch-action: none;
		position: relative;
	}

	.split.row > .split-divider {
		width: 1px;
		cursor: col-resize;
		margin: 0;
	}

	.split.column > .split-divider {
		height: 1px;
		cursor: row-resize;
		margin: 0;
	}

	/* Expanded hit area (12px) for easier grabbing */
	.split-divider::before {
		content: '';
		position: absolute;
	}

	.split.row > .split-divider::before {
		top: 0;
		bottom: 0;
		width: 12px;
		left: 50%;
		transform: translateX(-50%);
	}

	.split.column > .split-divider::before {
		left: 0;
		right: 0;
		height: 12px;
		top: 50%;
		transform: translateY(-50%);
	}

	.split-divider:hover,
	.split-divider:focus,
	.split-divider.active {
		opacity: 1;
		background: var(--accent);
	}

	/* Make divider thicker on hover for better visibility */
	.split.row > .split-divider:hover,
	.split.row > .split-divider:focus,
	.split.row > .split-divider.active {
		width: 3px;
		margin: 0 -1px;
	}

	.split.column > .split-divider:hover,
	.split.column > .split-divider:focus,
	.split.column > .split-divider.active {
		height: 3px;
		margin: -1px 0;
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
		border-radius: 0;
		overflow: hidden;
		border: none;
		transition: all 0.2s ease;
		box-shadow: none;
	}

	.pane.focused {
		box-shadow:
			0 0 0 1px color-mix(in srgb, var(--accent) 30%, transparent),
			var(--shadow-lg);
		border-color: color-mix(in srgb, var(--accent) 40%, var(--border));
	}

	.pane.drag-active {
		box-shadow: var(--shadow-md);
	}

	.pane-header {
		display: flex;
		align-items: center;
		padding: 0 4px;
		background: color-mix(in srgb, var(--panel-strong) 80%, var(--panel));
		transition: background 0.2s ease;
		border-bottom: 1px solid var(--border);
		backdrop-filter: blur(12px);
		-webkit-backdrop-filter: blur(12px);
	}

	.pane-header.drop-target {
		background: color-mix(in srgb, var(--accent) 8%, var(--panel-strong));
	}

	.pane-tabs {
		display: flex;
		align-items: center;
		gap: 0;
		flex: 1;
		min-width: 0;
		overflow-x: auto;
		scrollbar-width: none;
		padding: 0;
	}

	.pane-tabs::-webkit-scrollbar {
		display: none;
	}

	.pane-body {
		flex: 1;
		min-height: 0;
		padding: 0;
		position: relative;
	}

	/* Dim the terminal area of inactive panes. Applied to pane-body only so
	   the tab header stays at full brightness and remains readable. */
	.pane:not(.focused) .pane-body {
		opacity: 0.45;
		transition: opacity 0.2s ease;
	}

	.pane-empty {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--muted);
		font-size: var(--text-sm);
	}
</style>
