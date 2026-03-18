<script lang="ts">
	import type { Snippet } from 'svelte';
	import Button from './ui/Button.svelte';

	interface Props {
		title: string;
		subtitle?: string;
		size?: 'sm' | 'md' | 'lg' | 'xl' | 'wide' | 'full';
		headerAlign?: 'center' | 'left';
		fill?: boolean;
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
		fill = false,
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

<div class="modal" class:fill style="--modal-width: {sizeMap[size]}">
	<header class="modal-header" class:left={headerAlign === 'left'} class:fill>
		<div class="modal-header-text">
			<h2 class="modal-title">{title}</h2>
			{#if subtitle && !fill}
				<div class="modal-subtitle">{subtitle}</div>
			{/if}
		</div>
		{#if onClose}
			{#if fill}
				<button
					class="panel-close-btn"
					onclick={onClose}
					disabled={disableClose}
					aria-label="Close panel">×</button
				>
			{:else}
				<Button variant="ghost" size="sm" onclick={onClose} disabled={disableClose}>Close</Button>
			{/if}
		{/if}
	</header>
	<div class="modal-body" class:fill>
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
		border-radius: 12px;
		border: 1px solid var(--border-strong);
		background: var(--surface-solid);
		box-shadow:
			0 16px 48px rgba(0, 0, 0, 0.45),
			0 0 0 1px color-mix(in srgb, var(--border) 32%, transparent);
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.modal.fill {
		width: 100%;
		height: 100%;
		border: none;
		border-radius: 0;
		background: transparent;
		box-shadow: none;
		padding: 0;
		gap: 0;
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

	.modal-header.fill {
		padding: 14px 18px;
		border-bottom: 1px solid var(--border);
		background: color-mix(in srgb, var(--panel-strong) 80%, var(--panel));
		flex-shrink: 0;
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

	.modal-header.fill .modal-title {
		font-size: var(--text-base);
		font-weight: 600;
		letter-spacing: -0.01em;
	}

	.modal-subtitle {
		font-size: var(--text-sm);
		color: var(--muted);
		margin-top: 4px;
	}

	.panel-close-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 6px;
		border: none;
		background: transparent;
		color: var(--subtle);
		font-size: var(--text-xl);
		line-height: 1;
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.panel-close-btn:hover {
		background: var(--panel-strong);
		color: var(--text);
	}

	.panel-close-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
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

	.modal-body.fill {
		border: none;
		border-radius: 0;
		background: transparent;
		padding: 16px 18px;
		flex: 1;
		min-height: 0;
		overflow-y: auto;
	}

	.modal-body.fill::-webkit-scrollbar {
		width: 6px;
	}

	.modal-body.fill::-webkit-scrollbar-track {
		background: transparent;
	}

	.modal-body.fill::-webkit-scrollbar-thumb {
		background: rgba(255, 255, 255, 0.12);
		border-radius: 3px;
	}

	.modal-body.fill::-webkit-scrollbar-thumb:hover {
		background: rgba(255, 255, 255, 0.22);
	}

	.modal-footer {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		margin-top: 4px;
	}
</style>
