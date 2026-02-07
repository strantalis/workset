<script lang="ts">
	import { CheckCircle2, Loader2, XCircle } from '@lucide/svelte';
	import { formatPath } from '../../pathUtils';
	import type {
		PullRequestCheck,
		PullRequestStatusResult,
		RepoDiffFileSummary,
		RepoDiffSummary,
	} from '../../types';
	import type { CheckStats, FilteredAnnotationsResult } from './checkSidebarController';
	import type { SummarySource } from './fileDiffController';
	import RepoDiffChecksSidebar from './RepoDiffChecksSidebar.svelte';

	interface Props {
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
		formatDuration: (ms: number) => string;
		getCheckStatusClass: (conclusion: string | undefined, status: string) => string;
		toggleCheckExpansion: (check: PullRequestCheck) => void;
		navigateToAnnotationFile: (path: string, line: number) => void;
		getFilteredAnnotations: (checkName: string) => FilteredAnnotationsResult;
		reviewCountForFile: (path: string) => number;
		selectFile: (file: RepoDiffFileSummary, source?: SummarySource) => void;
		onOpenDetailsUrl: (url: string) => void;
	}

	const {
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
	}: Props = $props();

	let sidebarTab: 'files' | 'checks' = $state('files');

	const statusLabel = (status: string): string => {
		switch (status) {
			case 'added':
				return 'added';
			case 'deleted':
				return 'deleted';
			case 'renamed':
				return 'renamed';
			case 'untracked':
				return 'untracked';
			case 'binary':
				return 'binary';
			default:
				return 'modified';
		}
	};
</script>

