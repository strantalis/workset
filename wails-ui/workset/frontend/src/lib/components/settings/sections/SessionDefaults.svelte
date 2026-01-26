<script lang="ts">
  import type {SettingsDefaults} from '../../../types'
  import SettingsSection from '../SettingsSection.svelte'

  type FieldId = keyof SettingsDefaults

  interface Props {
    draft: Record<FieldId, string>;
    baseline: Record<FieldId, string>;
    onUpdate: (id: FieldId, value: string) => void;
    onRestartSessiond: () => void;
    restartingSessiond?: boolean;
  }

  let { draft, baseline, onUpdate, onRestartSessiond, restartingSessiond = false }: Props = $props();

  type Field = {
    id: FieldId
    label: string
    description: string
    placeholder?: string
    type?: 'text' | 'select'
    options?: {label: string; value: string}[]
  }

  const fields: Field[] = [
    {
      id: 'sessionBackend',
      label: 'Session backend',
      description: 'auto, tmux, screen, or exec.',
      type: 'select',
      options: [
        {label: 'auto', value: 'auto'},
        {label: 'tmux', value: 'tmux'},
        {label: 'screen', value: 'screen'},
        {label: 'exec', value: 'exec'}
      ]
    },
    {
      id: 'agent',
      label: 'Preferred agent',
      description: 'Default coding agent for the terminal launcher.',
      type: 'select',
      options: [
        {label: 'Codex', value: 'codex'},
        {label: 'Claude Code', value: 'claude'},
        {label: 'OpenCode', value: 'opencode'},
        {label: 'Pi', value: 'pi'},
        {label: 'Cursor Agent', value: 'cursor'}
      ]
    },
    {
      id: 'terminalRenderer',
      label: 'Terminal renderer',
      description: 'Auto picks WebGL when healthy, otherwise Canvas.',
      type: 'select',
      options: [
        {label: 'Auto', value: 'auto'},
        {label: 'WebGL', value: 'webgl'},
        {label: 'Canvas', value: 'canvas'}
      ]
    },
    {
      id: 'terminalIdleTimeout',
      label: 'Terminal idle timeout',
      description: 'How long before idle terminals are closed. Use 0 to disable.',
      placeholder: '30m'
    },
    {
      id: 'sessionNameFormat',
      label: 'Session name format',
      description: 'Used when creating a new session.',
      placeholder: 'ws-{workspace}'
    },
    {
      id: 'sessionTheme',
      label: 'Session theme',
      description: 'Applied to tmux or screen sessions.',
      placeholder: 'dark'
    },
    {
      id: 'sessionTmuxStyle',
      label: 'Tmux status style',
      description: 'Status bar style string.',
      placeholder: 'fg=white,bg=black'
    },
    {
      id: 'sessionTmuxLeft',
      label: 'Tmux status left',
      description: 'Left status content.',
      placeholder: '#S'
    },
    {
      id: 'sessionTmuxRight',
      label: 'Tmux status right',
      description: 'Right status content.',
      placeholder: '%Y-%m-%d %H:%M'
    },
    {
      id: 'sessionScreenHard',
      label: 'Screen hardstatus',
      description: 'Hardstatus format for screen.',
      placeholder: '%{= kG}%H'
    }
  ]

  const getValue = (id: FieldId): string => draft[id] ?? ''

  const isChanged = (id: FieldId): boolean => draft[id] !== baseline[id]

  const handleInput = (id: FieldId, event: Event): void => {
    const target = event.target as HTMLInputElement | null
    onUpdate(id, target?.value ?? '')
  }

  const handleSelect = (id: FieldId, event: Event): void => {
    const target = event.target as HTMLSelectElement | null
    onUpdate(id, target?.value ?? '')
  }
</script>

<SettingsSection
  title="Session defaults"
  description="How workset creates and names terminal sessions."
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
        {:else}
          <input
            id={field.id}
            type="text"
            placeholder={field.placeholder ?? ''}
            value={getValue(field.id)}
            oninput={(event) => handleInput(field.id, event)}
          />
        {/if}
        <p>{field.description}</p>
      </div>
    {/each}
  </div>
</SettingsSection>
<div class="sessiond-actions">
  <div class="hint">
    Restart the session daemon after changing idle timeout or other daemon settings.
  </div>
  <button class="restart" type="button" onclick={onRestartSessiond} disabled={restartingSessiond}>
    {restartingSessiond ? 'Restarting session daemon…' : 'Restart session daemon'}
  </button>
</div>

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

  .field input,
  .field select {
    background: var(--panel-strong);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--text);
    border-radius: var(--radius-md);
    padding: 10px 12px;
    font-size: 13px;
    transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  }

  .field input:focus,
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

  .sessiond-actions {
    margin-top: 16px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 12px 14px;
    border-radius: 12px;
    border: 1px dashed var(--border);
    background: rgba(255, 255, 255, 0.02);
  }

  .sessiond-actions .hint {
    font-size: 12px;
    color: var(--muted);
  }

  .restart {
    background: var(--panel-strong);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: var(--radius-md);
    padding: 8px 12px;
    font-size: 12px;
    cursor: pointer;
    transition: border-color var(--transition-fast), color var(--transition-fast), transform var(--transition-fast);
  }

  .restart:hover {
    border-color: var(--accent);
    color: var(--text);
  }

  .restart:active {
    transform: scale(0.98);
  }
</style>
