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

<div
	class="form form-removing ws-form-stack ws-form-stack-lg ws-removal-form"
	class:removing
	class:success={removalSuccess}
>
	<div class="form-content ws-removal-content">
		<div class="hint hint-intro ws-hint ws-hint-intro">
			Remove workspace registration only by default.
		</div>
		<label class="option option-main ws-option ws-option-main">
			<input
				type="checkbox"
				checked={removeDeleteFiles}
				onchange={(event) => onToggleDeleteFiles((event.currentTarget as HTMLInputElement).checked)}
			/>
			<span>Also delete workspace files and worktrees</span>
		</label>
		{#if removeDeleteFiles}
			<div class="deletion-options ws-deletion-options">
				<div class="hint deletion-hint ws-hint ws-deletion-hint">
					Deletes the workspace directory and removes all worktrees.
				</div>
				<label class="field ws-field">
					<span>Type DELETE to confirm</span>
					<input
						class="ws-field-input"
						value={removeConfirmText}
						oninput={(event) => onConfirmTextInput((event.currentTarget as HTMLInputElement).value)}
						placeholder="DELETE"
						autocapitalize="off"
						autocorrect="off"
						spellcheck="false"
					/>
				</label>
				<label class="option ws-option">
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
			class="action-btn ws-action-btn"
		>
			{loading ? 'Removing…' : 'Remove workspace'}
		</Button>
	</div>
	<RemovalOverlay {removing} {removalSuccess} removingText="Removing workspace…" />
</div>
