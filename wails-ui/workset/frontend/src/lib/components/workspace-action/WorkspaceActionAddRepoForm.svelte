<script lang="ts">
	import type { Alias, GroupSummary } from '../../types';
	import type {
		ExistingRepoContext,
		WorkspaceActionAddRepoSelectedItem,
	} from '../../services/workspaceActionContextService';
	import WorkspaceActionAddRepoDirectTab from './WorkspaceActionAddRepoDirectTab.svelte';
	import WorkspaceActionAddRepoSummaryPanel from './WorkspaceActionAddRepoSummaryPanel.svelte';
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
		worksetName: string;
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
		worksetName,
		getAliasSource,
		onTabChange,
		onSearchQueryInput,
		onAddSourceInput,
		onBrowse,
		onToggleAlias,
		onToggleGroup,
		onToggleGroupExpand,
		// eslint-disable-next-line @typescript-eslint/no-unused-vars
		onRemoveAlias,
		// eslint-disable-next-line @typescript-eslint/no-unused-vars
		onRemoveGroup,
		onSubmit,
	}: Props = $props();

	const existingRepoNames = $derived(new Set(existingRepos.map((r) => r.name)));
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
				<WorkspaceActionAddRepoDirectTab {addSource} {onAddSourceInput} {onBrowse} />
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
								{@const isExisting = existingRepoNames.has(alias.name)}
								<label
									class="checkbox-item"
									class:selected={selectedAliases.has(alias.name)}
									class:disabled={isExisting}
									title={isExisting ? 'Already in workset' : undefined}
								>
									<input
										type="checkbox"
										checked={selectedAliases.has(alias.name) || isExisting}
										disabled={isExisting}
										onchange={() => !isExisting && onToggleAlias(alias.name)}
									/>
									<div class="checkbox-content">
										<span class="checkbox-name">{alias.name}</span>
										<span class="checkbox-meta">{getAliasSource(alias)}</span>
										{#if isExisting}
											<span class="existing-badge">in workset</span>
										{/if}
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
								{@const groupRepos = groupDetails.get(group.name) || []}
								{@const existingCount = groupRepos.filter((r) => existingRepoNames.has(r)).length}
								{@const allExisting = existingCount === groupRepos.length && groupRepos.length > 0}
								<label
									class="group-card"
									class:selected={selectedGroups.has(group.name)}
									class:disabled={allExisting}
									title={allExisting ? 'All repos in this group are already in workset' : undefined}
								>
									<input
										type="checkbox"
										checked={selectedGroups.has(group.name)}
										disabled={allExisting}
										onchange={() => !allExisting && onToggleGroup(group.name)}
									/>
									<div class="group-content">
										<div class="group-header">
											<span class="group-name">{group.name}</span>
											<span class="group-badge">
												{group.repo_count} repo{group.repo_count !== 1 ? 's' : ''}
												{#if existingCount > 0}
													<span class="existing-count">({existingCount} in workset)</span>
												{/if}
											</span>
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
												{#each groupRepos as repoName (repoName)}
													<li class:existing={existingRepoNames.has(repoName)}>
														{repoName}
														{#if existingRepoNames.has(repoName)}
															<span class="existing-tag">in workset</span>
														{/if}
													</li>
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
		<WorkspaceActionAddRepoSummaryPanel
			{loading}
			{worksetName}
			{existingRepos}
			{addRepoSelectedItems}
			{addRepoTotalItems}
			{onSubmit}
		/>
	</div>
</div>

<style>
	.form {
		display: flex;
		flex-direction: column;
		gap: 12px;
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
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.search-input {
		flex: 1;
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 8px 10px;
		font-size: var(--text-base);
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
		font-size: var(--text-sm);
		cursor: pointer;
		padding: 4px 8px;
	}

	.search-clear:hover {
		color: var(--text);
	}

	.empty-search {
		padding: 20px;
		text-align: center;
		font-size: var(--text-base);
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

	.checkbox-item.disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.checkbox-item.disabled:hover {
		background: transparent;
	}

	.existing-badge {
		font-size: var(--text-xs);
		color: var(--muted);
		background: rgba(255, 255, 255, 0.08);
		padding: 1px 6px;
		border-radius: 4px;
		margin-left: auto;
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
		font-size: var(--text-md);
		color: var(--text);
	}

	.checkbox-meta {
		font-size: var(--text-sm);
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

	.group-card.disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.group-card.disabled:hover {
		background: transparent;
	}

	.existing-count {
		color: var(--muted);
		font-size: var(--text-xs);
		font-weight: 400;
	}

	.group-members li.existing {
		color: var(--muted);
		opacity: 0.6;
	}

	.existing-tag {
		font-size: var(--text-xs);
		color: var(--muted);
		background: rgba(255, 255, 255, 0.08);
		padding: 1px 6px;
		border-radius: 4px;
		margin-left: 8px;
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
		font-size: var(--text-md);
		color: var(--text);
	}

	.group-badge {
		font-size: var(--text-xs);
		color: var(--muted);
		background: rgba(255, 255, 255, 0.05);
		padding: 2px 6px;
		border-radius: var(--radius-sm);
	}

	.group-description {
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.group-expand {
		font-size: var(--text-xs);
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
		font-size: var(--text-sm);
		color: var(--muted);
		list-style: disc;
	}

	.group-members li {
		margin: 2px 0;
	}

	.form.add-two-column {
		display: grid;
		grid-template-columns: 1fr 340px;
		gap: 20px;
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
		font-size: var(--text-base);
	}

	.add-two-column .checkbox-meta {
		font-size: var(--text-xs);
	}

	.add-two-column .group-card {
		padding: 8px 10px;
	}

	.add-two-column .group-name {
		font-size: var(--text-base);
	}

	.add-two-column .group-description {
		font-size: var(--text-xs);
	}

	.selection-area {
		display: flex;
		flex-direction: column;
		gap: 8px;
		flex: 1;
		min-height: 0;
	}
</style>
