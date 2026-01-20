<script lang="ts">
  import {onMount, tick} from 'svelte'
  import {get} from 'svelte/store'
  import {
    addRepo,
    applyGroup,
    archiveWorkspace,
    createWorkspace,
    listAliases,
    listGroups,
    openDirectoryDialog,
    removeRepo,
    removeWorkspace,
    renameWorkspace,
    updateRepoRemotes
  } from '../api'
  import {activeWorkspaceId, clearRepo, clearWorkspace, loadWorkspaces, selectWorkspace, workspaces} from '../state'
  import type {Alias, GroupSummary, Repo, Workspace} from '../types'

  export let onClose: () => void
  export let mode:
    | 'create'
    | 'rename'
    | 'add-repo'
    | 'remotes'
    | 'archive'
    | 'remove-workspace'
    | 'remove-repo'
    | null
  export let workspaceId: string | null = null
  export let repoName: string | null = null

  let workspace: Workspace | null = null
  let repo: Repo | null = null

  let error: string | null = null
  let success: string | null = null
  let loading = false

  let nameInput: HTMLInputElement | null = null

  let createName = ''
  let renameName = ''

  let addSource = ''
  let addName = ''
  let addRepoDir = ''
  let groupName = ''
  let aliasItems: Alias[] = []
  let groupItems: GroupSummary[] = []

  let baseRemote = ''
  let baseBranch = ''
  let writeRemote = ''
  let writeBranch = ''

  let archiveReason = ''

  const formatError = (err: unknown, fallback: string): string => {
    if (err instanceof Error) return err.message
    return fallback
  }

  const loadContext = async (): Promise<void> => {
    await loadWorkspaces(true)
    const current = get(workspaces)
    workspace = workspaceId ? current.find((entry) => entry.id === workspaceId) ?? null : null
    repo =
      workspace && repoName
        ? workspace.repos.find((entry) => entry.name === repoName) ?? null
        : null
    if (mode === 'rename' && workspace) {
      renameName = workspace.name
    }
    if (mode === 'remotes' && repo) {
      baseRemote = repo.baseRemote ?? ''
      baseBranch = repo.baseBranch ?? ''
      writeRemote = repo.writeRemote ?? ''
      writeBranch = repo.writeBranch ?? ''
    }
    if (mode === 'add-repo') {
      aliasItems = await listAliases()
      groupItems = await listGroups()
    }
  }

  const handleCreate = async (): Promise<void> => {
    const name = createName.trim()
    if (!name) {
      error = 'Workspace name is required.'
      return
    }
    loading = true
    error = null
    try {
      const result = await createWorkspace(name, '')
      await loadWorkspaces(true)
      selectWorkspace(result.workspace.name)
      success = `Created ${result.workspace.name}.`
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to create workspace.')
    } finally {
      loading = false
    }
  }

  const handleRename = async (): Promise<void> => {
    if (!workspace) return
    const nextName = renameName.trim()
    if (!nextName) {
      error = 'New name is required.'
      return
    }
    loading = true
    error = null
    try {
      await renameWorkspace(workspace.id, nextName)
      await loadWorkspaces(true)
      if (get(activeWorkspaceId) === workspace.id) {
        selectWorkspace(nextName)
      }
      success = `Renamed to ${nextName}.`
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to rename workspace.')
    } finally {
      loading = false
    }
  }

  const handleAddRepo = async (): Promise<void> => {
    if (!workspace) return
    const source = addSource.trim()
    if (!source) {
      error = 'Repo source is required.'
      return
    }
    loading = true
    error = null
    try {
      await addRepo(workspace.id, source, addName.trim(), addRepoDir.trim())
      await loadWorkspaces(true)
      success = 'Repo added.'
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to add repo.')
    } finally {
      loading = false
    }
  }

  const handleBrowse = async (): Promise<void> => {
    try {
      const defaultDirectory = addSource.trim()
      const path = await openDirectoryDialog(
        'Select repo directory',
        defaultDirectory
      )
      if (!path) return
      addSource = path
    } catch (err) {
      error = formatError(err, 'Failed to open directory picker.')
    }
  }

  const handleApplyGroup = async (): Promise<void> => {
    if (!workspace) return
    if (!groupName.trim()) {
      error = 'Select a group to apply.'
      return
    }
    loading = true
    error = null
    try {
      await applyGroup(workspace.id, groupName.trim())
      await loadWorkspaces(true)
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to apply group.')
    } finally {
      loading = false
    }
  }

  const handleRemotes = async (): Promise<void> => {
    if (!workspace || !repo) return
    loading = true
    error = null
    try {
      await updateRepoRemotes(
        workspace.id,
        repo.name,
        baseRemote.trim(),
        baseBranch.trim(),
        writeRemote.trim(),
        writeBranch.trim()
      )
      await loadWorkspaces(true)
      success = 'Remotes updated.'
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to update remotes.')
    } finally {
      loading = false
    }
  }

  const handleArchive = async (): Promise<void> => {
    if (!workspace) return
    loading = true
    error = null
    try {
      await archiveWorkspace(workspace.id, archiveReason.trim())
      await loadWorkspaces(true)
      if (get(activeWorkspaceId) === workspace.id) {
        clearWorkspace()
      }
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to archive workspace.')
    } finally {
      loading = false
    }
  }

  const handleRemoveWorkspace = async (): Promise<void> => {
    if (!workspace) return
    loading = true
    error = null
    try {
      await removeWorkspace(workspace.id)
      await loadWorkspaces(true)
      if (get(activeWorkspaceId) === workspace.id) {
        clearWorkspace()
      }
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to remove workspace.')
    } finally {
      loading = false
    }
  }

  const handleRemoveRepo = async (): Promise<void> => {
    if (!workspace || !repo) return
    loading = true
    error = null
    try {
      await removeRepo(workspace.id, repo.name, false, false)
      await loadWorkspaces(true)
      if (get(activeWorkspaceId) === workspace.id) {
        clearRepo()
      }
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to remove repo.')
    } finally {
      loading = false
    }
  }

  onMount(async () => {
    await loadContext()
    await tick()
    nameInput?.focus()
  })
