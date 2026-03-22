<script lang="ts">
	import {
		AlertCircle,
		CheckCircle2,
		ChevronDown,
		Circle,
		ExternalLink,
		FileDiff,
		GitCommit,
		FileCode,
		Loader2,
		MessageCircle,
		Upload,
		XCircle,
	} from '@lucide/svelte';
	import DOMPurify from 'dompurify';
	import { marked } from 'marked';
	import type { PullRequestCreated, PullRequestStatusResult } from '../../types';
	import { fetchPullRequestStatus, startCommitAndPushAsync } from '../../api/github';
	import type { GitHubOperationStatus, RepoLocalStatus } from '../../api/github';
	import { fetchRepoLocalStatus } from '../../api/github';
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

	const statusCache = new Map<string, StatusCacheEntry>();

	interface Props {
		open: boolean;
		workspaceId: string;
		repoId: string;
		repoName: string;
		branch: string;
		trackedPr: PullRequestCreated | null;
		diffStats?: { filesChanged: number; additions: number; deletions: number } | null;
		unresolvedThreads?: number;
		onClose: () => void;
		onStatusChanged: () => void;
	}

	const {
		open,
		workspaceId,
		repoId,
		repoName,
		branch,
		trackedPr,
		diffStats = null,
		unresolvedThreads = 0,
		onClose,
		onStatusChanged,
	}: Props = $props();

	let checksExpanded = $state(false);
	let descriptionExpanded = $state(false);
	let prStatus: PullRequestStatusResult | null = $state(null);
	let localStatus: RepoLocalStatus | null = $state(null);
	let pushLoading = $state(false);
	let pushSuccess = $state(false);
	let pushError: string | null = $state(null);
	let descriptionHtml: string = $state('');
	let statusRequestId = 0;

	const buildStatusCacheKey = (): string =>
		`${workspaceId}\u0000${repoId}\u0000${trackedPr?.number ?? 0}\u0000${branch}`;

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
		trackedPr != null && (trackedPr.merged === true || trackedPr.state.toLowerCase() === 'merged'),
	);

	const prState = $derived.by(() => {
		if (!trackedPr) return 'open';
		if (trackedPr.draft) return 'draft';
		const s = trackedPr.state.toLowerCase();
		if (s === 'merged' || trackedPr.merged) return 'merged';
		if (s === 'closed') return 'closed';
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

	// Auto-expand checks when there are failures
	$effect(() => {
		if (checkStats.failed > 0) checksExpanded = true;
	});

	const pushBarNeedsAction = $derived.by(() => {
		const ls = localStatus;
		if (!ls) return false;
		return ls.hasUncommitted || ls.ahead > 0;
	});

	const pushDisabled = $derived.by(() => {
		const ls = localStatus;
		return pushLoading || !ls || (!ls.hasUncommitted && ls.ahead === 0);
	});

	const loadStatus = async (): Promise<void> => {
		const requestId = ++statusRequestId;
		try {
			const [status, local] = await Promise.all([
				fetchPullRequestStatus(workspaceId, repoId, trackedPr?.number, branch),
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

	// Render description as markdown
	$effect(() => {
		const body = trackedPr?.body;
		if (!body) {
			descriptionHtml = '';
			return;
		}
		const raw = marked.parse(body, { gfm: true, breaks: true }) as string;
		descriptionHtml = DOMPurify.sanitize(raw);
	});

	// Load status on open, poll while open
	$effect(() => {
		if (!open || !trackedPr) {
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

	// Listen for push events
	$effect(() => {
		if (!open) return;
		const unsub = subscribeGitHubOperationEvent((status: GitHubOperationStatus) => {
			if (status.workspaceId !== workspaceId || status.repoId !== repoId) return;
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
		return unsub;
	});
</script>

<SlideDrawer {open} title="Pull Request" {onClose}>
	{#if trackedPr}
		<div class="pld-content">
			<!-- Header + GitHub link -->
			<div class="pld-header-info">
				<div class="pld-title-row">
					<h3 class="pld-pr-title">{trackedPr.title}</h3>
					<span class="pld-state-badge pld-state-{prState}">{prStateLabel}</span>
				</div>
				<div class="pld-meta-row">
					<div class="pld-meta">
						<span class="pld-repo">{repoName}</span>
						<span class="pld-dot">/</span>
						<span class="pld-branch">{branch} → {trackedPr.baseBranch ?? 'main'}</span>
					</div>
					<button
						type="button"
						class="pld-github-link"
						onclick={() => trackedPr?.url && Browser.OpenURL(trackedPr.url)}
					>
						<ExternalLink size={11} />
						GitHub
					</button>
				</div>
			</div>

			<!-- Push status -->
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

			<!-- Stats + Review (compact, at-a-glance) -->
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

			<!-- Checks (collapsible) -->
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

			<!-- Description (collapsible) -->
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
		<div class="pld-empty">
			<p>No tracked pull request.</p>
		</div>
	{/if}
</SlideDrawer>

<style>
	.pld-content {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	/* ── Header ──────────────────────────────────────── */
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
	.pld-meta-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}
	.pld-meta {
		display: flex;
		align-items: center;
		gap: 6px;
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

	/* ── State badge ─────────────────────────────────── */
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
	.pld-state-badge--inline {
		padding: 1px 6px;
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

	/* ── Sections ────────────────────────────────────── */
	.pld-section {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.pld-section--divided {
		border-top: 1px solid var(--border);
		padding-top: 16px;
	}
	.pld-section-head {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xxs);
		color: var(--subtle);
		text-transform: uppercase;
		letter-spacing: 0.06em;
		font-weight: 500;
	}
	.pld-section-toggle {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: 0;
		border: none;
		background: transparent;
		cursor: pointer;
		color: inherit;
	}
	.pld-section-toggle:hover .pld-section-head {
		color: var(--text);
	}
	.pld-section-chevron {
		color: var(--subtle);
		transition: transform 150ms ease;
	}
	.pld-section-chevron.expanded {
		transform: rotate(180deg);
	}

	/* ── Overview (compact stats + review) ───────────── */
	.pld-overview {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 10px 12px;
		border-radius: 8px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
	}
	.pld-overview-stats,
	.pld-overview-review {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}
	.pld-overview-stat {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	/* ── Description (markdown) ──────────────────────── */
	.pld-description {
		font-size: var(--text-sm);
		color: var(--muted);
		line-height: 1.65;
		margin: 0;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 12px 14px;
		overflow: hidden;
	}
	.pld-description :global(> *:first-child) {
		margin-top: 0;
	}
	.pld-description :global(> *:last-child) {
		margin-bottom: 0;
	}
	.pld-description :global(h1),
	.pld-description :global(h2),
	.pld-description :global(h3) {
		color: var(--text);
		margin: 0.8em 0 0.3em;
		font-size: var(--text-sm);
		font-weight: 600;
	}
	.pld-description :global(p) {
		margin: 0.4em 0;
	}
	.pld-description :global(ul),
	.pld-description :global(ol) {
		padding-left: 1.4em;
		margin: 0.3em 0;
	}
	.pld-description :global(li) {
		margin: 0.15em 0;
	}
	.pld-description :global(code) {
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		background: var(--panel);
		padding: 1px 4px;
		border-radius: 3px;
	}
	.pld-description :global(pre) {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 10px 12px;
		overflow-x: auto;
		margin: 0.5em 0;
	}
	.pld-description :global(pre code) {
		background: none;
		padding: 0;
	}
	.pld-description :global(a) {
		color: var(--accent);
		text-decoration: none;
	}
	.pld-description :global(a:hover) {
		text-decoration: underline;
	}
	.pld-description :global(blockquote) {
		border-left: 2px solid var(--accent);
		padding-left: 10px;
		color: var(--subtle);
		margin: 0.5em 0;
	}

	/* ── Push bar ────────────────────────────────────── */
	.pld-push-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 8px 12px;
		background: color-mix(in srgb, var(--panel-strong) 50%, transparent);
		border: 1px solid var(--border);
		border-radius: 8px;
		transition:
			background var(--transition-fast),
			border-color var(--transition-fast);
	}
	.pld-push-bar--action {
		background: color-mix(in srgb, var(--warning) 6%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--warning) 20%, var(--border));
	}
	.pld-push-stats {
		display: flex;
		align-items: center;
		gap: 10px;
		font-size: var(--text-xs);
		color: var(--muted);
	}
	.pld-push-stat {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}
	.pld-push-success {
		color: var(--success);
	}
	.pld-push-ok {
		color: var(--subtle);
	}
	.pld-push-error {
		color: var(--danger);
	}

	/* ── Checks ──────────────────────────────────────── */
	.pld-checks-container {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 10px 12px;
		border-radius: 8px;
		border: 1px solid var(--border);
		background: var(--panel-strong);
		transition:
			background var(--transition-fast),
			border-color var(--transition-fast);
	}
	.pld-checks-pass {
		background: color-mix(in srgb, var(--success) 6%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--success) 25%, var(--border));
	}
	.pld-checks-fail {
		background: color-mix(in srgb, var(--danger) 6%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--danger) 25%, var(--border));
	}
	.pld-checks-pending {
		background: color-mix(in srgb, var(--warning) 6%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--warning) 25%, var(--border));
	}
	.pld-check-row {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-xs);
	}
	.pld-check-name {
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	:global(.pld-check-pass) {
		color: var(--success);
	}
	:global(.pld-check-fail) {
		color: var(--danger);
	}
	:global(.pld-check-pending) {
		color: var(--warning);
	}
	:global(.pld-check-neutral) {
		color: var(--muted);
	}

	/* ── Stats ───────────────────────────────────────── */
	.pld-stat-add {
		color: var(--success);
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
	}
	.pld-stat-del {
		color: var(--danger);
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
	}
	.pld-icon-accent {
		color: var(--accent);
	}
	.pld-icon-warn {
		color: var(--warning);
	}
	.pld-empty {
		color: var(--muted);
		font-size: var(--text-xs);
	}
</style>
