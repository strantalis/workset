<script lang="ts">
	import type { Snippet } from 'svelte';
	import { X } from '@lucide/svelte';

	interface Props {
		open: boolean;
		title: string;
		width?: number;
		onClose: () => void;
		children: Snippet;
	}

	const { open, title, width = 380, onClose, children }: Props = $props();

	const handleKeydown = (event: KeyboardEvent): void => {
		if (event.key === 'Escape' && open) {
			event.preventDefault();
			onClose();
		}
	};

	const handleBackdropClick = (): void => {
		onClose();
	};
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="sd-backdrop" onclick={handleBackdropClick}></div>
	<div
		class="sd-drawer"
		style="width: {width}px"
		role="dialog"
		aria-modal="true"
		aria-label={title}
	>
		<div class="sd-header">
			<h2 class="sd-title">{title}</h2>
			<button type="button" class="sd-close" aria-label="Close" onclick={onClose}>
				<X size={14} />
			</button>
		</div>
		<div class="sd-body">
			{@render children()}
		</div>
	</div>
{/if}

<style>
	.sd-backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.35);
		z-index: 90;
		animation: sd-fade-in 120ms ease-out;
	}
	.sd-drawer {
		position: fixed;
		top: 0;
		right: 0;
		bottom: 0;
		z-index: 91;
		display: flex;
		flex-direction: column;
		background: var(--panel);
		border-left: 1px solid var(--border);
		box-shadow: var(--shadow-lg);
		animation: sd-slide-in 160ms ease-out;
	}
	.sd-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 16px 20px;
		border-bottom: 1px solid var(--border);
		flex-shrink: 0;
	}
	.sd-title {
		margin: 0;
		font-size: var(--text-base);
		font-weight: 600;
		color: var(--text);
	}
	.sd-close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border: 1px solid transparent;
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all var(--transition-fast);
	}
	.sd-close:hover {
		color: var(--text);
		background: var(--hover-bg);
		border-color: var(--border);
	}
	.sd-body {
		flex: 1;
		overflow-y: auto;
		padding: 20px;
	}

	@keyframes sd-fade-in {
		from {
			opacity: 0;
		}
	}
	@keyframes sd-slide-in {
		from {
			transform: translateX(100%);
		}
	}
</style>
