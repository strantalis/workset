<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { Browser } from '@wailsio/runtime';
	import type { FileDiffMetadata, ParsedPatch } from '@pierre/diffs';
	import type {
		CheckAnnotation,
		PullRequestCheck,
		PullRequestCreated,
		PullRequestReviewComment,
		PullRequestStatusResult,
		RemoteInfo,
		Repo,
		RepoDiffFileSummary,
		RepoDiffSummary,
		RepoFileDiff,
	} from '../types';
	import {
		editReviewComment,
		fetchGitHubOperationStatus,
		fetchCheckAnnotations,
		fetchCurrentGitHubUser,
		fetchPullRequestReviews,
		fetchPullRequestStatus,
		fetchRepoLocalStatus,
		replyToReviewComment,
		startCommitAndPushAsync,
		startCreatePullRequestAsync,
	} from '../api/github';
	import type { GitHubOperationStatus, GitHubOperationStage, RepoLocalStatus } from '../api/github';
	import {
		fetchRepoDiffSummary,
		fetchRepoFileDiff,
		fetchBranchDiffSummary,
		fetchBranchFileDiff,
		startRepoDiffWatch,
		updateRepoDiffWatch,
		stopRepoDiffWatch,
	} from '../api/repo-diff';
	import { getPrCreateStageCopy } from '../prCreateProgress';
	import type { PrCreateStage } from '../prCreateProgress';
	import { applyRepoDiffSummary, applyRepoLocalStatus } from '../state';
	import type { DiffLineAnnotation, ReviewAnnotation } from './repo-diff/annotations';
	import { buildLineAnnotations } from './repo-diff/annotations';
	import { createPrStatusController } from './repo-diff/prStatusController';
	import {
		applyPrReviewsLifecycleEvent,
		applyPrStatusLifecycleEvent,
		buildGitHubOperationsStateSurface,
		buildPrStatusStateSurface,
		resolveEffectivePrMode,
	} from './repo-diff/prOrchestrationSurface';
	import {
		createRepoDiffGitHubHandlers,
		type RepoDiffGitHubHandlers,
	} from './repo-diff/githubHandlers';
	import {
		buildReviewCountsByPath,
		filterReviewsBySelectedPath,
		getReviewCountForFile,
	} from './repo-diff/reviewDerived';
	import {
		createRepoDiffFileController,
		type BranchDiffRefs,
		type SummarySource,
	} from './repo-diff/fileDiffController';
	import { createGitHubOperationsController } from './repo-diff/githubOperationsController';
	import { createReviewAnnotationActionsController } from './repo-diff/reviewAnnotationActions';
	import { createSummarySourceController } from './repo-diff/summarySourceController';
	import { createRepoDiffWatcherLifecycle } from './repo-diff/watcherLifecycle';
	import {
		createSidebarResizeController,
		REPO_DIFF_DEFAULT_SIDEBAR_WIDTH,
		REPO_DIFF_MIN_SIDEBAR_WIDTH,
		REPO_DIFF_SIDEBAR_WIDTH_KEY,
	} from './repo-diff/sidebarResizeController';
	import {
		createRepoDiffLifecycle,
		type RepoDiffLocalStatusEvent,
		type RepoDiffSummaryEvent,
	} from './repo-diff/repoDiffLifecycle';
	import { createCheckSidebarController, getCheckStats } from './repo-diff/checkSidebarController';
	import {
		createDiffRenderController,
		type FileDiffRenderOptions,
		type FileDiffRendererModule,
	} from './repo-diff/diffRenderController';
	import {
		formatRepoDiffError,
		getCommitPushStageCopy,
		openTrustedGitHubURL,
		parseOptionalNumber,
	} from './repo-diff/utils';
	import RepoDiffAnnotationStyles from './repo-diff/RepoDiffAnnotationStyles.svelte';
	import RepoDiffAuthModal from './repo-diff/RepoDiffAuthModal.svelte';
	import RepoDiffContentPane from './repo-diff/RepoDiffContentPane.svelte';
	import RepoDiffHeader from './repo-diff/RepoDiffHeader.svelte';
	import RepoDiffPrPanel from './repo-diff/RepoDiffPrPanel.svelte';

	const validateAndOpenURL = (url: string | undefined | null): void =>
		openTrustedGitHubURL(url, Browser.OpenURL);

	interface Props {
		repo: Repo | null;
		workspaceId: string;
		onClose: () => void;
	}

	const { repo, workspaceId, onClose }: Props = $props();

	const repoId = $derived(repo?.id ?? '');
	const repoName = $derived(repo?.name ?? '');
	const repoDefaultBranch = $derived(repo?.defaultBranch ?? '');
	const repoStatusKnown = $derived(repo?.statusKnown ?? true);
	const repoMissing = $derived(repo?.missing ?? false);
	const repoDirty = $derived(repo?.dirty ?? false);

	type DiffsModule = FileDiffRendererModule<ReviewAnnotation> & {
		parsePatchFiles: (patch: string) => ParsedPatch[];
	};

	let summary: RepoDiffSummary | null = $state(null);
	let summaryLoading = $state(true);
	let summaryError: string | null = $state(null);

	let selected: RepoDiffFileSummary | null = $state(null);
	let selectedDiff: FileDiffMetadata | null = $state(null);
	let fileMeta: RepoFileDiff | null = $state(null);
	let fileLoading = $state(false);
	let fileError: string | null = $state(null);

	let diffMode: 'split' | 'unified' = $state('split');
	let diffContainer: HTMLElement | null = $state(null);
	let diffModule = $state<DiffsModule | null>(null);
	let rendererLoading = $state(false);
	let rendererError: string | null = $state(null);

	let prBase = $state('');
	let prBaseRemote = $state('');
	let prDraft = $state(false);
	let prPanelExpanded = $state(false);
	let prCreateError: string | null = $state(null);
	let prCreateSuccess: PullRequestCreated | null = $state(null);
	let prTracked: PullRequestCreated | null = $state(null);
	let prCreating = $state(false);
	let prCreatingStage: PrCreateStage | null = $state(null);

	const prCreateStageCopy = $derived.by(() => getPrCreateStageCopy(prCreatingStage));
	const commitPushStageCopy = $derived.by(() => getCommitPushStageCopy(commitPushStage));

	// Remotes list for base remote dropdown
	let remotes: RemoteInfo[] = $state([]);
	let remotesLoading = $state(false);

	// PR panel mode state
	let forceMode: 'create' | 'status' | null = $state(null);

	let prNumberInput = $state('');
	let prBranchInput = $state('');
	let prStatus: PullRequestStatusResult | null = $state(null);
	let prStatusError: string | null = $state(null);
	let prStatusLoading = $state(false);

	let prReviews: PullRequestReviewComment[] = $state([]);
	let prReviewsLoading = $state(false);
	let prReviewsSent = $state(false);

	// Comment management state
	let currentUserId: number | null = $state(null);
	let authModalOpen = $state(false);
	let authModalMessage: string | null = $state(null);
	let authPendingAction: (() => Promise<void>) | null = null;

	let localStatus: RepoLocalStatus | null = $state(null);
	let commitPushLoading = $state(false);
	let commitPushStage: GitHubOperationStage | null = $state(null);
	let commitPushError: string | null = $state(null);
	let commitPushSuccess = $state(false);
	const handledOperationCompletions = new Set<string>();

	// Local uncommitted changes summary (separate from PR branch diff)
	let localSummary: RepoDiffSummary | null = $state(null);

	// Track which source the selected file is from
	let selectedSource: SummarySource = $state('pr');

	// Check annotations state
	let expandedCheck: string | null = $state(null);
	let checkAnnotations: Record<string, CheckAnnotation[]> = $state({});
	let checkAnnotationsLoading: Record<string, boolean> = $state({});

	// Sidebar resize state
	let sidebarWidth = $state(REPO_DIFF_DEFAULT_SIDEBAR_WIDTH);
	let isResizing = $state(false);

	let repoDiffGitHubHandlers: RepoDiffGitHubHandlers | null = null;
	const loadRemotes = (): Promise<void> =>
		repoDiffGitHubHandlers?.loadRemotes() ?? Promise.resolve();
	const loadTrackedPR = (): Promise<void> =>
		repoDiffGitHubHandlers?.loadTrackedPR() ?? Promise.resolve();
	const handleDeleteComment = (commentId: number): Promise<void> =>
		repoDiffGitHubHandlers?.handleDeleteComment(commentId) ?? Promise.resolve();
	const handleResolveThread = (threadId: string, resolve: boolean): Promise<void> =>
		repoDiffGitHubHandlers?.handleResolveThread(threadId, resolve) ?? Promise.resolve();

	const sidebarResizeController = createSidebarResizeController({
		document,
		window,
		storage: localStorage,
		storageKey: REPO_DIFF_SIDEBAR_WIDTH_KEY,
		minWidth: REPO_DIFF_MIN_SIDEBAR_WIDTH,
		getSidebarWidth: () => sidebarWidth,
		setSidebarWidth: (value) => (sidebarWidth = value),
		setIsResizing: (value) => (isResizing = value),
	});

	const watcherLifecycle = createRepoDiffWatcherLifecycle({
		startWatch: startRepoDiffWatch,
		updateWatch: updateRepoDiffWatch,
		stopWatch: stopRepoDiffWatch,
	});

	// Derived mode: status when PR exists, create otherwise
	const effectiveMode = $derived(resolveEffectivePrMode(forceMode, prTracked));

	const annotationActionsController = createReviewAnnotationActionsController({
		document,
		workspaceId: () => workspaceId,
		repoId: () => repoId,
		prNumberInput: () => prNumberInput,
		prBranchInput: () => prBranchInput,
		parseNumber: (value) => parseOptionalNumber(value),
		getCurrentUserId: () => currentUserId,
		getPrReviews: () => prReviews,
		setPrReviews: (value) => (prReviews = value),
		replyToReviewComment,
		editReviewComment,
		handleDeleteComment: (commentId) => handleDeleteComment(commentId),
		handleResolveThread: (threadId, resolve) => handleResolveThread(threadId, resolve),
		formatError: (error, fallback) => formatRepoDiffError(error, fallback),
		showAlert: alert,
	});

	const buildOptions = (): FileDiffRenderOptions<ReviewAnnotation> => ({
		theme: 'pierre-dark',
		themeType: 'dark',
		diffStyle: diffMode,
		diffIndicators: 'bars',
		hunkSeparators: 'line-info',
		lineDiffType: 'word',
		overflow: 'scroll',
		disableFileHeader: true,
		renderAnnotation: (annotation: DiffLineAnnotation<ReviewAnnotation>) =>
			annotationActionsController.renderAnnotation(annotation),
	});

	const startResize = (event: MouseEvent): void => sidebarResizeController.startResize(event);

	const githubOperationsController = createGitHubOperationsController({
		...buildGitHubOperationsStateSurface({
			workspaceId: () => workspaceId,
			repoId: () => repoId,
			prBase: () => prBase,
			prBaseRemote: () => prBaseRemote,
			prDraft: () => prDraft,
			prCreating: () => prCreating,
			commitPushLoading: () => commitPushLoading,
			authModalOpen: () => authModalOpen,
			getAuthPendingAction: () => authPendingAction,
			setAuthModalOpen: (value) => (authModalOpen = value),
			setAuthModalMessage: (value) => (authModalMessage = value),
			setAuthPendingAction: (value) => (authPendingAction = value),
			setPrPanelExpanded: (value) => (prPanelExpanded = value),
			setPrCreating: (value) => (prCreating = value),
			setPrCreatingStage: (value) => (prCreatingStage = value),
			setPrCreateError: (value) => (prCreateError = value),
			setPrCreateSuccess: (value) => (prCreateSuccess = value),
			setPrTracked: (value) => (prTracked = value),
			setForceMode: (value) => (forceMode = value),
			setPrNumberInput: (value) => (prNumberInput = value),
			setPrStatus: (value) => (prStatus = value),
			setCommitPushLoading: (value) => (commitPushLoading = value),
			setCommitPushStage: (value) => (commitPushStage = value),
			setCommitPushError: (value) => (commitPushError = value),
			setCommitPushSuccess: (value) => (commitPushSuccess = value),
		}),
		handledOperationCompletions,
		handleRefresh: () => handleRefresh(),
		formatError: formatRepoDiffError,
		startCreatePullRequestAsync,
		startCommitAndPushAsync,
		fetchGitHubOperationStatus,
	});

	const runGitHubAction = (
		action: () => Promise<void>,
		onError: (message: string) => void,
		fallback: string,
	): Promise<void> => githubOperationsController.runGitHubAction(action, onError, fallback);

	const handleAuthSuccess = (): Promise<void> => githubOperationsController.handleAuthSuccess();

	const handleAuthClose = (): void => githubOperationsController.handleAuthClose();

	const applyGitHubOperationStatus = (status: GitHubOperationStatus): void =>
		githubOperationsController.applyGitHubOperationStatus(status);

	const loadGitHubOperationStatuses = (): Promise<void> =>
		githubOperationsController.loadGitHubOperationStatuses();

	const handleCommitAndPush = (): Promise<void> => githubOperationsController.handleCommitAndPush();

	const handleCreatePR = (): Promise<void> => githubOperationsController.handleCreatePR();

	const filteredReviews = $derived.by(() =>
		filterReviewsBySelectedPath(prReviews, selected ? selected.path : undefined),
	);

	// Check stats for compact display
	const checkStats = $derived.by(() => getCheckStats(prStatus?.checks ?? []));

	const reviewCountsByPath = $derived.by(() => buildReviewCountsByPath(prReviews));
	const reviewCountForFile = (path: string): number =>
		getReviewCountForFile(reviewCountsByPath, path);

	const ensureRenderer = async (): Promise<void> => {
		if (diffModule || rendererLoading) return;
		rendererLoading = true;
		rendererError = null;
		try {
			diffModule = (await import('@pierre/diffs')) as DiffsModule;
		} catch (err) {
			rendererError = formatRepoDiffError(err, 'Diff renderer failed to load.');
		} finally {
			rendererLoading = false;
		}
	};

	const diffRenderController = createDiffRenderController<ReviewAnnotation>({
		getDiffModule: () => diffModule,
		getSelectedDiff: () => selectedDiff,
		getDiffContainer: () => diffContainer,
		buildOptions,
		getLineAnnotations: () => buildLineAnnotations(filteredReviews),
		requestAnimationFrame: (callback) => requestAnimationFrame(callback),
		setTimeout: (callback, delay) => setTimeout(callback, delay),
	});

	const renderDiff = (): void => diffRenderController.renderDiff();

	let summarySourceController: ReturnType<typeof createSummarySourceController> | null = null;
	const useBranchDiff = (): BranchDiffRefs | null =>
		summarySourceController?.useBranchDiff() ?? null;

	const fileDiffController = createRepoDiffFileController({
		workspaceId: () => workspaceId,
		repoId: () => repoId,
		selectedSource: () => selectedSource,
		useBranchDiff,
		setSelected: (value) => (selected = value),
		setSelectedSource: (value) => (selectedSource = value),
		setSelectedDiff: (value) => (selectedDiff = value),
		setFileMeta: (value) => (fileMeta = value),
		setFileLoading: (value) => (fileLoading = value),
		setFileError: (value) => (fileError = value),
		ensureRenderer,
		getDiffModule: () => diffModule,
		getRendererError: () => rendererError,
		fetchRepoFileDiff,
		fetchBranchFileDiff,
		formatError: formatRepoDiffError,
		requestAnimationFrame: (callback) => requestAnimationFrame(callback),
		renderDiff,
	});

	const queueRenderDiff = (): void => fileDiffController.queueRenderDiff();

	const selectFile = (file: RepoDiffFileSummary, source: SummarySource = 'pr'): void =>
		fileDiffController.selectFile(file, source);

	const reportCheckSidebarError = (message: unknown): void => {
		if (prStatusError) return;
		prStatusError =
			typeof message === 'string' && message.trim().length > 0
				? message
				: 'Failed to load check annotations.';
	};

	const checkSidebarController = createCheckSidebarController({
		getExpandedCheck: () => expandedCheck,
		setExpandedCheck: (value) => (expandedCheck = value),
		getCheckAnnotations: () => checkAnnotations,
		setCheckAnnotations: (value) => (checkAnnotations = value),
		getCheckAnnotationsLoading: () => checkAnnotationsLoading,
		setCheckAnnotationsLoading: (value) => (checkAnnotationsLoading = value),
		getPrStatus: () => prStatus,
		getRemotes: () => remotes,
		getPrBaseRemote: () => prBaseRemote,
		getSummary: () => summary,
		fetchCheckAnnotations: (owner, repoName, checkRunId) =>
			fetchCheckAnnotations(owner, repoName, checkRunId),
		selectFile,
		setPendingScrollLine: (line) => {
			diffRenderController.setPendingScrollLine(line);
		},
		logError: (message) => reportCheckSidebarError(message),
	});

	const formatDuration = (ms: number): string => checkSidebarController.formatDuration(ms);
	const getCheckStatusClass = (conclusion: string | undefined, status: string): string =>
		checkSidebarController.getCheckStatusClass(conclusion, status);
	const toggleCheckExpansion = (check: PullRequestCheck): void =>
		checkSidebarController.toggleCheckExpansion(check);
	const navigateToAnnotationFile = (path: string, line: number): void =>
		checkSidebarController.navigateToAnnotationFile(path, line);
	const getFilteredAnnotations = (
		checkName: string,
	): { annotations: CheckAnnotation[]; filteredCount: number } =>
		checkSidebarController.getFilteredAnnotations(checkName);

	const shouldSplitLocalPendingSection = $derived.by(
		() => effectiveMode === 'status' && useBranchDiff() !== null,
	);

	summarySourceController = createSummarySourceController({
		workspaceId: () => workspaceId,
		repoId: () => repoId,
		repoStatusKnown: () => repoStatusKnown,
		repoMissing: () => repoMissing,
		localHasUncommitted: () => localStatus?.hasUncommitted ?? false,
		getRemotes: () => remotes,
		getPullRequestRefs: () => prStatus?.pullRequest ?? prTracked,
		selected: () => selected,
		selectedSource: () => selectedSource,
		summary: () => summary,
		setSummary: (value) => (summary = value),
		setSummaryLoading: (value) => (summaryLoading = value),
		setSummaryError: (value) => (summaryError = value),
		setLocalSummary: (value) => (localSummary = value),
		setSelected: (value) => (selected = value),
		setSelectedDiff: (value) => (selectedDiff = value),
		setFileMeta: (value) => (fileMeta = value),
		setFileError: (value) => (fileError = value),
		selectFile,
		fetchRepoDiffSummary,
		fetchBranchDiffSummary,
		applyRepoDiffSummary,
		formatError: formatRepoDiffError,
	});

	const loadSummary = (): Promise<void> => summarySourceController.loadSummary();
	const loadLocalSummary = (): Promise<void> => summarySourceController.loadLocalSummary();
	const applySummaryUpdate = (data: RepoDiffSummary, source: SummarySource): void =>
		summarySourceController.applySummaryUpdate(data, source);
	const prStatusController = createPrStatusController({
		...buildPrStatusStateSurface({
			workspaceId: () => workspaceId,
			repoId: () => repoId,
			prNumberInput: () => prNumberInput,
			prBranchInput: () => prBranchInput,
			effectiveMode: () => effectiveMode,
			currentUserId: () => currentUserId,
			setCurrentUserId: (value) => (currentUserId = value),
			setPrStatus: (value) => (prStatus = value),
			setPrStatusLoading: (value) => (prStatusLoading = value),
			setPrStatusError: (value) => (prStatusError = value),
			setPrReviews: (value) => (prReviews = value),
			setPrReviewsLoading: (value) => (prReviewsLoading = value),
			setPrReviewsSent: (value) => (prReviewsSent = value),
			setLocalStatus: (value) => (localStatus = value),
		}),
		parseNumber: parseOptionalNumber,
		runGitHubAction,
		loadSummary,
		loadLocalSummary,
		fetchPullRequestStatus,
		fetchPullRequestReviews,
		fetchCurrentGitHubUser,
		fetchRepoLocalStatus,
		applyRepoLocalStatus,
	});
	const loadPrStatus = (): Promise<void> => prStatusController.loadPrStatus();
	const loadPrReviews = (): Promise<void> => prStatusController.loadPrReviews();
	const loadCurrentUser = (): Promise<void> => prStatusController.loadCurrentUser();
	const loadLocalStatus = (): Promise<void> => prStatusController.loadLocalStatus();
	const handleRefresh = (): Promise<void> => prStatusController.handleRefresh();

	repoDiffGitHubHandlers = createRepoDiffGitHubHandlers({
		workspaceId: () => workspaceId,
		repoId: () => repoId,
		runGitHubAction,
		loadPrReviews,
		loadPrStatus,
		loadLocalStatus,
		loadLocalSummary,
		setRemotesLoading: (value) => (remotesLoading = value),
		setRemotes: (value) => (remotes = value),
		setPrTracked: (value) => (prTracked = value),
		getPrNumberInput: () => prNumberInput,
		setPrNumberInput: (value) => (prNumberInput = value),
		getPrBranchInput: () => prBranchInput,
		setPrBranchInput: (value) => (prBranchInput = value),
		alertUser: alert,
	});

	const repoDiffLifecycle = createRepoDiffLifecycle({
		workspaceId: () => workspaceId,
		repoId: () => repoId,
		onGitHubOperationEvent: (payload) => {
			applyGitHubOperationStatus(payload);
		},
		onSummaryEvent: (payload: RepoDiffSummaryEvent) => {
			applySummaryUpdate(payload.summary, 'pr');
		},
		onLocalSummaryEvent: (payload: RepoDiffSummaryEvent) => {
			applySummaryUpdate(payload.summary, 'local');
			applyRepoDiffSummary(payload.workspaceId, payload.repoId, payload.summary);
		},
		onLocalStatusEvent: (payload: RepoDiffLocalStatusEvent) => {
			localStatus = payload.status;
			if (!payload.status.hasUncommitted) localSummary = null;
			applyRepoLocalStatus(payload.workspaceId, payload.repoId, payload.status);
		},
		onPrStatusEvent: (payload) => {
			applyPrStatusLifecycleEvent(payload, {
				setPrStatus: (value) => (prStatus = value),
				setPrStatusError: (value) => (prStatusError = value),
				setPrStatusLoading: (value) => (prStatusLoading = value),
			});
		},
		onPrReviewsEvent: (payload) => {
			applyPrReviewsLifecycleEvent(payload, {
				setPrReviews: (value) => (prReviews = value),
				setPrReviewsLoading: (value) => (prReviewsLoading = value),
				setPrReviewsSent: (value) => (prReviewsSent = value),
				currentUserId: () => currentUserId,
				loadCurrentUser,
			});
		},
		loadSummary,
		loadTrackedPR,
		loadRemotes,
		loadLocalStatus,
		loadLocalSummary,
		loadGitHubOperationStatuses,
		cleanupDiff: () => {
			diffRenderController.cleanUp();
		},
		watcherLifecycle,
	});

	onMount(() => {
		sidebarResizeController.loadPersistedWidth();
		repoDiffLifecycle.mount();
	});

	onDestroy(() => {
		sidebarResizeController.destroy();
		repoDiffLifecycle.destroy();
	});

	$effect(() => {
		repoDiffLifecycle.syncWatchLifecycle({
			workspaceId,
			repoId,
			prNumber: parseOptionalNumber(prNumberInput),
			prBranch: prBranchInput.trim() || undefined,
		});
	});

	$effect(() => {
		if (!repoId) return;
		void loadGitHubOperationStatuses();
	});

	$effect(() => {
		const reviewCount = filteredReviews.length;
		if (!selectedDiff || !diffContainer) return;
		void reviewCount;
		void diffMode;
		void diffModule;
		queueRenderDiff();
	});

	$effect(() => {
		summarySourceController.reloadSummaryOnBranchRefChange();
	});

	$effect(() => {
		repoDiffLifecycle.syncWatchUpdate({
			workspaceId,
			repoId,
			prNumber: parseOptionalNumber(prNumberInput),
			prBranch: prBranchInput.trim() || undefined,
		});
	});
