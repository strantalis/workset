<script lang="ts">
	import type { Snippet } from 'svelte';
	import { clickOutside } from '../../actions/clickOutside';
	import { portal } from '../../actions/portal';
	import { tick } from 'svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
		position?: 'left' | 'right';
		children: Snippet;
		trigger?: HTMLElement | null;
	}

	const { open, onClose, position = 'right', children, trigger = null }: Props = $props();

	let menuElement: HTMLElement | null = $state(null);
	let menuPosition = $state({ top: 0, left: 0 });

	$effect(() => {
		if (open && trigger && menuElement) {
			tick().then(() => {
				updatePosition();
			});
		}
	});

	function updatePosition() {
		if (!trigger || !menuElement) return;

		const triggerRect = trigger.getBoundingClientRect();
		const menuRect = menuElement.getBoundingClientRect();
		const gap = 4; // px gap between trigger and menu

		// Calculate position relative to viewport
		let top = triggerRect.bottom + gap;
		let left = 0;

		// Check if menu would go off bottom of viewport
		const spaceBelow = window.innerHeight - triggerRect.bottom - gap;
		const spaceAbove = triggerRect.top - gap;

		if (menuRect.height > spaceBelow && spaceAbove > spaceBelow) {
			// Flip to open upward if there's more space above
			top = triggerRect.top - menuRect.height - gap;
		}

		if (position === 'right') {
			left = triggerRect.right - menuRect.width;
		} else {
			left = triggerRect.left;
		}

		// Prevent going off screen horizontally
		if (left < 8) left = 8;
		if (left + menuRect.width > window.innerWidth - 8) {
			left = window.innerWidth - menuRect.width - 8;
		}

		// Ensure menu doesn't go off top of viewport
		if (top < 8) top = 8;

		menuPosition = { top, left };
	}
</script>

{#if open}
	<div
		bind:this={menuElement}
		class="dropdown-menu {position}"
		style="top: {menuPosition.top}px; left: {menuPosition.left}px;"
		use:portal
		use:clickOutside={{ callback: onClose, exclude: trigger }}
		role="menu"
	>
		{@render children()}
	</div>
{/if}

<style>
	.dropdown-menu {
		position: fixed;
		background: #141f2e;
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 10px;
		padding: 6px;
		display: grid;
		gap: 4px;
		z-index: 9999;
		min-width: 140px;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
	}

	/* Menu item styles */
	.dropdown-menu :global(button) {
		display: flex;
		align-items: center;
		gap: 8px;
		background: none;
		border: none;
		color: var(--text);
		text-align: left;
		padding: 8px 12px;
		border-radius: var(--radius-sm);
		cursor: pointer;
		font-size: var(--text-base);
		font-family: inherit;
		transition: background var(--transition-fast);
	}

	.dropdown-menu :global(button svg) {
		width: 14px;
		height: 14px;
		stroke: currentColor;
		stroke-width: 1.6;
		fill: none;
		flex-shrink: 0;
	}

	.dropdown-menu :global(button:hover) {
		background: rgba(255, 255, 255, 0.06);
	}

	.dropdown-menu :global(button.danger) {
		color: var(--danger);
	}

	.dropdown-menu :global(button.danger:hover) {
		background: color-mix(in srgb, var(--danger) 15%, transparent);
	}
</style>
