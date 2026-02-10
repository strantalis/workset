<script lang="ts">
	import type { WorkspaceActionPreviewItem } from '../../services/workspaceActionContextService';
	import Button from '../ui/Button.svelte';

	interface Props {
		selectedItems: WorkspaceActionPreviewItem[];
		totalRepos: number;
		customizeName: string;
		generatedName: string | null;
		alternatives: string[];
		finalName: string;
		loading: boolean;
		onCustomizeNameInput: (value: string) => void;
		onSelectAlternative: (name: string) => void;
		onSubmit: () => void;
		onRemoveDirectRepo: (url: string) => void;
		onRemoveAlias: (name: string) => void;
		onRemoveGroup: (name: string) => void;
	}

	const {
		selectedItems,
		totalRepos,
		customizeName,
		generatedName,
		alternatives,
		finalName,
		loading,
		onCustomizeNameInput,
		onSelectAlternative,
		onSubmit,
		onRemoveDirectRepo,
		onRemoveAlias,
		onRemoveGroup,
	}: Props = $props();
</script>

<div class="selection-panel">
	<h4 class="panel-title">Selected ({totalRepos} repos)</h4>

	<div class="selected-list">
		{#if selectedItems.length === 0}
			<div class="empty-selection">No repos selected</div>
		{:else}
			{#each selectedItems as item (item.name)}
				<div class="selected-item" class:pending={item.pending}>
					<span class="selected-badge {item.type}">{item.type}</span>
					<span class="selected-name">{item.name}</span>
					{#if item.pending}
						<span class="pending-label">pending</span>
					{:else}
						<button
							type="button"
							class="selected-remove"
							onclick={() => {
								if (item.type === 'repo' && item.url) onRemoveDirectRepo(item.url);
								else if (item.type === 'alias') onRemoveAlias(item.name);
								else if (item.type === 'group') onRemoveGroup(item.name);
							}}
						>
							×
						</button>
					{/if}
				</div>
			{/each}
		{/if}
	</div>

	<div class="panel-section">
		<span class="panel-label">Workspace name</span>
		<input
			value={customizeName}
			oninput={(event) => onCustomizeNameInput((event.currentTarget as HTMLInputElement).value)}
			placeholder={generatedName || 'workspace-name'}
			class="name-input"
			autocapitalize="off"
			autocorrect="off"
			spellcheck="false"
		/>
		{#if alternatives.length > 0}
			<div class="alt-chips">
				{#each alternatives as alt, i (i)}
					<button type="button" class="alt-chip" onclick={() => onSelectAlternative(alt)}
						>{alt}</button
					>
				{/each}
			</div>
		{/if}
	</div>

	<Button variant="primary" onclick={onSubmit} disabled={loading || !finalName} class="create-btn">
		{loading ? 'Creating…' : 'Create'}
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
		font-size: var(--text-md);
		font-weight: 600;
		color: var(--text);
		padding-bottom: 8px;
		border-bottom: 1px solid var(--border);
	}

	.panel-label {
		font-size: var(--text-sm);
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
		font-size: var(--text-base);
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
		font-size: var(--text-base);
	}

	.selected-item.pending {
		background: rgba(255, 255, 255, 0.01);
		border: 1px dashed rgba(255, 255, 255, 0.15);
	}

	.pending-label {
		font-size: var(--text-xs);
		color: var(--muted);
		font-style: italic;
	}

	.selected-badge {
		font-size: var(--text-xs);
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
		font-size: var(--text-xl);
		line-height: 1;
		transition: color var(--transition-fast);
		flex-shrink: 0;
	}

	.selected-remove:hover {
		color: var(--danger, #ef4444);
	}

	.name-input {
		width: 100%;
		font-size: var(--text-md);
		padding: 10px 12px;
		background: transparent;
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		color: var(--text);
		box-sizing: border-box;
	}

	.name-input:focus {
		outline: none;
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.02);
	}

	.alt-chips {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.alt-chip {
		background: transparent;
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		color: var(--accent);
		cursor: pointer;
		padding: 6px 12px;
		font-size: var(--text-sm);
		transition: all var(--transition-fast);
	}

	.alt-chip:hover {
		background: rgba(255, 255, 255, 0.05);
		border-color: var(--accent);
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
