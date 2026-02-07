<script lang="ts">
	import { onDestroy, onMount, tick } from 'svelte';
	import { get } from 'svelte/store';
	import { runRepoHooks, trustRepoHooks } from '../api/workspaces';
	import { getGroup, listAliases, listGroups, openDirectoryDialog } from '../api/settings';
	import {
		activeWorkspaceId,
		clearRepo,
		clearWorkspace,
		loadWorkspaces,
		refreshWorkspacesStatus,
		selectWorkspace,
		workspaces,
	} from '../state';
	import type { Alias, GroupSummary, HookExecution, Repo, Workspace } from '../types';
	import { subscribeHookProgressEvent } from '../hookEventService';
	import { deriveRepoName, isRepoSource } from '../names';
	import {
		applyHookProgress,
		appendHookRuns,
		beginHookTracking,
		clearHookTracking,
		handleRunPendingHookCore,
		handleTrustPendingHookCore,
		shouldTrackHookEvent,
		type WorkspaceActionPendingHook,
	} from '../services/workspaceActionHooks';
	import { workspaceActionMutations } from '../services/workspaceActionService';
	import {
		deriveWorkspaceActionModalSize,
		deriveWorkspaceActionModalSubtitle,
		deriveWorkspaceActionModalTitle,
		resetWorkspaceActionFlow,
		resolveMutationHookTransition,
		resolveRemovalState,
		shouldRefreshRemoveRepoStatus,
	} from '../services/workspaceActionModalController';
	import {
		deriveAddRepoContext,
		deriveExistingReposContext,
		deriveWorkspaceActionContext,
		getAliasSource,
		loadWorkspaceActionContext,
		type WorkspaceActionDirectRepo,
	} from '../services/workspaceActionContextService';
	import Alert from './ui/Alert.svelte';
	import Button from './ui/Button.svelte';
	import Modal from './Modal.svelte';
	import WorkspaceActionHookResults from './workspace-action/WorkspaceActionHookResults.svelte';
	import WorkspaceActionSelectionTabs from './workspace-action/WorkspaceActionSelectionTabs.svelte';
	import WorkspaceActionAddRepoForm from './workspace-action/WorkspaceActionAddRepoForm.svelte';
	import WorkspaceActionRemoveRepoForm from './workspace-action/WorkspaceActionRemoveRepoForm.svelte';
	import WorkspaceActionRemoveWorkspaceForm from './workspace-action/WorkspaceActionRemoveWorkspaceForm.svelte';

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
	let warnings: string[] = $state([]);
	let pendingHooks: WorkspaceActionPendingHook[] = $state([]);
	let hookRuns: HookExecution[] = $state([]);
	let activeHookOperation: string | null = $state(null);
	let activeHookWorkspace: string | null = $state(null);
	let hookWorkspaceId: string | null = $state(null);
	let hookEventUnsubscribe: (() => void) | null = null;
	let autoCloseTimer: ReturnType<typeof setTimeout> | null = null;
	let loading = $state(false);

	// Phase state: 'form' for input, 'hook-results' after successful create/add with hooks
	let phase = $state<'form' | 'hook-results'>('form');
	let hookResultContext = $state<{
		action: 'created' | 'added';
		name: string;
		itemCount?: number;
	} | null>(null);

	// Removal modal state for loading overlay
	let removing = $state(false);
	let removalSuccess = $state(false);

	let nameInput: HTMLInputElement | null = $state(null);

	// Create mode: smart single input
	let primaryInput = $state(''); // URL, path, or workspace name
	let directRepos: WorkspaceActionDirectRepo[] = $state([]); // Multiple direct URLs/paths
	let customizeName = $state(''); // Override for generated name

	// Tabbed interface state
	type CreateTab = 'direct' | 'repos' | 'groups';
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
	const createContext = $derived.by(() =>
		deriveWorkspaceActionContext({
			primaryInput,
			directRepos,
			customizeName,
			searchQuery,
			aliasItems,
			groupItems,
			selectedAliases,
			selectedGroups,
		}),
	);
	const generatedName = $derived(createContext.generatedName);
	const finalName = $derived(createContext.finalName);
	const alternatives = $derived(createContext.alternatives);
	const filteredAliases = $derived(createContext.filteredAliases);
	const filteredGroups = $derived(createContext.filteredGroups);
	const totalRepos = $derived(createContext.totalRepos);
	const selectedItems = $derived(createContext.selectedItems);

	// Add-repo mode: derived state for selected items
	const addRepoContext = $derived.by(() =>
		deriveAddRepoContext({
			addSource,
			selectedAliases,
			selectedGroups,
		}),
	);
	const addRepoSelectedItems = $derived(addRepoContext.selectedItems);
	const addRepoTotalItems = $derived(addRepoContext.totalItems);

	// Existing repos in workspace (read-only context for add-repo mode)
	const existingRepos = $derived(deriveExistingReposContext({ mode, workspace }));

	// Initialize tab based on available items
	$effect(() => {
		if (mode === 'create' && aliasItems.length > 0) {
			activeTab = 'repos';
		} else if (mode === 'create') {
			activeTab = 'direct';
		}
	});

	function selectAlternative(name: string): void {
		customizeName = name;
	}

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

	function handleTabChange(tab: CreateTab): void {
		activeTab = tab;
		searchQuery = '';
	}

	function addDirectRepo(): void {
		const source = primaryInput.trim();
		if (source && isRepoSource(source) && !directRepos.some((r) => r.url === source)) {
			directRepos = [...directRepos, { url: source, register: true }];
			primaryInput = '';
		}
	}

	function removeDirectRepo(url: string): void {
		directRepos = directRepos.filter((r) => r.url !== url);
	}

	function toggleDirectRepoRegister(url: string): void {
		directRepos = directRepos.map((r) => (r.url === url ? { ...r, register: !r.register } : r));
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

	const removeConfirmValid = $derived(!removeDeleteFiles || removeConfirmText === 'DELETE');
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
		const resolved = resolveRemovalState({
			removeDeleteFiles,
			removeForceDelete,
			removeConfirmText,
			removeRepoConfirmRequired,
			removeRepoConfirmText,
			removeRepoStatusRequested,
		});
		removeForceDelete = resolved.removeForceDelete;
		removeConfirmText = resolved.removeConfirmText;
		removeRepoConfirmText = resolved.removeRepoConfirmText;
		removeRepoStatusRequested = resolved.removeRepoStatusRequested;
	});

	$effect(() => {
		if (shouldRefreshRemoveRepoStatus(removeRepoConfirmRequired, removeRepoStatusRequested)) {
			removeRepoStatusRequested = true;
			removeRepoStatusRefreshing = true;
			void (async () => {
				await refreshWorkspacesStatus(true);
				removeRepoStatusRefreshing = false;
			})();
		}
	});

	const modeTitle = $derived(deriveWorkspaceActionModalTitle(mode, phase));

	const modalSubtitle = $derived.by(() => {
		return deriveWorkspaceActionModalSubtitle({
			phase,
			mode,
			workspaceName: workspace?.name ?? null,
			hookResultContext,
		});
	});

	const modalSize = $derived(deriveWorkspaceActionModalSize(mode, phase));

	const formatError = (err: unknown, fallback: string): string => {
		if (err instanceof Error) return err.message;
		if (typeof err === 'string') return err;
		if (err && typeof err === 'object' && 'message' in err) {
			const message = (err as { message?: string }).message;
			if (typeof message === 'string') return message;
		}
		return fallback;
	};

	const handleRunPendingHook = async (pending: WorkspaceActionPendingHook): Promise<void> => {
		const runState = handleRunPendingHookCore(
			{
				pending,
				pendingHooks,
				hookRuns,
				workspaceReferences: [workspace?.id, workspaceId, hookWorkspaceId, activeHookWorkspace],
				activeHookOperation,
				getPendingHooks: () => pendingHooks,
				getHookRuns: () => hookRuns,
			},
			{
				runRepoHooks,
				formatError,
			},
		);
		pendingHooks = runState.pendingHooks;
		hookRuns = runState.hookRuns;
		const completed = await runState.completion;
		pendingHooks = completed.pendingHooks;
		hookRuns = completed.hookRuns;
	};

	const handleTrustPendingHook = async (pending: WorkspaceActionPendingHook): Promise<void> => {
		const trustState = handleTrustPendingHookCore(
			{
				pending,
				pendingHooks,
				getPendingHooks: () => pendingHooks,
			},
			{
				trustRepoHooks,
				formatError,
			},
		);
		pendingHooks = trustState.pendingHooks;
		pendingHooks = await trustState.completion;
	};

	const loadContext = async (): Promise<void> => {
		({ phase, hookResultContext } = resetWorkspaceActionFlow());
		const context = await loadWorkspaceActionContext(
			{
				mode,
				workspaceId,
				repoName,
			},
			{
				loadWorkspaces,
				getWorkspaces: () => get(workspaces),
				listAliases,
				listGroups,
				getGroup,
			},
		);
		workspace = context.workspace;
		repo = context.repo;
		if (mode === 'rename' && context.workspace) {
			renameName = context.renameName;
		}
		if (mode === 'add-repo' || mode === 'create') {
			aliasItems = context.aliasItems;
			groupItems = context.groupItems;
			groupDetails = context.groupDetails;
		}
	};

	const handleCreate = async (): Promise<void> => {
		if (!finalName) {
			error = 'Enter a repo URL, path, or workspace name.';
			return;
		}
		loading = true;
		error = null;
		success = null;
		warnings = [];
		pendingHooks = [];
		hookRuns = [];
		hookWorkspaceId = null;
		({ activeHookOperation, activeHookWorkspace, hookRuns, pendingHooks } = beginHookTracking(
			'workspace.create',
			finalName,
		));
		try {
			const result = await workspaceActionMutations.createWorkspace({
				finalName,
				primaryInput,
				directRepos,
				selectedAliases,
				selectedGroups,
			});

			hookRuns = appendHookRuns(hookRuns, result.hookRuns);
			pendingHooks = result.pendingHooks.map((pending) => ({ ...pending }));
			hookWorkspaceId = result.workspaceName;
			await loadWorkspaces(true);
			selectWorkspace(result.workspaceName);
			warnings = result.warnings;
			const transition = resolveMutationHookTransition({
				action: 'created',
				workspaceName: result.workspaceName,
				warnings,
				pendingHooks,
				hookRuns,
			});
			success = transition.success;
			hookResultContext = transition.hookResultContext;
			phase = transition.phase;
			if (transition.shouldClose) {
				onClose();
			} else if (transition.shouldAutoClose) {
				autoCloseTimer = setTimeout(() => onClose(), 1500);
			}
		} catch (err) {
			error = formatError(err, 'Failed to create workspace.');
		} finally {
			loading = false;
			({ activeHookOperation, activeHookWorkspace } = clearHookTracking());
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
		success = null;
		warnings = [];
		try {
			const result = await workspaceActionMutations.renameWorkspace({
				workspaceId: workspace.id,
				workspaceName: nextName,
			});
			await loadWorkspaces(true);
			if (get(activeWorkspaceId) === workspace.id) {
				selectWorkspace(result.workspaceName);
			}
			success = `Renamed to ${result.workspaceName}.`;
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
		success = null;
		warnings = [];
		pendingHooks = [];
		hookRuns = [];
		hookWorkspaceId = workspace.id;
		({ activeHookOperation, activeHookWorkspace, hookRuns, pendingHooks } = beginHookTracking(
			'repo.add',
			workspace.name,
		));
		try {
			const result = await workspaceActionMutations.addItems({
				workspaceId: workspace.id,
				source,
				selectedAliases,
				selectedGroups,
			});

			hookRuns = appendHookRuns(hookRuns, result.hookRuns);
			pendingHooks = result.pendingHooks.map((pending) => ({ ...pending }));

			await loadWorkspaces(true);
			const itemCount = result.itemCount;
			warnings = result.warnings;
			const transition = resolveMutationHookTransition({
				action: 'added',
				workspaceName: workspace.name,
				itemCount,
				warnings,
				pendingHooks,
				hookRuns,
			});
			success = transition.success;
			hookResultContext = transition.hookResultContext;
			phase = transition.phase;
			if (transition.shouldClose) {
				onClose();
			} else if (transition.shouldAutoClose) {
				autoCloseTimer = setTimeout(() => onClose(), 1500);
			}
		} catch (err) {
			error = formatError(err, 'Failed to add items.');
		} finally {
			loading = false;
			({ activeHookOperation, activeHookWorkspace } = clearHookTracking());
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
		success = null;
		warnings = [];
		try {
			const result = await workspaceActionMutations.archiveWorkspace({
				workspaceId: workspace.id,
				reason: archiveReason.trim(),
			});
			await loadWorkspaces(true);
			if (get(activeWorkspaceId) === result.workspaceId) {
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
		success = null;
		warnings = [];
		try {
			if (removeDeleteFiles && !removeConfirmValid) {
				error = 'Type DELETE to confirm file deletion.';
				removing = false;
				return;
			}
			const result = await workspaceActionMutations.removeWorkspace({
				workspaceId,
				deleteFiles: removeDeleteFiles,
				force: removeForceDelete,
			});
			workspaces.update((current) => current.filter((entry) => entry.id !== result.workspaceId));
			if (get(activeWorkspaceId) === result.workspaceId) {
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
		success = null;
		warnings = [];
		try {
			if (!removeRepoConfirmValid) {
				error = 'Type DELETE to confirm repo deletion.';
				removing = false;
				return;
			}
			const result = await workspaceActionMutations.removeRepo({
				workspaceId: workspace.id,
				repoName: repo.name,
				deleteWorktree: removeDeleteWorktree,
			});
			await loadWorkspaces(true);
			if (get(activeWorkspaceId) === result.workspaceId) {
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
		hookEventUnsubscribe = subscribeHookProgressEvent((payload) => {
			if (!shouldTrackHookEvent(payload, { activeHookOperation, activeHookWorkspace, loading })) {
				return;
			}
			hookRuns = applyHookProgress(hookRuns, payload);
		});
		await loadContext();
		await tick();
		nameInput?.focus();
	});

	onDestroy(() => {
		hookEventUnsubscribe?.();
		hookEventUnsubscribe = null;
		if (autoCloseTimer) {
			clearTimeout(autoCloseTimer);
			autoCloseTimer = null;
		}
	});
</script>

<Modal
	title={modeTitle}
	subtitle={modalSubtitle}
	size={modalSize}
	headerAlign="left"
	{onClose}
	disableClose={removing}
>
	{#if phase === 'hook-results'}
		<WorkspaceActionHookResults
			{success}
			{warnings}
			{hookRuns}
			{pendingHooks}
			onRunPendingHook={handleRunPendingHook}
			onTrustPendingHook={handleTrustPendingHook}
			onDone={() => {
				if (autoCloseTimer) {
					clearTimeout(autoCloseTimer);
					autoCloseTimer = null;
				}
				onClose();
			}}
		/>
	{:else}
		{#if error}
			<Alert variant="error">{error}</Alert>
		{/if}
		{#if success}
			<Alert variant="success">{success}</Alert>
		{/if}
		{#if warnings.length > 0}
			<Alert variant="warning">
				{#each warnings as warning (warning)}
					<div>{warning}</div>
				{/each}
			</Alert>
		{/if}
		{#if hookRuns.length > 0}
			<Alert variant="info">
				{#each hookRuns as run (`${run.repo}:${run.event}:${run.id}`)}
					<div>
						<code>{run.repo}</code> <code>{run.id}</code>: <code>{run.status}</code>
						{#if run.log_path}
							(log: <code>{run.log_path}</code>)
						{/if}
					</div>
				{/each}
			</Alert>
		{/if}
		{#if pendingHooks.length > 0}
			<Alert variant="warning">
				{#each pendingHooks as pending (`${pending.repo}:${pending.event}`)}
					<div class="pending-hook-row">
						<div>
							{pending.repo} pending hooks: {pending.hooks.join(', ')}
							{#if pending.trusted}
								(trusted)
							{/if}
						</div>
						<div class="pending-hook-actions">
							<Button
								variant="ghost"
								size="sm"
								disabled={pending.running}
								onclick={() => void handleRunPendingHook(pending)}
							>
								{pending.running ? 'Running…' : 'Run now'}
							</Button>
							<Button
								variant="ghost"
								size="sm"
								disabled={pending.trusting || pending.trusted}
								onclick={() => void handleTrustPendingHook(pending)}
							>
								{pending.trusting ? 'Trusting…' : pending.trusted ? 'Trusted' : 'Trust'}
							</Button>
						</div>
						{#if pending.runError}
							<div class="pending-hook-error">{pending.runError}</div>
						{/if}
					</div>
				{/each}
			</Alert>
		{/if}

		{#if mode === 'create'}
			<div class="form create-two-column">
				<div class="column-left">
					<WorkspaceActionSelectionTabs
						{activeTab}
						aliasCount={aliasItems.length}
						groupCount={groupItems.length}
						onTabChange={handleTabChange}
					/>

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
										onkeydown={(e) => {
											if (e.key === 'Enter') {
												e.preventDefault();
												addDirectRepo();
											}
										}}
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
									<Button
										variant="primary"
										size="sm"
										onclick={addDirectRepo}
										disabled={!primaryInput.trim() || !isRepoSource(primaryInput)}>Add</Button
									>
								</div>
							</label>
							{#if directRepos.length > 0}
								<div class="direct-repos-list">
									{#each directRepos as repo (repo.url)}
										<div class="direct-repo-item">
											<div class="direct-repo-info">
												<span class="direct-repo-name">{deriveRepoName(repo.url) || repo.url}</span>
												<span class="direct-repo-url">{repo.url}</span>
											</div>
											<label
												class="direct-repo-register"
												title="Save to Repo Registry for future use"
											>
												<input
													type="checkbox"
													checked={repo.register}
													onchange={() => toggleDirectRepoRegister(repo.url)}
												/>
												<span>Register</span>
											</label>
											<button
												type="button"
												class="direct-repo-remove"
												onclick={() => removeDirectRepo(repo.url)}
											>
												×
											</button>
										</div>
									{/each}
								</div>
							{/if}
						{:else if activeTab === 'repos'}
							<div class="field">
								<div class="inline">
									<input
										bind:value={searchQuery}
										placeholder="Search repos..."
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
										<div class="empty-search">No repos match "{searchQuery}"</div>
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
						<div class="hint">No registered repos or groups configured. Add them in Settings.</div>
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
									<div class="selected-item" class:pending={item.pending}>
										<span class="selected-badge {item.type}">{item.type}</span>
										<span class="selected-name">{item.name}</span>
										{#if item.pending}
											<span class="pending-label">pending</span>
										{:else}
											<button
												type="button"
												class="selected-remove"
												onclick={() => {
													if (item.type === 'repo' && item.url) removeDirectRepo(item.url);
													else if (item.type === 'alias') removeAlias(item.name);
													else if (item.type === 'group') removeGroup(item.name);
												}}
											>
												×
											</button>
										{/if}
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
			<WorkspaceActionAddRepoForm
				{loading}
				{activeTab}
				{aliasItems}
				{groupItems}
				{searchQuery}
				{addSource}
				{filteredAliases}
				{filteredGroups}
				{selectedAliases}
				{selectedGroups}
				{expandedGroups}
				{groupDetails}
				{existingRepos}
				{addRepoSelectedItems}
				{addRepoTotalItems}
				{getAliasSource}
				onTabChange={handleTabChange}
				onSearchQueryInput={(value) => (searchQuery = value)}
				onAddSourceInput={(value) => (addSource = value)}
				onBrowse={handleBrowse}
				onToggleAlias={toggleAlias}
				onToggleGroup={toggleGroup}
				onToggleGroupExpand={toggleGroupExpand}
				onRemoveAlias={removeAlias}
				onRemoveGroup={removeGroup}
				onSubmit={handleAddItems}
			/>
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
			<WorkspaceActionRemoveWorkspaceForm
				{loading}
				{removing}
				{removalSuccess}
				{removeDeleteFiles}
				{removeForceDelete}
				{removeConfirmText}
				{removeConfirmValid}
				onToggleDeleteFiles={(checked) => (removeDeleteFiles = checked)}
				onToggleForceDelete={(checked) => (removeForceDelete = checked)}
				onConfirmTextInput={(value) => (removeConfirmText = value)}
				onSubmit={handleRemoveWorkspace}
			/>
		{:else if mode === 'remove-repo'}
			<WorkspaceActionRemoveRepoForm
				{loading}
				{removing}
				{removalSuccess}
				{removeDeleteWorktree}
				{removeRepoConfirmRequired}
				{removeRepoConfirmText}
				{removeRepoStatusRefreshing}
				{removeRepoStatus}
				{removeRepoConfirmValid}
				onToggleDeleteWorktree={(checked) => (removeDeleteWorktree = checked)}
				onConfirmTextInput={(value) => (removeRepoConfirmText = value)}
				onSubmit={handleRemoveRepo}
			/>
		{/if}
	{/if}
</Modal>

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

	:global(.action-btn) {
		width: 100%;
		margin-top: 8px;
	}

	.hint {
		font-size: 12px;
		color: var(--muted);
	}

	/* Direct repos list */
	.direct-repos-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
		margin-top: 8px;
		max-height: 180px;
		overflow-y: auto;
	}

	.direct-repo-item {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		font-size: 13px;
	}

	.direct-repo-info {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.direct-repo-name {
		font-weight: 500;
		color: var(--text);
	}

	.direct-repo-url {
		font-size: 11px;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.direct-repo-register {
		display: flex;
		align-items: center;
		gap: 4px;
		font-size: 11px;
		color: var(--muted);
		cursor: pointer;
		flex-shrink: 0;
	}

	.direct-repo-register input {
		accent-color: var(--accent);
	}

	.direct-repo-register:hover {
		color: var(--text);
	}

	.direct-repo-remove {
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

	.direct-repo-remove:hover {
		color: var(--danger, #ef4444);
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

	/* Two-column create layout */
	.form.create-two-column {
		display: grid;
		grid-template-columns: 1fr 280px;
		gap: 16px;
		max-height: 80vh;
		min-height: 500px;
		overflow: hidden;
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

	.selected-item.pending {
		background: rgba(255, 255, 255, 0.01);
		border: 1px dashed rgba(255, 255, 255, 0.15);
	}

	.pending-label {
		font-size: 10px;
		color: var(--muted);
		font-style: italic;
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

	.pending-hook-row {
		display: grid;
		gap: 6px;
		margin-bottom: 10px;
	}

	.pending-hook-actions {
		display: flex;
		gap: 8px;
	}

	.pending-hook-error {
		color: var(--danger);
		font-size: 12px;
	}
</style>
