<script lang="ts">
	import Alert from '../ui/Alert.svelte';
	import Button from '../ui/Button.svelte';
	import RemovalOverlay from './RemovalOverlay.svelte';

	interface Props {
		loading: boolean;
		removing: boolean;
		removalSuccess: boolean;
		removeDeleteFiles: boolean;
		removeForceDelete: boolean;
		removeConfirmText: string;
		removeConfirmValid: boolean;
		onToggleDeleteFiles: (checked: boolean) => void;
		onToggleForceDelete: (checked: boolean) => void;
		onConfirmTextInput: (value: string) => void;
		onSubmit: () => void;
	}

	const {
		loading,
		removing,
		removalSuccess,
		removeDeleteFiles,
		removeForceDelete,
		removeConfirmText,
		removeConfirmValid,
		onToggleDeleteFiles,
		onToggleForceDelete,
		onConfirmTextInput,
		onSubmit,
	}: Props = $props();
</script>

<div class="form form-removing" class:removing class:success={removalSuccess}>
	<div class="form-content">
		<div class="hint hint-intro">Remove workspace registration only by default.</div>
		<label class="option option-main">
			<input
				type="checkbox"
				checked={removeDeleteFiles}
				onchange={(event) => onToggleDeleteFiles((event.currentTarget as HTMLInputElement).checked)}
			/>
			<span>Also delete workspace files and worktrees</span>
		</label>
		{#if removeDeleteFiles}
			<div class="deletion-options">
				<div class="hint deletion-hint">
					Deletes the workspace directory and removes all worktrees.
				</div>
				<label class="field">
					<span>Type DELETE to confirm</span>
					<input
						value={removeConfirmText}
						oninput={(event) => onConfirmTextInput((event.currentTarget as HTMLInputElement).value)}
						placeholder="DELETE"
						autocapitalize="off"
						autocorrect="off"
						spellcheck="false"
					/>
				</label>
				<label class="option">
					<input
						type="checkbox"
						checked={removeForceDelete}
						onchange={(event) =>
							onToggleForceDelete((event.currentTarget as HTMLInputElement).checked)}
					/>
					<span>Force delete (skip safety checks)</span>
				</label>
				{#if removeForceDelete}
					<Alert variant="warning">
						Force delete bypasses dirty/unmerged checks and may delete uncommitted work.
					</Alert>
				{/if}
			</div>
		{/if}
		<Button
			variant="danger"
			onclick={onSubmit}
			disabled={loading || !removeConfirmValid}
			class="action-btn"
		>
			{loading ? 'Removing…' : 'Remove workspace'}
		</Button>
	</div>
	<RemovalOverlay {removing} {removalSuccess} removingText="Removing workspace…" />
</div>

<style>
	.form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.form.form-removing {
		gap: 20px;
		position: relative;
	}

	.form-content {
		transition:
			opacity 0.3s ease,
			filter 0.3s ease;
	}

	.form-removing.removing .form-content {
		opacity: 0.4;
		filter: blur(1px);
		pointer-events: none;
	}

	.form-removing.success .form-content {
		opacity: 0.3;
		filter: blur(2px);
		pointer-events: none;
	}

	.deletion-options {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 16px;
		margin-top: 4px;
	}

	.deletion-options :global(.alert) {
		margin: 0;
	}

	.hint-intro {
		margin-bottom: 8px;
		line-height: 1.5;
	}

	.deletion-hint {
		line-height: 1.5;
		margin: 0;
	}

	.option-main {
		margin-top: 4px;
		margin-bottom: 4px;
	}

	.option {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-base);
		color: var(--text);
	}

	.option input {
		accent-color: var(--accent);
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.field input {
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		color: var(--text);
		padding: 8px 10px;
		font-size: var(--text-md);
	}

	:global(.action-btn) {
		width: 100%;
		margin-top: 8px;
	}

	.form-removing :global(.action-btn) {
		margin-top: 16px;
	}

	.hint {
		font-size: var(--text-sm);
		color: var(--muted);
	}
</style>
