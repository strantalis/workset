<script lang="ts">
	import { ExternalLink, RefreshCw, X } from '@lucide/svelte';
	import type { UpdateNotificationCardModel } from '../composables/createUpdateNotificationController.svelte';
	import Button from './ui/Button.svelte';

	interface Props {
		notification: UpdateNotificationCardModel;
		busy?: boolean;
		onDismiss: () => void;
		onUpdate: () => void;
	}

	const { notification, busy = false, onDismiss, onUpdate }: Props = $props();

	const isApplying = $derived(notification.mode === 'applying');
</script>

<div class="update-toast" aria-live="polite" aria-label="Update available">
	<div class="update-toast__header">
		{#if isApplying}
			<span class="update-toast__icon spinning"><RefreshCw size={13} /></span>
			<span class="update-toast__title">Installing update{notification.latestVersion ? ` ${notification.latestVersion}` : ''}…</span>
		{:else}
			<span class="update-toast__title">
				{notification.latestVersion ? `v${notification.latestVersion} available` : 'Update available'}
			</span>
			<button type="button" class="update-toast__close" aria-label="Dismiss" onclick={onDismiss} disabled={busy}>
				<X size={13} />
			</button>
		{/if}
	</div>
	<p class="update-toast__message">{notification.message}</p>
	{#if notification.error}
		<p class="update-toast__error">{notification.error}</p>
	{/if}
	{#if !isApplying}
		<div class="update-toast__actions">
			<a
				class="update-toast__link"
				href={notification.notesUrl}
				target="_blank"
				rel="noopener noreferrer"
			>
				Release Notes
				<ExternalLink size={11} />
			</a>
			<Button size="sm" variant="primary" onclick={onUpdate} disabled={busy}>
				{busy ? 'Preparing…' : 'Update & Restart'}
			</Button>
		</div>
	{/if}
</div>

<style>
	.update-toast {
		pointer-events: auto;
		padding: 10px 12px;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		line-height: 1.4;
		border: 1px solid color-mix(in srgb, var(--accent) 40%, var(--border));
		background: color-mix(in srgb, var(--accent) 8%, var(--panel));
		color: var(--text);
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.update-toast__header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.update-toast__icon {
		display: inline-flex;
		align-items: center;
		color: var(--accent);
		flex-shrink: 0;
	}

	.update-toast__title {
		font-weight: 600;
		font-size: var(--text-sm);
		color: var(--text);
		flex: 1;
		min-width: 0;
	}

	.update-toast__close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: none;
		border: none;
		color: var(--muted);
		cursor: pointer;
		padding: 2px;
		border-radius: 4px;
		flex-shrink: 0;
		transition: color var(--transition-fast);
	}

	.update-toast__close:hover:not(:disabled) {
		color: var(--text);
	}

	.update-toast__close:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.update-toast__message {
		margin: 0;
		font-size: var(--text-xs);
		color: var(--muted);
		line-height: 1.4;
	}

	.update-toast__error {
		margin: 0;
		font-size: var(--text-xs);
		line-height: 1.4;
		color: var(--danger);
	}

	.update-toast__actions {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		margin-top: 2px;
	}

	.update-toast__link {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		font-size: var(--text-xs);
		color: var(--muted);
		text-decoration: none;
		transition: color var(--transition-fast);
	}

	.update-toast__link:hover {
		color: var(--accent);
	}

	.spinning {
		animation: updateSpin 1.2s linear infinite;
	}

	@keyframes updateSpin {
		from { transform: rotate(0deg); }
		to { transform: rotate(360deg); }
	}
</style>
