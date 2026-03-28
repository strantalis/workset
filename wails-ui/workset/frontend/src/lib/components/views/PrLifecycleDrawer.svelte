<script lang="ts">
	import {
		AlertCircle,
		CheckCircle2,
		ChevronDown,
		Circle,
		ExternalLink,
		FileCode,
		FileDiff,
		GitCommit,
		Loader2,
		MessageCircle,
		Upload,
		XCircle,
	} from '@lucide/svelte';
	import DOMPurify from 'dompurify';
	import { marked } from 'marked';
	import {
		fetchGitHubOperationStatus,
		fetchPullRequestStatus,
		fetchRepoLocalStatus,
		generatePullRequestText,
		startCommitAndPushAsync,
		startCreatePullRequestAsync,
	} from '../../api/github';
	import type {
		GitHubOperationStage,
		GitHubOperationStatus,
		RepoLocalStatus,
	} from '../../api/github';
	import type { PullRequestCreated, PullRequestStatusResult } from '../../types';
	import { subscribeGitHubOperationEvent } from '../../githubOperationService';
	import { Browser } from '@wailsio/runtime';
	import SlideDrawer from '../ui/SlideDrawer.svelte';
	import Button from '../ui/Button.svelte';
	import { buildCheckStats } from '../../pullRequestUiHelpers';

	const POLL_INTERVAL_MS = 10_000;
	const STATUS_CACHE_TTL_MS = 30_000;

	type StatusCacheEntry = {
		prStatus: PullRequestStatusResult | null;
		localStatus: RepoLocalStatus | null;
		cachedAt: number;
	};

	type CreateStage = Extract<GitHubOperationStage, 'queued' | 'generating' | 'creating'> | null;

	const statusCache = new Map<string, StatusCacheEntry>();

	interface Props {
		open: boolean;
		workspaceId: string;
		repoId: string;
		repoName: string;
		branch: string;
		baseBranch: string;
		trackedPr: PullRequestCreated | null;
		diffStats?: { filesChanged: number; additions: number; deletions: number } | null;
		unresolvedThreads?: number;
		onClose: () => void;
		onStatusChanged: () => void;
		onTrackedPrChanged: (pr: PullRequestCreated) => void;
	}

	const {
		open,
		workspaceId,
		repoId,
		repoName,
		branch,
		baseBranch,
		trackedPr,
		diffStats = null,
		unresolvedThreads = 0,
		onClose,
		onStatusChanged,
		onTrackedPrChanged,
	}: Props = $props();

	let checksExpanded = $state(false);
	let descriptionExpanded = $state(false);
	let prStatus: PullRequestStatusResult | null = $state(null);
	let localStatus: RepoLocalStatus | null = $state(null);
	let pushLoading = $state(false);
	let pushSuccess = $state(false);
	let pushError: string | null = $state(null);
	const descriptionHtml = $derived.by(() => {
		const body = activeTrackedPr?.body;
		if (!body) return '';
		const raw = marked.parse(body, { gfm: true, breaks: true }) as string;
		return DOMPurify.sanitize(raw);
	});
	let statusRequestId = 0;

	let prTitle = $state('');
	let prBody = $state('');
	let isDraft = $state(false);
	let suggestionLoading = $state(false);
	let createStage: CreateStage = $state(null);
	let createError: string | null = $state(null);
	let generationRequestId = 0;
	let createStatusRequestId = 0;
	let lastCreateContextKey = '';
	let recoveredPullRequest: PullRequestCreated | null = $state(null);
	let recoveredPullRequestContextKey = $state('');

	const buildCreateContextKey = (): string =>
		`${workspaceId}\u0000${repoId}\u0000${branch}\u0000${baseBranch}`;

	const activeTrackedPr = $derived.by(() => {
		if (trackedPr) return trackedPr;
		if (recoveredPullRequest && recoveredPullRequestContextKey === buildCreateContextKey()) {
			return recoveredPullRequest;
		}
		return null;
	});

	const buildStatusCacheKey = (): string =>
		`${workspaceId}\u0000${repoId}\u0000${activeTrackedPr?.number ?? 0}\u0000${branch}`;

	const readCachedStatus = (): StatusCacheEntry | null => {
		const cached = statusCache.get(buildStatusCacheKey());
		if (!cached) return null;
		if (Date.now() - cached.cachedAt > STATUS_CACHE_TTL_MS) {
			statusCache.delete(buildStatusCacheKey());
			return null;
		}
		return cached;
	};

	const checkStats = $derived(buildCheckStats(prStatus));

	const isMerged = $derived(
		activeTrackedPr != null &&
			(activeTrackedPr.merged === true || activeTrackedPr.state.toLowerCase() === 'merged'),
	);

	const prState = $derived.by(() => {
		if (!activeTrackedPr) return 'open';
		if (activeTrackedPr.draft) return 'draft';
		const state = activeTrackedPr.state.toLowerCase();
		if (state === 'merged' || activeTrackedPr.merged) return 'merged';
		if (state === 'closed') return 'closed';
		return 'open';
	});

	const prStateLabel = $derived.by(() => {
		if (prState === 'draft') return 'Draft';
		if (prState === 'merged') return 'Merged';
		if (prState === 'closed') return 'Closed';
		return 'Open';
	});

	const checksStatus = $derived.by(() => {
		if (checkStats.total === 0) return 'none';
		if (checkStats.failed > 0) return 'fail';
		if (checkStats.pending > 0) return 'pending';
		return 'pass';
	});

	const pushBarNeedsAction = $derived.by(() => {
		const status = localStatus;
		if (!status) return false;
		return status.hasUncommitted || status.ahead > 0;
	});

	const pushDisabled = $derived.by(() => {
		const status = localStatus;
		return pushLoading || !status || (!status.hasUncommitted && status.ahead === 0);
	});

	const createInProgress = $derived(createStage !== null);
	const createButtonDisabled = $derived(createInProgress || !prTitle.trim());
	const createStageLabel = $derived.by(() => {
		switch (createStage) {
			case 'queued':
				return 'Starting pull request...';
			case 'generating':
				return 'Generating pull request...';
			case 'creating':
				return 'Creating pull request...';
			default:
				return 'Create Pull Request';
		}
	});

	const resetCreateDraft = (): void => {
		prTitle = '';
		prBody = '';
		isDraft = false;
		suggestionLoading = false;
		createStage = null;
		createError = null;
		generationRequestId += 1;
		createStatusRequestId += 1;
	};

	const clearRecoveredPullRequest = (): void => {
		recoveredPullRequest = null;
		recoveredPullRequestContextKey = '';
	};

	const loadStatus = async (): Promise<void> => {
		const pr = activeTrackedPr;
		if (!pr) return;
		const requestId = ++statusRequestId;
		try {
			const [status, local] = await Promise.all([
				fetchPullRequestStatus(workspaceId, repoId, pr.number, branch),
				fetchRepoLocalStatus(workspaceId, repoId),
			]);
			if (requestId !== statusRequestId) return;
			prStatus = status;
			localStatus = local;
			statusCache.set(buildStatusCacheKey(), {
				prStatus: status,
				localStatus: local,
				cachedAt: Date.now(),
			});
		} catch {
			if (requestId !== statusRequestId) return;
		}
	};

	const loadSuggestion = async (): Promise<void> => {
		const requestId = ++generationRequestId;
		suggestionLoading = true;
		try {
			const generated = await generatePullRequestText(workspaceId, repoId);
			if (requestId !== generationRequestId) return;
			if (generated.title && !prTitle) prTitle = generated.title;
			if (generated.body && !prBody) prBody = generated.body;
		} catch {
			// non-fatal
		} finally {
			if (requestId === generationRequestId) suggestionLoading = false;
		}
	};

	const applyTrackedPullRequest = (pullRequest: PullRequestCreated): void => {
		recoveredPullRequest = pullRequest;
		recoveredPullRequestContextKey = buildCreateContextKey();
		createStage = null;
		createError = null;
		onTrackedPrChanged(pullRequest);
		void loadStatus();
	};

	const applyCreateOperationStatus = (status: GitHubOperationStatus): void => {
		if (status.workspaceId !== workspaceId || status.repoId !== repoId) return;
		if (status.type !== 'create_pr') return;

		if (status.state === 'running') {
			if (status.stage === 'generating' || status.stage === 'creating') {
				createStage = status.stage;
			} else {
				createStage = 'queued';
			}
			createError = null;
			return;
		}

		createStage = null;
		if (status.state === 'completed') {
			createError = null;
			if (status.pullRequest) {
				applyTrackedPullRequest(status.pullRequest);
			}
			return;
		}

		createError = status.error || 'Failed to create pull request.';
	};

	const loadCreateOperationStatus = async (): Promise<boolean> => {
		const requestId = ++createStatusRequestId;
		try {
			const status = await fetchGitHubOperationStatus(workspaceId, repoId, 'create_pr');
			if (requestId !== createStatusRequestId) return true;
			if (!status) return false;
			applyCreateOperationStatus(status);
			return true;
		} catch {
			if (requestId !== createStatusRequestId) return true;
			return false;
		}
	};

	const handleCreate = async (): Promise<void> => {
		if (createStage) return;
		const title = prTitle.trim();
		if (!title) {
			createError = 'Title is required.';
			return;
		}

		createStage = 'queued';
		createError = null;
		generationRequestId += 1;
		suggestionLoading = false;

		try {
			const status = await startCreatePullRequestAsync(workspaceId, repoId, {
				title,
				body: prBody.trim(),
				base: baseBranch,
				head: branch,
				draft: isDraft,
			});
			applyCreateOperationStatus(status);
		} catch (error) {
			createStage = null;
			createError = error instanceof Error ? error.message : 'Failed to create pull request.';
		}
	};

	const handlePush = async (): Promise<void> => {
		if (pushLoading) return;
		pushLoading = true;
		pushError = null;
		pushSuccess = false;
		try {
			await startCommitAndPushAsync(workspaceId, repoId);
		} catch {
			pushLoading = false;
		}
	};

	$effect(() => {
		if (checkStats.failed > 0) checksExpanded = true;
	});

	$effect(() => {
		if (activeTrackedPr) {
			createStage = null;
			createError = null;
		}
	});

	$effect(() => {
		const drawerOpen = open;
		const currentTrackedPr = activeTrackedPr;
		if (!drawerOpen || !currentTrackedPr) {
			prStatus = null;
			localStatus = null;
			pushSuccess = false;
			pushError = null;
			statusRequestId += 1;
			return;
		}

		const cached = readCachedStatus();
		if (cached) {
			prStatus = cached.prStatus;
			localStatus = cached.localStatus;
		}

		void loadStatus();
		const timer = setInterval(() => void loadStatus(), POLL_INTERVAL_MS);
		return () => clearInterval(timer);
	});

	$effect(() => {
		if (!open) return;
		const unsubscribe = subscribeGitHubOperationEvent((status: GitHubOperationStatus) => {
			if (status.workspaceId !== workspaceId || status.repoId !== repoId) return;
			if (status.type === 'create_pr') {
				applyCreateOperationStatus(status);
				return;
			}
			if (status.type !== 'commit_push') return;
			if (status.state === 'completed') {
				pushLoading = false;
				pushSuccess = true;
				void loadStatus();
				onStatusChanged();
			} else if (status.state === 'failed') {
				pushLoading = false;
				pushError = status.error || 'Push failed.';
			}
		});
		return unsubscribe;
	});

	$effect(() => {
		const drawerOpen = open;
		const currentTrackedPr = activeTrackedPr;
		const contextKey = buildCreateContextKey();
		if (!drawerOpen || currentTrackedPr) return;

		let cancelled = false;
		const hydrateCreateState = async (): Promise<void> => {
			const hasAsyncStatus = await loadCreateOperationStatus();
			if (cancelled) return;
			if (hasAsyncStatus) return;
			if (lastCreateContextKey === contextKey) return;
			clearRecoveredPullRequest();
			resetCreateDraft();
			lastCreateContextKey = contextKey;
			void loadSuggestion();
		};

		void hydrateCreateState();

		return () => {
			cancelled = true;
			createStatusRequestId += 1;
		};
	});
