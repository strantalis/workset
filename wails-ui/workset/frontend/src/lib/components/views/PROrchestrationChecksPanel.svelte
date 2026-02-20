<script lang="ts">
	import {
		CheckCircle2,
		ExternalLink,
		Loader2,
		RefreshCw,
		Terminal,
		XCircle,
	} from '@lucide/svelte';
	import type { PullRequestStatusResult } from '../../types';
	import { formatCheckDuration, getCheckIcon } from './prOrchestrationHelpers';

	interface Props {
		prStatusLoading: boolean;
		prStatus: PullRequestStatusResult | null;
		onRefreshChecks: () => void;
		onOpenExternalUrl: (url: string | undefined | null) => void;
	}

	const { prStatusLoading, prStatus, onRefreshChecks, onOpenExternalUrl }: Props = $props();
</script>

<div class="checks-panel">
	<div class="checks-max">
		{#if prStatusLoading}
			<div class="panel-loading">
				<Loader2 size={20} class="spin" />
				<span>Loading checks...</span>
			</div>
		{:else if !prStatus || prStatus.checks.length === 0}
			<div class="panel-loading">
				<CheckCircle2 size={32} />
				<span>No checks available</span>
				<button type="button" class="ghost-btn" onclick={onRefreshChecks}>
					<RefreshCw size={12} /> Refresh checks
				</button>
			</div>
		{:else}
			<div class="checks-header-row">
				<h2>Checks</h2>
				<button type="button" class="ghost-btn" onclick={onRefreshChecks}>
					<RefreshCw size={12} /> Refresh checks
				</button>
			</div>
			<div class="checks-list">
				{#each prStatus.checks as check (check.name)}
					{@const iconType = getCheckIcon(check)}
					<div class="ck-row">
						<div class="ck-circle {iconType}">
							{#if iconType === 'success'}<CheckCircle2 size={16} />
							{:else if iconType === 'failure'}<XCircle size={16} />
							{:else}<Loader2 size={16} class="spin" />
							{/if}
						</div>
						<div class="ck-info">
							<div class="ck-name-row">
								<h3>{check.name}</h3>
							</div>
							<p class="ck-dur">
								{#if check.conclusion === 'success'}Completed in {formatCheckDuration(check)}
								{:else if check.status === 'in_progress'}Running for {formatCheckDuration(check)}...
								{:else}Pending
								{/if}
							</p>
						</div>
						<div class="ck-actions">
							<button type="button" class="ck-action" title="View Logs">
								<Terminal size={14} />
							</button>
							{#if check.detailsUrl}
								<button
									type="button"
									class="ck-action"
									title="View on Provider"
									onclick={() => onOpenExternalUrl(check.detailsUrl)}
								>
									<ExternalLink size={14} />
								</button>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>

<style src="./PROrchestrationChecksPanel.css"></style>
