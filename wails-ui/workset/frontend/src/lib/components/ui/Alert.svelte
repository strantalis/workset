<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant: 'error' | 'success' | 'warning' | 'info';
		children: Snippet;
	}

	const { variant, children }: Props = $props();
</script>

<div class="alert {variant}">
	{@render children()}
</div>

<style>
	.alert {
		font-size: 13px;
	}

	.alert.error {
		color: var(--danger);
	}

	.alert.success {
		color: var(--success);
	}

	.alert.warning {
		color: var(--warning);
	}

	.alert.info {
		color: var(--accent);
	}
</style>
