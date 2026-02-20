<script lang="ts">
	import {
		ArrowUpRight,
		Box,
		CheckCircle2,
		ChevronLeft,
		ChevronRight,
		GitBranch,
		GitPullRequest,
		MessageSquare,
		Upload,
	} from '@lucide/svelte';
	import type { PrListItem } from '../../view-models/prViewModel';

	type Partitions = {
		active: PrListItem[];
		merged: PrListItem[];
		tracked: PrListItem[];
		readyToPR: PrListItem[];
	};

	interface Props {
		workspaceName: string;
		viewMode: 'active' | 'ready';
		canCollapseSidebar: boolean;
		trackedCount: number;
		readyCount: number;
		activeCount: number;
		mergedCount: number;
		partitions: Partitions;
		selectedItemId: string | null;
		resolveTrackedTitle: (repoId: string, fallbackTitle: string) => string;
		onToggleSidebar: () => void;
		onViewModeChange: (mode: 'active' | 'ready') => void;
		onSelectItem: (itemId: string) => void;
	}

	const {
		workspaceName,
		viewMode,
		canCollapseSidebar,
		trackedCount,
		readyCount,
		activeCount,
		mergedCount,
		partitions,
		selectedItemId,
		resolveTrackedTitle,
		onToggleSidebar,
		onViewModeChange,
		onSelectItem,
	}: Props = $props();
</script>

<aside class="sidebar">
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
				onclick={onToggleSidebar}
			>
				<ChevronLeft size={14} />
			</button>
		</div>
		<div class="ws-badge">
			<Box size={14} class="ws-badge-icon" />
			<span class="ws-badge-name">{workspaceName}</span>
		</div>
	</div>

	<div class="mode-switch">
		<button
			type="button"
			class="ms-btn"
			class:active={viewMode === 'active'}
			onclick={() => onViewModeChange('active')}
		>
			<GitPullRequest size={14} />
			Tracked PRs
			<span class="ms-count">{trackedCount}</span>
		</button>
		<button
			type="button"
			class="ms-btn"
			class:active={viewMode === 'ready'}
			onclick={() => onViewModeChange('ready')}
		>
			<Upload size={14} class={viewMode === 'ready' ? 'text-green' : ''} />
			Ready to PR
			{#if readyCount > 0}
				<span class="ms-count ready">{readyCount}</span>
			{/if}
		</button>
	</div>

	<div class="list">
		{#if viewMode === 'active'}
			{#if partitions.tracked.length > 0}
				{#if partitions.active.length > 0}
					<div class="list-group-title">Open ({activeCount})</div>
					{#each partitions.active as item (item.id)}
						{@const isActive = item.id === selectedItemId}
						<button
							type="button"
							class="list-item"
							class:active={isActive}
							onclick={() => onSelectItem(item.id)}
						>
							<div class="li-top">
								<h3 class="li-title" class:bright={isActive}>
									{resolveTrackedTitle(item.repoId, item.title)}
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
				{/if}
				{#if partitions.merged.length > 0}
					<div class="list-group-title">Merged ({mergedCount})</div>
					{#each partitions.merged as item (item.id)}
						{@const isActive = item.id === selectedItemId}
						<button
							type="button"
							class="list-item list-item-merged"
							class:active={isActive}
							onclick={() => onSelectItem(item.id)}
						>
							<div class="li-top">
								<h3 class="li-title" class:bright={isActive}>
									{resolveTrackedTitle(item.repoId, item.title)}
								</h3>
								{#if isActive}<ChevronRight size={14} class="li-chevron" />{/if}
							</div>
							<div class="li-meta">
								<span class="li-repo">{item.repoName}</span>
								<span class="li-sep">·</span>
								<span class="li-merged">merged</span>
								<span class="li-sep">·</span>
								<span class="li-cleanup">cleanup candidate</span>
							</div>
						</button>
					{/each}
				{/if}
			{:else}
				<div class="list-empty">
					<CheckCircle2 size={24} />
					<p>No tracked PRs</p>
				</div>
			{/if}
		{:else if partitions.readyToPR.length > 0}
			{#each partitions.readyToPR as item (item.id)}
				{@const isActive = item.id === selectedItemId}
				<button
					type="button"
					class="list-item"
					class:active-ready={isActive}
					onclick={() => onSelectItem(item.id)}
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

<style src="./PROrchestrationSidebar.css"></style>
