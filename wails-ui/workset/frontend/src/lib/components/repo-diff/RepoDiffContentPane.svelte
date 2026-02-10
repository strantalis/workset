<script lang="ts">
	import type {
		PullRequestCheck,
		PullRequestStatusResult,
		RepoDiffFileSummary,
		RepoDiffSummary,
		RepoFileDiff,
	} from '../../types';
	import type { FilteredAnnotationsResult, CheckStats } from './checkSidebarController';
	import type { SummarySource } from './fileDiffController';
	import RepoDiffFileListSidebar from './RepoDiffFileListSidebar.svelte';

	interface Props {
		summaryLoading: boolean;
		summaryError: string | null;
		summary: RepoDiffSummary | null;
		localSummary: RepoDiffSummary | null;
		selected: RepoDiffFileSummary | null;
		selectedSource: SummarySource;
		shouldSplitLocalPendingSection: boolean;
		effectiveMode: 'create' | 'status';
		prStatus: PullRequestStatusResult | null;
		checkStats: CheckStats;
		expandedCheck: string | null;
		checkAnnotationsLoading: Record<string, boolean>;
		formatDuration: (milliseconds: number) => string;
		getCheckStatusClass: (conclusion: string | undefined, status: string) => string;
		toggleCheckExpansion: (check: PullRequestCheck) => void;
		navigateToAnnotationFile: (path: string, line: number) => void;
		getFilteredAnnotations: (checkName: string) => FilteredAnnotationsResult;
		reviewCountForFile: (path: string) => number;
		selectFile: (file: RepoDiffFileSummary, source?: SummarySource) => void;
		onOpenDetailsUrl: (url: string) => void;
		sidebarWidth: number;
		isResizing: boolean;
		onStartResize: (event: MouseEvent) => void;
		fileMeta: RepoFileDiff | null;
		fileLoading: boolean;
		rendererLoading: boolean;
		fileError: string | null;
		rendererError: string | null;
		diffContainer?: HTMLElement | null;
		onRetrySummary: () => void | Promise<void>;
	}

	/* eslint-disable prefer-const */
	// `diffContainer` is bindable, so this needs a `let` `$props()` declaration.
	let {
		summaryLoading,
		summaryError,
		summary,
		localSummary,
		selected,
		selectedSource,
		shouldSplitLocalPendingSection,
		effectiveMode,
		prStatus,
		checkStats,
		expandedCheck,
		checkAnnotationsLoading,
		formatDuration,
		getCheckStatusClass,
		toggleCheckExpansion,
		navigateToAnnotationFile,
		getFilteredAnnotations,
		reviewCountForFile,
		selectFile,
		onOpenDetailsUrl,
		sidebarWidth,
		isResizing,
		onStartResize,
		fileMeta,
		fileLoading,
		rendererLoading,
		fileError,
		rendererError,
		diffContainer = $bindable(null),
		onRetrySummary,
	}: Props = $props();
	/* eslint-enable prefer-const */
</script>

{#if summaryLoading}
	<div class="state">Loading diff summary...</div>
{:else if summaryError}
	<div class="state error">
		<div class="message">{summaryError}</div>
		<button class="ghost" type="button" onclick={onRetrySummary}>Retry</button>
	</div>
{:else if (!summary || summary.files.length === 0) && (!localSummary || localSummary.files.length === 0)}
	<div class="state">No changes detected in this repo.</div>
{:else}
	<div class="diff-body" style="--sidebar-width: {sidebarWidth}px">
		<RepoDiffFileListSidebar
			{summary}
			{localSummary}
			{selected}
			{selectedSource}
			{shouldSplitLocalPendingSection}
			{effectiveMode}
			{prStatus}
			{checkStats}
			{expandedCheck}
			{checkAnnotationsLoading}
			{formatDuration}
			{getCheckStatusClass}
			{toggleCheckExpansion}
			{navigateToAnnotationFile}
			{getFilteredAnnotations}
			{reviewCountForFile}
			{selectFile}
			{onOpenDetailsUrl}
		/>
		<button
			class="resize-handle"
			class:resizing={isResizing}
			onmousedown={onStartResize}
			aria-label="Resize sidebar"
			type="button"
		></button>
		<div class="diff-view">
			<div class="file-header">
				<div class="file-title">
					<span>{selected?.path}</span>
					{#if selected?.prevPath}
						<span class="rename">from {selected.prevPath}</span>
					{/if}
				</div>
				<span class="diffstat">
					<span class="add">+{selected?.added ?? 0}</span><span class="sep">/</span><span
						class="del">-{selected?.removed ?? 0}</span
					>
					{#if fileMeta && !fileMeta.truncated && fileMeta.totalLines > 0}
						<span class="line-count">{fileMeta.totalLines} lines</span>
					{/if}
				</span>
			</div>
			{#if fileLoading || rendererLoading}
				<div class="state compact">Loading file diff...</div>
			{:else if fileError}
				<div class="state compact">{fileError}</div>
			{:else if rendererError}
				<div class="state compact">{rendererError}</div>
			{:else}
				<div class="diff-renderer">
					<diffs-container bind:this={diffContainer}></diffs-container>
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
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

	.line-count {
		font-size: var(--text-xs);
		color: var(--muted);
		font-weight: 500;
	}

	.state {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 20px;
		color: var(--muted);
	}

	.state.compact {
		padding: 16px;
		border-radius: 12px;
		background: var(--panel-soft);
		border: 1px dashed var(--border);
		text-align: center;
	}

	.state.error {
		color: var(--warning);
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.diff-body {
		display: grid;
		grid-template-columns: var(--sidebar-width, 280px) 1fr;
		gap: 8px;
		flex: 1;
		min-height: 0;
		position: relative;
	}

	.resize-handle {
		position: absolute;
		left: calc(var(--sidebar-width, 280px) + 2px);
		top: 0;
		bottom: 0;
		width: 4px;
		background: transparent;
		border: none;
		padding: 0;
		cursor: col-resize;
		transition: background var(--transition-fast);
		z-index: 10;
		border-radius: 2px;
	}

	.resize-handle:hover,
	.resize-handle.resizing {
		background: var(--accent);
	}

	.resize-handle::after {
		content: '';
		position: absolute;
		inset: 0;
		width: 12px;
		transform: translateX(-4px);
	}

	.diff-view {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		overflow: hidden;
	}

	.file-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-size: var(--text-base);
		color: var(--muted);
		height: 24px;
	}

	.file-title {
		display: flex;
		gap: 8px;
		align-items: center;
		color: var(--text);
		font-weight: 500;
	}

	.rename {
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.diff-renderer {
		flex: 1;
		min-height: 0;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: var(--panel-soft);
		padding: 8px;
		overflow: hidden;
		--diffs-dark-bg: var(--panel-soft);
		--diffs-dark: var(--text);
		--diffs-dark-addition-color: var(--success);
		--diffs-dark-deletion-color: var(--danger);
		--diffs-dark-modified-color: var(--accent);
		--diffs-font-family: var(--font-mono);
		--diffs-font-size: var(--text-mono-sm);
		--diffs-header-font-family: var(--font-body);
		--diffs-gap-block: 8px;
		--diffs-gap-inline: 10px;
	}

	diffs-container {
		display: block;
		height: 100%;
		width: 100%;
		overflow: auto;
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

	.ghost:active:not(:disabled) {
		transform: scale(0.98);
	}
</style>
