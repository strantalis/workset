<script lang="ts">
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
			<svg width="12" height="12" viewBox="0 0 12 12" fill="none">
				<path
					d="M3 3L9 9M9 3L3 9"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
				/>
			</svg>
		</button>
	{/if}
</div>

<style>
	.pane-tab {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px 6px 12px;
		font-size: 12px;
		font-weight: 500;
		background: transparent;
		color: var(--muted);
		cursor: grab;
		border-radius: 8px;
		transition: all 0.15s cubic-bezier(0.4, 0, 0.2, 1);
		white-space: nowrap;
		position: relative;
		border: 1px solid transparent;
	}

	.pane-tab:hover {
		color: var(--text);
		background: color-mix(in srgb, var(--panel-strong) 50%, transparent);
		border-color: var(--border);
	}

	.pane-tab:active {
		cursor: grabbing;
		transform: scale(0.98);
	}

	.pane-tab.active {
		color: var(--text);
		background: var(--panel);
		border-color: var(--border);
		box-shadow:
			var(--shadow-sm),
			inset 0 1px 0 rgba(255, 255, 255, 0.04);
		z-index: 1;
	}

	.pane-tab.active::after {
		content: '';
		position: absolute;
		bottom: -5px;
		left: 50%;
		transform: translateX(-50%);
		width: 6px;
		height: 2px;
		background: var(--accent);
		border-radius: 1px;
		box-shadow: 0 0 8px var(--accent);
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
		width: 20px;
		height: 20px;
		margin-left: 4px;
		color: var(--muted);
		border: none;
		background: transparent;
		border-radius: 4px;
		cursor: pointer;
		opacity: 0;
		transition:
			opacity 0.15s ease,
			background 0.15s ease,
			color 0.15s ease;
	}

	.pane-tab:hover .tab-close,
	.pane-tab.active .tab-close {
		opacity: 1;
	}

	.tab-close:hover {
		background: color-mix(in srgb, var(--warning) 20%, transparent);
		color: var(--warning);
	}
</style>
