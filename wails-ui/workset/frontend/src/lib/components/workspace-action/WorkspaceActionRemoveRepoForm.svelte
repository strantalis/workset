<script lang="ts">
	import Alert from '../ui/Alert.svelte';
	import RemovalOverlay from './RemovalOverlay.svelte';

	type RemoveRepoStatus = {
		statusKnown?: boolean;
		dirty?: boolean;
	} | null;

	interface Props {
		loading: boolean;
		removing: boolean;
		removalSuccess: boolean;
		removeDeleteWorktree: boolean;
		removeRepoConfirmRequired: boolean;
		removeRepoConfirmText: string;
		removeRepoStatusRefreshing: boolean;
		removeRepoStatus: RemoveRepoStatus;
		removeRepoConfirmValid: boolean;
		onToggleDeleteWorktree: (checked: boolean) => void;
		onConfirmTextInput: (value: string) => void;
		onSubmit: () => void;
	}

	const {
		loading,
		removing,
		removalSuccess,
		removeDeleteWorktree,
		removeRepoConfirmRequired,
		removeRepoConfirmText,
		removeRepoStatusRefreshing,
		removeRepoStatus,
		removeRepoConfirmValid,
		onToggleDeleteWorktree,
		onConfirmTextInput,
		onSubmit,
	}: Props = $props();
</script>

<div class="remove-panel" class:removing class:success={removalSuccess}>
	<div class="remove-panel-body">
		<p class="remove-hint">Removes the repo from workspace config by default.</p>

		<label class="remove-option">
			<input
				type="checkbox"
				checked={removeDeleteWorktree}
				onchange={(event) =>
					onToggleDeleteWorktree((event.currentTarget as HTMLInputElement).checked)}
			/>
			<span>Also delete worktrees for this repo</span>
		</label>

		{#if removeRepoConfirmRequired}
			<div class="remove-confirm-section">
				<label class="remove-confirm-field">
					<span class="remove-confirm-label">Type DELETE to confirm</span>
					<input
						class="remove-confirm-input"
						value={removeRepoConfirmText}
						oninput={(event) => onConfirmTextInput((event.currentTarget as HTMLInputElement).value)}
						placeholder="DELETE"
						autocapitalize="off"
						autocorrect="off"
						spellcheck="false"
					/>
				</label>
				{#if removeDeleteWorktree}
					<p class="remove-hint remove-hint-danger">
						Destructive deletes are permanent and cannot be undone.
					</p>
				{/if}
				{#if removeRepoStatusRefreshing}
					<Alert variant="warning">Fetching repo status…</Alert>
				{:else if removeRepoStatus?.statusKnown === false && removeDeleteWorktree}
					<Alert variant="warning">
						Repo status unknown. Destructive deletes may be blocked if the repo is dirty.
					</Alert>
				{/if}
				{#if removeRepoStatus?.dirty && removeDeleteWorktree}
					<Alert variant="warning">
						Uncommitted changes detected. Destructive deletes will be blocked until the repo is
						clean.
					</Alert>
				{/if}
			</div>
		{/if}
	</div>

	<div class="remove-panel-footer">
		<button
			type="button"
			class="remove-panel-submit"
			onclick={onSubmit}
			disabled={loading || !removeRepoConfirmValid}
		>
			{loading ? 'Removing…' : 'Remove Repo'}
		</button>
	</div>

	<RemovalOverlay {removing} {removalSuccess} removingText="Removing repo…" />
</div>

<style>
	.remove-panel {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
		position: relative;
	}

	.remove-panel-body {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.remove-hint {
		font-size: var(--text-sm);
		color: var(--muted);
		margin: 0;
		line-height: 1.5;
	}

	.remove-hint-danger {
		color: var(--danger-text, var(--danger));
	}

	.remove-option {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-sm);
		color: var(--text);
		cursor: pointer;
		padding: 6px 8px;
		border-radius: 6px;
		transition: background var(--transition-fast);
	}

	.remove-option:hover {
		background: color-mix(in srgb, var(--panel-strong) 60%, transparent);
	}

	.remove-option input[type='checkbox'] {
		accent-color: var(--accent);
		flex-shrink: 0;
	}

	.remove-confirm-section {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 10px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--danger) 24%, var(--border));
		background: color-mix(in srgb, var(--danger) 4%, transparent);
	}

	.remove-confirm-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.remove-confirm-label {
		font-size: var(--text-xs);
		font-weight: 600;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.remove-confirm-input {
		width: 100%;
		height: 32px;
		box-sizing: border-box;
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 6px 10px;
		font-size: var(--text-sm);
		font-family: var(--font-mono);
		color: var(--text);
		letter-spacing: 0.1em;
	}

	.remove-confirm-input:focus {
		outline: none;
		border-color: color-mix(in srgb, var(--danger) 48%, var(--border));
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--danger) 20%, transparent);
	}

	.remove-panel-footer {
		flex-shrink: 0;
		display: flex;
		justify-content: flex-end;
		padding-top: 10px;
		border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
		margin-top: auto;
	}

	.remove-panel-submit {
		padding: 7px 16px;
		border: none;
		border-radius: var(--radius-md);
		font-size: var(--text-sm);
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
		background: var(--danger);
		color: white;
		transition:
			background var(--transition-fast),
			opacity var(--transition-fast);
	}

	.remove-panel-submit:hover:not(:disabled) {
		background: color-mix(in srgb, var(--danger) 85%, white);
	}

	.remove-panel-submit:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}
</style>
