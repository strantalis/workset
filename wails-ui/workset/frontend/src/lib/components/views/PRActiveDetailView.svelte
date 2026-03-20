<script lang="ts">
	// prettier-ignore
	import { AlertCircle, CheckCircle2, Circle, FileCode, GitCommit, Loader2, MessageSquare, Upload, XCircle } from '@lucide/svelte';
	// prettier-ignore
	import type { PullRequestCreated, PullRequestReviewComment, PullRequestStatusResult, RepoDiffFileSummary, RepoDiffSummary, RepoFileDiff, Workspace } from '../../types';
	import { fetchPullRequestReviews, fetchPullRequestStatus } from '../../api/github';
	import type { RepoLocalStatus } from '../../api/github';
	import { deleteReviewComment, editReviewComment } from '../../api/github/review';
	import { fetchCurrentGitHubUser } from '../../api/github/user';
	import { replyToReviewComment, resolveReviewThread } from '../../api/github';
	import { buildLineAnnotations } from '../repo-diff/annotations';
	import type { DiffLineAnnotation, ReviewAnnotation } from '../repo-diff/annotations';
	import { createReviewAnnotationActionsController } from '../repo-diff/reviewAnnotationActions';
	import DiffRenderer from '../repo-diff/DiffRenderer.svelte';
	import RepoDiffAnnotationStyles from '../repo-diff/RepoDiffAnnotationStyles.svelte';
	import { EVENT_REPO_DIFF_PR_STATUS } from '../../events';
	import { subscribeRepoDiffEvent } from '../../repoDiffService';
	import type { CommitPushState } from '../repo-diff/commitPushController.svelte';
	import type { PrListItem } from '../../view-models/prViewModel';
	import PROrchestrationActiveHeader from './PROrchestrationActiveHeader.svelte';
	import PROrchestrationChecksPanel from './PROrchestrationChecksPanel.svelte';
	import { buildCheckStats } from './prOrchestrationHelpers';
	import {
		applyPrStatusEvent,
		getPullRequestFeedbackCounts,
		hasTrackedPrMetadataChanged,
		type RepoDiffPrStatusEvent,
	} from './prOrchestrationView.helpers';

	const PR_STATUS_SYNC_INTERVAL_MS = 8000;
	const isMergedTrackedPr = (pr: PullRequestCreated | undefined | null): boolean =>
		Boolean(pr && (pr.merged === true || pr.state.toLowerCase() === 'merged'));

	interface Props {
		workspace: Workspace;
		selectedItem: PrListItem;
		selectedRepo: Workspace['repos'][number] | null;
		trackedPrMap: Map<string, PullRequestCreated>;
		diffSummary: RepoDiffSummary | null;
		localSummary: RepoDiffSummary | null;
		diffSummaryLoading: boolean;
		fileDiffContent: RepoFileDiff | null;
		fileDiffLoading: boolean;
		fileDiffError: string | null;
		selectedFileIdx: number;
		selectedSource: 'pr' | 'local';
		filesForDetail: RepoDiffFileSummary[];
		totalAdd: number;
		totalDel: number;
		shouldSplitLocalPendingSection: boolean;
		commitPush: CommitPushState;
		repoLocalStatus: RepoLocalStatus | null;
		onSelectedFileIdxChange: (idx: number) => void;
		onSelectedSourceChange: (source: 'pr' | 'local') => void;
		onStartPush: () => Promise<void> | void;
		onOpenExternalUrl: (url: string | undefined | null) => void;
		onSetFileDiffError: (error: string | null) => void;
		onReconcileTrackedPr: (wsId: string, repoId: string) => Promise<void> | void;
		onPrStatusEventApplied: (
			repoId: string,
			trackedPr: PullRequestCreated | null,
			previousTracked: PullRequestCreated | null,
			updatedMap: Map<string, PullRequestCreated>,
		) => void;
	}

	const {
		workspace,
		selectedItem,
		selectedRepo,
		trackedPrMap,
		diffSummary,
		localSummary,
		diffSummaryLoading,
		fileDiffContent,
		fileDiffLoading,
		fileDiffError,
		selectedFileIdx,
		selectedSource,
		filesForDetail,
		totalAdd,
		totalDel,
		shouldSplitLocalPendingSection,
		commitPush,
		repoLocalStatus,
		onSelectedFileIdxChange,
		onSelectedSourceChange,
		onStartPush,
		onOpenExternalUrl,
		onSetFileDiffError,
		onReconcileTrackedPr,
		onPrStatusEventApplied,
	}: Props = $props();

	// ── Active-only state ───────────────────────────────────────
	let activeTab: 'overview' | 'files' | 'checks' = $state('overview');
	// eslint-disable-next-line svelte/prefer-writable-derived -- also set by PR status event handler
	let trackedPr: PullRequestCreated | null = $state(null);
	let prStatus: PullRequestStatusResult | null = $state(null),
		prStatusLoading = $state(false),
		prStatusRequestId = 0;
	let prReviews: PullRequestReviewComment[] = $state([]),
		prReviewsLoading = $state(false),
		prReviewsAttempted = false,
		currentUserId: number | null = $state(null);

	// ── Derived from trackedPrMap prop ──────────────────────────
	$effect(() => {
		trackedPr = trackedPrMap.get(selectedItem.repoId) ?? null;
	});

	// Reset activeTab when selectedItem changes
	let lastSelectedItemId = '';
	$effect(() => {
		if (selectedItem.id !== lastSelectedItemId) {
			lastSelectedItemId = selectedItem.id;
			activeTab = 'overview';
			prStatus = null;
			prStatusLoading = false;
			prStatusRequestId += 1;
			prReviews = [];
			prReviewsLoading = false;
			prReviewsAttempted = false;
		}
	});

	// ── Derived state ───────────────────────────────────────────
	const feedbackCounts = $derived.by(() =>
		getPullRequestFeedbackCounts(trackedPr, prStatus, selectedItem),
	);
	const checkStats = $derived(buildCheckStats(prStatus));

	const pushStatusVisible = $derived.by(
		() =>
			trackedPr != null &&
			!isMergedTrackedPr(trackedPr) &&
			trackedPr.state.toLowerCase() === 'open' &&
			repoLocalStatus != null,
	);

	const pushDisabled = $derived.by(
		() =>
			commitPush.loading ||
			!repoLocalStatus ||
			(!repoLocalStatus.hasUncommitted && repoLocalStatus.ahead === 0),
	);

	const selectedFile = $derived.by(
		() =>
			(selectedSource === 'local' ? (localSummary?.files ?? []) : (diffSummary?.files ?? []))[
				selectedFileIdx
			] ?? null,
	);

	const lineAnnotations: DiffLineAnnotation<ReviewAnnotation>[] = $derived.by(() => {
		if (selectedSource === 'local' || prReviews.length === 0) return [];
		const file = selectedFile;
		if (!file) return [];
		return buildLineAnnotations(prReviews.filter((r) => r.path === file.path));
	});

	// ── Annotation controller ───────────────────────────────────
	const annotationController = createReviewAnnotationActionsController({
		document,
		workspaceId: () => workspace.id,
		repoId: () => selectedItem.repoId,
		prNumberInput: () => String(trackedPr?.number ?? ''),
		prBranchInput: () => selectedItem.branch,
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
			await deleteReviewComment(workspace.id, selectedItem.repoId, commentId);
			void loadReviews();
		},
		handleResolveThread: async (threadId, resolve) => {
			await resolveReviewThread(workspace.id, selectedItem.repoId, threadId, resolve);
			void loadReviews();
		},
		formatError: (err, fallback) => (err instanceof Error ? err.message : fallback),
		showAlert: (msg) => {
			onSetFileDiffError(msg);
		},
	});

	// ── Data loading ────────────────────────────────────────────
	const loadChecks = async (options: { reconcileTracked?: boolean } = {}): Promise<void> => {
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
			if (requestId !== prStatusRequestId) return;
			prStatus = result;

			if (
				options.reconcileTracked &&
				hasTrackedPrMetadataChanged(previousTracked, result.pullRequest)
			) {
				await onReconcileTrackedPr(wsId, repoId);
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

	const loadReviews = async (): Promise<void> => {
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

	const handleRefreshChecks = (): void => {
		void loadChecks();
	};

	const handlePushToPr = async (): Promise<void> => {
		await onStartPush();
	};

	// ── Effects ─────────────────────────────────────────────────

	// Lazy-load checks when checks tab opened
	$effect(() => {
		if (activeTab === 'checks' && !prStatus && !prStatusLoading) {
			void loadChecks();
		}
	});

	// PR status polling (every 8s while active view is mounted)
	$effect(() => {
		if (!workspace || !selectedItem) return;

		let stopped = false;
		let inFlight = false;

		const sync = async (): Promise<void> => {
			if (stopped || inFlight) return;
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

	// Load reviews when PR is available
	$effect(() => {
		if (trackedPr && selectedItem && !prReviewsLoading && !prReviewsAttempted) {
			prReviewsAttempted = true;
			void loadReviews();
			if (!currentUserId) {
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

	// Listen for PR status events from repoDiffService
	$effect(() => {
		const unsub = subscribeRepoDiffEvent<RepoDiffPrStatusEvent>(
			EVENT_REPO_DIFF_PR_STATUS,
			(payload) => {
				if (payload.workspaceId !== workspace.id || payload.repoId !== selectedItem.repoId) return;
				const previousTracked = trackedPrMap.get(selectedItem.repoId) ?? null;
				const next = applyPrStatusEvent(payload, selectedItem.repoId, trackedPrMap);
				prStatus = next.prStatus;
				trackedPr = next.trackedPr;
				onPrStatusEventApplied(
					selectedItem.repoId,
					next.trackedPr,
					previousTracked,
					next.trackedPrMap,
				);
				if (next.shouldReconcileTrackedPr) {
					void onReconcileTrackedPr(workspace.id, selectedItem.repoId);
				}
			},
		);
		return unsub;
	});
</script>

<PROrchestrationActiveHeader
	{trackedPr}
	trackedPrLoading={false}
	{selectedItem}
	workspaceName={workspace.name}
	trackedTitle={trackedPrMap.get(selectedItem.repoId)?.title ?? selectedItem.title}
	{checkStats}
	activeTab={activeTab === 'checks' ? 'overview' : activeTab}
	filesCount={filesForDetail.length || selectedItem.dirtyFiles}
	onActiveTabChange={(tab) => (activeTab = tab)}
	{onOpenExternalUrl}
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
							{#if commitPush.success}
								<span class="psb-stat psb-success">
									<CheckCircle2 size={12} />
									Pushed successfully
								</span>
							{:else if commitPush.error}
								<span class="psb-stat psb-error">
									<AlertCircle size={12} />
									{commitPush.error}
								</span>
							{:else if repoLocalStatus && (repoLocalStatus.ahead > 0 || repoLocalStatus.hasUncommitted)}
								{#if repoLocalStatus.ahead > 0}
									<span class="psb-stat">
										<GitCommit size={12} />
										{repoLocalStatus.ahead} unpushed commit{repoLocalStatus.ahead !== 1 ? 's' : ''}
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
							{#if commitPush.loading}
								<Loader2 size={14} class="spin" />
								{commitPush.stageLabel ?? 'Pushing...'}
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
										onSelectedSourceChange('pr');
										onSelectedFileIdxChange(i);
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
							<span class="ov-stat-value">{filesForDetail.length || selectedItem.dirtyFiles}</span>
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
									onSelectedSourceChange('pr');
									onSelectedFileIdxChange(i);
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
									onSelectedSourceChange('pr');
									onSelectedFileIdxChange(i);
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
									onSelectedSourceChange('local');
									onSelectedFileIdxChange(i);
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
								{#if activeFile.added > 0}<span class="text-green">+{activeFile.added}</span>{/if}
								{#if activeFile.removed > 0}
									<span class="text-red">-{activeFile.removed}</span>{/if}
							</span>
						</div>
						<div class="diff-body">
							<DiffRenderer
								patch={fileDiffContent?.patch ?? null}
								loading={fileDiffLoading}
								error={fileDiffError}
								binary={fileDiffContent?.binary ?? false}
								truncated={fileDiffContent?.truncated ?? false}
								totalLines={fileDiffContent?.totalLines ?? 0}
								{lineAnnotations}
								renderAnnotation={(a) => annotationController.renderAnnotation(a)}
								onRenderError={(msg) => onSetFileDiffError(msg)}
							/>
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
			{onOpenExternalUrl}
		/>
	{/if}
</div>
<RepoDiffAnnotationStyles />

<style src="./PRActiveDetailView.css"></style>
