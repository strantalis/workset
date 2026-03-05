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
	import {
		evaluateHookTransition,
		workspaceActionMutations,
	} from '../services/workspaceActionService';
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
	const inlineThreadHookResults = $derived(mode === 'create-thread' && phase === 'form');

	const queueModalAutoClose = (delayMs = 1500): void => {
		if (autoCloseTimer) return;
		autoCloseTimer = setTimeout(() => {
			autoCloseTimer = null;
			onClose();
		}, delayMs);
	};

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

	type CreateWorkspacePlan = {
		isThreadMode: boolean;
		threadRepos: string[];
		aliasesToCreate: Set<string>;
		directReposForMutation: WorkspaceActionDirectRepo[];
	};

	const buildCreateWorkspacePlan = (): CreateWorkspacePlan => {
		const isThreadMode = mode === 'create-thread';
		const threadRepos = Array.from(
			new Set(
				worksetRepos.map((repoName) => repoName.trim()).filter((repoName) => repoName.length > 0),
			),
		);
		const aliasesToCreate = isThreadMode ? new Set(threadRepos) : selectedAliases;
		return {
			isThreadMode,
			threadRepos,
			aliasesToCreate,
			directReposForMutation: isThreadMode ? [] : directRepos,
		};
	};

	const getCreateValidationError = (): string | null => {
		if (!finalName) {
			return mode === 'create-thread' ? 'Enter a thread name.' : 'Enter a workset name.';
		}
		return null;
	};

	const applyMutationTransition = (
		transition: ReturnType<typeof resolveMutationHookTransition>,
	): void => {
		success = transition.success;
		hookResultContext = transition.hookResultContext;
		phase = transition.phase;
		if (transition.shouldClose) {
			onClose();
			return;
		}
		if (transition.shouldAutoClose) {
			queueModalAutoClose();
		}
	};

	$effect(() => {
		if (!inlineThreadHookResults || loading || !success) return;
		if (pendingHooks.length === 0 && hookRuns.length === 0) return;

		const transition = evaluateHookTransition({
			warnings,
			pendingHooks,
			hookRuns,
		});
		if (!transition.shouldAutoClose) return;
		queueModalAutoClose();
	});

	const handleCreate = async (): Promise<void> => {
		const plan = buildCreateWorkspacePlan();
		const validationError = getCreateValidationError();
		if (validationError) {
			error = validationError;
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
				primaryInput: plan.isThreadMode ? '' : primaryInput,
				directRepos: plan.directReposForMutation,
				selectedAliases: plan.aliasesToCreate,
				worksetName: plan.isThreadMode ? (worksetName ?? undefined) : undefined,
				worksetOnly: !plan.isThreadMode,
			});

			hookRuns = appendHookRuns(hookRuns, result.hookRuns);
			pendingHooks = result.pendingHooks.map((pending) => ({ ...pending }));
			hookWorkspaceId = result.workspaceName;
			await loadWorkspaces(true);
			if (plan.isThreadMode) {
				selectWorkspace(result.workspaceName);
			} else {
				clearWorkspace();
			}
			warnings = result.warnings;
			applyMutationTransition(
				resolveMutationHookTransition({
					action: 'created',
					workspaceName: result.workspaceName,
					warnings,
					pendingHooks,
					hookRuns,
					inlineResults: plan.isThreadMode,
				}),
			);
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

	type AddItemsPlan =
		| { ok: false; error: string }
		| {
				ok: true;
				mode: 'threads' | 'workset';
				displayName: string;
				worksetName: string;
				source: string;
				targetWorkspaces: Workspace[];
				aliasSelections: string[];
		  };

	const buildAddItemsPlan = (): AddItemsPlan => {
		const source = addSource.trim();
		const hasSource = source.length > 0;
		const hasAliases = selectedAliases.size > 0;
		if (!hasSource && !hasAliases) {
			return { ok: false, error: 'Provide a repo URL/path or select repositories.' };
		}

		const targetIds =
			targetWorkspaceIds.length > 0 ? targetWorkspaceIds : workspace ? [workspace.id] : [];
		const workspaceById = new Map($workspaces.map((entry) => [entry.id, entry]));
		const resolvedTargets = targetIds
			.map((id) => workspaceById.get(id))
			.filter((entry): entry is Workspace => entry !== undefined);
		const targetWorkspaces = resolvedTargets.filter((entry) => entry.placeholder !== true);
		const normalizedWorksetName = (
			worksetName ??
			workspace?.worksetLabel ??
			workspace?.name ??
			''
		).trim();
		if (targetWorkspaces.length === 0 && !normalizedWorksetName) {
			return { ok: false, error: 'Unable to locate workset.' };
		}
		const displayName = targetWorkspaces[0]?.name ?? normalizedWorksetName;
		if (!displayName) {
			return { ok: false, error: 'Unable to locate workset.' };
		}

		return {
			ok: true,
			mode: targetWorkspaces.length > 0 ? 'threads' : 'workset',
			displayName,
			worksetName: normalizedWorksetName || displayName,
			source,
			targetWorkspaces,
			aliasSelections: Array.from(selectedAliases),
		};
	};

	type AddItemsResultBucket = {
		itemCount: number;
		mutatedCount: number;
		warnings: string[];
		pendingHooks: WorkspaceActionPendingHook[];
		hookRuns: HookExecution[];
	};

	const runAddItemsMutations = async (
		targetWorkspaces: Workspace[],
		source: string,
		aliasSelections: string[],
	): Promise<AddItemsResultBucket> => {
		const sourceName = source.length > 0 ? deriveRepoName(source) || source : '';
		let itemCount = 0;
		let mutatedCount = 0;
		const warnings: string[] = [];
		const pendingHooks: WorkspaceActionPendingHook[] = [];
		const hookRuns: HookExecution[] = [];

		for (const target of targetWorkspaces) {
			const existingNames = new Set(target.repos.map((repoEntry) => repoEntry.name));
			const sourceForTarget = sourceName.length > 0 && !existingNames.has(sourceName) ? source : '';
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
			warnings.push(...result.warnings);
			pendingHooks.push(...result.pendingHooks);
			hookRuns.push(...result.hookRuns);
		}

		return { itemCount, mutatedCount, warnings, pendingHooks, hookRuns };
	};

	const handleAddItems = async (): Promise<void> => {
		const plan = buildAddItemsPlan();
		if (!plan.ok) {
			error = plan.error;
			return;
		}

		loading = true;
		error = null;
		success = null;
		warnings = [];
		pendingHooks = [];
		hookRuns = [];
		hookWorkspaceId = plan.mode === 'threads' ? (plan.targetWorkspaces[0]?.id ?? null) : null;
		({ activeHookOperation, activeHookWorkspace, hookRuns, pendingHooks } = beginHookTracking(
			'repo.add',
			plan.displayName,
		));
		try {
			const result =
				plan.mode === 'threads'
					? await runAddItemsMutations(plan.targetWorkspaces, plan.source, plan.aliasSelections)
					: {
							mutatedCount: 1,
							...(await workspaceActionMutations.addReposToWorkset({
								worksetName: plan.worksetName,
								source: plan.source,
								selectedAliases: new Set(plan.aliasSelections),
							})),
						};
			if (result.itemCount === 0 || result.mutatedCount === 0) {
				error =
					plan.mode === 'threads'
						? 'Selected repositories are already present in every thread for this workset.'
						: 'Selected repositories are already present in this workset.';
				return;
			}

			hookRuns = appendHookRuns(hookRuns, result.hookRuns);
			pendingHooks = result.pendingHooks.map((pending) => ({ ...pending }));

			await loadWorkspaces(true);
			warnings = Array.from(new Set(result.warnings));
			applyMutationTransition(
				resolveMutationHookTransition({
					action: 'added',
					workspaceName: plan.displayName,
					itemCount: result.itemCount,
					warnings,
					pendingHooks,
					hookRuns,
				}),
			);
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
			{hookRuns}
			{pendingHooks}
			showHooks={!inlineThreadHookResults}
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

		{#if inlineThreadHookResults && (hookRuns.length > 0 || pendingHooks.length > 0)}
			<WorkspaceActionStatusAlerts
				{error}
				{success}
				{warnings}
				{hookRuns}
				{pendingHooks}
				showMessages={false}
				showHooks={true}
				onRunPendingHook={handleRunPendingHook}
				onTrustPendingHook={handleTrustPendingHook}
			/>
		{/if}
	{/if}
</Modal>
