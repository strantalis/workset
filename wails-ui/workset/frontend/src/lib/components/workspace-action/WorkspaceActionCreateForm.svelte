<script lang="ts">
	import { onDestroy } from 'svelte';
	import { Loader2 } from '@lucide/svelte';
	import { searchGitHubRepositories } from '../../api/github';
	import type { Alias } from '../../types';
	import type { GitHubRepoSearchItem } from '../../types';
	import type { WorkspaceActionDirectRepo } from '../../services/workspaceActionContextService';
	import { looksLikeUrl } from '../../names';
	import Button from '../ui/Button.svelte';

	type ThreadHookPreviewRow = {
		repoName: string;
		hooks: string[];
		hasSource: boolean;
	};

	interface Props {
		loading: boolean;
		modeVariant?: 'workset' | 'thread';
		worksetLabel?: string | null;
		workspaceName: string;
		searchQuery: string;
		sourceInput: string;
		directRepos: WorkspaceActionDirectRepo[];
		threadHookRows?: ThreadHookPreviewRow[];
		threadHooksLoading?: boolean;
		threadHooksError?: string | null;
		filteredAliases: Alias[];
		selectedAliases: Set<string>;
		getAliasSource: (alias: Alias) => string;
		onWorkspaceNameInput: (value: string) => void;
		onSearchQueryInput: (value: string) => void;
		onSourceInput: (value: string) => void;
		onAddDirectRepo: () => void;
		onRemoveDirectRepo: (url: string) => void;
		onToggleDirectRepoRegister: (url: string) => void;
		onToggleAlias: (name: string) => void;
		onSubmit: () => void;
	}

	const {
		loading,
		modeVariant = 'workset',
		worksetLabel = null,
		workspaceName,
		searchQuery,
		sourceInput,
		directRepos,
		threadHookRows = [],
		threadHooksLoading = false,
		threadHooksError = null,
		filteredAliases,
		selectedAliases,
		getAliasSource,
		onWorkspaceNameInput,
		onSearchQueryInput,
		onSourceInput,
		onAddDirectRepo,
		onRemoveDirectRepo,
		onToggleDirectRepoRegister,
		onToggleAlias,
		onSubmit,
	}: Props = $props();

	let remoteSuggestions: GitHubRepoSearchItem[] = $state([]);
	let searchLoading = $state(false);
	let searchError: string | null = $state(null);
	let suggestionsOpen = $state(false);
	let sourceInputFocused = $state(false);
	let activeSuggestionIndex = $state(-1);
	let lastSearchedQuery = $state('');
	let sourceInputDraft = $state('');
	let sourceSearchDebounce: ReturnType<typeof setTimeout> | null = null;
	let sourceSearchCloseTimer: ReturnType<typeof setTimeout> | null = null;
	let sourceSearchSequence = 0;

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

	const selectedCount = $derived(selectedAliases.size);
	const isThreadMode = $derived(modeVariant === 'thread');
	const createActionLabel = $derived(modeVariant === 'thread' ? 'Create Thread' : 'Create Workset');
	const nameLabel = $derived(modeVariant === 'thread' ? 'Thread Name' : 'Workset Name');
	const namePlaceholder = $derived(modeVariant === 'thread' ? 'oauth2-migration' : 'platform-core');
	const canSubmit = $derived(
		modeVariant === 'thread'
			? workspaceName.trim().length > 0
			: selectedCount > 0 || directRepos.length > 0,
	);
	const displayedSourceInput = $derived(sourceInputFocused ? sourceInputDraft : sourceInput);
	const sourceQuery = $derived(displayedSourceInput.trim());
	const canAddSource = $derived(sourceQuery.length > 0);
	const showRemoteSuggestionPanel = $derived(suggestionsOpen && sourceInputFocused);
	const showSearchStartHint = $derived(sourceQuery.length === 0);
	const showSearchMinCharsHint = $derived(
		sourceQuery.length > 0 &&
			sourceQuery.length < 2 &&
			!looksLikeUrl(sourceQuery) &&
			!isLikelyLocalPath(sourceQuery),
	);
	const showNoSearchResults = $derived(
		!searchLoading &&
			searchError === null &&
			!showSearchStartHint &&
			!showSearchMinCharsHint &&
			remoteSuggestions.length === 0 &&
			lastSearchedQuery !== '' &&
			sourceQuery === lastSearchedQuery,
	);

	const shouldSearchRemote = (value: string): boolean => {
		const trimmed = value.trim();
		return trimmed.length >= 2 && !looksLikeUrl(trimmed) && !isLikelyLocalPath(trimmed);
	};

	const clearSourceTimers = (): void => {
		if (sourceSearchDebounce) {
			clearTimeout(sourceSearchDebounce);
			sourceSearchDebounce = null;
		}
		if (sourceSearchCloseTimer) {
			clearTimeout(sourceSearchCloseTimer);
			sourceSearchCloseTimer = null;
		}
	};

	const resetRemoteSuggestions = (): void => {
		clearSourceTimers();
		sourceSearchSequence += 1;
		remoteSuggestions = [];
		searchLoading = false;
		searchError = null;
		suggestionsOpen = false;
		activeSuggestionIndex = -1;
		lastSearchedQuery = '';
	};

	const showRemoteSearchHints = (query: string): void => {
		sourceSearchSequence += 1;
		remoteSuggestions = [];
		searchLoading = false;
		searchError = null;
		suggestionsOpen = sourceInputFocused;
		activeSuggestionIndex = -1;
		lastSearchedQuery = query;
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
		suggestionsOpen = sourceInputFocused;
		lastSearchedQuery = query;
		try {
			const results = await searchGitHubRepositories(query, 8);
			if (requestSequence !== sourceSearchSequence) return;
			remoteSuggestions = results;
			activeSuggestionIndex = results.length > 0 ? 0 : -1;
		} catch (err) {
			if (requestSequence !== sourceSearchSequence) return;
			remoteSuggestions = [];
			activeSuggestionIndex = -1;
			searchError = toSearchErrorMessage(err);
		} finally {
			if (requestSequence === sourceSearchSequence) {
				searchLoading = false;
			}
		}
	};

	const queueRemoteSearch = (value: string): void => {
		const query = value.trim();
		if (sourceSearchDebounce) {
			clearTimeout(sourceSearchDebounce);
			sourceSearchDebounce = null;
		}
		if (query.length === 0) {
			showRemoteSearchHints('');
			return;
		}
		if (!shouldSearchRemote(query)) {
			if (looksLikeUrl(query) || isLikelyLocalPath(query)) {
				resetRemoteSuggestions();
				return;
			}
			showRemoteSearchHints(query);
			return;
		}
		sourceSearchDebounce = setTimeout(() => {
			void runRemoteSearch(query);
		}, 250);
	};

	const handleSourceInput = (value: string): void => {
		sourceInputDraft = value;
		onSourceInput(value);
		queueRemoteSearch(value);
	};

	const selectRemoteSuggestion = (suggestion: GitHubRepoSearchItem): void => {
		const value = suggestion.sshUrl || suggestion.cloneUrl;
		sourceInputDraft = value;
		onSourceInput(value);
		resetRemoteSuggestions();
	};

	const handleSourceKeydown = (event: KeyboardEvent): void => {
		if (!showRemoteSuggestionPanel || searchLoading || searchError) {
			if (event.key === 'Escape') resetRemoteSuggestions();
			return;
		}
		if (remoteSuggestions.length === 0) return;
		if (event.key === 'ArrowDown') {
			event.preventDefault();
			activeSuggestionIndex = (activeSuggestionIndex + 1) % remoteSuggestions.length;
			return;
		}
		if (event.key === 'ArrowUp') {
			event.preventDefault();
			activeSuggestionIndex =
				(activeSuggestionIndex - 1 + remoteSuggestions.length) % remoteSuggestions.length;
			return;
		}
		if (event.key === 'Enter' && activeSuggestionIndex >= 0) {
			event.preventDefault();
			selectRemoteSuggestion(remoteSuggestions[activeSuggestionIndex]);
			return;
		}
		if (event.key === 'Escape') {
			event.preventDefault();
			resetRemoteSuggestions();
		}
	};

	const handleSourceBlur = (): void => {
		sourceInputFocused = false;
		if (sourceSearchCloseTimer) {
			clearTimeout(sourceSearchCloseTimer);
		}
		sourceSearchCloseTimer = setTimeout(() => {
			suggestionsOpen = false;
		}, 120);
	};

	const openRemoteSuggestions = (): void => {
		sourceInputFocused = true;
		sourceInputDraft = sourceInput;
		if (sourceSearchCloseTimer) {
			clearTimeout(sourceSearchCloseTimer);
			sourceSearchCloseTimer = null;
		}
		queueRemoteSearch(sourceInputDraft);
	};

	onDestroy(() => {
		clearSourceTimers();
	});
