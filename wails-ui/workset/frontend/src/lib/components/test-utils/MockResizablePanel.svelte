<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		collapsedSecond?: boolean;
		children: Snippet;
		second: Snippet;
	}

	const { collapsedSecond = false, children, second }: Props = $props();
</script>

<div data-testid="mock-resizable-panel">
	<div data-testid="mock-resizable-panel-first">
		{@render children()}
	</div>
	{#if !collapsedSecond}
		<div data-testid="mock-resizable-panel-second">
			{@render second()}
		</div>
	{/if}
</div>
