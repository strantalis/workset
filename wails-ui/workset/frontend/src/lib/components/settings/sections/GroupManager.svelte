<script lang="ts">
  import {onMount} from 'svelte'
  import {
    addGroupMember,
    createGroup,
    deleteGroup,
    getGroup,
    listAliases,
    listGroups,
    removeGroupMember,
    updateGroup
  } from '../../../api'
  import type {Alias, Group, GroupSummary} from '../../../types'
  import SettingsSection from '../SettingsSection.svelte'
  import GroupMemberRow from './GroupMemberRow.svelte'

  interface Props {
    onGroupCountChange: (count: number) => void;
  }

  let { onGroupCountChange }: Props = $props();

  let groups: GroupSummary[] = $state([])
  let aliases: Alias[] = $state([])
  let selectedGroup: Group | null = $state(null)
  let isNew = $state(false)
  let loading = $state(false)
  let error: string | null = $state(null)
  let success: string | null = $state(null)

  let formName = $state('')
  let formDescription = $state('')

  let addingMember = $state(false)
  let memberRepo = $state('')
  let memberBaseRemote = $state('origin')
  let memberBaseBranch = $state('main')
  let memberWriteRemote = $state('origin')

  let expandedMember: string | null = $state(null)

  const formatError = (err: unknown): string => {
    if (err instanceof Error) return err.message
    return 'An error occurred.'
  }

  const loadGroups = async (): Promise<void> => {
    try {
      groups = await listGroups()
      aliases = await listAliases()
      onGroupCountChange(groups.length)
    } catch (err) {
      error = formatError(err)
    }
  }

  const selectGroup = async (summary: GroupSummary): Promise<void> => {
    try {
      selectedGroup = await getGroup(summary.name)
      isNew = false
      formName = selectedGroup.name
      formDescription = selectedGroup.description ?? ''
      addingMember = false
      expandedMember = null
      error = null
      success = null
    } catch (err) {
      error = formatError(err)
    }
  }

  const startNew = (): void => {
    selectedGroup = null
    isNew = true
    formName = ''
    formDescription = ''
    addingMember = false
    expandedMember = null
    error = null
    success = null
  }

  const cancelEdit = (): void => {
    if (groups.length > 0) {
      void selectGroup(groups[0])
    } else {
      selectedGroup = null
      isNew = false
      formName = ''
      formDescription = ''
    }
    error = null
    success = null
  }

  const handleSave = async (): Promise<void> => {
    const name = formName.trim()
    const description = formDescription.trim()

    if (!name) {
      error = 'Group name is required.'
      return
    }

    loading = true
    error = null
    success = null

    try {
      if (isNew) {
        await createGroup(name, description)
        success = `Created ${name}.`
      } else {
        await updateGroup(name, description)
        success = `Updated ${name}.`
      }
      await loadGroups()
      const summary = groups.find((g) => g.name === name)
      if (summary) {
        await selectGroup(summary)
      }
    } catch (err) {
      error = formatError(err)
    } finally {
      loading = false
    }
  }

  const handleDelete = async (): Promise<void> => {
    if (!selectedGroup) return

    const name = selectedGroup.name
    loading = true
    error = null
    success = null

    try {
      await deleteGroup(name)
      success = `Deleted ${name}.`
      await loadGroups()
      if (groups.length > 0) {
        await selectGroup(groups[0])
      } else {
        selectedGroup = null
        isNew = false
        formName = ''
        formDescription = ''
      }
    } catch (err) {
      error = formatError(err)
    } finally {
      loading = false
    }
  }

  const startAddMember = (): void => {
    addingMember = true
    memberRepo = ''
    memberBaseRemote = 'origin'
    memberBaseBranch = 'main'
    memberWriteRemote = 'origin'
    expandedMember = null
  }

  const cancelAddMember = (): void => {
    addingMember = false
  }

  const handleRepoInput = (event: Event): void => {
    const target = event.target as HTMLInputElement | null
    const value = target?.value ?? ''
    memberRepo = value

    // Auto-fill from alias if matched
    const alias = aliases.find((a) => a.name === value)
    if (alias?.default_branch) {
      memberBaseBranch = alias.default_branch
    }
  }

  const handleAddMember = async (): Promise<void> => {
    if (!selectedGroup) return

    const repo = memberRepo.trim()
    if (!repo) {
      error = 'Repo name is required.'
      return
    }

    loading = true
    error = null
    success = null

    try {
      await addGroupMember(
        selectedGroup.name,
        repo,
        memberBaseRemote.trim() || 'origin',
        memberBaseBranch.trim() || 'main',
        memberWriteRemote.trim() || 'origin'
      )
      success = `Added ${repo}.`
      selectedGroup = await getGroup(selectedGroup.name)
      addingMember = false
    } catch (err) {
      error = formatError(err)
    } finally {
      loading = false
    }
  }

  const handleUpdateMember = async (
    repo: string,
    baseRemote: string,
    baseBranch: string,
    writeRemote: string
  ): Promise<void> => {
    if (!selectedGroup) return

    loading = true
    error = null
    success = null

    try {
      await removeGroupMember(selectedGroup.name, repo)
      await addGroupMember(selectedGroup.name, repo, baseRemote, baseBranch, writeRemote)
      success = `Updated ${repo}.`
      selectedGroup = await getGroup(selectedGroup.name)
      expandedMember = null
    } catch (err) {
      error = formatError(err)
    } finally {
      loading = false
    }
  }

  const handleRemoveMember = async (repo: string): Promise<void> => {
    if (!selectedGroup) return

    loading = true
    error = null
    success = null

    try {
      await removeGroupMember(selectedGroup.name, repo)
      success = `Removed ${repo}.`
      selectedGroup = await getGroup(selectedGroup.name)
      expandedMember = null
    } catch (err) {
      error = formatError(err)
    } finally {
      loading = false
    }
  }

  const toggleMember = (repo: string): void => {
    expandedMember = expandedMember === repo ? null : repo
    addingMember = false
  }

  onMount(() => {
    void loadGroups()
  })
