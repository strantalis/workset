<script lang="ts">
	import Icon from '@iconify/svelte';
	import {
		ChevronDown,
		ChevronRight,
		FilePlus,
		FolderTree,
		GitBranch,
		GitMerge,
		GitPullRequest,
		LoaderCircle,
		MessageCircle,
		PanelLeftClose,
		RotateCcw,
		Search,
		Trash2,
	} from '@lucide/svelte';
	import type { RepoDiffFileSummary } from '../../types';
	import type { RepoTreeNode } from '../repo-files/tree';
	import { getRepoFileIcon } from '../repo-files/fileIcons';
	import { tooltip } from '../../actions/tooltip';
	type InlineCreateTreeNode = {
		kind: 'inline-create';
		key: 'inline-create';
		repoId: string;
		parentDirPath: string;
		depth: number;
	};
	type SidebarTreeNode = RepoTreeNode | InlineCreateTreeNode;

	type RootMeta = {
		kind: 'repo' | 'extra';
		gitDetected: boolean;
	};

	type RootState =
		| { status: 'idle' }
		| { status: 'loading' }
		| { status: 'loaded' }
		| { status: 'error'; message?: string };

	interface Props {
		showRepoActions: boolean;
		showSelectedPrAction: boolean;
		searchQuery: string;
		treeNodes: SidebarTreeNode[];
		focusedNodeIndex: number;
		expandedNodes: Set<string>;
		childCounts: Map<string, number>;
		repoChangeStats: Map<string, { added: number; removed: number; count: number }>;
		changedDirSet: Set<string>;
		dirChangeCount: Map<string, number>;
		dirCommentCount: Map<string, number>;
		selectedTreeKey: string | null;
		pendingDeleteKey: string | null;
		selectedRepoId: string | null;
		selectedFilePath: string | null;
		editedContent: string | null;
		inlineCreateDraft: string;
		inlineCreatePending: boolean;
		getRootMeta: (repoId: string) => RootMeta | undefined;
		getRepoPrState: (repoId: string) => 'none' | 'open' | 'merged' | 'draft';
		getRootState: (repoId: string) => RootState | undefined;
		getDirEntryError: (repoId: string, dirPath: string) => string | undefined;
		isFileChanged: (repoId: string, path: string) => boolean;
		getFileDiffInfo: (repoId: string, path: string) => RepoDiffFileSummary | undefined;
		getFileCommentCount: (repoId: string, path: string) => number;
		onOpenSelectedPr: () => void;
		onCreatePr: () => void;
		onLocalMerge: () => void;
		onRefresh: () => void;
		onNewFile: () => void;
		onHideTree: () => void;
		onSearchQueryChange: (value: string) => void;
		onToggleNode: (node: Extract<RepoTreeNode, { kind: 'repo' | 'dir' }>) => void;
		onTreeKeydown: (event: KeyboardEvent) => void;
		onInlineCreateDraftChange: (value: string) => void;
		onCommitInlineCreate: () => void;
		onCancelInlineCreate: () => void;
		onOpenTrackedPr: (repoId: string) => void;
		onSelectFile: (repoId: string, path: string) => void;
		onDeleteFile: (repoId: string, path: string) => void;
		onConfirmDelete: () => void;
		onCancelDelete: () => void;
	}

	const {
		showRepoActions,
		showSelectedPrAction,
		searchQuery,
		treeNodes,
		focusedNodeIndex,
		expandedNodes,
		childCounts,
		repoChangeStats,
		changedDirSet,
		dirChangeCount,
		dirCommentCount,
		selectedTreeKey,
		pendingDeleteKey,
		selectedRepoId,
		selectedFilePath,
		editedContent,
		inlineCreateDraft,
		inlineCreatePending,
		getRootMeta,
		getRepoPrState,
		getRootState,
		getDirEntryError,
		isFileChanged,
		getFileDiffInfo,
		getFileCommentCount,
		onOpenSelectedPr,
		onCreatePr,
		onLocalMerge,
		onRefresh,
		onNewFile,
		onHideTree,
		onSearchQueryChange,
		onToggleNode,
		onTreeKeydown,
		onInlineCreateDraftChange,
		onCommitInlineCreate,
		onCancelInlineCreate,
		onOpenTrackedPr,
		onSelectFile,
		onDeleteFile,
		onConfirmDelete,
		onCancelDelete,
	}: Props = $props();

	let inlineCreateInput = $state<HTMLInputElement | null>(null);
	let inlineCreateWasVisible = false;
	$effect(() => {
		const inlineCreateVisible = treeNodes.some((node) => node.kind === 'inline-create');
		if (!inlineCreateVisible) {
			inlineCreateWasVisible = false;
			return;
		}
		if (inlineCreateWasVisible) return;
		inlineCreateWasVisible = true;
		queueMicrotask(() => {
			inlineCreateInput?.focus();
			inlineCreateInput?.select();
		});
	});