</script>

<div class="form ws-form-stack" class:thread-mode={isThreadMode}>
	{#if modeVariant === 'thread' && worksetLabel}
		<div class="context-chip">
			<span class="context-chip-label">Workset</span>
			<strong>{worksetLabel}</strong>
		</div>
	{/if}

	<label class="field ws-field">
		<span class="field-label">{nameLabel}</span>
		<input
			class="ws-field-input"
			value={workspaceName}
			oninput={(event) => onWorkspaceNameInput((event.currentTarget as HTMLInputElement).value)}
			placeholder={namePlaceholder}
			autocapitalize="off"
			autocorrect="off"
			spellcheck="false"
		/>
	</label>

	{#if isThreadMode}
		<div class="hint ws-hint thread-scope-note">
			Repositories are inherited from this workset ({selectedCount} total).
		</div>
		<div class="field ws-field thread-hooks-section">
			<div class="field-title">
				<span>Post-Checkout Hooks</span>
				<span class="count">per repo · optional</span>
			</div>
			<div class="thread-hooks-list">
				{#if threadHooksLoading}
					<div class="suggestion-loading">
						<Loader2 size={14} />
						<span>Checking lifecycle hooks…</span>
					</div>
				{:else if threadHookRows.length === 0}
					<div class="thread-hooks-empty">No repositories found for this workset.</div>
				{:else}
					{#each threadHookRows as row (`${row.repoName}`)}
						<div class="thread-hooks-row">
							<div class="thread-hooks-repo">{row.repoName}</div>
							{#if !row.hasSource}
								<div class="thread-hooks-empty">Repo source unavailable in catalog.</div>
							{:else if row.hooks.length === 0}
								<div class="thread-hooks-empty">No hooks discovered.</div>
							{:else}
								<div class="thread-hooks-chip-row">
									{#each row.hooks as hook (`${row.repoName}-${hook}`)}
										<span class="thread-hooks-chip">+ {hook}</span>
									{/each}
								</div>
							{/if}
						</div>
					{/each}
				{/if}
			</div>
			{#if threadHooksError}
				<div class="hint ws-hint">{threadHooksError}</div>
			{/if}
		</div>
	{:else}
		<div class="field ws-field">
			<div class="field-title">
				<span>Add Repository</span>
				<span class="count">{directRepos.length} added</span>
			</div>
			<div class="inline ws-inline">
				<div class="source-input-shell">
					<input
						value={displayedSourceInput}
						oninput={(event) => handleSourceInput((event.currentTarget as HTMLInputElement).value)}
						onfocus={openRemoteSuggestions}
						onblur={handleSourceBlur}
						onkeydown={handleSourceKeydown}
						placeholder="git@github.com:org/repo.git or search GitHub"
						class="search-input"
						autocapitalize="off"
						autocorrect="off"
						spellcheck="false"
					/>
					{#if showRemoteSuggestionPanel}
						<div class="repo-suggestions" role="listbox" aria-label="GitHub repository suggestions">
							{#if showSearchStartHint}
								<div class="suggestion-hint">Start typing to search GitHub repositories.</div>
							{:else if showSearchMinCharsHint}
								<div class="suggestion-hint">Type at least 2 characters to search GitHub.</div>
							{:else if searchLoading}
								<div class="suggestion-loading">
									<Loader2 size={14} />
									<span>Searching GitHub…</span>
								</div>
							{:else if searchError}
								<div class="suggestion-error">{searchError}</div>
							{:else if showNoSearchResults}
								<div class="suggestion-hint">No repositories found for "{sourceQuery}".</div>
							{:else}
								{#each remoteSuggestions as suggestion, index (suggestion.fullName)}
									<button
										type="button"
										role="option"
										class="suggestion-item"
										class:active={index === activeSuggestionIndex}
										aria-selected={index === activeSuggestionIndex}
										onmousedown={() => selectRemoteSuggestion(suggestion)}
									>
										<div class="suggestion-main">
											<span class="suggestion-name">{suggestion.fullName}</span>
											<span class="suggestion-meta">{suggestion.defaultBranch}</span>
										</div>
										<span class="suggestion-url">{suggestion.sshUrl || suggestion.cloneUrl}</span>
									</button>
								{/each}
							{/if}
						</div>
					{/if}
				</div>
				<button
					type="button"
					class="add-repo-btn"
					onclick={onAddDirectRepo}
					disabled={!canAddSource}>Add</button
				>
			</div>
			{#if directRepos.length > 0}
				<div class="direct-repo-list">
					{#each directRepos as repo (repo.url)}
						<div class="direct-repo-item">
							<div class="direct-repo-main">
								<span class="direct-repo-url">{repo.url}</span>
								<label class="register-toggle">
									<input
										type="checkbox"
										checked={repo.register}
										onchange={() => onToggleDirectRepoRegister(repo.url)}
									/>
									<span>Save to catalog</span>
								</label>
							</div>
							<button
								type="button"
								class="remove-repo-btn"
								onclick={() => onRemoveDirectRepo(repo.url)}>Remove</button
							>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<div class="field ws-field">
			<div class="field-title">
				<span>Repo Catalog</span>
				<span class="count">{selectedCount} selected</span>
			</div>

			<div class="inline ws-inline">
				<input
					value={searchQuery}
					oninput={(event) => onSearchQueryInput((event.currentTarget as HTMLInputElement).value)}
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
	{/if}

	<Button
		variant="primary"
		onclick={onSubmit}
		disabled={loading || !canSubmit}
		class={`action-btn${isThreadMode ? ' thread-submit' : ''}`}
	>
		{loading ? 'Creating…' : createActionLabel}
	</Button>
</div>

<style>
	.form.thread-mode {
		gap: 14px;
	}

	.context-chip {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		border-radius: 10px;
		background: linear-gradient(
			120deg,
			color-mix(in srgb, var(--accent) 18%, transparent),
			color-mix(in srgb, var(--panel-strong) 84%, transparent)
		);
		border: 1px solid color-mix(in srgb, var(--accent) 26%, var(--border));
		color: var(--muted);
		font-size: var(--text-xs);
		font-weight: 600;
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
	}

	.context-chip-label {
		font-size: 10px;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: color-mix(in srgb, var(--muted) 82%, transparent);
	}

	.context-chip strong {
		color: var(--text);
		font-weight: 600;
	}

	.field-label {
		color: color-mix(in srgb, var(--text) 88%, transparent);
		font-size: var(--text-sm);
		font-weight: 600;
	}

	.thread-mode .field-label {
		color: color-mix(in srgb, var(--text) 92%, transparent);
	}

	.field-title {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		font-size: var(--text-sm);
		font-weight: 600;
	}

	.count {
		font-size: var(--text-xs);
		color: var(--muted);
		font-weight: 500;
		letter-spacing: 0.02em;
	}

	.thread-mode :global(.ws-field-input) {
		background: color-mix(in srgb, var(--panel) 70%, transparent);
		border-color: color-mix(in srgb, var(--border) 92%, transparent);
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
	}

	.thread-mode :global(.ws-field-input:focus) {
		border-color: color-mix(in srgb, var(--accent) 55%, var(--border));
		background: color-mix(in srgb, var(--panel) 76%, transparent);
	}

	.thread-scope-note {
		margin-top: -2px;
		padding: 0 2px;
		font-size: var(--text-sm);
		color: color-mix(in srgb, var(--muted) 88%, transparent);
	}

	.thread-hooks-section {
		padding: 12px;
		border-radius: 12px;
		border: 1px solid color-mix(in srgb, var(--border) 82%, transparent);
		background: color-mix(in srgb, var(--panel-soft) 56%, transparent);
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
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

	.source-input-shell {
		position: relative;
		flex: 1;
	}

	.repo-suggestions {
		position: absolute;
		top: calc(100% + 8px);
		left: 0;
		right: 0;
		background: color-mix(in srgb, var(--panel) 92%, transparent);
		border: 1px solid color-mix(in srgb, var(--border) 85%, transparent);
		border-radius: var(--radius-md);
		box-shadow: 0 16px 36px rgba(5, 10, 22, 0.5);
		max-height: 260px;
		overflow-y: auto;
		z-index: 12;
	}

	.suggestion-hint,
	.suggestion-error,
	.suggestion-loading {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 12px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.suggestion-error {
		color: color-mix(in srgb, var(--danger) 74%, #fff);
	}

	.suggestion-item {
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 3px;
		padding: 10px 12px;
		text-align: left;
		background: transparent;
		border: none;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 55%, transparent);
		color: var(--text);
		cursor: pointer;
	}

	.suggestion-item:last-child {
		border-bottom: none;
	}

	.suggestion-item:hover,
	.suggestion-item.active {
		background: color-mix(in srgb, var(--accent) 14%, transparent);
	}

	.suggestion-main {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
	}

	.suggestion-name {
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--text);
	}

	.suggestion-meta {
		font-size: var(--text-xs);
		color: var(--muted);
		white-space: nowrap;
	}

	.suggestion-url {
		font-size: var(--text-xs);
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.suggestion-loading :global(svg) {
		animation: spin 900ms linear infinite;
	}

	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
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

	.thread-hooks-list {
		display: flex;
		flex-direction: column;
		max-height: 220px;
		overflow-y: auto;
		border-radius: var(--radius-md);
		border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
		background: color-mix(in srgb, var(--panel) 68%, transparent);
		box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
	}

	.thread-hooks-row {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 11px 12px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 45%, transparent);
		transition: background var(--transition-fast);
	}

	.thread-hooks-row:hover {
		background: color-mix(in srgb, var(--accent) 6%, transparent);
	}

	.thread-hooks-row:last-child {
		border-bottom: none;
	}

	.thread-hooks-repo {
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--text);
	}

	.thread-hooks-chip-row {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.thread-hooks-chip {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 4px 9px;
		border-radius: 999px;
		border: 1px solid color-mix(in srgb, var(--accent) 18%, var(--border));
		background: color-mix(in srgb, var(--accent) 12%, var(--panel-strong));
		font-size: var(--text-xs);
		color: color-mix(in srgb, var(--text) 86%, transparent);
		font-weight: 500;
	}

	.thread-hooks-empty {
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.add-repo-btn {
		background: var(--panel-strong);
		color: var(--text);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 8px 10px;
		font-size: var(--text-sm);
		font-weight: 500;
		cursor: pointer;
	}

	.add-repo-btn:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.add-repo-btn:hover {
		border-color: color-mix(in srgb, var(--accent) 50%, var(--border));
	}

	.direct-repo-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin-top: 10px;
	}

	.direct-repo-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 10px;
		border-radius: var(--radius-md);
		background: color-mix(in srgb, var(--panel-soft) 70%, transparent);
		border: 1px solid var(--border);
	}

	.direct-repo-main {
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}

	.direct-repo-url {
		font-size: var(--text-sm);
		color: var(--text);
		word-break: break-all;
	}

	.register-toggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.remove-repo-btn {
		background: transparent;
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 6px 8px;
		font-size: var(--text-xs);
		cursor: pointer;
	}

	.remove-repo-btn:hover {
		color: var(--danger);
		border-color: color-mix(in srgb, var(--danger) 40%, var(--border));
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
		max-height: 260px;
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
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--panel-soft);
		position: relative;
		flex-shrink: 0;
		cursor: pointer;
	}

	.checkbox-item input[type='checkbox']:checked {
		background: var(--accent);
		border-color: var(--accent);
	}

	.checkbox-item input[type='checkbox']:checked::after {
		content: '✓';
		position: absolute;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		color: white;
		font-size: 11px;
		font-weight: 700;
	}

	.checkbox-content {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
		flex: 1;
	}

	.checkbox-name {
		font-size: var(--text-base);
		color: var(--text);
		font-weight: 500;
	}

	.checkbox-meta {
		font-size: var(--text-sm);
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	:global(.action-btn) {
		width: 100%;
		margin-top: 8px;
	}

	:global(.thread-submit.btn.primary) {
		margin-top: 10px;
		min-height: 42px;
		font-size: var(--text-base);
		font-weight: 700;
		letter-spacing: 0.01em;
		box-shadow:
			0 10px 24px rgba(var(--accent-rgb), 0.26),
			inset 0 1px 0 rgba(255, 255, 255, 0.18);
	}

	:global(.thread-submit.btn.primary:hover:not(:disabled)) {
		transform: translateY(-1px);
		box-shadow:
			0 14px 28px rgba(var(--accent-rgb), 0.32),
			inset 0 1px 0 rgba(255, 255, 255, 0.2);
	}
</style>
