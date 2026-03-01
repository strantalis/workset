<script lang="ts">
	import { onDestroy } from 'svelte';
	import { ArrowRight, Check, FolderOpen, Loader2, Search } from '@lucide/svelte';
	import { searchGitHubRepositories } from '../../api/github';
	import { deriveRepoName, looksLikeUrl } from '../../names';
	import type { GitHubRepoSearchItem } from '../../types';
	import type { Alias } from '../../types';
	import type {
		ExistingRepoContext,
		WorkspaceActionAddRepoSelectedItem,
	} from '../../services/workspaceActionContextService';
	import WorkspaceActionAddRepoSummaryPanel from './WorkspaceActionAddRepoSummaryPanel.svelte';

	interface Props {
		loading: boolean;
		aliasItems: Alias[];
		searchQuery: string;
		addSource: string;
		filteredAliases: Alias[];
		selectedAliases: Set<string>;
		existingRepos: ExistingRepoContext[];
		addRepoSelectedItems: WorkspaceActionAddRepoSelectedItem[];
		addRepoTotalItems: number;
		worksetName: string;
		getAliasSource: (alias: Alias) => string;
		onSearchQueryInput: (value: string) => void;
		onAddSourceInput: (value: string) => void;
		onBrowse: () => void;
		onToggleAlias: (name: string) => void;
		onRemoveAlias: (name: string) => void;
		onSubmit: () => void;
	}

	const props: Props = $props();

	const loading = $derived(props.loading);
	const aliasItems = $derived(props.aliasItems);
	const searchQuery = $derived(props.searchQuery);
	const filteredAliases = $derived(props.filteredAliases);
	const selectedAliases = $derived(props.selectedAliases);
	const existingRepos = $derived(props.existingRepos);
	const addRepoSelectedItems = $derived(props.addRepoSelectedItems);
	const addRepoTotalItems = $derived(props.addRepoTotalItems);
	const worksetName = $derived(props.worksetName);

	const getAliasSource = (alias: Alias): string => props.getAliasSource(alias);
	const onSearchQueryInput = (value: string): void => props.onSearchQueryInput(value);
	const onAddSourceInput = (value: string): void => props.onAddSourceInput(value);
	const onBrowse = (): void => props.onBrowse();
	const onToggleAlias = (name: string): void => props.onToggleAlias(name);
	const onRemoveAlias = (name: string): void => props.onRemoveAlias(name);
	const onSubmit = (): void => props.onSubmit();

	let sourceDraft = $state('');
	let remoteSuggestions: GitHubRepoSearchItem[] = $state([]);
	let searchLoading = $state(false);
	let searchError: string | null = $state(null);
	let sourceSearchDebounce: ReturnType<typeof setTimeout> | null = null;
	let sourceSearchSequence = 0;
	let lastSearchedQuery = $state('');

	const isLikelyLocalPath = (value: string): boolean => {
		const trimmed = value.trim();
		return (
			trimmed.startsWith('/') ||
			trimmed.startsWith('./') ||
			trimmed.startsWith('../') ||
			trimmed.startsWith('~') ||
			/^[a-zA-Z]:[\\/]/.test(trimmed) ||
			trimmed.includes('\\')
		);
	};

	const existingRepoNames = $derived(new Set(existingRepos.map((repo) => repo.name)));
	const sourceQuery = $derived(sourceDraft.trim());
	const canAddSource = $derived(looksLikeUrl(sourceQuery) || isLikelyLocalPath(sourceQuery));
	const showSearchMinCharsHint = $derived(
		sourceQuery.length > 0 &&
			sourceQuery.length < 2 &&
			!looksLikeUrl(sourceQuery) &&
			!isLikelyLocalPath(sourceQuery),
	);
	const showNoSearchResults = $derived(
		!searchLoading &&
			searchError === null &&
			!showSearchMinCharsHint &&
			sourceQuery.length >= 2 &&
			remoteSuggestions.length === 0 &&
			lastSearchedQuery === sourceQuery,
	);
	const availableAliases = $derived(
		filteredAliases.filter((alias) => !existingRepoNames.has(alias.name)),
	);
	const inWorksetAliases = $derived(
		filteredAliases.filter((alias) => existingRepoNames.has(alias.name)),
	);
	const showSearchMeta = $derived(
		showSearchMinCharsHint || searchLoading || searchError !== null || showNoSearchResults,
	);
	const hasPendingSource = $derived(canAddSource && sourceQuery.length > 0);

	const shouldSearchRemote = (value: string): boolean => {
		const trimmed = value.trim();
		return trimmed.length >= 2 && !looksLikeUrl(trimmed) && !isLikelyLocalPath(trimmed);
	};

	const clearSearchTimer = (): void => {
		if (sourceSearchDebounce) {
			clearTimeout(sourceSearchDebounce);
			sourceSearchDebounce = null;
		}
	};

	const resetRemoteSuggestions = (): void => {
		clearSearchTimer();
		sourceSearchSequence += 1;
		remoteSuggestions = [];
		searchLoading = false;
		searchError = null;
		lastSearchedQuery = '';
	};

	const toSearchErrorMessage = (err: unknown): string => {
		const message = err instanceof Error ? err.message : 'Failed to search repositories.';
		const normalized = message.toLowerCase();
		if (
			normalized.includes('auth required') ||
			normalized.includes('not authenticated') ||
			normalized.includes('authentication') ||
			normalized.includes('authenticate') ||
			normalized.includes('github auth')
		) {
			return 'Connect GitHub in Settings -> GitHub authentication to search.';
		}
		return message;
	};

	const runRemoteSearch = async (query: string): Promise<void> => {
		const requestSequence = ++sourceSearchSequence;
		searchLoading = true;
		searchError = null;
		lastSearchedQuery = query;
		try {
			const results = await searchGitHubRepositories(query, 8);
			if (requestSequence !== sourceSearchSequence) return;
			remoteSuggestions = results;
		} catch (err) {
			if (requestSequence !== sourceSearchSequence) return;
			remoteSuggestions = [];
			searchError = toSearchErrorMessage(err);
		} finally {
			if (requestSequence === sourceSearchSequence) {
				searchLoading = false;
			}
		}
	};

	const queueRemoteSearch = (value: string): void => {
		const query = value.trim();
		clearSearchTimer();
		if (!shouldSearchRemote(query)) {
			sourceSearchSequence += 1;
			remoteSuggestions = [];
			searchLoading = false;
			searchError = null;
			lastSearchedQuery = query;
			return;
		}
		sourceSearchDebounce = setTimeout(() => {
			void runRemoteSearch(query);
		}, 250);
	};

	const handleSourceInput = (value: string): void => {
		sourceDraft = value;
		onSearchQueryInput(value);
		queueRemoteSearch(value);
	};

	const commitDirectSource = (value: string): void => {
		const trimmed = value.trim();
		if (trimmed.length === 0) return;
		onAddSourceInput(trimmed);
		onSearchQueryInput('');
		sourceDraft = '';
		resetRemoteSuggestions();
	};

	const handleAddSource = (): void => {
		if (!canAddSource) return;
		commitDirectSource(sourceDraft);
	};

	const handleContinue = (): void => {
		if (hasPendingSource) {
			commitDirectSource(sourceDraft);
		}
		onSubmit();
	};

	const handleSelectRemoteSuggestion = (suggestion: GitHubRepoSearchItem): void => {
		const source = suggestion.sshUrl || suggestion.cloneUrl;
		commitDirectSource(source);
	};

	const handleRemoveSelection = (item: WorkspaceActionAddRepoSelectedItem): void => {
		if (item.type === 'alias') {
			onRemoveAlias(item.name);
			return;
		}
		onAddSourceInput('');
	};

	const formatSelectionLabel = (item: WorkspaceActionAddRepoSelectedItem): string => {
		if (item.type !== 'repo') return item.name;
		return deriveRepoName(item.name) || item.name;
	};

	const formatSelectionKind = (item: WorkspaceActionAddRepoSelectedItem): string => {
		if (item.type === 'alias') return 'Catalog';
		return 'Source';
	};

	onDestroy(() => {
		clearSearchTimer();
	});
