<script lang="ts">
	import { onDestroy } from 'svelte';
	import { ArrowRight, Check, Loader2, Search } from '@lucide/svelte';
	import type { Alias } from '../../types';
	import type { GitHubRepoSearchItem } from '../../types';
	import type { WorkspaceActionDirectRepo } from '../../services/workspaceActionContextService';
	import { createRepoSearch } from '../../composables/createRepoSearch.svelte';
	import { deriveRepoName } from '../../names';

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
		description?: string;
		nameValidationError?: string | null;
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
		onDescriptionInput?: (value: string) => void;
		onSearchQueryInput: (value: string) => void;
		onSourceInput: (value: string) => void;
		onAddDirectRepo: () => void;
		onRemoveDirectRepo: (url: string) => void;
		onToggleDirectRepoRegister: (url: string) => void;
		onToggleAlias: (name: string) => void;
		onSubmit: () => void;
	}

	/* eslint-disable @typescript-eslint/no-unused-vars -- onToggleDirectRepoRegister kept for interface compat */
	const {
		loading,
		modeVariant = 'workset',
		worksetLabel = null,
		workspaceName,
		description = '',
		nameValidationError = null,
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
		onDescriptionInput,
		onSearchQueryInput,
		onSourceInput,
		onAddDirectRepo,
		onRemoveDirectRepo,
		onToggleDirectRepoRegister,
		onToggleAlias,
		onSubmit,
	}: Props = $props();
	/* eslint-enable @typescript-eslint/no-unused-vars */

	const repoSearch = createRepoSearch();
	let sourceInputDraft = $state('');

	const selectedCount = $derived(selectedAliases.size);
	const isThreadMode = $derived(modeVariant === 'thread');
	const nameLabel = $derived(modeVariant === 'thread' ? 'Thread Name' : 'Workset Name');
	const namePlaceholder = $derived(modeVariant === 'thread' ? 'oauth2-migration' : 'platform-core');
	const canSubmit = $derived(workspaceName.trim().length > 0 && !nameValidationError);
	const displayedSourceInput = $derived(repoSearch.focused ? sourceInputDraft : sourceInput);
	const sourceQuery = $derived(displayedSourceInput.trim());
	const canAddSource = $derived(sourceQuery.length > 0);
	const totalRepoCount = $derived(directRepos.length + selectedCount);
	const showSearchMeta = $derived(
		repoSearch.showSearchMinCharsHint ||
			repoSearch.loading ||
			repoSearch.error !== null ||
			repoSearch.showNoSearchResults,
	);

	const handleSourceInput = (value: string): void => {
		sourceInputDraft = value;
		onSourceInput(value);
		onSearchQueryInput(value);
		repoSearch.queue(value);
	};

	const commitSource = (value: string): void => {
		const trimmed = value.trim();
		if (trimmed.length === 0) return;
		onSourceInput(trimmed);
		onAddDirectRepo();
		sourceInputDraft = '';
		onSourceInput('');
		onSearchQueryInput('');
		repoSearch.reset();
	};

	const selectRemoteSuggestion = (suggestion: GitHubRepoSearchItem): void => {
		const source = suggestion.sshUrl || suggestion.cloneUrl;
		commitSource(source);
	};

	const handleAddClick = (): void => {
		commitSource(sourceInputDraft);
	};

	const handleSourceKeydown = (event: KeyboardEvent): void => {
		if (event.key === 'Enter' && canAddSource) {
			event.preventDefault();
			commitSource(sourceInputDraft);
			return;
		}
		repoSearch.handleKeydown(event, selectRemoteSuggestion);
	};

	const openRemoteSuggestions = (): void => {
		sourceInputDraft = sourceInput;
		repoSearch.handleFocus(sourceInputDraft);
	};

	onDestroy(() => {
		repoSearch.destroy();
	});
</script>

