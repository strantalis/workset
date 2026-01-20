<script lang="ts">
  import type {GroupMember} from '../../../types'

  export let member: GroupMember
  export let expanded: boolean = false
  export let loading: boolean = false
  export let onToggle: () => void
  export let onSave: (baseRemote: string, baseBranch: string, writeRemote: string) => void
  export let onRemove: () => void

  let baseRemote = member.remotes.base.name
  let baseBranch = member.remotes.base.default_branch ?? ''
  let writeRemote = member.remotes.write.name

  const handleSave = (): void => {
    onSave(baseRemote.trim(), baseBranch.trim(), writeRemote.trim())
  }

  $: if (!expanded) {
    baseRemote = member.remotes.base.name
    baseBranch = member.remotes.base.default_branch ?? ''
    writeRemote = member.remotes.write.name
  }
</script>

<div class="member" class:expanded>
  <button class="member-header" type="button" on:click={onToggle}>
    <span class="member-name">{member.repo}</span>
    <span class="member-remotes">
      {member.remotes.base.name}/{member.remotes.base.default_branch ?? 'main'}
      {#if member.remotes.write.name !== member.remotes.base.name}
        → {member.remotes.write.name}
      {/if}
    </span>
    <span class="member-toggle">{expanded ? '▾' : '▸'}</span>
  </button>

  {#if expanded}
    <div class="member-detail">
      <div class="form-row">
        <label class="field">
          <span>Base remote</span>
          <input type="text" bind:value={baseRemote} placeholder="origin" />
        </label>
        <label class="field">
          <span>Base branch</span>
          <input type="text" bind:value={baseBranch} placeholder="main" />
        </label>
        <label class="field">
          <span>Write remote</span>
          <input type="text" bind:value={writeRemote} placeholder="origin" />
        </label>
      </div>
      <div class="member-actions">
        <button class="danger small" type="button" on:click={onRemove} disabled={loading}>
          Remove
        </button>
        <div class="spacer"></div>
        <button class="ghost small" type="button" on:click={onToggle} disabled={loading}>
          Cancel
        </button>
        <button class="primary small" type="button" on:click={handleSave} disabled={loading}>
          {loading ? 'Saving...' : 'Save'}
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .member {
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    background: var(--panel);
    overflow: hidden;
  }

  .member.expanded {
    background: var(--panel-soft);
  }

  .member-header {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 10px 12px;
    border: none;
    background: transparent;
    color: var(--text);
    font-size: 13px;
    text-align: left;
    cursor: pointer;
    transition: background var(--transition-fast);
  }

  .member-header:hover {
    background: rgba(255, 255, 255, 0.04);
  }

  .member-name {
    font-weight: 500;
    flex-shrink: 0;
  }

  .member-remotes {
    flex: 1;
    font-size: 12px;
    color: var(--muted);
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
  }

  .member-toggle {
    font-size: 10px;
    color: var(--muted);
  }

  .member-detail {
    padding: 12px;
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .form-row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 12px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 11px;
    color: var(--muted);
  }

  .field input {
    background: var(--panel-strong);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--text);
    border-radius: var(--radius-sm);
    padding: 8px 10px;
    font-size: 12px;
    transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  }

  .field input:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-soft);
  }

  .member-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .spacer {
    flex: 1;
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

  .primary.small {
    padding: 6px 10px;
    font-size: 12px;
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

  .danger.small {
    padding: 6px 10px;
    font-size: 12px;
  }

  .danger:hover:not(:disabled) {
    background: var(--danger-soft);
    border-color: var(--danger);
  }

  .danger:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
