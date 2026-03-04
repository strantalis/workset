<script lang="ts">
	import type { Snippet } from 'svelte';
	import Button from './ui/Button.svelte';

	interface Props {
		title: string;
		subtitle?: string;
		size?: 'sm' | 'md' | 'lg' | 'xl' | 'wide' | 'full';
		headerAlign?: 'center' | 'left';
		onClose?: () => void;
		disableClose?: boolean;
		children: Snippet;
		footer?: Snippet;
	}

	const {
		title,
		subtitle = '',
		size = 'md',
		headerAlign = 'center',
		onClose,
		disableClose = false,
		children,
		footer,
	}: Props = $props();

	const sizeMap = {
		sm: '360px',
		md: '420px',
		lg: '500px',
		xl: '480px',
		wide: '780px',
		full: '1120px',
	};
</script>

<div class="modal" style="--modal-width: {sizeMap[size]}">
	<header class="modal-header" class:left={headerAlign === 'left'}>
		<div class="modal-header-text">
			<div class="modal-title">{title}</div>
			{#if subtitle}
				<div class="modal-subtitle">{subtitle}</div>
			{/if}
		</div>
		{#if onClose}
			<Button variant="ghost" size="sm" onclick={onClose} disabled={disableClose}>Close</Button>
		{/if}
	</header>
	<div class="modal-body">
		{@render children()}
	</div>
	{#if footer}
		<div class="modal-footer">
			{@render footer()}
		</div>
	{/if}
</div>

<style>
	.modal {
		width: min(var(--modal-width, 420px), 90%);
		padding: 20px 22px;
		border-radius: 16px;
		border: 1px solid color-mix(in srgb, var(--glass-border) 90%, transparent);
		background:
			linear-gradient(
				180deg,
				rgba(255, 255, 255, 0.08) 0%,
				rgba(255, 255, 255, 0.02) 42%,
				rgba(255, 255, 255, 0) 100%
			),
			var(--glass-bg-strong);
		backdrop-filter: blur(calc(var(--glass-blur) + 1px)) saturate(var(--glass-saturate));
		-webkit-backdrop-filter: blur(calc(var(--glass-blur) + 1px)) saturate(var(--glass-saturate));
		box-shadow:
			var(--glass-shadow),
			var(--inset-highlight),
			0 0 0 1px color-mix(in srgb, var(--border) 32%, transparent);
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.modal-header {
		text-align: center;
	}

	.modal-header.left {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		text-align: left;
	}

	.modal-header-text {
		flex: 1;
		min-width: 0;
	}

	.modal-title {
		font-size: var(--text-xl);
		font-weight: 600;
		color: var(--text);
	}

	.modal-subtitle {
		font-size: var(--text-sm);
		color: var(--muted);
		margin-top: 4px;
	}

	.modal-body {
		display: flex;
		flex-direction: column;
		gap: 12px;
		border-radius: 12px;
		border: 1px solid color-mix(in srgb, var(--border) 65%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 88%, var(--panel));
		padding: 12px;
	}

	.modal-footer {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		margin-top: 4px;
	}
</style>
