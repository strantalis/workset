<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import {
		Plus,
		Search,
		ChevronDown,
		Loader2,
		Copy,
		Trash2,
		FolderOpen,
		Pencil,
	} from '@lucide/svelte';
	import {
		createAlias,
		deleteAlias,
		listAliases,
		openDirectoryDialog,
		updateAlias,
	} from '../../../api/settings';
	import { searchGitHubRepositories } from '../../../api/github';
	import type { Alias, GitHubRepoSearchItem } from '../../../types';
	import { toErrorMessage } from '../../../errors';
	import SettingsSection from '../SettingsSection.svelte';
	import Button from '../../ui/Button.svelte';
	import { deriveRepoName, looksLikeUrl } from '../../../names';

	interface Props {
		onAliasCountChange: (count: number) => void;
	}

	const { onAliasCountChange }: Props = $props();

	let aliases: Alias[] = $state([]);
	let loading = $state(false);
	let error: string | null = $state(null);
	let success: string | null = $state(null);

	// Registration/edit form state
	let isRegistering = $state(false);
	let isEditing = $state(false);
	let editingName = $state('');
	let formName = $state('');
	let formSource = $state('');
	let formRemote = $state('origin');
	let formBranch = $state('main');
	let advancedOpen = $state(false);
	let detecting = $state(false);
	let remoteSuggestions: GitHubRepoSearchItem[] = $state([]);
	let searchLoading = $state(false);
	let searchError: string | null = $state(null);
	let suggestionsOpen = $state(false);
	let sourceInputFocused = $state(false);
	let activeSuggestionIndex = $state(-1);
	let lastSearchedQuery = $state('');
	let sourceSearchDebounce: ReturnType<typeof setTimeout> | null = null;
	let sourceSearchCloseTimer: ReturnType<typeof setTimeout> | null = null;
	let sourceSearchSequence = 0;

	// Search and copy state
	let searchQuery = $state('');
	let copiedId: string | null = $state(null);

	const loadAliases = async (): Promise<void> => {
		try {
			aliases = await listAliases();
			onAliasCountChange(aliases.length);
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
		}
	};

	const filteredAliases = $derived(
		aliases.filter((a) => a.name.toLowerCase().includes(searchQuery.toLowerCase())),
	);
	const showRemoteSuggestionPanel = $derived(isRegistering && !isEditing && suggestionsOpen);

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
	const sourceQuery = $derived(formSource.trim());
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

	const toSearchErrorMessage = (err: unknown): string => {
		const message = toErrorMessage(err, 'Failed to search remote repositories.');
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
		if (!isRegistering || isEditing) {
			resetRemoteSuggestions();
			return;
		}
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
		formSource = value;
		queueRemoteSearch(value);
	};

	const selectRemoteSuggestion = (suggestion: GitHubRepoSearchItem): void => {
		formSource = suggestion.sshUrl || suggestion.cloneUrl;
		formName = suggestion.name;
		formBranch = suggestion.defaultBranch.trim() || 'main';
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
		if (!isRegistering || isEditing) return;
		sourceInputFocused = true;
		if (sourceSearchCloseTimer) {
			clearTimeout(sourceSearchCloseTimer);
			sourceSearchCloseTimer = null;
		}
		queueRemoteSearch(formSource);
	};

	$effect((): void => {
		if (isEditing || !isRegistering) {
			return;
		}

		if (formName.trim()) {
			return;
		}

		const source = formSource.trim();
		if (!source || !looksLikeUrl(source)) {
			return;
		}

		const derivedName = deriveRepoName(source);
		if (derivedName) {
			formName = derivedName;
		}
	});

	const handleCancel = (): void => {
		isRegistering = false;
		isEditing = false;
		editingName = '';
		formName = '';
		formSource = '';
		formRemote = 'origin';
		formBranch = 'main';
		advancedOpen = false;
		resetRemoteSuggestions();
		error = null;
	};

	const startEdit = (alias: Alias): void => {
		isEditing = true;
		isRegistering = true; // Show the form
		editingName = alias.name;
		formName = alias.name;
		formSource = alias.url ?? alias.path ?? '';
		formRemote = alias.remote ?? 'origin';
		formBranch = alias.default_branch ?? 'main';
		advancedOpen = false;
		resetRemoteSuggestions();
		error = null;
		success = null;
	};

	const handleSave = async (): Promise<void> => {
		const name = formName.trim();
		const source = formSource.trim();
		const remote = formRemote.trim();
		const branch = formBranch.trim();

		if (!name) {
			error = 'Repo name is required.';
			return;
		}
		if (!source) {
			error = 'Source URL or path is required.';
			return;
		}

		loading = true;
		error = null;
		success = null;
		detecting = true;

		try {
			if (isEditing) {
				await updateAlias(name, source, remote, branch);
				success = `Updated ${name}.`;
			} else {
				await createAlias(name, source, remote, branch);
				success = `Registered ${name}.`;
			}
			await loadAliases();
			// Reset form
			isRegistering = false;
			isEditing = false;
			editingName = '';
			formName = '';
			formSource = '';
			formRemote = 'origin';
			formBranch = 'main';
			advancedOpen = false;
			resetRemoteSuggestions();
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
		} finally {
			loading = false;
			detecting = false;
		}
	};

	const handleDelete = async (alias: Alias): Promise<void> => {
		const confirmed = window.confirm(`Remove "${alias.name}" from the catalog?`);
		if (!confirmed) return;

		loading = true;
		error = null;
		success = null;

		try {
			await deleteAlias(alias.name);
			success = `Removed ${alias.name}.`;
			await loadAliases();
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
		} finally {
			loading = false;
		}
	};

	const handleCopyUrl = async (alias: Alias): Promise<void> => {
		const url = alias.url ?? alias.path ?? '';
		try {
			await navigator.clipboard.writeText(url);
			copiedId = alias.name;
			setTimeout(() => {
				copiedId = null;
			}, 1500);
		} catch {
			// Ignore clipboard errors
		}
	};

	const handleBrowseSource = async (): Promise<void> => {
		try {
			const defaultDirectory = formSource.trim();
			const path = await openDirectoryDialog('Select repository directory', defaultDirectory);
			if (!path) return;
			formSource = path;
			resetRemoteSuggestions();
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
		}
	};

	onMount(() => {
		void loadAliases();
	});

	onDestroy(() => {
		clearSourceTimers();
	});