</script>

<SettingsSection
  title="Groups"
  description="Collections of repos with preset remotes. Apply a group to quickly add multiple repos to a workspace."
>
  <div class="manager">
    <div class="list-header">
      <span class="list-count">{groups.length} group{groups.length === 1 ? '' : 's'}</span>
      <button class="ghost small" type="button" onclick={startNew}>+ New</button>
    </div>

    {#if groups.length > 0 || isNew}
      <div class="list">
        {#each groups as group}
          <button
            class="list-item"
            class:active={selectedGroup?.name === group.name && !isNew}
            type="button"
            onclick={() => selectGroup(group)}
          >
            <span class="item-name">{group.name}</span>
            <span class="item-count">({group.repo_count} repo{group.repo_count === 1 ? '' : 's'})</span>
          </button>
        {/each}
        {#if isNew}
          <button class="list-item active" type="button">
            <span class="item-name new">New group</span>
          </button>
        {/if}
      </div>

      {#if error}
        <div class="message error">{error}</div>
      {:else if success}
        <div class="message success">{success}</div>
      {/if}

      <div class="detail">
        <div class="detail-header">
          {#if isNew}
            New group
          {:else if selectedGroup}
            {selectedGroup.name}
          {/if}
        </div>
        <div class="form">
          <label class="field">
            <span>Name</span>
            <input
              type="text"
              bind:value={formName}
              placeholder="core-services"
              disabled={!isNew && !!selectedGroup}
            />
          </label>
          <label class="field">
            <span>Description</span>
            <input
              type="text"
              bind:value={formDescription}
              placeholder="Core backend microservices"
            />
          </label>
        </div>
        <div class="actions">
          {#if !isNew && selectedGroup}
            <button class="danger" type="button" onclick={handleDelete} disabled={loading}>
              Delete group
            </button>
          {/if}
          <div class="spacer"></div>
          {#if isNew}
            <button class="ghost" type="button" onclick={cancelEdit} disabled={loading}>
              Cancel
            </button>
          {/if}
          <button class="primary" type="button" onclick={handleSave} disabled={loading}>
            {loading ? 'Saving...' : isNew ? 'Create group' : 'Save group'}
          </button>
        </div>

        {#if selectedGroup && !isNew}
          <div class="members-section">
            <div class="members-header">
              <span class="members-label">
                Members ({selectedGroup.members.length})
              </span>
              <button class="ghost small" type="button" onclick={startAddMember}>
                + Add repo
              </button>
            </div>

            {#if addingMember}
              <div class="add-member-form">
                <div class="form-row">
                  <label class="field">
                    <span>Repo name (or alias)</span>
                    <input
                      type="text"
                      value={memberRepo}
                      oninput={handleRepoInput}
                      placeholder="auth-api"
                      list="alias-options"
                    />
                    <datalist id="alias-options">
                      {#each aliases as alias}
                        <option value={alias.name}></option>
                      {/each}
                    </datalist>
                  </label>
                </div>
                <div class="form-row">
                  <label class="field">
                    <span>Base remote</span>
                    <input type="text" bind:value={memberBaseRemote} placeholder="origin" />
                  </label>
                  <label class="field">
                    <span>Base branch</span>
                    <input type="text" bind:value={memberBaseBranch} placeholder="main" />
                  </label>
                  <label class="field">
                    <span>Write remote</span>
                    <input type="text" bind:value={memberWriteRemote} placeholder="origin" />
                  </label>
                </div>
                <div class="add-member-actions">
                  <button class="ghost small" type="button" onclick={cancelAddMember} disabled={loading}>
                    Cancel
                  </button>
                  <button class="primary small" type="button" onclick={handleAddMember} disabled={loading}>
                    {loading ? 'Adding...' : 'Add repo'}
                  </button>
                </div>
              </div>
            {/if}

            <div class="members-list">
              {#each selectedGroup.members as member}
                <GroupMemberRow
                  {member}
                  expanded={expandedMember === member.repo}
                  {loading}
                  onToggle={() => toggleMember(member.repo)}
                  onSave={(base, branch, write) => handleUpdateMember(member.repo, base, branch, write)}
                  onRemove={() => handleRemoveMember(member.repo)}
                />
              {/each}
              {#if selectedGroup.members.length === 0 && !addingMember}
                <div class="empty-members">
                  No members yet. Add repos to this group.
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    {:else}
      <div class="empty">
        <p>No groups defined yet.</p>
        <button class="ghost" type="button" onclick={startNew}>Create your first group</button>
      </div>
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
    max-height: 160px;
    overflow-y: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 4px;
    background: var(--panel);
  }

  .list-item {
    display: flex;
    align-items: center;
    gap: 8px;
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

  .item-count {
    font-size: 12px;
    color: var(--muted);
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

  .members-section {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--border);
  }

  .members-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .members-label {
    font-size: 13px;
    font-weight: 600;
  }

  .members-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    max-height: 240px;
    overflow-y: auto;
  }

  .empty-members {
    padding: 16px;
    text-align: center;
    font-size: 13px;
    color: var(--muted);
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
  }

  .add-member-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 12px;
    background: var(--panel);
    border: 1px solid var(--accent-soft);
    border-radius: var(--radius-md);
  }

  .form-row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 12px;
  }

  .add-member-actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
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

  .danger:hover:not(:disabled) {
    background: var(--danger-soft);
    border-color: var(--danger);
  }

  .danger:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
