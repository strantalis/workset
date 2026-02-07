<script lang="ts">
	import type { Alias, GroupSummary } from '../../types';
	import type {
		ExistingRepoContext,
		WorkspaceActionAddRepoSelectedItem,
	} from '../../services/workspaceActionContextService';
	import Button from '../ui/Button.svelte';
	import WorkspaceActionSelectionTabs from './WorkspaceActionSelectionTabs.svelte';

	type CreateTab = 'direct' | 'repos' | 'groups';

	interface Props {
		loading: boolean;
		activeTab: CreateTab;
		aliasItems: Alias[];
		groupItems: GroupSummary[];
		searchQuery: string;
		addSource: string;
		filteredAliases: Alias[];
		filteredGroups: GroupSummary[];
		selectedAliases: Set<string>;
		selectedGroups: Set<string>;
		expandedGroups: Set<string>;
		groupDetails: Map<string, string[]>;
		existingRepos: ExistingRepoContext[];
		addRepoSelectedItems: WorkspaceActionAddRepoSelectedItem[];
		addRepoTotalItems: number;
		getAliasSource: (alias: Alias) => string;
		onTabChange: (tab: CreateTab) => void;
		onSearchQueryInput: (value: string) => void;
		onAddSourceInput: (value: string) => void;
		onBrowse: () => void;
		onToggleAlias: (name: string) => void;
		onToggleGroup: (name: string) => void;
		onToggleGroupExpand: (name: string) => void;
		onRemoveAlias: (name: string) => void;
		onRemoveGroup: (name: string) => void;
		onSubmit: () => void;
	}

	const {
		loading,
		activeTab,
		aliasItems,
		groupItems,
		searchQuery,
		addSource,
		filteredAliases,
		filteredGroups,
		selectedAliases,
		selectedGroups,
		expandedGroups,
		groupDetails,
		existingRepos,
		addRepoSelectedItems,
		addRepoTotalItems,
		getAliasSource,
		onTabChange,
		onSearchQueryInput,
		onAddSourceInput,
		onBrowse,
		onToggleAlias,
		onToggleGroup,
		onToggleGroupExpand,
		onRemoveAlias,
		onRemoveGroup,
		onSubmit,
	}: Props = $props();
</script>

<div class="form add-two-column">
	<div class="column-left">
		<WorkspaceActionSelectionTabs
			{activeTab}
			aliasCount={aliasItems.length}
			groupCount={groupItems.length}
			{onTabChange}
		/>

		<div class="selection-area">
			{#if activeTab === 'direct'}
				<label class="field">
					<span>Repo URL or local path</span>
					<div class="inline">
						<input
							value={addSource}
							oninput={(event) => onAddSourceInput((event.currentTarget as HTMLInputElement).value)}
							placeholder="git@github.com:org/repo.git"
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
						<Button variant="ghost" size="sm" onclick={onBrowse}>Browse</Button>
					</div>
				</label>
			{:else if activeTab === 'repos'}
				<div class="field">
					<div class="inline">
						<input
							value={searchQuery}
							oninput={(event) =>
								onSearchQueryInput((event.currentTarget as HTMLInputElement).value)}
							placeholder="Search repos..."
							class="search-input"
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
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
			{:else if activeTab === 'groups'}
				<div class="field">
					<div class="inline">
						<input
							value={searchQuery}
							oninput={(event) =>
								onSearchQueryInput((event.currentTarget as HTMLInputElement).value)}
							placeholder="Search groups..."
							class="search-input"
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
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
			{/if}
		</div>

		{#if aliasItems.length === 0 && groupItems.length === 0}
			<div class="hint">No registered repos or groups configured. Add them in Settings.</div>
		{/if}
	</div>

	<div class="column-right">
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
	</div>
</div>

<style>
	.form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 12px;
		color: var(--muted);
	}

	.field input {
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		color: var(--text);
		padding: 8px 10px;
		font-size: 14px;
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.field input:focus {
		background: rgba(255, 255, 255, 0.04);
	}

	.inline {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.inline input {
		flex: 1;
	}

	.hint {
		font-size: 12px;
		color: var(--muted);
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
		max-height: 200px;
		overflow-y: auto;
	}

	.checkbox-item {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
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
		width: 18px;
		height: 18px;
		min-width: 18px;
		min-height: 18px;
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
		font-size: 14px;
		color: var(--text);
	}

	.checkbox-meta {
		font-size: 12px;
		color: var(--muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.group-list {
		display: flex;
		flex-direction: column;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		max-height: 240px;
		overflow-y: auto;
	}

	.group-card {
		display: flex;
		align-items: flex-start;
		gap: 12px;
		padding: 12px;
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
		font-size: 14px;
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
		font-size: 12px;
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

	.form.add-two-column {
		display: grid;
		grid-template-columns: 1fr 280px;
		gap: 16px;
		max-height: 80vh;
		min-height: 400px;
		overflow: hidden;
	}

	.add-two-column .column-left {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		overflow: hidden;
	}

	.add-two-column .column-right {
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow: hidden;
	}

	.add-two-column .checkbox-list,
	.add-two-column .group-list {
		max-height: 65vh;
		min-height: 200px;
	}

	.add-two-column .checkbox-item {
		padding: 6px 10px;
	}

	.add-two-column .checkbox-item input[type='checkbox'] {
		width: 16px;
		height: 16px;
		min-width: 16px;
		min-height: 16px;
	}

	.add-two-column .checkbox-name {
		font-size: 13px;
	}

	.add-two-column .checkbox-meta {
		font-size: 11px;
	}

	.add-two-column .group-card {
		padding: 8px 10px;
	}

	.add-two-column .group-name {
		font-size: 13px;
	}

	.add-two-column .group-description {
		font-size: 11px;
	}

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

	.selection-area {
		display: flex;
		flex-direction: column;
		gap: 8px;
		flex: 1;
		min-height: 0;
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
