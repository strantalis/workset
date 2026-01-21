<script lang="ts">
  import {onMount} from 'svelte'
  import {fetchSettings, setDefaultSetting} from '../api'
  import type {SettingsDefaults, SettingsSnapshot} from '../types'
  import SettingsSidebar from './settings/SettingsSidebar.svelte'
  import WorkspaceDefaults from './settings/sections/WorkspaceDefaults.svelte'
  import SessionDefaults from './settings/sections/SessionDefaults.svelte'
  import AliasManager from './settings/sections/AliasManager.svelte'
  import GroupManager from './settings/sections/GroupManager.svelte'

  export let onClose: () => void

  type FieldId = keyof SettingsDefaults
  type Field = {
    id: FieldId
    key: string
  }

  const allFields: Field[] = [
    {id: 'workspace', key: 'defaults.workspace'},
    {id: 'baseBranch', key: 'defaults.base_branch'},
    {id: 'workspaceRoot', key: 'defaults.workspace_root'},
    {id: 'repoStoreRoot', key: 'defaults.repo_store_root'},
    {id: 'sessionBackend', key: 'defaults.session_backend'},
    {id: 'sessionNameFormat', key: 'defaults.session_name_format'},
    {id: 'sessionTheme', key: 'defaults.session_theme'},
    {id: 'sessionTmuxStyle', key: 'defaults.session_tmux_status_style'},
    {id: 'sessionTmuxLeft', key: 'defaults.session_tmux_status_left'},
    {id: 'sessionTmuxRight', key: 'defaults.session_tmux_status_right'},
    {id: 'sessionScreenHard', key: 'defaults.session_screen_hardstatus'},
    {id: 'agent', key: 'defaults.agent'}
  ]

  let snapshot: SettingsSnapshot | null = null
  let loading = true
  let saving = false
  let error: string | null = null
  let success: string | null = null
  let baseline: Record<FieldId, string> = {} as Record<FieldId, string>
  let draft: Record<FieldId, string> = {} as Record<FieldId, string>

  let activeSection = 'workspace'
  let aliasCount = 0
  let groupCount = 0

  const formatError = (err: unknown): string => {
    if (err instanceof Error) {
      return err.message
    }
    return 'Failed to update settings.'
  }

  const buildDraft = (defaults: SettingsDefaults): void => {
    const next: Record<FieldId, string> = {} as Record<FieldId, string>
    allFields.forEach((field) => {
      next[field.id] = defaults[field.id] ?? ''
    })
    baseline = next
    draft = {...next}
  }

  const updateField = (id: FieldId, value: string): void => {
    draft = {...draft, [id]: value}
  }

  const changedFields = (): Field[] =>
    allFields.filter((field) => draft[field.id] !== baseline[field.id])

  const dirtyCount = (): number => changedFields().length

  const loadSettings = async (): Promise<void> => {
    loading = true
    error = null
    success = null
    try {
      const data = await fetchSettings()
      snapshot = data
      buildDraft(data.defaults)
    } catch (err) {
      error = formatError(err)
    } finally {
      loading = false
    }
  }

  const saveChanges = async (): Promise<void> => {
    if (saving || !snapshot) {
      return
    }
    const updates = changedFields()
    if (updates.length === 0) {
      success = 'No changes to save.'
      return
    }
    saving = true
    error = null
    success = null
    for (const field of updates) {
      try {
        await setDefaultSetting(field.key, draft[field.id] ?? '')
      } catch (err) {
        error = `Failed to save: ${formatError(err)}`
        break
      }
    }
    if (!error) {
      baseline = {...draft}
      success = `Saved ${updates.length} change${updates.length === 1 ? '' : 's'}.`
    }
    saving = false
  }

  const resetChanges = (): void => {
    draft = {...baseline}
    success = null
    error = null
  }

  const selectSection = (section: string): void => {
    activeSection = section
    success = null
    error = null
  }

  onMount(() => {
    void loadSettings()
  })
</script>

