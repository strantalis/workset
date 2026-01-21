<script lang="ts">
  import {onMount} from 'svelte'
  import {createAlias, deleteAlias, listAliases, updateAlias} from '../../../api'
  import type {Alias} from '../../../types'
  import SettingsSection from '../SettingsSection.svelte'

  interface Props {
    onAliasCountChange: (count: number) => void;
  }

  let { onAliasCountChange }: Props = $props();

  let aliases: Alias[] = $state([])
  let selectedAlias: Alias | null = $state(null)
  let isNew = $state(false)
  let loading = $state(false)
  let error: string | null = $state(null)
  let success: string | null = $state(null)

  let formName = $state('')
  let formSource = $state('')
  let formBranch = $state('')

  const formatError = (err: unknown): string => {
    if (err instanceof Error) return err.message
    return 'An error occurred.'
  }

  const loadAliases = async (): Promise<void> => {
    try {
      aliases = await listAliases()
      onAliasCountChange(aliases.length)
    } catch (err) {
      error = formatError(err)
    }
  }

  const selectAlias = (alias: Alias): void => {
    selectedAlias = alias
    isNew = false
    formName = alias.name
    formSource = alias.url ?? alias.path ?? ''
    formBranch = alias.default_branch ?? ''
    error = null
    success = null
  }

  const startNew = (): void => {
    selectedAlias = null
    isNew = true
    formName = ''
    formSource = ''
    formBranch = ''
    error = null
    success = null
  }

  const cancelEdit = (): void => {
    selectedAlias = null
    isNew = false
    formName = ''
    formSource = ''
    formBranch = ''
    error = null
    success = null
  }

  const handleSave = async (): Promise<void> => {
    const name = formName.trim()
    const source = formSource.trim()
    const branch = formBranch.trim()

    if (!name) {
      error = 'Alias name is required.'
      return
    }
    if (!source) {
      error = 'Source URL or path is required.'
      return
    }

    loading = true
    error = null
    success = null

    try {
      if (isNew) {
        await createAlias(name, source, branch)
        success = `Created ${name}.`
      } else {
        await updateAlias(name, source, branch)
        success = `Updated ${name}.`
      }
      await loadAliases()
      const updated = aliases.find((a) => a.name === name)
      if (updated) {
        selectAlias(updated)
      }
    } catch (err) {
      error = formatError(err)
    } finally {
      loading = false
    }
  }

  const handleDelete = async (): Promise<void> => {
    if (!selectedAlias) return

    const name = selectedAlias.name
    loading = true
    error = null
    success = null

    try {
      await deleteAlias(name)
      success = `Deleted ${name}.`
      await loadAliases()
      selectedAlias = null
      isNew = false
      formName = ''
      formSource = ''
      formBranch = ''
    } catch (err) {
      error = formatError(err)
    } finally {
      loading = false
    }
  }

  const truncateSource = (alias: Alias): string => {
    const source = alias.url ?? alias.path ?? ''
    if (source.length > 40) {
      return source.substring(0, 37) + '...'
    }
    return source
  }

  onMount(() => {
    void loadAliases()
  })
</script>

<SettingsSection
  title="Aliases"
  description="Shorthand names for repository sources. Use aliases when adding repos to workspaces."