</script>

<SettingsSection
	title="Repo Catalog"
	description="Your catalog of known repositories. Repos registered here can be quickly added to worksets and templates by name."
>
	<div class="catalog-container">
		<!-- Header with Register button -->
		<div class="catalog-header">
			<div class="header-text">
				<h3 class="section-title ws-section-title">Registered Repositories</h3>
				<p class="section-desc">
					{aliases.length}
					{aliases.length === 1 ? 'repo' : 'repos'} in catalog
				</p>
			</div>
			{#if !isRegistering}
				<Button variant="primary" size="sm" onclick={() => (isRegistering = true)}>
					<Plus size={14} />
					Register Repo
				</Button>
			{/if}
		</div>

		<!-- Success/Error Messages -->
		{#if success && !isRegistering}
			<div class="message success ws-message">{success}</div>
		{/if}
		{#if error && !isRegistering}
			<div class="message error ws-message ws-message-error">{error}</div>
		{/if}

		<!-- Registration Form -->
		{#if isRegistering}
			<div class="registration-form">
				<div class="form-header">
					{#if isEditing}
						<h4>Editing: {editingName}</h4>
					{:else}
						<h4>New Repo</h4>
					{/if}
				</div>

				<div class="form-fields">
					<div class="form-field">
						<label for="reg-name">Name</label>
						<input
							id="reg-name"
							type="text"
							bind:value={formName}
							placeholder="my-repo"
							disabled={isEditing}
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
						{#if isEditing}
							<p class="field-hint">Repo name cannot be changed</p>
						{/if}
					</div>

					<div class="form-field">
						<label for="reg-source">Source (URL/path or GitHub repo search)</label>
						<div class="input-with-button">
							<div class="source-input-wrap">
								<input
									id="reg-source"
									type="text"
									bind:value={formSource}
									placeholder="Search GitHub repos or paste URL/path"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
									oninput={(event) =>
										handleSourceInput((event.currentTarget as HTMLInputElement).value)}
									onkeydown={handleSourceKeydown}
									onfocus={openRemoteSuggestions}
									onblur={handleSourceBlur}
								/>
								<span class="source-affordance" aria-hidden="true">
									{#if searchLoading}
										<span class="spin-icon"><Loader2 size={14} /></span>
									{:else}
										<Search size={14} />
									{/if}
								</span>
								{#if showRemoteSuggestionPanel}
									<div class="repo-suggestions">
										{#if searchLoading}
											<div class="repo-suggestion-state">
												<span class="spin-icon"><Loader2 size={14} /></span>
												<span>Searching remote repositories…</span>
											</div>
										{:else if searchError}
											<div class="repo-suggestion-state repo-suggestion-error">{searchError}</div>
										{:else if showSearchStartHint}
											<div class="repo-suggestion-state">
												Start typing to search GitHub repositories.
											</div>
										{:else if showSearchMinCharsHint}
											<div class="repo-suggestion-state">Type at least 2 characters to search.</div>
										{:else if showNoSearchResults}
											<div class="repo-suggestion-state">
												No remote repositories found for "{lastSearchedQuery}".
											</div>
										{:else if remoteSuggestions.length === 0}
											<div class="repo-suggestion-state">Type at least 2 characters to search.</div>
										{:else}
											{#each remoteSuggestions as suggestion, index (suggestion.fullName)}
												<button
													type="button"
													class="repo-suggestion-item"
													class:active={index === activeSuggestionIndex}
													onmousedown={(event) => {
														event.preventDefault();
														selectRemoteSuggestion(suggestion);
													}}
												>
													<div class="repo-suggestion-main">
														<span class="repo-suggestion-name">{suggestion.fullName}</span>
														<span class="repo-suggestion-branch"
															>{suggestion.defaultBranch || 'main'}</span
														>
													</div>
													<div class="repo-suggestion-meta">
														<span>{suggestion.sshUrl || suggestion.cloneUrl}</span>
														{#if suggestion.private}
															<span class="repo-suggestion-flag">private</span>
														{/if}
														{#if suggestion.archived}
															<span class="repo-suggestion-flag">archived</span>
														{/if}
													</div>
												</button>
											{/each}
										{/if}
									</div>
								{/if}
							</div>
							<Button variant="ghost" size="sm" onclick={handleBrowseSource}>
								<FolderOpen size={14} />
								Browse
							</Button>
						</div>
						<p class="field-hint">
							Tip: type 2+ characters to search your GitHub repos, or paste a URL/path.
						</p>
					</div>

					<!-- Advanced section -->
					<details class="advanced-section" bind:open={advancedOpen}>
						<summary>
							<span class="summary-icon"><ChevronDown size={14} /></span>
							<span>Advanced</span>
						</summary>
						<div class="advanced-fields">
							<div class="form-field">
								<label for="reg-remote">Remote (optional)</label>
								<input
									id="reg-remote"
									type="text"
									bind:value={formRemote}
									placeholder="origin"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
							</div>
							<div class="form-field">
								<label for="reg-branch">Default branch</label>
								<input
									id="reg-branch"
									type="text"
									bind:value={formBranch}
									placeholder="main"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
							</div>
						</div>
					</details>
				</div>

				{#if detecting}
					<div class="detecting-feedback">
						<span class="spin-icon"><Loader2 size={14} /></span>
						<span>Registering repository...</span>
					</div>
				{/if}

				{#if error && isRegistering}
					<div class="message error ws-message ws-message-error">{error}</div>
				{/if}

				<div class="form-actions">
					<Button variant="ghost" size="sm" onclick={handleCancel} disabled={detecting}>
						Cancel
					</Button>
					<Button
						variant="primary"
						size="sm"
						onclick={handleSave}
						disabled={!formName.trim() || detecting}
					>
						{detecting
							? isEditing
								? 'Saving...'
								: 'Registering...'
							: isEditing
								? 'Save'
								: 'Register'}
					</Button>
				</div>
			</div>
		{/if}

		<!-- Search and List -->
		<div class="repo-list-container">
			<div class="search-bar">
				<span class="search-icon"><Search size={14} /></span>
				<input type="text" placeholder="Filter by name..." bind:value={searchQuery} />
			</div>

			<div class="repo-list">
				{#each filteredAliases as alias (alias.name)}
					<div class="repo-card">
						<div class="repo-info">
							<div class="repo-header">
								<span class="repo-name">{alias.name}</span>
								<span class="repo-branch">{alias.default_branch || 'main'}</span>
							</div>
							<div class="repo-source">{alias.url ?? alias.path ?? ''}</div>
						</div>
						<div class="repo-actions">
							<button
								class="action-btn ws-icon-action-btn"
								onclick={() => startEdit(alias)}
								title="Edit"
							>
								<Pencil size={14} />
							</button>
							<button
								class="action-btn ws-icon-action-btn"
								onclick={() => handleCopyUrl(alias)}
								title={copiedId === alias.name ? 'Copied!' : 'Copy remote URL'}
							>
								<Copy size={14} />
							</button>
							<button
								class="action-btn ws-icon-action-btn danger"
								onclick={() => handleDelete(alias)}
								disabled={loading}
								title="Remove from catalog"
							>
								<Trash2 size={14} />
							</button>
						</div>
					</div>
				{:else}
					{#if !isRegistering}
						<div class="empty-state ws-empty-state">
							<p class="ws-empty-state-copy">No repositories found.</p>
							<Button variant="ghost" onclick={() => (isRegistering = true)}>
								Register your first repo
							</Button>
						</div>
					{/if}
				{/each}
			</div>

			{#if filteredAliases.length > 0}
				<div class="list-footer">
					{filteredAliases.length} of {aliases.length} repositories
				</div>
			{/if}
		</div>
	</div>
</SettingsSection>

<style>
	.catalog-container {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.catalog-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: var(--space-3);
	}

	.header-text {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.section-title {
		font-size: var(--text-md);
		font-weight: 600;
		color: var(--text);
		margin: 0;
		text-transform: none;
		letter-spacing: normal;
	}

	.section-desc {
		font-size: var(--text-base);
		color: var(--muted);
		margin: 0;
	}

	.registration-form {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: var(--space-5);
	}

	.form-header h4 {
		font-size: var(--text-sm);
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--text);
		margin: 0 0 var(--space-4) 0;
	}

	.form-fields {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.form-field {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.form-field label {
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.form-field input {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-md);
		padding: 10px 12px;
		font-size: var(--text-mono-base);
		font-family: var(--font-mono);
		transition: border-color var(--transition-fast);
	}

	.form-field input:focus {
		outline: none;
		border-color: var(--accent);
	}

	.form-field input:disabled {
		opacity: 0.6;
		cursor: not-allowed;
		background: var(--panel-soft);
	}

	.field-hint {
		font-size: var(--text-xs);
		color: var(--subtle);
		margin: 2px 0 0 0;
		font-style: italic;
	}

	.input-with-button {
		display: flex;
		gap: 8px;
		align-items: flex-start;
	}

	.source-input-wrap {
		position: relative;
		flex: 1;
	}

	.source-input-wrap input {
		width: 100%;
		padding-right: 36px;
	}

	.source-affordance {
		position: absolute;
		right: 10px;
		top: 50%;
		transform: translateY(-50%);
		display: inline-flex;
		color: var(--subtle);
		pointer-events: none;
	}

	.source-input-wrap:focus-within .source-affordance {
		color: var(--accent);
	}

	.repo-suggestions {
		position: absolute;
		left: 0;
		right: 0;
		top: calc(100% + 6px);
		z-index: 30;
		max-height: 260px;
		overflow-y: auto;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		box-shadow: 0 8px 24px rgb(0 0 0 / 24%);
	}

	.repo-suggestion-state {
		padding: var(--space-3);
		font-size: var(--text-base);
		color: var(--muted);
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.repo-suggestion-error {
		color: var(--danger);
	}

	.repo-suggestion-item {
		width: 100%;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 4px;
		padding: var(--space-3);
		border: none;
		border-bottom: 1px solid var(--border);
		background: transparent;
		color: inherit;
		text-align: left;
		cursor: pointer;
	}

	.repo-suggestion-item:last-child {
		border-bottom: none;
	}

	.repo-suggestion-item:hover,
	.repo-suggestion-item.active {
		background: color-mix(in srgb, var(--accent) 10%, transparent);
	}

	.repo-suggestion-main {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.repo-suggestion-name {
		font-size: var(--text-mono-base);
		font-family: var(--font-mono);
		color: var(--text);
	}

	.repo-suggestion-branch {
		font-size: var(--text-xs);
		padding: 2px 8px;
		border-radius: 999px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		color: var(--muted);
	}

	.repo-suggestion-meta {
		font-size: var(--text-mono-sm);
		color: var(--subtle);
		font-family: var(--font-mono);
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-2);
		align-items: center;
	}

	.repo-suggestion-flag {
		font-family: var(--font-sans);
		font-size: var(--text-xs);
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 1px 6px;
	}

	.advanced-section {
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		overflow: hidden;
	}

	.advanced-section summary {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-3);
		font-size: var(--text-base);
		font-weight: 500;
		color: var(--muted);
		cursor: pointer;
		user-select: none;
		list-style: none;
	}

	.advanced-section summary::-webkit-details-marker {
		display: none;
	}

	.advanced-section summary .summary-icon {
		display: inline-flex;
		transition: transform var(--transition-fast);
	}

	.advanced-section[open] summary .summary-icon {
		transform: rotate(180deg);
	}

	.advanced-fields {
		padding: 0 var(--space-4) var(--space-4);
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		border-top: 1px solid var(--border);
		padding-top: var(--space-4);
	}

	.detecting-feedback {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-3);
		background: var(--accent-soft);
		border-radius: var(--radius-md);
		font-size: var(--text-base);
		color: var(--text);
		margin-top: var(--space-3);
	}

	.spin-icon {
		display: inline-flex;
	}

	.spin-icon :global(svg) {
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}

	.form-actions {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
		margin-top: var(--space-4);
		padding-top: var(--space-4);
		border-top: 1px solid var(--border);
	}

	.repo-list-container {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.search-bar {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		border-bottom: 1px solid var(--border);
		background: var(--panel-strong);
	}

	.search-icon {
		color: var(--muted);
		flex-shrink: 0;
	}

	.search-bar input {
		flex: 1;
		background: transparent;
		border: none;
		color: var(--text);
		font-size: var(--text-base);
		outline: none;
	}

	.search-bar input::placeholder {
		color: var(--subtle);
	}

	.repo-list {
		max-height: 320px;
		overflow-y: auto;
	}

	.repo-card {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: var(--space-3) var(--space-4);
		border-bottom: 1px solid var(--border);
		transition: background var(--transition-fast);
	}

	.repo-card:last-child {
		border-bottom: none;
	}

	.repo-card:hover {
		background: color-mix(in srgb, var(--text) 3%, transparent);
	}

	.repo-info {
		display: flex;
		flex-direction: column;
		gap: 4px;
		flex: 1;
		min-width: 0;
	}

	.repo-header {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.repo-name {
		font-family: var(--font-mono);
		font-size: var(--text-mono-base);
		font-weight: 500;
		color: var(--text);
	}

	.repo-branch {
		font-size: var(--text-xs);
		padding: 2px 8px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 999px;
		color: var(--muted);
	}

	.repo-source {
		font-size: var(--text-mono-sm);
		color: var(--subtle);
		font-family: var(--font-mono);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.repo-actions {
		display: flex;
		gap: var(--space-1);
		opacity: 0;
		transition: opacity var(--transition-fast);
	}

	.repo-card:hover .repo-actions {
		opacity: 1;
	}

	.list-footer {
		padding: var(--space-2) var(--space-3);
		background: var(--panel-strong);
		border-top: 1px solid var(--border);
		font-size: var(--text-sm);
		color: var(--subtle);
		text-align: center;
	}

	.empty-state {
		padding: 48px var(--space-4);
	}

	:global(.repo-card:hover .menu-btn) {
		opacity: 1;
	}
</style>
