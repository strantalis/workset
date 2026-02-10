<script lang="ts">
	import {
		AlertCircle,
		ArrowUpRight,
		Box,
		CheckCircle2,
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
		PullRequestCheck,
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
	import RepoDiffAnnotationStyles from '../repo-diff/RepoDiffAnnotationStyles.svelte';
	import {
		fetchBranchDiffSummary,
		fetchBranchFileDiff,
		fetchRepoDiffSummary,
		fetchRepoFileDiff,
		startRepoStatusWatch,
		stopRepoStatusWatch,
	} from '../../api/repo-diff';
	import ResizablePanel from '../ui/ResizablePanel.svelte';
	import { mapWorkspaceToPrItems } from '../../view-models/prViewModel';

	interface Props {
		workspace: Workspace | null;
		focusRepoId?: string | null;
		focusToken?: number;
	}

	const { workspace, focusRepoId = null, focusToken = 0 }: Props = $props();

	// ─── Derived workspace data ──────────────────────────────────────────
	const prItems = $derived(mapWorkspaceToPrItems(workspace));

	// ─── Tracked PR map (drives active/ready partition) ────────────────
	let trackedPrMap: Map<string, PullRequestCreated> = $state(new Map());

	const loadTrackedPrMap = (ws: Workspace): void => {
		const nextMap = new Map<string, PullRequestCreated>();
		for (const repo of ws.repos) {
			if (repo.trackedPullRequest) {
				nextMap.set(repo.id, repo.trackedPullRequest);
			}
		}
		trackedPrMap = nextMap;
	};

	$effect(() => {
		if (workspace) {
			loadTrackedPrMap(workspace);
		} else {
			trackedPrMap = new Map();
		}
	});

	const partitions = $derived.by(() => {
		const active = prItems.filter((item) => trackedPrMap.has(item.repoId));
		const readyToPR = prItems.filter(
			(item) =>
				!trackedPrMap.has(item.repoId) && (item.dirty || item.dirtyFiles > 0 || item.ahead > 0),
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

	// ─── Files tab ──────────────────────────────────────────────────────
	let diffSummary: RepoDiffSummary | null = $state(null);
	let localSummary: RepoDiffSummary | null = $state(null);
	let diffSummaryLoading = $state(false);
	let selectedFileIdx = $state(0);
	let selectedSource = $state<'pr' | 'local'>('pr');
	let fileDiffContent: RepoFileDiff | null = $state(null);
	let fileDiffLoading = $state(false);
	let fileDiffError: string | null = $state(null);
	let activeWatchKey: { wsId: string; repoId: string } | null = $state(null);
	let activePrBranches: { base: string; head: string } | null = $state(null);

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

	// ─── Derived selectors ──────────────────────────────────────────────
	const selectedItem = $derived(prItems.find((item) => item.id === selectedItemId) ?? null);

	const selectedRepo = $derived.by(() => {
		if (!selectedItem || !workspace) return null;
		return workspace.repos.find((r) => r.id === selectedItem.repoId) ?? null;
	});

	const getMode = (): 'active' | 'ready' => viewMode;
	const isActiveDetail = $derived(getMode() === 'active' && selectedItem != null);
	const isReadyDetail = $derived(getMode() === 'ready' && selectedItem != null);

	const checkStats = $derived.by(() => {
		const checks = prStatus?.checks ?? [];
		let passed = 0;
		let failed = 0;
		let pending = 0;
		for (const c of checks) {
			if (c.conclusion === 'success') passed++;
			else if (c.conclusion === 'failure') failed++;
			else pending++;
		}
		return { passed, failed, pending, total: checks.length };
	});

	const reviewThreads = $derived.by(() => {
		const threadMap = new Map<string, PullRequestReviewComment[]>();
		for (const comment of prReviews) {
			const key = comment.threadId ?? `single-${comment.id}`;
			const arr = threadMap.get(key) ?? [];
			arr.push(comment);
			threadMap.set(key, arr);
		}
		return Array.from(threadMap.entries())
			.map(([id, comments]) => ({
				id,
				comments: comments.sort((a, b) => (a.createdAt ?? '').localeCompare(b.createdAt ?? '')),
				path: comments[0]?.path ?? '',
				line: comments[0]?.line,
				resolved: comments[0]?.resolved ?? false,
				outdated: comments[0]?.outdated ?? false,
			}))
			.sort((a, b) => {
				if (a.resolved !== b.resolved) return a.resolved ? 1 : -1;
				return a.path.localeCompare(b.path);
			});
	});

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
		const file = diffSummary?.files[selectedFileIdx];
		if (!file) return [];
		return buildLineAnnotations(prReviews.filter((r) => r.path === file.path));
	});

	// ─── Actions ────────────────────────────────────────────────────────

	const selectItem = (itemId: string): void => {
		selectedItemId = itemId;
		activeTab = 'files';
		trackedPr = null;
		prStatus = null;
		prReviews = [];
		diffSummary = null;
		localSummary = null;
		selectedFileIdx = 0;
		selectedSource = 'pr';
		fileDiffContent = null;
		fileDiffError = null;
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

		const item = prItems.find((i) => i.id === itemId);
		if (item && workspace) {
			void loadTrackedPr(workspace.id, item.repoId);
			void loadRepoLocalStatus(workspace.id, item.repoId);
			const pr = trackedPrMap.get(item.repoId);
			void loadDiffSummary(workspace.id, item.repoId, pr);
			if (viewMode === 'ready') {
				void loadSuggestedPrText(workspace.id, item.repoId);
			}
		}
	};

	const loadTrackedPr = async (wsId: string, repoId: string): Promise<void> => {
		trackedPr = trackedPrMap.get(repoId) ?? null;
		trackedPrLoading = true;
		try {
			const resolved = await fetchTrackedPullRequest(wsId, repoId);
			trackedPr = resolved;
			const nextMap = new Map(trackedPrMap);
			if (resolved) {
				nextMap.set(repoId, resolved);
			} else {
				nextMap.delete(repoId);
			}
			trackedPrMap = nextMap;
		} catch {
			trackedPr = null;
		} finally {
			trackedPrLoading = false;
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
			const { wsId, repoId } = activeWatchKey;
			activeWatchKey = null;
			try {
				await stopRepoStatusWatch(wsId, repoId);
			} catch {
				/* ignore */
			}
		}
	};

	const loadLocalSummary = async (wsId: string, repoId: string): Promise<void> => {
		try {
			localSummary = await fetchRepoDiffSummary(wsId, repoId);
		} catch {
			localSummary = null;
		}
	};

	const loadDiffSummary = async (
		wsId: string,
		repoId: string,
		pr?: PullRequestCreated,
	): Promise<void> => {
		diffSummaryLoading = true;
		activePrBranches = null;
		try {
			await stopActiveWatch();
			if (pr) {
				activePrBranches = { base: pr.baseBranch, head: pr.headBranch };
				diffSummary = await fetchBranchDiffSummary(wsId, repoId, pr.baseBranch, pr.headBranch);
				void loadLocalSummary(wsId, repoId);
			} else {
				await startRepoStatusWatch(wsId, repoId);
				activeWatchKey = { wsId, repoId };
				diffSummary = await fetchRepoDiffSummary(wsId, repoId);
			}
		} catch {
			diffSummary = null;
		} finally {
			diffSummaryLoading = false;
		}
	};

	const loadFileDiff = async (
		wsId: string,
		repoId: string,
		file: RepoDiffFileSummary,
		source: 'pr' | 'local' = 'pr',
	): Promise<void> => {
		fileDiffLoading = true;
		fileDiffError = null;
		fileDiffContent = null;
		try {
			if (source === 'local') {
				fileDiffContent = await fetchRepoFileDiff(
					wsId,
					repoId,
					file.path,
					file.prevPath ?? '',
					file.status ?? '',
				);
			} else if (activePrBranches) {
				fileDiffContent = await fetchBranchFileDiff(
					wsId,
					repoId,
					activePrBranches.base,
					activePrBranches.head,
					file.path,
					file.prevPath ?? '',
				);
			} else {
				fileDiffContent = await fetchRepoFileDiff(
					wsId,
					repoId,
					file.path,
					file.prevPath ?? '',
					file.status ?? '',
				);
			}
		} catch (err) {
			fileDiffError = err instanceof Error ? err.message : 'Failed to load diff';
			fileDiffContent = null;
		} finally {
			fileDiffLoading = false;
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

	// ─── Effects ────────────────────────────────────────────────────────

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
		if (selectedItemId && !prItems.find((i) => i.id === selectedItemId)) {
			selectedItemId = null;
		}
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

	// Load file diff when selection changes
	$effect(() => {
		const files =
			selectedSource === 'local' ? (localSummary?.files ?? []) : (diffSummary?.files ?? []);
		const file = files[selectedFileIdx];
		if (file && workspace && selectedItem) {
			void loadFileDiff(workspace.id, selectedItem.repoId, file, selectedSource);
		} else {
			fileDiffContent = null;
			fileDiffError = null;
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

	const buildDiffOptions = (): FileDiffRenderOptions<ReviewAnnotation> => ({
		theme: 'pierre-dark',
		themeType: 'dark',
		diffStyle: 'split',
		diffIndicators: 'bars',
		renderAnnotation: (a) => annotationController.renderAnnotation(a),
	});

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

		void ensureDiffsModule().then((mod) => {
			const parsed = mod.parsePatchFiles(patch);
			const fileDiff = parsed[0]?.files?.[0] ?? null;
			if (!fileDiff) return;

			if (!diffInstance) {
				diffInstance = new mod.FileDiff(buildDiffOptions());
			} else {
				diffInstance.setOptions(buildDiffOptions());
			}
			diffInstance.render({
				fileDiff,
				fileContainer: container,
				forceRender: true,
				lineAnnotations: annotations,
			});
		});

		return () => {
			diffInstance?.cleanUp();
			diffInstance = null;
		};
	});

	// ─── Helpers ────────────────────────────────────────────────────────

	const getCheckIcon = (check: PullRequestCheck): string => {
		if (check.conclusion === 'success') return 'success';
		if (check.conclusion === 'failure') return 'failure';
		if (check.status === 'in_progress' || check.status === 'queued') return 'running';
		return 'pending';
	};

	const formatCheckDuration = (check: PullRequestCheck): string => {
		if (!check.startedAt || !check.completedAt) {
			if (check.status === 'in_progress' || check.status === 'queued') return 'Running...';
			return 'Pending';
		}
		const ms = new Date(check.completedAt).getTime() - new Date(check.startedAt).getTime();
		if (ms < 1000) return `${ms}ms`;
		if (ms < 60000) return `${Math.round(ms / 1000)}s`;
		return `${Math.round(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`;
	};

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
					<div class="ws-eyebrow">Current Workset</div>
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

			<!-- ═══════════════════ RIGHT PANEL ═══════════════════ -->
			{#snippet second()}
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
													{#if fileDiffLoading}
														<div class="diff-placeholder">
															<Loader2 size={20} class="spin" />
															<p>Loading diff...</p>
														</div>
													{:else if fileDiffError}
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
														<div class="diff-renderer">
															<diffs-container bind:this={diffContainer}></diffs-container>
														</div>
														{#if fileDiffContent.truncated}
															<div class="diff-truncated">
																Diff truncated ({fileDiffContent.totalLines} total lines)
															</div>
														{/if}
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
		</ResizablePanel>
	{/if}
	<RepoDiffAnnotationStyles />
</div>

<style>
	.pro {
		display: flex;
		flex-direction: column;
		height: 100%;
		background: color-mix(in srgb, var(--bg) 90%, transparent);
	}

	/* ── Empty state ──────────────────────────────────────────── */
	.empty-state {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 16px;
		color: var(--muted);
		opacity: 0.5;
	}
	.empty-state p {
		font-size: var(--text-md);
	}

	/* ── Sidebar ──────────────────────────────────────────────── */
	.sidebar {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		background: color-mix(in srgb, var(--panel) 80%, transparent);
		border-right: 1px solid var(--border);
		backdrop-filter: blur(16px);
	}

	.ws-header {
		padding: 12px;
		border-bottom: 1px solid var(--border);
	}
	.ws-eyebrow {
		font-size: var(--text-xs);
		font-weight: 700;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		margin-bottom: 6px;
	}
	.ws-badge {
		display: flex;
		align-items: center;
		gap: 8px;
		background: color-mix(in srgb, var(--panel-strong) 50%, transparent);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 6px 8px;
	}
	.ws-badge :global(.ws-badge-icon) {
		color: var(--accent);
		flex-shrink: 0;
	}
	.ws-badge-name {
		font-size: var(--text-base);
		font-weight: 500;
		color: var(--text);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* ── Mode Switch (pill tabs) ──────────────────────────────── */
	.mode-switch {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 4px;
		padding: 8px;
		border-bottom: 1px solid var(--border);
		background: var(--panel-soft);
	}
	.ms-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		padding: 6px 8px;
		font-size: var(--text-sm);
		font-weight: 500;
		border-radius: 6px;
		border: none;
		cursor: pointer;
		transition: all 150ms;
		color: var(--muted);
		background: transparent;
	}
	.ms-btn:hover:not(.active) {
		color: var(--text);
		background: color-mix(in srgb, var(--panel-strong) 50%, transparent);
	}
	.ms-btn.active {
		color: var(--text);
		background: var(--panel-strong);
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
	}
	.ms-count {
		font-size: var(--text-xs);
		opacity: 0.6;
		background: var(--border);
		padding: 1px 6px;
		border-radius: 999px;
	}
	.ms-count.ready {
		background: color-mix(in srgb, var(--success) 20%, transparent);
		color: var(--success);
		opacity: 1;
	}

	/* ── List items ───────────────────────────────────────────── */
	.list {
		flex: 1;
		overflow-y: auto;
		padding: 8px;
		display: flex;
		flex-direction: column;
		gap: 4px;
		scrollbar-width: thin;
		scrollbar-color: var(--border) transparent;
	}
	.list::-webkit-scrollbar {
		width: 5px;
	}
	.list::-webkit-scrollbar-thumb {
		background: var(--border);
		border-radius: 3px;
	}

	.list-item {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 10px 12px;
		border-radius: 8px;
		cursor: pointer;
		border: 1px solid transparent;
		background: transparent;
		text-align: left;
		color: var(--text);
		transition: all 150ms;
	}
	.list-item:hover:not(.active):not(.active-ready) {
		background: var(--panel-strong);
		border-color: var(--border);
	}
	.list-item.active {
		background: var(--panel-strong);
		border-color: var(--accent-soft);
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
	}
	.list-item.active-ready {
		background: var(--panel-strong);
		border-color: color-mix(in srgb, var(--success) 40%, transparent);
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
	}

	.li-top {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 4px;
	}
	.li-title {
		margin: 0;
		font-size: var(--text-base);
		font-weight: 500;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		transition: color 150ms;
	}
	.li-title.bright {
		color: var(--text);
	}
	.list-item:hover .li-title {
		color: var(--text);
	}
	:global(.li-chevron) {
		color: var(--accent);
		flex-shrink: 0;
	}
	:global(.li-chevron-green) {
		color: var(--success);
		flex-shrink: 0;
	}

	.li-meta {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		color: var(--muted);
	}
	.li-repo {
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		background: var(--bg);
		padding: 1px 4px;
		border-radius: 3px;
	}
	.li-sep {
		opacity: 0.5;
	}
	.li-passing {
		color: var(--success);
	}
	.li-running {
		color: var(--warning);
	}
	.li-blocked {
		color: var(--status-error);
	}
	.li-warn {
		display: flex;
		align-items: center;
		gap: 3px;
		color: var(--yellow);
	}

	.li-branch-row {
		display: flex;
		align-items: center;
		gap: 4px;
	}
	.li-branch-name {
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: var(--success);
	}
	.li-commits {
		display: flex;
		align-items: center;
		gap: 3px;
	}
	.li-time {
		margin-left: auto;
		opacity: 0.7;
	}

	.list-empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 160px;
		color: color-mix(in srgb, var(--muted) 50%, transparent);
		font-size: var(--text-base);
		gap: 8px;
	}

	/* ── Detail panel ─────────────────────────────────────────── */
	.detail {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		min-width: 0;
		background: var(--bg);
	}

	/* ── PR Header ────────────────────────────────────────────── */
	.pr-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 12px 24px;
		border-bottom: 1px solid var(--border);
		background: color-mix(in srgb, var(--panel) 80%, transparent);
		backdrop-filter: blur(16px);
	}
	.prh-left {
		flex: 1;
		min-width: 0;
	}
	.prh-title-row {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 4px;
	}
	.prh-repo-tag {
		font-size: var(--text-mono-sm);
		font-family: var(--font-mono);
		color: var(--muted);
		background: var(--panel-strong);
		padding: 2px 6px;
		border-radius: 4px;
		border: 1px solid var(--border);
		flex-shrink: 0;
	}
	.prh-title {
		margin: 0;
		font-size: var(--text-xl);
		font-weight: 600;
		color: var(--text);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.prh-meta {
		display: flex;
		align-items: center;
		gap: 16px;
		font-size: var(--text-sm);
		color: var(--muted);
	}
	.prh-status {
		display: flex;
		align-items: center;
		gap: 4px;
	}
	:global(.prh-status-dot) {
		color: var(--success);
		fill: var(--success);
	}
	.prh-branch {
		display: flex;
		align-items: center;
		gap: 4px;
		font-family: var(--font-mono);
		color: var(--accent);
	}
	.prh-action-link {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 16px;
		border-radius: 8px;
		font-size: var(--text-base);
		font-weight: 500;
		cursor: pointer;
		background: transparent;
		border: 1px solid var(--border);
		color: var(--muted);
		text-decoration: none;
		transition: all 150ms;
	}
	.prh-action-link:hover {
		color: var(--text);
		border-color: var(--accent-soft);
	}
	.prh-loading {
		color: var(--subtle);
	}

	/* ── Push Status Bar ──────────────────────────────────────── */
	.pr-push-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 8px 12px;
		margin: 8px 24px;
		background: color-mix(in srgb, var(--panel) 60%, transparent);
		border: 1px solid var(--border);
		border-radius: 10px;
		font-size: var(--text-sm);
		gap: 12px;
	}

	.psb-stats {
		display: flex;
		align-items: center;
		gap: 12px;
		color: var(--muted);
	}

	.psb-stat {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.psb-success {
		color: var(--success);
	}

	.psb-up-to-date {
		color: var(--subtle);
	}

	.psb-error {
		color: var(--status-error);
		font-size: var(--text-xs);
	}

	.psb-push-btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 14px;
		background: var(--warning);
		color: var(--bg);
		border: none;
		border-radius: 8px;
		font-size: var(--text-sm);
		font-weight: 600;
		cursor: pointer;
		transition: opacity 150ms;
		white-space: nowrap;
	}

	.psb-push-btn:hover:not(:disabled) {
		opacity: 0.9;
	}

	.psb-push-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	/* ── Tab bar ──────────────────────────────────────────────── */
	.tab-bar {
		display: flex;
		align-items: center;
		gap: 24px;
		height: 40px;
		padding: 0 16px;
		border-bottom: 1px solid var(--border);
		background: var(--panel-soft);
		flex-shrink: 0;
	}
	.tab-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		height: 100%;
		padding: 0 4px;
		font-size: var(--text-sm);
		font-weight: 500;
		border: none;
		border-bottom: 2px solid transparent;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all 150ms;
	}
	.tab-btn:hover:not(.active) {
		color: var(--text);
	}
	.tab-btn.active {
		color: var(--text);
		border-bottom-color: var(--accent);
	}
	.tab-count {
		font-size: var(--text-xs);
		background: var(--border);
		color: var(--muted);
		padding: 1px 6px;
		border-radius: 999px;
	}

	/* ── Tab content ──────────────────────────────────────────── */
	.tab-content {
		flex: 1;
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}

	/* ── Files Tab ────────────────────────────────────────────── */
	.files-panel {
		flex: 1;
		display: flex;
		overflow: hidden;
	}
	.fp-sidebar {
		width: 256px;
		flex-shrink: 0;
		display: flex;
		flex-direction: column;
		border-right: 1px solid var(--border);
		background: var(--panel-soft);
	}
	.fp-sidebar-head {
		padding: 12px;
		font-size: var(--text-xs);
		font-weight: 700;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.08em;
	}
	.fp-file-list {
		flex: 1;
		overflow-y: auto;
	}
	.fp-file {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 8px 16px;
		font-size: var(--text-sm);
		color: var(--muted);
		border: none;
		border-left: 2px solid transparent;
		background: transparent;
		text-align: left;
		cursor: pointer;
		transition: all 100ms;
	}
	.fp-file:hover:not(.active) {
		background: color-mix(in srgb, var(--panel-strong) 50%, transparent);
	}
	.fp-file.active {
		background: var(--panel-strong);
		color: var(--text);
		border-left-color: var(--accent);
	}
	.fp-file-name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.fp-loading {
		padding: 16px;
		font-size: var(--text-sm);
		color: var(--subtle);
	}
	.fp-divider {
		height: 1px;
		background: var(--border);
		margin: 4px 12px;
	}
	.fp-local-head {
		color: var(--yellow);
	}

	.fp-diff {
		flex: 1;
		overflow-y: auto;
		padding: 16px;
	}
	.diff-card {
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
		background: var(--bg);
	}
	.diff-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 8px 16px;
		background: var(--panel-strong);
		border-bottom: 1px solid var(--border);
		font-size: var(--text-mono-sm);
		font-family: var(--font-mono);
		color: var(--muted);
	}
	.diff-body {
		min-height: 200px;
	}
	.diff-placeholder {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 48px;
		color: var(--subtle);
		font-size: var(--text-base);
		text-align: center;
	}
	.diff-placeholder.full {
		flex: 1;
		height: 100%;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
	}
	/* ── Diff renderer (@pierre/diffs) ───────────────────────── */
	.diff-renderer {
		flex: 1;
		min-height: 0;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: var(--bg);
		padding: 8px;
		overflow: hidden;
		--diffs-dark-bg: var(--bg);
		--diffs-dark: var(--text);
		--diffs-dark-addition-color: var(--success);
		--diffs-dark-deletion-color: var(--status-error);
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
	.diff-truncated {
		padding: 12px;
		text-align: center;
		font-size: var(--text-xs);
		color: var(--yellow);
		background: color-mix(in srgb, var(--yellow) 5%, transparent);
		border-top: 1px solid var(--border);
	}

	/* ── Checks Tab ───────────────────────────────────────────── */
	.checks-panel {
		flex: 1;
		overflow-y: auto;
		padding: 24px;
		background: var(--bg);
	}
	.checks-max {
		max-width: 900px;
		margin: 0 auto;
	}
	.checks-header-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 24px;
	}
	.checks-header-row h2 {
		margin: 0;
		font-size: var(--text-xl);
		font-weight: 600;
		color: var(--text);
	}
	.checks-list {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.ck-row {
		display: flex;
		align-items: center;
		gap: 16px;
		padding: 16px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: var(--panel);
		transition: background 150ms;
	}
	.ck-row:hover {
		background: var(--panel-strong);
	}
	.ck-circle {
		width: 32px;
		height: 32px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
	.ck-circle.success {
		background: color-mix(in srgb, var(--success) 10%, transparent);
		color: var(--success);
	}
	.ck-circle.failure {
		background: color-mix(in srgb, var(--status-error) 10%, transparent);
		color: var(--status-error);
	}
	.ck-circle.running {
		background: color-mix(in srgb, var(--yellow) 10%, transparent);
		color: var(--yellow);
	}
	.ck-circle.pending {
		background: color-mix(in srgb, var(--yellow) 10%, transparent);
		color: var(--yellow);
	}
	.ck-info {
		flex: 1;
		min-width: 0;
	}
	.ck-name-row {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 2px;
	}
	.ck-name-row h3 {
		margin: 0;
		font-size: var(--text-base);
		font-weight: 500;
		color: var(--text);
	}
	.ck-dur {
		margin: 0;
		font-size: var(--text-sm);
		color: var(--muted);
	}
	.ck-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		opacity: 0;
		transition: opacity 150ms;
	}
	.ck-row:hover .ck-actions {
		opacity: 1;
	}
	.ck-action {
		padding: 8px;
		border-radius: 6px;
		border: none;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all 100ms;
	}
	.ck-action:hover {
		background: var(--border);
		color: var(--text);
	}

	/* ── Tab review badge ─────────────────────────────────────── */
	.tab-review-badge {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-left: auto;
		font-size: var(--text-xs);
		color: var(--yellow);
		padding: 2px 8px;
		border-radius: 999px;
		background: color-mix(in srgb, var(--yellow) 10%, transparent);
	}

	/* ── Ready to PR Detail ───────────────────────────────────── */
	.ready-detail {
		border-bottom: 1px solid var(--border);
		padding: 16px 24px;
		background: color-mix(in srgb, var(--panel) 80%, transparent);
		backdrop-filter: blur(16px);
	}
	.rd-header {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 12px;
	}
	.rd-icon {
		width: 36px;
		height: 36px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--success) 10%, transparent);
		color: var(--success);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.rd-info h1 {
		margin: 0;
		font-size: var(--text-xl);
		font-weight: 600;
		color: var(--text);
	}
	.rd-branch-row {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-sm);
	}
	.rd-branch {
		font-family: var(--font-mono);
		color: var(--success);
	}
	.rd-arrow {
		color: var(--subtle);
	}
	.rd-base {
		font-family: var(--font-mono);
		color: var(--subtle);
	}
	.rd-stats {
		display: flex;
		align-items: center;
		gap: 24px;
		font-size: var(--text-sm);
		color: var(--muted);
	}
	.rd-stat {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.rd-stat strong {
		color: var(--text);
	}

	.rd-content {
		flex: 1;
		overflow-y: auto;
		padding: 24px;
		background: var(--bg);
	}
	.rd-max {
		max-width: 640px;
		margin: 0 auto;
	}

	.rd-success {
		text-align: center;
		padding: 48px 0;
	}
	.rd-success-icon {
		width: 64px;
		height: 64px;
		border-radius: 50%;
		margin: 0 auto 16px;
		background: color-mix(in srgb, var(--success) 10%, transparent);
		color: var(--success);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.rd-success h2 {
		margin: 0 0 8px;
		font-size: var(--text-2xl);
		font-weight: 600;
		color: var(--text);
	}
	.rd-success p {
		margin: 0 0 24px;
		font-size: var(--text-base);
		color: var(--muted);
	}

	.form-field {
		margin-bottom: 16px;
	}
	.form-label {
		display: block;
		font-size: var(--text-xs);
		font-weight: 700;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		margin-bottom: 6px;
	}
	.form-optional {
		color: var(--subtle);
		text-transform: none;
		font-weight: 400;
	}
	.form-input,
	.form-textarea {
		width: 100%;
		padding: 10px 16px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: var(--panel-strong);
		font-size: var(--text-base);
		color: var(--text);
		font-family: inherit;
		transition: border-color 150ms;
	}
	.form-input::placeholder,
	.form-textarea::placeholder {
		color: var(--subtle);
	}
	.form-input:focus,
	.form-textarea:focus {
		outline: none;
		border-color: var(--accent-soft);
	}
	.form-textarea {
		resize: none;
	}

	.rd-file-list {
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
	}
	.rd-file-row {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 16px;
		color: var(--muted);
		background: var(--panel-strong);
		transition: background 150ms;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
	}
	.rd-file-row:last-child {
		border-bottom: none;
	}
	.rd-file-row:hover {
		background: var(--panel);
	}
	.rd-file-name {
		flex: 1;
		font-size: var(--text-mono-sm);
		font-family: var(--font-mono);
		color: var(--text);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.rd-actions {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-top: 8px;
	}
	.rd-draft-toggle {
		display: flex;
		align-items: center;
		gap: 8px;
		cursor: pointer;
		font-size: var(--text-sm);
		color: var(--muted);
	}
	.rd-draft-toggle input {
		width: 14px;
		height: 14px;
		border-radius: 4px;
		border: 1px solid var(--border);
		background: var(--panel-strong);
		accent-color: var(--accent);
	}
	.rd-create-btn {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 20px;
		border-radius: 8px;
		border: none;
		font-size: var(--text-base);
		font-weight: 500;
		cursor: pointer;
		background: var(--success);
		color: white;
		transition: all 150ms;
		box-shadow: 0 4px 16px color-mix(in srgb, var(--success) 20%, transparent);
	}
	.rd-create-btn:hover:not(:disabled) {
		background: color-mix(in srgb, var(--success) 90%, transparent);
	}
	.rd-create-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	/* ── Shared ────────────────────────────────────────────────── */
	.ghost-btn {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 6px 12px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: var(--panel-strong);
		font-size: var(--text-sm);
		color: var(--muted);
		cursor: pointer;
		transition: all 150ms;
	}
	.ghost-btn:hover {
		color: var(--text);
		border-color: var(--accent-soft);
	}

	.panel-loading {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 12px;
		padding: 64px;
		color: var(--muted);
		font-size: var(--text-base);
		opacity: 0.5;
	}

	.mono {
		font-family: var(--font-mono);
	}
	:global(.text-green) {
		color: var(--success);
	}
	:global(.text-blue) {
		color: var(--accent);
	}
	:global(.text-yellow) {
		color: var(--yellow);
	}
	:global(.text-red) {
		color: var(--status-error);
	}
	:global(.text-white) {
		color: var(--text);
	}
	:global(.text-muted) {
		color: var(--subtle);
	}
	.text-green {
		color: var(--success);
	}
	.text-red {
		color: var(--status-error);
	}
	.text-xs {
		font-size: var(--text-xs);
	}

	:global(.spin) {
		animation: spin 1s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.form-input-wrap {
		position: relative;
		border-radius: 6px;
	}
	.form-input-wrap.shimmer {
		overflow: hidden;
	}
	.form-input-wrap.shimmer::after {
		content: '';
		position: absolute;
		inset: 0;
		border-radius: 6px;
		background: linear-gradient(
			90deg,
			transparent 0%,
			color-mix(in srgb, var(--accent) 8%, transparent) 40%,
			color-mix(in srgb, var(--accent) 15%, transparent) 50%,
			color-mix(in srgb, var(--accent) 8%, transparent) 60%,
			transparent 100%
		);
		animation: shimmer 1.8s ease-in-out infinite;
		pointer-events: none;
	}
	@keyframes shimmer {
		0% {
			transform: translateX(-100%);
		}
		100% {
			transform: translateX(100%);
		}
	}

	.form-generating {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-size: var(--text-xs);
		font-weight: 400;
		color: var(--accent);
		margin-left: 8px;
	}
</style>