>
  <div class="manager">
    <div class="list-header">
      <span class="list-count">{aliases.length} alias{aliases.length === 1 ? '' : 'es'}</span>
      <button class="ghost small" type="button" onclick={startNew}>+ New</button>
    </div>

    {#if aliases.length > 0}
      <div class="list">
        {#each aliases as alias}
          <button
            class="list-item"
            class:active={selectedAlias?.name === alias.name && !isNew}
            type="button"
            onclick={() => selectAlias(alias)}
          >
            <span class="item-name">{alias.name}</span>
            <span class="item-source">{truncateSource(alias)}</span>
          </button>
        {/each}
        {#if isNew}
          <button class="list-item active" type="button">
            <span class="item-name new">New alias</span>
          </button>
        {/if}
      </div>
    {:else if !isNew}
      <div class="empty">
        <p>No aliases defined yet.</p>
        <button class="ghost" type="button" onclick={startNew}>Create your first alias</button>
      </div>
    {/if}

    {#if error}
      <div class="message error">{error}</div>
    {:else if success}
      <div class="message success">{success}</div>
    {/if}

    {#if isNew || selectedAlias}
      <div class="detail">
        <div class="detail-header">
          {#if isNew}
            New alias
          {:else if selectedAlias}
            Editing: {selectedAlias.name}
          {/if}
        </div>
        <div class="form">
          <label class="field">
            <span>Name</span>
            <input
              type="text"
              bind:value={formName}
              placeholder="repo-alias"
              disabled={!isNew && !!selectedAlias}
            />
          </label>
          <label class="field">
            <span>Source (URL or path)</span>
            <input
              type="text"
              bind:value={formSource}
              placeholder="git@github.com:org/repo.git"
            />
          </label>
          <label class="field">
            <span>Default branch</span>
            <input
              type="text"
              bind:value={formBranch}
              placeholder="main"
            />
          </label>
        </div>
        <div class="actions">
          {#if !isNew && selectedAlias}
            <button class="danger" type="button" onclick={handleDelete} disabled={loading}>
              Delete
            </button>
          {/if}
          <div class="spacer"></div>
          <button class="ghost" type="button" onclick={cancelEdit} disabled={loading}>
            Cancel
          </button>
          <button class="primary" type="button" onclick={handleSave} disabled={loading}>
            {loading ? 'Saving...' : isNew ? 'Create alias' : 'Save alias'}
          </button>
        </div>
      </div>
    {:else if aliases.length > 0}
      <div class="hint">Select an alias to edit, or click "+ New" to create one.</div>
    {/if}
  </div>
</SettingsSection>

<style>
  .manager {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .list-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .list-count {
    font-size: 12px;
    color: var(--muted);
  }

  .list {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 200px;
    overflow-y: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 4px;
    background: var(--panel);
  }

  .list-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 12px;
    border: none;
    background: transparent;
    color: var(--text);
    font-size: 13px;
    text-align: left;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background var(--transition-fast);
  }

  .list-item:hover {
    background: rgba(255, 255, 255, 0.04);
  }

  .list-item.active {
    background: rgba(255, 255, 255, 0.08);
  }

  .item-name {
    font-weight: 500;
  }

  .item-name.new {
    font-style: italic;
    color: var(--accent);
  }

  .item-source {
    font-size: 12px;
    color: var(--muted);
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
  }

  .detail {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px;
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
  }

  .detail-header {
    font-size: 14px;
    font-weight: 600;
    color: var(--muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 12px;
    color: var(--muted);
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

  .field input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-top: 8px;
    border-top: 1px solid var(--border);
  }

  .spacer {
    flex: 1;
  }

  .message {
    font-size: 13px;
    padding: 8px 12px;
    border-radius: var(--radius-md);
  }

  .message.error {
    background: var(--danger-subtle);
    color: var(--danger);
  }

  .message.success {
    background: rgba(74, 222, 128, 0.1);
    color: var(--success);
  }

  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 32px;
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    text-align: center;
  }

  .empty p {
    margin: 0;
    color: var(--muted);
    font-size: 14px;
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

  .ghost.small {
    padding: 6px 10px;
    font-size: 12px;
  }

  .ghost:hover:not(:disabled) {
    border-color: var(--accent);
    background: rgba(255, 255, 255, 0.04);
  }

  .ghost:disabled {
    opacity: 0.5;
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
    font-size: 13px;
    transition: background var(--transition-fast);
  }

  .primary:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 85%, white);
  }

  .primary:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .danger {
    background: var(--danger-subtle);
    border: 1px solid var(--danger-soft);
    color: #ff9a9a;
    padding: 8px 14px;
    border-radius: var(--radius-md);
    font-weight: 600;
    cursor: pointer;
    font-size: 13px;
    transition: background var(--transition-fast), border-color var(--transition-fast);
  }

  .danger:hover:not(:disabled) {
    background: var(--danger-soft);
    border-color: var(--danger);
  }

  .danger:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .hint {
    font-size: 13px;
    color: var(--muted);
    padding: 16px;
    text-align: center;
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
  }
</style>
