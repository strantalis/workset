<script lang="ts">
	import { ArrowRight } from '@lucide/svelte';
	import type { ExistingRepoContext } from '../../services/workspaceActionContextService';

	interface Props {
		loading: boolean;
		worksetName: string;
		existingRepos: ExistingRepoContext[];
		addRepoTotalItems: number;
		hasPendingSource?: boolean;
		onSubmit: () => void;
	}

	const {
		loading,
		worksetName,
		existingRepos,
		addRepoTotalItems,
		hasPendingSource = false,
		onSubmit,
	}: Props = $props();

	const hasNewSelections = $derived(addRepoTotalItems > 0);
	const canContinue = $derived(hasNewSelections || hasPendingSource);
	const existingCount = $derived(existingRepos.length);
</script>

<div class="selection-panel">
	<div class="summary-strip">
		<span>{existingCount} already in workset</span>
		<span>{addRepoTotalItems} queued to add</span>
	</div>

	<div class="selection-hint">
		{#if hasNewSelections}
			Ready to add {addRepoTotalItems} repo{addRepoTotalItems === 1 ? '' : 's'} to {worksetName}.
		{:else if hasPendingSource}
			Press Continue to add the typed repository source.
		{:else}
			Select one or more repositories to continue.
		{/if}
	</div>

	<button type="button" class="continue-btn" onclick={onSubmit} disabled={loading || !canContinue}>
		{#if loading}
			Adding…
		{:else}
			Continue <ArrowRight size={16} />
		{/if}
	</button>
</div>

<style>
	.selection-panel {
		display: flex;
		flex-direction: column;
		gap: 10px;
		min-height: 0;
	}

	.summary-strip {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 10px 12px;
		border-radius: 10px;
		border: 1px solid color-mix(in srgb, var(--border) 82%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 72%, transparent);
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.selection-hint {
		font-size: var(--text-sm);
		color: var(--muted);
		border: 1px solid color-mix(in srgb, var(--border) 88%, transparent);
		border-radius: 10px;
		padding: 10px 12px;
		background: color-mix(in srgb, var(--panel-strong) 68%, transparent);
	}

	.continue-btn {
		margin-top: auto;
		width: 100%;
		padding: 12px;
		border: none;
		border-radius: 8px;
		font-size: var(--text-md);
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		background: var(--accent);
		color: white;
		box-shadow: 0 0 20px color-mix(in srgb, var(--accent) 20%, transparent);
		transition:
			background 0.15s,
			box-shadow 0.15s,
			opacity 0.15s;
	}

	.continue-btn:hover:not(:disabled) {
		background: color-mix(in srgb, var(--accent) 88%, white);
		box-shadow: 0 0 30px color-mix(in srgb, var(--accent) 40%, transparent);
	}

	.continue-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	@media (max-width: 1120px) {
		.summary-strip {
			flex-direction: column;
			align-items: flex-start;
		}
	}
</style>