{#if isThreadMode}
	<div class="thread-panel">
		<div class="thread-panel-form">
			<label class="thread-name-field">
				<span class="thread-name-label">{nameLabel}</span>
				<input
					class="thread-name-input"
					value={workspaceName}
					oninput={(event) => onWorkspaceNameInput((event.currentTarget as HTMLInputElement).value)}
					placeholder={namePlaceholder}
					autocapitalize="off"
					autocorrect="off"
					spellcheck="false"
				/>
			</label>

			{#if threadHookRows.length > 0 || threadHooksLoading}
				<div class="thread-hooks-section">
					<div class="thread-hooks-header">
						<span class="thread-hooks-title">Hooks</span>
						<span class="thread-hooks-meta"
							>{selectedCount} repo{selectedCount === 1 ? '' : 's'}</span
						>
					</div>
					<div class="thread-hooks-list">
						{#if threadHooksLoading}
							<div class="thread-hooks-loading">
								<Loader2 size={12} />
								<span>Checking hooks…</span>
							</div>
						{:else}
							{#each threadHookRows as row (`${row.repoName}`)}
								<div class="thread-hooks-row">
									<div class="thread-hooks-repo">{row.repoName}</div>
									{#if !row.hasSource}
										<div class="thread-hooks-empty">No source in catalog</div>
									{:else if row.hooks.length === 0}
										<div class="thread-hooks-empty">No hooks</div>
									{:else}
										<div class="thread-hooks-chip-row">
											{#each row.hooks as hook (`${row.repoName}-${hook}`)}
												<span class="thread-hooks-chip">{hook}</span>
											{/each}
										</div>
									{/if}
								</div>
							{/each}
						{/if}
					</div>
					{#if threadHooksError}
						<div class="thread-hooks-error">{threadHooksError}</div>
					{/if}
				</div>
			{/if}
		</div>

		<div class="thread-panel-footer">
			{#if worksetLabel}
				<span class="thread-footer-workset">{worksetLabel}</span>
			{/if}
			<button
				type="button"
				class="thread-panel-submit"
				onclick={onSubmit}
				disabled={loading || !canSubmit}
				aria-busy={loading}
			>
				{loading ? 'Creating…' : 'Create Thread'}
			</button>
		</div>
	</div>
{:else}
	<div class="create-panel" aria-busy={loading}>
		<div class="create-panel-header">
			<label class="create-field">
				<span class="create-field-label">{nameLabel}</span>
				<input
					class="create-field-input"
					value={workspaceName}
					oninput={(event) => onWorkspaceNameInput((event.currentTarget as HTMLInputElement).value)}
					placeholder={namePlaceholder}
					autocapitalize="off"
					autocorrect="off"
					spellcheck="false"
				/>
				{#if nameValidationError}
					<p class="create-field-error">{nameValidationError}</p>
				{/if}
			</label>

			{#if onDescriptionInput}
				<label class="create-field">
					<span class="create-field-label">
						Description
						<span class="create-field-hint-inline">optional</span>
					</span>
					<textarea
						class="create-field-input create-description"
						value={description}
						oninput={(event) =>
							onDescriptionInput((event.currentTarget as HTMLTextAreaElement).value)}
						placeholder="What are you working on?"
						rows="2"
					></textarea>
				</label>
			{/if}

			<div class="create-source-section">
				<div class="repo-input-row">
					<div class="repo-input-shell">
						<span class="repo-input-icon"><Search size={14} /></span>
						<input
							value={displayedSourceInput}
							oninput={(event) =>
								handleSourceInput((event.currentTarget as HTMLInputElement).value)}
							onfocus={openRemoteSuggestions}
							onblur={repoSearch.handleBlur}
							onkeydown={handleSourceKeydown}
							placeholder="Search catalog, GitHub, or paste URL"
							aria-label="Add repository URL or search GitHub"
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
					</div>
					<button
						type="button"
						class="create-add-btn"
						onclick={handleAddClick}
						disabled={!canAddSource}
						aria-label="Add repository">Add</button
					>
				</div>
				{#if showSearchMeta}
					<div class="repo-input-help-row">
						{#if repoSearch.showSearchMinCharsHint}
							<span class="repo-search-status">Type at least 2 characters to search GitHub.</span>
						{:else if repoSearch.loading}
							<span class="repo-search-status">
								<Loader2 size={12} />
								<span>Searching GitHub…</span>
							</span>
						{:else if repoSearch.error}
							<span class="repo-search-error">{repoSearch.error}</span>
						{:else if repoSearch.showNoSearchResults}
							<span class="repo-search-status">No GitHub results for "{sourceQuery}".</span>
						{/if}
					</div>
				{/if}
			</div>
		</div>

		{#if directRepos.length > 0}
			<div class="selected-repos-panel">
				<div class="selected-repos-header">
					<span>Added</span>
					<span class="selected-repos-count">{directRepos.length}</span>
				</div>
				<div class="selected-repos-list">
					{#each directRepos as repo (repo.url)}
						<button
							type="button"
							class="selected-repo-chip"
							onclick={() => onRemoveDirectRepo(repo.url)}
						>
							<span class="selected-repo-kind">Source</span>
							<span>{deriveRepoName(repo.url) || repo.url}</span>
							<span class="selected-repo-remove">×</span>
						</button>
					{/each}
				</div>
			</div>
		{/if}

		<div class="registry-list">
			{#if filteredAliases.length > 0}
				<div class="result-group-label">Catalog</div>
				{#each filteredAliases as alias (alias.name)}
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
								<span class="source-badge">Catalog</span>
							</div>
							<div class="registry-url">{getAliasSource(alias)}</div>
						</div>
					</button>
				{/each}
			{/if}

			{#if repoSearch.results.length > 0}
				<div class="result-group-label">GitHub</div>
				{#each repoSearch.results as suggestion (`${suggestion.owner}/${suggestion.name}`)}
					<button
						type="button"
						class="registry-item github-result"
						onmousedown={() => selectRemoteSuggestion(suggestion)}
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

			{#if filteredAliases.length === 0 && repoSearch.results.length === 0}
				<div class="registry-empty">
					{#if sourceQuery.length > 0}
						No matching repositories.
					{:else}
						Search or paste a URL to add repositories.
					{/if}
				</div>
			{/if}
		</div>

		<div class="create-panel-footer">
			<div class="create-panel-status">
				<span>{totalRepoCount} repo{totalRepoCount === 1 ? '' : 's'}</span>
			</div>
			<button
				type="button"
				class="create-panel-submit"
				onclick={onSubmit}
				disabled={loading || !canSubmit}
			>
				{loading ? 'Creating…' : 'Create'}
			</button>
		</div>
	</div>
{/if}

<style>
	/* ─── Thread panel layout ─── */
	.thread-panel {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
	}

	.thread-panel-form {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.thread-panel-form::-webkit-scrollbar {
		width: 5px;
	}

	.thread-panel-form::-webkit-scrollbar-track {
		background: transparent;
	}

	.thread-panel-form::-webkit-scrollbar-thumb {
		background: rgba(255, 255, 255, 0.1);
		border-radius: 3px;
	}

	.thread-name-field {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.thread-name-label {
		font-size: var(--text-xs);
		font-weight: 600;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.thread-name-input {
		width: 100%;
		height: 34px;
		box-sizing: border-box;
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 8px 10px;
		font-size: var(--text-sm);
		color: var(--text);
		font-family: inherit;
	}

	.thread-name-input:focus {
		outline: none;
		border-color: color-mix(in srgb, var(--accent) 48%, var(--border));
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 26%, transparent);
		background: rgba(255, 255, 255, 0.04);
	}

	.thread-hooks-section {
		display: flex;
		flex-direction: column;
		gap: 6px;
		flex: 1;
		min-height: 0;
	}

	.thread-hooks-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.thread-hooks-title {
		font-size: var(--text-xs);
		font-weight: 600;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.thread-hooks-meta {
		font-size: var(--text-xs);
		color: var(--subtle);
	}

	.thread-hooks-list {
		display: flex;
		flex-direction: column;
		gap: 1px;
		flex: 1;
		min-height: 0;
		overflow-y: auto;
	}

	.thread-hooks-list::-webkit-scrollbar {
		width: 5px;
	}

	.thread-hooks-list::-webkit-scrollbar-track {
		background: transparent;
	}

	.thread-hooks-list::-webkit-scrollbar-thumb {
		background: rgba(255, 255, 255, 0.1);
		border-radius: 3px;
	}

	.thread-hooks-loading {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 10px 0;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.thread-hooks-loading :global(svg) {
		animation: spin 900ms linear infinite;
	}

	.thread-hooks-row {
		display: flex;
		align-items: baseline;
		gap: 8px;
		padding: 8px 10px;
		border-radius: 6px;
		transition: background var(--transition-fast);
	}

	.thread-hooks-row:hover {
		background: var(--hover-bg);
	}

	.thread-hooks-repo {
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--text);
		flex-shrink: 0;
	}

	.thread-hooks-chip-row {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}

	.thread-hooks-chip {
		display: inline-flex;
		align-items: center;
		padding: 1px 7px;
		border-radius: 4px;
		background: color-mix(in srgb, var(--panel-strong) 80%, transparent);
		border: 1px solid color-mix(in srgb, var(--border) 65%, transparent);
		font-size: 11px;
		color: var(--muted);
		font-family: var(--font-mono);
	}

	.thread-hooks-empty {
		font-size: var(--text-xs);
		color: var(--subtle);
	}

	.thread-hooks-error {
		font-size: var(--text-xs);
		color: var(--danger-text);
		padding: 2px 0;
	}

	.thread-panel-footer {
		flex-shrink: 0;
		display: flex;
		align-items: center;
		gap: 10px;
		padding-top: 10px;
		border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
		margin-top: auto;
	}

	.thread-footer-workset {
		flex: 1;
		min-width: 0;
		font-size: var(--text-xs);
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.thread-panel-submit {
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

	.thread-panel-submit:hover:not(:disabled) {
		background: color-mix(in srgb, var(--cta) 88%, white);
	}

	.thread-panel-submit:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	/* ─── Create workset panel ─── */
	.create-panel {
		display: flex;
		flex-direction: column;
		gap: 0;
		flex: 1;
		min-height: 0;
	}

	.create-panel-header {
		flex-shrink: 0;
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding-bottom: 10px;
	}

	.create-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.create-field-label {
		font-size: var(--text-xs);
		font-weight: 600;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.create-field-hint-inline {
		font-weight: 400;
		text-transform: none;
		letter-spacing: normal;
		color: var(--subtle);
	}

	.create-field-input {
		width: 100%;
		height: 34px;
		box-sizing: border-box;
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 8px 10px;
		font-size: var(--text-sm);
		color: var(--text);
		font-family: inherit;
	}

	.create-field-input:focus {
		outline: none;
		border-color: color-mix(in srgb, var(--accent) 48%, var(--border));
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 26%, transparent);
		background: rgba(255, 255, 255, 0.04);
	}

	.create-description {
		height: auto;
		resize: vertical;
		min-height: 48px;
		line-height: 1.4;
	}

	.create-field-error {
		margin: 0;
		font-size: var(--text-xs);
		color: var(--danger-text, #f87171);
	}

	.create-source-section {
		display: flex;
		flex-direction: column;
		gap: 4px;
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

	.create-add-btn {
		height: var(--repo-control-height);
		padding: 0 12px;
		border: 1px solid color-mix(in srgb, var(--border) 82%, transparent);
		border-radius: var(--radius-md);
		background: color-mix(in srgb, var(--panel-strong) 72%, transparent);
		color: var(--muted);
		font-size: var(--text-sm);
		font-weight: 500;
		font-family: inherit;
		cursor: pointer;
		transition:
			color var(--transition-fast),
			border-color var(--transition-fast),
			background var(--transition-fast);
	}

	.create-add-btn:hover:not(:disabled) {
		color: var(--text);
		border-color: color-mix(in srgb, var(--accent) 36%, var(--border));
		background: color-mix(in srgb, var(--panel) 76%, transparent);
	}

	.create-add-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.repo-input-help-row {
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

	.repo-search-status :global(svg) {
		animation: spin 900ms linear infinite;
	}

	/* ─── Selected repos chips ─── */
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

	/* ─── Registry list ─── */
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
		background: var(--hover-bg);
	}

	.registry-item.selected {
		border-color: var(--active-accent-border);
		background: var(--active-accent-bg);
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

	.github-result-check {
		border-style: dashed;
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

	.registry-empty {
		padding: 20px 10px;
		text-align: center;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	/* ─── Footer ─── */
	.create-panel-footer {
		flex-shrink: 0;
		display: flex;
		align-items: center;
		gap: 10px;
		padding-top: 10px;
		border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
		margin-top: auto;
	}

	.create-panel-status {
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.create-panel-submit {
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

	.create-panel-submit:hover:not(:disabled) {
		background: color-mix(in srgb, var(--cta) 88%, white);
	}

	.create-panel-submit:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}
</style>