</script>

<div class="form add-two-column ws-form-stack add-flow-shell">
	<div class="column-left">
		<div class="field">
			<div class="repo-input-row">
				<div class="repo-input-shell">
					<span class="repo-input-icon"><Search size={14} /></span>
					<input
						type="text"
						value={sourceDraft}
						oninput={(event) => handleSourceInput((event.currentTarget as HTMLInputElement).value)}
						onkeydown={(event) => {
							if (event.key !== 'Enter') return;
							event.preventDefault();
							handleAddSource();
						}}
						placeholder="Search catalog/GitHub, or paste repo URL/path (Enter)"
						class="mono"
						autocapitalize="off"
						autocorrect="off"
						spellcheck="false"
					/>
				</div>
				<button
					type="button"
					class="repo-browse-icon-btn"
					onclick={onBrowse}
					aria-label="Browse local path"
					title="Browse local path"
				>
					<FolderOpen size={14} />
				</button>
			</div>
			{#if showSearchMeta}
				<div class="repo-input-help-row">
					{#if showSearchMinCharsHint}
						<span class="repo-search-status">Type at least 2 characters to search GitHub.</span>
					{:else if searchLoading}
						<span class="repo-search-status">
							<Loader2 size={12} />
							<span>Searching GitHub…</span>
						</span>
					{:else if searchError}
						<span class="repo-search-error">{searchError}</span>
					{:else if showNoSearchResults}
						<span class="repo-search-status">No GitHub repositories found for "{sourceQuery}".</span
						>
					{/if}
				</div>
			{/if}
		</div>

		{#if addRepoSelectedItems.length > 0}
			<div class="selected-repos-panel">
				<div class="selected-repos-header">
					<span>Selected Repositories</span>
					<span>{addRepoSelectedItems.length}</span>
				</div>
				<div class="selected-repos-list">
					{#each addRepoSelectedItems as item (`${item.type}:${item.name}`)}
						<button
							type="button"
							class="selected-repo-chip"
							onclick={() => handleRemoveSelection(item)}
						>
							<span class="selected-repo-kind">{formatSelectionKind(item)}</span>
							<span>{formatSelectionLabel(item)}</span>
							<span class="selected-repo-remove">x</span>
						</button>
					{/each}
				</div>
			</div>
		{/if}

		<div class="field">
			<div class="registry-list">
				{#if availableAliases.length > 0}
					<div class="result-group-label">Available</div>
					{#each availableAliases as alias (alias.name)}
						<button
							type="button"
							class="registry-item"
							class:selected={selectedAliases.has(alias.name)}
							onclick={() => onToggleAlias(alias.name)}
						>
							<div class="registry-check" class:checked={selectedAliases.has(alias.name)}>
								{#if selectedAliases.has(alias.name)}<Check size={10} />{/if}
							</div>
							<div class="registry-info">
								<div class="registry-name-row">
									<span class="registry-name">{alias.name}</span>
									<span class="source-badge source-badge-catalog">Catalog</span>
								</div>
								<div class="registry-url">{getAliasSource(alias)}</div>
							</div>
						</button>
					{/each}
				{/if}

				{#if inWorksetAliases.length > 0}
					<div class="result-group-label">Already In Workset</div>
					{#each inWorksetAliases as alias (alias.name)}
						<div class="registry-item registry-item-existing" aria-disabled="true">
							<div class="registry-check checked">
								<Check size={10} />
							</div>
							<div class="registry-info">
								<div class="registry-name-row">
									<span class="registry-name">{alias.name}</span>
									<span class="source-badge source-badge-in-workset">In workset</span>
								</div>
								<div class="registry-url">{getAliasSource(alias)}</div>
							</div>
						</div>
					{/each}
				{/if}

				{#if remoteSuggestions.length > 0}
					<div class="result-group-label">GitHub</div>
					{#each remoteSuggestions as suggestion (`${suggestion.owner}/${suggestion.name}`)}
						<button
							type="button"
							class="registry-item github-result"
							onclick={() => handleSelectRemoteSuggestion(suggestion)}
						>
							<div class="registry-check github-result-check">
								<ArrowRight size={10} />
							</div>
							<div class="registry-info">
								<div class="registry-name-row">
									<span class="registry-name">{suggestion.owner}/{suggestion.name}</span>
									<span class="source-badge source-badge-github">GitHub</span>
								</div>
								<div class="registry-url">{suggestion.sshUrl || suggestion.cloneUrl}</div>
							</div>
						</button>
					{/each}
				{/if}

				{#if availableAliases.length === 0 && inWorksetAliases.length === 0 && remoteSuggestions.length === 0}
					<div class="registry-empty">
						{#if aliasItems.length === 0}
							No repositories available in Repo Catalog yet.
						{:else if searchQuery.trim().length > 0}
							No matching repositories.
						{:else}
							Start typing to search Repo Catalog or GitHub.
						{/if}
					</div>
				{/if}
			</div>
		</div>
	</div>

	<div class="column-right">
		<WorkspaceActionAddRepoSummaryPanel
			{loading}
			{worksetName}
			{existingRepos}
			{addRepoTotalItems}
			{hasPendingSource}
			onSubmit={handleContinue}
		/>
	</div>
</div>

<style>
	.add-flow-shell {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 340px;
		gap: 14px;
		align-items: start;
		min-height: 0;
	}

	.column-left,
	.column-right {
		min-width: 0;
	}

	.column-left {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.column-right {
		position: sticky;
		top: 0;
	}

	.repo-input-row {
		--repo-control-height: 36px;
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: 8px;
		align-items: stretch;
	}

	.repo-input-shell {
		position: relative;
		display: flex;
		align-items: center;
	}

	.repo-input-icon {
		position: absolute;
		left: 10px;
		color: var(--muted);
		pointer-events: none;
	}

	.repo-input-shell input {
		width: 100%;
		height: var(--repo-control-height);
		box-sizing: border-box;
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 9px 10px 9px 30px;
		font-size: var(--text-base);
		color: var(--text);
	}

	.repo-input-shell input:focus {
		outline: none;
		border-color: color-mix(in srgb, var(--accent) 48%, var(--border));
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 26%, transparent);
		background: rgba(255, 255, 255, 0.04);
	}

	.repo-browse-icon-btn {
		width: var(--repo-control-height);
		height: var(--repo-control-height);
		border: 1px solid color-mix(in srgb, var(--border) 82%, transparent);
		border-radius: var(--radius-md);
		background: color-mix(in srgb, var(--panel-strong) 72%, transparent);
		color: var(--muted);
		display: inline-flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		transition:
			color var(--transition-fast),
			border-color var(--transition-fast),
			background var(--transition-fast);
	}

	.repo-browse-icon-btn:hover {
		color: var(--text);
		border-color: color-mix(in srgb, var(--accent) 36%, var(--border));
		background: color-mix(in srgb, var(--panel) 76%, transparent);
	}

	.repo-input-help-row {
		margin-top: 6px;
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 10px;
	}

	.repo-search-status,
	.repo-search-error {
		font-size: var(--text-sm);
		color: var(--muted);
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}

	.repo-search-error {
		color: var(--danger-text);
	}

	.selected-repos-panel {
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 10px;
		background: color-mix(in srgb, var(--panel) 86%, transparent);
	}

	.selected-repos-header {
		display: flex;
		justify-content: space-between;
		font-size: var(--text-sm);
		color: var(--muted);
		margin-bottom: 8px;
	}

	.selected-repos-list {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.selected-repo-chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 8px;
		border-radius: 999px;
		border: 1px solid color-mix(in srgb, var(--border) 80%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 68%, transparent);
		color: var(--text);
		font-size: var(--text-xs);
		cursor: pointer;
	}

	.selected-repo-kind {
		font-size: 10px;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.selected-repo-remove {
		color: var(--muted);
	}

	.registry-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
		max-height: 294px;
		overflow-y: auto;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 8px;
	}

	.result-group-label {
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: color-mix(in srgb, var(--muted) 84%, transparent);
		padding: 4px 2px;
	}

	.registry-item {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		border: 1px solid transparent;
		background: transparent;
		border-radius: 8px;
		padding: 8px;
		color: var(--text);
		cursor: pointer;
		text-align: left;
	}

	.registry-item:hover {
		background: color-mix(in srgb, var(--panel-strong) 70%, transparent);
	}

	.registry-item.selected {
		border-color: color-mix(in srgb, var(--accent) 45%, var(--border));
		background: color-mix(in srgb, var(--accent) 9%, var(--panel-strong));
	}

	.registry-item-existing {
		opacity: 0.65;
		cursor: default;
	}

	.registry-check {
		width: 16px;
		height: 16px;
		border: 1px solid var(--border);
		border-radius: 4px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		color: var(--muted);
		flex-shrink: 0;
	}

	.registry-check.checked {
		border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
		background: color-mix(in srgb, var(--accent) 24%, transparent);
		color: var(--accent);
	}

	.registry-info {
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.registry-name-row {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}

	.registry-name {
		font-weight: 600;
		font-size: var(--text-sm);
	}

	.registry-url {
		color: var(--muted);
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.source-badge {
		display: inline-flex;
		align-items: center;
		border-radius: 6px;
		padding: 1px 6px;
		font-size: 10px;
		font-weight: 600;
	}

	.source-badge-catalog {
		color: var(--muted);
		background: color-mix(in srgb, var(--panel-strong) 80%, transparent);
		border: 1px solid color-mix(in srgb, var(--border) 82%, transparent);
	}

	.source-badge-github {
		color: var(--muted);
		background: color-mix(in srgb, var(--panel-strong) 80%, transparent);
		border: 1px solid color-mix(in srgb, var(--border) 82%, transparent);
	}

	.source-badge-in-workset {
		color: var(--muted);
		background: color-mix(in srgb, var(--panel-strong) 80%, transparent);
		border: 1px solid color-mix(in srgb, var(--border) 82%, transparent);
	}

	.github-result-check {
		border-style: dashed;
	}

	.registry-empty {
		padding: 14px 10px;
		text-align: center;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	@media (max-width: 1120px) {
		.add-flow-shell {
			grid-template-columns: minmax(0, 1fr);
			min-height: 0;
		}

		.column-right {
			position: static;
		}
	}
</style>