</script>

<aside class="urv-sidebar">
	<div class="urv-tree-header">
		<div class="urv-tree-title">
			<FolderTree size={14} />
			<span>Files</span>
		</div>
		<div class="urv-tree-actions">
			{#if showRepoActions}
				{#if showSelectedPrAction}
					<button
						type="button"
						class="urv-tree-action urv-action-pr"
						aria-label="View Pull Request"
						use:tooltip={'View Pull Request'}
						onclick={onOpenSelectedPr}
					>
						<GitPullRequest size={13} />
					</button>
				{:else}
					<button
						type="button"
						class="urv-tree-action"
						aria-label="Create Pull Request"
						use:tooltip={'Create Pull Request'}
						onclick={onCreatePr}
					>
						<GitPullRequest size={13} />
					</button>
					<button
						type="button"
						class="urv-tree-action"
						aria-label="Local Merge"
						use:tooltip={'Local Merge'}
						onclick={onLocalMerge}
					>
						<GitMerge size={13} />
					</button>
				{/if}
			{/if}
			<button
				type="button"
				class="urv-tree-action"
				aria-label="Refresh file tree"
				use:tooltip={'Refresh file tree'}
				onclick={onRefresh}
			>
				<RotateCcw size={14} />
			</button>
			<button type="button" class="urv-tree-action" aria-label="New file" use:tooltip={'New file'} onclick={onNewFile}>
				<FilePlus size={14} />
			</button>
			<button
				type="button"
				class="urv-tree-action"
				aria-label="Hide file tree"
				use:tooltip={'Hide file tree'}
				onclick={onHideTree}
			>
				<PanelLeftClose size={14} />
			</button>
		</div>
	</div>
	<div class="urv-tree-search">
		<Search size={13} />
		<input
			class="ws-field-input ws-field-input--ghost"
			type="text"
			placeholder="Filter files..."
			value={searchQuery}
			oninput={(event) => onSearchQueryChange((event.currentTarget as HTMLInputElement).value)}
		/>
	</div>
	<div class="urv-tree-list" tabindex="0" role="tree" onkeydown={onTreeKeydown}>
		{#each treeNodes as node, idx (node.key)}
			{#if node.kind === 'repo'}
				{@const rootMeta = getRootMeta(node.repoId)}
				{@const isRepoRoot = rootMeta?.kind === 'repo'}
				{@const stats = isRepoRoot ? repoChangeStats.get(node.repoId) : undefined}
				{@const prState = isRepoRoot ? getRepoPrState(node.repoId) : 'none'}
				{@const rootState = isRepoRoot ? getRootState(node.repoId) : undefined}
				<button
					type="button"
					class="urv-tree-repo"
					class:extra-root={!isRepoRoot}
					class:active={selectedTreeKey === node.key}
					class:expanded={expandedNodes.has(node.key)}
					class:focused={idx === focusedNodeIndex}
					style={`--depth:${node.depth};`}
					onclick={() => onToggleNode(node)}
				>
					{#if expandedNodes.has(node.key)}
						<ChevronDown size={13} />
					{:else}
						<ChevronRight size={13} />
					{/if}
					{#if isRepoRoot}
						<GitBranch size={14} />
					{:else}
						<FolderTree size={14} />
					{/if}
					<span class="urv-tree-label">{node.label}</span>
					{#if !isRepoRoot && rootMeta?.gitDetected}
						<span class="urv-tree-extra-badge">git</span>
					{/if}
					{#if prState === 'open'}
						<span
							class="urv-pr-indicator urv-pr-open"
							role="button"
							tabindex="-1"
							title="View Pull Request"
							onclick={(event) => {
								event.stopPropagation();
								onOpenTrackedPr(node.repoId);
							}}
							onkeydown={() => {}}
						>
							<GitPullRequest size={12} />
						</span>
					{:else if prState === 'draft'}
						<span
							class="urv-pr-indicator urv-pr-draft"
							role="button"
							tabindex="-1"
							title="View Draft PR"
							onclick={(event) => {
								event.stopPropagation();
								onOpenTrackedPr(node.repoId);
							}}
							onkeydown={() => {}}
						>
							<GitPullRequest size={12} />
						</span>
					{:else if prState === 'merged'}
						<span
							class="urv-pr-indicator urv-pr-merged"
							role="button"
							tabindex="-1"
							title="View Merged PR"
							onclick={(event) => {
								event.stopPropagation();
								onOpenTrackedPr(node.repoId);
							}}
							onkeydown={() => {}}
						>
							<GitMerge size={12} />
						</span>
					{/if}
					{#if stats && stats.count > 0}
						<span class="urv-tree-change-badge">
							<span class="urv-badge-add">+{stats.added}</span>
							<span class="urv-badge-del">-{stats.removed}</span>
						</span>
					{:else if childCounts.has(node.key)}
						<span class="urv-tree-count">{childCounts.get(node.key)}</span>
					{/if}
				</button>
				{#if expandedNodes.has(node.key) && rootState?.status === 'loading'}
					<div class="urv-tree-state" style="--depth:1;">
						<span class="spin"><LoaderCircle size={16} /></span>
						<span>Loading...</span>
					</div>
				{:else if expandedNodes.has(node.key) && rootState?.status === 'error'}
					<div class="urv-tree-state error" style="--depth:1;">
						{rootState.message}
					</div>
				{/if}
				{#if expandedNodes.has(node.key) && getDirEntryError(node.repoId, '')}
					<div class="urv-tree-state error" style="--depth:1;">
						{getDirEntryError(node.repoId, '')}
					</div>
				{/if}
			{:else if node.kind === 'dir'}
				{@const dirChanged = changedDirSet.has(node.key)}
				{@const dirChanges = dirChangeCount.get(node.key) ?? 0}
				{@const commentCount = dirCommentCount.get(node.key) ?? 0}
				<button
					type="button"
					class="urv-tree-dir"
					class:active={selectedTreeKey === node.key}
					class:expanded={expandedNodes.has(node.key)}
					class:has-changes={dirChanged}
					class:focused={idx === focusedNodeIndex}
					style={`--depth:${node.depth};`}
					onclick={() => onToggleNode(node)}
				>
					{#if expandedNodes.has(node.key)}
						<ChevronDown size={13} />
					{:else}
						<ChevronRight size={13} />
					{/if}
					<span class="urv-tree-label">{node.label}</span>
					{#if commentCount > 0}
						<span
							class="urv-tree-comment-badge"
							title={`${commentCount} unresolved review thread${commentCount === 1 ? '' : 's'}`}
						>
							<MessageCircle size={12} />
							<span>{commentCount}</span>
						</span>
					{/if}
					{#if dirChanged && dirChanges > 0}
						<span class="urv-tree-dir-changes">{dirChanges}</span>
					{:else if childCounts.has(node.key)}
						<span class="urv-tree-count">{childCounts.get(node.key)}</span>
					{/if}
				</button>
				{#if expandedNodes.has(node.key) && getDirEntryError(node.repoId, node.path)}
					<div class="urv-tree-state error" style={`--depth:${node.depth + 1};`}>
						{getDirEntryError(node.repoId, node.path)}
					</div>
				{/if}
			{:else if node.kind === 'file'}
				{@const changed = isFileChanged(node.repoId, node.path)}
				{@const diffInfo = changed ? getFileDiffInfo(node.repoId, node.path) : undefined}
				{@const commentCount = getFileCommentCount(node.repoId, node.path)}
				<div class="urv-tree-file-row">
					<button
						type="button"
						class="urv-tree-file"
						class:active={selectedTreeKey === node.key}
						class:selected={node.path === selectedFilePath && node.repoId === selectedRepoId}
						class:changed
						class:dirty={node.repoId === selectedRepoId &&
							node.path === selectedFilePath &&
							editedContent !== null}
						class:focused={idx === focusedNodeIndex}
						style={`--depth:${node.depth};`}
						title={node.path}
						onclick={() => onSelectFile(node.repoId, node.path)}
					>
						<span class="urv-file-icon" data-icon={getRepoFileIcon(node.path)}>
							<Icon icon={getRepoFileIcon(node.path)} width="14" />
						</span>
						<span class="urv-tree-file-name">{node.label}</span>
						{#if diffInfo}
							<span class="urv-tree-file-diff">
								{#if diffInfo.added > 0}<span class="urv-badge-add">+{diffInfo.added}</span>{/if}
								{#if diffInfo.removed > 0}<span class="urv-badge-del">-{diffInfo.removed}</span
									>{/if}
							</span>
						{/if}
						{#if commentCount > 0}
							<span
								class="urv-tree-comment-badge urv-tree-file-comments"
								title={`${commentCount} unresolved review thread${commentCount === 1 ? '' : 's'}`}
							>
								<MessageCircle size={12} />
								<span>{commentCount}</span>
							</span>
						{/if}
					</button>
					{#if pendingDeleteKey === node.key}
						<div class="urv-tree-file-confirm">
							<button
								type="button"
								class="urv-tree-file-confirm-btn danger"
								aria-label={`Confirm delete ${node.path}`}
								onmousedown={(event) => event.stopPropagation()}
								onclick={(event) => {
									event.stopPropagation();
									onConfirmDelete();
								}}
							>
								Delete?
							</button>
							<button
								type="button"
								class="urv-tree-file-confirm-btn"
								aria-label={`Cancel delete ${node.path}`}
								onmousedown={(event) => event.stopPropagation()}
								onclick={(event) => {
									event.stopPropagation();
									onCancelDelete();
								}}
							>
								Cancel
							</button>
						</div>
					{:else}
						<button
							type="button"
							class="urv-tree-file-delete"
							aria-label={`Delete ${node.path}`}
							title="Delete file"
							onmousedown={(event) => event.stopPropagation()}
							onclick={(event) => {
								event.stopPropagation();
								onDeleteFile(node.repoId, node.path);
							}}
						>
							<Trash2 size={12} />
						</button>
					{/if}
				</div>
			{:else}
				<div class="urv-tree-inline" style={`--depth:${node.depth};`}>
					<span class="urv-file-icon" data-icon={getRepoFileIcon('new-file.ts')}>
						<Icon icon={getRepoFileIcon('new-file.ts')} width="13" />
					</span>
					<input
						bind:this={inlineCreateInput}
						type="text"
						class="ws-field-input ws-field-input--compact ws-field-input--mono"
						placeholder="new-file.ts"
						value={inlineCreateDraft}
						disabled={inlineCreatePending}
						oninput={(event) =>
							onInlineCreateDraftChange((event.currentTarget as HTMLInputElement).value)}
						onkeydown={(event) => {
							if (event.key === 'Enter') {
								event.preventDefault();
								onCommitInlineCreate();
							} else if (event.key === 'Escape') {
								event.preventDefault();
								onCancelInlineCreate();
							}
						}}
						onblur={() => {
							if (!inlineCreatePending) onCancelInlineCreate();
						}}
					/>
				</div>
			{/if}
		{/each}
	</div>
</aside>
