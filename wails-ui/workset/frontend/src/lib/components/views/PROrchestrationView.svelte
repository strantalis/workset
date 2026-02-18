<script lang="ts">
	import {
		AlertCircle,
		ArrowUpRight,
		Box,
		CheckCircle2,
		ChevronLeft,
		ChevronRight,
		Circle,
		ExternalLink,
		FileCode,
		GitBranch,
		GitCommit,
		GitPullRequest,
		Loader2,
		MessageSquare,
		RefreshCw,
		Terminal,
		Upload,
		XCircle,
	} from '@lucide/svelte';
	import { Browser } from '@wailsio/runtime';
	import type {
		PullRequestCreated,
		PullRequestReviewComment,
		PullRequestStatusResult,
		RepoFileDiff,
		RepoDiffFileSummary,
		RepoDiffSummary,
		Workspace,
	} from '../../types';
	import {
		createPullRequest,
		fetchPullRequestReviews,
		fetchPullRequestStatus,
		fetchRepoLocalStatus,
		fetchTrackedPullRequest,
		generatePullRequestText,
		listRemotes,
		replyToReviewComment,
		resolveReviewThread,
		startCommitAndPushAsync,
	} from '../../api/github';
	import type {
		GitHubOperationStage,
		GitHubOperationStatus,
		RepoLocalStatus,
	} from '../../api/github';
	import { subscribeGitHubOperationEvent } from '../../githubOperationService';
	import { deleteReviewComment, editReviewComment } from '../../api/github/review';
	import { fetchCurrentGitHubUser } from '../../api/github/user';
	import { buildLineAnnotations } from '../repo-diff/annotations';
	import type { DiffLineAnnotation, ReviewAnnotation } from '../repo-diff/annotations';
	import { createReviewAnnotationActionsController } from '../repo-diff/reviewAnnotationActions';
	import type {
		FileDiffRenderOptions,
		FileDiffRenderer,
		FileDiffRendererModule,
	} from '../repo-diff/diffRenderController';
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
	import {
		buildFileLocalCacheKey,
		buildFilePrCacheKey,
		buildSummaryLocalCacheKey,
		buildSummaryPrCacheKey,
		repoDiffCache,
	} from '../../cache/repoDiffCache';
	import { resolveBranchRefs } from '../../diff/branchRefs';
	import ResizablePanel from '../ui/ResizablePanel.svelte';
	import { mapWorkspaceToPrItems } from '../../view-models/prViewModel';
	import {
		buildCheckStats,
		buildDiffTargetKey,
		buildReviewThreads,
		buildTrackedPrMap,
		formatCheckDuration,
		getCheckIcon,
	} from './prOrchestrationHelpers';

	interface Props {
		workspace: Workspace | null;
		focusRepoId?: string | null;
		focusToken?: number;
	}

	type RepoDiffPrStatusEvent = {
		workspaceId: string;
		repoId: string;
		status: {
			pullRequest: {
				repo: string;
				number: number;
				url: string;
				title: string;
				state: string;
				draft: boolean;
				base_repo: string;
				base_branch: string;
				head_repo: string;
				head_branch: string;
				mergeable?: string;
			};
			checks: Array<{
				name: string;
				status: string;
				conclusion?: string;
				details_url?: string;
				started_at?: string;
				completed_at?: string;
				check_run_id?: number;
			}>;
		};
	};

	const { workspace, focusRepoId = null, focusToken = 0 }: Props = $props();

	// ─── Derived workspace data ──────────────────────────────────────────
	const prItems = $derived(mapWorkspaceToPrItems(workspace));

	// ─── Tracked PR map (drives active/ready partition) ────────────────
	let trackedPrMap: Map<string, PullRequestCreated> = $state(new Map());

	$effect(() => {
		if (workspace) {
			trackedPrMap = buildTrackedPrMap(workspace);
		} else {
			trackedPrMap = new Map();
		}
	});

	const partitions = $derived.by(() => {
		const active = prItems.filter((item) => trackedPrMap.has(item.repoId));
		const readyToPR = prItems.filter(
			(item) => !trackedPrMap.has(item.repoId) && (item.hasLocalDiff || item.ahead > 0),
		);
		return { active, readyToPR };
	});

	const activeCount = $derived(partitions.active.length);
	const readyCount = $derived(partitions.readyToPR.length);

	// ─── Sidebar state ──────────────────────────────────────────────────
	let viewMode: 'active' | 'ready' = $state('active');
	let selectedItemId: string | null = $state(null);
	let lastAppliedFocusKey = $state<string | null>(null);

	// ─── Detail tab state ───────────────────────────────────────────────
	let activeTab: 'files' | 'checks' = $state('files');

	// ─── PR tracking ────────────────────────────────────────────────────
	let trackedPr: PullRequestCreated | null = $state(null);
	let trackedPrLoading = $state(false);
	let trackedPrRequestId = 0;

	// ─── Files tab ──────────────────────────────────────────────────────
	let diffSummary: RepoDiffSummary | null = $state(null);
	let localSummary: RepoDiffSummary | null = $state(null);
	let diffSummaryLoading = $state(false);
	let selectedFileIdx = $state(0);
	let selectedSource = $state<'pr' | 'local'>('pr');
	let fileDiffContent: RepoFileDiff | null = $state(null);
	let fileDiffLoading = $state(false);
	let fileDiffError: string | null = $state(null);
	let activeWatchKey: { wsId: string; repoId: string; mode: 'local' | 'pr' } | null = $state(null);
	let activePrBranches: { base: string; head: string } | null = $state(null);
	let activeFileKey: string | null = $state(null);
	let lastDiffSummaryTargetKey: string | null = $state(null);
	let diffSummaryRequestId = 0;
	let localSummaryRequestId = 0;
	let fileDiffRequestId = 0;
	let trackedPrReconcileInFlight = false;

	// ─── Checks ─────────────────────────────────────────────────────────
	let prStatus: PullRequestStatusResult | null = $state(null);
	let prStatusLoading = $state(false);

	// ─── Reviews ────────────────────────────────────────────────────────
	let prReviews: PullRequestReviewComment[] = $state([]);
	let prReviewsLoading = $state(false);
	let currentUserId: number | null = $state(null);

	// ─── Ready-to-PR form ───────────────────────────────────────────────
	let prTitle = $state('');
	let prBody = $state('');
	let isDraft = $state(false);
	let isCreating = $state(false);
	let prCreated = $state(false);
	let prTextGenerating = $state(false);

	// ─── Commit & Push ──────────────────────────────────────────────────
	let repoLocalStatus: RepoLocalStatus | null = $state(null);
	let commitPushLoading = $state(false);
	let commitPushStage: GitHubOperationStage | null = $state(null);
	let commitPushError: string | null = $state(null);
	let commitPushSuccess = $state(false);
	let commitPushSuccessTimer: ReturnType<typeof setTimeout> | null = null;
	const SIDEBAR_COLLAPSED_KEY = 'workset:pr-orchestration:sidebarCollapsed';
	const readSidebarCollapsed = (): boolean => {
		try {
			return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === 'true';
		} catch {
			return false;
		}
	};
	const persistSidebarCollapsed = (collapsed: boolean): void => {
		try {
			localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(collapsed));
		} catch {
			// ignore storage failures
		}
	};
	let sidebarCollapsed = $state(readSidebarCollapsed());
	const canCollapseSidebar = $derived(selectedItemId !== null);

	const toggleSidebar = (): void => {
		if (!sidebarCollapsed && !canCollapseSidebar) return;
		sidebarCollapsed = !sidebarCollapsed;
		persistSidebarCollapsed(sidebarCollapsed);
	};

	// ─── Derived selectors ──────────────────────────────────────────────
	const selectedItem = $derived(prItems.find((item) => item.id === selectedItemId) ?? null);
	const wsId = $derived(workspace?.id ?? '');
	const selectedRepoId = $derived(selectedItem?.repoId ?? '');

	const selectedRepo = $derived.by(() => {
		if (!selectedItem || !workspace) return null;
		return workspace.repos.find((r) => r.id === selectedItem.repoId) ?? null;
	});

	const selectedFile = $derived.by(() => {
		const files =
			selectedSource === 'local' ? (localSummary?.files ?? []) : (diffSummary?.files ?? []);
		return files[selectedFileIdx] ?? null;
	});

	const selectedKey = $derived.by(() => {
		if (!selectedFile) return '';
		return `${selectedSource}:${selectedFile.path}:${selectedFile.prevPath ?? ''}`;
	});

	const selectedFilePath = $derived(selectedFile?.path ?? '');
	const selectedFilePrevPath = $derived(selectedFile?.prevPath ?? '');
	const selectedFileStatus = $derived(selectedFile?.status ?? '');
	const selectedFileAdded = $derived(selectedFile?.added ?? 0);
	const selectedFileRemoved = $derived(selectedFile?.removed ?? 0);
	const selectedFileBinary = $derived(selectedFile?.binary ?? false);
	const activePrKey = $derived.by(() =>
		activePrBranches ? `${activePrBranches.base}:${activePrBranches.head}` : '',
	);

	const getMode = (): 'active' | 'ready' => viewMode;
	const isActiveDetail = $derived(getMode() === 'active' && selectedItem != null);
	const isReadyDetail = $derived(getMode() === 'ready' && selectedItem != null);

	const checkStats = $derived(buildCheckStats(prStatus));
	const reviewThreads = $derived(buildReviewThreads(prReviews));

	const unresolvedCount = $derived(reviewThreads.filter((t) => !t.resolved).length);

	const shouldSplitLocalPendingSection = $derived.by(() => {
		const ls = localSummary;
		return activePrBranches !== null && (ls?.files.length ?? 0) > 0;
	});

	const clearCommitPushSuccessTimer = (): void => {
		if (commitPushSuccessTimer != null) {
			clearTimeout(commitPushSuccessTimer);
			commitPushSuccessTimer = null;
		}
	};

	const pushStatusVisible = $derived(
		isActiveDetail && trackedPr != null && repoLocalStatus != null,
	);

	const pushDisabled = $derived.by(() => {
		if (commitPushLoading) return true;
		const s = repoLocalStatus;
		if (!s) return true;
		return !s.hasUncommitted && s.ahead === 0;
	});

	const commitPushStageLabel = $derived.by(() => {
		const labels: Record<string, string> = {
			queued: 'Queuing...',
			generating_message: 'Generating commit message...',
			staging: 'Staging files...',
			committing: 'Committing...',
			pushing: 'Pushing...',
		};
		return commitPushStage ? (labels[commitPushStage] ?? 'Processing...') : null;
	});

	// ─── Annotation actions controller ──────────────────────────────────
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

	// ─── Line annotations for current file ──────────────────────────────
	const lineAnnotations: DiffLineAnnotation<ReviewAnnotation>[] = $derived.by(() => {
		const src = selectedSource;
		if (src === 'local' || prReviews.length === 0) return [];
		const file = selectedFile;
		if (!file) return [];
		return buildLineAnnotations(prReviews.filter((r) => r.path === file.path));
	});

	// ─── Actions ────────────────────────────────────────────────────────

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

	const selectItem = (itemId: string): void => {
		selectedItemId = itemId;
		activeTab = 'files';
		const item = prItems.find((i) => i.id === itemId);
		trackedPr = item ? (trackedPrMap.get(item.repoId) ?? null) : null;
		trackedPrLoading = false;
		prStatus = null;
		prReviews = [];
		diffSummary = null;
		localSummary = null;
		selectedFileIdx = 0;
		selectedSource = 'pr';
		fileDiffContent = null;
		fileDiffError = null;
		activeFileKey = null;
		activePrBranches = null;
		lastDiffSummaryTargetKey = null;
		diffSummaryRequestId += 1;
		localSummaryRequestId += 1;
		fileDiffRequestId += 1;
		prTitle = '';
		prBody = '';
		isDraft = false;
		isCreating = false;
		prCreated = false;
		repoLocalStatus = null;
		commitPushLoading = false;
		commitPushStage = null;
		commitPushError = null;
		commitPushSuccess = false;
		clearCommitPushSuccessTimer();

		if (item && workspace) {
			void loadTrackedPr(workspace.id, item.repoId);
			void loadRepoLocalStatus(workspace.id, item.repoId);
			if (viewMode === 'ready') {
				void loadSuggestedPrText(workspace.id, item.repoId);
			}
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
			const nextMap = new Map(trackedPrMap);
			if (resolved) {
				nextMap.set(repoId, resolved);
			} else {
				nextMap.delete(repoId);
			}
			trackedPrMap = nextMap;
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

	const buildFileDiffCacheKey = (
		wsId: string,
		repoId: string,
		file: RepoDiffFileSummary,
		source: 'pr' | 'local',
	): string => {
		if (source === 'local') {
			return buildFileLocalCacheKey(
				wsId,
				repoId,
				file.status ?? '',
				file.path,
				file.prevPath ?? '',
			);
		}
		if (activePrBranches) {
			return buildFilePrCacheKey(
				wsId,
				repoId,
				activePrBranches.base,
				activePrBranches.head,
				file.path,
				file.prevPath ?? '',
			);
		}
		return buildFileLocalCacheKey(wsId, repoId, file.status ?? '', file.path, file.prevPath ?? '');
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
		const cacheKey = buildFileDiffCacheKey(wsId, repoId, file, source);
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

	const loadSuggestedPrText = async (wsId: string, repoId: string): Promise<void> => {
		prTextGenerating = true;
		try {
			const generated = await generatePullRequestText(wsId, repoId);
			if (generated.title && !prTitle) prTitle = generated.title;
			if (generated.body && !prBody) prBody = generated.body;
		} catch {
			// non-fatal: user can still type manually
		} finally {
			prTextGenerating = false;
		}
	};

	const loadChecks = async (): Promise<void> => {
		if (!workspace || !selectedItem) return;
		prStatusLoading = true;
		try {
			prStatus = await fetchPullRequestStatus(
				workspace.id,
				selectedItem.repoId,
				trackedPr?.number,
				selectedItem.branch,
			);
		} catch {
			prStatus = null;
		} finally {
			prStatusLoading = false;
		}
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

	const handleCreatePr = async (): Promise<void> => {
		if (!workspace || !selectedItem || !prTitle.trim()) return;
		isCreating = true;
		try {
			const created = await createPullRequest(workspace.id, selectedItem.repoId, {
				title: prTitle.trim(),
				body: prBody.trim(),
				draft: isDraft,
				autoCommit: true,
				autoPush: true,
			});
			trackedPr = created;
			const nextMap = new Map(trackedPrMap);
			nextMap.set(selectedItem.repoId, created);
			trackedPrMap = nextMap;
			prCreated = true;
		} catch {
			// non-fatal
		} finally {
			isCreating = false;
		}
	};

	const handlePushToPr = async (): Promise<void> => {
		if (!workspace || !selectedItem || commitPushLoading) return;
		commitPushLoading = true;
		commitPushStage = 'queued';
		commitPushError = null;
		commitPushSuccess = false;
		try {
			await startCommitAndPushAsync(workspace.id, selectedItem.repoId);
			// Event subscription handles progress updates from here
		} catch (err) {
			commitPushLoading = false;
			commitPushStage = null;
			commitPushError = err instanceof Error ? err.message : 'Failed to start push.';
		}
	};

	const openExternalUrl = (url: string | undefined | null): void => {
		if (url) Browser.OpenURL(url);
	};

	const mapPrStatusEventToTrackedPr = (payload: RepoDiffPrStatusEvent): PullRequestCreated => ({
		repo: payload.status.pullRequest.repo,
		number: payload.status.pullRequest.number,
		url: payload.status.pullRequest.url,
		title: payload.status.pullRequest.title,
		state: payload.status.pullRequest.state,
		draft: payload.status.pullRequest.draft,
		baseRepo: payload.status.pullRequest.base_repo,
		baseBranch: payload.status.pullRequest.base_branch,
		headRepo: payload.status.pullRequest.head_repo,
		headBranch: payload.status.pullRequest.head_branch,
	});

	const applyPrStatusEvent = (payload: RepoDiffPrStatusEvent): void => {
		prStatus = {
			pullRequest: {
				repo: payload.status.pullRequest.repo,
				number: payload.status.pullRequest.number,
				url: payload.status.pullRequest.url,
				title: payload.status.pullRequest.title,
				state: payload.status.pullRequest.state,
				draft: payload.status.pullRequest.draft,
				baseRepo: payload.status.pullRequest.base_repo,
				baseBranch: payload.status.pullRequest.base_branch,
				headRepo: payload.status.pullRequest.head_repo,
				headBranch: payload.status.pullRequest.head_branch,
				mergeable: payload.status.pullRequest.mergeable,
			},
			checks: (payload.status.checks ?? []).map((check) => ({
				name: check.name,
				status: check.status,
				conclusion: check.conclusion,
				detailsUrl: check.details_url,
				startedAt: check.started_at,
				completedAt: check.completed_at,
				checkRunId: check.check_run_id,
			})),
		};
	};

	const reconcileTrackedPrState = async (wsId: string, repoId: string): Promise<void> => {
		if (trackedPrReconcileInFlight) return;
		trackedPrReconcileInFlight = true;
		try {
			await loadTrackedPr(wsId, repoId);
			await refreshWorkspacesStatus(true);
			if (selectedRepoId === repoId) {
				await loadRepoLocalStatus(wsId, repoId);
				await loadDiffSummary(wsId, repoId, trackedPrMap.get(repoId));
			}
		} finally {
			trackedPrReconcileInFlight = false;
		}
	};

	// ─── Effects ────────────────────────────────────────────────────────

	$effect(() => {
		const currentWsId = wsId;
		const currentRepoId = selectedRepoId;
		if (!selectedItem || currentWsId === '' || currentRepoId === '') {
			return;
		}
		const targetKey = buildDiffTargetKey(currentWsId, currentRepoId, trackedPr ?? undefined);
		if (targetKey === lastDiffSummaryTargetKey) {
			return;
		}
		lastDiffSummaryTargetKey = targetKey;
		void loadDiffSummary(currentWsId, currentRepoId, trackedPr ?? undefined);
	});

	$effect(() => {
		if (activeTab === 'checks' && !prStatus && !prStatusLoading && selectedItem) {
			void loadChecks();
		}
	});

	// Load reviews eagerly when an active PR is selected
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

	// Subscribe to commit & push operation events
	$effect(() => {
		const unsub = subscribeGitHubOperationEvent((status: GitHubOperationStatus) => {
			if (!workspace || !selectedItem) return;
			if (status.workspaceId !== workspace.id || status.repoId !== selectedItem.repoId) return;
			if (status.type !== 'commit_push') return;

			if (status.state === 'running') {
				commitPushLoading = true;
				commitPushStage = status.stage;
				commitPushError = null;
				commitPushSuccess = false;
			} else if (status.state === 'completed') {
				commitPushLoading = false;
				commitPushStage = null;
				commitPushError = null;
				commitPushSuccess = true;
				// Refresh data after successful push
				void loadRepoLocalStatus(workspace.id, selectedItem.repoId);
				void loadLocalSummary(workspace.id, selectedItem.repoId);
				if (activePrBranches) {
					void loadDiffSummary(workspace.id, selectedItem.repoId, trackedPr ?? undefined);
				}
				// Auto-dismiss success after 3s
				clearCommitPushSuccessTimer();
				commitPushSuccessTimer = setTimeout(() => {
					commitPushSuccess = false;
					commitPushSuccessTimer = null;
				}, 3000);
			} else if (status.state === 'failed') {
				commitPushLoading = false;
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
				applyPrStatusEvent(payload);
				const nextState = payload.status.pullRequest.state.trim().toLowerCase();
				if (nextState === 'open') {
					const tracked = mapPrStatusEventToTrackedPr(payload);
					trackedPr = tracked;
					const nextMap = new Map(trackedPrMap);
					nextMap.set(selectedItem.repoId, tracked);
					trackedPrMap = nextMap;
					return;
				}

				const nextMap = new Map(trackedPrMap);
				const hadTracked = nextMap.delete(selectedItem.repoId);
				trackedPrMap = nextMap;
				trackedPr = null;
				if (hadTracked) {
					void reconcileTrackedPrState(workspace.id, selectedItem.repoId);
				}
			},
		);
		return unsub;
	});

	$effect(() => {
		if (selectedItemId && !prItems.find((i) => i.id === selectedItemId)) {
			selectedItemId = null;
		}
	});

	$effect(() => {
		const id = selectedItemId;
		if (!id) return;
		const visibleInMode =
			viewMode === 'active'
				? partitions.active.some((item) => item.id === id)
				: partitions.readyToPR.some((item) => item.id === id);
		if (visibleInMode) return;
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

	// Reset selection on viewMode change
	$effect(() => {
		void viewMode;
		selectedItemId = null;
		activeTab = 'files';
	});

	$effect(() => {
		if (selectedItemId !== null || !sidebarCollapsed) {
			return;
		}
		sidebarCollapsed = false;
		persistSidebarCollapsed(false);
	});

	// Load file diff when selection changes
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

	// Stop diff watch on component teardown
	$effect(() => {
		return () => {
			void stopActiveWatch();
		};
	});

	// ─── @pierre/diffs integration ──────────────────────────────────────
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

	// ─── Helpers ────────────────────────────────────────────────────────

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
		<div class="empty-state">
			<GitPullRequest size={48} />
			<p>Select a workspace to view pull requests</p>
		</div>
	{:else if prItems.length === 0}
		<div class="empty-state">
			<GitPullRequest size={48} />
			<p>No repositories in this workspace</p>
		</div>
	{:else}
		{#snippet detailPanel()}
			<main class="detail">
				{#if isActiveDetail && selectedItem && workspace}
					<!-- ── Active PR Detail ── -->

					<!-- PR Header -->
					<div class="pr-header">
						<div class="prh-left">
							<div class="prh-title-row">
								<span class="prh-repo-tag">{selectedItem.repoName}</span>
								<h1 class="prh-title">
									{trackedPrMap.get(selectedItem.repoId)?.title ?? selectedItem.title}
								</h1>
							</div>
							<div class="prh-meta">
								{#if trackedPr}
									<span class="prh-status">
										<Circle size={10} class="prh-status-dot" />
										{trackedPr.state === 'open' ? 'Open' : trackedPr.state}
									</span>
								{/if}
								<span class="prh-branch">
									<GitBranch size={10} />
									<span>{selectedItem.branch}</span>
								</span>
								<span>{selectedItem.updatedAtLabel}</span>
							</div>
						</div>
						{#if trackedPr}
							<a
								href={trackedPr.url}
								class="prh-action-link"
								onclick={(e) => {
									e.preventDefault();
									openExternalUrl(trackedPr?.url);
								}}
							>
								<ExternalLink size={14} />
								View on GitHub
							</a>
						{:else if trackedPrLoading}
							<span class="prh-loading"><Loader2 size={14} class="spin" /></span>
						{/if}
					</div>

					<!-- Push Status Bar -->
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
											{localSummary?.files.length ?? '?'} dirty file{(localSummary?.files.length ??
												0) !== 1
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

					<!-- Tab Bar -->
					<div class="tab-bar">
						<button
							type="button"
							class="tab-btn"
							class:active={activeTab === 'files'}
							onclick={() => (activeTab = 'files')}
						>
							<FileCode size={14} />
							Files Changed
							<span class="tab-count">{filesForDetail.length || selectedItem.dirtyFiles}</span>
						</button>
						<button
							type="button"
							class="tab-btn"
							class:active={activeTab === 'checks'}
							onclick={() => (activeTab = 'checks')}
						>
							{#if checkStats.total === 0}
								<Circle size={14} class="text-muted" />
							{:else if checkStats.failed > 0}
								<XCircle size={14} class="text-red" />
							{:else if checkStats.pending > 0}
								<Loader2 size={14} class="text-yellow spin" />
							{:else}
								<CheckCircle2 size={14} class="text-green" />
							{/if}
							Checks
							<span class="tab-count">{checkStats.total || ''}</span>
						</button>
						{#if unresolvedCount > 0}
							<span class="tab-review-badge">
								<MessageSquare size={12} />
								{unresolvedCount} unresolved
							</span>
						{/if}
					</div>

					<!-- Tab Content -->
					<div class="tab-content">
						{#if activeTab === 'files'}
							<!-- ── Files Tab ── -->
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
							<!-- ── Checks Tab ── -->
							<div class="checks-panel">
								<div class="checks-max">
									{#if prStatusLoading}
										<div class="panel-loading">
											<Loader2 size={20} class="spin" />
											<span>Loading checks...</span>
										</div>
									{:else if !prStatus || prStatus.checks.length === 0}
										<div class="panel-loading">
											<CheckCircle2 size={32} />
											<span>No checks available</span>
											<button type="button" class="ghost-btn" onclick={loadChecks}>
												<RefreshCw size={12} /> Refresh checks
											</button>
										</div>
									{:else}
										<div class="checks-header-row">
											<h2>Checks</h2>
											<button type="button" class="ghost-btn" onclick={loadChecks}>
												<RefreshCw size={12} /> Refresh checks
											</button>
										</div>
										<div class="checks-list">
											{#each prStatus.checks as check (check.name)}
												{@const iconType = getCheckIcon(check)}
												<div class="ck-row">
													<div class="ck-circle {iconType}">
														{#if iconType === 'success'}<CheckCircle2 size={16} />
														{:else if iconType === 'failure'}<XCircle size={16} />
														{:else}<Loader2 size={16} class="spin" />
														{/if}
													</div>
													<div class="ck-info">
														<div class="ck-name-row">
															<h3>{check.name}</h3>
														</div>
														<p class="ck-dur">
															{#if check.conclusion === 'success'}Completed in {formatCheckDuration(
																	check,
																)}
															{:else if check.status === 'in_progress'}Running for {formatCheckDuration(
																	check,
																)}...
															{:else}Pending
															{/if}
														</p>
													</div>
													<div class="ck-actions">
														<button type="button" class="ck-action" title="View Logs">
															<Terminal size={14} />
														</button>
														{#if check.detailsUrl}
															<button
																type="button"
																class="ck-action"
																title="View on Provider"
																onclick={() => openExternalUrl(check.detailsUrl)}
															>
																<ExternalLink size={14} />
															</button>
														{/if}
													</div>
												</div>
											{/each}
										</div>
									{/if}
								</div>
							</div>
						{/if}
					</div>
				{:else if isReadyDetail && selectedItem && workspace}
					<!-- ── Ready to PR Detail ── -->
					<div class="ready-detail">
						<!-- Header -->
						<div class="rd-header">
							<div class="rd-icon">
								<Upload size={18} />
							</div>
							<div class="rd-info">
								<h1>{selectedItem.repoName}</h1>
								<div class="rd-branch-row">
									<GitBranch size={11} class="text-green" />
									<span class="rd-branch">{selectedItem.branch}</span>
									<ArrowUpRight size={10} class="rd-arrow" />
									<span class="rd-base">{selectedRepo?.defaultBranch ?? 'main'}</span>
								</div>
							</div>
						</div>

						<div class="rd-stats">
							<span class="rd-stat">
								<GitCommit size={12} class="text-blue" />
								<strong>{selectedItem.ahead}</strong> commit{selectedItem.ahead !== 1 ? 's' : ''} ahead
								of main
							</span>
							<span class="rd-stat">
								<FileCode size={12} />
								<strong>{filesForDetail.length || selectedItem.dirtyFiles}</strong>
								file{(filesForDetail.length || selectedItem.dirtyFiles) !== 1 ? 's' : ''}
							</span>
							{#if totalAdd > 0}<span class="text-green">+{totalAdd}</span>{/if}
							{#if totalDel > 0}<span class="text-red">-{totalDel}</span>{/if}
						</div>
					</div>

					<!-- Content -->
					<div class="rd-content">
						<div class="rd-max">
							{#if prCreated}
								<div class="rd-success">
									<div class="rd-success-icon">
										<CheckCircle2 size={32} />
									</div>
									<h2>Pull Request Created</h2>
									<p>
										{isDraft ? 'Draft PR' : 'PR'} for
										<span class="mono text-green">{selectedItem.branch}</span>
										is now open on <span class="text-white">{selectedItem.repoName}</span>
									</p>
									<button
										type="button"
										class="ghost-btn"
										onclick={() => openExternalUrl(trackedPr?.url)}
									>
										<ExternalLink size={14} />
										View on GitHub
									</button>
								</div>
							{:else}
								<!-- PR Title -->
								<div class="form-field">
									<label class="form-label" for="pr-title-input">
										PR Title
										{#if prTextGenerating}
											<span class="form-generating"
												><Loader2 size={12} class="spin" /> Generating…</span
											>
										{/if}
									</label>
									<div class="form-input-wrap" class:shimmer={prTextGenerating && !prTitle}>
										<input
											id="pr-title-input"
											type="text"
											class="form-input"
											bind:value={prTitle}
											placeholder={prTextGenerating ? '' : 'Enter PR title...'}
										/>
									</div>
								</div>

								<!-- PR Body -->
								<div class="form-field">
									<label class="form-label" for="pr-body-input">
										Description <span class="form-optional">(optional)</span>
										{#if prTextGenerating && prTitle && !prBody}
											<span class="form-generating"
												><Loader2 size={12} class="spin" /> Generating…</span
											>
										{/if}
									</label>
									<div class="form-input-wrap" class:shimmer={prTextGenerating && !prBody}>
										<textarea
											id="pr-body-input"
											class="form-textarea"
											rows={4}
											bind:value={prBody}
											placeholder={prTextGenerating ? '' : 'Describe the changes in this PR...'}
										></textarea>
									</div>
								</div>

								<!-- Files changed summary -->
								<div class="form-field">
									<span class="form-label">Files Changed</span>
									<div class="rd-file-list">
										{#if filesForDetail.length > 0}
											{#each filesForDetail as file (file.path)}
												<div class="rd-file-row">
													<FileCode size={13} />
													<span class="rd-file-name">{file.path}</span>
													<span class="text-green text-xs mono">+{file.added}</span>
													{#if file.removed > 0}<span class="text-red text-xs mono"
															>-{file.removed}</span
														>{/if}
												</div>
											{/each}
										{:else if selectedRepo}
											{#each selectedRepo.files as file (file.path)}
												<div class="rd-file-row">
													<FileCode size={13} />
													<span class="rd-file-name">{file.path}</span>
													<span class="text-green text-xs mono">+{file.added}</span>
													{#if file.removed > 0}<span class="text-red text-xs mono"
															>-{file.removed}</span
														>{/if}
												</div>
											{/each}
										{:else}
											<div class="rd-file-row"><span>No files detected</span></div>
										{/if}
									</div>
								</div>

								<!-- Actions -->
								<div class="rd-actions">
									<label class="rd-draft-toggle">
										<input type="checkbox" bind:checked={isDraft} />
										<span>Create as draft</span>
									</label>
									<button
										type="button"
										class="rd-create-btn"
										disabled={!prTitle.trim() || isCreating}
										onclick={() => void handleCreatePr()}
									>
										{#if isCreating}
											<Loader2 size={16} class="spin" /> Creating...
										{:else}
											<GitPullRequest size={16} />
											{isDraft ? 'Create Draft PR' : 'Create Pull Request'}
										{/if}
									</button>
								</div>
							{/if}
						</div>
					</div>
				{:else}
					<!-- ── Empty Detail ── -->
					<div class="empty-state">
						{#if viewMode === 'active'}
							<GitPullRequest size={48} />
							<p>Select a PR to view details</p>
						{:else}
							<Upload size={48} />
							<p>Select a branch to create a PR</p>
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
				<!-- ═══════════════════ LEFT PANEL ═══════════════════ -->
				<aside class="sidebar">
					<!-- Workset Header -->
					<div class="ws-header">
						<div class="ws-header-top">
							<div class="ws-eyebrow">Current Workset</div>
							<button
								type="button"
								class="sidebar-toggle-btn"
								class:disabled={!canCollapseSidebar}
								aria-label="Collapse sidebar"
								title={canCollapseSidebar ? 'Collapse sidebar' : 'Select an item to collapse'}
								disabled={!canCollapseSidebar}
								onclick={toggleSidebar}
							>
								<ChevronLeft size={14} />
							</button>
						</div>
						<div class="ws-badge">
							<Box size={14} class="ws-badge-icon" />
							<span class="ws-badge-name">{workspace.name}</span>
						</div>
					</div>

					<!-- Tab Switcher -->
					<div class="mode-switch">
						<button
							type="button"
							class="ms-btn"
							class:active={viewMode === 'active'}
							onclick={() => (viewMode = 'active')}
						>
							<GitPullRequest size={14} />
							Active PRs
							<span class="ms-count">{activeCount}</span>
						</button>
						<button
							type="button"
							class="ms-btn"
							class:active={viewMode === 'ready'}
							onclick={() => (viewMode = 'ready')}
						>
							<Upload size={14} class={viewMode === 'ready' ? 'text-green' : ''} />
							Ready to PR
							{#if readyCount > 0}
								<span class="ms-count ready">{readyCount}</span>
							{/if}
						</button>
					</div>

					<!-- List -->
					<div class="list">
						{#if viewMode === 'active'}
							{#if partitions.active.length > 0}
								{#each partitions.active as item (item.id)}
									{@const isActive = item.id === selectedItemId}
									<button
										type="button"
										class="list-item"
										class:active={isActive}
										onclick={() => selectItem(item.id)}
									>
										<div class="li-top">
											<h3 class="li-title" class:bright={isActive}>
												{trackedPrMap.get(item.repoId)?.title ?? item.title}
											</h3>
											{#if isActive}<ChevronRight size={14} class="li-chevron" />{/if}
										</div>
										<div class="li-meta">
											<span class="li-repo">{item.repoName}</span>
											<span class="li-sep">·</span>
											<span
												class:li-passing={item.status === 'open'}
												class:li-running={item.status === 'running'}
												class:li-blocked={item.status === 'blocked'}
											>
												{item.status}
											</span>
											{#if item.dirtyFiles > 0}
												<span class="li-sep">·</span>
												<span class="li-warn">
													<MessageSquare size={8} />
													{item.dirtyFiles}
												</span>
											{/if}
										</div>
									</button>
								{/each}
							{:else}
								<div class="list-empty">
									<CheckCircle2 size={24} />
									<p>No active PRs</p>
								</div>
							{/if}
						{:else if partitions.readyToPR.length > 0}
							{#each partitions.readyToPR as item (item.id)}
								{@const isActive = item.id === selectedItemId}
								<button
									type="button"
									class="list-item"
									class:active-ready={isActive}
									onclick={() => selectItem(item.id)}
								>
									<div class="li-top">
										<h3 class="li-title" class:bright={isActive}>{item.repoName}</h3>
										{#if isActive}<ChevronRight size={14} class="li-chevron-green" />{/if}
									</div>
									<div class="li-branch-row">
										<GitBranch size={11} class="text-green" />
										<span class="li-branch-name">{item.branch}</span>
									</div>
									<div class="li-meta">
										<span class="li-commits">
											<ArrowUpRight size={10} class="text-blue" />
											{item.ahead} commit{item.ahead !== 1 ? 's' : ''} ahead
										</span>
										<span class="text-green">+{item.dirtyFiles}</span>
										<span class="li-time">{item.updatedAtLabel}</span>
									</div>
								</button>
							{/each}
						{:else}
							<div class="list-empty">
								<CheckCircle2 size={24} />
								<p>All branches have PRs</p>
							</div>
						{/if}
					</div>
				</aside>

				{#snippet second()}
					{@render detailPanel()}
				{/snippet}
			</ResizablePanel>
		{/if}
	{/if}
	<RepoDiffAnnotationStyles />
</div>

<style src="./PROrchestrationView.css"></style>
