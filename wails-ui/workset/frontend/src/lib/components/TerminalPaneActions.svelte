<script lang="ts">
	interface Props {
		paneId: string;
		onAddTab: (paneId: string) => void;
		onSplitPane: (paneId: string, direction: 'row' | 'column') => void;
	}

	const { paneId, onAddTab, onSplitPane }: Props = $props();
</script>

<div class="pane-actions">
	<button
		type="button"
		class="ws-icon-action-btn"
		data-hover-label="New tab (⌘T)"
		aria-label="New tab"
		onclick={() => onAddTab(paneId)}
	>
		<svg width="12" height="12" viewBox="0 0 12 12" fill="none">
			<path d="M6 1v10M1 6h10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
		</svg>
	</button>
	<div class="split-actions">
		<button
			type="button"
			class="ws-icon-action-btn"
			data-hover-label="Split vertical (⌘\)"
			aria-label="Split vertical"
			onclick={() => onSplitPane(paneId, 'row')}
		>
			<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
				<rect
					x="1"
					y="2"
					width="12"
					height="10"
					rx="1.5"
					stroke="currentColor"
					stroke-width="1.2"
				/>
				<path d="M7 2v10" stroke="currentColor" stroke-width="1.2" />
			</svg>
		</button>
		<button
			type="button"
			class="ws-icon-action-btn"
			data-hover-label="Split horizontal (⌘⇧\)"
			aria-label="Split horizontal"
			onclick={() => onSplitPane(paneId, 'column')}
		>
			<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
				<rect
					x="1"
					y="2"
					width="12"
					height="10"
					rx="1.5"
					stroke="currentColor"
					stroke-width="1.2"
				/>
				<path d="M1 7h12" stroke="currentColor" stroke-width="1.2" />
			</svg>
		</button>
	</div>
</div>

<style>
	.pane-actions {
		display: flex;
		align-items: center;
		gap: 2px;
		padding: 0 6px 0 2px;
		margin-left: auto;
	}

	.split-actions {
		display: flex;
		align-items: center;
		gap: 1px;
		margin-left: 2px;
		padding-left: 6px;
		border-left: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
	}
</style>
