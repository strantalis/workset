<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { get } from 'svelte/store';
	import { previewRepoHooks } from '../api/workspaces';
	import {
		activeWorkspaceId,
		clearRepo,
		clearWorkspace,
		loadWorkspaces,
		refreshWorkspacesStatus,
		selectWorkspace,
		workspaces,
	} from '../state';
	import type { Alias, HookExecution, Repo, Workspace } from '../types';
	import { subscribeHookProgressEvent } from '../hookEventService';
	import { deriveRepoName, isRepoSource } from '../names';
	import {
		applyHookProgress,
		appendHookRuns,
		beginHookTracking,
		clearHookTracking,
		shouldTrackHookEvent,
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
		type WorkspaceActionDirectRepo,
	} from '../services/workspaceActionContextService';
	import {
		archiveWorkspaceAction,
		browseWorkspaceActionDirectory,
		loadWorkspaceActionModalContext,
		renameWorkspaceAction,
		runWorkspaceActionPendingHook,
		trustWorkspaceActionPendingHook,
	} from '../services/workspaceActionModalActions';
	import Modal from './Modal.svelte';
	import WorkspaceActionFormContent from './workspace-action/WorkspaceActionFormContent.svelte';
	import WorkspaceActionHookResults from './workspace-action/WorkspaceActionHookResults.svelte';
	import WorkspaceActionStatusAlerts from './workspace-action/WorkspaceActionStatusAlerts.svelte';

	interface Props {
		onClose: () => void;
		mode:
			| 'create'
			| 'create-thread'
			| 'rename'
			| 'add-repo'
			| 'archive'
			| 'remove-workspace'
			| 'remove-repo'
			| null;
		workspaceId?: string | null;
		workspaceIds?: string[];
		repoName?: string | null;
		worksetName?: string | null;
		worksetRepos?: string[];
	}

	const {
		onClose,
		mode,
		workspaceId = null,
		workspaceIds = [],
		repoName = null,
		worksetName = null,
		worksetRepos = [],
	}: Props = $props();

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

	let searchQuery = $state('');

	let renameName = $state('');

	let addSource = $state('');
	let aliasItems: Alias[] = $state([]);

	let selectedAliases: Set<string> = $state(new Set());
	let seededThreadAliases = $state(false);
	let threadHookRows = $state<Array<{ repoName: string; hooks: string[]; hasSource: boolean }>>([]);
	let threadHooksLoading = $state(false);
	let threadHooksError: string | null = $state(null);
	let threadHooksFingerprint = $state('');
	let threadHooksPreviewSequence = 0;

	const createContext = $derived.by(() =>
		deriveWorkspaceActionContext({
			primaryInput,
			directRepos,
			customizeName,
			searchQuery,
			aliasItems,
			selectedAliases,
		}),
	);
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const generatedName = $derived(createContext.generatedName);
	const finalName = $derived(createContext.finalName);
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const alternatives = $derived(createContext.alternatives);
	const filteredAliases = $derived.by(() => {
		if (mode === 'add-repo') {
			const query = searchQuery.trim().toLowerCase();
			if (!query) return aliasItems;
			return aliasItems.filter(
				(alias) =>
					alias.name.toLowerCase().includes(query) ||
					getAliasSource(alias).toLowerCase().includes(query),
			);
		}
		return createContext.filteredAliases;
	});
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const totalRepos = $derived(createContext.totalRepos);
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const selectedItems = $derived(createContext.selectedItems);

	const addRepoContext = $derived.by(() =>
		deriveAddRepoContext({
			addSource,
			selectedAliases,
		}),
	);
	const addRepoSelectedItems = $derived(addRepoContext.selectedItems);
	const addRepoTotalItems = $derived(addRepoContext.totalItems);

	const targetWorkspaceIds = $derived.by(() => {
		const ids = workspaceIds.map((id) => id.trim()).filter((id) => id.length > 0);
		if (workspace?.id && !ids.includes(workspace.id)) {
			ids.unshift(workspace.id);
		}
		return Array.from(new Set(ids));
	});
	const existingRepos = $derived.by(() =>
		deriveExistingReposContext({
			mode,
			workspace,
			workspaces: $workspaces,
			workspaceIds: targetWorkspaceIds,
		}),
	);
	const normalizedWorksetRepos = $derived.by(() =>
		Array.from(
			new Set(
				worksetRepos.map((repoName) => repoName.trim()).filter((repoName) => repoName.length > 0),
			),
		),
	);
	const threadRepoSources = $derived.by(() => {
		const sourceByRepo = new Map<string, string>();
		for (const repoName of normalizedWorksetRepos) {
			const alias = aliasItems.find((entry) => entry.name === repoName);
			const source = alias ? getAliasSource(alias).trim() : '';
			if (source.length > 0) {
				sourceByRepo.set(repoName, source);
			}
		}
		return sourceByRepo;
	});

	$effect(() => {
		if (mode !== 'create-thread' || seededThreadAliases) return;
		const seeded = new Set(
			worksetRepos.map((repoName) => repoName.trim()).filter((repoName) => repoName.length > 0),
		);
		selectedAliases = seeded;
		seededThreadAliases = true;
	});

	$effect(() => {
		if (mode !== 'create-thread') {
			threadHookRows = [];
			threadHooksLoading = false;
			threadHooksError = null;
			threadHooksFingerprint = '';
			return;
		}

		const repos = normalizedWorksetRepos;
		const fingerprint = repos
			.map((repoName) => `${repoName}:${threadRepoSources.get(repoName) ?? ''}`)
			.join('|');
		if (threadHooksFingerprint === fingerprint && threadHookRows.length === repos.length) {
			return;
		}
		threadHooksFingerprint = fingerprint;
		if (repos.length === 0) {
			threadHookRows = [];
			threadHooksLoading = false;
			threadHooksError = null;
			return;
		}

		const sequence = ++threadHooksPreviewSequence;
		threadHooksLoading = true;
		threadHooksError = null;
		void (async () => {
			const rows = await Promise.all(
				repos.map(async (repoName) => {
					const source = threadRepoSources.get(repoName);
					if (!source) {
						return {
							repoName,
							hooks: [] as string[],
							hasSource: false,
							failed: false,
						};
					}
					try {
						const hooks = await previewRepoHooks(source);
						return {
							repoName,
							hooks: Array.from(
								new Set(hooks.map((hook) => hook.trim()).filter((hook) => hook.length > 0)),
							),
							hasSource: true,
							failed: false,
						};
					} catch {
						return {
							repoName,
							hooks: [] as string[],
							hasSource: true,
							failed: true,
						};
					}
				}),
			);
			if (sequence !== threadHooksPreviewSequence) return;
			threadHookRows = rows.map(({ repoName, hooks, hasSource }) => ({
				repoName,
				hooks,
				hasSource,
			}));
			const failedRepos = rows.filter((row) => row.failed).map((row) => row.repoName);
			if (failedRepos.length === 1) {
				threadHooksError = `Unable to preview hooks for ${failedRepos[0]}.`;
			} else if (failedRepos.length > 1) {
				threadHooksError = `Unable to preview hooks for ${failedRepos.length} repositories.`;
			} else {
				threadHooksError = null;
			}
			threadHooksLoading = false;
		})();
	});

	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const selectAlternative = (name: string): void => {
		customizeName = name;
	};

	const toggleAlias = (name: string): void => {
		selectedAliases = toggleSetItem(selectedAliases, name);
	};

	const removeAlias = (name: string): void => {
		selectedAliases = removeSetItem(selectedAliases, name);
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
		await runWorkspaceActionPendingHook({
			pending,
			pendingHooks,
			hookRuns,
			workspaceReferences: [
				workspace?.id,
				workspaceId,
				hookWorkspaceId,
				activeHookWorkspace,
				worksetName,
				...targetWorkspaceIds,
			],
			activeHookOperation,
			getPendingHooks: () => pendingHooks,
			getHookRuns: () => hookRuns,
			setPendingHooks: (next) => (pendingHooks = next),
			setHookRuns: (next) => (hookRuns = next),
		});
	};

	const handleTrustPendingHook = async (pending: WorkspaceActionPendingHook): Promise<void> => {
		await trustWorkspaceActionPendingHook({
			pending,
			pendingHooks,
			hookRuns,
			workspaceReferences: [
				workspace?.id,
				workspaceId,
				hookWorkspaceId,
				activeHookWorkspace,
				worksetName,
				...targetWorkspaceIds,
			],
			activeHookOperation,
			getPendingHooks: () => pendingHooks,
			getHookRuns: () => hookRuns,
			setPendingHooks: (next) => (pendingHooks = next),
			setHookRuns: (next) => (hookRuns = next),
		});
	};

	const loadContext = async (): Promise<void> => {
		({ phase, hookResultContext } = resetWorkspaceActionFlow());
		const context = await loadWorkspaceActionModalContext(
			{
				mode,
				workspaceId,
				repoName,
			},
			{
				loadWorkspaces,
				getWorkspaces: () => get(workspaces),
			},
		);
		workspace = context.workspace;
		repo = context.repo;
		if (mode === 'rename' && context.workspace) {
			renameName = context.renameName;
		}
		if (mode === 'add-repo' || mode === 'create' || mode === 'create-thread') {
			aliasItems = context.aliasItems;
		}
	};

	const handleCreate = async (): Promise<void> => {
		const isThreadMode = mode === 'create-thread';
		const threadRepos = Array.from(
			new Set(
				worksetRepos.map((repoName) => repoName.trim()).filter((repoName) => repoName.length > 0),
			),
		);
		const aliasesToCreate = isThreadMode ? new Set(threadRepos) : selectedAliases;
		const pendingSource = isThreadMode ? '' : primaryInput.trim();
		const hasPendingSource = pendingSource.length > 0 && isRepoSource(pendingSource);
		const hasDirectRepos = !isThreadMode && (directRepos.length > 0 || hasPendingSource);
		const hasCatalogRepos = aliasesToCreate.size > 0;
		if (isThreadMode && threadRepos.length === 0) {
			error = 'Selected workset has no repositories.';
			return;
		}
		if (!isThreadMode && !hasCatalogRepos && !hasDirectRepos) {
			error = 'Select at least one repository or add a repository source.';
			return;
		}
		if (!finalName) {
			error = mode === 'create-thread' ? 'Enter a thread name.' : 'Enter a workset name.';
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
				primaryInput: isThreadMode ? '' : primaryInput,
				directRepos: isThreadMode ? [] : directRepos,
				selectedAliases: aliasesToCreate,
				worksetName: isThreadMode ? (worksetName ?? undefined) : undefined,
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
			error = formatWorkspaceActionError(
				err,
				mode === 'create-thread' ? 'Failed to create thread.' : 'Failed to create workspace.',
			);
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
			const workspaceName = await renameWorkspaceAction(
				{
					workspaceId: workspace.id,
					workspaceName: nextName,
				},
				{
					loadWorkspaces,
					getActiveWorkspaceId: () => get(activeWorkspaceId),
					selectWorkspace,
				},
			);
			success = `Renamed to ${workspaceName}.`;
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

		if (!hasSource && !hasAliases) {
			error = 'Provide a repo URL/path or select repositories.';
			return;
		}

		const displayName = worksetName?.trim() || workspace.name;
		const targetIds = targetWorkspaceIds.length > 0 ? targetWorkspaceIds : [workspace.id];
		const workspaceById = new Map($workspaces.map((entry) => [entry.id, entry]));
		const targetWorkspaces = targetIds
			.map((id) => workspaceById.get(id))
			.filter((entry): entry is Workspace => entry !== undefined);
		if (targetWorkspaces.length === 0) {
			error = 'Unable to locate threads for this workset.';
			return;
		}

		const sourceName = source.length > 0 ? deriveRepoName(source) || source : '';
		const aliasSelections = Array.from(selectedAliases);
		let mutatedCount = 0;

		loading = true;
		error = null;
		success = null;
		warnings = [];
		pendingHooks = [];
		hookRuns = [];
		hookWorkspaceId = targetWorkspaces[0]?.id ?? workspace.id;
		({ activeHookOperation, activeHookWorkspace, hookRuns, pendingHooks } = beginHookTracking(
			'repo.add',
			displayName,
		));
		try {
			let itemCount = 0;
			const warningsBucket: string[] = [];
			const pendingBucket: WorkspaceActionPendingHook[] = [];
			const hookRunBucket: HookExecution[] = [];

			for (const target of targetWorkspaces) {
				const existingNames = new Set(target.repos.map((repoEntry) => repoEntry.name));
				const sourceForTarget =
					sourceName.length > 0 && !existingNames.has(sourceName) ? source : '';
				const aliasesForTarget = new Set(
					aliasSelections.filter((aliasName) => !existingNames.has(aliasName)),
				);

				if (!sourceForTarget && aliasesForTarget.size === 0) {
					continue;
				}

				mutatedCount += 1;
				const result = await workspaceActionMutations.addItems({
					workspaceId: target.id,
					source: sourceForTarget,
					selectedAliases: aliasesForTarget,
				});
				itemCount += result.itemCount;
				warningsBucket.push(...result.warnings);
				pendingBucket.push(...result.pendingHooks);
				hookRunBucket.push(...result.hookRuns);
			}

			if (mutatedCount === 0) {
				error = 'Selected repositories are already present in every thread for this workset.';
				return;
			}

			hookRuns = appendHookRuns(hookRuns, hookRunBucket);
			pendingHooks = pendingBucket.map((pending) => ({ ...pending }));

			await loadWorkspaces(true);
			warnings = Array.from(new Set(warningsBucket));
			const transition = resolveMutationHookTransition({
				action: 'added',
				workspaceName: displayName,
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

	const handleBrowse = async (): Promise<void> => {
		try {
			const path = await browseWorkspaceActionDirectory(addSource);
			if (path) addSource = path;
		} catch (err) {
			error = formatWorkspaceActionError(err, 'Failed to open directory picker.');
		}
	};

	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const handleCreateBrowse = async (): Promise<void> => {
		try {
			const path = await browseWorkspaceActionDirectory(primaryInput);
			if (path) primaryInput = path;
		} catch (err) {
			error = formatWorkspaceActionError(err, 'Failed to open directory picker.');
		}
	};

	const handleArchive = async (): Promise<void> => {
		if (!workspace) return;
		loading = true;
		error = null;
		success = null;
		warnings = [];
		try {
			await archiveWorkspaceAction(
				{
					workspaceId: workspace.id,
					reason: archiveReason.trim(),
				},
				{
					loadWorkspaces,
					getActiveWorkspaceId: () => get(activeWorkspaceId),
					clearWorkspace,
				},
			);
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
			{pendingHooks}
			onRunPendingHook={handleRunPendingHook}
			onTrustPendingHook={handleTrustPendingHook}
		/>

		<WorkspaceActionFormContent
			{mode}
			{loading}
			{aliasItems}
			{searchQuery}
			{filteredAliases}
			{selectedAliases}
			{getAliasSource}
			{renameName}
			onRenameNameInput={(value) => (renameName = value)}
			onRenameSubmit={handleRename}
			{addSource}
			{existingRepos}
			{addRepoSelectedItems}
			{addRepoTotalItems}
			worksetName={workspace?.name ?? ''}
			onAddSearchQueryInput={(value) => (searchQuery = value)}
			onAddSourceInput={(value) => (addSource = value)}
			onAddBrowse={handleBrowse}
			onAddToggleAlias={toggleAlias}
			onAddRemoveAlias={removeAlias}
			onAddSubmit={handleAddItems}
			createWorkspaceName={customizeName}
			createWorksetLabel={worksetName}
			createSourceInput={primaryInput}
			createDirectRepos={directRepos}
			createThreadHookRows={threadHookRows}
			createThreadHooksLoading={threadHooksLoading}
			createThreadHooksError={threadHooksError}
			onCreateWorkspaceNameInput={(value) => (customizeName = value)}
			onCreateSearchQueryInput={(value) => (searchQuery = value)}
			onCreateSourceInput={(value) => (primaryInput = value)}
			onCreateAddDirectRepo={addDirectRepo}
			onCreateRemoveDirectRepo={removeDirectRepo}
			onCreateToggleDirectRepoRegister={toggleDirectRepoRegister}
			onCreateToggleAlias={toggleAlias}
			onCreateSubmit={handleCreate}
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
