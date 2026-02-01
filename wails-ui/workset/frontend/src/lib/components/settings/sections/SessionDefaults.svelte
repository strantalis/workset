<script lang="ts">
	import type { SettingsDefaults } from '../../../types';
	import SettingsSection from '../SettingsSection.svelte';
	import Select from '../../ui/Select.svelte';

	type FieldId = keyof SettingsDefaults;

	interface Props {
		draft: Record<FieldId, string>;
		baseline: Record<FieldId, string>;
		onUpdate: (id: FieldId, value: string) => void;
		onRestartSessiond: () => void;
		restartingSessiond?: boolean;
	}

	const {
		draft,
		baseline,
		onUpdate,
		onRestartSessiond,
		restartingSessiond = false,
	}: Props = $props();

	let restartCompleted = $state(false);
	let previousRestarting = $state(false);

	$effect(() => {
		const isRestarting = restartingSessiond;
		if (previousRestarting && !isRestarting) {
			restartCompleted = true;
			setTimeout(() => {
				restartCompleted = false;
			}, 1500);
		}
		previousRestarting = isRestarting;
	});

	type Field = {
		id: FieldId;
		label: string;
		description: string;
		placeholder?: string;
		type?: 'text' | 'select';
		options?: { label: string; value: string }[];
	};

	const fields: Field[] = [
		{
			id: 'terminalRenderer',
			label: 'Terminal renderer',
			description: 'Auto picks WebGL when healthy, otherwise Canvas.',
			type: 'select',
			options: [
				{ label: 'Auto', value: 'auto' },
				{ label: 'WebGL', value: 'webgl' },
				{ label: 'Canvas', value: 'canvas' },
			],
		},
		{
			id: 'terminalProtocolLog',
			label: 'Protocol logging',
			description: 'Logs OSC/CSI/DSR traffic to ~/.workset/terminal_logs (restart daemon).',
			type: 'select',
			options: [
				{ label: 'Off', value: 'off' },
				{ label: 'On', value: 'on' },
			],
		},
		{
			id: 'terminalIdleTimeout',
			label: 'Terminal idle timeout',
			description: 'Idle terminals are closed after this duration. Use 0 to disable.',
			placeholder: '0',
		},
	];

	const getValue = (id: FieldId): string => draft[id] ?? '';

	const isChanged = (id: FieldId): boolean => draft[id] !== baseline[id];

	const handleInput = (id: FieldId, event: Event): void => {
		const target = event.target as HTMLInputElement | null;
		onUpdate(id, target?.value ?? '');
	};
</script>

<SettingsSection title="Terminal defaults" description="Defaults for the GUI terminal launcher.">
	<div class="compact-fields">
		{#each fields as field (field.id)}
			<div class="compact-field" class:changed={isChanged(field.id)}>
				<label for={field.id}>{field.label}</label>
				<div class="input-row">
					{#if field.type === 'select'}
						<Select
							id={field.id}
							value={getValue(field.id)}
							options={field.options ?? []}
							onchange={(val) => onUpdate(field.id, val)}
						/>
					{:else}
						<input
							id={field.id}
							type="text"
							placeholder={field.placeholder ?? ''}
							value={getValue(field.id)}
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
							oninput={(event) => handleInput(field.id, event)}
						/>
					{/if}
					<span class="hint">{field.description}</span>
				</div>
			</div>
		{/each}
	</div>

	<div class="sessiond-actions" class:restart-completed={restartCompleted}>
		<span class="hint">Restart if terminals get stuck or after changing daemon settings.</span>
		<button
			class="restart"
			type="button"
			onclick={onRestartSessiond}
			disabled={restartingSessiond}
			class:restarting={restartingSessiond}
		>
			{#if restartingSessiond}
				<span class="spin-icon">⟳</span> Restarting…
			{:else}
				Restart daemon
			{/if}
		</button>
	</div>
</SettingsSection>

<style>
	.compact-fields {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.compact-field {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.compact-field.changed label::after {
		content: '*';
		color: var(--warning);
		margin-left: 4px;
	}

	.compact-field label {
		font-size: 13px;
		color: var(--text);
		min-width: 140px;
		flex-shrink: 0;
	}

	.input-row {
		display: flex;
		align-items: center;
		gap: 12px;
		flex: 1;
	}

	.input-row :global(.select-wrapper) {
		width: 140px;
		flex-shrink: 0;
	}

	.compact-field input {
		width: 100px;
		background: var(--panel-strong);
		border: 1px solid rgba(255, 255, 255, 0.08);
		color: var(--text);
		border-radius: var(--radius-md);
		padding: 8px 12px;
		font-size: 13px;
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.compact-field input:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 2px var(--accent-soft);
	}

	.hint {
		font-size: 12px;
		color: var(--muted);
	}

	.sessiond-actions {
		margin-top: 16px;
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 14px;
		border-radius: 10px;
		border: 1px dashed var(--border);
		background: rgba(255, 255, 255, 0.02);
	}

	.sessiond-actions .hint {
		flex: 1;
	}

	.restart {
		background: var(--panel-strong);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-md);
		padding: 6px 12px;
		font-size: 12px;
		cursor: pointer;
		white-space: nowrap;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		transition:
			border-color var(--transition-fast),
			color var(--transition-fast),
			transform var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.restart:hover {
		border-color: var(--accent);
	}

	.restart:active {
		transform: scale(0.96) translateY(1px);
	}

	.restart.restarting {
		border-color: var(--warning);
		color: var(--warning);
	}

	.spin-icon {
		display: inline-block;
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

	.sessiond-actions.restart-completed {
		animation: containerPulse 1.2s ease-out;
		border-color: var(--success-soft);
		border-style: solid;
	}

	@keyframes containerPulse {
		0% {
			box-shadow: 0 0 0 0 rgba(var(--success-rgb), 0.4);
		}
		50% {
			box-shadow: 0 0 16px 6px rgba(var(--success-rgb), 0.15);
			border-color: var(--success);
		}
		100% {
			box-shadow: 0 0 0 0 rgba(var(--success-rgb), 0);
		}
	}

	@media (max-width: 600px) {
		.compact-field {
			flex-direction: column;
			align-items: flex-start;
			gap: 6px;
		}

		.compact-field label {
			min-width: unset;
		}

		.input-row {
			flex-direction: column;
			align-items: flex-start;
			width: 100%;
		}

		.input-row :global(.select-wrapper) {
			width: 100%;
		}

		.compact-field input {
			width: 100%;
		}
	}
</style>
