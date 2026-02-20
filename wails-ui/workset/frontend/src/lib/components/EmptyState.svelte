<script lang="ts">
	import Button from './ui/Button.svelte';

	interface Props {
		title: string;
		body: string;
		actionLabel?: string;
		onAction?: () => void;
		hint?: string;
		variant?: 'default' | 'centered';
	}

	const {
		title,
		body,
		actionLabel = 'Create workspace',
		onAction,
		hint,
		variant = 'default',
	}: Props = $props();
</script>

<section
	class="empty"
	class:centered={variant === 'centered'}
	class:ws-empty-state={variant === 'centered'}
>
	<div class="content">
		<div class="title">{title}</div>
		<div class="body ws-empty-state-copy">{body}</div>
		{#if onAction}
			<div class="actions">
				<Button variant="primary" onclick={onAction}>{actionLabel}</Button>
			</div>
		{/if}
		{#if hint}
			<div class="hint">{hint}</div>
		{/if}
	</div>
</section>

<style>
	.empty {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		align-items: flex-start;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-lg);
		padding: 32px;
		width: 100%;
	}

	.empty.centered {
		background: transparent;
		border: none;
		max-width: 480px;
		margin: auto;
		height: 100%;
		min-height: 300px;
	}

	.content {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		align-items: inherit;
	}

	.centered .content {
		align-items: center;
	}

	.title {
		font-size: var(--text-2xl);
		font-weight: 600;
	}

	.centered .title {
		font-size: var(--text-3xl);
	}

	.body {
		line-height: 1.6;
	}

	.centered .body {
		max-width: 420px;
	}

	.actions {
		display: flex;
		gap: var(--space-3);
		margin-top: var(--space-2);
	}

	.centered .actions {
		margin-top: var(--space-4);
	}

	.hint {
		color: var(--muted);
		font-size: var(--text-sm);
		opacity: 0.7;
		margin-top: var(--space-2);
	}

	.centered .hint {
		margin-top: var(--space-4);
	}
</style>
