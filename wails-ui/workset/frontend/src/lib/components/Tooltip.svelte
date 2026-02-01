<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		text: string | string[];
		position?: 'top' | 'bottom' | 'left' | 'right' | 'cursor';
		class?: string;
		children: Snippet;
	}

	const { text, position = 'top', class: className = '', children }: Props = $props();

	let visible = $state(false);
	let wrapperEl: HTMLSpanElement | null = $state(null);
	let tooltipStyle = $state('');
	let mouseX = $state(0);
	let mouseY = $state(0);

	const lines = $derived(Array.isArray(text) ? text : [text]);
	const hasContent = $derived(lines.length > 0 && lines.some((l) => l.length > 0));

	const updatePosition = () => {
		if (!wrapperEl) return;
		const rect = wrapperEl.getBoundingClientRect();
		const gap = 8;

		if (position === 'cursor') {
			// Position near mouse cursor, offset to bottom-right
			tooltipStyle = `top: ${mouseY + 12}px; left: ${mouseX + 12}px;`;
			return;
		}

		let top = 0;
		let left = 0;

		switch (position) {
			case 'top':
				top = rect.top - gap;
				left = rect.left + rect.width / 2;
				tooltipStyle = `top: ${top}px; left: ${left}px; transform: translateX(-50%) translateY(-100%);`;
				break;
			case 'bottom':
				top = rect.bottom + gap;
				left = rect.left + rect.width / 2;
				tooltipStyle = `top: ${top}px; left: ${left}px; transform: translateX(-50%);`;
				break;
			case 'left':
				top = rect.top + rect.height / 2;
				left = rect.left - gap;
				tooltipStyle = `top: ${top}px; left: ${left}px; transform: translateX(-100%) translateY(-50%);`;
				break;
			case 'right':
				top = rect.top + rect.height / 2;
				left = rect.right + gap;
				tooltipStyle = `top: ${top}px; left: ${left}px; transform: translateY(-50%);`;
				break;
		}
	};

	const handleMouseMove = (e: MouseEvent) => {
		mouseX = e.clientX;
		mouseY = e.clientY;
		if (visible && position === 'cursor') {
			updatePosition();
		}
	};

	const show = () => {
		visible = true;
		updatePosition();
	};

	const hide = () => {
		visible = false;
	};
</script>

<span
	bind:this={wrapperEl}
	class="tooltip-wrapper {className}"
	role="group"
	onmouseenter={show}
	onmouseleave={hide}
	onmousemove={handleMouseMove}
	onfocusin={show}
	onfocusout={hide}
>
	{@render children()}
	{#if visible && hasContent}
		<div class="tooltip" role="tooltip" style={tooltipStyle}>
			{#each lines as line, index (index)}
				<div class="tooltip-line">{line}</div>
			{/each}
		</div>
	{/if}
</span>

<style>
	.tooltip-wrapper {
		position: relative;
		display: block;
	}

	.tooltip {
		position: fixed;
		z-index: 10000;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 8px 10px;
		font-size: 12px;
		color: var(--text);
		white-space: nowrap;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
		pointer-events: none;
		animation: tooltipFadeIn 0.15s ease-out;
	}

	.tooltip-line {
		line-height: 1.4;
	}

	.tooltip-line:not(:last-child) {
		margin-bottom: 2px;
	}

	@keyframes tooltipFadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}
</style>
