<script lang="ts">
	import type { PullRequestStatusResult, RepoDiffSummary } from '../../types';
	import type { CheckStats } from './checkSidebarController';
	import RepoDiffHeaderPrBadge from './RepoDiffHeaderPrBadge.svelte';

	interface Props {
		repoName: string;
		repoDefaultBranch: string;
		repoStatusKnown: boolean;
		repoMissing: boolean;
		repoDirty: boolean;
		summary: RepoDiffSummary | null;
		effectiveMode: 'create' | 'status';
		prStatus: PullRequestStatusResult | null;
		checkStats: CheckStats;
		prStatusLoading: boolean;
		prReviewsLoading: boolean;
		diffMode?: 'split' | 'unified';
		onRefresh: () => void | Promise<void>;
		onClose: () => void;
		onOpenPrUrl: (url: string | undefined | null) => void;
	}

	/* eslint-disable prefer-const */
	// Svelte 5 bindable props require `let` in the `$props()` declaration.
	let {
		repoName,
		repoDefaultBranch,
		repoStatusKnown,
		repoMissing,
		repoDirty,
		summary,
		effectiveMode,
		prStatus,
		checkStats,
		prStatusLoading,
		prReviewsLoading,
		diffMode = $bindable('split'),
		onRefresh,
		onClose,
		onOpenPrUrl,
	}: Props = $props();
	/* eslint-enable prefer-const */
</script>

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
					><span class="add">+{summary.totalAdded}</span><span class="sep">/</span><span class="del"
						>-{summary.totalRemoved}</span
					></span
				>
			{/if}
			<RepoDiffHeaderPrBadge
				{effectiveMode}
				{prStatus}
				{checkStats}
				{prStatusLoading}
				{prReviewsLoading}
				{onOpenPrUrl}
			/>
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
		<button class="ghost" type="button" onclick={onRefresh}>Refresh</button>
		<button class="close" onclick={onClose} type="button">Back to terminal</button>
	</div>
</header>

<style>
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
		font-size: var(--text-2xl);
		font-weight: 600;
	}

	.meta {
		display: flex;
		align-items: center;
		gap: 12px;
		color: var(--muted);
		font-size: var(--text-sm);
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
		font-size: var(--text-sm);
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

	.ghost {
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		color: var(--text);
		padding: 8px 12px;
		border-radius: var(--radius-md);
		cursor: pointer;
		font-size: var(--text-sm);
		transition:
			border-color var(--transition-fast),
			background var(--transition-fast);
	}

	.ghost:hover:not(:disabled) {
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.04);
	}
</style>
