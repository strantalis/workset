<script lang="ts">
	import type {
		ExistingRepoContext,
		WorkspaceActionAddRepoSelectedItem,
	} from '../../services/workspaceActionContextService';
	import Button from '../ui/Button.svelte';

	interface Props {
		loading: boolean;
		existingRepos: ExistingRepoContext[];
		addRepoSelectedItems: WorkspaceActionAddRepoSelectedItem[];
		addRepoTotalItems: number;
		onAddSourceInput: (value: string) => void;
		onRemoveAlias: (name: string) => void;
		onRemoveGroup: (name: string) => void;
		onSubmit: () => void;
	}

	const {
		loading,
		existingRepos,
		addRepoSelectedItems,
		addRepoTotalItems,
		onAddSourceInput,
		onRemoveAlias,
		onRemoveGroup,
		onSubmit,
	}: Props = $props();
</script>

<div class="selection-panel">
	{#if existingRepos.length > 0}
		<div class="panel-section existing-section">
			<span class="panel-label">Already in workspace ({existingRepos.length} repos)</span>
			<div class="existing-list">
				{#each existingRepos as repo (repo.name)}
					<div class="existing-item">
						<span class="selected-badge existing">repo</span>
						<span class="selected-name">{repo.name}</span>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	<h4 class="panel-title">Selected ({addRepoTotalItems} items)</h4>

	<div class="selected-list">
		{#if addRepoSelectedItems.length === 0}
			<div class="empty-selection">No items selected</div>
		{:else}
			{#each addRepoSelectedItems as item (item.name)}
				<div class="selected-item">
					<span class="selected-badge {item.type}">{item.type}</span>
					<span class="selected-name">{item.name}</span>
					<button
						type="button"
						class="selected-remove"
						onclick={() => {
							if (item.type === 'repo') onAddSourceInput('');
							else if (item.type === 'alias') onRemoveAlias(item.name);
							else if (item.type === 'group') onRemoveGroup(item.name);
						}}
					>
						×
					</button>
				</div>
			{/each}
		{/if}
	</div>

	<Button
		variant="primary"
		onclick={onSubmit}
		disabled={loading || addRepoTotalItems === 0}
		class="create-btn"
	>
		{loading ? 'Adding…' : 'Add'}
	</Button>
</div>

<style>
	.selection-panel {
		display: flex;
		flex-direction: column;
		gap: 12px;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 12px;
		height: 100%;
		max-height: 100%;
		overflow: hidden;
	}

	.panel-title {
		margin: 0;
		font-size: 14px;
		font-weight: 600;
		color: var(--text);
		padding-bottom: 8px;
		border-bottom: 1px solid var(--border);
	}

	.panel-label {
		font-size: 12px;
		color: var(--muted);
	}

	.panel-section {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.selected-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
		overflow-y: auto;
		flex: 1;
		min-height: 0;
	}

	.empty-selection {
		font-size: 13px;
		color: var(--muted);
		font-style: italic;
		padding: 12px 0;
		text-align: center;
	}

	.selected-item {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 8px;
		background: rgba(255, 255, 255, 0.03);
		border-radius: var(--radius-sm);
		font-size: 13px;
	}

	.selected-badge {
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		padding: 2px 6px;
		border-radius: var(--radius-sm);
		white-space: nowrap;
		flex-shrink: 0;
	}

	.selected-badge.repo {
		background: var(--accent);
		color: #0a0f14;
	}

	.selected-badge.alias {
		background: #8b5cf6;
		color: white;
	}

	.selected-badge.group {
		background: #f59e0b;
		color: #0a0f14;
	}

	.selected-name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text);
	}

	.selected-remove {
		background: transparent;
		border: none;
		color: var(--muted);
		cursor: pointer;
		padding: 0 4px;
		font-size: 18px;
		line-height: 1;
		transition: color var(--transition-fast);
		flex-shrink: 0;
	}

	.selected-remove:hover {
		color: var(--danger, #ef4444);
	}

	.existing-section {
		background: rgba(255, 255, 255, 0.03);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: var(--radius-md);
		padding: 12px;
		margin-bottom: 8px;
	}

	.existing-section .panel-label {
		font-weight: 600;
		color: var(--text);
		font-size: 13px;
	}

	.existing-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.existing-item {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 4px 0;
		font-size: 13px;
		opacity: 0.8;
	}

	.existing-item .selected-badge {
		background: rgba(255, 255, 255, 0.15);
		color: var(--muted);
	}

	.create-btn {
		padding: 10px 32px;
		min-width: 100px;
		align-self: flex-end;
	}

	:global(.create-btn) {
		margin-top: 0;
		width: auto;
	}
</style>
