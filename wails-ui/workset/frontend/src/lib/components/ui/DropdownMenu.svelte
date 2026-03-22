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
		anchorPoint?: { top: number; left: number } | null;
	}

	const {
		open,
		onClose,
		position = 'right',
		children,
		trigger = null,
		anchorPoint = null,
	}: Props = $props();

	let menuElement: HTMLElement | null = $state(null);
	let menuPosition = $state({ top: 0, left: 0 });
	let previousFocus: HTMLElement | null = null;

	const getMenuItems = (): HTMLElement[] => {
		if (!menuElement) return [];
		return Array.from(menuElement.querySelectorAll<HTMLElement>('button, [role="menuitem"]'));
	};

	const handleKeydown = (event: KeyboardEvent): void => {
		const items = getMenuItems();
		if (items.length === 0) return;

		if (event.key === 'Escape') {
			event.preventDefault();
			onClose();
			return;
		}

		if (event.key === 'Tab') {
			event.preventDefault();
			onClose();
			return;
		}

		const currentIndex = items.indexOf(document.activeElement as HTMLElement);

		if (event.key === 'ArrowDown') {
			event.preventDefault();
			const next = currentIndex < items.length - 1 ? currentIndex + 1 : 0;
			items[next]?.focus();
			return;
		}

		if (event.key === 'ArrowUp') {
			event.preventDefault();
			const prev = currentIndex > 0 ? currentIndex - 1 : items.length - 1;
			items[prev]?.focus();
			return;
		}

		if (event.key === 'Home') {
			event.preventDefault();
			items[0]?.focus();
			return;
		}

		if (event.key === 'End') {
			event.preventDefault();
			items[items.length - 1]?.focus();
		}
	};

	$effect(() => {
		if (open && menuElement && (trigger || anchorPoint)) {
			tick().then(() => {
				updatePosition();
			});
		}
	});

	$effect(() => {
		if (open) {
			previousFocus = document.activeElement as HTMLElement | null;
			tick().then(() => {
				const items = getMenuItems();
				items.forEach((item) => {
					if (!item.getAttribute('role')) {
						item.setAttribute('role', 'menuitem');
					}
					item.setAttribute('tabindex', '-1');
				});
				items[0]?.focus();
			});
		} else if (previousFocus) {
			previousFocus.focus();
			previousFocus = null;
		}
	});

	function updatePosition() {
		if (!menuElement) return;
		const menuRect = menuElement.getBoundingClientRect();
		const gap = 4;
		let top: number;
		let left: number;

		if (anchorPoint) {
			top = anchorPoint.top + gap;
			left = anchorPoint.left + gap;
		} else if (trigger) {
			const triggerRect = trigger.getBoundingClientRect();
			top = triggerRect.bottom + gap;
			left = position === 'right' ? triggerRect.right - menuRect.width : triggerRect.left;

			const spaceBelow = window.innerHeight - triggerRect.bottom - gap;
			const spaceAbove = triggerRect.top - gap;
			if (menuRect.height > spaceBelow && spaceAbove > spaceBelow) {
				top = triggerRect.top - menuRect.height - gap;
			}
		} else {
			return;
		}

		if (left < 8) left = 8;
		if (left + menuRect.width > window.innerWidth - 8) {
			left = window.innerWidth - menuRect.width - 8;
		}

		if (top < 8) top = 8;
		if (top + menuRect.height > window.innerHeight - 8) {
			top = window.innerHeight - menuRect.height - 8;
		}

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
		tabindex="-1"
		onkeydown={handleKeydown}
	>
		{@render children()}
	</div>
{/if}

<style>
	.dropdown-menu {
		position: fixed;
		background: var(--glass-bg-strong);
		border: 1px solid var(--glass-border);
		border-radius: 10px;
		padding: 6px;
		display: grid;
		gap: 4px;
		z-index: 9999;
		min-width: 140px;
		backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
		-webkit-backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
		box-shadow: var(--glass-shadow), var(--inset-highlight);
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
		background: var(--hover-bg);
	}

	.dropdown-menu :global(button.danger) {
		color: var(--danger);
	}

	.dropdown-menu :global(button.danger:hover) {
		background: var(--hover-danger-bg);
	}
</style>
