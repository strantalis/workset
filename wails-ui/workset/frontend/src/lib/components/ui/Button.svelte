<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'primary' | 'ghost' | 'danger';
		size?: 'sm' | 'md';
		disabled?: boolean;
		type?: 'button' | 'submit';
		class?: string;
		onclick?: () => void;
		children: Snippet;
	}

	const {
		variant = 'ghost',
		size = 'md',
		disabled = false,
		type = 'button',
		class: className = '',
		onclick,
		children,
	}: Props = $props();

	let buttonRef = $state<HTMLButtonElement | null>(null);
	let ripples = $state<{ x: number; y: number; id: number }[]>([]);
	let rippleId = 0;

	const createRipple = (event: MouseEvent) => {
		if (!buttonRef || disabled) return;
		const rect = buttonRef.getBoundingClientRect();
		const x = event.clientX - rect.left;
		const y = event.clientY - rect.top;
		const id = rippleId++;
		ripples = [...ripples, { x, y, id }];
		setTimeout(() => {
			ripples = ripples.filter((r) => r.id !== id);
		}, 600);
	};

	const handleClick = (event: MouseEvent) => {
		createRipple(event);
		onclick?.();
	};
</script>

<button
	bind:this={buttonRef}
	class="btn {variant} {size} {className}"
	{type}
	{disabled}
	onclick={handleClick}
>
	{@render children()}
	{#each ripples as ripple (ripple.id)}
		<span class="ripple" style="left: {ripple.x}px; top: {ripple.y}px;"></span>
	{/each}
</button>

<style>
	.btn {
		position: relative;
		overflow: hidden;
		border-radius: var(--radius-md);
		cursor: pointer;
		font-size: var(--text-base);
		font-family: inherit;
		transition:
			background var(--transition-fast),
			border-color var(--transition-fast),
			transform var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.btn:active:not(:disabled) {
		transform: scale(0.96) translateY(1px);
	}

	.btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	/* Ripple effect */
	.ripple {
		position: absolute;
		width: 8px;
		height: 8px;
		margin-left: -4px;
		margin-top: -4px;
		border-radius: 50%;
		background: rgba(255, 255, 255, 0.35);
		pointer-events: none;
		animation: rippleExpand 0.6s ease-out forwards;
	}

	.btn.primary .ripple {
		background: rgba(255, 255, 255, 0.25);
	}

	@keyframes rippleExpand {
		0% {
			transform: scale(0);
			opacity: 1;
		}
		100% {
			transform: scale(32);
			opacity: 0;
		}
	}

	/* Size variants */
	.btn.md {
		padding: 8px 14px;
	}

	.btn.sm {
		padding: 6px 10px;
		font-size: var(--text-sm);
	}

	/* Ghost variant */
	.btn.ghost {
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		color: var(--text);
	}

	.btn.ghost:hover:not(:disabled) {
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.04);
	}

	/* Primary variant */
	.btn.primary {
		background: var(--accent);
		border: none;
		color: white;
		font-weight: 600;
		box-shadow:
			var(--shadow-sm),
			inset 0 1px 0 rgba(255, 255, 255, 0.15),
			0 0 0 0 rgba(var(--accent-rgb), 0.4);
	}

	.btn.primary:hover:not(:disabled) {
		background: color-mix(in srgb, var(--accent) 85%, white);
		box-shadow:
			var(--shadow-md),
			inset 0 1px 0 rgba(255, 255, 255, 0.15),
			0 0 0 0 rgba(var(--accent-rgb), 0.4);
	}

	.btn.primary:active:not(:disabled) {
		box-shadow:
			inset 0 2px 4px rgba(0, 0, 0, 0.2),
			0 0 0 0 rgba(var(--accent-rgb), 0.4);
	}

	.btn.primary:disabled {
		opacity: 0.6;
	}

	/* Danger variant */
	.btn.danger {
		background: var(--danger-subtle);
		border: 1px solid var(--danger-soft);
		color: #ff9a9a;
		font-weight: 600;
	}

	.btn.danger:hover:not(:disabled) {
		background: var(--danger-soft);
		border-color: var(--danger);
	}
</style>
