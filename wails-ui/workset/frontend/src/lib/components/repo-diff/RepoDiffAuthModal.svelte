<script lang="ts">
	import GitHubLoginModal from '../GitHubLoginModal.svelte';

	interface Props {
		notice: string | null;
		onClose: () => void;
		onSuccess: () => Promise<void> | void;
	}

	const { notice, onClose, onSuccess }: Props = $props();
</script>

<div
	class="overlay"
	role="button"
	tabindex="0"
	onclick={onClose}
	onkeydown={(event) => {
		if (event.key === 'Escape') onClose();
	}}
>
	<div
		class="overlay-panel"
		role="presentation"
		onclick={(event) => event.stopPropagation()}
		onkeydown={(event) => event.stopPropagation()}
	>
		<GitHubLoginModal {notice} {onClose} {onSuccess} />
	</div>
</div>

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(6, 9, 14, 0.78);
		display: grid;
		place-items: center;
		z-index: 30;
		padding: 24px;
		animation: overlayFadeIn var(--transition-normal) ease-out;
	}

	.overlay-panel {
		width: 100%;
		display: flex;
		justify-content: center;
		animation: modalSlideIn 200ms ease-out;
	}

	@keyframes overlayFadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	@keyframes modalSlideIn {
		from {
			opacity: 0;
			transform: translateY(-8px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

	@media (max-width: 720px) {
		.overlay {
			padding: 0;
		}
	}
</style>
