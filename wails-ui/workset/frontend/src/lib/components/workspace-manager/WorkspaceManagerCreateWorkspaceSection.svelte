<script lang="ts">
	interface Props {
		createName: string;
		createPath: string;
		createError: string | null;
		createSuccess: string | null;
		creating: boolean;
		onCreateNameChange: (value: string) => void;
		onCreatePathChange: (value: string) => void;
		onCreateInputChange: (input: HTMLInputElement | null) => void;
		onCreate: () => void;
	}

	const {
		createName,
		createPath,
		createError,
		createSuccess,
		creating,
		onCreateNameChange,
		onCreatePathChange,
		onCreateInputChange,
		onCreate,
	}: Props = $props();

	let createInput: HTMLInputElement | null = $state(null);

	$effect(() => {
		onCreateInputChange(createInput);
	});

	const handleEnter = (event: KeyboardEvent): void => {
		if (event.key === 'Enter') {
			onCreate();
		}
	};
</script>

<section class="create">
	<div class="section-title">Create workspace</div>
	<div class="form-grid">
		<label class="field">
			<span>Name</span>
			<input
				placeholder="acme"
				bind:this={createInput}
				value={createName}
				autocapitalize="off"
				autocorrect="off"
				spellcheck="false"
				oninput={(event) => onCreateNameChange((event.currentTarget as HTMLInputElement).value)}
				onkeydown={handleEnter}
			/>
		</label>
		<label class="field span-2">
			<span>Path (optional)</span>
			<input
				placeholder="~/workspaces/acme"
				value={createPath}
				autocapitalize="off"
				autocorrect="off"
				spellcheck="false"
				oninput={(event) => onCreatePathChange((event.currentTarget as HTMLInputElement).value)}
				onkeydown={handleEnter}
			/>
		</label>
	</div>
	<div class="inline-actions">
		<button class="primary" type="button" onclick={onCreate} disabled={creating}>
			{creating ? 'Creating…' : 'Create workspace'}
		</button>
		{#if createError}
			<div class="note error">{createError}</div>
		{:else if createSuccess}
			<div class="note success">{createSuccess}</div>
		{/if}
	</div>
</section>

<style>
	.create {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 16px;
	}

	.section-title {
		font-size: var(--text-base);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
		font-weight: 600;
	}

	.form-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 12px;
		margin-top: 12px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.field input {
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: 10px;
		color: var(--text);
		padding: 8px 10px;
		font-size: var(--text-md);
	}

	.span-2 {
		grid-column: span 2;
	}

	.inline-actions {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-top: 12px;
	}

	.primary {
		background: var(--accent);
		color: #081018;
		border: none;
		padding: 8px 16px;
		border-radius: 10px;
		font-weight: 600;
		cursor: pointer;
	}

	.note {
		font-size: var(--text-base);
	}

	.note.error {
		color: var(--danger);
	}

	.note.success {
		color: var(--success);
	}
</style>
