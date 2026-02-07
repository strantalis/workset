<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
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
		runPendingHookWithState,
		shouldTrackHookEvent,
		trustPendingHookWithState,
		type WorkspaceActionPendingHook,
	} from '../services/workspaceActionHooks';
	import { formatWorkspaceActionError } from '../services/workspaceActionErrors';
	import { workspaceActionMutations } from '../services/workspaceActionService';
	import {
		addDirectRepoSource,
		removeDirectRepoByURL,
		removeSetItem,
		toggleDirectRepoRegisterByURL,
		toggleSetItem,
	} from '../services/workspaceActionSelectionState';
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
	import Modal from './Modal.svelte';
	import WorkspaceActionFormContent from './workspace-action/WorkspaceActionFormContent.svelte';
	import WorkspaceActionHookResults from './workspace-action/WorkspaceActionHookResults.svelte';
	import WorkspaceActionStatusAlerts from './workspace-action/WorkspaceActionStatusAlerts.svelte';

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

	let phase = $state<'form' | 'hook-results'>('form');
	let hookResultContext = $state<{
		action: 'created' | 'added';
		name: string;
		itemCount?: number;
	} | null>(null);

	let removing = $state(false);
	let removalSuccess = $state(false);

	let primaryInput = $state('');
	let directRepos: WorkspaceActionDirectRepo[] = $state([]);
	let customizeName = $state('');

	type CreateTab = 'direct' | 'repos' | 'groups';
	let activeTab = $state<CreateTab>('direct');
	let searchQuery = $state('');
	let expandedGroups = $state<Set<string>>(new Set());

	let renameName = $state('');

	let addSource = $state('');
	let aliasItems: Alias[] = $state([]);
	let groupItems: GroupSummary[] = $state([]);
	let groupDetails: Map<string, string[]> = $state(new Map());

	let selectedAliases: Set<string> = $state(new Set());
	let selectedGroups: Set<string> = $state(new Set());

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

	const addRepoContext = $derived.by(() =>
		deriveAddRepoContext({
			addSource,
			selectedAliases,
			selectedGroups,
		}),
	);
	const addRepoSelectedItems = $derived(addRepoContext.selectedItems);
	const addRepoTotalItems = $derived(addRepoContext.totalItems);

	const existingRepos = $derived(deriveExistingReposContext({ mode, workspace }));

	$effect(() => {
		if (mode === 'create' && aliasItems.length > 0) {
			activeTab = 'repos';
		} else if (mode === 'create') {
			activeTab = 'direct';
		}
	});

	const selectAlternative = (name: string): void => {
		customizeName = name;
	};

	const toggleAlias = (name: string): void => {
		selectedAliases = toggleSetItem(selectedAliases, name);
	};

	const toggleGroup = (name: string): void => {
		selectedGroups = toggleSetItem(selectedGroups, name);
	};

	const toggleGroupExpand = (name: string): void => {
		expandedGroups = toggleSetItem(expandedGroups, name);
	};

	const removeAlias = (name: string): void => {
		selectedAliases = removeSetItem(selectedAliases, name);
	};

	const removeGroup = (name: string): void => {
		selectedGroups = removeSetItem(selectedGroups, name);
	};

	const handleTabChange = (tab: CreateTab): void => {
		activeTab = tab;
		searchQuery = '';
	};

	const addDirectRepo = (): void => {
		const next = addDirectRepoSource(directRepos, primaryInput, isRepoSource);
		directRepos = next.directRepos;
		primaryInput = next.source;
	};

	const removeDirectRepo = (url: string): void => {
		directRepos = removeDirectRepoByURL(directRepos, url);
	};

	const toggleDirectRepoRegister = (url: string): void => {
		directRepos = toggleDirectRepoRegisterByURL(directRepos, url);
	};

	let archiveReason = $state('');
	let removeDeleteWorktree = $state(false);
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

	const handleRunPendingHook = async (pending: WorkspaceActionPendingHook): Promise<void> => {
		await runPendingHookWithState(
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
				formatError: formatWorkspaceActionError,
				setPendingHooks: (next) => (pendingHooks = next),
				setHookRuns: (next) => (hookRuns = next),
			},
		);
	};

	const handleTrustPendingHook = async (pending: WorkspaceActionPendingHook): Promise<void> => {
		await trustPendingHookWithState(
			{
				pending,
				pendingHooks,
				getPendingHooks: () => pendingHooks,
			},
			{
				trustRepoHooks,
				formatError: formatWorkspaceActionError,
				setPendingHooks: (next) => (pendingHooks = next),
			},
		);
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
			error = formatWorkspaceActionError(err, 'Failed to create workspace.');
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
			error = formatWorkspaceActionError(err, 'Failed to rename workspace.');
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
			error = formatWorkspaceActionError(err, 'Failed to add items.');
		} finally {
			loading = false;
			({ activeHookOperation, activeHookWorkspace } = clearHookTracking());
		}
	};

	const selectRepoDirectory = async (
		defaultDirectory: string,
		onPathSelected: (path: string) => void,
	): Promise<void> => {
		try {
			const path = await openDirectoryDialog('Select repo directory', defaultDirectory.trim());
			if (path) onPathSelected(path);
		} catch (err) {
			error = formatWorkspaceActionError(err, 'Failed to open directory picker.');
		}
	};

	const handleBrowse = (): Promise<void> =>
		selectRepoDirectory(addSource, (path) => (addSource = path));

	const handleCreateBrowse = (): Promise<void> =>
		selectRepoDirectory(primaryInput, (path) => (primaryInput = path));

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
			error = formatWorkspaceActionError(err, 'Failed to archive workspace.');
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
			error = formatWorkspaceActionError(err, 'Failed to remove workspace.');
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
			error = formatWorkspaceActionError(err, 'Failed to remove repo.');
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
	});

	onDestroy(() => {
		hookEventUnsubscribe?.();
		if (autoCloseTimer) clearTimeout(autoCloseTimer);
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
			onDone={onClose}
		/>
	{:else}
		<WorkspaceActionStatusAlerts
			{error}
			{success}
			{warnings}
			{hookRuns}
			{pendingHooks}
			onRunPendingHook={handleRunPendingHook}
			onTrustPendingHook={handleTrustPendingHook}
		/>

		<WorkspaceActionFormContent
			{mode}
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
			onCreateTabChange={handleTabChange}
			onCreatePrimaryInput={(value) => (primaryInput = value)}
			onCreateSearchQueryInput={(value) => (searchQuery = value)}
			onCreateAddDirectRepo={addDirectRepo}
			onCreateBrowsePrimary={handleCreateBrowse}
			onCreateToggleDirectRepoRegister={toggleDirectRepoRegister}
			onCreateRemoveDirectRepo={removeDirectRepo}
			onCreateToggleAlias={toggleAlias}
			onCreateToggleGroup={toggleGroup}
			onCreateToggleGroupExpand={toggleGroupExpand}
			onCreateRemoveAlias={removeAlias}
			onCreateRemoveGroup={removeGroup}
			onCreateCustomizeNameInput={(value) => (customizeName = value)}
			onCreateSelectAlternative={selectAlternative}
			onCreateSubmit={handleCreate}
			{renameName}
			onRenameNameInput={(value) => (renameName = value)}
			onRenameSubmit={handleRename}
			{addSource}
			{existingRepos}
			{addRepoSelectedItems}
			{addRepoTotalItems}
			onAddTabChange={handleTabChange}
			onAddSearchQueryInput={(value) => (searchQuery = value)}
			onAddSourceInput={(value) => (addSource = value)}
			onAddBrowse={handleBrowse}
			onAddToggleAlias={toggleAlias}
			onAddToggleGroup={toggleGroup}
			onAddToggleGroupExpand={toggleGroupExpand}
			onAddRemoveAlias={removeAlias}
			onAddRemoveGroup={removeGroup}
			onAddSubmit={handleAddItems}
			{archiveReason}
			onArchiveReasonInput={(value) => (archiveReason = value)}
			onArchiveSubmit={handleArchive}
			{removing}
			{removalSuccess}
			{removeDeleteFiles}
			{removeForceDelete}
			{removeConfirmText}
			{removeConfirmValid}
			onRemoveWorkspaceDeleteFilesToggle={(checked) => (removeDeleteFiles = checked)}
			onRemoveWorkspaceForceDeleteToggle={(checked) => (removeForceDelete = checked)}
			onRemoveWorkspaceConfirmTextInput={(value) => (removeConfirmText = value)}
			onRemoveWorkspaceSubmit={handleRemoveWorkspace}
			{removeDeleteWorktree}
			{removeRepoConfirmRequired}
			{removeRepoConfirmText}
			{removeRepoStatusRefreshing}
			{removeRepoStatus}
			{removeRepoConfirmValid}
			onRemoveRepoDeleteWorktreeToggle={(checked) => (removeDeleteWorktree = checked)}
			onRemoveRepoConfirmTextInput={(value) => (removeRepoConfirmText = value)}
			onRemoveRepoSubmit={handleRemoveRepo}
		/>
	{/if}
</Modal>