</script>

<section class="panel" role="dialog" aria-modal="true">
  <header class="header">
    <div>
      <div class="title">
        {#if mode === 'create'}
          Create workspace
        {:else if mode === 'rename'}
          Rename workspace
        {:else if mode === 'add-repo'}
          Add repo
        {:else if mode === 'remotes'}
          Update remotes
        {:else if mode === 'archive'}
          Archive workspace
        {:else if mode === 'remove-workspace'}
          Remove workspace
        {:else if mode === 'remove-repo'}
          Remove repo
        {:else}
          Workspace action
        {/if}
      </div>
      <div class="subtitle">
        {#if workspace}
          {workspace.name}
        {:else}
          Workset
        {/if}
      </div>
    </div>
    <button class="ghost" type="button" on:click={onClose}>Close</button>
  </header>

  {#if error}
    <div class="note error">{error}</div>
  {:else if success}
    <div class="note success">{success}</div>
  {/if}

  {#if mode === 'create'}
    <div class="form">
      <label class="field">
        <span>Name</span>
        <input bind:this={nameInput} bind:value={createName} placeholder="acme" />
      </label>
      <button class="primary" type="button" on:click={handleCreate} disabled={loading}>
        {loading ? 'Creating…' : 'Create'}
      </button>
    </div>
  {:else if mode === 'rename'}
    <div class="form">
      <label class="field">
        <span>New name</span>
        <input bind:this={nameInput} bind:value={renameName} placeholder="acme" />
      </label>
      <div class="hint">Renaming updates config and workset.yaml. Files stay in place.</div>
      <button class="primary" type="button" on:click={handleRename} disabled={loading}>
        {loading ? 'Renaming…' : 'Rename'}
      </button>
    </div>
  {:else if mode === 'add-repo'}
    <div class="form">
      <label class="field">
        <span>Source (alias, URL, or path)</span>
        <div class="inline">
          <input
            bind:this={nameInput}
            bind:value={addSource}
            placeholder="alias, URL, or local path"
            list="alias-options"
          />
          <button class="ghost" type="button" on:click={handleBrowse}>Browse</button>
        </div>
        <datalist id="alias-options">
          {#each aliasItems as alias}
            <option value={alias.name} />
          {/each}
        </datalist>
      </label>
      <div class="hint">Aliases are resolved from config; URLs and local paths work too.</div>
      <label class="field">
        <span>Repo name (optional)</span>
        <input bind:value={addName} placeholder="agent-skills" />
      </label>
      <label class="field">
        <span>Repo dir (optional)</span>
        <input bind:value={addRepoDir} placeholder="agent-skills" />
      </label>
      <label class="field">
        <span>Apply group (optional)</span>
        <input bind:value={groupName} placeholder="group name" list="group-options" />
        <datalist id="group-options">
          {#each groupItems as group}
            <option value={group.name} />
          {/each}
        </datalist>
      </label>
      <button class="ghost" type="button" on:click={handleApplyGroup} disabled={loading}>
        Apply group
      </button>
      <button class="primary" type="button" on:click={handleAddRepo} disabled={loading}>
        {loading ? 'Adding…' : 'Add repo'}
      </button>
    </div>
  {:else if mode === 'remotes'}
    <div class="form">
      <label class="field">
        <span>Base remote</span>
        <input bind:this={nameInput} bind:value={baseRemote} placeholder="origin" />
      </label>
      <label class="field">
        <span>Base branch</span>
        <input bind:value={baseBranch} placeholder="main" />
      </label>
      <label class="field">
        <span>Write remote</span>
        <input bind:value={writeRemote} placeholder="origin" />
      </label>
      <label class="field">
        <span>Write branch</span>
        <input bind:value={writeBranch} placeholder="main" />
      </label>
      <button class="primary" type="button" on:click={handleRemotes} disabled={loading}>
        {loading ? 'Saving…' : 'Save remotes'}
      </button>
    </div>
  {:else if mode === 'archive'}
    <div class="form">
      <div class="hint">Archiving hides the workspace but keeps files on disk.</div>
      <label class="field">
        <span>Reason (optional)</span>
        <input bind:this={nameInput} bind:value={archiveReason} placeholder="paused" />
      </label>
      <button class="danger" type="button" on:click={handleArchive} disabled={loading}>
        {loading ? 'Archiving…' : 'Archive'}
      </button>
    </div>
  {:else if mode === 'remove-workspace'}
    <div class="form">
      <div class="hint">
        This only removes the workspace registration. Files and worktrees stay on disk.
      </div>
      <button class="danger" type="button" on:click={handleRemoveWorkspace} disabled={loading}>
        {loading ? 'Removing…' : 'Remove workspace'}
      </button>
    </div>
  {:else if mode === 'remove-repo'}
    <div class="form">
      <div class="hint">This only removes the repo from the workspace config.</div>
      <button class="danger" type="button" on:click={handleRemoveRepo} disabled={loading}>
        {loading ? 'Removing…' : 'Remove repo'}
      </button>
    </div>
  {/if}
</section>

<style>
  .panel {
    background: var(--panel-strong);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 20px;
    width: min(480px, 100%);
    display: flex;
    flex-direction: column;
    gap: 16px;
    box-shadow: 0 24px 60px rgba(6, 10, 16, 0.6);
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .title {
    font-size: 18px;
    font-weight: 600;
  }

  .subtitle {
    font-size: 12px;
    color: var(--muted);
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
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    color: var(--text);
    padding: 8px 10px;
    font-size: 14px;
    transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  }

  .inline {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .inline input {
    flex: 1;
  }

  .primary {
    background: var(--accent);
    color: #081018;
    border: none;
    padding: 8px 14px;
    border-radius: var(--radius-md);
    font-weight: 600;
    cursor: pointer;
    width: fit-content;
    transition: background var(--transition-fast), transform var(--transition-fast);
  }

  .primary:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 85%, white);
  }

  .primary:active:not(:disabled) {
    transform: scale(0.98);
  }

  .primary:disabled {
    opacity: 0.5;
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
    width: fit-content;
    transition: background var(--transition-fast), border-color var(--transition-fast), transform var(--transition-fast);
  }

  .danger:hover:not(:disabled) {
    background: var(--danger-soft);
    border-color: var(--danger);
  }

  .danger:active:not(:disabled) {
    transform: scale(0.98);
  }

  .danger:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .ghost {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--border);
    color: var(--text);
    padding: 6px 12px;
    border-radius: var(--radius-sm);
    cursor: pointer;
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
    opacity: 0.5;
    cursor: not-allowed;
  }

  .hint {
    font-size: 12px;
    color: var(--muted);
  }

  .note {
    font-size: 13px;
  }

  .note.error {
    color: var(--danger);
  }

  .note.success {
    color: var(--success);
  }
</style>
