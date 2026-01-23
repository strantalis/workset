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
    placeholder?: string
  }

  const fields: Field[] = [
    {
      id: 'workspace',
      label: 'Default workspace name',
      description: 'Used when a workspace name is not provided.',
      placeholder: 'acme'
    },
    {
      id: 'remote',
      label: 'Default remote',
      description: 'Primary remote for repos without an alias override.',
      placeholder: 'origin'
    },
    {
      id: 'baseBranch',
      label: 'Base branch',
      description: 'Fallback branch for repos that do not specify one.',
      placeholder: 'main'
    },
    {
      id: 'workspaceRoot',
      label: 'Workspace root',
      description: 'Root folder for workspace checkouts.',
      placeholder: '~/workspaces'
    },
    {
      id: 'repoStoreRoot',
      label: 'Repo store root',
      description: 'Local mirror cache for repo cloning.',
      placeholder: '~/.workset/repos'
    }
  ]

  const getValue = (id: FieldId): string => draft[id] ?? ''

  const isChanged = (id: FieldId): boolean => draft[id] !== baseline[id]

  const handleInput = (id: FieldId, event: Event): void => {
    const target = event.target as HTMLInputElement | null
    onUpdate(id, target?.value ?? '')
  }
</script>

<SettingsSection
  title="Workspace defaults"
  description="Defaults used when creating or importing workspaces."
>
  <div class="fields">
    {#each fields as field}
      <div class="field" class:changed={isChanged(field.id)}>
        <label for={field.id}>{field.label}</label>
        <input
          id={field.id}
          type="text"
          placeholder={field.placeholder ?? ''}
          value={getValue(field.id)}
          oninput={(event) => handleInput(field.id, event)}
        />
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

  .field input {
    background: var(--panel-strong);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--text);
    border-radius: var(--radius-md);
    padding: 10px 12px;
    font-size: 13px;
    transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  }

  .field input:focus {
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
