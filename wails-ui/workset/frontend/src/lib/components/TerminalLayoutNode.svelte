<script lang="ts">
	import DropdownMenu from './ui/DropdownMenu.svelte';
	import TerminalPane from './TerminalPane.svelte';
	import Self from './TerminalLayoutNode.svelte';
	import { shouldHandlePaneKeydown } from './terminalLayoutKeydown';

	const {
		node,
		workspaceId,
		workspaceName,
		active = true,
		focusedPaneId,
		onFocusPane,
		onClosePane,
		onSplitPane,
		onResizeSplit,
	}: {
		node: {
			id?: string;
			kind?: string;
			terminalId?: string;
			direction?: 'row' | 'column';
			ratio?: number;
			first?: unknown;
			second?: unknown;
		} | null;
		workspaceId: string;
		workspaceName: string;
		active?: boolean;
		focusedPaneId?: string;
		onFocusPane: (paneId: string) => void;
		onClosePane: (paneId: string) => void;
		onSplitPane: (paneId: string, direction: 'row' | 'column') => void;
		onResizeSplit?: (splitId: string, ratio: number) => void;
	} = $props();

	const isPane = (value: typeof node): boolean => value?.kind === 'pane';
	const isSplit = (value: typeof node): boolean => value?.kind === 'split';

	let isDraggingDivider = $state(false);
	let dividerRef: HTMLDivElement | null = null;
	let splitContainerRef = $state<HTMLDivElement | null>(null);
	let contextMenuOpen = $state(false);
	let contextMenuPoint = $state<{ top: number; left: number } | null>(null);

	const openContextMenu = (event: MouseEvent, paneId: string): void => {
		event.preventDefault();
		onFocusPane(paneId);
		contextMenuPoint = { top: event.clientY, left: event.clientX };
		contextMenuOpen = true;
	};

	const closeContextMenu = (): void => {
		contextMenuOpen = false;
		contextMenuPoint = null;
	};

	const handleSplitFromMenu = (paneId: string, direction: 'row' | 'column'): void => {
		onSplitPane(paneId, direction);
		closeContextMenu();
	};

	const handleDividerPointerDown = (event: PointerEvent): void => {
		if (!onResizeSplit || !isSplit(node)) return;
		event.preventDefault();
		isDraggingDivider = true;
		dividerRef = event.currentTarget as HTMLDivElement;
		dividerRef.setPointerCapture(event.pointerId);
	};

	const handleDividerPointerMove = (event: PointerEvent): void => {
		if (!isDraggingDivider || !splitContainerRef || !onResizeSplit || !isSplit(node)) return;
		const splitNode = node as NonNullable<typeof node> & { kind: 'split' };
		const rect = splitContainerRef.getBoundingClientRect();
		const ratio =
			splitNode.direction === 'row'
				? (event.clientX - rect.left) / rect.width
				: (event.clientY - rect.top) / rect.height;
		onResizeSplit(splitNode.id ?? '', ratio);
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
		const splitNode = node as NonNullable<typeof node> & { kind: 'split' };

		const step = event.shiftKey ? 0.1 : 0.02;
		let delta = 0;

		if (splitNode.direction === 'row') {
			if (event.key === 'ArrowLeft') delta = -step;
			else if (event.key === 'ArrowRight') delta = step;
		} else {
			if (event.key === 'ArrowUp') delta = -step;
			else if (event.key === 'ArrowDown') delta = step;
		}

		if (delta !== 0) {
			event.preventDefault();
			onResizeSplit(splitNode.id ?? '', (splitNode.ratio ?? 0.5) + delta);
		}
	};
</script>

{#if !node}
	<div class="pane-empty">
		<span class="empty-label">No terminals</span>
	</div>
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
				{onFocusPane}
				{onClosePane}
				{onSplitPane}
				{onResizeSplit}
			/>
		</div>
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
				{onFocusPane}
				{onClosePane}
				{onSplitPane}
				{onResizeSplit}
			/>
		</div>
	</div>
{:else if !isPane(node)}
	<div class="pane-empty">
		<span class="empty-label">Terminal layout unavailable</span>
	</div>
{:else}
	{@const paneId = node?.id ?? ''}
	{@const isFocused = focusedPaneId === paneId}
	<div
		class="pane"
		class:focused={isFocused}
		data-pane-id={paneId}
		role="button"
		tabindex="0"
		oncontextmenu={(event) => paneId && openContextMenu(event, paneId)}
		onclick={() => paneId && onFocusPane(paneId)}
		onkeydown={(event) => {
			if (!shouldHandlePaneKeydown(event)) return;
			event.preventDefault();
			if (paneId) onFocusPane(paneId);
		}}
	>
		<div class="pane-body" role="region">
			<TerminalPane
				{workspaceId}
				{workspaceName}
				terminalId={node?.terminalId ?? ''}
				active={isFocused && active}
				compact={true}
				onTerminalClosed={() => paneId && onClosePane(paneId)}
			/>
		</div>
		<DropdownMenu open={contextMenuOpen} onClose={closeContextMenu} anchorPoint={contextMenuPoint}>
			<button type="button" onclick={() => paneId && onClosePane(paneId)}>
				<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
					<rect x="1.5" y="2" width="11" height="9.5" rx="1.5" />
					<path d="M4.5 5l5 4" />
					<path d="M9.5 5l-5 4" />
				</svg>
				Close split
			</button>
			<button type="button" onclick={() => handleSplitFromMenu(paneId, 'row')}>
				<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
					<rect x="1" y="2" width="12" height="10" rx="1.5" />
					<path d="M7 2v10" />
				</svg>
				Split vertical
			</button>
			<button type="button" onclick={() => handleSplitFromMenu(paneId, 'column')}>
				<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
					<rect x="1" y="2" width="12" height="10" rx="1.5" />
					<path d="M1 7h12" />
				</svg>
				Split horizontal
			</button>
		</DropdownMenu>
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
			opacity var(--transition-fast),
			background var(--transition-fast);
		touch-action: none;
		position: relative;
	}

	.split.row > .split-divider {
		width: 1px;
		cursor: col-resize;
	}

	.split.column > .split-divider {
		height: 1px;
		cursor: row-resize;
	}

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

	.pane {
		display: flex;
		flex: 1;
		min-width: 0;
		min-height: 0;
		flex-direction: column;
		background: color-mix(in srgb, var(--panel) 92%, black 8%);
		border: 1px solid transparent;
	}

	.pane.focused {
		border-color: color-mix(in srgb, var(--accent) 45%, transparent);
	}

	.pane-body {
		flex: 1;
		min-height: 0;
		min-width: 0;
	}

	.pane-empty {
		display: flex;
		align-items: center;
		justify-content: center;
		flex: 1;
		color: var(--muted);
	}

	:global(.dropdown-menu button svg rect),
	:global(.dropdown-menu button svg path) {
		stroke: currentColor;
		stroke-width: 1.2;
		stroke-linecap: round;
		stroke-linejoin: round;
	}
</style>
