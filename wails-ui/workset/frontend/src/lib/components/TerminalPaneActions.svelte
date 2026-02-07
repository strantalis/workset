<script lang="ts">
	interface Props {
		paneId: string;
		onAddTab: (paneId: string) => void;
		onSplitPane: (paneId: string, direction: 'row' | 'column') => void;
	}

	const { paneId, onAddTab, onSplitPane }: Props = $props();
</script>

<div class="pane-actions">
	<button type="button" class="action-btn" title="New tab" onclick={() => onAddTab(paneId)}>
		<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
			<path d="M7 2v10M2 7h10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
		</svg>
	</button>
	<button
		type="button"
		class="action-btn"
		title="Split vertical"
		onclick={() => onSplitPane(paneId, 'row')}
	>
		<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
			<rect x="1" y="2" width="12" height="10" rx="1.5" stroke="currentColor" stroke-width="1.2" />
			<path d="M7 2v10" stroke="currentColor" stroke-width="1.2" />
		</svg>
	</button>
	<button
		type="button"
		class="action-btn"
		title="Split horizontal"
		onclick={() => onSplitPane(paneId, 'column')}
	>
		<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
			<rect x="1" y="2" width="12" height="10" rx="1.5" stroke="currentColor" stroke-width="1.2" />
			<path d="M1 7h12" stroke="currentColor" stroke-width="1.2" />
		</svg>
	</button>
</div>

<style>
	.pane-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		padding: 0 8px 0 4px;
		margin-left: auto;
	}

	.action-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: var(--panel);
		color: var(--muted);
		cursor: pointer;
		transition: all 0.15s ease;
		box-shadow: var(--shadow-sm);
	}

	.action-btn:hover {
		background: var(--panel-strong);
		color: var(--text);
		border-color: var(--accent);
	}

	.action-btn:active {
		transform: scale(0.95);
		box-shadow: none;
	}
</style>
