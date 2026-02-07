<script lang="ts">
	import type { Alias } from '../../types';

	interface Props {
		searchQuery: string;
		filteredAliases: Alias[];
		selectedAliases: Set<string>;
		getAliasSource: (alias: Alias) => string;
		onSearchQueryInput: (value: string) => void;
		onToggleAlias: (name: string) => void;
	}

	const {
		searchQuery,
		filteredAliases,
		selectedAliases,
		getAliasSource,
		onSearchQueryInput,
		onToggleAlias,
	}: Props = $props();
</script>

<div class="field">
	<div class="inline">
		<input
			value={searchQuery}
			placeholder="Search repos..."
			class="search-input"
			autocapitalize="off"
			autocorrect="off"
			spellcheck="false"
			oninput={(event) => onSearchQueryInput((event.currentTarget as HTMLInputElement).value)}
		/>
		{#if searchQuery}
			<button type="button" class="search-clear" onclick={() => onSearchQueryInput('')}
				>Clear</button
			>
		{/if}
	</div>
	<div class="checkbox-list">
		{#if filteredAliases.length === 0}
			<div class="empty-search">No repos match "{searchQuery}"</div>
		{:else}
			{#each filteredAliases as alias (alias.name)}
				<label class="checkbox-item" class:selected={selectedAliases.has(alias.name)}>
					<input
						type="checkbox"
						checked={selectedAliases.has(alias.name)}
						onchange={() => onToggleAlias(alias.name)}
					/>
					<div class="checkbox-content">
						<span class="checkbox-name">{alias.name}</span>
						<span class="checkbox-meta">{getAliasSource(alias)}</span>
					</div>
				</label>
			{/each}
		{/if}
	</div>
</div>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 12px;
		color: var(--muted);
	}

	.inline {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.search-input {
		flex: 1;
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 8px 10px;
		font-size: 13px;
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast),
			background var(--transition-fast);
	}

	.search-input:focus {
		background: rgba(255, 255, 255, 0.04);
	}

	.search-clear {
		background: transparent;
		border: none;
		color: var(--muted);
		font-size: 12px;
		cursor: pointer;
		padding: 4px 8px;
	}

	.search-clear:hover {
		color: var(--text);
	}

	.empty-search {
		padding: 20px;
		text-align: center;
		font-size: 13px;
		color: var(--muted);
	}

	.checkbox-list {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		max-height: 65vh;
		min-height: 300px;
		overflow-y: auto;
	}

	.checkbox-item {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 6px 10px;
		cursor: pointer;
		transition: all var(--transition-fast);
		border-bottom: 1px solid rgba(255, 255, 255, 0.06);
	}

	.checkbox-item:last-child {
		border-bottom: none;
	}

	.checkbox-item:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.checkbox-item.selected {
		background: rgba(var(--accent-rgb, 59, 130, 246), 0.08);
	}

	.checkbox-item input[type='checkbox'] {
		appearance: none;
		-webkit-appearance: none;
		width: 16px;
		height: 16px;
		min-width: 16px;
		min-height: 16px;
		flex-shrink: 0;
		background: var(--panel-strong);
		border: 2px solid rgba(255, 255, 255, 0.2);
		border-radius: 4px;
		cursor: pointer;
		display: grid;
		place-content: center;
		transition: all var(--transition-fast);
	}

	.checkbox-item input[type='checkbox']:hover {
		border-color: rgba(255, 255, 255, 0.4);
		background: var(--panel);
	}

	.checkbox-item input[type='checkbox']:checked {
		background: var(--accent);
		border-color: var(--accent);
	}

	.checkbox-item input[type='checkbox']::before {
		content: '';
		width: 8px;
		height: 8px;
		transform: scale(0);
		transition: transform 0.1s ease-in-out;
		clip-path: polygon(14% 44%, 0 65%, 50% 100%, 100% 16%, 80% 0%, 43% 62%);
		background: #0a0f14;
	}

	.checkbox-item input[type='checkbox']:checked::before {
		transform: scale(1);
	}

	.checkbox-content {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
		flex: 1;
	}

	.checkbox-name {
		font-weight: 500;
		font-size: 13px;
		color: var(--text);
	}

	.checkbox-meta {
		font-size: 11px;
		color: var(--muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
</style>
