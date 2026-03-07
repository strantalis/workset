<script lang="ts">
	import {
		AlertCircle,
		ArrowUpFromLine,
		CheckCircle2,
		ChevronLeft,
		ChevronRight,
		Circle,
		FileCode,
		FileMinus,
		FilePlus,
		GitBranch,
		GitMerge,
		GitPullRequest,
		MessageSquare,
		XCircle,
	} from '@lucide/svelte';
	import type { RepoDiffFileSummary, Workspace } from '../../types';
	import type { PrListItem } from '../../view-models/prViewModel';

	type Partitions = {
		active: PrListItem[];
		merged: PrListItem[];
		tracked: PrListItem[];
		readyToPR: PrListItem[];
	};

	interface Props {
		workspace: Workspace | null;
		workspaceName: string;
		viewMode: 'active' | 'ready';
		canCollapseSidebar: boolean;
		partitions: Partitions;
		prItems: PrListItem[];
		selectedItemId: string | null;
		prComposerItemId: string | null;
		prComposerMode?: 'pull_request' | 'local_merge';
		selectedReadyRepoFiles?: RepoDiffFileSummary[];
		selectedFilePath?: string | null;
		resolveTrackedTitle: (repoId: string, fallbackTitle: string) => string;
		onToggleSidebar: () => void;
		onViewModeChange: (mode: 'active' | 'ready') => void;
		onSelectItem: (itemId: string) => void;
		onSelectRepoFile: (itemId: string, filePath: string) => void;
		onOpenPrComposer: (itemId: string, mode?: 'pull_request' | 'local_merge') => void;
	}

	const {
		workspace,
		workspaceName,
		viewMode,
		canCollapseSidebar,
		partitions,
		prItems,
		selectedItemId,
		prComposerItemId,
		prComposerMode = 'pull_request',
		selectedReadyRepoFiles = [],
		selectedFilePath = null,
		resolveTrackedTitle,
		onToggleSidebar,
		onViewModeChange,
		onSelectItem,
		onSelectRepoFile,
		onOpenPrComposer,
	}: Props = $props();

	const repoById = $derived.by(() => {
		const map = new Map<string, Workspace['repos'][number]>();
		for (const repo of workspace?.repos ?? []) {
			map.set(repo.id, repo);
		}
		return map;
	});

	const changedRepoIds = $derived.by(
		() => new Set(partitions.readyToPR.map((item) => item.repoId)),
	);
	const trackedRepoIds = $derived.by(() => new Set(partitions.tracked.map((item) => item.repoId)));
	const changedItems = $derived.by(() => prItems.filter((item) => changedRepoIds.has(item.repoId)));
	const cleanItems = $derived.by(() => prItems.filter((item) => !changedRepoIds.has(item.repoId)));
	const trackedItems = $derived.by(() => prItems.filter((item) => trackedRepoIds.has(item.repoId)));
	const noPrItems = $derived.by(() => prItems.filter((item) => !trackedRepoIds.has(item.repoId)));

	const repoCount = $derived(prItems.length);
	const changedCount = $derived(changedItems.length);
	const openPrCount = $derived(partitions.active.length);

	const selectRepo = (itemId: string): void => {
		if (viewMode !== 'ready') onViewModeChange('ready');
		onSelectItem(itemId);
	};

	const selectPr = (itemId: string): void => {
		if (viewMode !== 'active') onViewModeChange('active');
		onSelectItem(itemId);
	};

	const trackedStatusTone = (item: PrListItem): 'open' | 'merged' | 'blocked' | 'pending' => {
		if (item.status === 'merged') return 'merged';
		if (item.status === 'blocked') return 'blocked';
		if (item.status === 'running') return 'pending';
		return 'open';
	};

	const fileName = (path: string): string => {
		const parts = path.split('/');
		return parts[parts.length - 1] || path;
	};

	type SidebarRepoFile = Workspace['repos'][number]['files'][number] | RepoDiffFileSummary;

	const resolveReadyRepoFiles = (
		repo: Workspace['repos'][number] | undefined,
		isActive: boolean,
	): SidebarRepoFile[] => {
		if (viewMode === 'ready' && isActive) return selectedReadyRepoFiles;
		return repo?.files ?? [];
	};

	const fileTone = (file: SidebarRepoFile): 'added' | 'removed' | 'changed' => {
		if (file.added > 0 && file.removed === 0) return 'added';
		if (file.removed > 0 && file.added === 0) return 'removed';
		return 'changed';
	};
</script>

