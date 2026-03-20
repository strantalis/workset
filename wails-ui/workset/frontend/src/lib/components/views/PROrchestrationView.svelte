<script lang="ts">
	// prettier-ignore
	import { GitPullRequest, Upload } from '@lucide/svelte';
	import { Browser } from '@wailsio/runtime';
	// prettier-ignore
	import type { PullRequestCreated, RepoFileDiff, RepoDiffFileSummary, RepoDiffSummary, Workspace } from '../../types';
	// prettier-ignore
	import { fetchRepoLocalStatus, fetchTrackedPullRequest, listRemotes, startCommitAndPushAsync } from '../../api/github';
	import type { GitHubOperationStatus, RepoLocalStatus } from '../../api/github';
	import { subscribeGitHubOperationEvent } from '../../githubOperationService';
	import {
		EVENT_REPO_DIFF_LOCAL_STATUS,
		EVENT_REPO_DIFF_LOCAL_SUMMARY,
		EVENT_REPO_DIFF_SUMMARY,
	} from '../../events';
	import { subscribeRepoDiffEvent } from '../../repoDiffService';
	import {
		fetchBranchDiffSummary,
		fetchBranchFileDiff,
		fetchRepoDiffSummary,
		fetchRepoFileDiff,
		startRepoDiffWatch,
		startRepoStatusWatch,
		stopRepoDiffWatch,
		stopRepoStatusWatch,
	} from '../../api/repo-diff';
	import { refreshWorkspacesStatus } from '../../state';
	// prettier-ignore
	import { buildSummaryLocalCacheKey, buildSummaryPrCacheKey, repoDiffCache } from '../../cache/repoDiffCache';
	import { resolveBranchRefs } from '../../diff/branchRefs';
	import ResizablePanel from '../ui/ResizablePanel.svelte';
	import PRActiveDetailView from './PRActiveDetailView.svelte';
	import PROrchestrationReadyDetail from './PROrchestrationReadyDetail.svelte';
	import PROrchestrationSidebar from './PROrchestrationSidebar.svelte';
	import { mapWorkspaceToPrItems } from '../../view-models/prViewModel';
	import { buildDiffTargetKey } from './prOrchestrationHelpers';
	import {
		applyTrackedPrCreated,
		buildReadyViewSyntheticPr,
		buildFileDiffCacheKeyForSource,
		createPrViewInteractionHandlers,
		createTrackedPrMapCoordinator,
		createTrackedPrStateReconciler,
		refreshReadyDetail,
		persistSidebarCollapsed,
		readSidebarCollapsed,
		shouldClearSelectedItem,
		withTrackedPr,
	} from './prOrchestrationView.helpers';
	import { createCommitPushController } from '../repo-diff/commitPushController.svelte';

	type RepoDiffSummaryEvent = {
		workspaceId: string;
		repoId: string;
		summary: RepoDiffSummary;
	};

	type RepoDiffLocalStatusEvent = {
		workspaceId: string;
		repoId: string;
		status: RepoLocalStatus;
	};

	interface Props {
		workspace: Workspace | null;
		focusRepoId?: string | null;
		focusToken?: number;
	}

	const { workspace, focusRepoId = null, focusToken = 0 }: Props = $props();
	const prItems = $derived(mapWorkspaceToPrItems(workspace));
	const isMergedTrackedPr = (pr: PullRequestCreated | undefined | null): boolean =>
		Boolean(pr && (pr.merged === true || pr.state.toLowerCase() === 'merged'));
	let trackedPrMap = $state<Map<string, PullRequestCreated>>(new Map());
	const trackedPrMapCoordinator = createTrackedPrMapCoordinator();
	const partitions = $derived.by(() => {
		const active = prItems.filter((item) => {
			const tracked = trackedPrMap.get(item.repoId);
			return tracked != null && !isMergedTrackedPr(tracked);
		});
		const merged = prItems.filter((item) => {
			const tracked = trackedPrMap.get(item.repoId);
			return tracked != null && isMergedTrackedPr(tracked);
		});
		const tracked = [...active, ...merged];
		const readyToPR = prItems.filter(
			(item) => !trackedPrMap.has(item.repoId) && (item.hasLocalDiff || item.ahead > 0),
		);
		return { active, merged, tracked, readyToPR };
	});
	let viewMode: 'active' | 'ready' = $state('active'),
		selectedItemId: string | null = $state(null),
		lastAppliedFocusKey = $state<string | null>(null);
	let diffSummary: RepoDiffSummary | null = $state(null),
		localSummary: RepoDiffSummary | null = $state(null),
		diffSummaryLoading = $state(false),
		selectedFileIdx = $state(0),
		selectedSource = $state<'pr' | 'local'>('pr'),
		fileDiffContent: RepoFileDiff | null = $state(null),
		fileDiffLoading = $state(false),
		fileDiffError: string | null = $state(null);
	let activeWatchKey: { wsId: string; repoId: string; mode: 'local' | 'pr' } | null = $state(null);
	let activePrBranches: { base: string; head: string } | null = $state(null),
		activeFileKey: string | null = $state(null),
		lastDiffSummaryTargetKey: string | null = $state(null),
		diffSummaryRequestId = 0,
		localSummaryRequestId = 0,
		fileDiffRequestId = 0,
		fileDiffRefreshVersion = 0;
	let prComposerItemId: string | null = $state(null);
	let prComposerMode: 'pull_request' | 'local_merge' = $state('pull_request');
	let repoLocalStatus: RepoLocalStatus | null = $state(null);
	const commitPush = createCommitPushController();
	let sidebarCollapsed = $state(readSidebarCollapsed());
	const canCollapseSidebar = $derived(selectedItemId !== null);

	const toggleSidebar = (): void =>
		void (!sidebarCollapsed && !canCollapseSidebar
			? undefined
			: persistSidebarCollapsed((sidebarCollapsed = !sidebarCollapsed)));
	const setViewMode = (mode: 'active' | 'ready'): void =>
		void ((viewMode = mode), mode !== 'ready' && (prComposerItemId = null));
	const resolveTrackedTitle = (repoId: string, fallbackTitle: string): string =>
		trackedPrMap.get(repoId)?.title ?? fallbackTitle;

	const selectedItem = $derived(prItems.find((item) => item.id === selectedItemId) ?? null),
		wsId = $derived(workspace?.id ?? ''),
		selectedRepoId = $derived(selectedItem?.repoId ?? '');

	const selectedRepo = $derived.by(() =>
		!selectedItem || !workspace
			? null
			: (workspace.repos.find((r) => r.id === selectedItem.repoId) ?? null),
	);

	const selectedFile = $derived.by(
		() =>
			(selectedSource === 'local' ? (localSummary?.files ?? []) : (diffSummary?.files ?? []))[
				selectedFileIdx
			] ?? null,
	);

	const selectedKey = $derived.by(() =>
		selectedFile ? `${selectedSource}:${selectedFile.path}:${selectedFile.prevPath ?? ''}` : '',
	);

	const selectedFilePath = $derived(selectedFile?.path ?? ''),
		selectedFilePrevPath = $derived(selectedFile?.prevPath ?? ''),
		selectedFileStatus = $derived(selectedFile?.status ?? ''),
		selectedFileAdded = $derived(selectedFile?.added ?? 0),
		selectedFileRemoved = $derived(selectedFile?.removed ?? 0),
		selectedFileBinary = $derived(selectedFile?.binary ?? false),
		activePrKey = $derived.by(() =>
			activePrBranches ? `${activePrBranches.base}:${activePrBranches.head}` : '',
		);

	const isActiveDetail = $derived.by(() => viewMode === 'active' && selectedItem != null),
		isReadyDetail = $derived.by(() => viewMode === 'ready' && selectedItem != null);

	const shouldSplitLocalPendingSection = $derived.by(
		() => activePrBranches !== null && (localSummary?.files.length ?? 0) > 0,
	);

	const filesForDetail = $derived.by(() => {
		const src = selectedSource;
		return src === 'local' ? (localSummary?.files ?? []) : (diffSummary?.files ?? []);
	});
	const totalAdd = $derived(
		filesForDetail.reduce((s: number, f: RepoDiffFileSummary) => s + f.added, 0),
	);
	const totalDel = $derived(
		filesForDetail.reduce((s: number, f: RepoDiffFileSummary) => s + f.removed, 0),
	);

	// ── Functions ────────────────────────────────────────────────

	const resolvePrBranches = async (
		wsId: string,
		repoId: string,
		pr: PullRequestCreated,
	): Promise<{ base: string; head: string } | null> => {
		if (!pr.baseBranch || !pr.headBranch) {
			return null;
		}
		const fallback = { base: pr.baseBranch, head: pr.headBranch };
		try {
			const remotes = await listRemotes(wsId, repoId);
			return resolveBranchRefs(remotes, pr) ?? fallback;
		} catch {
			return fallback;
		}
	};

	const loadTrackedPr = async (wsId: string, repoId: string): Promise<void> => {
		const cached = trackedPrMap.get(repoId) ?? null;
		try {
			const resolved = await fetchTrackedPullRequest(wsId, repoId);
			trackedPrMapCoordinator.markResolved(repoId, resolved, cached);
			trackedPrMap = withTrackedPr(trackedPrMap, repoId, resolved);
		} catch {
			/* non-fatal */
		}
	};

	const loadRepoLocalStatus = async (wsId: string, repoId: string): Promise<void> => {
		try {
			repoLocalStatus = await fetchRepoLocalStatus(wsId, repoId);
		} catch {
			repoLocalStatus = null;
		}
	};

	const selectItem = (itemId: string): void => {
		if (viewMode === 'ready' && prComposerItemId !== itemId) {
			prComposerItemId = null;
		}
		selectedItemId = itemId;
		const item = prItems.find((i) => i.id === itemId);
		diffSummary = localSummary = fileDiffContent = null;
		selectedFileIdx = 0;
		selectedSource = 'pr';
		fileDiffError = activeFileKey = lastDiffSummaryTargetKey = null;
		activePrBranches = null;
		diffSummaryRequestId += 1;
		localSummaryRequestId += 1;
		fileDiffRequestId += 1;
		repoLocalStatus = null;
		commitPush.reset();

		if (item && workspace) {
			void loadTrackedPr(workspace.id, item.repoId);
			void loadRepoLocalStatus(workspace.id, item.repoId);
		}
	};

	const stopActiveWatch = async (): Promise<void> => {
		if (activeWatchKey) {
			const { wsId, repoId, mode } = activeWatchKey;
			activeWatchKey = null;
			try {
				if (mode === 'pr') {
					await stopRepoDiffWatch(wsId, repoId);
				} else {
					await stopRepoStatusWatch(wsId, repoId);
				}
			} catch {
				/* ignore */
			}
		}
	};

	const loadLocalSummary = async (wsId: string, repoId: string): Promise<void> => {
		const requestId = ++localSummaryRequestId;
		const cacheKey = buildSummaryLocalCacheKey(wsId, repoId);
		const cached = repoDiffCache.getSummary(cacheKey);
		if (cached) {
			localSummary = cached.value;
			if (!cached.stale) return;
		}
		try {
			const fetched = await fetchRepoDiffSummary(wsId, repoId);
			if (requestId !== localSummaryRequestId) return;
			localSummary = fetched;
			repoDiffCache.setSummary(cacheKey, fetched);
		} catch {
			if (requestId !== localSummaryRequestId) return;
			localSummary = cached?.value ?? null;
		}
	};

	const loadDiffSummary = async (
		wsId: string,
		repoId: string,
		pr?: PullRequestCreated,
	): Promise<void> => {
		const requestId = ++diffSummaryRequestId;
		diffSummaryLoading = true;
		const branches = pr ? await resolvePrBranches(wsId, repoId, pr) : null;
		if (requestId !== diffSummaryRequestId) return;
		activePrBranches = branches;
		const cacheKey = branches
			? buildSummaryPrCacheKey(wsId, repoId, branches.base, branches.head)
			: buildSummaryLocalCacheKey(wsId, repoId);
		const cached = repoDiffCache.getSummary(cacheKey);
		if (cached) {
			diffSummary = cached.value;
		}
		try {
			await stopActiveWatch();
			if (!branches) {
				await startRepoStatusWatch(wsId, repoId);
				if (requestId !== diffSummaryRequestId) return;
				activeWatchKey = { wsId, repoId, mode: 'local' };
			} else {
				await startRepoDiffWatch(wsId, repoId, pr?.number, pr?.headBranch || branches.head);
				if (requestId !== diffSummaryRequestId) return;
				activeWatchKey = { wsId, repoId, mode: 'pr' };
			}
			if (!cached || cached.stale) {
				const fetched = branches
					? await fetchBranchDiffSummary(wsId, repoId, branches.base, branches.head)
					: await fetchRepoDiffSummary(wsId, repoId);
				if (requestId !== diffSummaryRequestId) return;
				diffSummary = fetched;
				repoDiffCache.setSummary(cacheKey, fetched);
			}
			if (branches) {
				void loadLocalSummary(wsId, repoId);
			}
		} catch {
			if (requestId !== diffSummaryRequestId) return;
			diffSummary = cached?.value ?? null;
		} finally {
			if (requestId === diffSummaryRequestId) {
				diffSummaryLoading = false;
			}
		}
	};

	const loadFileDiff = async (
		wsId: string,
		repoId: string,
		file: RepoDiffFileSummary,
		source: 'pr' | 'local' = 'pr',
		fileKey: string,
	): Promise<void> => {
		const requestId = ++fileDiffRequestId;
		fileDiffLoading = true;
		fileDiffError = null;
		const cacheKey = buildFileDiffCacheKeyForSource(wsId, repoId, file, source, activePrBranches);
		const cached = repoDiffCache.getFileDiff(cacheKey);
		if (cached) {
			fileDiffContent = cached.value;
			if (!cached.stale) {
				if (requestId === fileDiffRequestId && activeFileKey === fileKey) {
					fileDiffLoading = false;
				}
				return;
			}
		}
		try {
			const fetched =
				source === 'local'
					? await fetchRepoFileDiff(wsId, repoId, file.path, file.prevPath ?? '', file.status ?? '')
					: activePrBranches
						? await fetchBranchFileDiff(
								wsId,
								repoId,
								activePrBranches.base,
								activePrBranches.head,
								file.path,
								file.prevPath ?? '',
							)
						: await fetchRepoFileDiff(
								wsId,
								repoId,
								file.path,
								file.prevPath ?? '',
								file.status ?? '',
							);
			if (requestId !== fileDiffRequestId || activeFileKey !== fileKey) return;
			fileDiffContent = fetched;
			repoDiffCache.setFileDiff(cacheKey, fetched);
		} catch (err) {
			if (requestId !== fileDiffRequestId || activeFileKey !== fileKey) return;
			fileDiffError = err instanceof Error ? err.message : 'Failed to load diff';
			fileDiffContent = null;
		} finally {
			if (requestId === fileDiffRequestId && activeFileKey === fileKey) {
				fileDiffLoading = false;
			}
		}
	};

	const startPushForRepo = async (repoId: string): Promise<void> => {
		if (!workspace || commitPush.loading) return;
		commitPush.start(repoId);
		try {
			await startCommitAndPushAsync(workspace.id, repoId);
		} catch {
			commitPush.reset();
		}
	};

	const handleTrackedPrCreated = (repoId: string, created: PullRequestCreated): void =>
		applyTrackedPrCreated({
			repoId,
			created,
			trackedPrMapCoordinator,
			trackedPrMap,
			setTrackedPrMap: (next) => (trackedPrMap = next),
			setTrackedPr: () => {},
			setViewMode,
			setActiveTab: () => {},
			refreshWorkspacesStatus: () => void refreshWorkspacesStatus(true),
			loadChecks: () => {},
			loadReviews: () => {},
		});
	const { openPrComposer, handlePushFromSidebar, handlePullRequestCreated } =
		createPrViewInteractionHandlers({
			findItem: (itemId) => prItems.find((entry) => entry.id === itemId),
			getViewMode: () => viewMode,
			setViewMode,
			getSelectedItemId: () => selectedItemId,
			getPrComposerItemId: () => prComposerItemId,
			setPrComposerItemId: (itemId) => (prComposerItemId = itemId),
			setPrComposerMode: (mode) => (prComposerMode = mode),
			selectItem,
			startPushForRepo,
			handleTrackedPrCreated,
		});
	const resolveRepoFileSelectionIndex = (itemId: string, filePath: string): number => {
		const item = prItems.find((entry) => entry.id === itemId);
		const repoFiles =
			viewMode === 'ready' && selectedItemId === itemId
				? (diffSummary?.files ?? [])
				: ((workspace?.repos.find((repo) => repo.id === item?.repoId)?.files ?? []) as {
						path: string;
					}[]);
		const index = repoFiles.findIndex((file) => file.path === filePath);
		return index >= 0 ? index : 0;
	};
	const handleSelectRepoFile = (itemId: string, filePath: string): void => {
		const index = resolveRepoFileSelectionIndex(itemId, filePath);
		if (viewMode !== 'ready') setViewMode('ready');
		selectedSource = 'pr';
		if (selectedItemId !== itemId) selectItem(itemId);
		selectedFileIdx = index;
	};

	const openExternalUrl = (url: string | undefined | null): void =>
		void (url && Browser.OpenURL(url));
	const clampSelectedFileIndex = (files: RepoDiffFileSummary[]): void => {
		if (files.length === 0) {
			selectedFileIdx = 0;
			return;
		}
		if (selectedFileIdx >= files.length) {
			selectedFileIdx = files.length - 1;
		}
	};
	const cacheSummaryForSelection = (
		wsId: string,
		repoId: string,
		summary: RepoDiffSummary,
		source: 'pr' | 'local',
	): void => {
		const cacheKey =
			source === 'pr' && activePrBranches
				? buildSummaryPrCacheKey(wsId, repoId, activePrBranches.base, activePrBranches.head)
				: buildSummaryLocalCacheKey(wsId, repoId);
		repoDiffCache.setSummary(cacheKey, summary);
	};
	const refreshSelectedFileDiff = (): void => {
		const currentWsId = wsId;
		const currentRepoId = selectedRepoId;
		const currentFile = selectedFile;
		if (currentWsId === '' || currentRepoId === '' || !currentFile) {
			return;
		}
		const cacheKey = buildFileDiffCacheKeyForSource(
			currentWsId,
			currentRepoId,
			currentFile,
			selectedSource,
			activePrBranches,
		);
		repoDiffCache.deleteFileDiff(cacheKey);
		fileDiffContent = null;
		fileDiffError = null;
		fileDiffRefreshVersion += 1;
	};
	const reconcileTrackedPrState = createTrackedPrStateReconciler({
		loadTrackedPr,
		refreshWorkspacesStatus: () => refreshWorkspacesStatus(true),
		getSelectedRepoId: () => selectedRepoId,
		loadRepoLocalStatus,
		loadDiffSummary,
		getTrackedPr: (repoId) => trackedPrMap.get(repoId),
		getActiveWatchKey: () => activeWatchKey,
		clearActivePrBranches: () => (activePrBranches = null),
		stopActiveWatch,
	});

	// ── Effects ─────────────────────────────────────────────────
	$effect(() => {
		const nextMap = trackedPrMapCoordinator.applyWorkspace(workspace, trackedPrMap);
		if (nextMap !== trackedPrMap) {
			trackedPrMap = nextMap;
		}
	});

	$effect(() => {
		const currentWsId = wsId;
		const currentRepoId = selectedRepoId;
		if (!selectedItem || currentWsId === '' || currentRepoId === '') {
			return;
		}
		const effectivePr = buildReadyViewSyntheticPr({
			viewMode,
			trackedPr: trackedPrMap.get(selectedItem.repoId) ?? null,
			selectedItem,
			selectedRepo,
		});
		const targetKey = buildDiffTargetKey(currentWsId, currentRepoId, effectivePr);
		if (targetKey === lastDiffSummaryTargetKey) {
			return;
		}
		lastDiffSummaryTargetKey = targetKey;
		void loadDiffSummary(currentWsId, currentRepoId, effectivePr);
	});

	$effect(() => {
		const unsub = subscribeGitHubOperationEvent((status: GitHubOperationStatus) => {
			if (!workspace) return;
			if (status.workspaceId !== workspace.id) return;
			commitPush.handleEvent(status, (completedRepoId) => {
				if (!workspace) return;
				void loadRepoLocalStatus(workspace.id, completedRepoId);
				void loadLocalSummary(workspace.id, completedRepoId);
				if (activePrBranches && completedRepoId === selectedRepoId) {
					void loadDiffSummary(
						workspace.id,
						completedRepoId,
						trackedPrMap.get(completedRepoId) ?? undefined,
					);
				}
			});
		});
		return unsub;
	});

	$effect(() => {
		if (!workspace || !selectedItem) return;
		const currentWsId = workspace.id;
		const currentRepoId = selectedItem.repoId;

		const unsubscribers = [
			subscribeRepoDiffEvent<RepoDiffSummaryEvent>(EVENT_REPO_DIFF_SUMMARY, (payload) => {
				if (payload.workspaceId !== currentWsId || payload.repoId !== currentRepoId) return;
				diffSummary = payload.summary;
				cacheSummaryForSelection(currentWsId, currentRepoId, payload.summary, 'pr');
				if (selectedSource === 'pr') {
					clampSelectedFileIndex(payload.summary.files);
					refreshSelectedFileDiff();
				}
			}),
			subscribeRepoDiffEvent<RepoDiffSummaryEvent>(EVENT_REPO_DIFF_LOCAL_SUMMARY, (payload) => {
				if (payload.workspaceId !== currentWsId || payload.repoId !== currentRepoId) return;
				cacheSummaryForSelection(currentWsId, currentRepoId, payload.summary, 'local');
				if (activePrBranches) {
					localSummary = payload.summary;
					if (selectedSource === 'local') {
						clampSelectedFileIndex(payload.summary.files);
						refreshSelectedFileDiff();
					}
					return;
				}
				diffSummary = payload.summary;
				localSummary = payload.summary;
				if (selectedSource === 'pr' || selectedSource === 'local') {
					clampSelectedFileIndex(payload.summary.files);
					refreshSelectedFileDiff();
				}
			}),
			subscribeRepoDiffEvent<RepoDiffLocalStatusEvent>(EVENT_REPO_DIFF_LOCAL_STATUS, (payload) => {
				if (payload.workspaceId !== currentWsId || payload.repoId !== currentRepoId) return;
				repoLocalStatus = payload.status;
			}),
		];

		return () => {
			for (const unsubscribe of unsubscribers) {
				unsubscribe();
			}
		};
	});

	$effect(() => {
		if (
			!shouldClearSelectedItem(
				selectedItemId,
				viewMode,
				prItems,
				partitions.tracked,
				partitions.readyToPR,
			)
		) {
			return;
		}
		selectedItemId = null;
	});

	$effect(() => {
		const repoId = focusRepoId?.trim() ?? '';
		if (!workspace || repoId === '') {
			lastAppliedFocusKey = null;
			return;
		}
		const focusKey = `${workspace.id}:${repoId}:${focusToken}`;
		if (focusKey === lastAppliedFocusKey) {
			return;
		}
		lastAppliedFocusKey = focusKey;

		const target = prItems.find((item) => item.repoId === repoId);
		if (!target) {
			return;
		}

		const nextMode: 'active' | 'ready' = trackedPrMap.has(repoId) ? 'active' : 'ready';
		if (viewMode !== nextMode) {
			viewMode = nextMode;
		}
		queueMicrotask(() => {
			const refreshedTarget = prItems.find((item) => item.repoId === repoId);
			if (!refreshedTarget || selectedItemId === refreshedTarget.id) {
				return;
			}
			selectItem(refreshedTarget.id);
		});
	});

	$effect(() => {
		if (selectedItemId !== null || !sidebarCollapsed) {
			return;
		}
		sidebarCollapsed = false;
		persistSidebarCollapsed(false);
	});

	$effect(() => {
		const currentWsId = wsId;
		const currentRepoId = selectedRepoId;
		const key = selectedKey;
		const path = selectedFilePath;
		void activePrKey;
		void fileDiffRefreshVersion;
		if (currentWsId !== '' && currentRepoId !== '' && key !== '' && path !== '') {
			if (activeFileKey !== key) {
				activeFileKey = key;
				fileDiffContent = null;
				fileDiffError = null;
			}
			void loadFileDiff(
				currentWsId,
				currentRepoId,
				{
					path,
					prevPath: selectedFilePrevPath,
					status: selectedFileStatus,
					added: selectedFileAdded,
					removed: selectedFileRemoved,
					binary: selectedFileBinary,
				},
				selectedSource,
				key,
			);
		} else {
			activeFileKey = null;
			fileDiffContent = null;
			fileDiffError = null;
			fileDiffLoading = false;
		}
	});

	$effect(() => {
		return () => {
			void stopActiveWatch();
			commitPush.destroy();
		};
	});
