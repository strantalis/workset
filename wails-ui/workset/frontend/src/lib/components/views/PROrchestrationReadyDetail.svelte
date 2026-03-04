<script lang="ts">
	import { AlertCircle, FileCode, GitBranch, Loader2, Upload } from '@lucide/svelte';
	import type { RepoDiffFileSummary, RepoFileDiff } from '../../types';

	interface ReadyDetailItem {
		id: string;
		repoId: string;
		repoName: string;
		branch: string;
		dirtyFiles: number;
		ahead: number;
		hasLocalDiff: boolean;
	}

	type FallbackFile = {
		path: string;
		added: number;
		removed: number;
	};

	interface Props {
		selectedItem: ReadyDetailItem;
		workspaceName: string;
		filesForDetail: RepoDiffFileSummary[];
		totalAdd: number;
		totalDel: number;
		diffSummaryLoading: boolean;
		fallbackFiles: FallbackFile[];
		selectedSource: 'pr' | 'local';
		selectedFileIdx: number;
		fileDiffError: string | null;
		fileDiffContent: RepoFileDiff | null;
		fileDiffLoading: boolean;
		commitPushLoading: boolean;
		commitPushRepoId: string | null;
		onPushFromSidebar: (itemId: string) => Promise<void> | void;
		onSelectSourceFile: (source: 'pr' | 'local', index: number) => void;
		diffContainer?: HTMLElement | null;
	}

	/* eslint-disable prefer-const */
	let {
		selectedItem,
		workspaceName,
		filesForDetail,
		totalAdd,
		totalDel,
		diffSummaryLoading,
		fallbackFiles,
		selectedSource,
		selectedFileIdx,
		fileDiffError,
		fileDiffContent,
		fileDiffLoading,
		commitPushLoading,
		commitPushRepoId,
		onPushFromSidebar,
		onSelectSourceFile,
		diffContainer = $bindable(null),
	}: Props = $props();
	/* eslint-enable prefer-const */

	const getAddBarCount = (file: RepoDiffFileSummary): number =>
		Math.min(5, file.added > 0 ? Math.max(1, Math.ceil((file.added / (file.added + file.removed || 1)) * 5)) : 0);

	const getDelBarCount = (file: RepoDiffFileSummary): number =>
		Math.min(
			5,
			file.removed > 0 ? Math.max(1, Math.ceil((file.removed / (file.added + file.removed || 1)) * 5)) : 0,
		);
</script>

