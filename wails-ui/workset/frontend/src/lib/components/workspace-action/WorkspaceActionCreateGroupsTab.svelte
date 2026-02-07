<script lang="ts">
	import type { GroupSummary } from '../../types';

	interface Props {
		searchQuery: string;
		filteredGroups: GroupSummary[];
		selectedGroups: Set<string>;
		expandedGroups: Set<string>;
		groupDetails: Map<string, string[]>;
		onSearchQueryInput: (value: string) => void;
		onToggleGroup: (name: string) => void;
		onToggleGroupExpand: (name: string) => void;
	}

	const {
		searchQuery,
		filteredGroups,
		selectedGroups,
		expandedGroups,
		groupDetails,
		onSearchQueryInput,
		onToggleGroup,
		onToggleGroupExpand,
	}: Props = $props();
</script>

<div class="field">
	<div class="inline">
		<input
			value={searchQuery}
			placeholder="Search groups..."
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
	<div class="group-list">
		{#if filteredGroups.length === 0}
			<div class="empty-search">No groups match "{searchQuery}"</div>
		{:else}
			{#each filteredGroups as group (group.name)}
				<label class="group-card" class:selected={selectedGroups.has(group.name)}>
					<input
						type="checkbox"
						checked={selectedGroups.has(group.name)}
						onchange={() => onToggleGroup(group.name)}
					/>
					<div class="group-content">
						<div class="group-header">
							<span class="group-name">{group.name}</span>
							<span class="group-badge"
								>{group.repo_count} repo{group.repo_count !== 1 ? 's' : ''}</span
							>
						</div>
						{#if group.description}
							<span class="group-description">{group.description}</span>
						{/if}
						<button
							type="button"
							class="group-expand"
							onclick={(event) => {
								event.preventDefault();
								onToggleGroupExpand(group.name);
							}}
						>
							{expandedGroups.has(group.name) ? '▾ Hide' : '▸ Show'} repos
						</button>
						{#if expandedGroups.has(group.name)}
							<ul class="group-members">
								{#each groupDetails.get(group.name) || [] as repoName (repoName)}
									<li>{repoName}</li>
								{/each}
							</ul>
						{/if}
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

	.group-list {
		display: flex;
		flex-direction: column;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		max-height: 65vh;
		min-height: 300px;
		overflow-y: auto;
	}

	.group-card {
		display: flex;
		align-items: flex-start;
		gap: 12px;
		padding: 8px 10px;
		cursor: pointer;
		transition: all var(--transition-fast);
		border-bottom: 1px solid rgba(255, 255, 255, 0.06);
	}

	.group-card:last-child {
		border-bottom: none;
	}

	.group-card:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.group-card.selected {
		background: rgba(var(--accent-rgb, 59, 130, 246), 0.08);
	}

	.group-card input[type='checkbox'] {
		appearance: none;
		-webkit-appearance: none;
		width: 18px;
		height: 18px;
		min-width: 18px;
		margin-top: 2px;
		background: var(--panel-strong);
		border: 2px solid rgba(255, 255, 255, 0.2);
		border-radius: 4px;
		cursor: pointer;
		display: grid;
		place-content: center;
		transition: all var(--transition-fast);
	}

	.group-card input[type='checkbox']:hover {
		border-color: rgba(255, 255, 255, 0.4);
		background: var(--panel);
	}

	.group-card input[type='checkbox']:checked {
		background: var(--accent);
		border-color: var(--accent);
	}

	.group-card input[type='checkbox']::before {
		content: '';
		width: 8px;
		height: 8px;
		transform: scale(0);
		transition: transform 0.1s ease-in-out;
		clip-path: polygon(14% 44%, 0 65%, 50% 100%, 100% 16%, 80% 0%, 43% 62%);
		background: #0a0f14;
	}

	.group-card input[type='checkbox']:checked::before {
		transform: scale(1);
	}

	.group-content {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.group-header {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.group-name {
		font-weight: 500;
		font-size: 13px;
		color: var(--text);
	}

	.group-badge {
		font-size: 11px;
		color: var(--muted);
		background: rgba(255, 255, 255, 0.05);
		padding: 2px 6px;
		border-radius: var(--radius-sm);
	}

	.group-description {
		font-size: 11px;
		color: var(--muted);
	}

	.group-expand {
		font-size: 11px;
		color: var(--accent);
		background: transparent;
		border: none;
		padding: 0;
		cursor: pointer;
		text-align: left;
		margin-top: 2px;
	}

	.group-expand:hover {
		text-decoration: underline;
	}

	.group-members {
		margin: 6px 0 0 0;
		padding-left: 16px;
		font-size: 12px;
		color: var(--muted);
		list-style: disc;
	}

	.group-members li {
		margin: 2px 0;
	}
</style>