</script>

<SlideDrawer {open} title="Pull Request" {onClose}>
	{#if activeTrackedPr}
		<div class="pld-content">
			<div class="pld-header-info">
				<div class="pld-title-row">
					<h3 class="pld-pr-title">{activeTrackedPr.title}</h3>
					<span class="pld-state-badge pld-state-{prState}">{prStateLabel}</span>
				</div>
				<div class="pld-meta-row">
					<div class="pld-meta">
						<span class="pld-repo">{repoName}</span>
						<span class="pld-dot">/</span>
						<span class="pld-branch">{branch} → {activeTrackedPr.baseBranch ?? 'main'}</span>
					</div>
					<button
						type="button"
						class="pld-github-link"
						onclick={() => activeTrackedPr.url && Browser.OpenURL(activeTrackedPr.url)}
					>
						<ExternalLink size={11} />
						GitHub
					</button>
				</div>
			</div>

			{#if !isMerged}
				<div class="pld-push-bar" class:pld-push-bar--action={pushBarNeedsAction}>
					<div class="pld-push-stats">
						{#if pushSuccess}
							<span class="pld-push-stat pld-push-success">
								<CheckCircle2 size={12} />
								Pushed
							</span>
						{:else if pushError}
							<span class="pld-push-stat pld-push-error">
								<AlertCircle size={12} />
								{pushError}
							</span>
						{:else if localStatus}
							{#if localStatus.ahead > 0}
								<span class="pld-push-stat">
									<GitCommit size={12} />
									{localStatus.ahead} unpushed
								</span>
							{/if}
							{#if localStatus.hasUncommitted}
								<span class="pld-push-stat">
									<FileCode size={12} />
									uncommitted changes
								</span>
							{/if}
							{#if !localStatus.hasUncommitted && localStatus.ahead === 0}
								<span class="pld-push-stat pld-push-ok">
									<CheckCircle2 size={12} />
									Up to date
								</span>
							{/if}
						{/if}
					</div>
					<Button
						variant={pushBarNeedsAction ? 'primary' : 'ghost'}
						size="sm"
						disabled={pushDisabled}
						onclick={() => void handlePush()}
					>
						{#if pushLoading}
							<Loader2 size={12} class="spin" />
							Pushing...
						{:else}
							<Upload size={12} />
							Push to PR
						{/if}
					</Button>
				</div>
			{/if}

			<div class="pld-overview">
				{#if diffStats}
					<div class="pld-overview-stats">
						<span class="pld-overview-stat">
							<span class="pld-icon-accent"><FileDiff size={11} /></span>
							{diffStats.filesChanged} file{diffStats.filesChanged === 1 ? '' : 's'}
						</span>
						<span class="pld-overview-stat">
							<span class="pld-stat-add">+{diffStats.additions}</span>
							<span class="pld-stat-del">-{diffStats.deletions}</span>
						</span>
					</div>
				{/if}
				<div class="pld-overview-review">
					{#if unresolvedThreads > 0}
						<span class="pld-overview-stat">
							<span class="pld-icon-warn"><MessageCircle size={11} /></span>
							{unresolvedThreads} unresolved thread{unresolvedThreads === 1 ? '' : 's'}
						</span>
					{/if}
					{#if prStatus?.pullRequest?.mergeable}
						<span class="pld-overview-stat">
							{#if prStatus.pullRequest.mergeable === 'mergeable'}
								<CheckCircle2 size={11} class="pld-check-pass" /> Ready to merge
							{:else if prStatus.pullRequest.mergeable === 'conflicts'}
								<XCircle size={11} class="pld-check-fail" /> Has conflicts
							{:else}
								<Circle size={11} class="pld-check-neutral" /> Merge status unknown
							{/if}
						</span>
					{/if}
				</div>
			</div>

			<div class="pld-section pld-section--divided">
				<button
					type="button"
					class="pld-section-toggle"
					onclick={() => (checksExpanded = !checksExpanded)}
				>
					<span class="pld-section-head">
						{#if checkStats.total === 0}
							<Circle size={12} class="pld-check-neutral" />
							No checks
						{:else if checkStats.failed > 0}
							<XCircle size={12} class="pld-check-fail" />
							{checkStats.failed} check{checkStats.failed === 1 ? '' : 's'} failing
						{:else if checkStats.pending > 0}
							<AlertCircle size={12} class="pld-check-pending" />
							{checkStats.pending} check{checkStats.pending === 1 ? '' : 's'} running
						{:else}
							<CheckCircle2 size={12} class="pld-check-pass" />
							{checkStats.total} checks passing
						{/if}
					</span>
					{#if checkStats.total > 0}
						<span class="pld-section-chevron" class:expanded={checksExpanded}>
							<ChevronDown size={12} />
						</span>
					{/if}
				</button>
				{#if checksExpanded && prStatus?.checks}
					<div class="pld-checks-container pld-checks-{checksStatus}">
						{#each prStatus.checks as check (check.name)}
							<div class="pld-check-row">
								{#if check.conclusion === 'success'}
									<CheckCircle2 size={12} class="pld-check-pass" />
								{:else if check.conclusion === 'failure'}
									<XCircle size={12} class="pld-check-fail" />
								{:else}
									<AlertCircle size={12} class="pld-check-pending" />
								{/if}
								<span class="pld-check-name">{check.name}</span>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			{#if descriptionHtml}
				<div class="pld-section pld-section--divided">
					<button
						type="button"
						class="pld-section-toggle"
						onclick={() => (descriptionExpanded = !descriptionExpanded)}
					>
						<span class="pld-section-head">Description</span>
						<span class="pld-section-chevron" class:expanded={descriptionExpanded}>
							<ChevronDown size={12} />
						</span>
					</button>
					{#if descriptionExpanded}
						<div class="pld-description">
							<!-- eslint-disable-next-line svelte/no-at-html-tags -->
							{@html descriptionHtml}
						</div>
					{/if}
				</div>
			{/if}
		</div>
	{:else}
		<div class="pld-create">
			<div class="pld-create-context">
				<span class="pld-repo">{repoName}</span>
				<span class="pld-dot">/</span>
				<span class="pld-branch">{branch} → {baseBranch}</span>
			</div>

			{#if suggestionLoading}
				<div class="pld-create-note">
					<Loader2 size={12} class="spin" />
					AI is drafting title and description...
				</div>
			{/if}

			{#if createStage}
				<div class="pld-progress-card">
					<div class="pld-progress-head">
						<Loader2 size={14} class="spin" />
						<span>{createStageLabel}</span>
					</div>
					<p>Closing this drawer will not cancel the pull request creation.</p>
				</div>
			{/if}

			<label class="pld-field">
				<span class="pld-label">Title</span>
				<input
					type="text"
					class="pld-input"
					class:pld-shimmer={suggestionLoading && !prTitle}
					value={prTitle}
					disabled={createInProgress}
					oninput={(event) => {
						prTitle = (event.currentTarget as HTMLInputElement).value;
						if (createError) createError = null;
					}}
					placeholder={suggestionLoading && !prTitle ? 'Generating...' : 'PR title'}
				/>
			</label>

			<label class="pld-field">
				<span class="pld-label">Description</span>
				<textarea
					class="pld-textarea"
					class:pld-shimmer={suggestionLoading && !prBody}
					rows={5}
					value={prBody}
					disabled={createInProgress}
					oninput={(event) => {
						prBody = (event.currentTarget as HTMLTextAreaElement).value;
						if (createError) createError = null;
					}}
					placeholder={suggestionLoading && !prBody ? 'Generating...' : 'Describe the changes...'}
				></textarea>
			</label>

			<label class="pld-draft">
				<input
					type="checkbox"
					checked={isDraft}
					disabled={createInProgress}
					onchange={(event) => {
						isDraft = (event.currentTarget as HTMLInputElement).checked;
					}}
				/>
				<span>Create as draft</span>
			</label>

			{#if createError}
				<div class="pld-error">{createError}</div>
			{/if}

			<Button
				variant="primary"
				size="sm"
				disabled={createButtonDisabled}
				onclick={() => void handleCreate()}
			>
				{#if createInProgress}
					<Loader2 size={12} class="spin" />
					{createStageLabel}
				{:else}
					Create Pull Request
				{/if}
			</Button>
		</div>
	{/if}
</SlideDrawer>

<style>
	.pld-content,
	.pld-create {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.pld-header-info {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.pld-title-row {
		display: flex;
		align-items: flex-start;
		gap: 8px;
	}
	.pld-pr-title {
		margin: 0;
		font-size: var(--text-md);
		font-weight: 600;
		color: var(--text);
		line-height: 1.4;
		flex: 1;
		min-width: 0;
	}
	.pld-meta-row,
	.pld-create-context {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}
	.pld-meta,
	.pld-create-context {
		font-size: var(--text-2xs);
		color: var(--muted);
		min-width: 0;
	}
	.pld-github-link {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 3px 8px;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		font-size: var(--text-2xs);
		cursor: pointer;
		flex-shrink: 0;
		transition: all var(--transition-fast);
	}
	.pld-github-link:hover {
		color: var(--text);
		border-color: var(--accent);
		background: color-mix(in srgb, var(--accent) 8%, transparent);
	}
	.pld-repo {
		font-family: var(--font-mono);
		font-size: var(--text-mono-2xs);
	}
	.pld-dot {
		opacity: 0.4;
	}
	.pld-branch {
		font-family: var(--font-mono);
		font-size: var(--text-mono-2xs);
		color: var(--accent);
	}

	.pld-state-badge {
		display: inline-flex;
		align-items: center;
		padding: 2px 8px;
		border-radius: 999px;
		font-size: var(--text-2xs);
		font-weight: 600;
		letter-spacing: 0.02em;
		white-space: nowrap;
		flex-shrink: 0;
	}
	.pld-state-open {
		background: color-mix(in srgb, var(--success) 14%, transparent);
		border: 1px solid color-mix(in srgb, var(--success) 42%, transparent);
		color: var(--success);
	}
	.pld-state-merged {
		background: color-mix(in srgb, var(--accent) 14%, transparent);
		border: 1px solid color-mix(in srgb, var(--accent) 42%, transparent);
		color: var(--accent);
	}
	.pld-state-closed {
		background: color-mix(in srgb, var(--danger) 14%, transparent);
		border: 1px solid color-mix(in srgb, var(--danger) 42%, transparent);
		color: var(--danger);
	}
	.pld-state-draft {
		background: color-mix(in srgb, var(--warning) 10%, transparent);
		border: 1px solid color-mix(in srgb, var(--warning) 30%, transparent);
		color: var(--warning);
	}

	.pld-progress-card,
	.pld-push-bar {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 12px 14px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: color-mix(in srgb, var(--panel-strong) 72%, transparent);
	}
	.pld-progress-head {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-xs);
		font-weight: 600;
		color: var(--text);
	}
	.pld-progress-card p {
		margin: 0;
		font-size: var(--text-2xs);
		color: var(--muted);
	}
	.pld-push-bar--action {
		border-color: color-mix(in srgb, var(--accent) 26%, var(--border));
	}
	.pld-push-stats {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}
	.pld-push-stat {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font-size: var(--text-2xs);
		color: var(--muted);
	}
	.pld-push-success,
	.pld-push-ok,
	.pld-check-pass {
		color: var(--success);
	}
	.pld-push-error,
	.pld-check-fail {
		color: var(--danger);
	}
	.pld-check-pending,
	.pld-icon-warn {
		color: var(--warning);
	}
	.pld-check-neutral {
		color: var(--muted);
	}

	.pld-overview {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.pld-overview-stats,
	.pld-overview-review {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}
	.pld-overview-stat {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 5px 8px;
		border-radius: 999px;
		background: color-mix(in srgb, var(--panel-strong) 70%, transparent);
		font-size: var(--text-2xs);
		color: var(--muted);
	}
	.pld-icon-accent,
	.pld-stat-add {
		color: var(--success);
	}
	.pld-stat-del {
		color: var(--danger);
	}

	.pld-section {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.pld-section--divided {
		padding-top: 16px;
		border-top: 1px solid var(--border);
	}
	.pld-section-toggle {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding: 0;
		border: 0;
		background: transparent;
		color: inherit;
		cursor: pointer;
	}
	.pld-section-head {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		font-weight: 600;
		color: var(--text);
	}
	.pld-section-chevron {
		display: inline-flex;
		color: var(--muted);
		transition: transform var(--transition-fast);
	}
	.pld-section-chevron.expanded {
		transform: rotate(180deg);
	}
	.pld-checks-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.pld-check-row {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-xs);
		color: var(--muted);
	}
	.pld-check-name {
		min-width: 0;
	}
	.pld-description {
		font-size: var(--text-xs);
		color: var(--text);
		line-height: 1.6;
	}

	.pld-create-note {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		color: color-mix(in srgb, var(--accent) 65%, var(--text));
	}
	.pld-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.pld-label {
		font-size: var(--text-2xs);
		color: var(--muted);
	}
	.pld-input,
	.pld-textarea {
		width: 100%;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: color-mix(in srgb, var(--panel-strong) 70%, transparent);
		color: var(--text);
		font-family: inherit;
		font-size: var(--text-xs);
		padding: 8px 10px;
	}
	.pld-textarea {
		resize: vertical;
		min-height: 96px;
	}
	.pld-input:disabled,
	.pld-textarea:disabled {
		opacity: 0.75;
		cursor: not-allowed;
	}
	.pld-input:focus,
	.pld-textarea:focus {
		outline: 1px solid color-mix(in srgb, var(--accent) 60%, var(--border));
		outline-offset: 0;
	}
	.pld-shimmer {
		background: linear-gradient(
				110deg,
				color-mix(in srgb, var(--panel-strong) 78%, transparent) 8%,
				color-mix(in srgb, var(--accent) 14%, transparent) 18%,
				color-mix(in srgb, var(--panel-strong) 78%, transparent) 33%
			)
			0 0 / 220% 100%;
		animation: pld-shimmer 1.1s linear infinite;
	}
	.pld-draft {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		color: var(--muted);
	}
	.pld-error {
		font-size: var(--text-xs);
		color: var(--danger);
	}
	.spin {
		animation: pld-spin 0.85s linear infinite;
	}

	@keyframes pld-shimmer {
		to {
			background-position: -220% 0;
		}
	}
	@keyframes pld-spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
