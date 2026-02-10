<script lang="ts">
	interface Props {
		removing: boolean;
		removalSuccess: boolean;
		removingText: string;
	}

	const { removing, removalSuccess, removingText }: Props = $props();
</script>

{#if removing}
	<div class="removal-overlay">
		{#if removalSuccess}
			<div class="removal-success">
				<svg
					class="success-icon"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
				>
					<path d="M20 6L9 17l-5-5" />
				</svg>
				<span class="removal-text">Removed successfully</span>
			</div>
		{:else}
			<div class="removal-loading">
				<div class="spinner"></div>
				<span class="removal-text">{removingText}</span>
			</div>
		{/if}
	</div>
{/if}

<style>
	.removal-overlay {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(11, 15, 24, 0.6);
		border-radius: var(--radius-md);
		animation: overlayFadeIn 0.2s ease-out;
	}

	@keyframes overlayFadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	.removal-loading,
	.removal-success {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 16px;
		padding: 24px;
		animation: contentSlideIn 0.3s ease-out;
	}

	@keyframes contentSlideIn {
		from {
			opacity: 0;
			transform: translateY(8px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.spinner {
		width: 32px;
		height: 32px;
		border: 3px solid var(--muted);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}

	.removal-text {
		font-size: var(--text-md);
		font-weight: 500;
		color: var(--text);
	}

	.success-icon {
		width: 48px;
		height: 48px;
		color: var(--success);
		animation: successPop 0.4s ease-out;
	}

	@keyframes successPop {
		0% {
			transform: scale(0.5);
			opacity: 0;
		}
		50% {
			transform: scale(1.1);
		}
		100% {
			transform: scale(1);
			opacity: 1;
		}
	}

	.removal-success {
		animation: containerPulse 1.2s ease-out;
	}

	@keyframes containerPulse {
		0% {
			box-shadow: 0 0 0 0 rgba(var(--success-rgb), 0.4);
		}
		50% {
			box-shadow: 0 0 16px 6px rgba(var(--success-rgb), 0.15);
		}
		100% {
			box-shadow: 0 0 0 0 rgba(var(--success-rgb), 0);
		}
	}
</style>
