<script lang="ts">
	import {
		CheckCircle2,
		XCircle,
		Loader2,
		Ban,
		MinusCircle,
		ChevronDown,
		ChevronRight,
		ExternalLink,
	} from '@lucide/svelte';
	import type { PullRequestCheck, PullRequestStatusResult } from '../../types';
	import type { CheckStats, FilteredAnnotationsResult } from './checkSidebarController';

	interface Props {
		prStatus: PullRequestStatusResult;
		checkStats: CheckStats;
		expandedCheck: string | null;
		checkAnnotationsLoading: Record<string, boolean>;
		formatDuration: (ms: number) => string;
		getCheckStatusClass: (conclusion: string | undefined, status: string) => string;
		toggleCheckExpansion: (check: PullRequestCheck) => void;
		navigateToAnnotationFile: (path: string, line: number) => void;
		getFilteredAnnotations: (checkName: string) => FilteredAnnotationsResult;
		onOpenDetailsUrl: (url: string) => void;
	}

	const {
		prStatus,
		checkStats,
		expandedCheck,
		checkAnnotationsLoading,
		formatDuration,
		getCheckStatusClass,
		toggleCheckExpansion,
		navigateToAnnotationFile,
		getFilteredAnnotations,
		onOpenDetailsUrl,
	}: Props = $props();
</script>

