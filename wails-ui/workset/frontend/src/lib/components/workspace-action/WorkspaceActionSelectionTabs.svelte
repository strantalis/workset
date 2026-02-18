<script lang="ts">
	export type WorkspaceActionSelectionTab = 'direct' | 'repos' | 'groups';

	interface Props {
		activeTab: WorkspaceActionSelectionTab;
		aliasCount: number;
		groupCount: number;
		onTabChange: (tab: WorkspaceActionSelectionTab) => void;
	}

	const { activeTab, aliasCount, groupCount, onTabChange }: Props = $props();
</script>

{#if aliasCount > 0 || groupCount > 0}
	<div class="tab-bar">
		<button
			class="tab"
			class:active={activeTab === 'direct'}
			type="button"
			onclick={() => onTabChange('direct')}
		>
			Direct
		</button>
		{#if aliasCount > 0}
			<button
				class="tab"
				class:active={activeTab === 'repos'}
				type="button"
				onclick={() => onTabChange('repos')}
			>
				Repos ({aliasCount})
			</button>
		{/if}
		{#if groupCount > 0}
			<button
				class="tab"
				class:active={activeTab === 'groups'}
				type="button"
				onclick={() => onTabChange('groups')}
			>
				Groups ({groupCount})
			</button>
		{/if}
	</div>
{/if}

<style>
	.tab-bar {
		display: flex;
		gap: 8px;
		border-bottom: 1px solid var(--border);
		padding-bottom: 8px;
	}

	.tab {
		display: flex;
		align-items: center;
		gap: 6px;
		background: transparent;
		border: none;
		color: var(--muted);
		padding: 6px 12px;
		font-size: var(--text-base);
		cursor: pointer;
		border-radius: var(--radius-md);
		transition: all var(--transition-fast);
	}

	.tab:hover {
		color: var(--text);
		background: rgba(255, 255, 255, 0.05);
	}

	.tab.active {
		color: var(--text);
		background: var(--accent);
		font-weight: 500;
	}
</style>