</script>

<div class="pro">
	{#if !workspace}
		<div class="empty-state ws-empty-state">
			<GitPullRequest size={48} />
			<p class="ws-empty-state-copy">Select a thread to view pull requests</p>
		</div>
	{:else if prItems.length === 0}
		<div class="empty-state ws-empty-state">
			<GitPullRequest size={48} />
			<p class="ws-empty-state-copy">No repositories in this workspace</p>
		</div>
	{:else}
		{#snippet detailPanel()}
			<main class="detail">
				{#if isActiveDetail && selectedItem && workspace}
					<PRActiveDetailView
						{workspace}
						{selectedItem}
						{selectedRepo}
						{trackedPrMap}
						{diffSummary}
						{localSummary}
						{diffSummaryLoading}
						{fileDiffContent}
						{fileDiffLoading}
						{fileDiffError}
						{selectedFileIdx}
						{selectedSource}
						{filesForDetail}
						{totalAdd}
						{totalDel}
						{shouldSplitLocalPendingSection}
						{commitPush}
						{repoLocalStatus}
						onSelectedFileIdxChange={(i) => (selectedFileIdx = i)}
						onSelectedSourceChange={(s) => (selectedSource = s)}
						onStartPush={() => startPushForRepo(selectedItem.repoId)}
						onOpenExternalUrl={openExternalUrl}
						onSetFileDiffError={(err) => (fileDiffError = err)}
						onReconcileTrackedPr={reconcileTrackedPrState}
						onPrStatusEventApplied={(repoId, resolvedPr, previousTracked, updatedMap) => {
							trackedPrMapCoordinator.markResolved(repoId, resolvedPr, previousTracked);
							trackedPrMap = updatedMap;
						}}
					/>
				{:else if isReadyDetail && selectedItem && workspace}
					<PROrchestrationReadyDetail
						{selectedItem}
						workspaceName={workspace.name}
						showCreatePanel={prComposerItemId === selectedItem.id}
						initialMode={prComposerMode}
						workspaceId={workspace.id}
						baseBranch={selectedRepo?.defaultBranch ?? ''}
						{filesForDetail}
						{totalAdd}
						{totalDel}
						{diffSummaryLoading}
						{selectedFileIdx}
						{fileDiffError}
						{fileDiffContent}
						{fileDiffLoading}
						commitPushLoading={commitPush.loading}
						commitPushRepoId={commitPush.repoId}
						onPushFromSidebar={handlePushFromSidebar}
						onPullRequestCreated={(created) => {
							handlePullRequestCreated(selectedItem.repoId, created);
						}}
						onRefreshReadyState={() =>
							refreshReadyDetail({
								workspace,
								selectedItem,
								trackedPr: trackedPrMap.get(selectedItem.repoId) ?? null,
								refreshWorkspacesStatus: () => refreshWorkspacesStatus(true),
								loadRepoLocalStatus,
								loadLocalSummary,
								loadDiffSummary,
							})}
					/>
				{:else}
					<div class="empty-state ws-empty-state">
						{#if viewMode === 'active'}
							<GitPullRequest size={32} strokeWidth={1.5} />
							<p class="ws-empty-state-copy">
								Select a tracked PR to view its diff, checks, and comments
							</p>
						{:else}
							<Upload size={32} strokeWidth={1.5} />
							<p class="ws-empty-state-copy">Select a branch to prepare a PR or local merge</p>
						{/if}
					</div>
				{/if}
			</main>
		{/snippet}

		{#if sidebarCollapsed}
			<div class="sidebar-collapsed-layout">
				<button
					type="button"
					class="ws-panel-edge-tab ws-panel-edge-tab--left ws-panel-edge-tab--inline"
					aria-label="Expand sidebar"
					title="Expand sidebar"
					onclick={toggleSidebar}
				></button>
				{@render detailPanel()}
			</div>
		{:else}
			<ResizablePanel
				direction="horizontal"
				initialRatio={0.3}
				minRatio={0.22}
				maxRatio={0.42}
				storageKey="workset:pr-orchestration:sidebarRatio"
			>
				<PROrchestrationSidebar
					{workspace}
					workspaceName={workspace.name}
					{viewMode}
					{canCollapseSidebar}
					{partitions}
					{prItems}
					{selectedItemId}
					{prComposerItemId}
					{prComposerMode}
					selectedReadyRepoFiles={viewMode === 'ready' ? (diffSummary?.files ?? []) : []}
					selectedFilePath={viewMode === 'ready' ? selectedFilePath : null}
					{resolveTrackedTitle}
					onToggleSidebar={toggleSidebar}
					onViewModeChange={setViewMode}
					onSelectItem={selectItem}
					onSelectRepoFile={handleSelectRepoFile}
					onOpenPrComposer={openPrComposer}
				/>

				{#snippet second()}
					{@render detailPanel()}
				{/snippet}
			</ResizablePanel>
		{/if}
	{/if}
</div>

<style src="./PROrchestrationView.css"></style>
