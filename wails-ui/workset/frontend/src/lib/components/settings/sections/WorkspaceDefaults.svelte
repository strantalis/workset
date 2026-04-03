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

	const threadFields: Field[] = [
		{
			id: 'thread',
			label: 'Default thread name',
			description: 'Name used when you do not specify one.',
			placeholder: 'acme',
		},
		{
			id: 'remote',
			label: 'Default remote',
			description: 'Primary remote for repos without an alias override.',
			placeholder: 'origin',
		},
		{
			id: 'baseBranch',
			label: 'Base branch',
			description: 'Fallback branch for repos that do not specify one.',
			placeholder: 'main',
		},
	];

	const agentFields: Field[] = [
		{
			id: 'agent',
			label: 'Preferred agent',
			description:
				'Used for PR title/description generation and commit messages; also the default coding agent for the terminal launcher.',
			type: 'select',
			options: [
				{ label: 'Codex (Default)', value: 'codex' },
				{ label: 'Claude Code', value: 'claude' },
			],
		},
		{
			id: 'agentModel',
			label: 'Model override',
			description:
				'Optional model for PR and commit message generation only. Leave blank to use the agent default.',
			type: 'text',
		},
	];

	const getValue = (id: FieldId): string => draft[id] ?? '';

	const isChanged = (id: FieldId): boolean => draft[id] !== baseline[id];

	const handleInput = (id: FieldId, event: Event): void => {
		const target = event.target as HTMLInputElement | null;
		onUpdate(id, target?.value ?? '');
	};
</script>

<SettingsSection title="Defaults" description="Applied when creating threads and running agents.">
	<!-- Thread defaults -->
	<div class="subsection">
		<h3 class="subsection-title">Thread</h3>
		<div class="fields-grid">
			{#each threadFields as field (field.id)}
				<div class="field" class:changed={isChanged(field.id)}>
					<label for={field.id}>{field.label}</label>
					<input
						class="ws-field-input ws-field-input--strong-bg"
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
					<p class="ws-hint" id="{field.id}-hint">{field.description}</p>
				</div>
			{/each}
		</div>
	</div>

	<!-- Agent preferences -->
	<div class="subsection">
		<h3 class="subsection-title">Agent</h3>
		<div class="fields-grid">
			{#each agentFields as field (field.id)}
				<div class="field" class:changed={isChanged(field.id)}>
					<label for={field.id}>{field.label}</label>
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
							class="ws-field-input ws-field-input--strong-bg"
							id={field.id}
							type="text"
							placeholder={field.placeholder ?? 'e.g. gpt-4-0125-preview'}
							value={getValue(field.id)}
							spellcheck="false"
							autocomplete="off"
							aria-describedby="{field.id}-hint"
							oninput={(event) => handleInput(field.id, event)}
						/>
					{/if}
					<p class="ws-hint" id="{field.id}-hint">{field.description}</p>
				</div>
			{/each}
		</div>
	</div>
</SettingsSection>

<style>
	.subsection {
		padding-top: 16px;
	}

	.subsection + .subsection {
		margin-top: 8px;
		border-top: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
	}

	.subsection-title {
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--text);
		margin: 0 0 8px 0;
		letter-spacing: 0.01em;
	}

	.fields-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 16px;
	}

	.field.changed label::after {
		content: '*';
		color: var(--warning);
		margin-left: 4px;
	}

	.field label {
		font-size: var(--text-sm);
		font-weight: 500;
		color: var(--muted);
		display: block;
		margin-bottom: 4px;
	}

	@media (max-width: 600px) {
		.fields-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
