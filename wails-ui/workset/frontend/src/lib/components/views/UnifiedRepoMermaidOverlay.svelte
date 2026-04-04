<script lang="ts">
	import { Minus, Plus, X } from '@lucide/svelte';

	interface Props {
		open: boolean;
		markup: string;
		zoom: number;
		fitScale: number;
		offsetX: number;
		offsetY: number;
		intrinsicW: number;
		intrinsicH: number;
		dragging: boolean;
		setCanvasEl: (node: HTMLElement | null) => void;
		setStageEl: (node: HTMLElement | null) => void;
		onClose: () => void;
		onAdjustZoom: (delta: number) => void;
		onResetZoom: () => void;
		onPointerDown: (event: PointerEvent) => void;
		onPointerMove: (event: PointerEvent) => void;
		onPointerUp: (event: PointerEvent) => void;
	}

	const {
		open,
		markup,
		zoom,
		fitScale,
		offsetX,
		offsetY,
		intrinsicW,
		intrinsicH,
		dragging,
		setCanvasEl,
		setStageEl,
		onClose,
		onAdjustZoom,
		onResetZoom,
		onPointerDown,
		onPointerMove,
		onPointerUp,
	}: Props = $props();

	let canvasEl = $state<HTMLElement | null>(null);
	let stageEl = $state<HTMLElement | null>(null);

	$effect(() => {
		setCanvasEl(canvasEl);
	});

	$effect(() => {
		setStageEl(stageEl);
	});
</script>

{#if open}
	<div
		class="mm-overlay"
		role="presentation"
		onclick={onClose}
		onkeydown={(event) => {
			if (event.key === 'Escape') onClose();
		}}
	>
		<div
			class="mm-panel"
			role="dialog"
			aria-modal="true"
			tabindex="-1"
			onclick={(event) => event.stopPropagation()}
			onkeydown={(event) => event.stopPropagation()}
		>
			<div class="mm-toolbar">
				<div class="mm-zoom-actions">
					<button
						type="button"
						class="mm-btn"
						aria-label="Zoom out"
						onclick={() => onAdjustZoom(-0.1)}><Minus size={15} /></button
					>
					<button type="button" class="mm-btn-text" onclick={onResetZoom}
						>{Math.round(zoom * 100)}%</button
					>
					<button
						type="button"
						class="mm-btn"
						aria-label="Zoom in"
						onclick={() => onAdjustZoom(0.1)}><Plus size={15} /></button
					>
				</div>
				<button type="button" class="mm-btn" aria-label="Close" onclick={onClose}
					><X size={15} /></button
				>
			</div>
			<div class="mm-canvas">
				<div
					bind:this={canvasEl}
					class="mm-surface"
					class:dragging
					role="presentation"
					onpointerdown={onPointerDown}
					onpointermove={onPointerMove}
					onpointerup={onPointerUp}
					onpointercancel={onPointerUp}
				>
					<div
						bind:this={stageEl}
						class="mm-stage"
						style={`--mm-scale:${fitScale * zoom}; --mm-x:${offsetX}px; --mm-y:${offsetY}px; --mm-w:${intrinsicW}px; --mm-h:${intrinsicH}px;`}
					>
						<!-- eslint-disable-next-line svelte/no-at-html-tags -->
						{@html markup}
					</div>
				</div>
			</div>
		</div>
	</div>
{/if}