</script>

{#if repo}
	<section class="diff">
		<RepoDiffHeader
			{repoName}
			{repoDefaultBranch}
			{repoStatusKnown}
			{repoMissing}
			{repoDirty}
			{summary}
			{effectiveMode}
			{prStatus}
			{checkStats}
			{prStatusLoading}
			{prReviewsLoading}
			bind:diffMode
			onRefresh={handleRefresh}
			{onClose}
			onOpenPrUrl={validateAndOpenURL}
		/>

		<RepoDiffPrPanel
			{effectiveMode}
			bind:prPanelExpanded
			{remotes}
			{remotesLoading}
			bind:prBaseRemote
			bind:prBase
			bind:prDraft
			{prCreating}
			{prCreateStageCopy}
			{prCreateError}
			{prTracked}
			{prCreateSuccess}
			{prStatusError}
			{prReviewsSent}
			hasUncommittedChanges={localStatus?.hasUncommitted ?? false}
			{commitPushLoading}
			{commitPushStageCopy}
			{commitPushError}
			{commitPushSuccess}
			onCreatePr={handleCreatePR}
			onViewStatus={() => {
				forceMode = 'status';
			}}
			onCommitAndPush={handleCommitAndPush}
		/>

		<RepoDiffContentPane
			{summaryLoading}
			{summaryError}
			{summary}
			{localSummary}
			{selected}
			{selectedSource}
			{shouldSplitLocalPendingSection}
			{effectiveMode}
			{prStatus}
			{checkStats}
			{expandedCheck}
			{checkAnnotationsLoading}
			{formatDuration}
			{getCheckStatusClass}
			{toggleCheckExpansion}
			{navigateToAnnotationFile}
			{getFilteredAnnotations}
			{reviewCountForFile}
			{selectFile}
			onOpenDetailsUrl={(url) => Browser.OpenURL(url)}
			{sidebarWidth}
			{isResizing}
			onStartResize={startResize}
			{fileMeta}
			{fileLoading}
			{rendererLoading}
			{fileError}
			{rendererError}
			bind:diffContainer
			onRetrySummary={loadSummary}
		/>
	</section>
{:else}
	<div class="state">Select a repo to view diffs.</div>
{/if}

{#if authModalOpen}
	<RepoDiffAuthModal
		notice={authModalMessage}
		onClose={handleAuthClose}
		onSuccess={handleAuthSuccess}
	/>
{/if}

<RepoDiffAnnotationStyles />

<style>
	.diff {
		display: flex;
		flex-direction: column;
		gap: 16px;
		height: 100%;
		padding: 16px;
	}

	.state {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 20px;
		color: var(--muted);
	}
</style>