<aside class="file-list">
	{#if effectiveMode === 'status' && prStatus && prStatus.checks.length > 0}
		<div class="sidebar-tabs">
			<button
				class="sidebar-tab"
				class:active={sidebarTab === 'files'}
				type="button"
				onclick={() => (sidebarTab = 'files')}
			>
				Files
				<span class="tab-count"
					>{(summary?.files.length ?? 0) +
						(shouldSplitLocalPendingSection ? (localSummary?.files.length ?? 0) : 0)}</span
				>
			</button>
			<button
				class="sidebar-tab"
				class:active={sidebarTab === 'checks'}
				type="button"
				onclick={() => (sidebarTab = 'checks')}
			>
				Checks
				{#if checkStats.failed > 0}
					<span class="tab-count failed"><XCircle size={12} /> {checkStats.failed}</span>
				{:else if checkStats.pending > 0}
					<span class="tab-count pending"
						><Loader2 size={12} class="spin" /> {checkStats.pending}</span
					>
				{:else}
					<span class="tab-count passed"><CheckCircle2 size={12} /> {checkStats.passed}</span>
				{/if}
			</button>
		</div>
	{/if}

	{#if sidebarTab === 'files'}
		{#if summary && summary.files.length > 0}
			<div class="section-title">
				{shouldSplitLocalPendingSection && localSummary && localSummary.files.length > 0
					? 'PR files'
					: 'Changed files'}
			</div>
			{#each summary.files as file (file.path)}
				{@const reviewCount = reviewCountForFile(file.path)}
				<button
					class:selected={file.path === selected?.path &&
						file.prevPath === selected?.prevPath &&
						selectedSource === 'pr'}
					class="file-row"
					onclick={() => selectFile(file, 'pr')}
					type="button"
				>
					<div class="file-meta">
						<span class="path" title={file.path}>{formatPath(file.path)}</span>
						{#if file.prevPath}
							<span class="rename">from {file.prevPath}</span>
						{/if}
					</div>
					<div class="stats">
						{#if reviewCount > 0}
							<span
								class="review-badge"
								title="{reviewCount} review comment{reviewCount > 1 ? 's' : ''}"
							>
								💬 {reviewCount}
							</span>
						{/if}
						<span class="tag {file.status}">{statusLabel(file.status)}</span>
						<span class="diffstat"
							><span class="add">+{file.added}</span><span class="sep">/</span><span class="del"
								>-{file.removed}</span
							></span
						>
					</div>
				</button>
			{/each}
		{/if}

		{#if shouldSplitLocalPendingSection && localSummary && localSummary.files.length > 0}
			<div class="section-title local-section-title">Local pending changes</div>
			{#each localSummary.files as file (`local:${file.path}:${file.prevPath ?? ''}`)}
				<button
					class:selected={file.path === selected?.path &&
						file.prevPath === selected?.prevPath &&
						selectedSource === 'local'}
					class="file-row local-file"
					onclick={() => selectFile(file, 'local')}
					type="button"
				>
					<div class="file-meta">
						<span class="path" title={file.path}>{formatPath(file.path)}</span>
						{#if file.prevPath}
							<span class="rename">from {file.prevPath}</span>
						{/if}
					</div>
					<div class="stats">
						<span class="tag {file.status} local-tag">{statusLabel(file.status)}</span>
						<span class="diffstat local-diffstat"
							><span class="add">+{file.added}</span><span class="sep">/</span><span class="del"
								>-{file.removed}</span
							></span
						>
					</div>
				</button>
			{/each}
		{/if}
	{/if}

	{#if sidebarTab === 'checks' && prStatus}
		<RepoDiffChecksSidebar
			{prStatus}
			{checkStats}
			{expandedCheck}
			{checkAnnotationsLoading}
			{formatDuration}
			{getCheckStatusClass}
			{toggleCheckExpansion}
			{navigateToAnnotationFile}
			{getFilteredAnnotations}
			{onOpenDetailsUrl}
		/>
	{/if}
</aside>

<style>
	.file-list {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		overflow: auto;
		scrollbar-width: thin;
		scrollbar-color: var(--border) transparent;
	}

	.file-list::-webkit-scrollbar {
		width: 6px;
	}

	.file-list::-webkit-scrollbar-track {
		background: transparent;
	}

	.file-list::-webkit-scrollbar-thumb {
		background: var(--border);
		border-radius: 3px;
	}

	.file-list::-webkit-scrollbar-thumb:hover {
		background: var(--accent);
	}

	.sidebar-tabs {
		display: flex;
		gap: 0;
		margin-bottom: 16px;
		border-bottom: 1px solid var(--border);
	}

	.sidebar-tab {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 10px 12px;
		border: none;
		border-bottom: 2px solid transparent;
		margin-bottom: -1px;
		background: transparent;
		color: var(--muted);
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.sidebar-tab:hover:not(.active) {
		color: var(--text);
	}

	.sidebar-tab.active {
		color: var(--text);
		border-bottom-color: var(--accent);
	}

	.tab-count {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 6px;
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.08);
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.tab-count.passed {
		color: #3fb950;
		background: rgba(46, 160, 67, 0.15);
	}

	.tab-count.failed {
		color: #f85149;
		background: rgba(248, 81, 73, 0.15);
	}

	.tab-count.pending {
		color: #d29922;
		background: rgba(210, 153, 34, 0.15);
	}

	.tab-count :global(.spin) {
		animation: spin 1s linear infinite;
	}

	.section-title {
		font-size: 11px;
		font-weight: 500;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		height: 24px;
		display: flex;
		align-items: center;
	}

	.local-section-title {
		color: #d29922 !important;
	}

	.file-row {
		display: flex;
		flex-direction: column;
		gap: 6px;
		background: transparent;
		outline: 1px solid transparent;
		outline-offset: -1px;
		color: var(--text);
		text-align: left;
		padding: 10px;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition:
			outline-color var(--transition-fast),
			background var(--transition-fast);
	}

	.file-row:hover:not(.selected) {
		outline-color: var(--border);
		background: rgba(255, 255, 255, 0.02);
	}

	.file-row.selected {
		background: var(--accent-subtle);
		outline-color: var(--accent);
	}

	.file-row.local-file {
		border-left: 2px solid rgba(210, 153, 34, 0.5);
		background: rgba(210, 153, 34, 0.06);
	}

	.file-row.local-file:hover:not(.selected) {
		border-color: rgba(210, 153, 34, 0.4);
		background: rgba(210, 153, 34, 0.12);
		border-left-color: #d29922;
	}

	.file-row.local-file.selected {
		background: rgba(210, 153, 34, 0.18);
		border-color: rgba(210, 153, 34, 0.5);
		border-left-color: #d29922;
	}

	.file-meta {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.path {
		font-size: 13px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.rename {
		font-size: 11px;
		color: var(--muted);
	}

	.stats {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--muted);
	}

	.review-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 6px;
		border-radius: 999px;
		background: linear-gradient(135deg, rgba(99, 102, 241, 0.15) 0%, rgba(139, 92, 246, 0.15) 100%);
		border: 1px solid rgba(99, 102, 241, 0.3);
		font-size: 10px;
		font-weight: 600;
		color: #a78bfa;
	}

	.tag {
		text-transform: uppercase;
		letter-spacing: 0.08em;
		font-size: 10px;
		font-weight: 600;
	}

	.tag.added {
		color: var(--success);
	}

	.tag.deleted {
		color: var(--danger);
	}

	.tag.renamed {
		color: var(--accent);
	}

	.tag.untracked {
		color: var(--warning);
	}

	.tag.binary {
		color: var(--muted);
	}

	.local-tag {
		color: #d29922 !important;
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

	.local-diffstat .add {
		color: #d29922 !important;
	}

	.local-diffstat .del {
		color: #d29922 !important;
		opacity: 0.7;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
