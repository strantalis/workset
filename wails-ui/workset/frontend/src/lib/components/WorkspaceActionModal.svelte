<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { get } from 'svelte/store';
	import {
		addRepo,
		applyGroup,
		archiveWorkspace,
		createWorkspace,
		getGroup,
		listAliases,
		listGroups,
		openDirectoryDialog,
		removeRepo,
		removeWorkspace,
		renameWorkspace,
	} from '../api';
	import {
		activeWorkspaceId,
		clearRepo,
		clearWorkspace,
		loadWorkspaces,
		refreshWorkspacesStatus,
		selectWorkspace,
		workspaces,
	} from '../state';
	import type { Alias, GroupSummary, Repo, Workspace } from '../types';
	import {
		generateWorkspaceName,
		generateAlternatives,
		deriveRepoName,
		isRepoSource,
	} from '../names';
	import Alert from './ui/Alert.svelte';
	import Button from './ui/Button.svelte';
	import Modal from './Modal.svelte';

	interface Props {
		onClose: () => void;
		mode: 'create' | 'rename' | 'add-repo' | 'archive' | 'remove-workspace' | 'remove-repo' | null;
		workspaceId?: string | null;
		repoName?: string | null;
	}

	const { onClose, mode, workspaceId = null, repoName = null }: Props = $props();

	let workspace: Workspace | null = $state(null);
	let repo: Repo | null = $state(null);

	let error: string | null = $state(null);
	let success: string | null = $state(null);
	let loading = $state(false);

	// Removal modal state for loading overlay
	let removing = $state(false);
	let removalSuccess = $state(false);

	let nameInput: HTMLInputElement | null = $state(null);

	// Create mode: smart single input
	let primaryInput = $state(''); // URL, path, or workspace name
	let customizeName = $state(''); // Override for generated name
	let alternatives: string[] = $state([]); // Alternative name suggestions

	// Tabbed interface state
	type CreateTab = 'direct' | 'aliases' | 'groups';
	let activeTab = $state<CreateTab>('direct');
	let searchQuery = $state('');
	let expandedGroups = $state<Set<string>>(new Set());

	let renameName = $state('');

	let addSource = $state('');
	let aliasItems: Alias[] = $state([]);
	let groupItems: GroupSummary[] = $state([]);
	let groupDetails: Map<string, string[]> = $state(new Map()); // group name -> repo names

	// Selection state for create mode expanded section and add-repo mode
	let selectedAliases: Set<string> = $state(new Set());
	let selectedGroups: Set<string> = $state(new Set());

	// Create mode: derived state
	const detectedRepoName = $derived(deriveRepoName(primaryInput));
	const inputIsSource = $derived(isRepoSource(primaryInput));

	// Get the first selected alias or group name for auto-generation
	const firstSelectedAlias = $derived(
		selectedAliases.size > 0 ? Array.from(selectedAliases)[0] : null,
	);
	const firstSelectedGroup = $derived(
		selectedGroups.size > 0 ? Array.from(selectedGroups)[0] : null,
	);

	// Source for name generation: URL/path repo name, first alias, or first group
	const nameSource = $derived(detectedRepoName || firstSelectedAlias || firstSelectedGroup);

	const generatedName = $derived(nameSource ? generateWorkspaceName(nameSource) : null);

	// Final name: custom override > generated > plain text input
	const finalName = $derived(customizeName || generatedName || primaryInput.trim());

	// Tab filtering logic
	const filteredAliases = $derived(
		searchQuery
			? aliasItems.filter(
					(a) =>
						a.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
						getAliasSource(a).toLowerCase().includes(searchQuery.toLowerCase()),
				)
			: aliasItems,
	);

	const filteredGroups = $derived(
		searchQuery
			? groupItems.filter(
					(g) =>
						g.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
						(g.description?.toLowerCase() || '').includes(searchQuery.toLowerCase()),
				)
			: groupItems,
	);

	// Total repos for preview
	const totalRepos = $derived(
		(inputIsSource ? 1 : 0) +
			selectedAliases.size +
			Array.from(selectedGroups).reduce(
				(sum, g) => sum + (groupItems.find((i) => i.name === g)?.repo_count || 0),
				0,
			),
	);

	// Get selected items for preview
	const selectedItems = $derived([
		...(inputIsSource ? [{ type: 'repo', name: detectedRepoName || primaryInput.trim() }] : []),
		...Array.from(selectedAliases).map((name) => ({ type: 'alias', name })),
		...Array.from(selectedGroups).map((name) => ({ type: 'group', name })),
	]);

	// Add-repo mode: derived state for selected items
	const addRepoHasSource = $derived(addSource.trim().length > 0);
	const addRepoSelectedItems = $derived([
		...(addRepoHasSource ? [{ type: 'repo' as const, name: addSource.trim() }] : []),
		...Array.from(selectedAliases).map((name) => ({ type: 'alias' as const, name })),
		...Array.from(selectedGroups).map((name) => ({ type: 'group' as const, name })),
	]);
	const addRepoTotalItems = $derived(addRepoSelectedItems.length);

	// Existing repos in workspace (read-only context for add-repo mode)
	const existingRepos = $derived(
		mode === 'add-repo' && workspace
			? (workspace as Workspace).repos.map((r: Repo) => ({ name: r.name }))
			: [],
	);

	// Regenerate alternatives when name source changes
	$effect(() => {
		if (nameSource) {
			alternatives = generateAlternatives(nameSource, 2);
		} else {
			alternatives = [];
		}
	});

	// Initialize tab based on available items
	$effect(() => {
		if (mode === 'create' && aliasItems.length > 0) {
			activeTab = 'aliases';
		} else if (mode === 'create') {
			activeTab = 'direct';
		}
	});

	function selectAlternative(name: string): void {
		customizeName = name;
	}

	// Helper to get alias display info
	const getAliasSource = (alias: Alias): string => alias.url || alias.path || '';

	// Tab helper functions
	function toggleAlias(name: string): void {
		if (selectedAliases.has(name)) {
			selectedAliases.delete(name);
		} else {
			selectedAliases.add(name);
		}
		selectedAliases = new Set(selectedAliases);
	}

	function toggleGroup(name: string): void {
		if (selectedGroups.has(name)) {
			selectedGroups.delete(name);
		} else {
			selectedGroups.add(name);
		}
		selectedGroups = new Set(selectedGroups);
	}

	function toggleGroupExpand(name: string): void {
		if (expandedGroups.has(name)) {
			expandedGroups.delete(name);
		} else {
			expandedGroups.add(name);
		}
		expandedGroups = new Set(expandedGroups);
	}

	function removeAlias(name: string): void {
		selectedAliases.delete(name);
		selectedAliases = new Set(selectedAliases);
	}

	function removeGroup(name: string): void {
		selectedGroups.delete(name);
		selectedGroups = new Set(selectedGroups);
	}

	function removeRepoFromPreview(): void {
		primaryInput = '';
	}

	let archiveReason = $state('');
	let removeDeleteWorktree = $state(false);
	// Note: removeDeleteLocal is disabled in UI due to cross-workspace safety concerns
	const removeDeleteLocal = $state(false); // eslint-disable-line @typescript-eslint/no-unused-vars
	let removeDeleteFiles = $state(false);
	let removeForceDelete = $state(false);
	let removeConfirmText = $state('');
	let removeRepoConfirmText = $state('');
	let removeRepoStatusRequested = $state(false);
	let removeRepoStatusRefreshing = $state(false);

	const removeConfirmValid = $derived(
		!removeDeleteFiles || removeConfirmText === 'DELETE',
	);
	const removeRepoConfirmRequired = $derived(removeDeleteWorktree);
	const removeRepoConfirmValid = $derived(
		!removeRepoConfirmRequired || removeRepoConfirmText === 'DELETE',
	);
	const removeRepoStatus = $derived(
		workspaceId && repoName
			? ($workspaces
					.find((entry) => entry.id === workspaceId)
					?.repos.find((entry) => entry.name === repoName) ?? null)
			: null,
	);

	$effect(() => {
		if (!removeDeleteFiles && removeForceDelete) {
			removeForceDelete = false;
		}
		if (!removeDeleteFiles && removeConfirmText) {
			removeConfirmText = '';
		}
		if (!removeRepoConfirmRequired && removeRepoConfirmText) {
			removeRepoConfirmText = '';
		}
		if (!removeRepoConfirmRequired) {
			removeRepoStatusRequested = false;
		}
	});

	$effect(() => {
		if (removeRepoConfirmRequired && !removeRepoStatusRequested) {
			removeRepoStatusRequested = true;
			removeRepoStatusRefreshing = true;
			void (async () => {
				await refreshWorkspacesStatus(true);
				removeRepoStatusRefreshing = false;
			})();
		}
	});

	const modeTitle = $derived(
		mode === 'create'
			? 'Create workspace'
			: mode === 'rename'
				? 'Rename workspace'
				: mode === 'add-repo'
					? 'Add to workspace'
					: mode === 'archive'
						? 'Archive workspace'
						: mode === 'remove-workspace'
							? 'Remove workspace'
							: mode === 'remove-repo'
								? 'Remove repo'
								: 'Workspace action',
	);

	const modalSize = $derived(
		mode === 'create' || mode === 'add-repo' ? 'wide' : 'md',
	);

	const formatError = (err: unknown, fallback: string): string => {
		if (err instanceof Error) return err.message;
		if (typeof err === 'string') return err;
		if (err && typeof err === 'object' && 'message' in err) {
			const message = (err as { message?: string }).message;
			if (typeof message === 'string') return message;
		}
		return fallback;
	};

	const loadContext = async (): Promise<void> => {
		await loadWorkspaces(true);
		const current = get(workspaces);
		workspace = workspaceId ? (current.find((entry) => entry.id === workspaceId) ?? null) : null;
		repo =
			workspace && repoName
				? (workspace.repos.find((entry) => entry.name === repoName) ?? null)
				: null;
		if (mode === 'rename' && workspace) {
			renameName = workspace.name;
		}
		if (mode === 'add-repo' || mode === 'create') {
			aliasItems = await listAliases();
			groupItems = await listGroups();
			// Fetch full details for each group to show repo names in tooltips
			const details = new Map<string, string[]>();
			for (const g of groupItems) {
				const full = await getGroup(g.name);
				details.set(
					g.name,
					full.members.map((m) => m.repo),
				);
			}
			groupDetails = details;
		}
	};

	const handleCreate = async (): Promise<void> => {
		if (!finalName) {
			error = 'Enter a repo URL, path, or workspace name.';
			return;
		}
		loading = true;
		error = null;
		try {
			const repos: string[] = [];

			// If primary input is URL/path, add it as first repo
			if (inputIsSource) {
				repos.push(primaryInput.trim());
			}

			// Add any selected aliases
			for (const alias of selectedAliases) {
				repos.push(alias);
			}

			// Groups from selection
			const groups = Array.from(selectedGroups);

			const result = await createWorkspace(
				finalName,
				'',
				repos.length > 0 ? repos : undefined,
				groups.length > 0 ? groups : undefined,
			);

			await loadWorkspaces(true);
			selectWorkspace(result.workspace.name);
			success = `Created ${result.workspace.name}.`;
			onClose();
		} catch (err) {
			error = formatError(err, 'Failed to create workspace.');
		} finally {
			loading = false;
		}
	};

	const handleRename = async (): Promise<void> => {
		if (!workspace) return;
		const nextName = renameName.trim();
		if (!nextName) {
			error = 'New name is required.';
			return;
		}
		loading = true;
		error = null;
		try {
			await renameWorkspace(workspace.id, nextName);
			await loadWorkspaces(true);
			if (get(activeWorkspaceId) === workspace.id) {
				selectWorkspace(nextName);
			}
			success = `Renamed to ${nextName}.`;
			onClose();
		} catch (err) {
			error = formatError(err, 'Failed to rename workspace.');
		} finally {
			loading = false;
		}
	};

	const handleAddItems = async (): Promise<void> => {
		if (!workspace) return;
		const source = addSource.trim();
		const hasSource = source.length > 0;
		const hasAliases = selectedAliases.size > 0;
		const hasGroups = selectedGroups.size > 0;

		if (!hasSource && !hasAliases && !hasGroups) {
			error = 'Provide a repo URL/path, select aliases, or select groups.';
			return;
		}

		loading = true;
		error = null;
		try {
			// 1. Add direct repo URL if provided
			if (hasSource) {
				await addRepo(workspace.id, source, '', '');
			}
			// 2. Add each selected alias
			for (const alias of selectedAliases) {
				await addRepo(workspace.id, alias, '', '');
			}
			// 3. Apply each selected group
			for (const group of selectedGroups) {
				await applyGroup(workspace.id, group);
			}

			await loadWorkspaces(true);
			const itemCount = (hasSource ? 1 : 0) + selectedAliases.size + selectedGroups.size;
			success = `Added ${itemCount} item${itemCount !== 1 ? 's' : ''}.`;
			onClose();
		} catch (err) {
			error = formatError(err, 'Failed to add items.');
		} finally {
			loading = false;
		}
	};

	const handleBrowse = async (): Promise<void> => {
		try {
			const defaultDirectory = addSource.trim();
			const path = await openDirectoryDialog('Select repo directory', defaultDirectory);
			if (!path) return;
			addSource = path;
		} catch (err) {
			error = formatError(err, 'Failed to open directory picker.');
		}
	};

	const handleArchive = async (): Promise<void> => {
		if (!workspace) return;
		loading = true;
		error = null;
		try {
			await archiveWorkspace(workspace.id, archiveReason.trim());
			await loadWorkspaces(true);
			if (get(activeWorkspaceId) === workspace.id) {
				clearWorkspace();
			}
			onClose();
		} catch (err) {
			error = formatError(err, 'Failed to archive workspace.');
		} finally {
			loading = false;
		}
	};

	const handleRemoveWorkspace = async (): Promise<void> => {
		if (!workspaceId) return;
		loading = true;
		removing = true;
		error = null;
		try {
			if (removeDeleteFiles && !removeConfirmValid) {
				error = 'Type DELETE to confirm file deletion.';
				removing = false;
				return;
			}
			await removeWorkspace(workspaceId, {
				deleteFiles: removeDeleteFiles,
				force: removeForceDelete,
			});
			workspaces.update((current) => current.filter((entry) => entry.id !== workspaceId));
			if (get(activeWorkspaceId) === workspaceId) {
				clearWorkspace();
			}
			// Show success state before closing
			removalSuccess = true;
			await new Promise((resolve) => setTimeout(resolve, 800));
			onClose();
			void loadWorkspaces(true);
		} catch (err) {
			error = formatError(err, 'Failed to remove workspace.');
			removing = false;
		} finally {
			loading = false;
		}
	};

	const handleRemoveRepo = async (): Promise<void> => {
		if (!workspace || !repo) return;
		loading = true;
		removing = true;
		error = null;
		try {
			if (!removeRepoConfirmValid) {
				error = 'Type DELETE to confirm repo deletion.';
				removing = false;
				return;
			}
			await removeRepo(workspace.id, repo.name, removeDeleteWorktree, false);
			await loadWorkspaces(true);
			if (get(activeWorkspaceId) === workspace.id) {
				clearRepo();
			}
			// Show success state before closing
			removalSuccess = true;
			await new Promise((resolve) => setTimeout(resolve, 800));
			onClose();
		} catch (err) {
			error = formatError(err, 'Failed to remove repo.');
			removing = false;
		} finally {
			removeRepoConfirmText = '';
			loading = false;
		}
	};

	onMount(async () => {
		await loadContext();
		await tick();
		nameInput?.focus();
	});
