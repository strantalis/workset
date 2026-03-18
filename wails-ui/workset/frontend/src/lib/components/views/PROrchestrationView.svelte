<script lang="ts">
	// prettier-ignore
	import { AlertCircle, CheckCircle2, ChevronRight, Circle, FileCode, GitCommit, GitPullRequest, Loader2, MessageSquare, Upload, XCircle } from '@lucide/svelte';
	import { Browser } from '@wailsio/runtime';
	// prettier-ignore
	import type { PullRequestCreated, PullRequestReviewComment, PullRequestStatusResult, RepoFileDiff, RepoDiffFileSummary, RepoDiffSummary, Workspace } from '../../types';
	// prettier-ignore
	import { fetchPullRequestReviews, fetchPullRequestStatus, fetchRepoLocalStatus, fetchTrackedPullRequest, listRemotes, replyToReviewComment, resolveReviewThread, startCommitAndPushAsync } from '../../api/github';
	// prettier-ignore
	import type { GitHubOperationStage, GitHubOperationStatus, RepoLocalStatus } from '../../api/github';
	import { subscribeGitHubOperationEvent } from '../../githubOperationService';
	import { deleteReviewComment, editReviewComment } from '../../api/github/review';
	import { fetchCurrentGitHubUser } from '../../api/github/user';
	import { buildLineAnnotations } from '../repo-diff/annotations';
	import type { DiffLineAnnotation, ReviewAnnotation } from '../repo-diff/annotations';
	import { createReviewAnnotationActionsController } from '../repo-diff/reviewAnnotationActions';
	// prettier-ignore
	import type { FileDiffRenderOptions, FileDiffRenderer, FileDiffRendererModule } from '../repo-diff/diffRenderController';
	import { buildDiffRenderOptions } from '../repo-diff/diffRenderOptions';
	import RepoDiffAnnotationStyles from '../repo-diff/RepoDiffAnnotationStyles.svelte';
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
	import { EVENT_REPO_DIFF_PR_STATUS } from '../../events';
	import { subscribeRepoDiffEvent } from '../../repoDiffService';
	import { refreshWorkspacesStatus } from '../../state';
	// prettier-ignore
	import { buildSummaryLocalCacheKey, buildSummaryPrCacheKey, repoDiffCache } from '../../cache/repoDiffCache';
	import { resolveBranchRefs } from '../../diff/branchRefs';
	import ResizablePanel from '../ui/ResizablePanel.svelte';
	import PROrchestrationChecksPanel from './PROrchestrationChecksPanel.svelte';
	import PROrchestrationActiveHeader from './PROrchestrationActiveHeader.svelte';
	import PROrchestrationReadyDetail from './PROrchestrationReadyDetail.svelte';
	import PROrchestrationSidebar from './PROrchestrationSidebar.svelte';
	import { mapWorkspaceToPrItems } from '../../view-models/prViewModel';
	import { buildCheckStats, buildDiffTargetKey } from './prOrchestrationHelpers';
	import {
		applyPrStatusEvent,
		applyTrackedPrCreated,
		buildReadyViewSyntheticPr,
		buildFileDiffCacheKeyForSource,
		commitPushStageLabel as formatCommitPushStageLabel,
		createPrViewInteractionHandlers,
		createTrackedPrMapCoordinator,
		createTrackedPrStateReconciler,
		getPullRequestFeedbackCounts,
		hasTrackedPrMetadataChanged,
		refreshReadyDetail,
		persistSidebarCollapsed,
		readSidebarCollapsed,
		shouldClearSelectedItem,
		withTrackedPr,
		type RepoDiffPrStatusEvent,
	} from './prOrchestrationView.helpers';

	interface Props {
		workspace: Workspace | null;
		focusRepoId?: string | null;
		focusToken?: number;
	}

	const { workspace, focusRepoId = null, focusToken = 0 }: Props = $props();
	const prItems = $derived(mapWorkspaceToPrItems(workspace));
	const PR_STATUS_SYNC_INTERVAL_MS = 8000;
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
	let activeTab: 'overview' | 'files' | 'checks' = $state('overview');
	let trackedPr: PullRequestCreated | null = $state(null),
		trackedPrLoading = $state(false),
		trackedPrRequestId = 0;
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
		fileDiffRequestId = 0;
	let prStatus: PullRequestStatusResult | null = $state(null),
		prStatusLoading = $state(false),
		prStatusRequestId = 0;
	let prReviews: PullRequestReviewComment[] = $state([]),
		prReviewsLoading = $state(false),
		currentUserId: number | null = $state(null);
	let prComposerItemId: string | null = $state(null);
	let prComposerMode: 'pull_request' | 'local_merge' = $state('pull_request');
	let repoLocalStatus: RepoLocalStatus | null = $state(null),
		commitPushLoading = $state(false),
		commitPushRepoId: string | null = $state(null),
		commitPushStage: GitHubOperationStage | null = $state(null),
		commitPushError: string | null = $state(null),
		commitPushSuccess = $state(false);
	let commitPushSuccessTimer: ReturnType<typeof setTimeout> | null = null;
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
	const feedbackCounts = $derived.by(() =>
		getPullRequestFeedbackCounts(trackedPr, prStatus, selectedItem),
	);

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
		isReadyDetail = $derived.by(() => viewMode === 'ready' && selectedItem != null),
		checkStats = $derived(buildCheckStats(prStatus));

	const shouldSplitLocalPendingSection = $derived.by(
		() => activePrBranches !== null && (localSummary?.files.length ?? 0) > 0,
	);

	const clearCommitPushSuccessTimer = (): void =>
		void (commitPushSuccessTimer != null && clearTimeout(commitPushSuccessTimer),
		(commitPushSuccessTimer = null));

	const pushStatusVisible = $derived.by(
		() =>
			trackedPr != null &&
			isActiveDetail &&
			!isMergedTrackedPr(trackedPr) &&
			trackedPr.state.toLowerCase() === 'open' &&
			repoLocalStatus != null,
	);

	const pushDisabled = $derived.by(
		() =>
			commitPushLoading ||
			!repoLocalStatus ||
			(!repoLocalStatus.hasUncommitted && repoLocalStatus.ahead === 0),
	);

	const commitPushStageLabel = $derived(formatCommitPushStageLabel(commitPushStage));

	const annotationController = createReviewAnnotationActionsController({
		document,
		workspaceId: () => workspace?.id ?? '',
		repoId: () => selectedItem?.repoId ?? '',
		prNumberInput: () => String(trackedPr?.number ?? ''),
		prBranchInput: () => selectedItem?.branch ?? '',
		parseNumber: (v) => {
			const n = Number.parseInt(v, 10);
			return Number.isNaN(n) ? undefined : n;
		},
		getCurrentUserId: () => currentUserId,
		getPrReviews: () => prReviews,
		setPrReviews: (v) => {
			prReviews = v;
		},
		replyToReviewComment,
		editReviewComment,
		handleDeleteComment: async (commentId) => {
			if (!workspace || !selectedItem) return;
			await deleteReviewComment(workspace.id, selectedItem.repoId, commentId);
			void loadReviews();
		},
		handleResolveThread: async (threadId, resolve) => {
			if (!workspace || !selectedItem) return;
			await resolveReviewThread(workspace.id, selectedItem.repoId, threadId, resolve);
			void loadReviews();
		},
		formatError: (err, fallback) => (err instanceof Error ? err.message : fallback),
		showAlert: (msg) => {
			fileDiffError = msg;
		},
	});

	const lineAnnotations: DiffLineAnnotation<ReviewAnnotation>[] = $derived.by(() => {
		const src = selectedSource;
		if (src === 'local' || prReviews.length === 0) return [];
		const file = selectedFile;
		if (!file) return [];
		return buildLineAnnotations(prReviews.filter((r) => r.path === file.path));
	});

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
		const requestId = ++trackedPrRequestId;
		const isSelectedRepo = (): boolean => selectedItem?.repoId === repoId;
		const cached = trackedPrMap.get(repoId) ?? null;
		if (isSelectedRepo()) {
			trackedPr = cached;
			trackedPrLoading = true;
		}
		try {
			const resolved = await fetchTrackedPullRequest(wsId, repoId);
			trackedPrMapCoordinator.markResolved(repoId, resolved, cached);
			trackedPrMap = withTrackedPr(trackedPrMap, repoId, resolved);
			if (requestId !== trackedPrRequestId || !isSelectedRepo()) {
				return;
			}
			trackedPr = resolved;
		} catch {
			if (requestId !== trackedPrRequestId || !isSelectedRepo()) {
				return;
			}
			trackedPr = null;
		} finally {
			if (requestId === trackedPrRequestId && isSelectedRepo()) {
				trackedPrLoading = false;
			}
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
		activeTab = viewMode === 'active' ? 'overview' : 'files';
		const item = prItems.find((i) => i.id === itemId);
		trackedPr = item ? (trackedPrMap.get(item.repoId) ?? null) : null;
		trackedPrLoading = false;
		prStatus = null;
		prReviews = [];
		diffSummary = localSummary = fileDiffContent = null;
		selectedFileIdx = 0;
		selectedSource = 'pr';
		fileDiffError = activeFileKey = lastDiffSummaryTargetKey = null;
		activePrBranches = null;
		diffSummaryRequestId += 1;
		localSummaryRequestId += 1;
		fileDiffRequestId += 1;
		repoLocalStatus = null;
		commitPushLoading = commitPushSuccess = false;
		commitPushStage = commitPushError = null;
		clearCommitPushSuccessTimer();

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

	const loadChecks = async (options: { reconcileTracked?: boolean } = {}): Promise<void> => {
		if (!workspace || !selectedItem) return;
		const wsId = workspace.id;
		const repoId = selectedItem.repoId;
		const previousTracked = trackedPrMap.get(repoId) ?? null;
		const requestId = ++prStatusRequestId;
		prStatusLoading = true;
		try {
			const result = await fetchPullRequestStatus(
				wsId,
				repoId,
				previousTracked?.number ?? trackedPr?.number,
				selectedItem.branch,
			);
			if (requestId !== prStatusRequestId || !workspace || !selectedItem) {
				return;
			}
			if (workspace.id !== wsId || selectedItem.repoId !== repoId) {
				return;
			}
			prStatus = result;

			if (
				options.reconcileTracked &&
				hasTrackedPrMetadataChanged(previousTracked, result.pullRequest)
			) {
				await loadTrackedPr(wsId, repoId);
				await refreshWorkspacesStatus(true);
			}
		} catch {
			if (requestId === prStatusRequestId) {
				prStatus = null;
			}
		} finally {
			if (requestId === prStatusRequestId) {
				prStatusLoading = false;
			}
		}
	};
	const handleRefreshChecks = (): void => {
		void loadChecks();
	};

	const loadReviews = async (): Promise<void> => {
		if (!workspace || !selectedItem) return;
		prReviewsLoading = true;
		try {
			prReviews = await fetchPullRequestReviews(
				workspace.id,
				selectedItem.repoId,
				trackedPr?.number,
				selectedItem.branch,
			);
		} catch {
			prReviews = [];
		} finally {
			prReviewsLoading = false;
		}
	};

	const startPushForRepo = async (repoId: string): Promise<void> => {
		if (!workspace || commitPushLoading) return;
		commitPushLoading = true;
		commitPushRepoId = repoId;
		commitPushStage = 'queued';
		commitPushError = null;
		commitPushSuccess = false;
		try {
			await startCommitAndPushAsync(workspace.id, repoId);
			// Event subscription handles progress updates from here
		} catch (err) {
			commitPushLoading = false;
			commitPushRepoId = null;
			commitPushStage = null;
			commitPushError = err instanceof Error ? err.message : 'Failed to start push.';
		}
	};

	const handlePushToPr = async (): Promise<void> => {
		if (!selectedItem) return;
		await startPushForRepo(selectedItem.repoId);
	};

	const handleTrackedPrCreated = (repoId: string, created: PullRequestCreated): void =>
		applyTrackedPrCreated({
			repoId,
			created,
			trackedPrMapCoordinator,
			trackedPrMap,
			setTrackedPrMap: (next) => (trackedPrMap = next),
			setTrackedPr: (next) => (trackedPr = next),
			setViewMode,
			setActiveTab: (tab) => (activeTab = tab),
			refreshWorkspacesStatus: () => void refreshWorkspacesStatus(true),
			loadChecks: () => void loadChecks(),
			loadReviews: () => void loadReviews(),
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
			trackedPr,
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
		if (activeTab === 'checks' && !prStatus && !prStatusLoading && selectedItem) {
			void loadChecks();
		}
	});

	$effect(() => {
		if (viewMode !== 'active' || !workspace || !selectedItem) {
			return;
		}

		let stopped = false;
		let inFlight = false;

		const sync = async (): Promise<void> => {
			if (stopped || inFlight) {
				return;
			}
			inFlight = true;
			try {
				await loadChecks({ reconcileTracked: true });
			} finally {
				inFlight = false;
			}
		};

		void sync();
		const timer = setInterval(() => {
			void sync();
		}, PR_STATUS_SYNC_INTERVAL_MS);

		return () => {
			stopped = true;
			clearInterval(timer);
		};
	});
	$effect(() => {
		if (trackedPr && selectedItem && !prReviewsLoading && prReviews.length === 0) {
			void loadReviews();
			if (!currentUserId && workspace) {
				void fetchCurrentGitHubUser(workspace.id, selectedItem.repoId)
					.then((user) => {
						currentUserId = user.id;
					})
					.catch(() => {
						/* non-fatal */
					});
			}
		}
	});
	$effect(() => {
		const unsub = subscribeGitHubOperationEvent((status: GitHubOperationStatus) => {
			if (!workspace) return;
			if (status.workspaceId !== workspace.id) return;
			if (status.type !== 'commit_push') return;
			if (commitPushRepoId && status.repoId !== commitPushRepoId) return;

			const targetRepoId = status.repoId;
			const selectedMatchesTarget = targetRepoId === selectedRepoId;

			if (status.state === 'running') {
				commitPushLoading = true;
				commitPushRepoId = targetRepoId;
				commitPushStage = status.stage;
				commitPushError = null;
				commitPushSuccess = false;
			} else if (status.state === 'completed') {
				commitPushLoading = false;
				commitPushRepoId = null;
				commitPushStage = null;
				commitPushError = null;
				commitPushSuccess = true;
				// Refresh data after successful push
				void loadRepoLocalStatus(workspace.id, targetRepoId);
				void loadLocalSummary(workspace.id, targetRepoId);
				if (activePrBranches && selectedMatchesTarget) {
					void loadDiffSummary(workspace.id, targetRepoId, trackedPr ?? undefined);
				}
				// Auto-dismiss success after 3s
				clearCommitPushSuccessTimer();
				commitPushSuccessTimer = setTimeout(() => {
					commitPushSuccess = false;
					commitPushSuccessTimer = null;
				}, 3000);
			} else if (status.state === 'failed') {
				commitPushLoading = false;
				commitPushRepoId = null;
				commitPushStage = null;
				commitPushSuccess = false;
				commitPushError = status.error || 'Failed to commit and push.';
			}
		});
		return unsub;
	});

	$effect(() => {
		const unsub = subscribeRepoDiffEvent<RepoDiffPrStatusEvent>(
			EVENT_REPO_DIFF_PR_STATUS,
			(payload) => {
				if (!workspace || !selectedItem) return;
				if (payload.workspaceId !== workspace.id || payload.repoId !== selectedItem.repoId) return;
				const previousTracked = trackedPrMap.get(selectedItem.repoId) ?? null;
				const next = applyPrStatusEvent(payload, selectedItem.repoId, trackedPrMap);
				trackedPrMapCoordinator.markResolved(selectedItem.repoId, next.trackedPr, previousTracked);
				prStatus = next.prStatus;
				trackedPr = next.trackedPr;
				trackedPrMap = next.trackedPrMap;
				if (next.shouldReconcileTrackedPr) {
					void reconcileTrackedPrState(workspace.id, selectedItem.repoId);
				}
			},
		);
		return unsub;
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
		activeTab = 'files';
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
		};
	});

	type ParsedFileDiff = Parameters<FileDiffRenderer<ReviewAnnotation>['render']>[0]['fileDiff'];

	type DiffsModule = FileDiffRendererModule<ReviewAnnotation> & {
		parsePatchFiles: (patch: string) => { files?: ParsedFileDiff[] }[];
	};

	let diffsModule: DiffsModule | null = $state(null);
	let diffContainer: HTMLElement | null = $state(null);
	let diffInstance: FileDiffRenderer<ReviewAnnotation> | null = $state(null);
	let diffRenderContainer: HTMLElement | null = $state(null);
	let diffRenderEpoch = 0;

	const buildDiffOptions = (
		container: HTMLElement | null = diffContainer,
	): FileDiffRenderOptions<ReviewAnnotation> =>
		buildDiffRenderOptions(container?.clientWidth, (a) => annotationController.renderAnnotation(a));

	const ensureDiffsModule = async (): Promise<DiffsModule> => {
		if (diffsModule) return diffsModule;
		diffsModule = (await import('@pierre/diffs')) as unknown as DiffsModule;
		return diffsModule;
	};

	$effect(() => {
		const patch = fileDiffContent?.patch;
		const container = diffContainer;
		const annotations = lineAnnotations;
		if (!patch || !container) return;
		const currentEpoch = ++diffRenderEpoch;

		void ensureDiffsModule().then((mod) => {
			if (currentEpoch !== diffRenderEpoch) return;
			if (!container.isConnected) return;
			if (fileDiffContent?.patch !== patch || diffContainer !== container) {
				return;
			}

			const parsed = mod.parsePatchFiles(patch);
			const fileDiff = parsed[0]?.files?.[0] ?? null;
			if (!fileDiff) return;

			if (diffRenderContainer !== container) {
				diffInstance?.cleanUp();
				diffInstance = null;
				diffRenderContainer = container;
			}

			if (!diffInstance) {
				diffInstance = new mod.FileDiff(buildDiffOptions(container));
			} else {
				diffInstance.setOptions(buildDiffOptions(container));
			}
			if (currentEpoch !== diffRenderEpoch) return;
			if (!container.isConnected) return;
			if (fileDiffContent?.patch !== patch || diffContainer !== container) {
				return;
			}
			try {
				diffInstance.render({
					fileDiff,
					fileContainer: container,
					forceRender: true,
					lineAnnotations: annotations,
				});
			} catch (err) {
				// Guard against DOM races inside @pierre/diffs when container nodes were replaced.
				diffInstance?.cleanUp();
				diffInstance = new mod.FileDiff(buildDiffOptions(container));
				try {
					diffInstance.render({
						fileDiff,
						fileContainer: container,
						forceRender: true,
						lineAnnotations: annotations,
					});
				} catch (innerErr) {
					const renderErr = innerErr instanceof Error ? innerErr : err;
					fileDiffError = renderErr instanceof Error ? renderErr.message : 'Failed to render diff.';
				}
			}
		});
	});

	$effect(() => {
		return () => {
			diffInstance?.cleanUp();
			diffInstance = null;
			diffRenderContainer = null;
		};
	});

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
					<PROrchestrationActiveHeader
						{trackedPr}
						{trackedPrLoading}
						{selectedItem}
						workspaceName={workspace.name}
						trackedTitle={trackedPrMap.get(selectedItem.repoId)?.title ?? selectedItem.title}
						{checkStats}
						activeTab={activeTab === 'checks' ? 'overview' : activeTab}
						filesCount={filesForDetail.length || selectedItem.dirtyFiles}
						onActiveTabChange={(tab) => (activeTab = tab)}
						onOpenExternalUrl={openExternalUrl}
					/>

					<div class="tab-content">
						{#if activeTab === 'overview'}
							<div class="overview-panel">
								<div class="ov-main">
									{#if trackedPr?.body}
										<div class="ov-section">
											<div class="ov-section-head">Description</div>
											<div class="ov-description">{trackedPr.body}</div>
										</div>
									{/if}

									{#if pushStatusVisible}
										<div class="pr-push-bar">
											<div class="psb-stats">
												{#if commitPushSuccess}
													<span class="psb-stat psb-success">
														<CheckCircle2 size={12} />
														Pushed successfully
													</span>
												{:else if commitPushError}
													<span class="psb-stat psb-error">
														<AlertCircle size={12} />
														{commitPushError}
													</span>
												{:else if repoLocalStatus && (repoLocalStatus.ahead > 0 || repoLocalStatus.hasUncommitted)}
													{#if repoLocalStatus.ahead > 0}
														<span class="psb-stat">
															<GitCommit size={12} />
															{repoLocalStatus.ahead} unpushed commit{repoLocalStatus.ahead !== 1
																? 's'
																: ''}
														</span>
													{/if}
													{#if repoLocalStatus.hasUncommitted}
														<span class="psb-stat">
															<FileCode size={12} />
															{localSummary?.files.length ?? '?'} dirty file{(localSummary?.files
																.length ?? 0) !== 1
																? 's'
																: ''}
														</span>
													{/if}
												{:else}
													<span class="psb-stat psb-up-to-date">
														<CheckCircle2 size={12} />
														Up to date
													</span>
												{/if}
											</div>
											<button
												type="button"
												class="psb-push-btn"
												disabled={pushDisabled}
												onclick={() => void handlePushToPr()}
											>
												{#if commitPushLoading}
													<Loader2 size={14} class="spin" />
													{commitPushStageLabel ?? 'Pushing...'}
												{:else}
													<Upload size={14} />
													Push to PR
												{/if}
											</button>
										</div>
									{/if}

									<div class="ov-section">
										<div class="ov-section-head">
											Files Changed ·
											<span class="ov-section-count"
												>{filesForDetail.length || selectedItem.dirtyFiles} files</span
											>
											{#if totalAdd > 0 || totalDel > 0}
												<span class="ov-stat-plus">+{totalAdd}</span>
												<span class="ov-stat-minus">-{totalDel}</span>
											{/if}
										</div>
										<div class="ov-file-list">
											{#if diffSummaryLoading}
												<div class="ov-file-loading">Loading files...</div>
											{:else}
												{#each filesForDetail as file, i (file.path)}
													<button
														type="button"
														class="ov-file-row"
														onclick={() => {
															selectedSource = 'pr';
															selectedFileIdx = i;
															activeTab = 'files';
														}}
													>
														<FileCode size={11} class="ov-file-icon" />
														<span class="ov-file-path">{file.path}</span>
														<span class="ov-file-add">+{file.added}</span>
														{#if file.removed > 0}
															<span class="ov-file-del">-{file.removed}</span>
														{/if}
													</button>
												{/each}
											{/if}
										</div>
									</div>
								</div>

								<div class="ov-sidebar">
									<div class="ov-sidebar-section">
										<div class="ov-checks-header">
											{#if checkStats.total === 0}
												<Circle size={12} class="ov-check-icon-neutral" />
												<span>No checks</span>
											{:else if checkStats.failed > 0}
												<XCircle size={12} class="ov-check-icon-fail" />
												<span>Checks failing</span>
											{:else if checkStats.pending > 0}
												<AlertCircle size={12} class="ov-check-icon-pending" />
												<span>Checks running</span>
											{:else}
												<CheckCircle2 size={12} class="ov-check-icon-pass" />
												<span>All checks passing</span>
											{/if}
										</div>
										{#if prStatus?.checks}
											<div class="ov-checks-list">
												{#each prStatus.checks as check (check.name)}
													<div class="ov-check-row">
														{#if check.conclusion === 'success'}
															<CheckCircle2 size={11} class="ov-check-icon-pass" />
														{:else if check.conclusion === 'failure'}
															<XCircle size={11} class="ov-check-icon-fail" />
														{:else}
															<AlertCircle size={11} class="ov-check-icon-pending" />
														{/if}
														<span class="ov-check-name">{check.name}</span>
													</div>
												{/each}
											</div>
										{/if}
									</div>

									<div class="ov-sidebar-divider"></div>

									<div class="ov-sidebar-section">
										<div class="ov-stats">
											{#if feedbackCounts.reviewComments > 0}
												<div class="ov-stat-row">
													<span>Review feedback</span>
													<span class="ov-stat-value">
														<MessageSquare size={9} />
														{feedbackCounts.reviewComments}
													</span>
												</div>
											{/if}
											{#if feedbackCounts.conversationComments > 0}
												<div class="ov-stat-row">
													<span>Comments</span>
													<span class="ov-stat-value">
														<MessageSquare size={9} />
														{feedbackCounts.conversationComments}
													</span>
												</div>
											{/if}
											<div class="ov-stat-row">
												<span>Files changed</span>
												<span class="ov-stat-value"
													>{filesForDetail.length || selectedItem.dirtyFiles}</span
												>
											</div>
											{#if totalAdd > 0 || totalDel > 0}
												<div class="ov-stat-row">
													<span>Additions</span>
													<span class="ov-stat-value ov-stat-plus">+{totalAdd}</span>
												</div>
												<div class="ov-stat-row">
													<span>Deletions</span>
													<span class="ov-stat-value ov-stat-minus">-{totalDel}</span>
												</div>
											{/if}
										</div>
									</div>
								</div>
							</div>
						{:else if activeTab === 'files'}
							<div class="files-panel">
								<div class="fp-sidebar">
									<div class="fp-sidebar-head">
										{shouldSplitLocalPendingSection ? 'PR Files' : 'Changed Files'}
									</div>
									<div class="fp-file-list">
										{#if diffSummaryLoading}
											<div class="fp-loading">Loading files...</div>
										{:else if (diffSummary?.files.length ?? 0) > 0}
											{#each diffSummary?.files ?? [] as file, i (file.path)}
												<button
													type="button"
													class="fp-file"
													class:active={selectedSource === 'pr' && i === selectedFileIdx}
													onclick={() => {
														selectedSource = 'pr';
														selectedFileIdx = i;
													}}
												>
													<FileCode size={14} />
													<span class="fp-file-name">{file.path}</span>
												</button>
											{/each}
										{:else if !shouldSplitLocalPendingSection && selectedRepo}
											{#each selectedRepo.files as file, i (file.path)}
												<button
													type="button"
													class="fp-file"
													class:active={selectedSource === 'pr' && i === selectedFileIdx}
													onclick={() => {
														selectedSource = 'pr';
														selectedFileIdx = i;
													}}
												>
													<FileCode size={14} />
													<span class="fp-file-name">{file.path}</span>
												</button>
											{/each}
										{:else if !shouldSplitLocalPendingSection}
											<div class="fp-loading">No files</div>
										{/if}
									</div>
									{#if shouldSplitLocalPendingSection}
										<div class="fp-divider"></div>
										<div class="fp-sidebar-head fp-local-head">Local Pending Changes</div>
										<div class="fp-file-list">
											{#each localSummary?.files ?? [] as file, i (file.path)}
												<button
													type="button"
													class="fp-file"
													class:active={selectedSource === 'local' && i === selectedFileIdx}
													onclick={() => {
														selectedSource = 'local';
														selectedFileIdx = i;
													}}
												>
													<FileCode size={14} />
													<span class="fp-file-name">{file.path}</span>
												</button>
											{/each}
										</div>
									{/if}
								</div>
								<div class="fp-diff">
									{#if filesForDetail[selectedFileIdx]}
										{@const activeFile = filesForDetail[selectedFileIdx]}
										<div class="diff-card">
											<div class="diff-header">
												<span>{activeFile.path}</span>
												<span>
													{#if activeFile.added > 0}<span class="text-green"
															>+{activeFile.added}</span
														>{/if}
													{#if activeFile.removed > 0}
														<span class="text-red">-{activeFile.removed}</span>{/if}
												</span>
											</div>
											<div class="diff-body">
												{#if fileDiffError}
													<div class="diff-placeholder">
														<AlertCircle size={20} />
														<p>{fileDiffError}</p>
													</div>
												{:else if fileDiffContent?.binary}
													<div class="diff-placeholder">
														<FileCode size={24} />
														<p>Binary file</p>
													</div>
												{:else if fileDiffContent?.patch}
													<div class="diff-renderer-wrap">
														<div class="diff-renderer">
															<diffs-container bind:this={diffContainer}></diffs-container>
														</div>
														{#if fileDiffLoading}
															<div class="diff-loading-overlay">
																<Loader2 size={18} class="spin" />
																<p>Refreshing diff...</p>
															</div>
														{/if}
													</div>
													{#if fileDiffContent.truncated}
														<div class="diff-truncated">
															Diff truncated ({fileDiffContent.totalLines} total lines)
														</div>
													{/if}
												{:else if fileDiffLoading}
													<div class="diff-placeholder">
														<Loader2 size={20} class="spin" />
														<p>Loading diff...</p>
													</div>
												{:else}
													<div class="diff-placeholder">
														<FileCode size={24} />
														<p>No diff content</p>
													</div>
												{/if}
											</div>
										</div>
									{:else}
										<div class="diff-placeholder full">
											<FileCode size={24} />
											<p>Select a file to view its diff</p>
										</div>
									{/if}
								</div>
							</div>
						{:else if activeTab === 'checks'}
							<PROrchestrationChecksPanel
								{prStatusLoading}
								{prStatus}
								onRefreshChecks={handleRefreshChecks}
								onOpenExternalUrl={openExternalUrl}
							/>
						{/if}
					</div>
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
						{commitPushLoading}
						{commitPushRepoId}
						onPushFromSidebar={handlePushFromSidebar}
						onPullRequestCreated={(created) => {
							handlePullRequestCreated(selectedItem.repoId, created);
						}}
						onRefreshReadyState={() =>
							refreshReadyDetail({
								workspace,
								selectedItem,
								trackedPr,
								refreshWorkspacesStatus: () => refreshWorkspacesStatus(true),
								loadRepoLocalStatus,
								loadLocalSummary,
								loadDiffSummary,
							})}
						bind:diffContainer
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
				<aside class="sidebar-collapsed">
					<button
						type="button"
						class="sidebar-toggle-btn expanded"
						aria-label="Expand sidebar"
						title="Expand sidebar"
						onclick={toggleSidebar}
					>
						<ChevronRight size={14} />
					</button>
				</aside>
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
	<RepoDiffAnnotationStyles />
</div>

<style src="./PROrchestrationView.css"></style>