<section class="panel" role="dialog" aria-modal="true" aria-label="Settings">
  <header class="header">
    <div>
      <div class="title">Settings</div>
      <div class="subtitle">Configure defaults, aliases, and groups.</div>
    </div>
    <button class="ghost" type="button" on:click={onClose}>Close</button>
  </header>

  {#if loading}
    <div class="state">Loading settings...</div>
  {:else if error && !snapshot}
    <div class="state error">
      <div class="message">{error}</div>
      <button class="ghost" type="button" on:click={loadSettings}>Retry</button>
    </div>
  {:else if snapshot}
    <div class="body">
      <SettingsSidebar
        {activeSection}
        onSelectSection={selectSection}
        {aliasCount}
        {groupCount}
      />

      <div class="content">
        {#if activeSection === 'workspace'}
          <WorkspaceDefaults {draft} {baseline} onUpdate={updateField} />
        {:else if activeSection === 'session'}
          <SessionDefaults {draft} {baseline} onUpdate={updateField} />
        {:else if activeSection === 'aliases'}
          <AliasManager onAliasCountChange={(count) => (aliasCount = count)} />
        {:else if activeSection === 'groups'}
          <GroupManager onGroupCountChange={(count) => (groupCount = count)} />
        {/if}
      </div>
    </div>

    <footer class="footer">
      <div class="meta">
        <span class="config-label">Config</span>
        <span class="config-path">{snapshot.configPath}</span>
      </div>
      <div class="spacer"></div>
      {#if error}
        <span class="status error">{error}</span>
      {:else if success}
        <span class="status success">{success}</span>
      {:else if dirtyCount() > 0}
        <span class="status dirty">{dirtyCount()} unsaved</span>
      {/if}
      {#if activeSection === 'workspace' || activeSection === 'session'}
        <button
          class="ghost"
          type="button"
          on:click={resetChanges}
          disabled={dirtyCount() === 0 || saving}
        >
          Reset
        </button>
        <button
          class="primary"
          type="button"
          on:click={saveChanges}
          disabled={saving || dirtyCount() === 0}
        >
          {saving ? 'Saving...' : 'Save'}
        </button>
      {/if}
    </footer>
  {/if}
</section>

<style>
  .panel {
    width: min(960px, 94vw);
    max-height: 86vh;
    display: flex;
    flex-direction: column;
    background: var(--panel-strong);
    border: 1px solid rgba(255, 255, 255, 0.06);
    border-radius: 20px;
    box-shadow: 0 20px 60px rgba(5, 10, 18, 0.55);
    overflow: hidden;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 20px 24px;
    border-bottom: 1px solid var(--border);
  }

  .title {
    font-size: 20px;
    font-weight: 600;
    font-family: var(--font-display);
  }

  .subtitle {
    color: var(--muted);
    font-size: 13px;
    margin-top: 4px;
  }

  .state {
    padding: 24px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .state.error {
    color: var(--warning);
  }

  .body {
    display: flex;
    flex: 1;
    min-height: 0;
    padding: 20px 24px;
    gap: 24px;
  }

  .content {
    flex: 1;
    min-width: 0;
    overflow-y: auto;
    padding-right: 4px;
  }

  .footer {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px 24px;
    border-top: 1px solid var(--border);
    background: var(--panel);
  }

  .meta {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .config-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
  }

  .config-path {
    font-size: 12px;
    color: var(--text);
    opacity: 0.7;
  }

  .spacer {
    flex: 1;
  }

  .status {
    font-size: 12px;
    font-weight: 500;
    padding: 4px 10px;
    border-radius: 999px;
  }

  .status.dirty {
    background: rgba(234, 179, 8, 0.15);
    color: var(--warning);
  }

  .status.success {
    background: rgba(74, 222, 128, 0.15);
    color: var(--success);
  }

  .status.error {
    background: var(--danger-subtle);
    color: var(--danger);
  }

  .ghost {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--border);
    color: var(--text);
    padding: 8px 14px;
    border-radius: var(--radius-md);
    cursor: pointer;
    font-size: 13px;
    transition: border-color var(--transition-fast), background var(--transition-fast);
  }

  .ghost:hover:not(:disabled) {
    border-color: var(--accent);
    background: rgba(255, 255, 255, 0.04);
  }

  .ghost:active:not(:disabled) {
    transform: scale(0.98);
  }

  .ghost:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .primary {
    background: var(--accent);
    border: none;
    color: #081018;
    padding: 8px 14px;
    border-radius: var(--radius-md);
    font-weight: 600;
    cursor: pointer;
    transition: background var(--transition-fast), transform var(--transition-fast);
  }

  .primary:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 85%, white);
  }

  .primary:active:not(:disabled) {
    transform: scale(0.98);
  }

  .primary:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  @media (max-width: 720px) {
    .panel {
      width: 100%;
      height: 100%;
      border-radius: 0;
      max-height: 100vh;
    }

    .body {
      flex-direction: column;
    }

    .meta {
      display: none;
    }
  }
</style>
