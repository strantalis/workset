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
	import WorkspaceActionCreateForm from './workspace-action/WorkspaceActionCreateForm.svelte';
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

	const handleCreateBrowse = async (): Promise<void> => {
		try {
			const path = await openDirectoryDialog('Select repo directory', primaryInput.trim());
			if (path) primaryInput = path;
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
			<WorkspaceActionCreateForm
				{loading}
				{activeTab}
				{aliasItems}
				{groupItems}
				{searchQuery}
				{primaryInput}
				{directRepos}
				{filteredAliases}
				{filteredGroups}
				{selectedAliases}
				{selectedGroups}
				{expandedGroups}
				{groupDetails}
				{selectedItems}
				{totalRepos}
				{customizeName}
				{generatedName}
				{alternatives}
				{finalName}
				{getAliasSource}
				{deriveRepoName}
				{isRepoSource}
				onTabChange={handleTabChange}
				onPrimaryInput={(value) => (primaryInput = value)}
				onSearchQueryInput={(value) => (searchQuery = value)}
				onAddDirectRepo={addDirectRepo}
				onBrowsePrimary={handleCreateBrowse}
				onToggleDirectRepoRegister={toggleDirectRepoRegister}
				onRemoveDirectRepo={removeDirectRepo}
				onToggleAlias={toggleAlias}
				onToggleGroup={toggleGroup}
				onToggleGroupExpand={toggleGroupExpand}
				onRemoveAlias={removeAlias}
				onRemoveGroup={removeGroup}
				onCustomizeNameInput={(value) => (customizeName = value)}
				onSelectAlternative={selectAlternative}
				onSubmit={handleCreate}
			/>
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

	:global(.action-btn) {
		width: 100%;
		margin-top: 8px;
	}

	.hint {
		font-size: 12px;
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