<div class="checks-tab-content">
	<div class="checks-summary">
		<div class="checks-summary-item passed">
			<CheckCircle2 size={16} />
			<span>{checkStats.passed}</span>
		</div>
		<div class="checks-summary-item failed">
			<XCircle size={16} />
			<span>{checkStats.failed}</span>
		</div>
		{#if checkStats.pending > 0}
			<div class="checks-summary-item pending">
				<Loader2 size={16} class="spin" />
				<span>{checkStats.pending}</span>
			</div>
		{/if}
	</div>

	<div class="checks-list">
		{#each prStatus.checks as check (check.name)}
			{@const statusClass = getCheckStatusClass(check.conclusion, check.status)}
			{@const isFailed = check.conclusion === 'failure'}
			{@const isExpanded = expandedCheck === check.name}
			{@const filteredResult = getFilteredAnnotations(check.name)}
			{@const hasAnnotations = filteredResult.annotations.length > 0}
			{@const isLoadingAnnotations = checkAnnotationsLoading[check.name]}
			<div class="check-item-container">
				{#if isFailed}
					<button
						class="check-row {statusClass} expandable"
						type="button"
						onclick={() => toggleCheckExpansion(check)}
					>
						<span class="check-indicator {statusClass}">
							{#if check.conclusion === 'success'}
								<CheckCircle2 size={16} />
							{:else if check.conclusion === 'failure'}
								<XCircle size={16} />
							{:else if check.conclusion === 'skipped'}
								<Ban size={16} />
							{:else if check.conclusion === 'cancelled'}
								<Ban size={16} />
							{:else if check.conclusion === 'neutral'}
								<MinusCircle size={16} />
							{:else if check.status === 'in_progress' || check.status === 'queued'}
								<Loader2 size={16} class="spin" />
							{:else}
								<MinusCircle size={16} />
							{/if}
						</span>
						<span class="check-name">{check.name}</span>
						{#if check.startedAt && check.completedAt}
							{@const duration =
								new Date(check.completedAt).getTime() - new Date(check.startedAt).getTime()}
							<span class="check-duration" title="Duration">
								{formatDuration(duration)}
							</span>
						{/if}
						<span class="check-expand-icon">
							{#if isExpanded}
								<ChevronDown size={16} />
							{:else}
								<ChevronRight size={16} />
							{/if}
						</span>
						{#if check.detailsUrl}
							<a
								class="check-link"
								href={check.detailsUrl}
								target="_blank"
								rel="noopener noreferrer"
								onclick={(event) => {
									event.stopPropagation();
									if (check.detailsUrl) onOpenDetailsUrl(check.detailsUrl);
								}}
								title="View on GitHub"
							>
								<ExternalLink size={14} />
							</a>
						{/if}
					</button>
				{:else}
					<div class="check-row {statusClass}">
						<span class="check-indicator {statusClass}">
							{#if check.conclusion === 'success'}
								<CheckCircle2 size={16} />
							{:else if check.conclusion === 'failure'}
								<XCircle size={16} />
							{:else if check.conclusion === 'skipped'}
								<Ban size={16} />
							{:else if check.conclusion === 'cancelled'}
								<Ban size={16} />
							{:else if check.conclusion === 'neutral'}
								<MinusCircle size={16} />
							{:else if check.status === 'in_progress' || check.status === 'queued'}
								<Loader2 size={16} class="spin" />
							{:else}
								<MinusCircle size={16} />
							{/if}
						</span>
						<span class="check-name">{check.name}</span>
						{#if check.startedAt && check.completedAt}
							{@const duration =
								new Date(check.completedAt).getTime() - new Date(check.startedAt).getTime()}
							<span class="check-duration" title="Duration">
								{formatDuration(duration)}
							</span>
						{/if}
						{#if check.detailsUrl}
							<a
								class="check-link"
								href={check.detailsUrl}
								target="_blank"
								rel="noopener noreferrer"
								onclick={() => check.detailsUrl && onOpenDetailsUrl(check.detailsUrl)}
								title="View on GitHub"
							>
								<ExternalLink size={14} />
							</a>
						{/if}
					</div>
				{/if}

				{#if isFailed && isExpanded}
					<div class="check-annotations">
						{#if isLoadingAnnotations}
							<div class="check-annotations-loading">
								<Loader2 size={16} class="spin" />
								<span>Loading annotations...</span>
							</div>
						{:else if hasAnnotations}
							{#each filteredResult.annotations as annotation (annotation.path + annotation.startLine)}
								<div class="check-annotation-item level-{annotation.level}">
									<button
										class="check-annotation-path"
										type="button"
										onclick={() => navigateToAnnotationFile(annotation.path, annotation.startLine)}
									>
										<span class="path-text">{annotation.path}:{annotation.startLine}</span>
										{#if annotation.startLine !== annotation.endLine}
											<span class="line-range">-{annotation.endLine}</span>
										{/if}
									</button>
									{#if annotation.title}
										<div class="check-annotation-title">{annotation.title}</div>
									{/if}
									<div class="check-annotation-message">{annotation.message}</div>
								</div>
							{/each}
							{#if filteredResult.filteredCount > 0}
								<div class="check-annotations-more">
									+{filteredResult.filteredCount} more in other files
								</div>
							{/if}
						{:else}
							<div class="check-annotations-empty">
								{#if !check.checkRunId}
									<span>Check run ID not available</span>
								{:else}
									<span>No annotations for this check</span>
								{/if}
							</div>
						{/if}
					</div>
				{/if}
			</div>
		{/each}
	</div>
</div>

<style>
	.checks-tab-content {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.checks-summary {
		display: flex;
		gap: 12px;
		padding: 12px;
		background: rgba(255, 255, 255, 0.03);
		border-radius: 10px;
		border: 1px solid var(--border);
	}

	.checks-summary-item {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-md);
		font-weight: 600;
	}

	.checks-summary-item.passed {
		color: #3fb950;
	}

	.checks-summary-item.failed {
		color: #f85149;
	}

	.checks-summary-item.pending {
		color: #d29922;
	}

	.checks-summary-item .spin {
		animation: spin 1s linear infinite;
	}

	.checks-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.check-item-container {
		display: flex;
		flex-direction: column;
		gap: 0;
	}

	.check-row {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px;
		border-radius: 10px;
		font-size: var(--text-base);
		transition: background 0.15s ease;
		border-left: 3px solid transparent;
		background: transparent;
		border: none;
		width: 100%;
		text-align: left;
		cursor: default;
	}

	.check-row.expandable {
		cursor: pointer;
	}

	.check-row:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.03);
	}

	.check-row.check-success {
		background: rgba(46, 160, 67, 0.08);
		border-left-color: #3fb950;
	}

	.check-row.check-failure {
		background: rgba(248, 81, 73, 0.08);
		border-left-color: #f85149;
	}

	.check-row.check-pending {
		background: rgba(210, 153, 34, 0.08);
		border-left-color: #d29922;
	}

	.check-row.check-neutral {
		background: rgba(139, 148, 158, 0.08);
		border-left-color: #8b949e;
	}

	.check-row .check-indicator {
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	.check-row .check-indicator.check-success {
		color: #3fb950;
	}

	.check-row .check-indicator.check-failure {
		color: #f85149;
	}

	.check-row .check-indicator.check-pending {
		color: #d29922;
	}

	.check-row .check-indicator.check-neutral {
		color: #8b949e;
	}

	.check-row .check-name {
		color: var(--text);
		font-weight: 500;
		flex: 1;
	}

	.check-row .check-duration {
		font-size: var(--text-mono-xs);
		color: var(--muted);
		font-family: var(--font-mono);
		padding: 2px 6px;
		background: rgba(255, 255, 255, 0.05);
		border-radius: 4px;
	}

	.check-row .check-expand-icon {
		color: var(--muted);
		display: flex;
		align-items: center;
	}

	.check-row .check-link {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		opacity: 0;
		transition: all 0.15s ease;
	}

	.check-row:hover .check-link,
	.check-row:focus-within .check-link {
		opacity: 1;
	}

	.check-row .check-link:hover {
		background: rgba(255, 255, 255, 0.1);
		color: var(--text);
	}

	.check-annotations {
		padding: 0 16px 16px 56px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.check-annotations-loading {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 16px;
		color: var(--muted);
		font-size: var(--text-sm);
	}

	.check-annotations-loading .spin {
		animation: spin 1s linear infinite;
	}

	.check-annotations-empty {
		padding: 16px;
		color: var(--muted);
		font-size: var(--text-sm);
		font-style: italic;
	}

	.check-annotations-more {
		padding: 12px 16px;
		color: var(--muted);
		font-size: var(--text-xs);
		font-style: italic;
		text-align: center;
		border-top: 1px solid var(--panel-border, rgba(255, 255, 255, 0.05));
	}

	.check-annotation-item {
		padding: 14px 16px;
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.03);
		border-left: 3px solid transparent;
		transition:
			background 0.15s ease,
			transform 0.1s ease;
	}

	.check-annotation-item:hover {
		background: rgba(255, 255, 255, 0.06);
		transform: translateX(2px);
	}

	.check-annotation-item.level-notice {
		border-left-color: #58a6ff;
		background: rgba(88, 166, 255, 0.08);
	}

	.check-annotation-item.level-warning {
		border-left-color: #d29922;
		background: rgba(210, 153, 34, 0.08);
	}

	.check-annotation-item.level-failure {
		border-left-color: #f85149;
		background: rgba(248, 81, 73, 0.08);
	}

	.check-annotation-path {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-mono-sm);
		font-family: var(--font-mono);
		color: var(--accent);
		background: none;
		border: none;
		padding: 4px 0;
		cursor: pointer;
		text-align: left;
		margin-bottom: 8px;
		font-weight: 500;
	}

	.check-annotation-path:hover {
		color: var(--text);
		text-decoration: underline;
	}

	.check-annotation-path .line-range {
		color: var(--muted);
	}

	.check-annotation-title {
		font-size: var(--text-base);
		font-weight: 600;
		color: var(--text);
		margin-bottom: 6px;
	}

	.check-annotation-message {
		font-size: var(--text-sm);
		color: var(--muted);
		line-height: 1.6;
		white-space: pre-wrap;
	}

	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}
</style>
