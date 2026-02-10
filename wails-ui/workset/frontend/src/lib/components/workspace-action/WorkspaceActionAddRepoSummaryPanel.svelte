<script lang="ts">
	import { ArrowRight } from '@lucide/svelte';
	import type {
		ExistingRepoContext,
		WorkspaceActionAddRepoSelectedItem,
	} from '../../services/workspaceActionContextService';
	import WorksetTopology from '../WorksetTopology.svelte';

	interface Props {
		loading: boolean;
		worksetName: string;
		existingRepos: ExistingRepoContext[];
		addRepoSelectedItems: WorkspaceActionAddRepoSelectedItem[];
		addRepoTotalItems: number;
		onSubmit: () => void;
	}

	const {
		loading,
		worksetName,
		existingRepos,
		addRepoSelectedItems,
		addRepoTotalItems,
		onSubmit,
	}: Props = $props();

	const topologyRepos = $derived.by(() => {
		// Combine existing repos (dimmed) and selected repos (highlighted)
		const existing = existingRepos.map((repo) => ({
			name: repo.name,
			highlighted: false,
		}));
		const selected = addRepoSelectedItems.map((item) => ({
			name: item.name,
			highlighted: true,
		}));
		return [...existing, ...selected];
	});

	const hasNewSelections = $derived(addRepoTotalItems > 0);
</script>

<div class="selection-panel">
	<WorksetTopology repos={topologyRepos} centerLabel={worksetName} centerDim={!hasNewSelections} />

	<button
		type="button"
		class="continue-btn"
		onclick={onSubmit}
		disabled={loading || !hasNewSelections}
	>
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
		gap: 16px;
		height: 100%;
		max-height: 100%;
		overflow: hidden;
	}

	.continue-btn {
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
</style>