</script>

<Modal
	title={modeTitle}
	subtitle={mode === 'create' ? '' : (workspace?.name ?? '')}
	size={modalSize}
	headerAlign="left"
	{onClose}
	disableClose={removing}
>
	{#if error}
		<Alert variant="error">{error}</Alert>
	{:else if success}
		<Alert variant="success">{success}</Alert>
	{/if}

	{#if mode === 'create'}
		<div class="form create-two-column">
			<div class="column-left">
				<!-- Tab Bar - only when aliases/groups exist -->
				{#if aliasItems.length > 0 || groupItems.length > 0}
					<div class="tab-bar">
						<button
							class="tab"
							class:active={activeTab === 'direct'}
							type="button"
							onclick={() => {
								activeTab = 'direct';
								searchQuery = '';
							}}
						>
							Direct
						</button>
						{#if aliasItems.length > 0}
							<button
								class="tab"
								class:active={activeTab === 'aliases'}
								type="button"
								onclick={() => {
									activeTab = 'aliases';
									searchQuery = '';
								}}
							>
								Aliases ({aliasItems.length})
							</button>
						{/if}
						{#if groupItems.length > 0}
							<button
								class="tab"
								class:active={activeTab === 'groups'}
								type="button"
								onclick={() => {
									activeTab = 'groups';
									searchQuery = '';
								}}
							>
								Groups ({groupItems.length})
							</button>
						{/if}
					</div>
				{/if}

				<!-- Selection Area - Left Column -->
				<div class="selection-area">
					{#if activeTab === 'direct'}
						<label class="field">
							<span>Repo URL or local path</span>
							<div class="inline">
								<input
									bind:value={primaryInput}
									placeholder="git@github.com:org/repo.git"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
								<Button
									variant="ghost"
									size="sm"
									onclick={async () => {
										try {
											const path = await openDirectoryDialog(
												'Select repo directory',
												primaryInput.trim(),
											);
											if (path) primaryInput = path;
										} catch (err) {
											error = formatError(err, 'Failed to open directory picker.');
										}
									}}>Browse</Button
								>
							</div>
						</label>
					{:else if activeTab === 'aliases'}
						<div class="field">
							<div class="inline">
								<input
									bind:value={searchQuery}
									placeholder="Search aliases..."
									class="search-input"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
								{#if searchQuery}
									<button type="button" class="search-clear" onclick={() => (searchQuery = '')}
										>Clear</button
									>
								{/if}
							</div>
							<div class="checkbox-list">
								{#if filteredAliases.length === 0}
									<div class="empty-search">No aliases match "{searchQuery}"</div>
								{:else}
									{#each filteredAliases as alias (alias)}
										<label class="checkbox-item" class:selected={selectedAliases.has(alias.name)}>
											<input
												type="checkbox"
												checked={selectedAliases.has(alias.name)}
												onchange={() => toggleAlias(alias.name)}
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
									bind:value={searchQuery}
									placeholder="Search groups..."
									class="search-input"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
								{#if searchQuery}
									<button type="button" class="search-clear" onclick={() => (searchQuery = '')}
										>Clear</button
									>
								{/if}
							</div>
							<div class="group-list">
								{#if filteredGroups.length === 0}
									<div class="empty-search">No groups match "{searchQuery}"</div>
								{:else}
									{#each filteredGroups as group (group)}
										<label class="group-card" class:selected={selectedGroups.has(group.name)}>
											<input
												type="checkbox"
												checked={selectedGroups.has(group.name)}
												onchange={() => toggleGroup(group.name)}
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
													onclick={(e) => {
														e.preventDefault();
														toggleGroupExpand(group.name);
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
					<div class="hint">No aliases or groups configured. Add them in Settings.</div>
				{/if}
			</div>

			<div class="column-right">
				<div class="selection-panel">
					<h4 class="panel-title">Selected ({totalRepos} repos)</h4>

					<div class="selected-list">
						{#if selectedItems.length === 0}
							<div class="empty-selection">No repos selected</div>
						{:else}
							{#each selectedItems as item (item.name)}
								<div class="selected-item">
									<span class="selected-badge {item.type}">{item.type}</span>
									<span class="selected-name">{item.name}</span>
									<button
										type="button"
										class="selected-remove"
										onclick={() => {
											if (item.type === 'repo') removeRepoFromPreview();
											else if (item.type === 'alias') removeAlias(item.name);
											else if (item.type === 'group') removeGroup(item.name);
										}}
									>
										×
									</button>
								</div>
							{/each}
						{/if}
					</div>

					<div class="panel-section">
						<span class="panel-label">Workspace name</span>
						<input
							bind:value={customizeName}
							placeholder={generatedName || 'workspace-name'}
							class="name-input"
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
						{#if alternatives.length > 0}
							<div class="alt-chips">
								{#each alternatives as alt, i (i)}
									<button type="button" class="alt-chip" onclick={() => selectAlternative(alt)}
										>{alt}</button
									>
								{/each}
							</div>
						{/if}
					</div>

					<Button
						variant="primary"
						onclick={handleCreate}
						disabled={loading || !finalName}
						class="create-btn"
					>
						{loading ? 'Creating…' : 'Create'}
					</Button>
				</div>
			</div>
		</div>
	{:else if mode === 'rename'}
		<div class="form">
			<label class="field">
				<span>New name</span>
				<input
					bind:this={nameInput}
					bind:value={renameName}
					placeholder="acme"
					autocapitalize="off"
					autocorrect="off"
					spellcheck="false"
				/>
			</label>
			<div class="hint">Renaming updates config and workset.yaml. Files stay in place.</div>
			<Button variant="primary" onclick={handleRename} disabled={loading} class="action-btn">
				{loading ? 'Renaming…' : 'Rename'}
			</Button>
		</div>
	{:else if mode === 'add-repo'}
		<div class="form add-two-column">
			<div class="column-left">
				<!-- Tab Bar - only when aliases/groups exist -->
				{#if aliasItems.length > 0 || groupItems.length > 0}
					<div class="tab-bar">
						<button
							class="tab"
							class:active={activeTab === 'direct'}
							type="button"
							onclick={() => {
								activeTab = 'direct';
								searchQuery = '';
							}}
						>
							Direct
						</button>
						{#if aliasItems.length > 0}
							<button
								class="tab"
								class:active={activeTab === 'aliases'}
								type="button"
								onclick={() => {
									activeTab = 'aliases';
									searchQuery = '';
								}}
							>
								Aliases ({aliasItems.length})
							</button>
						{/if}
						{#if groupItems.length > 0}
							<button
								class="tab"
								class:active={activeTab === 'groups'}
								type="button"
								onclick={() => {
									activeTab = 'groups';
									searchQuery = '';
								}}
							>
								Groups ({groupItems.length})
							</button>
						{/if}
					</div>
				{/if}

				<!-- Selection Area - Left Column -->
				<div class="selection-area">
					{#if activeTab === 'direct'}
						<label class="field">
							<span>Repo URL or local path</span>
							<div class="inline">
								<input
									bind:value={addSource}
									placeholder="git@github.com:org/repo.git"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
								<Button variant="ghost" size="sm" onclick={handleBrowse}>Browse</Button>
							</div>
						</label>
					{:else if activeTab === 'aliases'}
						<div class="field">
							<div class="inline">
								<input
									bind:value={searchQuery}
									placeholder="Search aliases..."
									class="search-input"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
								{#if searchQuery}
									<button type="button" class="search-clear" onclick={() => (searchQuery = '')}
										>Clear</button
									>
								{/if}
							</div>
							<div class="checkbox-list">
								{#if filteredAliases.length === 0}
									<div class="empty-search">No aliases match "{searchQuery}"</div>
								{:else}
									{#each filteredAliases as alias (alias)}
										<label class="checkbox-item" class:selected={selectedAliases.has(alias.name)}>
											<input
												type="checkbox"
												checked={selectedAliases.has(alias.name)}
												onchange={() => toggleAlias(alias.name)}
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
									bind:value={searchQuery}
									placeholder="Search groups..."
									class="search-input"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
								{#if searchQuery}
									<button type="button" class="search-clear" onclick={() => (searchQuery = '')}
										>Clear</button
									>
								{/if}
							</div>
							<div class="group-list">
								{#if filteredGroups.length === 0}
									<div class="empty-search">No groups match "{searchQuery}"</div>
								{:else}
									{#each filteredGroups as group (group)}
										<label class="group-card" class:selected={selectedGroups.has(group.name)}>
											<input
												type="checkbox"
												checked={selectedGroups.has(group.name)}
												onchange={() => toggleGroup(group.name)}
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
													onclick={(e) => {
														e.preventDefault();
														toggleGroupExpand(group.name);
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
					<div class="hint">No aliases or groups configured. Add them in Settings.</div>
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
											if (item.type === 'repo') addSource = '';
											else if (item.type === 'alias') removeAlias(item.name);
											else if (item.type === 'group') removeGroup(item.name);
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
						onclick={handleAddItems}
						disabled={loading || addRepoTotalItems === 0}
						class="create-btn"
					>
						{loading ? 'Adding…' : 'Add'}
					</Button>
				</div>
			</div>
		</div>
	{:else if mode === 'archive'}
		<div class="form">
			<div class="hint">Archiving hides the workspace but keeps files on disk.</div>
			<label class="field">
				<span>Reason (optional)</span>
				<input
					bind:this={nameInput}
					bind:value={archiveReason}
					placeholder="paused"
					autocapitalize="off"
					autocorrect="off"
					spellcheck="false"
				/>
			</label>
			<Button variant="danger" onclick={handleArchive} disabled={loading} class="action-btn">
				{loading ? 'Archiving…' : 'Archive'}
			</Button>
		</div>
	{:else if mode === 'remove-workspace'}
		<div class="form form-removing" class:removing class:success={removalSuccess}>
			<div class="form-content">
				<div class="hint hint-intro">Remove workspace registration only by default.</div>
				<label class="option option-main">
					<input type="checkbox" bind:checked={removeDeleteFiles} />
					<span>Also delete workspace files and worktrees</span>
				</label>
				{#if removeDeleteFiles}
					<div class="deletion-options">
						<div class="hint deletion-hint">
							Deletes the workspace directory and removes all worktrees.
						</div>
						<label class="field">
							<span>Type DELETE to confirm</span>
							<input
								bind:value={removeConfirmText}
								placeholder="DELETE"
								autocapitalize="off"
								autocorrect="off"
								spellcheck="false"
							/>
						</label>
						<label class="option">
							<input type="checkbox" bind:checked={removeForceDelete} />
							<span>Force delete (skip safety checks)</span>
						</label>
						{#if removeForceDelete}
							<Alert variant="warning">
								Force delete bypasses dirty/unmerged checks and may delete uncommitted work.
							</Alert>
						{/if}
					</div>
				{/if}
				<Button
					variant="danger"
					onclick={handleRemoveWorkspace}
					disabled={loading || !removeConfirmValid}
					class="action-btn"
				>
					{loading ? 'Removing…' : 'Remove workspace'}
				</Button>
			</div>
			{#if removing}
				<div class="removal-overlay">
					{#if removalSuccess}
						<div class="removal-success">
							<svg
								class="success-icon"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
							>
								<path d="M20 6L9 17l-5-5" />
							</svg>
							<span class="removal-text">Removed successfully</span>
						</div>
					{:else}
						<div class="removal-loading">
							<div class="spinner"></div>
							<span class="removal-text">Removing workspace…</span>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	{:else if mode === 'remove-repo'}
		<div class="form form-removing" class:removing class:success={removalSuccess}>
			<div class="form-content">
				<div class="hint hint-intro">
					This removes the repo from the workspace config by default.
				</div>
				<label class="option option-main">
					<input type="checkbox" bind:checked={removeDeleteWorktree} />
					<span>Also delete worktrees for this repo</span>
				</label>
				{#if removeRepoConfirmRequired}
					<div class="deletion-options">
						<label class="field">
							<span>Type DELETE to confirm</span>
							<input
								bind:value={removeRepoConfirmText}
								placeholder="DELETE"
								autocapitalize="off"
								autocorrect="off"
								spellcheck="false"
							/>
						</label>
						{#if removeDeleteWorktree}
							<div class="hint deletion-hint">
								Destructive deletes are permanent and cannot be undone.
							</div>
						{/if}
						{#if removeRepoStatusRefreshing}
							<Alert variant="warning">Fetching repo status…</Alert>
						{:else if removeRepoStatus?.statusKnown === false && removeDeleteWorktree}
							<Alert variant="warning">
								Repo status unknown. Destructive deletes may be blocked if the repo is dirty.
							</Alert>
						{/if}
						{#if removeRepoStatus?.dirty && removeDeleteWorktree}
							<Alert variant="warning">
								Uncommitted changes detected. Destructive deletes will be blocked until the repo is
								clean.
							</Alert>
						{/if}
					</div>
				{/if}
				<Button
					variant="danger"
					onclick={handleRemoveRepo}
					disabled={loading || !removeRepoConfirmValid}
					class="action-btn"
				>
					{loading ? 'Removing…' : 'Remove repo'}
				</Button>
			</div>
			{#if removing}
				<div class="removal-overlay">
					{#if removalSuccess}
						<div class="removal-success">
							<svg
								class="success-icon"
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
							>
								<path d="M20 6L9 17l-5-5" />
							</svg>
							<span class="removal-text">Removed successfully</span>
						</div>
					{:else}
						<div class="removal-loading">
							<div class="spinner"></div>
							<span class="removal-text">Removing repo…</span>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	{/if}
</Modal>

<style>
	.form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.form.form-removing {
		gap: 20px;
	}

	.deletion-options {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 16px;
		margin-top: 4px;
	}

	.deletion-options :global(.alert) {
		margin: 0;
	}

	.deletion-hint {
		line-height: 1.5;
		margin: 0;
	}

	/* Better spacing for removal modal elements */
	.hint-intro {
		margin-bottom: 8px;
		line-height: 1.5;
	}

	.option-main {
		margin-top: 4px;
		margin-bottom: 4px;
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

	:global(.action-btn) {
		width: 100%;
		margin-top: 8px;
	}

	.form-removing :global(.action-btn) {
		margin-top: 16px;
	}

	.hint {
		font-size: 12px;
		color: var(--muted);
	}

	/* Suggestions */
	.suggestions {
		font-size: 12px;
		color: var(--muted);
		margin-top: -4px;
	}

	.suggestion-btn {
		background: transparent;
		border: none;
		color: var(--accent);
		cursor: pointer;
		padding: 0;
		font-size: 12px;
		transition: opacity var(--transition-fast);
	}

	.suggestion-btn:hover {
		opacity: 0.8;
		text-decoration: underline;
	}

	.option {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: var(--text);
	}

	.option input {
		accent-color: var(--accent);
	}

	/* Checkbox list styles - clean with subtle dividers */
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

	/* Preview Panel */
	.preview-panel {
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 12px;
		margin-bottom: 8px;
	}

	.preview-header {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: var(--text);
	}

	.preview-check {
		color: var(--success, #4ade80);
		font-size: 14px;
		flex-shrink: 0;
	}

	.preview-chips {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		margin-top: 10px;
	}

	.preview-chip {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		background: var(--accent);
		color: #0a0f14;
		padding: 3px 8px;
		border-radius: var(--radius-sm);
		font-size: 12px;
		font-weight: 500;
	}

	.preview-chip.alias {
		background: #8b5cf6;
		color: white;
	}

	.preview-chip.group {
		background: #f59e0b;
		color: #0a0f14;
	}

	.chip-remove {
		background: transparent;
		border: none;
		color: inherit;
		cursor: pointer;
		padding: 0;
		font-size: 14px;
		line-height: 1;
		opacity: 0.7;
		transition: opacity var(--transition-fast);
	}

	.chip-remove:hover {
		opacity: 1;
	}

	/* Tab Bar */
	.tab-bar {
		display: flex;
		gap: 8px;
		border-bottom: 1px solid var(--border);
		padding-bottom: 8px;
	}

	.tab {
		display: flex;
		align-items: center;
		gap: 6px;
		background: transparent;
		border: none;
		color: var(--muted);
		padding: 6px 12px;
		font-size: 13px;
		cursor: pointer;
		border-radius: var(--radius-md);
		transition: all var(--transition-fast);
	}

	.tab:hover {
		color: var(--text);
		background: rgba(255, 255, 255, 0.05);
	}

	.tab.active {
		color: var(--text);
		background: var(--accent);
		font-weight: 500;
	}

	/* Tab Content */
	.tab-content {
		max-height: 280px;
		overflow-y: auto;
	}

	/* Search */
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

	/* Group Cards - clean with subtle dividers matching aliases */
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

	/* Removal modal loading overlay styles */
	.form-removing {
		position: relative;
	}

	.form-content {
		transition:
			opacity 0.3s ease,
			filter 0.3s ease;
	}

	.form-removing.removing .form-content {
		opacity: 0.4;
		filter: blur(1px);
		pointer-events: none;
	}

	.form-removing.success .form-content {
		opacity: 0.3;
		filter: blur(2px);
		pointer-events: none;
	}

	.removal-overlay {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(11, 15, 24, 0.6);
		border-radius: var(--radius-md);
		animation: overlayFadeIn 0.2s ease-out;
	}

	@keyframes overlayFadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	.removal-loading,
	.removal-success {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
		padding: 24px;
		animation: contentSlideIn 0.3s ease-out;
	}

	@keyframes contentSlideIn {
		from {
			opacity: 0;
			transform: translateY(8px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.spinner {
		width: 32px;
		height: 32px;
		border: 3px solid var(--muted);
		border-top-color: var(--accent);
		border-radius: 50%;
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

	.removal-text {
		font-size: 14px;
		font-weight: 500;
		color: var(--text);
	}

	.success-icon {
		width: 48px;
		height: 48px;
		color: var(--success);
		animation: successPop 0.4s ease-out;
	}

	@keyframes successPop {
		0% {
			transform: scale(0.5);
			opacity: 0;
		}
		50% {
			transform: scale(1.1);
		}
		100% {
			transform: scale(1);
			opacity: 1;
		}
	}

	.removal-success {
		animation: containerPulse 1.2s ease-out;
	}

	@keyframes containerPulse {
		0% {
			box-shadow: 0 0 0 0 rgba(var(--success-rgb), 0.4);
		}
		50% {
			box-shadow: 0 0 16px 6px rgba(var(--success-rgb), 0.15);
		}
		100% {
			box-shadow: 0 0 0 0 rgba(var(--success-rgb), 0);
		}
	}

	/* Two-column create layout */
	.form.create-two-column {
		display: grid;
		grid-template-columns: 1fr 280px;
		gap: 16px;
		max-height: 80vh;
		min-height: 500px;
		overflow: hidden;
	}

	/* Two-column add-repo layout */
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

	.column-left {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		overflow: hidden;
	}

	.column-right {
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow: hidden;
	}

	/* Selection panel - sticky right column */
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

	/* Selected items list */
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

	/* Existing repos section - more obvious styling */
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

	/* Selection area - left column */
	.selection-area {
		display: flex;
		flex-direction: column;
		gap: 8px;
		flex: 1;
		min-height: 0;
	}

	/* Maximize list height in left column */
	.create-two-column .checkbox-list,
	.create-two-column .group-list {
		max-height: 65vh;
		min-height: 300px;
	}

	/* Compact alias/group items for density */
	.create-two-column .checkbox-item {
		padding: 6px 10px;
	}

	.create-two-column .checkbox-item input[type='checkbox'] {
		width: 16px;
		height: 16px;
		min-width: 16px;
		min-height: 16px;
	}

	.create-two-column .checkbox-name {
		font-size: 13px;
	}

	.create-two-column .checkbox-meta {
		font-size: 11px;
	}

	.create-two-column .group-card {
		padding: 8px 10px;
	}

	.create-two-column .group-name {
		font-size: 13px;
	}

	.create-two-column .group-description {
		font-size: 11px;
	}

	/* Workspace name section */
	.name-section {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.name-input {
		width: 100%;
		font-size: 14px;
		padding: 10px 12px;
		background: transparent;
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		color: var(--text);
		box-sizing: border-box;
	}

	.name-input:focus {
		outline: none;
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.02);
	}

	/* Alternative name chips */
	.alt-chips {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.alt-chip {
		background: transparent;
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		color: var(--accent);
		cursor: pointer;
		padding: 6px 12px;
		font-size: 12px;
		transition: all var(--transition-fast);
	}

	.alt-chip:hover {
		background: rgba(255, 255, 255, 0.05);
		border-color: var(--accent);
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

	.footer-left {
		display: flex;
		flex-direction: column;
		gap: 8px;
		flex: 1;
		min-width: 0;
	}

	.field-label {
		font-size: 12px;
		color: var(--muted);
		white-space: nowrap;
		flex-shrink: 0;
	}

	.suggestions-inline {
		display: flex;
		align-items: center;
		gap: 6px;
		padding-left: 44px;
		font-size: 12px;
	}

	.alt-sep {
		color: var(--muted);
		opacity: 0.5;
	}

	.suggestion-link {
		background: transparent;
		border: none;
		color: var(--accent);
		cursor: pointer;
		padding: 0;
		font-size: 12px;
		transition: opacity var(--transition-fast);
	}

	.suggestion-link:hover {
		opacity: 0.8;
		text-decoration: underline;
	}

	.footer-right {
		display: flex;
		align-items: center;
		gap: 20px;
		max-width: 60%;
	}

	/* Compact preview in footer - improved layout */
	.preview-compact {
		display: flex;
		align-items: center;
		gap: 12px;
		font-size: 13px;
		padding: 6px 12px;
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
	}

	.repo-count {
		color: var(--muted);
		margin-left: 4px;
		font-weight: 500;
	}

	.mini-chips {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}

	.mini-chip {
		display: inline-flex;
		align-items: center;
		padding: 3px 8px;
		border-radius: var(--radius-sm);
		font-size: 11px;
		font-weight: 500;
		background: var(--accent);
		color: #0a0f14;
		white-space: nowrap;
	}

	.mini-chip.alias {
		background: #8b5cf6;
		color: white;
	}

	.mini-chip.group {
		background: #f59e0b;
		color: #0a0f14;
	}

	.mini-chip.more {
		background: rgba(255, 255, 255, 0.1);
		color: var(--muted);
	}
</style>
