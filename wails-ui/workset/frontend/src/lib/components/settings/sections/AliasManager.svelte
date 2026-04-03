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
		listRegisteredRepos,
		openDirectoryDialog,
		registerRepo,
		unregisterRepo,
		updateRegisteredRepo,
	} from '../../../api/settings';
	import type { Alias, GitHubRepoSearchItem } from '../../../types';
	import { toErrorMessage } from '../../../errors';
	import SettingsSection from '../SettingsSection.svelte';
	import Button from '../../ui/Button.svelte';
	import { deriveRepoName, looksLikeUrl } from '../../../names';
	import { createRepoSearch } from '../../../composables/createRepoSearch.svelte';

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
	const repoSearch = createRepoSearch({
		isActive: () => isRegistering && !isEditing,
	});

	// Search and copy state
	let searchQuery = $state('');
	let copiedId: string | null = $state(null);

	const loadAliases = async (): Promise<void> => {
		try {
			aliases = await listRegisteredRepos();
			onAliasCountChange(aliases.length);
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
		}
	};

	const filteredAliases = $derived(
		aliases.filter((a) => a.name.toLowerCase().includes(searchQuery.toLowerCase())),
	);
	const showRemoteSuggestionPanel = $derived(
		isRegistering && !isEditing && repoSearch.suggestionsOpen && repoSearch.focused,
	);

	const handleSourceInput = (value: string): void => {
		formSource = value;
		repoSearch.queue(value);
	};

	const selectRemoteSuggestion = (suggestion: GitHubRepoSearchItem): void => {
		formSource = suggestion.sshUrl || suggestion.cloneUrl;
		formName = suggestion.name;
		formBranch = suggestion.defaultBranch.trim() || 'main';
		repoSearch.reset();
	};

	const handleSourceKeydown = (event: KeyboardEvent): void => {
		repoSearch.handleKeydown(event, selectRemoteSuggestion);
	};

	const openRemoteSuggestions = (): void => {
		if (!isRegistering || isEditing) return;
		repoSearch.handleFocus(formSource);
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
		repoSearch.reset();
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
		repoSearch.reset();
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
				await updateRegisteredRepo(name, source, remote, branch);
				success = `Updated ${name}.`;
			} else {
				await registerRepo(name, source, remote, branch);
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
			repoSearch.reset();
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
			await unregisterRepo(alias.name);
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
			repoSearch.reset();
		} catch (err) {
			error = toErrorMessage(err, 'An error occurred.');
		}
	};

	onMount(() => {
		void loadAliases();
	});

	onDestroy(() => {
		repoSearch.destroy();
	});
</script>

<SettingsSection
	title="Repo Catalog"
	description="Your catalog of known repositories. Repos registered here can be quickly added to worksets by name."
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
							class="ws-field-input ws-field-input--strong-bg ws-field-input--mono"
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
									class="ws-field-input ws-field-input--strong-bg ws-field-input--mono"
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
									onblur={repoSearch.handleBlur}
								/>
								<span class="source-affordance" aria-hidden="true">
									{#if repoSearch.loading}
										<span class="spin-icon"><Loader2 size={14} /></span>
									{:else}
										<Search size={14} />
									{/if}
								</span>
								{#if showRemoteSuggestionPanel}
									<div class="repo-suggestions">
										{#if repoSearch.loading}
											<div class="repo-suggestion-state">
												<span class="spin-icon"><Loader2 size={14} /></span>
												<span>Searching remote repositories…</span>
											</div>
										{:else if repoSearch.error}
											<div class="repo-suggestion-state repo-suggestion-error">
												{repoSearch.error}
											</div>
										{:else if repoSearch.showSearchStartHint}
											<div class="repo-suggestion-state">
												Start typing to search GitHub repositories.
											</div>
										{:else if repoSearch.showSearchMinCharsHint}
											<div class="repo-suggestion-state">Type at least 2 characters to search.</div>
										{:else if repoSearch.showNoSearchResults}
											<div class="repo-suggestion-state">
												No remote repositories found for "{repoSearch.lastSearchedQuery}".
											</div>
										{:else if repoSearch.results.length === 0}
											<div class="repo-suggestion-state">Type at least 2 characters to search.</div>
										{:else}
											{#each repoSearch.results as suggestion, index (suggestion.fullName)}
												<button
													type="button"
													class="repo-suggestion-item"
													class:active={index === repoSearch.activeSuggestionIndex}
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
									class="ws-field-input ws-field-input--strong-bg ws-field-input--mono"
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
									class="ws-field-input ws-field-input--strong-bg ws-field-input--mono"
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
				<input
					class="ws-field-input ws-field-input--ghost"
					type="text"
					placeholder="Filter by name..."
					aria-label="Filter repositories by name"
					bind:value={searchQuery}
				/>
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
								data-hover-label="Edit"
								aria-label="Edit"
							>
								<Pencil size={14} />
							</button>
							<button
								class="action-btn ws-icon-action-btn"
								onclick={() => handleCopyUrl(alias)}
								data-hover-label={copiedId === alias.name ? 'Copied!' : 'Copy remote URL'}
								aria-label="Copy remote URL"
							>
								<Copy size={14} />
							</button>
							<button
								class="action-btn ws-icon-action-btn danger"
								onclick={() => handleDelete(alias)}
								disabled={loading}
								data-hover-label="Remove from catalog"
								aria-label="Remove from catalog"
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
