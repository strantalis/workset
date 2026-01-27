<script lang="ts">
  import type {SettingsDefaults} from '../../../types'
  import SettingsSection from '../SettingsSection.svelte'
  import Select from '../../ui/Select.svelte'

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
          <Select
            id={field.id}
            value={getValue(field.id)}
            options={field.options ?? []}
            onchange={(val) => onUpdate(field.id, val)}
          />
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

  .field p {
    margin: 0;
    font-size: 12px;
    color: var(--muted);
  }
</style>
