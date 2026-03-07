<script lang="ts">
	import {
		Clock,
		ExternalLink,
		GitMerge,
		GitPullRequest,
		Loader2,
		ThumbsDown,
		ThumbsUp,
	} from '@lucide/svelte';
	import type { PullRequestCreated } from '../../types';

	interface SelectedItem {
		repoName: string;
		branch: string;
		title: string;
		author: string;
		updatedAtLabel: string;
		dirtyFiles: number;
		repoId: string;
	}

	interface Props {
		trackedPr: PullRequestCreated | null;
		trackedPrLoading: boolean;
		selectedItem: SelectedItem;
		workspaceName: string;
		trackedTitle: string;
		checkStats: { failed: number; pending: number };
		activeTab: 'overview' | 'files';
		filesCount: number;
		onActiveTabChange: (tab: 'overview' | 'files') => void;
		onOpenExternalUrl: (url: string | undefined | null) => void;
	}

	const {
		trackedPr,
		trackedPrLoading,
		selectedItem,
		workspaceName,
		trackedTitle,
		checkStats,
		activeTab,
		filesCount,
		onActiveTabChange,
		onOpenExternalUrl,
	}: Props = $props();

	const isMergedTrackedPr = (pr: PullRequestCreated | undefined | null): boolean =>
		Boolean(pr && (pr.merged === true || pr.state.toLowerCase() === 'merged'));
</script>

<div class="pr-header">
	<div class="prh-top">
		<div class="prh-icon">
			{#if trackedPr && isMergedTrackedPr(trackedPr)}
				<GitMerge size={16} class="prh-icon-merged" />
			{:else if trackedPr?.draft}
				<GitPullRequest size={16} class="prh-icon-draft" />
			{:else}
				<GitPullRequest size={16} class="prh-icon-open" />
			{/if}
		</div>
		<div class="prh-left">
			<h1 class="prh-title">{trackedTitle}</h1>
			<div class="prh-meta">
				<span class="prh-meta-mono">{selectedItem.repoName}</span>
				<span class="prh-meta-dot">·</span>
				<span>{workspaceName}</span>
				<span class="prh-meta-dot">·</span>
				<span class="prh-meta-accent">{selectedItem.branch}</span>
				<span class="prh-meta-arrow">→</span>
				<span class="prh-meta-mono">{trackedPr?.baseBranch ?? 'main'}</span>
				{#if selectedItem.author}
					<span class="prh-meta-dot">·</span>
					<span>by {selectedItem.author}</span>
				{/if}
				<span class="prh-meta-dot">·</span>
				<Clock size={10} />
				<span>{selectedItem.updatedAtLabel}</span>
			</div>
		</div>
		{#if trackedPr?.draft}
			<span class="prh-draft-badge">Draft</span>
		{/if}
	</div>

	<div class="prh-actions-row">
		<div class="prh-actions">
			{#if trackedPr && !isMergedTrackedPr(trackedPr) && trackedPr.state === 'open'}
				<button type="button" class="prh-btn prh-btn-approve">
					<ThumbsUp size={12} />
					Approve
				</button>
				<button type="button" class="prh-btn prh-btn-neutral">
					<ThumbsDown size={12} />
					Request Changes
				</button>
				<button
					type="button"
					class="prh-btn prh-btn-merge"
					class:prh-btn-disabled={checkStats.failed > 0 || checkStats.pending > 0}
					disabled={checkStats.failed > 0 || checkStats.pending > 0}
				>
					<GitMerge size={12} />
					Merge PR
				</button>
			{/if}
			{#if trackedPr}
				<button
					type="button"
					class="prh-btn-icon"
					title="Open in GitHub"
					onclick={() => onOpenExternalUrl(trackedPr?.url)}
				>
					<ExternalLink size={12} />
				</button>
			{:else if trackedPrLoading}
				<span class="prh-loading"><Loader2 size={14} class="spin" /></span>
			{/if}
		</div>

		<div class="prh-tab-switcher">
			<button
				type="button"
				class="prh-tab-seg"
				class:active={activeTab === 'overview'}
				onclick={() => onActiveTabChange('overview')}
			>
				Overview
			</button>
			<button
				type="button"
				class="prh-tab-seg"
				class:active={activeTab === 'files'}
				onclick={() => onActiveTabChange('files')}
			>
				Files
				<span class="prh-tab-count">{filesCount}</span>
			</button>
		</div>
	</div>
</div>

<style src="./PROrchestrationActiveHeader.css"></style>
