<script lang="ts">
	import type { SettingsDefaultField } from '../../../types';
	import SettingsSection from '../SettingsSection.svelte';
	import Select from '../../ui/Select.svelte';

	type FieldId = SettingsDefaultField;

	interface Props {
		draft: Record<FieldId, string>;
		baseline: Record<FieldId, string>;
		onUpdate: (id: FieldId, value: string) => void;
	}

	const { draft, baseline, onUpdate }: Props = $props();

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
			id: 'terminalDebugOverlay',
			label: 'Debug overlay',
			description: 'Shows terminal debug stats like bytes in/out and CPR timing.',
			type: 'select',
			options: [
				{ label: 'Off', value: 'off' },
				{ label: 'On', value: 'on' },
			],
		},
		{
			id: 'terminalIdleTimeout',
			label: 'Idle timeout',
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

<SettingsSection title="Terminal" description="Preferences for the GUI terminal launcher.">
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
							aria-describedby="{field.id}-hint"
						/>
					{:else}
						<input
							class="ws-field-input"
							id={field.id}
							type="text"
							placeholder={field.placeholder ?? ''}
							value={getValue(field.id)}
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
							aria-describedby="{field.id}-hint"
							oninput={(event) => handleInput(field.id, event)}
						/>
					{/if}
					<span class="hint ws-hint" id="{field.id}-hint">{field.description}</span>
				</div>
			</div>
		{/each}
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
		font-size: var(--text-base);
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
		padding: 8px 12px;
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
