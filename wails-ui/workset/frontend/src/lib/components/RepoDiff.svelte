<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
	import {
		CheckCircle2,
		XCircle,
		Loader2,
		Ban,
		MinusCircle,
		ChevronDown,
		ChevronRight,
		ExternalLink,
	} from '@lucide/svelte';
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
	import { resolveBranchRefs } from '../diff/branchRefs';
	import {
		deleteReviewComment,
		editReviewComment,
		fetchGitHubOperationStatus,
		fetchCheckAnnotations,
		fetchCurrentGitHubUser,
		fetchTrackedPullRequest,
		fetchPullRequestReviews,
		fetchPullRequestStatus,
		fetchRepoLocalStatus,
		fetchRepoDiffSummary,
		fetchRepoFileDiff,
		fetchBranchDiffSummary,
		fetchBranchFileDiff,
		listRemotes,
		replyToReviewComment,
		resolveReviewThread,
		startCommitAndPushAsync,
		startCreatePullRequestAsync,
		startRepoDiffWatch,
		updateRepoDiffWatch,
		stopRepoDiffWatch,
	} from '../api';
	import type { GitHubOperationStatus, GitHubOperationStage, RepoLocalStatus } from '../api';
	import GitHubLoginModal from './GitHubLoginModal.svelte';
	import { formatPath } from '../pathUtils';
	import { getPrCreateStageCopy } from '../prCreateProgress';
	import type { PrCreateStage } from '../prCreateProgress';
	import { applyRepoDiffSummary, applyRepoLocalStatus } from '../state';
	import type { DiffLineAnnotation, ReviewAnnotation } from './repo-diff/annotations';
	import { buildLineAnnotations } from './repo-diff/annotations';
	import { createPrStatusController } from './repo-diff/prStatusController';
	import {
		applyPrReviewsLifecycleEvent,
		applyPrStatusLifecycleEvent,
		applyTrackedPullRequestContext,
		buildGitHubOperationsStateSurface,
		buildPrStatusStateSurface,
		resolveEffectivePrMode,
	} from './repo-diff/prOrchestrationSurface';
	import {
		createRepoDiffFileController,
		type BranchDiffRefs,
		type SummarySource,
	} from './repo-diff/fileDiffController';
	import { createGitHubOperationsController } from './repo-diff/githubOperationsController';
	import { createReviewAnnotationActionsController } from './repo-diff/reviewAnnotationActions';
	import { createSummaryController } from './repo-diff/summaryController';
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

	/**
	 * Validates and opens URL only if it belongs to trusted GitHub domains.
	 */
	function validateAndOpenURL(url: string | undefined | null): void {
		if (!url) return;

		try {
			const parsed = new URL(url);
			const hostname = parsed.hostname.toLowerCase();

			// Allow github.com and subdomains (for GitHub Enterprise)
			if (hostname === 'github.com' || hostname.endsWith('.github.com')) {
				BrowserOpenURL(url);
			}
		} catch {
			// Invalid URL - silently ignore
		}
	}

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

	// Sidebar tab: 'files' or 'checks'
	let sidebarTab: 'files' | 'checks' = $state('files');

	// Check annotations state
	let expandedCheck: string | null = $state(null);
	let checkAnnotations: Record<string, CheckAnnotation[]> = $state({});
	let checkAnnotationsLoading: Record<string, boolean> = $state({});

	// Sidebar resize state
	let sidebarWidth = $state(REPO_DIFF_DEFAULT_SIDEBAR_WIDTH);
	let isResizing = $state(false);

	const sidebarResizeController = createSidebarResizeController({
		document,
		window,
		storage: localStorage,
		storageKey: REPO_DIFF_SIDEBAR_WIDTH_KEY,
		minWidth: REPO_DIFF_MIN_SIDEBAR_WIDTH,
		getSidebarWidth: () => sidebarWidth,
		setSidebarWidth: (value) => {
			sidebarWidth = value;
		},
		setIsResizing: (value) => {
			isResizing = value;
		},
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
		parseNumber: (value) => parseNumber(value),
		getCurrentUserId: () => currentUserId,
		getPrReviews: () => prReviews,
		setPrReviews: (value) => {
			prReviews = value;
		},
		replyToReviewComment,
		editReviewComment,
		handleDeleteComment: (commentId) => handleDeleteComment(commentId),
		handleResolveThread: (threadId, resolve) => handleResolveThread(threadId, resolve),
		formatError: (error, fallback) => formatError(error, fallback),
		showAlert: (message) => alert(message),
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

	const statusLabel = (status: string): string => {
		switch (status) {
			case 'added':
				return 'added';
			case 'deleted':
				return 'deleted';
			case 'renamed':
				return 'renamed';
			case 'untracked':
				return 'untracked';
			case 'binary':
				return 'binary';
			default:
				return 'modified';
		}
	};

	const startResize = (event: MouseEvent): void => sidebarResizeController.startResize(event);

	const formatError = (err: unknown, fallback: string): string => {
		if (err instanceof Error) return err.message;
		if (typeof err === 'string') return err;
		if (err && typeof err === 'object' && 'message' in err) {
			const message = (err as { message?: string }).message;
			if (typeof message === 'string') return message;
		}
		return fallback;
	};

	const parseNumber = (value: string): number | undefined => {
		const parsed = Number.parseInt(value.trim(), 10);
		return Number.isFinite(parsed) ? parsed : undefined;
	};

	const commitPushStageCopy = $derived.by(() => {
		switch (commitPushStage) {
			case 'queued':
				return 'Preparing...';
			case 'generating_message':
				return 'Generating message...';
			case 'staging':
				return 'Staging changes...';
			case 'committing':
				return 'Committing...';
			case 'pushing':
				return 'Pushing...';
			default:
				return 'Committing...';
		}
	});

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
			setAuthModalOpen: (value) => {
				authModalOpen = value;
			},
			setAuthModalMessage: (value) => {
				authModalMessage = value;
			},
			setAuthPendingAction: (value) => {
				authPendingAction = value;
			},
			setPrPanelExpanded: (value) => {
				prPanelExpanded = value;
			},
			setPrCreating: (value) => {
				prCreating = value;
			},
			setPrCreatingStage: (value) => {
				prCreatingStage = value;
			},
			setPrCreateError: (value) => {
				prCreateError = value;
			},
			setPrCreateSuccess: (value) => {
				prCreateSuccess = value;
			},
			setPrTracked: (value) => {
				prTracked = value;
			},
			setForceMode: (value) => {
				forceMode = value;
			},
			setPrNumberInput: (value) => {
				prNumberInput = value;
			},
			setPrStatus: (value) => {
				prStatus = value;
			},
			setCommitPushLoading: (value) => {
				commitPushLoading = value;
			},
			setCommitPushStage: (value) => {
				commitPushStage = value;
			},
			setCommitPushError: (value) => {
				commitPushError = value;
			},
			setCommitPushSuccess: (value) => {
				commitPushSuccess = value;
			},
		}),
		handledOperationCompletions,
		handleRefresh: () => handleRefresh(),
		formatError,
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

	const loadRemotes = async (): Promise<void> => {
		if (!repoId) return;
		remotesLoading = true;
		try {
			remotes = await listRemotes(workspaceId, repoId);
		} catch {
			// Non-fatal: remotes loading is optional
			remotes = [];
		} finally {
			remotesLoading = false;
		}
	};

	const handleCommitAndPush = (): Promise<void> => githubOperationsController.handleCommitAndPush();

	const handleCreatePR = (): Promise<void> => githubOperationsController.handleCreatePR();

	const handleDeleteComment = async (commentId: number): Promise<void> => {
		if (!repoId) return;
		// Note: native confirm() doesn't work in Wails WebView on macOS
		// The delete button click is explicit enough user intent
		await runGitHubAction(
			async () => {
				await deleteReviewComment(workspaceId, repoId, commentId);
				await loadPrReviews();
			},
			(message) => {
				alert(message);
			},
			'Failed to delete comment.',
		);
	};

	let resolvingThread = $state(false);

	const handleResolveThread = async (threadId: string, resolve: boolean): Promise<void> => {
		if (!repoId) return;
		if (!threadId) {
			alert('No thread ID found for this comment');
			return;
		}
		if (resolvingThread) return;
		resolvingThread = true;
		await runGitHubAction(
			async () => {
				await resolveReviewThread(workspaceId, repoId, threadId, resolve);
				// Refresh reviews to get updated state
				await loadPrReviews();
			},
			(message) => {
				alert(message);
			},
			resolve ? 'Failed to resolve thread.' : 'Failed to unresolve thread.',
		);
		resolvingThread = false;
	};

	const filteredReviews = $derived(
		prReviews.filter((comment) => (selected?.path ? comment.path === selected.path : true)),
	);

	// Check stats for compact display
	const checkStats = $derived.by(() => getCheckStats(prStatus?.checks ?? []));

	const reviewCountsByPath = $derived.by(() => {
		const counts = new Map<string, number>();
		for (const comment of prReviews) {
			counts.set(comment.path, (counts.get(comment.path) ?? 0) + 1);
		}
		return counts;
	});

	// Count reviews for a specific file path
	const reviewCountForFile = (path: string): number => {
		return reviewCountsByPath.get(path) ?? 0;
	};

	const ensureRenderer = async (): Promise<void> => {
		if (diffModule || rendererLoading) return;
		rendererLoading = true;
		rendererError = null;
		try {
			diffModule = (await import('@pierre/diffs')) as DiffsModule;
		} catch (err) {
			rendererError = formatError(err, 'Diff renderer failed to load.');
		} finally {
			rendererLoading = false;
		}
	};

	const loadTrackedPR = async (): Promise<void> => {
		if (!repoId) return;
		try {
			const tracked = await fetchTrackedPullRequest(workspaceId, repoId);
			if (!tracked) {
				return;
			}
			applyTrackedPullRequestContext(tracked, {
				setPrTracked: (value) => {
					prTracked = value;
				},
				prNumberInput: () => prNumberInput,
				setPrNumberInput: (value) => {
					prNumberInput = value;
				},
				prBranchInput: () => prBranchInput,
				setPrBranchInput: (value) => {
					prBranchInput = value;
				},
			});
			void loadPrStatus();
			void loadPrReviews();
			void loadLocalStatus().then(() => loadLocalSummary());
		} catch {
			// ignore tracking failures
		}
	};

	const diffRenderController = createDiffRenderController<ReviewAnnotation>({
		getDiffModule: () => diffModule,
		getSelectedDiff: () => selectedDiff,
		getDiffContainer: () => diffContainer,
		buildOptions,
		getLineAnnotations: () => buildLineAnnotations(filteredReviews),
		requestAnimationFrame,
		setTimeout,
	});

	const renderDiff = (): void => diffRenderController.renderDiff();

	// Check if we should use branch diff (when PR exists with branches)
	const useBranchDiff = (): BranchDiffRefs | null => {
		return resolveBranchRefs(remotes, prStatus?.pullRequest ?? prTracked);
	};

	const fileDiffController = createRepoDiffFileController({
		workspaceId: () => workspaceId,
		repoId: () => repoId,
		selectedSource: () => selectedSource,
		useBranchDiff,
		setSelected: (value) => {
			selected = value;
		},
		setSelectedSource: (value) => {
			selectedSource = value;
		},
		setSelectedDiff: (value) => {
			selectedDiff = value;
		},
		setFileMeta: (value) => {
			fileMeta = value;
		},
		setFileLoading: (value) => {
			fileLoading = value;
		},
		setFileError: (value) => {
			fileError = value;
		},
		ensureRenderer,
		getDiffModule: () => diffModule,
		getRendererError: () => rendererError,
		fetchRepoFileDiff,
		fetchBranchFileDiff,
		formatError,
		requestAnimationFrame,
		renderDiff,
	});

	const queueRenderDiff = (): void => fileDiffController.queueRenderDiff();

	const selectFile = (file: RepoDiffFileSummary, source: SummarySource = 'pr'): void =>
		fileDiffController.selectFile(file, source);

	const checkSidebarController = createCheckSidebarController({
		getExpandedCheck: () => expandedCheck,
		setExpandedCheck: (value) => {
			expandedCheck = value;
		},
		getCheckAnnotations: () => checkAnnotations,
		setCheckAnnotations: (value) => {
			checkAnnotations = value;
		},
		getCheckAnnotationsLoading: () => checkAnnotationsLoading,
		setCheckAnnotationsLoading: (value) => {
			checkAnnotationsLoading = value;
		},
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
		// eslint-disable-next-line no-console
		logError: (...args) => console.error(...args),
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

	const summaryController = createSummaryController({
		workspaceId: () => workspaceId,
		repoId: () => repoId,
		repoStatusKnown: () => repoStatusKnown,
		repoMissing: () => repoMissing,
		localHasUncommitted: () => localStatus?.hasUncommitted ?? false,
		selected: () => selected,
		selectedSource: () => selectedSource,
		summary: () => summary,
		setSummary: (value) => {
			summary = value;
		},
		setSummaryLoading: (value) => {
			summaryLoading = value;
		},
		setSummaryError: (value) => {
			summaryError = value;
		},
		setLocalSummary: (value) => {
			localSummary = value;
		},
		setSelected: (value) => {
			selected = value;
		},
		setSelectedDiff: (value) => {
			selectedDiff = value;
		},
		setFileMeta: (value) => {
			fileMeta = value;
		},
		setFileError: (value) => {
			fileError = value;
		},
		selectFile,
		useBranchDiff,
		fetchRepoDiffSummary,
		fetchBranchDiffSummary,
		applyRepoDiffSummary,
		formatError,
	});

	const loadSummary = (): Promise<void> => summaryController.loadSummary();
	const loadLocalSummary = (): Promise<void> => summaryController.loadLocalSummary();
	const applySummaryUpdate = (data: RepoDiffSummary, source: SummarySource): void =>
		summaryController.applySummaryUpdate(data, source);
	const prStatusController = createPrStatusController({
		...buildPrStatusStateSurface({
			workspaceId: () => workspaceId,
			repoId: () => repoId,
			prNumberInput: () => prNumberInput,
			prBranchInput: () => prBranchInput,
			effectiveMode: () => effectiveMode,
			currentUserId: () => currentUserId,
			setCurrentUserId: (value) => {
				currentUserId = value;
			},
			setPrStatus: (value) => {
				prStatus = value;
			},
			setPrStatusLoading: (value) => {
				prStatusLoading = value;
			},
			setPrStatusError: (value) => {
				prStatusError = value;
			},
			setPrReviews: (value) => {
				prReviews = value;
			},
			setPrReviewsLoading: (value) => {
				prReviewsLoading = value;
			},
			setPrReviewsSent: (value) => {
				prReviewsSent = value;
			},
			setLocalStatus: (value) => {
				localStatus = value;
			},
		}),
		parseNumber,
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
			if (!payload.status.hasUncommitted) {
				localSummary = null;
			}
			applyRepoLocalStatus(payload.workspaceId, payload.repoId, payload.status);
		},
		onPrStatusEvent: (payload) => {
			applyPrStatusLifecycleEvent(payload, {
				setPrStatus: (value) => {
					prStatus = value;
				},
				setPrStatusError: (value) => {
					prStatusError = value;
				},
				setPrStatusLoading: (value) => {
					prStatusLoading = value;
				},
			});
		},
		onPrReviewsEvent: (payload) => {
			applyPrReviewsLifecycleEvent(payload, {
				setPrReviews: (value) => {
					prReviews = value;
				},
				setPrReviewsLoading: (value) => {
					prReviewsLoading = value;
				},
				setPrReviewsSent: (value) => {
					prReviewsSent = value;
				},
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
			prNumber: parseNumber(prNumberInput),
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

	// Reload diff when PR branch info becomes available (switch from local to branch diff)
	let lastBranchKey: string | null = null;
	$effect(() => {
		const branchRefs = useBranchDiff();
		const newKey = branchRefs ? `${branchRefs.base}..${branchRefs.head}` : null;
		if (newKey !== lastBranchKey && newKey !== null) {
			lastBranchKey = newKey;
			// Reload summary with branch diff
			void loadSummary();
		}
	});

	$effect(() => {
		repoDiffLifecycle.syncWatchUpdate({
			workspaceId,
			repoId,
			prNumber: parseNumber(prNumberInput),
			prBranch: prBranchInput.trim() || undefined,
		});
	});
</script>

{#if repo}
	<section class="diff">
		<header class="diff-header">
			<div class="title">
				<div class="repo-name">{repoName}</div>
				<div class="meta">
					{#if repoDefaultBranch}
						<span>Default branch: {repoDefaultBranch}</span>
					{/if}
					{#if repoStatusKnown === false}
						<span class="status unknown">unknown</span>
					{:else if repoMissing}
						<span class="status missing">missing</span>
					{:else if repoDirty}
						<span class="status dirty">dirty</span>
					{:else}
						<span class="status clean">clean</span>
					{/if}
					{#if summary}
						<span>Files: {summary.files.length}</span>
						<span class="diffstat"
							><span class="add">+{summary.totalAdded}</span><span class="sep">/</span><span
								class="del">-{summary.totalRemoved}</span
							></span
						>
					{/if}
					<!-- PR status badge (inline in header when in status mode) -->
					{#if effectiveMode === 'status' && prStatus}
						<button
							class="pr-badge"
							type="button"
							onclick={() => validateAndOpenURL(prStatus?.pullRequest?.url)}
							title="Open PR #{prStatus.pullRequest.number} on GitHub"
						>
							<span class="pr-badge-number">PR #{prStatus.pullRequest.number}</span>
							<span class="pr-badge-state pr-badge-state-{prStatus.pullRequest.state.toLowerCase()}"
								>{prStatus.pullRequest.state}</span
							>
							<span class="pr-badge-divider">·</span>
							{#if checkStats.total === 0}
								<span class="pr-badge-checks muted">No checks</span>
							{:else if checkStats.failed > 0}
								<span class="pr-badge-checks failed"
									><svg
										class="icon"
										viewBox="0 0 24 24"
										fill="none"
										stroke="currentColor"
										stroke-width="2"
										><circle cx="12" cy="12" r="10" /><path d="m15 9-6 6" /><path
											d="m9 9 6 6"
										/></svg
									>
									{checkStats.failed}</span
								>
							{:else if checkStats.pending > 0}
								<span class="pr-badge-checks pending"
									><svg
										class="icon spin"
										viewBox="0 0 24 24"
										fill="none"
										stroke="currentColor"
										stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56" /></svg
									>
									{checkStats.pending}</span
								>
							{:else}
								<span class="pr-badge-checks passed"
									><svg
										class="icon"
										viewBox="0 0 24 24"
										fill="none"
										stroke="currentColor"
										stroke-width="2"
										><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" /><path
											d="M22 4 12 14.01l-3-3"
										/></svg
									>
									{checkStats.passed}</span
								>
							{/if}
							{#if prStatusLoading || prReviewsLoading}
								<span class="pr-badge-sync"></span>
							{/if}
						</button>
					{:else if effectiveMode === 'status' && prStatusLoading}
						<span class="pr-badge-loading">PR...</span>
					{/if}
				</div>
			</div>
			<div class="controls">
				<div class="toggle">
					<button
						class:active={diffMode === 'split'}
						onclick={() => {
							diffMode = 'split';
						}}
						type="button"
					>
						Split
					</button>
					<button
						class:active={diffMode === 'unified'}
						onclick={() => {
							diffMode = 'unified';
						}}
						type="button"
					>
						Unified
					</button>
				</div>
				<button class="ghost" type="button" onclick={handleRefresh}>Refresh</button>
				<button class="close" onclick={onClose} type="button">Back to terminal</button>
			</div>
		</header>

		<!-- PR Create form (only shown in create mode) -->
		{#if effectiveMode === 'create'}
			<section class="pr-panel">
				<button
					class="pr-panel-toggle"
					type="button"
					onclick={() => (prPanelExpanded = !prPanelExpanded)}
				>
					<span class="pr-panel-toggle-icon">{prPanelExpanded ? '▾' : '▸'}</span>
					<span class="pr-title">Create Pull Request</span>
				</button>

				<div class="pr-panel-content" class:expanded={prPanelExpanded}>
					<div class="pr-panel-inner">
						<div class="pr-form-row">
							<label class="field-inline">
								<span>Target</span>
								<select
									bind:value={prBaseRemote}
									disabled={remotesLoading}
									title="Base remote (defaults to upstream if available)"
								>
									<option value="">Auto</option>
									{#each remotes as remote (remote.name)}
										<option value={remote.name}>{remote.name}</option>
									{/each}
								</select>
							</label>
							<span class="field-separator">/</span>
							<label class="field-inline">
								<input
									class="branch-input"
									type="text"
									bind:value={prBase}
									placeholder="main"
									autocapitalize="off"
									autocorrect="off"
									spellcheck="false"
								/>
							</label>
							<label class="checkbox-inline">
								<input type="checkbox" bind:checked={prDraft} />
								Draft
							</label>
							<button
								class="pr-create-btn"
								class:loading={prCreating}
								type="button"
								onclick={handleCreatePR}
								disabled={prCreating}
							>
								{#if prCreating}
									<span class="pr-create-spinner" aria-hidden="true">
										<svg
											class="pr-create-spinner-icon"
											viewBox="0 0 24 24"
											fill="none"
											stroke="currentColor"
											stroke-width="2"
										>
											<circle cx="12" cy="12" r="9" opacity="0.25" />
											<path d="M21 12a9 9 0 0 0-9-9" stroke-linecap="round" />
										</svg>
									</span>
								{/if}
								<span class="pr-create-label"
									>{prCreating
										? (prCreateStageCopy?.button ?? 'Creating PR...')
										: 'Create PR'}</span
								>
							</button>
						</div>

						{#if prCreating && prCreateStageCopy}
							<div class="pr-create-progress" role="status" aria-live="polite">
								{prCreateStageCopy.detail}
							</div>
						{/if}

						{#if prCreateError}
							<div class="error">{prCreateError}</div>
						{/if}

						{#if prTracked && !prCreateSuccess}
							<div class="info-banner">
								Existing PR #{prTracked.number} found.
								<button class="mode-link" type="button" onclick={() => (forceMode = 'status')}>
									View status →
								</button>
							</div>
						{/if}
					</div>
				</div>
			</section>
		{/if}

		<!-- Status errors/success banners (only in status mode) -->
		{#if effectiveMode === 'status'}
			{#if prStatusError}
				<div class="error-banner compact">{prStatusError}</div>
			{/if}
			{#if prReviewsSent}
				<div class="success-banner compact">Sent to terminal</div>
			{/if}
		{/if}

		<!-- Local uncommitted changes banner (in status mode when PR exists) -->
		{#if effectiveMode === 'status' && localStatus?.hasUncommitted}
			<section class="local-changes-banner">
				<span class="local-changes-text">You have uncommitted local changes</span>
				<button
					class="commit-push-btn"
					type="button"
					onclick={handleCommitAndPush}
					disabled={commitPushLoading}
				>
					{commitPushLoading ? commitPushStageCopy : 'Commit & Push'}
				</button>
			</section>
			{#if commitPushError}
				<div class="error-banner compact">{commitPushError}</div>
			{/if}
			{#if commitPushSuccess}
				<div class="success-banner compact">Changes committed and pushed</div>
			{/if}
		{/if}

		{#if summaryLoading}
			<div class="state">Loading diff summary...</div>
		{:else if summaryError}
			<div class="state error">
				<div class="message">{summaryError}</div>
				<button class="ghost" type="button" onclick={loadSummary}>Retry</button>
			</div>
		{:else if (!summary || summary.files.length === 0) && (!localSummary || localSummary.files.length === 0)}
			<div class="state">No changes detected in this repo.</div>
		{:else}
			<div class="diff-body" style="--sidebar-width: {sidebarWidth}px">
				<aside class="file-list">
					<!-- Sidebar tabs (only show when in status mode with checks) -->
					{#if effectiveMode === 'status' && prStatus && prStatus.checks.length > 0}
						<div class="sidebar-tabs">
							<button
								class="sidebar-tab"
								class:active={sidebarTab === 'files'}
								type="button"
								onclick={() => (sidebarTab = 'files')}
							>
								Files
								<span class="tab-count"
									>{(summary?.files.length ?? 0) +
										(shouldSplitLocalPendingSection ? (localSummary?.files.length ?? 0) : 0)}</span
								>
							</button>
							<button
								class="sidebar-tab"
								class:active={sidebarTab === 'checks'}
								type="button"
								onclick={() => (sidebarTab = 'checks')}
							>
								Checks
								{#if checkStats.failed > 0}
									<span class="tab-count failed"><XCircle size={12} /> {checkStats.failed}</span>
								{:else if checkStats.pending > 0}
									<span class="tab-count pending"
										><Loader2 size={12} class="spin" /> {checkStats.pending}</span
									>
								{:else}
									<span class="tab-count passed"
										><CheckCircle2 size={12} /> {checkStats.passed}</span
									>
								{/if}
							</button>
						</div>
					{/if}

					<!-- Files tab content -->
					{#if sidebarTab === 'files'}
						{#if summary && summary.files.length > 0}
							<div class="section-title">
								{shouldSplitLocalPendingSection && localSummary && localSummary.files.length > 0
									? 'PR files'
									: 'Changed files'}
							</div>
							{#each summary.files as file (file.path)}
								{@const reviewCount = reviewCountForFile(file.path)}
								<button
									class:selected={file.path === selected?.path &&
										file.prevPath === selected?.prevPath &&
										selectedSource === 'pr'}
									class="file-row"
									onclick={() => selectFile(file, 'pr')}
									type="button"
								>
									<div class="file-meta">
										<span class="path" title={file.path}>{formatPath(file.path)}</span>
										{#if file.prevPath}
											<span class="rename">from {file.prevPath}</span>
										{/if}
									</div>
									<div class="stats">
										{#if reviewCount > 0}
											<span
												class="review-badge"
												title="{reviewCount} review comment{reviewCount > 1 ? 's' : ''}"
											>
												💬 {reviewCount}
											</span>
										{/if}
										<span class="tag {file.status}">{statusLabel(file.status)}</span>
										<span class="diffstat"
											><span class="add">+{file.added}</span><span class="sep">/</span><span
												class="del">-{file.removed}</span
											></span
										>
									</div>
								</button>
							{/each}
						{/if}

						{#if shouldSplitLocalPendingSection && localSummary && localSummary.files.length > 0}
							<div class="section-title local-section-title">Local pending changes</div>
							{#each localSummary.files as file (`local:${file.path}:${file.prevPath ?? ''}`)}
								<button
									class:selected={file.path === selected?.path &&
										file.prevPath === selected?.prevPath &&
										selectedSource === 'local'}
									class="file-row local-file"
									onclick={() => selectFile(file, 'local')}
									type="button"
								>
									<div class="file-meta">
										<span class="path" title={file.path}>{formatPath(file.path)}</span>
										{#if file.prevPath}
											<span class="rename">from {file.prevPath}</span>
										{/if}
									</div>
									<div class="stats">
										<span class="tag {file.status} local-tag">{statusLabel(file.status)}</span>
										<span class="diffstat local-diffstat"
											><span class="add">+{file.added}</span><span class="sep">/</span><span
												class="del">-{file.removed}</span
											></span
										>
									</div>
								</button>
							{/each}
						{/if}
					{/if}

					<!-- Checks tab content -->
					{#if sidebarTab === 'checks' && prStatus}
						<div class="checks-tab-content">
							<!-- Checks Summary Header -->
							<div class="checks-summary">
								<div class="checks-summary-item passed">
									<CheckCircle2 size={16} />
									<span>{checkStats.passed}</span>
								</div>
								<div class="checks-summary-item failed">
									<XCircle size={16} />
									<span>{checkStats.failed}</span>
								</div>
								{#if checkStats.pending > 0}
									<div class="checks-summary-item pending">
										<Loader2 size={16} class="spin" />
										<span>{checkStats.pending}</span>
									</div>
								{/if}
							</div>

							<!-- Check List -->
							<div class="checks-list">
								{#each prStatus.checks as check (check.name)}
									{@const statusClass = getCheckStatusClass(check.conclusion, check.status)}
									{@const isFailed = check.conclusion === 'failure'}
									{@const isExpanded = expandedCheck === check.name}
									{@const filteredResult = getFilteredAnnotations(check.name)}
									{@const hasAnnotations = filteredResult.annotations.length > 0}
									{@const isLoadingAnnotations = checkAnnotationsLoading[check.name]}
									<div class="check-item-container">
										<!-- Use div for non-failed checks, button for failed checks -->
										{#if isFailed}
											<button
												class="check-row {statusClass} expandable"
												type="button"
												onclick={() => toggleCheckExpansion(check)}
											>
												<span class="check-indicator {statusClass}">
													{#if check.conclusion === 'success'}
														<CheckCircle2 size={16} />
													{:else if check.conclusion === 'failure'}
														<XCircle size={16} />
													{:else if check.conclusion === 'skipped'}
														<Ban size={16} />
													{:else if check.conclusion === 'cancelled'}
														<Ban size={16} />
													{:else if check.conclusion === 'neutral'}
														<MinusCircle size={16} />
													{:else if check.status === 'in_progress' || check.status === 'queued'}
														<Loader2 size={16} class="spin" />
													{:else}
														<MinusCircle size={16} />
													{/if}
												</span>
												<span class="check-name">{check.name}</span>
												{#if check.startedAt && check.completedAt}
													{@const duration =
														new Date(check.completedAt).getTime() -
														new Date(check.startedAt).getTime()}
													<span class="check-duration" title="Duration">
														{formatDuration(duration)}
													</span>
												{/if}
												<span class="check-expand-icon">
													{#if isExpanded}
														<ChevronDown size={16} />
													{:else}
														<ChevronRight size={16} />
													{/if}
												</span>
												{#if check.detailsUrl}
													<a
														class="check-link"
														href={check.detailsUrl}
														target="_blank"
														rel="noopener noreferrer"
														onclick={(e) => {
															e.stopPropagation();
															if (check.detailsUrl) BrowserOpenURL(check.detailsUrl);
														}}
														title="View on GitHub"
													>
														<ExternalLink size={14} />
													</a>
												{/if}
											</button>
										{:else}
											<div class="check-row {statusClass}">
												<span class="check-indicator {statusClass}">
													{#if check.conclusion === 'success'}
														<CheckCircle2 size={16} />
													{:else if check.conclusion === 'failure'}
														<XCircle size={16} />
													{:else if check.conclusion === 'skipped'}
														<Ban size={16} />
													{:else if check.conclusion === 'cancelled'}
														<Ban size={16} />
													{:else if check.conclusion === 'neutral'}
														<MinusCircle size={16} />
													{:else if check.status === 'in_progress' || check.status === 'queued'}
														<Loader2 size={16} class="spin" />
													{:else}
														<MinusCircle size={16} />
													{/if}
												</span>
												<span class="check-name">{check.name}</span>
												{#if check.startedAt && check.completedAt}
													{@const duration =
														new Date(check.completedAt).getTime() -
														new Date(check.startedAt).getTime()}
													<span class="check-duration" title="Duration">
														{formatDuration(duration)}
													</span>
												{/if}
												{#if check.detailsUrl}
													<a
														class="check-link"
														href={check.detailsUrl}
														target="_blank"
														rel="noopener noreferrer"
														onclick={() => check.detailsUrl && BrowserOpenURL(check.detailsUrl)}
														title="View on GitHub"
													>
														<ExternalLink size={14} />
													</a>
												{/if}
											</div>
										{/if}

										<!-- Expanded Annotations Section -->
										{#if isFailed && isExpanded}
											<div class="check-annotations">
												{#if isLoadingAnnotations}
													<div class="check-annotations-loading">
														<Loader2 size={16} class="spin" />
														<span>Loading annotations...</span>
													</div>
												{:else if hasAnnotations}
													{#each filteredResult.annotations as annotation (annotation.path + annotation.startLine)}
														<div class="check-annotation-item level-{annotation.level}">
															<button
																class="check-annotation-path"
																type="button"
																onclick={() =>
																	navigateToAnnotationFile(annotation.path, annotation.startLine)}
															>
																<span class="path-text"
																	>{annotation.path}:{annotation.startLine}</span
																>
																{#if annotation.startLine !== annotation.endLine}
																	<span class="line-range">-{annotation.endLine}</span>
																{/if}
															</button>
															{#if annotation.title}
																<div class="check-annotation-title">{annotation.title}</div>
															{/if}
															<div class="check-annotation-message">{annotation.message}</div>
														</div>
													{/each}
													{#if filteredResult.filteredCount > 0}
														<div class="check-annotations-more">
															+{filteredResult.filteredCount} more in other files
														</div>
													{/if}
												{:else}
													<div class="check-annotations-empty">
														{#if !check.checkRunId}
															<span>Check run ID not available</span>
														{:else}
															<span>No annotations for this check</span>
														{/if}
													</div>
												{/if}
											</div>
										{/if}
									</div>
								{/each}
							</div>
						</div>
					{/if}
				</aside>
				<button
					class="resize-handle"
					class:resizing={isResizing}
					onmousedown={startResize}
					aria-label="Resize sidebar"
					type="button"
				></button>
				<div class="diff-view">
					<div class="file-header">
						<div class="file-title">
							<span>{selected?.path}</span>
							{#if selected?.prevPath}
								<span class="rename">from {selected.prevPath}</span>
							{/if}
						</div>
						<span class="diffstat">
							<span class="add">+{selected?.added ?? 0}</span><span class="sep">/</span><span
								class="del">-{selected?.removed ?? 0}</span
							>
							{#if fileMeta && !fileMeta.truncated && fileMeta.totalLines > 0}
								<span class="line-count">{fileMeta.totalLines} lines</span>
							{/if}
						</span>
					</div>
					{#if fileLoading || rendererLoading}
						<div class="state compact">Loading file diff...</div>
					{:else if fileError}
						<div class="state compact">{fileError}</div>
					{:else if rendererError}
						<div class="state compact">{rendererError}</div>
					{:else}
						<div class="diff-renderer">
							<diffs-container bind:this={diffContainer}></diffs-container>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	</section>
{:else}
	<div class="state">Select a repo to view diffs.</div>
{/if}

{#if authModalOpen}
	<div
		class="overlay"
		role="button"
		tabindex="0"
		onclick={handleAuthClose}
		onkeydown={(event) => {
			if (event.key === 'Escape') handleAuthClose();
		}}
	>
		<div
			class="overlay-panel"
			role="presentation"
			onclick={(event) => event.stopPropagation()}
			onkeydown={(event) => event.stopPropagation()}
		>
			<GitHubLoginModal
				notice={authModalMessage}
				onClose={handleAuthClose}
				onSuccess={handleAuthSuccess}
			/>
		</div>
	</div>
{/if}

<style>
	/* Sidebar tabs */
	.sidebar-tabs {
		display: flex;
		gap: 0;
		margin-bottom: 16px;
		border-bottom: 1px solid var(--border);
	}

	.sidebar-tab {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 10px 12px;
		border: none;
		border-bottom: 2px solid transparent;
		margin-bottom: -1px;
		background: transparent;
		color: var(--muted);
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.sidebar-tab:hover:not(.active) {
		color: var(--text);
	}

	.sidebar-tab.active {
		color: var(--text);
		border-bottom-color: var(--accent);
	}

	.tab-count {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 6px;
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.08);
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.tab-count.passed {
		color: #3fb950;
		background: rgba(46, 160, 67, 0.15);
	}
	.tab-count.failed {
		color: #f85149;
		background: rgba(248, 81, 73, 0.15);
	}
	.tab-count.pending {
		color: #d29922;
		background: rgba(210, 153, 34, 0.15);
	}

	.tab-count .spin {
		animation: spin 1s linear infinite;
	}

	/* Checks tab content */
	.checks-tab-content {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	/* Checks Summary Header */
	.checks-summary {
		display: flex;
		gap: 12px;
		padding: 12px;
		background: rgba(255, 255, 255, 0.03);
		border-radius: 10px;
		border: 1px solid var(--border);
	}

	.checks-summary-item {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 14px;
		font-weight: 600;
	}

	.checks-summary-item.passed {
		color: #3fb950;
	}

	.checks-summary-item.failed {
		color: #f85149;
	}

	.checks-summary-item.pending {
		color: #d29922;
	}

	.checks-summary-item .spin {
		animation: spin 1s linear infinite;
	}

	/* Check List */
	.checks-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.check-item-container {
		display: flex;
		flex-direction: column;
		gap: 0;
	}

	.check-row {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px;
		border-radius: 10px;
		font-size: 13px;
		transition: background 0.15s ease;
		border-left: 3px solid transparent;
		background: transparent;
		border: none;
		width: 100%;
		text-align: left;
		cursor: default;
	}

	.check-row.expandable {
		cursor: pointer;
	}

	.check-row:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.03);
	}

	.check-row.check-success {
		background: rgba(46, 160, 67, 0.08);
		border-left-color: #3fb950;
	}

	.check-row.check-failure {
		background: rgba(248, 81, 73, 0.08);
		border-left-color: #f85149;
	}

	.check-row.check-pending {
		background: rgba(210, 153, 34, 0.08);
		border-left-color: #d29922;
	}

	.check-row.check-neutral {
		background: rgba(139, 148, 158, 0.08);
		border-left-color: #8b949e;
	}

	.check-row .check-indicator {
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	.check-row .check-indicator.check-success {
		color: #3fb950;
	}

	.check-row .check-indicator.check-failure {
		color: #f85149;
	}

	.check-row .check-indicator.check-pending {
		color: #d29922;
	}

	.check-row .check-indicator.check-neutral {
		color: #8b949e;
	}

	.check-row .check-name {
		color: var(--text);
		font-weight: 500;
		flex: 1;
	}

	.check-row .check-duration {
		font-size: 11px;
		color: var(--muted);
		font-family: var(--font-mono);
		padding: 2px 6px;
		background: rgba(255, 255, 255, 0.05);
		border-radius: 4px;
	}

	.check-row .check-expand-icon {
		color: var(--muted);
		display: flex;
		align-items: center;
	}

	.check-row .check-link {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		opacity: 0;
		transition: all 0.15s ease;
	}

	.check-row:hover .check-link,
	.check-row:focus-within .check-link {
		opacity: 1;
	}

	.check-row .check-link:hover {
		background: rgba(255, 255, 255, 0.1);
		color: var(--text);
	}

	/* Check Annotations Section */
	.check-annotations {
		padding: 0 16px 16px 56px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.check-annotations-loading {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 16px;
		color: var(--muted);
		font-size: 12px;
	}

	.check-annotations-loading .spin {
		animation: spin 1s linear infinite;
	}

	.check-annotations-empty {
		padding: 16px;
		color: var(--muted);
		font-size: 12px;
		font-style: italic;
	}

	.check-annotations-more {
		padding: 12px 16px;
		color: var(--muted);
		font-size: 11px;
		font-style: italic;
		text-align: center;
		border-top: 1px solid var(--panel-border, rgba(255, 255, 255, 0.05));
	}

	.check-annotation-item {
		padding: 14px 16px;
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.03);
		border-left: 3px solid transparent;
		transition:
			background 0.15s ease,
			transform 0.1s ease;
	}

	.check-annotation-item:hover {
		background: rgba(255, 255, 255, 0.06);
		transform: translateX(2px);
	}

	.check-annotation-item.level-notice {
		border-left-color: #58a6ff;
		background: rgba(88, 166, 255, 0.08);
	}

	.check-annotation-item.level-warning {
		border-left-color: #d29922;
		background: rgba(210, 153, 34, 0.08);
	}

	.check-annotation-item.level-failure {
		border-left-color: #f85149;
		background: rgba(248, 81, 73, 0.08);
	}

	.check-annotation-path {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		font-family: var(--font-mono);
		color: var(--accent);
		background: none;
		border: none;
		padding: 4px 0;
		cursor: pointer;
		text-align: left;
		margin-bottom: 8px;
		font-weight: 500;
	}

	.check-annotation-path:hover {
		color: var(--text);
		text-decoration: underline;
	}

	.check-annotation-path .line-range {
		color: var(--muted);
	}

	.check-annotation-title {
		font-size: 13px;
		font-weight: 600;
		color: var(--text);
		margin-bottom: 6px;
	}

	.check-annotation-message {
		font-size: 12px;
		color: var(--muted);
		line-height: 1.6;
		white-space: pre-wrap;
	}

	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}

	/* Local changes warning banner */
	.local-changes-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 12px 16px;
		border-radius: 10px;
		background: rgba(210, 153, 34, 0.12);
		border: 1px solid rgba(210, 153, 34, 0.35);
	}

	.local-changes-text {
		font-size: 13px;
		font-weight: 500;
		color: #d29922;
	}

	/* Local files section styling (yellow) */
	.local-section-title {
		color: #d29922 !important;
	}

	.file-row.local-file {
		border-left: 2px solid rgba(210, 153, 34, 0.5);
		background: rgba(210, 153, 34, 0.06);
	}

	.file-row.local-file:hover:not(.selected) {
		border-color: rgba(210, 153, 34, 0.4);
		background: rgba(210, 153, 34, 0.12);
		border-left-color: #d29922;
	}

	.file-row.local-file.selected {
		background: rgba(210, 153, 34, 0.18);
		border-color: rgba(210, 153, 34, 0.5);
		border-left-color: #d29922;
	}

	.local-tag {
		color: #d29922 !important;
	}

	.local-diffstat .add {
		color: #d29922 !important;
	}

	.local-diffstat .del {
		color: #d29922 !important;
		opacity: 0.7;
	}

	.commit-push-btn {
		padding: 8px 16px;
		border-radius: 8px;
		border: none;
		background: linear-gradient(135deg, #d29922 0%, #b8860b 100%);
		color: #1a1a1a;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		transition:
			transform 0.15s ease,
			box-shadow 0.15s ease,
			opacity 0.15s ease;
	}

	.commit-push-btn:hover:not(:disabled) {
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(210, 153, 34, 0.3);
	}

	.commit-push-btn:active:not(:disabled) {
		transform: translateY(0);
	}

	.commit-push-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.pr-panel {
		border-radius: 14px;
		background: var(--panel);
		border: 1px solid var(--border);
		overflow: hidden;
	}

	.pr-panel-toggle {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 14px 16px;
		background: transparent;
		border: none;
		cursor: pointer;
		text-align: left;
		transition: background 0.15s ease;
	}

	.pr-panel-toggle:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.pr-panel-toggle-icon {
		font-size: 12px;
		color: var(--muted);
		width: 12px;
	}

	.pr-title {
		font-weight: 600;
		font-size: 14px;
		color: var(--text);
	}

	.pr-panel-content {
		display: grid;
		grid-template-rows: 0fr;
		transition: grid-template-rows 0.2s ease;
	}

	.pr-panel-content.expanded {
		grid-template-rows: 1fr;
	}

	.pr-panel-inner {
		overflow: hidden;
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 0 16px 14px;
	}

	.pr-form-row {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.field-inline {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--muted);
	}

	.field-inline span {
		white-space: nowrap;
	}

	.field-inline input,
	.field-inline select {
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 6px 10px;
		color: var(--text);
		font-size: 13px;
		font-family: inherit;
	}

	.field-inline select {
		cursor: pointer;
		appearance: none;
		background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%238b949e' d='M3 4.5L6 7.5L9 4.5'/%3E%3C/svg%3E");
		background-repeat: no-repeat;
		background-position: right 8px center;
		padding-right: 26px;
		min-width: 80px;
	}

	.field-inline .branch-input {
		width: 120px;
	}

	.field-separator {
		color: var(--muted);
		font-size: 14px;
	}

	.checkbox-inline {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--muted);
		white-space: nowrap;
	}

	.pr-create-btn {
		padding: 6px 14px;
		border-radius: 8px;
		border: none;
		background: var(--accent);
		color: var(--text);
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
		white-space: nowrap;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		transition: opacity 0.15s ease;
	}

	.pr-create-btn.loading {
		animation: pr-create-pulse 1.6s ease-in-out infinite;
	}

	.pr-create-btn:hover:not(:disabled) {
		opacity: 0.9;
	}

	.pr-create-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.pr-create-spinner {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		animation: pr-create-glow 1.6s ease-in-out infinite;
	}

	.pr-create-spinner-icon {
		width: 12px;
		height: 12px;
		animation: pr-create-spin 0.8s linear infinite;
	}

	.pr-create-progress {
		font-size: 12px;
		color: var(--text);
		opacity: 0.75;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		background: var(--panel-soft);
		border-radius: 8px;
		border: 1px solid var(--border);
		border-left: 3px solid var(--accent);
	}

	@keyframes pr-create-spin {
		to {
			transform: rotate(360deg);
		}
	}

	@keyframes pr-create-glow {
		0%,
		100% {
			opacity: 0.6;
		}
		50% {
			opacity: 1;
		}
	}

	@keyframes pr-create-pulse {
		0%,
		100% {
			box-shadow: 0 0 0 rgba(0, 0, 0, 0);
		}
		50% {
			box-shadow: 0 0 0 4px rgba(255, 255, 255, 0.08);
		}
	}

	.poll-time {
		margin-left: auto;
		font-size: 11px;
		color: var(--muted);
	}

	.info-banner {
		font-size: 12px;
		color: var(--muted);
		padding: 8px 10px;
		background: var(--panel-soft);
		border-radius: 8px;
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.mode-link {
		font-size: 12px;
		color: var(--muted);
		cursor: pointer;
		background: none;
		border: none;
		padding: 0;
	}

	.mode-link:hover {
		color: var(--text);
	}

	.row {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
		gap: 10px;
	}

	.checkbox {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--text);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}

	.actions button {
		border-radius: 10px;
		border: 1px solid var(--border);
		background: var(--accent);
		color: var(--text);
		padding: 8px 12px;
		font-size: 12px;
		cursor: pointer;
	}

	/* Inline PR Badge (shown in header meta) */
	.pr-badge {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 5px 12px;
		border-radius: 999px;
		background: rgba(99, 102, 241, 0.1);
		border: 1px solid rgba(99, 102, 241, 0.25);
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.pr-badge:hover {
		background: rgba(99, 102, 241, 0.18);
		border-color: rgba(99, 102, 241, 0.4);
	}

	.pr-badge-number {
		color: var(--text);
		font-weight: 600;
	}

	.pr-badge-state {
		text-transform: uppercase;
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.04em;
	}

	.pr-badge-state-open {
		color: #3fb950;
	}
	.pr-badge-state-closed {
		color: #a78bfa;
	}
	.pr-badge-state-merged {
		color: #a78bfa;
	}

	.pr-badge-divider {
		color: var(--muted);
		opacity: 0.5;
	}

	.pr-badge-checks {
		font-size: 12px;
		font-weight: 500;
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.pr-badge-checks.passed {
		color: #3fb950;
	}
	.pr-badge-checks.failed {
		color: #f85149;
	}
	.pr-badge-checks.pending {
		color: #d29922;
	}
	.pr-badge-checks.muted {
		color: var(--muted);
	}

	.pr-badge-checks .spin {
		animation: spin 1s linear infinite;
	}

	.pr-badge-checks svg {
		width: 14px;
		height: 14px;
		flex-shrink: 0;
	}

	.pr-badge-sync {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--accent);
		animation: pulse 1.5s ease infinite;
	}

	.pr-badge-loading {
		font-size: 13px;
		color: var(--muted);
	}

	@keyframes pulse {
		0%,
		100% {
			opacity: 0.4;
		}
		50% {
			opacity: 1;
		}
	}

	.btn-text {
		background: none;
		border: none;
		padding: 0;
		font-size: 12px;
		color: var(--accent);
		cursor: pointer;
		transition: color 0.15s ease;
	}

	.btn-text:hover:not(:disabled) {
		color: var(--text);
	}

	.btn-text:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-text.muted {
		color: var(--muted);
	}

	.loading-skeleton.horizontal {
		display: flex;
		gap: 12px;
		padding: 8px 0;
	}

	.skeleton-line.short {
		width: 60px;
	}

	/* Colored Badges */
	.badge {
		padding: 3px 8px;
		border-radius: 999px;
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}

	.badge-open {
		background: rgba(46, 160, 67, 0.15);
		color: #3fb950;
		border: 1px solid rgba(46, 160, 67, 0.3);
	}

	.badge-closed {
		background: rgba(139, 92, 246, 0.15);
		color: #a78bfa;
		border: 1px solid rgba(139, 92, 246, 0.3);
	}

	.badge-merged {
		background: rgba(139, 92, 246, 0.15);
		color: #a78bfa;
		border: 1px solid rgba(139, 92, 246, 0.3);
	}

	.badge-draft {
		background: rgba(139, 148, 158, 0.15);
		color: #8b949e;
		border: 1px solid rgba(139, 148, 158, 0.3);
	}

	.badge-mergeable {
		background: rgba(46, 160, 67, 0.15);
		color: #3fb950;
		border: 1px solid rgba(46, 160, 67, 0.3);
	}

	.badge-conflicting {
		background: rgba(248, 81, 73, 0.15);
		color: #f85149;
		border: 1px solid rgba(248, 81, 73, 0.3);
	}

	.badge-unknown {
		background: rgba(139, 148, 158, 0.15);
		color: #8b949e;
		border: 1px solid rgba(139, 148, 158, 0.3);
	}

	/* Live Indicator */
	.live-indicator {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 11px;
		color: var(--accent);
	}

	.live-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--accent);
		animation: pulse 1.5s ease-in-out infinite;
	}

	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
			transform: scale(1);
		}
		50% {
			opacity: 0.5;
			transform: scale(0.8);
		}
	}

	.poll-time {
		font-size: 11px;
		color: var(--muted);
	}

	/* Loading Skeleton */
	.loading-skeleton {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 12px;
	}

	.skeleton-line {
		height: 12px;
		border-radius: 6px;
		background: linear-gradient(
			90deg,
			var(--panel-soft) 25%,
			var(--border) 50%,
			var(--panel-soft) 75%
		);
		background-size: 200% 100%;
		animation: shimmer 1.5s infinite;
	}

	.skeleton-line.wide {
		width: 80%;
	}
	.skeleton-line.medium {
		width: 50%;
	}

	@keyframes shimmer {
		0% {
			background-position: 200% 0;
		}
		100% {
			background-position: -200% 0;
		}
	}

	@keyframes fadeIn {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.branch {
		padding: 2px 6px;
		border-radius: 4px;
		background: rgba(56, 139, 253, 0.1);
		color: #58a6ff;
		font-family: var(--font-mono);
		font-size: 11px;
	}

	.arrow {
		color: var(--muted);
		font-size: 10px;
	}

	.pr-link {
		font-size: 11px;
		color: var(--accent);
		background: none;
		border: none;
		padding: 0;
		cursor: pointer;
		transition: color 0.15s ease;
	}

	.pr-link:hover {
		color: var(--text);
	}

	/* Sections */
	.section {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.section-header {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.section-title {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--muted);
	}

	.section-count {
		font-size: 10px;
		padding: 2px 6px;
		border-radius: 999px;
		background: var(--panel-soft);
		color: var(--muted);
	}

	.empty-state {
		font-size: 12px;
		color: var(--muted);
		padding: 12px;
		text-align: center;
		border: 1px dashed var(--border);
		border-radius: 8px;
	}

	/* Checks List */
	.checks-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.check-item {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		border-radius: 8px;
		background: var(--panel-soft);
		font-size: 12px;
		animation: fadeIn 0.2s ease;
	}

	.check-indicator {
		width: 18px;
		height: 18px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 10px;
		font-weight: 700;
		flex-shrink: 0;
	}

	.check-indicator.check-success {
		background: rgba(46, 160, 67, 0.2);
		color: #3fb950;
	}

	.check-indicator.check-failure {
		background: rgba(248, 81, 73, 0.2);
		color: #f85149;
	}

	.check-indicator.check-in_progress,
	.check-indicator.check-queued,
	.check-indicator.check-pending {
		background: rgba(210, 153, 34, 0.2);
		color: #d29922;
	}

	.check-indicator.check-skipped,
	.check-indicator.check-neutral {
		background: rgba(139, 148, 158, 0.2);
		color: #8b949e;
	}

	.spinner {
		width: 10px;
		height: 10px;
		border: 2px solid currentColor;
		border-top-color: transparent;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.check-name {
		flex: 1;
		color: var(--text);
		font-weight: 500;
	}

	.check-status-label {
		font-size: 11px;
		font-weight: 500;
		text-transform: capitalize;
	}

	.check-status-label.check-success {
		color: #3fb950;
	}
	.check-status-label.check-failure {
		color: #f85149;
	}
	.check-status-label.check-in_progress,
	.check-status-label.check-queued,
	.check-status-label.check-pending {
		color: #d29922;
	}
	.check-status-label.check-skipped,
	.check-status-label.check-neutral {
		color: #8b949e;
	}

	/* Error & Success Banners */
	.error-banner {
		padding: 10px 12px;
		border-radius: 8px;
		background: rgba(248, 81, 73, 0.1);
		border: 1px solid rgba(248, 81, 73, 0.3);
		color: #f85149;
		font-size: 12px;
	}

	.error-banner.compact,
	.success-banner.compact {
		padding: 6px 10px;
		font-size: 11px;
	}

	.success-banner {
		padding: 10px 12px;
		border-radius: 8px;
		background: rgba(46, 160, 67, 0.1);
		border: 1px solid rgba(46, 160, 67, 0.3);
		color: #3fb950;
		font-size: 12px;
		animation: fadeIn 0.2s ease;
	}

	.empty-state.compact {
		padding: 8px;
		font-size: 11px;
	}

	.btn-primary {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 10px 16px;
		border-radius: 10px;
		border: none;
		background: linear-gradient(135deg, var(--accent) 0%, #6366f1 100%);
		color: white;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		transition:
			transform 0.15s ease,
			box-shadow 0.15s ease;
	}

	.btn-primary:hover:not(:disabled) {
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
	}

	.btn-primary:active:not(:disabled) {
		transform: translateY(0);
	}

	.btn-primary:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-icon {
		font-size: 14px;
	}

	.btn-ghost {
		padding: 10px 16px;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: transparent;
		color: var(--muted);
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
		transition:
			border-color 0.15s ease,
			color 0.15s ease;
	}

	.btn-ghost:hover {
		border-color: var(--accent);
		color: var(--text);
	}

	/* Legacy support */
	.error {
		color: var(--danger);
		font-size: 12px;
	}

	.success {
		color: var(--success);
		font-size: 12px;
	}

	.diff {
		display: flex;
		flex-direction: column;
		gap: 16px;
		height: 100%;
		padding: 16px;
	}

	.diff-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 16px;
	}

	.title {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.repo-name {
		font-size: 20px;
		font-weight: 600;
	}

	.meta {
		display: flex;
		align-items: center;
		gap: 12px;
		color: var(--muted);
		font-size: 12px;
		flex-wrap: wrap;
	}

	.diffstat {
		font-weight: 600;
		display: inline-flex;
		gap: 8px;
		align-items: center;
	}

	.diffstat .add {
		color: var(--success);
	}

	.diffstat .del {
		color: var(--danger);
	}

	.diffstat .sep {
		color: var(--muted);
		margin: 0 -6px;
	}

	.line-count {
		font-size: 11px;
		color: var(--muted);
		font-weight: 500;
	}

	.status {
		font-weight: 600;
	}

	.dirty {
		color: var(--warning);
	}

	.missing {
		color: var(--danger);
	}

	.clean {
		color: var(--success);
	}

	.unknown {
		color: var(--muted);
	}

	.controls {
		display: flex;
		gap: 12px;
		align-items: center;
	}

	.toggle {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: 10px;
		overflow: hidden;
		background: var(--panel);
	}

	.toggle button {
		background: transparent;
		border: none;
		color: var(--muted);
		padding: 6px 12px;
		cursor: pointer;
		font-size: 12px;
		transition:
			background var(--transition-fast),
			color var(--transition-fast);
	}

	.toggle button:hover:not(.active) {
		background: rgba(255, 255, 255, 0.04);
	}

	.toggle button.active {
		color: var(--text);
		background: var(--accent-subtle);
	}

	.close {
		background: var(--panel);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-sm);
		padding: 8px 12px;
		cursor: pointer;
		transition:
			border-color var(--transition-fast),
			background var(--transition-fast);
	}

	.close:hover {
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.04);
	}

	.state {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 20px;
		color: var(--muted);
	}

	.state.compact {
		padding: 16px;
		border-radius: 12px;
		background: var(--panel-soft);
		border: 1px dashed var(--border);
		text-align: center;
	}

	.state.error {
		color: var(--warning);
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.diff-body {
		display: grid;
		grid-template-columns: var(--sidebar-width, 280px) 1fr;
		gap: 8px;
		flex: 1;
		min-height: 0;
		position: relative;
	}

	.resize-handle {
		position: absolute;
		left: calc(var(--sidebar-width, 280px) + 2px);
		top: 0;
		bottom: 0;
		width: 4px;
		background: transparent;
		border: none;
		padding: 0;
		cursor: col-resize;
		transition: background var(--transition-fast);
		z-index: 10;
		border-radius: 2px;
	}

	.resize-handle:hover,
	.resize-handle.resizing {
		background: var(--accent);
	}

	.resize-handle::after {
		content: '';
		position: absolute;
		inset: 0;
		width: 12px;
		transform: translateX(-4px);
	}

	.file-list {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		overflow: auto;
		scrollbar-width: thin;
		scrollbar-color: var(--border) transparent;
	}

	.file-list::-webkit-scrollbar {
		width: 6px;
	}

	.file-list::-webkit-scrollbar-track {
		background: transparent;
	}

	.file-list::-webkit-scrollbar-thumb {
		background: var(--border);
		border-radius: 3px;
	}

	.file-list::-webkit-scrollbar-thumb:hover {
		background: var(--accent);
	}

	.section-title {
		font-size: 11px;
		font-weight: 500;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		height: 24px;
		display: flex;
		align-items: center;
	}

	.file-row {
		display: flex;
		flex-direction: column;
		gap: 6px;
		background: transparent;
		outline: 1px solid transparent;
		outline-offset: -1px;
		color: var(--text);
		text-align: left;
		padding: 10px;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition:
			outline-color var(--transition-fast),
			background var(--transition-fast);
	}

	.file-row:hover:not(.selected) {
		outline-color: var(--border);
		background: rgba(255, 255, 255, 0.02);
	}

	.file-row.selected {
		background: var(--accent-subtle);
		outline-color: var(--accent);
	}

	.file-meta {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.path {
		font-size: 13px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.rename {
		font-size: 11px;
		color: var(--muted);
	}

	.stats {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--muted);
	}

	.review-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 6px;
		border-radius: 999px;
		background: linear-gradient(135deg, rgba(99, 102, 241, 0.15) 0%, rgba(139, 92, 246, 0.15) 100%);
		border: 1px solid rgba(99, 102, 241, 0.3);
		font-size: 10px;
		font-weight: 600;
		color: #a78bfa;
	}

	.tag {
		text-transform: uppercase;
		letter-spacing: 0.08em;
		font-size: 10px;
		font-weight: 600;
	}

	.tag.added {
		color: var(--success);
	}

	.tag.deleted {
		color: var(--danger);
	}

	.tag.renamed {
		color: var(--accent);
	}

	.tag.untracked {
		color: var(--warning);
	}

	.tag.binary {
		color: var(--muted);
	}

	.diff-view {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		overflow: hidden;
	}

	.file-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-size: 13px;
		color: var(--muted);
		height: 24px;
	}

	.file-title {
		display: flex;
		gap: 8px;
		align-items: center;
		color: var(--text);
		font-weight: 500;
	}

	.diff-renderer {
		flex: 1;
		min-height: 0;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: var(--panel-soft);
		padding: 8px;
		overflow: hidden;
		--diffs-dark-bg: var(--panel-soft);
		--diffs-dark: var(--text);
		--diffs-dark-addition-color: var(--success);
		--diffs-dark-deletion-color: var(--danger);
		--diffs-dark-modified-color: var(--accent);
		--diffs-font-family: var(--font-mono);
		--diffs-font-size: 12px;
		--diffs-header-font-family: var(--font-body);
		--diffs-gap-block: 8px;
		--diffs-gap-inline: 10px;
	}

	diffs-container {
		display: block;
		height: 100%;
		width: 100%;
		overflow: auto;
	}

	.ghost {
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		color: var(--text);
		padding: 8px 12px;
		border-radius: var(--radius-md);
		cursor: pointer;
		font-size: 12px;
		transition:
			border-color var(--transition-fast),
			background var(--transition-fast);
	}

	.ghost:hover:not(:disabled) {
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.04);
	}

	.ghost:active:not(:disabled) {
		transform: scale(0.98);
	}

	.link {
		color: var(--accent);
		text-decoration: none;
	}

	.link:hover {
		text-decoration: underline;
	}

	/* Inline Review Annotations via @pierre/diffs renderAnnotation callback */
	:global(.diff-annotation-thread) {
		margin: 8px 0;
		max-width: 720px;
		border-radius: 10px;
		overflow: hidden;
		border: 1px solid rgba(99, 102, 241, 0.25);
		border-left: 3px solid #6366f1;
		animation: fadeSlideIn 0.2s ease;
	}

	:global(.diff-annotation) {
		padding: 12px 14px;
		background: linear-gradient(135deg, rgba(99, 102, 241, 0.08) 0%, rgba(139, 92, 246, 0.08) 100%);
	}

	:global(.diff-annotation-reply) {
		background: linear-gradient(135deg, rgba(99, 102, 241, 0.04) 0%, rgba(139, 92, 246, 0.04) 100%);
		border-top: 1px solid rgba(99, 102, 241, 0.15);
		padding-left: 28px;
		position: relative;
	}

	:global(.diff-annotation-reply::before) {
		content: '';
		position: absolute;
		left: 14px;
		top: 12px;
		bottom: 12px;
		width: 2px;
		background: rgba(99, 102, 241, 0.3);
		border-radius: 1px;
	}

	@keyframes fadeSlideIn {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	:global(.diff-annotation-header) {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 8px;
	}

	:global(.diff-annotation-avatar) {
		width: 24px;
		height: 24px;
		border-radius: 50%;
		background: linear-gradient(135deg, #6366f1 0%, #a78bfa 100%);
		color: white;
		font-size: 11px;
		font-weight: 600;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	:global(.diff-annotation-reply .diff-annotation-avatar) {
		width: 20px;
		height: 20px;
		font-size: 10px;
	}

	:global(.diff-annotation-author) {
		font-size: 12px;
		font-weight: 600;
		color: var(--text);
	}

	:global(.diff-annotation-body) {
		font-size: 13px;
		line-height: 1.5;
		color: var(--text);
		white-space: pre-wrap;
		word-break: break-word;
	}

	:global(.diff-annotation-reply .diff-annotation-body) {
		font-size: 12px;
	}

	/* Comment action buttons */
	:global(.diff-annotation-actions) {
		display: flex;
		gap: 4px;
		margin-left: auto;
		opacity: 0;
		transition: opacity 0.15s ease;
	}

	:global(.diff-annotation:hover .diff-annotation-actions) {
		opacity: 1;
	}

	:global(.diff-action-btn) {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 4px;
		padding: 4px 8px;
		border: none;
		border-radius: 6px;
		background: rgba(255, 255, 255, 0.08);
		color: var(--muted);
		font-size: 11px;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	:global(.diff-action-btn:hover) {
		background: rgba(255, 255, 255, 0.15);
		color: var(--text);
	}

	:global(.diff-action-delete:hover) {
		background: rgba(248, 81, 73, 0.2);
		color: #f85149;
	}

	:global(.diff-annotation-footer) {
		display: flex;
		gap: 8px;
		padding: 10px 14px;
		background: rgba(99, 102, 241, 0.04);
		border-top: 1px solid rgba(99, 102, 241, 0.15);
	}

	:global(.diff-action-reply) {
		color: #6366f1;
	}

	:global(.diff-action-reply:hover) {
		background: rgba(99, 102, 241, 0.15);
		color: #818cf8;
	}

	:global(.diff-action-resolve) {
		color: #3fb950;
	}

	:global(.diff-action-resolve:hover) {
		background: rgba(46, 160, 67, 0.15);
		color: #4ade80;
	}

	/* Inline comment form */
	:global(.diff-annotation-inline-form) {
		padding: 12px 14px;
		background: rgba(99, 102, 241, 0.06);
		border-top: 1px solid rgba(99, 102, 241, 0.15);
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	:global(.diff-inline-textarea) {
		width: 100%;
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--panel);
		color: var(--text);
		font-family: inherit;
		font-size: 13px;
		line-height: 1.5;
		resize: vertical;
		min-height: 80px;
	}

	:global(.diff-inline-textarea:focus) {
		outline: none;
		border-color: var(--accent);
	}

	:global(.diff-inline-textarea:disabled) {
		opacity: 0.6;
		cursor: not-allowed;
	}

	:global(.diff-inline-form-actions) {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}

	:global(.diff-inline-form-actions .btn-ghost) {
		padding: 6px 12px;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		font-size: 12px;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	:global(.diff-inline-form-actions .btn-ghost:hover:not(:disabled)) {
		background: rgba(255, 255, 255, 0.05);
		color: var(--text);
	}

	:global(.diff-inline-form-actions .btn-primary) {
		padding: 6px 14px;
		border: none;
		border-radius: 6px;
		background: var(--accent);
		color: white;
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	:global(.diff-inline-form-actions .btn-primary:hover:not(:disabled)) {
		opacity: 0.9;
	}

	:global(.diff-inline-form-actions .btn-primary:disabled),
	:global(.diff-inline-form-actions .btn-ghost:disabled) {
		opacity: 0.5;
		cursor: not-allowed;
	}

	/* Resolved thread styles */
	:global(.diff-annotation-thread.diff-annotation-resolved) {
		border-color: rgba(46, 160, 67, 0.25);
		border-left-color: #3fb950;
	}

	:global(.diff-annotation-thread.diff-annotation-resolved .diff-annotation) {
		background: linear-gradient(135deg, rgba(46, 160, 67, 0.06) 0%, rgba(46, 160, 67, 0.04) 100%);
	}

	:global(.diff-annotation-thread.diff-annotation-resolved .diff-annotation-reply) {
		background: linear-gradient(135deg, rgba(46, 160, 67, 0.03) 0%, rgba(46, 160, 67, 0.02) 100%);
		border-top-color: rgba(46, 160, 67, 0.15);
	}

	:global(.diff-annotation-thread.diff-annotation-resolved .diff-annotation-footer) {
		background: rgba(46, 160, 67, 0.04);
		border-top-color: rgba(46, 160, 67, 0.15);
	}

	/* Collapsed header for resolved threads */
	:global(.diff-annotation-collapsed-header) {
		display: none;
		align-items: center;
		gap: 10px;
		padding: 10px 14px;
		background: linear-gradient(135deg, rgba(46, 160, 67, 0.08) 0%, rgba(46, 160, 67, 0.05) 100%);
		cursor: pointer;
		transition: background 0.15s ease;
	}

	:global(.diff-annotation-collapsed-header:hover) {
		background: linear-gradient(135deg, rgba(46, 160, 67, 0.12) 0%, rgba(46, 160, 67, 0.08) 100%);
	}

	:global(.diff-annotation-collapsed .diff-annotation-collapsed-header) {
		display: flex;
	}

	:global(.diff-annotation-collapsed .diff-annotation-content),
	:global(.diff-annotation-collapsed .diff-annotation-footer) {
		display: none;
	}

	:global(.diff-annotation-collapsed-icon) {
		font-size: 10px;
		color: #3fb950;
		transition: transform 0.15s ease;
	}

	:global(.diff-annotation-thread:not(.diff-annotation-collapsed) .diff-annotation-collapsed-icon) {
		transform: rotate(90deg);
	}

	:global(.diff-annotation-collapsed-badge) {
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		padding: 2px 8px;
		border-radius: 999px;
		background: rgba(46, 160, 67, 0.2);
		color: #3fb950;
	}

	:global(.diff-annotation-collapsed-preview) {
		flex: 1;
		font-size: 12px;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	:global(.diff-annotation-collapsed-count) {
		font-size: 11px;
		color: var(--muted);
		opacity: 0.7;
	}

	/* Unresolve button style */
	:global(.diff-action-unresolve) {
		color: #d29922;
	}

	:global(.diff-action-unresolve:hover) {
		background: rgba(210, 153, 34, 0.15);
		color: #f0b429;
	}

	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(6, 9, 14, 0.78);
		display: grid;
		place-items: center;
		z-index: 30;
		padding: 24px;
		animation: overlayFadeIn var(--transition-normal) ease-out;
	}

	.overlay-panel {
		width: 100%;
		display: flex;
		justify-content: center;
		animation: modalSlideIn 200ms ease-out;
	}

	@keyframes overlayFadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	@keyframes modalSlideIn {
		from {
			opacity: 0;
			transform: translateY(-8px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

	@media (max-width: 720px) {
		.overlay {
			padding: 0;
		}
	}

	/* Line highlight animation for annotation navigation */
	:global(.highlight-line) {
		animation: line-highlight 2s ease-out;
	}

	:global(.highlight-line td) {
		background: rgba(210, 153, 34, 0.2) !important;
	}

	@keyframes line-highlight {
		0% {
			background: rgba(210, 153, 34, 0.4);
		}
		100% {
			background: transparent;
		}
	}
</style>
