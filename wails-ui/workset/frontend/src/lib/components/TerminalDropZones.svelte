<script lang="ts">
	type DropZone = 'left' | 'right' | 'top' | 'bottom' | 'center' | null;

	interface Props {
		show: boolean;
		activeDropZone: DropZone;
	}

	const { show, activeDropZone }: Props = $props();
</script>

{#if show}
	<div class="drop-zones">
		<div class="drop-zone left" class:active={activeDropZone === 'left'}></div>
		<div class="drop-zone right" class:active={activeDropZone === 'right'}></div>
		<div class="drop-zone top" class:active={activeDropZone === 'top'}></div>
		<div class="drop-zone bottom" class:active={activeDropZone === 'bottom'}></div>
		<div class="drop-zone center" class:active={activeDropZone === 'center'}></div>
	</div>
{/if}

<style>
	.drop-zones {
		position: absolute;
		inset: 0;
		pointer-events: none;
		z-index: 10;
	}

	.drop-zone {
		position: absolute;
		background: color-mix(in srgb, var(--accent) 10%, transparent);
		border: 1px solid transparent;
		border-radius: 8px;
		opacity: 0;
		transition:
			opacity 0.15s ease,
			border-color 0.15s ease;
		backdrop-filter: blur(2px);
	}

	.drop-zone.active {
		opacity: 1;
		border-color: var(--accent);
		background: color-mix(in srgb, var(--accent) 15%, transparent);
	}

	.drop-zone.left {
		left: 4px;
		top: 4px;
		bottom: 4px;
		width: calc(25% - 4px);
	}

	.drop-zone.right {
		right: 4px;
		top: 4px;
		bottom: 4px;
		width: calc(25% - 4px);
	}

	.drop-zone.top {
		left: 4px;
		right: 4px;
		top: 4px;
		height: calc(25% - 4px);
	}

	.drop-zone.bottom {
		left: 4px;
		right: 4px;
		bottom: 4px;
		height: calc(25% - 4px);
	}

	.drop-zone.center {
		left: 30%;
		right: 30%;
		top: 30%;
		bottom: 30%;
	}
</style>
