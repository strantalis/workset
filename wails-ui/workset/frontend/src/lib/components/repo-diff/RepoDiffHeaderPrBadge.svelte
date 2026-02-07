<script lang="ts">
	import type { PullRequestStatusResult } from '../../types';
	import type { CheckStats } from './checkSidebarController';

	interface Props {
		effectiveMode: 'create' | 'status';
		prStatus: PullRequestStatusResult | null;
		checkStats: CheckStats;
		prStatusLoading: boolean;
		prReviewsLoading: boolean;
		onOpenPrUrl: (url: string | undefined | null) => void;
	}

	const {
		effectiveMode,
		prStatus,
		checkStats,
		prStatusLoading,
		prReviewsLoading,
		onOpenPrUrl,
	}: Props = $props();
</script>

{#if effectiveMode === 'status' && prStatus}
	<button
		class="pr-badge"
		type="button"
		onclick={() => onOpenPrUrl(prStatus.pullRequest.url)}
		title="Open PR #{prStatus.pullRequest.number} on GitHub"
	>
		<span class="pr-badge-number">PR #{prStatus.pullRequest.number}</span>
		<span class={`pr-badge-state pr-badge-state-${prStatus.pullRequest.state.toLowerCase()}`}
			>{prStatus.pullRequest.state}</span
		>
		<span class="pr-badge-divider">·</span>
		{#if checkStats.total === 0}
			<span class="pr-badge-checks muted">No checks</span>
		{:else if checkStats.failed > 0}
			<span class="pr-badge-checks failed"
				><svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
					><circle cx="12" cy="12" r="10" /><path d="m15 9-6 6" /><path d="m9 9 6 6" /></svg
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
				><svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
					><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" /><path d="M22 4 12 14.01l-3-3" /></svg
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

<style>
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
</style>
