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

  interface Props {
    onClose: () => void;
    mode: 
    | 'create'
    | 'rename'
    | 'add-repo'
    | 'remotes'
    | 'archive'
    | 'remove-workspace'
    | 'remove-repo'
    | null;
    workspaceId?: string | null;
    repoName?: string | null;
  }

  let {
    onClose,
    mode,
    workspaceId = null,
    repoName = null
  }: Props = $props();

  let workspace: Workspace | null = $state(null)
  let repo: Repo | null = $state(null)

  let error: string | null = $state(null)
  let success: string | null = $state(null)
  let loading = $state(false)

  let nameInput: HTMLInputElement | null = $state(null)

  let createName = $state('')
  let renameName = $state('')

  let addSource = $state('')
  let aliasItems: Alias[] = $state([])
  let groupItems: GroupSummary[] = $state([])

  // Tab state for add-repo mode
  type AddTab = 'repo' | 'alias' | 'group'
  let addTab: AddTab = $state('repo')
  let selectedAlias = $state('')
  let selectedGroup = $state('')
  let aliasDropdownOpen = $state(false)
  let groupDropdownOpen = $state(false)

  // Quick setup state for create mode
  let quickSetupExpanded = $state(false)
  let quickSetupTab: AddTab = $state('repo')
  let quickSetupSource = $state('')
  let quickSetupAlias = $state('')
  let quickSetupGroupSelection = $state('')
  let quickSetupAliasDropdownOpen = $state(false)
  let quickSetupGroupDropdownOpen = $state(false)

  // Helper to get alias display info
  const getAliasSource = (alias: Alias): string => alias.url || alias.path || ''

  // Helper to get selected alias object
  const getSelectedAliasObj = (name: string): Alias | undefined =>
    aliasItems.find(a => a.name === name)

  // Helper to get selected group object
  const getSelectedGroupObj = (name: string): GroupSummary | undefined =>
    groupItems.find(g => g.name === name)

  let baseRemote = $state('')
  let baseBranch = $state('')
  let writeRemote = $state('')
  let writeBranch = $state('')

  let archiveReason = $state('')
  let removeDeleteWorktree = $state(false)
  let removeDeleteLocal = $state(false)

  const formatError = (err: unknown, fallback: string): string => {
    if (err instanceof Error) return err.message
    if (typeof err === 'string') return err
    if (err && typeof err === 'object' && 'message' in err) {
      const message = (err as {message?: string}).message
      if (typeof message === 'string') return message
    }
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
    if (mode === 'add-repo' || mode === 'create') {
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

      // Handle quick setup if expanded and has content
      if (quickSetupExpanded) {
        // Reload workspaces to get the new workspace with its ID
        await loadWorkspaces(true)
        const newWorkspace = get(workspaces).find((w) => w.name === result.workspace.name)
        if (newWorkspace) {
          if (quickSetupTab === 'repo' && quickSetupSource.trim()) {
            await addRepo(newWorkspace.id, quickSetupSource.trim(), '', '')
          } else if (quickSetupTab === 'alias' && quickSetupAlias.trim()) {
            await addRepo(newWorkspace.id, quickSetupAlias.trim(), '', '')
          } else if (quickSetupTab === 'group' && quickSetupGroupSelection.trim()) {
            await applyGroup(newWorkspace.id, quickSetupGroupSelection.trim())
          }
        }
      }

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
      await addRepo(workspace.id, source, '', '')
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

  const handleAddAlias = async (): Promise<void> => {
    if (!workspace) return
    if (!selectedAlias.trim()) {
      error = 'Select an alias to add.'
      return
    }
    loading = true
    error = null
    try {
      await addRepo(workspace.id, selectedAlias.trim(), '', '')
      await loadWorkspaces(true)
      success = 'Alias added.'
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to add alias.')
    } finally {
      loading = false
    }
  }

  const handleApplyGroup = async (): Promise<void> => {
    if (!workspace) return
    if (!selectedGroup.trim()) {
      error = 'Select a group to apply.'
      return
    }
    loading = true
    error = null
    try {
      await applyGroup(workspace.id, selectedGroup.trim())
      await loadWorkspaces(true)
      success = 'Group applied.'
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
    if (!workspaceId) return
    loading = true
    error = null
    try {
      await removeWorkspace(workspaceId)
      workspaces.update((current) => current.filter((entry) => entry.id !== workspaceId))
      if (get(activeWorkspaceId) === workspaceId) {
        clearWorkspace()
      }
      onClose()
      void loadWorkspaces(true)
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
      await removeRepo(workspace.id, repo.name, removeDeleteWorktree, removeDeleteLocal)
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
          Add to workspace
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
    <button class="ghost" type="button" onclick={onClose}>Close</button>
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

      <button
        class="collapsible-header"
        type="button"
        onclick={() => quickSetupExpanded = !quickSetupExpanded}
      >
        <span class="chevron" class:expanded={quickSetupExpanded}>▸</span>
        Quick setup (optional)
      </button>

      {#if quickSetupExpanded}
        <div class="collapsible-content">
          <div class="tabs compact">
            <button
              class="tab"
              class:active={quickSetupTab === 'repo'}
              type="button"
              onclick={() => quickSetupTab = 'repo'}
            >Repo</button>
            <button
              class="tab"
              class:active={quickSetupTab === 'alias'}
              type="button"
              onclick={() => quickSetupTab = 'alias'}
            >Alias</button>
            <button
              class="tab"
              class:active={quickSetupTab === 'group'}
              type="button"
              onclick={() => quickSetupTab = 'group'}
            >Group</button>
          </div>

          {#if quickSetupTab === 'repo'}
            <label class="field">
              <span>URL or path</span>
              <div class="inline">
                <input
                  bind:value={quickSetupSource}
                  placeholder="https://github.com/org/repo or /path/to/repo"
                />
                <button class="ghost" type="button" onclick={async () => {
                  try {
                    const path = await openDirectoryDialog('Select repo directory', quickSetupSource.trim())
                    if (path) quickSetupSource = path
                  } catch (err) {
                    error = formatError(err, 'Failed to open directory picker.')
                  }
                }}>Browse</button>
              </div>
            </label>
          {:else if quickSetupTab === 'alias'}
            <div class="field">
              <span>Select alias</span>
              <div class="custom-dropdown">
                <button
                  class="dropdown-trigger"
                  type="button"
                  onclick={() => quickSetupAliasDropdownOpen = !quickSetupAliasDropdownOpen}
                >
                  {#if quickSetupAlias}
                    {@const alias = getSelectedAliasObj(quickSetupAlias)}
                    <span class="dropdown-selected">
                      <span class="dropdown-name">{quickSetupAlias}</span>
                      {#if alias}
                        <span class="dropdown-meta">{getAliasSource(alias)}</span>
                      {/if}
                    </span>
                  {:else}
                    <span class="dropdown-placeholder">Choose an alias…</span>
                  {/if}
                  <span class="dropdown-arrow">{quickSetupAliasDropdownOpen ? '▴' : '▾'}</span>
                </button>
                {#if quickSetupAliasDropdownOpen}
                  <div class="dropdown-menu">
                    {#each aliasItems as alias}
                      <button
                        class="dropdown-item"
                        class:selected={quickSetupAlias === alias.name}
                        type="button"
                        onclick={() => {
                          quickSetupAlias = alias.name
                          quickSetupAliasDropdownOpen = false
                        }}
                      >
                        <span class="dropdown-item-name">{alias.name}</span>
                        <span class="dropdown-item-meta">{getAliasSource(alias)}</span>
                      </button>
                    {/each}
                    {#if aliasItems.length === 0}
                      <div class="dropdown-empty">No aliases configured</div>
                    {/if}
                  </div>
                {/if}
              </div>
            </div>
          {:else if quickSetupTab === 'group'}
            <div class="field">
              <span>Select group</span>
              <div class="custom-dropdown">
                <button
                  class="dropdown-trigger"
                  type="button"
                  onclick={() => quickSetupGroupDropdownOpen = !quickSetupGroupDropdownOpen}
                >
                  {#if quickSetupGroupSelection}
                    {@const group = getSelectedGroupObj(quickSetupGroupSelection)}
                    <span class="dropdown-selected">
                      <span class="dropdown-name">{quickSetupGroupSelection}</span>
                      {#if group}
                        <span class="dropdown-meta">{group.repo_count} repo{group.repo_count !== 1 ? 's' : ''}</span>
                      {/if}
                    </span>
                  {:else}
                    <span class="dropdown-placeholder">Choose a group…</span>
                  {/if}
                  <span class="dropdown-arrow">{quickSetupGroupDropdownOpen ? '▴' : '▾'}</span>
                </button>
                {#if quickSetupGroupDropdownOpen}
                  <div class="dropdown-menu">
                    {#each groupItems as group}
                      <button
                        class="dropdown-item"
                        class:selected={quickSetupGroupSelection === group.name}
                        type="button"
                        onclick={() => {
                          quickSetupGroupSelection = group.name
                          quickSetupGroupDropdownOpen = false
                        }}
                      >
                        <span class="dropdown-item-name">{group.name}</span>
                        <span class="dropdown-item-meta">
                          {group.repo_count} repo{group.repo_count !== 1 ? 's' : ''}
                          {#if group.description}
                            · {group.description}
                          {/if}
                        </span>
                      </button>
                    {/each}
                    {#if groupItems.length === 0}
                      <div class="dropdown-empty">No groups configured</div>
                    {/if}
                  </div>
                {/if}
              </div>
            </div>
          {/if}
        </div>
      {/if}

      <button class="primary" type="button" onclick={handleCreate} disabled={loading}>
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
      <button class="primary" type="button" onclick={handleRename} disabled={loading}>
        {loading ? 'Renaming…' : 'Rename'}
      </button>
    </div>
  {:else if mode === 'add-repo'}
    <div class="tabs">
      <button
        class="tab"
        class:active={addTab === 'repo'}
        type="button"
        onclick={() => addTab = 'repo'}
      >Repo</button>
      <button
        class="tab"
        class:active={addTab === 'alias'}
        type="button"
        onclick={() => addTab = 'alias'}
      >Alias</button>
      <button
        class="tab"
        class:active={addTab === 'group'}
        type="button"
        onclick={() => addTab = 'group'}
      >Group</button>
    </div>

    {#if addTab === 'repo'}
      <div class="form">
        <label class="field">
          <span>URL or path</span>
          <div class="inline">
            <input
              bind:this={nameInput}
              bind:value={addSource}
              placeholder="https://github.com/org/repo or /path/to/repo"
            />
            <button class="ghost" type="button" onclick={handleBrowse}>Browse</button>
          </div>
        </label>
        <button class="primary" type="button" onclick={handleAddRepo} disabled={loading}>
          {loading ? 'Adding…' : 'Add repo'}
        </button>
      </div>
    {:else if addTab === 'alias'}
      <div class="form">
        <div class="field">
          <span>Select alias</span>
          <div class="custom-dropdown">
            <button
              class="dropdown-trigger"
              type="button"
              onclick={() => aliasDropdownOpen = !aliasDropdownOpen}
            >
              {#if selectedAlias}
                {@const alias = getSelectedAliasObj(selectedAlias)}
                <span class="dropdown-selected">
                  <span class="dropdown-name">{selectedAlias}</span>
                  {#if alias}
                    <span class="dropdown-meta">{getAliasSource(alias)}</span>
                  {/if}
                </span>
              {:else}
                <span class="dropdown-placeholder">Choose an alias…</span>
              {/if}
              <span class="dropdown-arrow">{aliasDropdownOpen ? '▴' : '▾'}</span>
            </button>
            {#if aliasDropdownOpen}
              <div class="dropdown-menu">
                {#each aliasItems as alias}
                  <button
                    class="dropdown-item"
                    class:selected={selectedAlias === alias.name}
                    type="button"
                    onclick={() => {
                      selectedAlias = alias.name
                      aliasDropdownOpen = false
                    }}
                  >
                    <span class="dropdown-item-name">{alias.name}</span>
                    <span class="dropdown-item-meta">{getAliasSource(alias)}</span>
                  </button>
                {/each}
                {#if aliasItems.length === 0}
                  <div class="dropdown-empty">No aliases configured</div>
                {/if}
              </div>
            {/if}
          </div>
        </div>
        {#if aliasItems.length === 0}
          <div class="hint">Add aliases in Settings → Aliases.</div>
        {/if}
        <button class="primary" type="button" onclick={handleAddAlias} disabled={loading || !selectedAlias.trim()}>
          {loading ? 'Adding…' : 'Add alias'}
        </button>
      </div>
    {:else if addTab === 'group'}
      <div class="form">
        <div class="field">
          <span>Select group</span>
          <div class="custom-dropdown">
            <button
              class="dropdown-trigger"
              type="button"
              onclick={() => groupDropdownOpen = !groupDropdownOpen}
            >
              {#if selectedGroup}
                {@const group = getSelectedGroupObj(selectedGroup)}
                <span class="dropdown-selected">
                  <span class="dropdown-name">{selectedGroup}</span>
                  {#if group}
                    <span class="dropdown-meta">{group.repo_count} repo{group.repo_count !== 1 ? 's' : ''}</span>
                  {/if}
                </span>
              {:else}
                <span class="dropdown-placeholder">Choose a group…</span>
              {/if}
              <span class="dropdown-arrow">{groupDropdownOpen ? '▴' : '▾'}</span>
            </button>
            {#if groupDropdownOpen}
              <div class="dropdown-menu">
                {#each groupItems as group}
                  <button
                    class="dropdown-item"
                    class:selected={selectedGroup === group.name}
                    type="button"
                    onclick={() => {
                      selectedGroup = group.name
                      groupDropdownOpen = false
                    }}
                  >
                    <span class="dropdown-item-name">{group.name}</span>
                    <span class="dropdown-item-meta">
                      {group.repo_count} repo{group.repo_count !== 1 ? 's' : ''}
                      {#if group.description}
                        · {group.description}
                      {/if}
                    </span>
                  </button>
                {/each}
                {#if groupItems.length === 0}
                  <div class="dropdown-empty">No groups configured</div>
                {/if}
              </div>
            {/if}
          </div>
        </div>
        {#if groupItems.length === 0}
          <div class="hint">Add groups in Settings → Groups.</div>
        {/if}
        <button class="primary" type="button" onclick={handleApplyGroup} disabled={loading || !selectedGroup.trim()}>
          {loading ? 'Applying…' : 'Apply group'}
        </button>
      </div>
    {/if}
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
      <button class="primary" type="button" onclick={handleRemotes} disabled={loading}>
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
      <button class="danger" type="button" onclick={handleArchive} disabled={loading}>
        {loading ? 'Archiving…' : 'Archive'}
      </button>
    </div>
  {:else if mode === 'remove-workspace'}
    <div class="form">
      <div class="hint">
        This only removes the workspace registration. Files and worktrees stay on disk.
      </div>
      <button class="danger" type="button" onclick={handleRemoveWorkspace} disabled={loading}>
        {loading ? 'Removing…' : 'Remove workspace'}
      </button>
    </div>
  {:else if mode === 'remove-repo'}
    <div class="form">
      <div class="hint">This removes the repo from the workspace config by default.</div>
      <label class="option">
        <input type="checkbox" bind:checked={removeDeleteWorktree} />
        <span>Also delete worktrees for this repo</span>
      </label>
      <label class="option">
        <input type="checkbox" bind:checked={removeDeleteLocal} />
        <span>Also delete local cache for this repo</span>
      </label>
      {#if removeDeleteWorktree || removeDeleteLocal}
        <div class="hint">Destructive deletes are permanent and cannot be undone.</div>
      {/if}
      {#if repo?.statusKnown === false && (removeDeleteWorktree || removeDeleteLocal)}
        <div class="note warning">
          Repo status is still loading. Destructive deletes may be blocked if the repo is dirty.
        </div>
      {/if}
      {#if repo?.dirty && (removeDeleteWorktree || removeDeleteLocal)}
        <div class="note warning">
          Uncommitted changes detected. Destructive deletes will be blocked until the repo is clean.
        </div>
      {/if}
      <button class="danger" type="button" onclick={handleRemoveRepo} disabled={loading}>
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

  .note.warning {
    color: var(--warning);
  }

  .option {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--text);
  }

  .option input {
    accent-color: var(--accent);
  }

  /* Tab bar styles */
  .tabs {
    display: flex;
    gap: 0;
    border-bottom: 1px solid var(--border);
    margin-bottom: 4px;
  }

  .tabs.compact {
    margin-bottom: 12px;
  }

  .tab {
    flex: 1;
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    color: var(--muted);
    padding: 10px 16px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: color var(--transition-fast), border-color var(--transition-fast);
  }

  .tab:hover {
    color: var(--text);
  }

  .tab.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
  }

  /* Collapsible section styles */
  .collapsible-header {
    display: flex;
    align-items: center;
    gap: 8px;
    background: transparent;
    border: none;
    color: var(--muted);
    font-size: 12px;
    cursor: pointer;
    padding: 4px 0;
    transition: color var(--transition-fast);
  }

  .collapsible-header:hover {
    color: var(--text);
  }

  .chevron {
    display: inline-block;
    transition: transform var(--transition-fast);
  }

  .chevron.expanded {
    transform: rotate(90deg);
  }

  .collapsible-content {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 12px 0 4px 0;
    animation: fadeIn var(--transition-fast) ease-out;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  /* Custom dropdown styles */
  .custom-dropdown {
    position: relative;
  }

  .dropdown-trigger {
    width: 100%;
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    color: var(--text);
    padding: 10px 12px;
    font-size: 14px;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    text-align: left;
    transition: border-color var(--transition-fast);
  }

  .dropdown-trigger:hover {
    border-color: var(--accent);
  }

  .dropdown-selected {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    flex: 1;
  }

  .dropdown-name {
    font-weight: 500;
    color: var(--text);
  }

  .dropdown-meta {
    font-size: 11px;
    color: var(--muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .dropdown-placeholder {
    color: var(--muted);
  }

  .dropdown-arrow {
    color: var(--muted);
    font-size: 10px;
    flex-shrink: 0;
  }

  .dropdown-menu {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    background: var(--panel-strong);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
    z-index: 100;
    max-height: 240px;
    overflow-y: auto;
    animation: dropdownFadeIn 0.15s ease-out;
  }

  @keyframes dropdownFadeIn {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .dropdown-item {
    width: 100%;
    background: transparent;
    border: none;
    padding: 10px 12px;
    text-align: left;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    gap: 2px;
    transition: background var(--transition-fast);
  }

  .dropdown-item:hover {
    background: rgba(255, 255, 255, 0.05);
  }

  .dropdown-item.selected {
    background: rgba(var(--accent-rgb), 0.1);
  }

  .dropdown-item-name {
    font-weight: 500;
    color: var(--text);
    font-size: 14px;
  }

  .dropdown-item-meta {
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .dropdown-empty {
    padding: 16px 12px;
    color: var(--muted);
    font-size: 13px;
    text-align: center;
  }
</style>