<aside class="sidebar">
	<div class="list">
		<div class="section">
			<div class="section-head">
				<div class="section-left">
					<ArrowUpFromLine size={11} />
					<span>Repositories</span>
				</div>
				<div class="section-head-right">
					<div class="section-count">
						<span class="accent">{changedCount} modified</span> / {repoCount}
					</div>
					<button
						type="button"
						class="sidebar-toggle-btn"
						class:disabled={!canCollapseSidebar}
						aria-label="Collapse sidebar"
						title={canCollapseSidebar ? 'Collapse sidebar' : 'Select an item to collapse'}
						disabled={!canCollapseSidebar}
						onclick={onToggleSidebar}
					>
						<ChevronLeft size={13} />
					</button>
				</div>
			</div>

			<div class="section-body">
				{#each changedItems as item (item.id)}
					{@const repo = repoById.get(item.repoId)}
					{@const isActive = selectedItemId === item.id && viewMode === 'ready'}
					{@const isComposerActive = isActive && prComposerItemId === item.id}
					{@const isPullRequestActive = isComposerActive && prComposerMode === 'pull_request'}
					{@const isLocalMergeActive = isComposerActive && prComposerMode === 'local_merge'}
					{@const repoFiles = resolveReadyRepoFiles(repo, isActive)}
					<div class="repo-block" class:active={isActive}>
						<button
							type="button"
							class="repo-row"
							class:active={isActive}
							onclick={() => selectRepo(item.id)}
						>
							<span class="repo-chevron" class:open={isActive}>
								<ChevronRight size={11} />
							</span>
							<span class="repo-icon"><GitBranch size={11} /></span>
							<div class="row-main">
								<div class="row-line-1">
									<span class="repo-name">{item.repoName}</span>
									<div class="line-stat">
										<span class="plus">+{repo?.diff.added ?? 0}</span>
										<span class="minus">-{repo?.diff.removed ?? 0}</span>
									</div>
								</div>
								<div class="row-line-2">
									<span class="thread-name">{workspaceName}</span>
									<span class="branch-name">{item.branch}</span>
								</div>
							</div>
						</button>
						{#if isActive && repo}
							<div class="repo-files">
								{#each repoFiles as file (`${item.id}:${file.path}`)}
									{@const tone = fileTone(file)}
									<button
										type="button"
										class="repo-file-row"
										class:active={selectedFilePath === file.path}
										onclick={() => onSelectRepoFile(item.id, file.path)}
									>
										<span class="repo-file-icon">
											{#if tone === 'added'}
												<FilePlus size={10} />
											{:else if tone === 'removed'}
												<FileMinus size={10} />
											{:else}
												<FileCode size={10} />
											{/if}
										</span>
										<span class="repo-file-name">{fileName(file.path)}</span>
										<span class="repo-file-add">+{file.added}</span>
										{#if file.removed > 0}
											<span class="repo-file-remove">-{file.removed}</span>
										{/if}
									</button>
								{/each}
								<div class="repo-actions">
									<button
										type="button"
										class="repo-action-btn"
										class:primary={isPullRequestActive}
										aria-pressed={isPullRequestActive}
										onclick={(event) => {
											event.stopPropagation();
											onOpenPrComposer(item.id, 'pull_request');
										}}
									>
										<GitPullRequest size={10} />
										Pull Request
									</button>
									<button
										type="button"
										class="repo-action-btn"
										class:primary={isLocalMergeActive}
										aria-pressed={isLocalMergeActive}
										onclick={(event) => {
											event.stopPropagation();
											onOpenPrComposer(item.id, 'local_merge');
										}}
									>
										<GitMerge size={10} />
										Local Merge
									</button>
								</div>
							</div>
						{/if}
					</div>
				{/each}

				{#each cleanItems as item (item.id)}
					<div class="repo-row clean">
						<span class="repo-chevron-spacer"></span>
						<span class="repo-icon"><GitBranch size={11} /></span>
						<div class="row-main">
							<div class="row-line-1">
								<span class="repo-name">{item.repoName}</span>
								<span class="clean-state">clean</span>
							</div>
							<div class="row-line-2">
								<span class="branch-name">{item.branch || 'main'}</span>
							</div>
						</div>
					</div>
				{/each}
			</div>
		</div>

		<div class="section">
			<div class="section-head">
				<div class="section-left">
					<GitPullRequest size={11} />
					<span>Pull Requests</span>
				</div>
				<div class="section-count">
					<span class="accent">{openPrCount} open</span> / {repoCount} repos
				</div>
			</div>

			<div class="section-body">
				{#each trackedItems as item (item.id)}
					{@const isActive = selectedItemId === item.id && viewMode === 'active'}
					{@const tone = trackedStatusTone(item)}
					<button
						type="button"
						class="pr-row"
						class:active={isActive}
						onclick={() => selectPr(item.id)}
					>
						<div class="pr-status">
							{#if tone === 'open'}
								<CheckCircle2 size={12} class="icon-open" />
							{:else if tone === 'merged'}
								<Circle size={12} class="icon-merged" />
							{:else if tone === 'pending'}
								<AlertCircle size={12} class="icon-pending" />
							{:else}
								<XCircle size={12} class="icon-blocked" />
							{/if}
						</div>
						<div class="pr-main">
							<div class="pr-title">{resolveTrackedTitle(item.repoId, item.title)}</div>
							<div class="pr-meta">
								<span class="pr-meta-mono">{item.repoName}</span>
								<span class="dot">·</span>
								<span>{workspaceName}</span>
								{#if item.draft}
									<span class="draft-pill">Draft</span>
								{/if}
							</div>
							<div class="pr-submeta">
								{#if item.author}
									<span>{item.author}</span>
									<span class="dot">·</span>
								{/if}
								<span>{item.updatedAtLabel}</span>
								{#if item.commentsCount > 0}
									<span class="dot">·</span>
									<span class="comment-count"><MessageSquare size={9} /> {item.commentsCount}</span>
								{/if}
								{#if tone === 'merged'}
									<span class="dot">·</span>
									<span class="merged-pill">Merged</span>
								{/if}
							</div>
						</div>
					</button>
				{/each}

				{#each noPrItems as item (item.id)}
					<div class="pr-row clean-pr">
						<div class="pr-status">
							<Circle size={12} class="icon-idle" />
						</div>
						<div class="pr-main">
							<span class="repo-name">{item.repoName}</span>
							<div class="pr-submeta">No open PRs</div>
						</div>
					</div>
				{/each}
			</div>
		</div>
	</div>
</aside>

<style src="./PROrchestrationSidebar.css"></style>