<div class="cd-header">
	<div class="cd-left">
		<GitBranch size={14} class="cd-icon" />
		<div class="cd-info">
			<div class="cd-title-row">
				<span class="cd-repo">{selectedItem.repoName}</span>
				<span class="cd-dot">·</span>
				<span class="cd-thread">{workspaceName}</span>
				<span class="cd-arrow">→</span>
				<span class="cd-branch">{selectedItem.branch}</span>
			</div>
			<div class="cd-meta">
				<span
					>{filesForDetail.length || selectedItem.dirtyFiles}
					{(filesForDetail.length || selectedItem.dirtyFiles) === 1 ? 'file' : 'files'} changed</span
				>
				<span class="cd-dot">·</span>
				{#if totalAdd > 0}<span class="cd-add">+{totalAdd}</span>{/if}
				{#if totalDel > 0}<span class="cd-del">-{totalDel}</span>{/if}
				{#if selectedItem.ahead > 0}
					<span class="cd-dot">·</span>
					<span>{selectedItem.ahead} commit{selectedItem.ahead !== 1 ? 's' : ''} ahead</span>
				{/if}
			</div>
		</div>
	</div>
	<div class="cd-actions">
		{#if selectedItem.ahead > 0 || selectedItem.hasLocalDiff}
			<button
				type="button"
				class="cd-push-btn"
				disabled={commitPushLoading}
				onclick={() => void onPushFromSidebar(selectedItem.id)}
			>
				{#if commitPushLoading && commitPushRepoId === selectedItem.repoId}
					<Loader2 size={12} class="spin" />
					Pushing...
				{:else}
					<Upload size={12} />
					Push {selectedItem.ahead > 0 ? `${selectedItem.ahead}↑` : ''}
				{/if}
			</button>
		{/if}
	</div>
</div>

<div class="cd-body">
	<div class="cd-file-sidebar">
		<div class="cd-file-head">Changed Files</div>
		<div class="cd-file-list">
			{#if diffSummaryLoading}
				<div class="cd-file-loading">Loading files...</div>
			{:else if filesForDetail.length > 0}
				{#each filesForDetail as file, i (file.path)}
					{@const fname = file.path.split('/').pop() ?? file.path}
					{@const dir = file.path.substring(0, file.path.lastIndexOf('/'))}
					<button
						type="button"
						class="cd-file-card"
						class:active={selectedSource === 'local' ? false : i === selectedFileIdx}
						onclick={() => onSelectSourceFile('pr', i)}
					>
						<div class="cd-file-top">
							<FileCode size={11} class="cd-file-icon" />
							<span class="cd-file-name">{fname}</span>
						</div>
						{#if dir}
							<div class="cd-file-dir">{dir}</div>
						{/if}
						<div class="cd-file-stats">
							<div class="cd-diff-bars">
								{#each Array.from({ length: getAddBarCount(file) }) as _, addBarIndex (addBarIndex)}
									<div class="cd-bar cd-bar-add"></div>
								{/each}
								{#each Array.from({ length: getDelBarCount(file) }) as _, delBarIndex (delBarIndex)}
									<div class="cd-bar cd-bar-del"></div>
								{/each}
							</div>
							<span class="cd-file-add">+{file.added}</span>
							{#if file.removed > 0}
								<span class="cd-file-del">-{file.removed}</span>
							{/if}
						</div>
					</button>
				{/each}
			{:else if fallbackFiles.length > 0}
				{#each fallbackFiles as file, i (file.path)}
					<button
						type="button"
						class="cd-file-card"
						class:active={i === selectedFileIdx}
						onclick={() => onSelectSourceFile('pr', i)}
					>
						<div class="cd-file-top">
							<FileCode size={11} class="cd-file-icon" />
							<span class="cd-file-name">{file.path.split('/').pop() ?? file.path}</span>
						</div>
						<div class="cd-file-stats">
							<span class="cd-file-add">+{file.added}</span>
							{#if file.removed > 0}
								<span class="cd-file-del">-{file.removed}</span>
							{/if}
						</div>
					</button>
				{/each}
			{:else}
				<div class="cd-file-loading">No files detected</div>
			{/if}
		</div>
	</div>

	<div class="fp-diff">
		{#if filesForDetail[selectedFileIdx]}
			{@const activeFile = filesForDetail[selectedFileIdx]}
			<div class="diff-card">
				<div class="diff-header">
					<span>{activeFile.path}</span>
					<span>
						{#if activeFile.added > 0}<span class="text-green">+{activeFile.added}</span>{/if}
						{#if activeFile.removed > 0}<span class="text-red">-{activeFile.removed}</span>{/if}
					</span>
				</div>
				<div class="diff-body">
					{#if fileDiffError}
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
						<div class="diff-renderer-wrap">
							<div class="diff-renderer">
								<diffs-container bind:this={diffContainer}></diffs-container>
							</div>
							{#if fileDiffLoading}
								<div class="diff-loading-overlay">
									<Loader2 size={18} class="spin" />
									<p>Refreshing diff...</p>
								</div>
							{/if}
						</div>
						{#if fileDiffContent.truncated}
							<div class="diff-truncated">
								Diff truncated ({fileDiffContent.totalLines} total lines)
							</div>
						{/if}
					{:else if fileDiffLoading}
						<div class="diff-placeholder">
							<Loader2 size={20} class="spin" />
							<p>Loading diff...</p>
						</div>
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

<style src="./PROrchestrationReadyDetail.css"></style>
