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
	const canContinue = $derived(addRepoTotalItems > 0 || hasPendingSource);

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

<div class="add-panel">
	<div class="add-panel-search">
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
					placeholder="Search catalog, GitHub, or paste URL"
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
					<span class="repo-search-status">No GitHub results for "{sourceQuery}".</span>
				{/if}
			</div>
		{/if}
	</div>

	{#if addRepoSelectedItems.length > 0}
		<div class="selected-repos-panel">
			<div class="selected-repos-header">
				<span>Selected</span>
				<span class="selected-repos-count">{addRepoSelectedItems.length}</span>
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
						<span class="selected-repo-remove">×</span>
					</button>
				{/each}
			</div>
		</div>
	{/if}

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
					No repositories in Repo Catalog yet.
				{:else if searchQuery.trim().length > 0}
					No matching repositories.
				{:else}
					Search or browse to find repositories.
				{/if}
			</div>
		{/if}
	</div>

	<div class="add-panel-footer">
		<div class="add-panel-status">
			<span>{existingRepos.length} in workset</span>
			{#if addRepoTotalItems > 0}
				<span class="add-panel-queued">{addRepoTotalItems} to add</span>
			{/if}
		</div>
		<button
			type="button"
			class="add-panel-submit"
			onclick={handleContinue}
			disabled={loading || !canContinue}
		>
			{#if loading}
				Adding…
			{:else}
				Add{addRepoTotalItems > 0 ? ` ${addRepoTotalItems}` : ''} <ArrowRight size={14} />
			{/if}
		</button>
	</div>
</div>

<style>
	.add-panel {
		display: flex;
		flex-direction: column;
		gap: 0;
		flex: 1;
		min-height: 0;
	}

	.add-panel-search {
		flex-shrink: 0;
		padding-bottom: 10px;
	}

	.repo-input-row {
		--repo-control-height: 34px;
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: 6px;
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
		padding: 8px 10px 8px 30px;
		font-size: var(--text-sm);
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
		margin-top: 4px;
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 8px;
	}

	.repo-search-status,
	.repo-search-error {
		font-size: var(--text-xs);
		color: var(--muted);
		display: inline-flex;
		align-items: center;
		gap: 5px;
	}

	.repo-search-error {
		color: var(--danger-text);
	}

	.selected-repos-panel {
		flex-shrink: 0;
		padding: 8px 10px;
		margin-bottom: 8px;
		border: 1px solid color-mix(in srgb, var(--accent) 28%, var(--border));
		border-radius: var(--radius-md);
		background: color-mix(in srgb, var(--accent) 6%, var(--panel));
	}

	.selected-repos-header {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		color: var(--muted);
		margin-bottom: 6px;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		font-weight: 600;
	}

	.selected-repos-count {
		background: color-mix(in srgb, var(--accent) 22%, transparent);
		color: var(--accent);
		border-radius: 999px;
		padding: 0 6px;
		font-size: 10px;
		font-weight: 700;
		line-height: 18px;
	}

	.selected-repos-list {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}

	.selected-repo-chip {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 7px;
		border-radius: 999px;
		border: 1px solid color-mix(in srgb, var(--border) 80%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 68%, transparent);
		color: var(--text);
		font-size: var(--text-xs);
		cursor: pointer;
		transition: border-color var(--transition-fast);
	}

	.selected-repo-chip:hover {
		border-color: var(--danger);
	}

	.selected-repo-kind {
		font-size: 9px;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.selected-repo-remove {
		color: var(--muted);
		font-size: var(--text-xs);
	}

	.registry-list {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
		overflow-y: auto;
		padding: 2px 0;
	}

	.registry-list::-webkit-scrollbar {
		width: 5px;
	}

	.registry-list::-webkit-scrollbar-track {
		background: transparent;
	}

	.registry-list::-webkit-scrollbar-thumb {
		background: rgba(255, 255, 255, 0.1);
		border-radius: 3px;
	}

	.result-group-label {
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: color-mix(in srgb, var(--muted) 84%, transparent);
		padding: 6px 2px 2px;
	}

	.result-group-label:first-child {
		padding-top: 0;
	}

	.registry-item {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		border: 1px solid transparent;
		background: transparent;
		border-radius: 6px;
		padding: 6px 8px;
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
		opacity: 0.55;
		cursor: default;
	}

	.registry-check {
		width: 15px;
		height: 15px;
		border: 1px solid var(--border);
		border-radius: 3px;
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
		gap: 1px;
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
		font-size: 11px;
		font-family: var(--font-mono);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.source-badge {
		display: inline-flex;
		align-items: center;
		border-radius: 4px;
		padding: 0 5px;
		font-size: 9px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--muted);
		background: color-mix(in srgb, var(--panel-strong) 80%, transparent);
		border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
	}

	.github-result-check {
		border-style: dashed;
	}

	.registry-empty {
		padding: 20px 10px;
		text-align: center;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.add-panel-footer {
		flex-shrink: 0;
		display: flex;
		align-items: center;
		gap: 10px;
		padding-top: 10px;
		border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
		margin-top: auto;
	}

	.add-panel-status {
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.add-panel-queued {
		color: var(--accent);
		font-weight: 600;
	}

	.add-panel-submit {
		flex-shrink: 0;
		padding: 7px 16px;
		border: none;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		background: var(--cta);
		color: var(--on-dark);
		transition:
			background var(--transition-fast),
			opacity var(--transition-fast);
	}

	.add-panel-submit:hover:not(:disabled) {
		background: color-mix(in srgb, var(--cta) 88%, white);
	}

	.add-panel-submit:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}
</style>
