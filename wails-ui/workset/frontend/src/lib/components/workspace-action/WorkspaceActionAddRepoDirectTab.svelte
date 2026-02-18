<script lang="ts">
	import Button from '../ui/Button.svelte';

	interface Props {
		addSource: string;
		onAddSourceInput: (value: string) => void;
		onBrowse: () => void;
	}

	const { addSource, onAddSourceInput, onBrowse }: Props = $props();
</script>

<label class="field">
	<span>Repo URL or local path</span>
	<div class="inline">
		<input
			value={addSource}
			oninput={(event) => onAddSourceInput((event.currentTarget as HTMLInputElement).value)}
			placeholder="git@github.com:org/repo.git"
			autocapitalize="off"
			autocorrect="off"
			spellcheck="false"
		/>
		<Button variant="ghost" size="sm" onclick={onBrowse}>Browse</Button>
	</div>
</label>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.field input {
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		color: var(--text);
		padding: 8px 10px;
		font-size: var(--text-md);
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.field input:focus {
		background: rgba(255, 255, 255, 0.04);
	}

	.inline {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.inline input {
		flex: 1;
	}
</style>
