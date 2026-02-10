<script lang="ts">
	import { Terminal, X } from '@lucide/svelte';

	interface Tab {
		id: string;
		title: string;
	}

	interface Props {
		tab: Tab;
		paneId: string;
		index: number;
		isActive: boolean;
		isDragging: boolean;
		isDropBefore: boolean;
		showClose: boolean;
		onSelectTab: (paneId: string, tabId: string) => void;
		onCloseTab: (paneId: string, tabId: string) => void;
		onTabDragStart: (event: DragEvent, index: number) => void;
		onTabDragEnd: () => void;
		onTabDragOver: (event: DragEvent, index: number) => void;
		onTabDrop: (event: DragEvent, index: number) => void;
	}

	const {
		tab,
		paneId,
		index,
		isActive,
		isDragging,
		isDropBefore,
		showClose,
		onSelectTab,
		onCloseTab,
		onTabDragStart,
		onTabDragEnd,
		onTabDragOver,
		onTabDrop,
	}: Props = $props();
</script>

<div
	class="pane-tab"
	class:active={isActive}
	class:dragging={isDragging}
	class:drop-before={isDropBefore}
	role="button"
	tabindex="0"
	draggable="true"
	ondragstart={(event) => onTabDragStart(event, index)}
	ondragend={onTabDragEnd}
	ondragover={(event) => {
		onTabDragOver(event, index);
		event.stopPropagation();
	}}
	ondrop={(event) => {
		onTabDrop(event, index);
		event.stopPropagation();
	}}
	onclick={() => onSelectTab(paneId, tab.id)}
	onauxclick={(event) => {
		if (event.button === 1) {
			event.preventDefault();
			onCloseTab(paneId, tab.id);
		}
	}}
	onkeydown={(event) => {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			onSelectTab(paneId, tab.id);
		}
	}}
>
	<span class="tab-prompt"><Terminal size={12} /></span>
	<span class="tab-label">{tab.title}</span>
	{#if showClose}
		<button
			type="button"
			class="tab-close"
			title="Close tab"
			onclick={(event) => {
				event.stopPropagation();
				onCloseTab(paneId, tab.id);
			}}
		>
			<X size={14} />
		</button>
	{/if}
</div>

<style>
	.pane-tab {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 8px 16px;
		font-size: var(--text-mono-sm);
		font-family: var(--font-mono);
		background: transparent;
		color: var(--muted);
		cursor: grab;
		border: none;
		border-top: 2px solid transparent;
		border-right: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
		border-radius: 0;
		transition:
			color 0.15s ease,
			background 0.15s ease,
			border-color 0.15s ease;
		white-space: nowrap;
		position: relative;
	}

	.pane-tab:hover {
		color: var(--text);
		background: color-mix(in srgb, var(--panel-strong) 40%, transparent);
	}

	.pane-tab:active {
		cursor: grabbing;
	}

	.pane-tab.active {
		color: var(--accent);
		background: var(--bg);
		border-top-color: var(--accent);
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

	.tab-prompt {
		color: var(--accent);
		font-weight: 500;
	}

	.tab-label {
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.tab-close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		margin-left: 2px;
		color: var(--muted);
		border: none;
		background: transparent;
		border-radius: 3px;
		cursor: pointer;
		font-size: var(--text-md);
		line-height: 1;
		opacity: 0;
		transition:
			opacity 0.12s ease,
			background 0.12s ease,
			color 0.12s ease;
	}

	.pane-tab:hover .tab-close,
	.pane-tab.active .tab-close {
		opacity: 0.7;
	}

	.tab-close:hover {
		opacity: 1;
		background: color-mix(in srgb, var(--warning) 20%, transparent);
		color: var(--warning);
	}
</style>
