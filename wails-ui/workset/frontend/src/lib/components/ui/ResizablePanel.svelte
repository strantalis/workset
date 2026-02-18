<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		direction?: 'horizontal' | 'vertical';
		initialRatio?: number;
		minRatio?: number;
		maxRatio?: number;
		storageKey?: string;
		children: Snippet;
		second: Snippet;
	}

	const {
		direction = 'horizontal',
		initialRatio = 0.2,
		minRatio = 0.1,
		maxRatio = 0.9,
		storageKey,
		children,
		second,
	}: Props = $props();

	const loadInitialRatio = (): number => {
		if (!storageKey) return initialRatio;
		try {
			const stored = localStorage.getItem(storageKey);
			if (stored) {
				const parsed = Number.parseFloat(stored);
				if (Number.isFinite(parsed) && parsed >= minRatio && parsed <= maxRatio) {
					return parsed;
				}
			}
		} catch {
			// storage unavailable, use default
		}
		return initialRatio;
	};

	let ratio = $state(loadInitialRatio());
	let isDragging = $state(false);
	let containerRef = $state<HTMLDivElement | null>(null);

	const persist = (): void => {
		if (!storageKey) return;
		try {
			localStorage.setItem(storageKey, String(ratio));
		} catch {
			// storage unavailable
		}
	};

	const handlePointerDown = (event: PointerEvent): void => {
		event.preventDefault();
		isDragging = true;
		const target = event.currentTarget as HTMLDivElement;
		target.setPointerCapture(event.pointerId);
	};

	const handlePointerMove = (event: PointerEvent): void => {
		if (!isDragging || !containerRef) return;
		const rect = containerRef.getBoundingClientRect();
		let newRatio: number;
		if (direction === 'horizontal') {
			newRatio = (event.clientX - rect.left) / rect.width;
		} else {
			newRatio = (event.clientY - rect.top) / rect.height;
		}
		ratio = Math.max(minRatio, Math.min(maxRatio, newRatio));
	};

	const handlePointerUp = (event: PointerEvent): void => {
		if (!isDragging) return;
		isDragging = false;
		const target = event.currentTarget as HTMLDivElement;
		target.releasePointerCapture(event.pointerId);
		persist();
	};

	const handleKeyDown = (event: KeyboardEvent): void => {
		const step = event.shiftKey ? 0.1 : 0.02;
		let delta = 0;
		if (direction === 'horizontal') {
			if (event.key === 'ArrowLeft') delta = -step;
			else if (event.key === 'ArrowRight') delta = step;
		} else {
			if (event.key === 'ArrowUp') delta = -step;
			else if (event.key === 'ArrowDown') delta = step;
		}
		if (delta !== 0) {
			event.preventDefault();
			ratio = Math.max(minRatio, Math.min(maxRatio, ratio + delta));
			persist();
		}
	};
</script>

<div class="resizable-panel {direction}" class:dragging={isDragging} bind:this={containerRef}>
	<div class="panel-first" style="flex: {ratio} 1 0%">
		{@render children()}
	</div>
	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="panel-divider"
		class:active={isDragging}
		role="separator"
		tabindex="0"
		aria-orientation={direction === 'horizontal' ? 'vertical' : 'horizontal'}
		aria-valuenow={Math.round(ratio * 100)}
		aria-valuemin={Math.round(minRatio * 100)}
		aria-valuemax={Math.round(maxRatio * 100)}
		onpointerdown={handlePointerDown}
		onpointermove={handlePointerMove}
		onpointerup={handlePointerUp}
		onpointercancel={handlePointerUp}
		onkeydown={handleKeyDown}
	></div>
	<div class="panel-second" style="flex: {1 - ratio} 1 0%">
		{@render second()}
	</div>
</div>

<style>
	.resizable-panel {
		display: flex;
		flex: 1;
		min-height: 0;
		min-width: 0;
		height: 100%;
	}

	.resizable-panel.horizontal {
		flex-direction: row;
	}

	.resizable-panel.vertical {
		flex-direction: column;
	}

	.resizable-panel.dragging {
		user-select: none;
	}

	.panel-first,
	.panel-second {
		display: flex;
		flex-direction: column;
		min-height: 0;
		min-width: 0;
		overflow: hidden;
	}

	.panel-divider {
		flex-shrink: 0;
		background: var(--border);
		opacity: 0.5;
		transition:
			opacity 0.15s ease,
			background 0.15s ease;
		touch-action: none;
		position: relative;
	}

	.horizontal > .panel-divider {
		width: 1px;
		cursor: col-resize;
	}

	.vertical > .panel-divider {
		height: 1px;
		cursor: row-resize;
	}

	/* Expanded hit area (12px) for easier grabbing */
	.panel-divider::before {
		content: '';
		position: absolute;
	}

	.horizontal > .panel-divider::before {
		top: 0;
		bottom: 0;
		width: 12px;
		left: 50%;
		transform: translateX(-50%);
	}

	.vertical > .panel-divider::before {
		left: 0;
		right: 0;
		height: 12px;
		top: 50%;
		transform: translateY(-50%);
	}

	.panel-divider:hover,
	.panel-divider:focus,
	.panel-divider.active {
		opacity: 1;
		background: var(--accent);
	}

	.horizontal > .panel-divider:hover,
	.horizontal > .panel-divider:focus,
	.horizontal > .panel-divider.active {
		width: 3px;
		margin: 0 -1px;
	}

	.vertical > .panel-divider:hover,
	.vertical > .panel-divider:focus,
	.vertical > .panel-divider.active {
		height: 3px;
		margin: -1px 0;
	}

	.panel-divider:focus {
		outline: none;
	}
</style>
