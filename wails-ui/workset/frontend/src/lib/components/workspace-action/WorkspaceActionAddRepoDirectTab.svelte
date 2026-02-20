<script lang="ts">
	import Button from '../ui/Button.svelte';

	interface Props {
		addSource: string;
		onAddSourceInput: (value: string) => void;
		onBrowse: () => void;
	}

	const { addSource, onAddSourceInput, onBrowse }: Props = $props();
</script>

<label class="field ws-field">
	<span>Repo URL or local path</span>
	<div class="inline ws-inline">
		<input
			class="ws-field-input ws-inline-grow"
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
