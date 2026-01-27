<script lang="ts">
  import type {SettingsDefaults} from '../../../types'
  import SettingsSection from '../SettingsSection.svelte'

  type FieldId = keyof SettingsDefaults

  interface Props {
    draft: Record<FieldId, string>;
    baseline: Record<FieldId, string>;
    onUpdate: (id: FieldId, value: string) => void;
  }

  let { draft, baseline, onUpdate }: Props = $props();

  type Field = {
    id: FieldId
    label: string
    description: string
    type?: 'text' | 'select'
    options?: {label: string; value: string}[]
  }

  const fields: Field[] = [
    {
      id: 'agent',
      label: 'Preferred agent',
      description: 'Used for PR title/description generation and commit messages; also the default coding agent for the terminal launcher.',
      type: 'select',
      options: [
        {label: 'Codex', value: 'codex'},
        {label: 'Claude Code', value: 'claude'},
        {label: 'OpenCode', value: 'opencode'},
        {label: 'Pi', value: 'pi'},
        {label: 'Cursor Agent', value: 'cursor'}
      ]
    }
  ]

  const getValue = (id: FieldId): string => draft[id] ?? ''

  const isChanged = (id: FieldId): boolean => draft[id] !== baseline[id]

  const handleSelect = (id: FieldId, event: Event): void => {
    const target = event.target as HTMLSelectElement | null
    onUpdate(id, target?.value ?? '')
  }
</script>

<SettingsSection
  title="Agent defaults"
  description="Choose which assistant Workset uses for generation tasks."
>
  <div class="fields">
    {#each fields as field}
      <div class="field" class:changed={isChanged(field.id)}>
        <label for={field.id}>{field.label}</label>
        {#if field.type === 'select'}
          <select
            id={field.id}
            value={getValue(field.id)}
            onchange={(event) => handleSelect(field.id, event)}
          >
            {#each field.options ?? [] as option}
              <option value={option.value}>{option.label}</option>
            {/each}
          </select>
        {/if}
        <p>{field.description}</p>
      </div>
    {/each}
  </div>
</SettingsSection>

<style>
  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 16px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .field.changed label::after {
    content: '*';
    color: var(--warning);
    margin-left: 4px;
  }

  .field label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: rgba(255, 255, 255, 0.7);
  }

  .field select {
    background: var(--panel-strong);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--text);
    border-radius: var(--radius-md);
    padding: 10px 12px;
    font-size: 13px;
    transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  }

  .field select:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-soft);
  }

  .field p {
    margin: 0;
    font-size: 12px;
    color: var(--muted);
  }
</style>
