<script lang="ts">
  import {onMount, tick} from 'svelte'
  import {get} from 'svelte/store'
  import {
    addRepo,
    applyGroup,
    archiveWorkspace,
    createWorkspace,
    getGroup,
    listAliases,
    listGroups,
    openDirectoryDialog,
    removeRepo,
    removeWorkspace,
    renameWorkspace
  } from '../api'
  import {activeWorkspaceId, clearRepo, clearWorkspace, loadWorkspaces, selectWorkspace, workspaces} from '../state'
  import type {Alias, GroupSummary, Repo, Workspace} from '../types'
  import Alert from './ui/Alert.svelte'
  import Button from './ui/Button.svelte'
  import Modal from './Modal.svelte'
  import Tooltip from './Tooltip.svelte'

  interface Props {
    onClose: () => void;
    mode: 
    | 'create'
    | 'rename'
    | 'add-repo'
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
  let groupDetails: Map<string, string[]> = $state(new Map()) // group name -> repo names

  // Quick setup state for create mode
  let quickSetupExpanded = $state(false)
  let quickSetupSource = $state('')
  let selectedAliases: Set<string> = $state(new Set())
  let selectedGroups: Set<string> = $state(new Set())

  // Helper to get alias display info
  const getAliasSource = (alias: Alias): string => alias.url || alias.path || ''

  let archiveReason = $state('')
  let removeDeleteWorktree = $state(false)
  let removeDeleteLocal = $state(false)

  const modeTitle = $derived(
    mode === 'create' ? 'Create workspace'
    : mode === 'rename' ? 'Rename workspace'
    : mode === 'add-repo' ? 'Add to workspace'
    : mode === 'archive' ? 'Archive workspace'
    : mode === 'remove-workspace' ? 'Remove workspace'
    : mode === 'remove-repo' ? 'Remove repo'
    : 'Workspace action'
  )

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
    if (mode === 'add-repo' || mode === 'create') {
      aliasItems = await listAliases()
      groupItems = await listGroups()
      // Fetch full details for each group to show repo names in tooltips
      const details = new Map<string, string[]>()
      for (const g of groupItems) {
        const full = await getGroup(g.name)
        details.set(g.name, full.members.map(m => m.repo))
      }
      groupDetails = details
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
      // Collect aliases to pass to backend
      const aliasesToAdd: string[] = []
      if (quickSetupExpanded) {
        // Add direct repo URL if provided
        if (quickSetupSource.trim()) {
          aliasesToAdd.push(quickSetupSource.trim())
        }
        // Add all selected aliases
        for (const alias of selectedAliases) {
          aliasesToAdd.push(alias)
        }
      }

      // Collect groups to pass to backend
      const groupsToAdd = quickSetupExpanded ? Array.from(selectedGroups) : []

      const result = await createWorkspace(
        name,
        '',
        aliasesToAdd.length > 0 ? aliasesToAdd : undefined,
        groupsToAdd.length > 0 ? groupsToAdd : undefined
      )

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

  const handleAddItems = async (): Promise<void> => {
    if (!workspace) return
    const source = addSource.trim()
    const hasSource = source.length > 0
    const hasAliases = selectedAliases.size > 0
    const hasGroups = selectedGroups.size > 0

    if (!hasSource && !hasAliases && !hasGroups) {
      error = 'Provide a repo URL/path, select aliases, or select groups.'
      return
    }

    loading = true
    error = null
    try {
      // 1. Add direct repo URL if provided
      if (hasSource) {
        await addRepo(workspace.id, source, '', '')
      }
      // 2. Add each selected alias
      for (const alias of selectedAliases) {
        await addRepo(workspace.id, alias, '', '')
      }
      // 3. Apply each selected group
      for (const group of selectedGroups) {
        await applyGroup(workspace.id, group)
      }

      await loadWorkspaces(true)
      const itemCount = (hasSource ? 1 : 0) + selectedAliases.size + selectedGroups.size
      success = `Added ${itemCount} item${itemCount !== 1 ? 's' : ''}.`
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to add items.')
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

<Modal
  title={modeTitle}
  subtitle={workspace?.name ?? 'Workset'}
  size="xl"
  headerAlign="left"
  {onClose}
>
  {#if error}
    <Alert variant="error">{error}</Alert>
  {:else if success}
    <Alert variant="success">{success}</Alert>
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
          <label class="field">
            <span>Add repo by URL or path</span>
            <div class="inline">
              <input
                bind:value={quickSetupSource}
                placeholder="https://github.com/org/repo or /path/to/repo"
              />
              <Button variant="ghost" size="sm" onclick={async () => {
                try {
                  const path = await openDirectoryDialog('Select repo directory', quickSetupSource.trim())
                  if (path) quickSetupSource = path
                } catch (err) {
                  error = formatError(err, 'Failed to open directory picker.')
                }
              }}>Browse</Button>
            </div>
          </label>

          {#if aliasItems.length > 0}
            <div class="field">
              <span>Select aliases</span>
              <div class="checkbox-list">
                {#each aliasItems as alias}
                  <label class="checkbox-item">
                    <input
                      type="checkbox"
                      checked={selectedAliases.has(alias.name)}
                      onchange={() => {
                        if (selectedAliases.has(alias.name)) {
                          selectedAliases.delete(alias.name)
                          selectedAliases = new Set(selectedAliases)
                        } else {
                          selectedAliases.add(alias.name)
                          selectedAliases = new Set(selectedAliases)
                        }
                      }}
                    />
                    <div class="checkbox-content">
                      <span class="checkbox-name">{alias.name}</span>
                      <span class="checkbox-meta">{getAliasSource(alias)}</span>
                    </div>
                  </label>
                {/each}
              </div>
            </div>
          {/if}

          {#if groupItems.length > 0}
            <div class="field">
              <span>Select groups</span>
              <div class="checkbox-list">
                {#each groupItems as group}
                  <Tooltip text={groupDetails.get(group.name) || []} position="cursor" class="checkbox-tooltip">
                    <label class="checkbox-item">
                      <input
                        type="checkbox"
                        checked={selectedGroups.has(group.name)}
                        onchange={() => {
                          if (selectedGroups.has(group.name)) {
                            selectedGroups.delete(group.name)
                            selectedGroups = new Set(selectedGroups)
                          } else {
                            selectedGroups.add(group.name)
                            selectedGroups = new Set(selectedGroups)
                          }
                        }}
                      />
                      <div class="checkbox-content">
                        <span class="checkbox-name">{group.name}</span>
                        <span class="checkbox-meta">
                          {group.repo_count} repo{group.repo_count !== 1 ? 's' : ''}
                          {#if group.description}
                            · {group.description}
                          {/if}
                        </span>
                      </div>
                    </label>
                  </Tooltip>
                {/each}
              </div>
            </div>
          {/if}

          {#if aliasItems.length === 0 && groupItems.length === 0}
            <div class="hint">No aliases or groups configured. Add them in Settings.</div>
          {/if}
        </div>
      {/if}

      <Button variant="primary" onclick={handleCreate} disabled={loading} class="action-btn">
        {loading ? 'Creating…' : 'Create'}
      </Button>
    </div>
  {:else if mode === 'rename'}
    <div class="form">
      <label class="field">
        <span>New name</span>
        <input bind:this={nameInput} bind:value={renameName} placeholder="acme" />
      </label>
      <div class="hint">Renaming updates config and workset.yaml. Files stay in place.</div>
      <Button variant="primary" onclick={handleRename} disabled={loading} class="action-btn">
        {loading ? 'Renaming…' : 'Rename'}
      </Button>
    </div>
  {:else if mode === 'add-repo'}
    <div class="form">
      <label class="field">
        <span>Add repo by URL or path</span>
        <div class="inline">
          <input
            bind:this={nameInput}
            bind:value={addSource}
            placeholder="https://github.com/org/repo or /path/to/repo"
          />
          <Button variant="ghost" size="sm" onclick={handleBrowse}>Browse</Button>
        </div>
      </label>

      {#if aliasItems.length > 0}
        <div class="field">
          <span>Select aliases</span>
          <div class="checkbox-list">
            {#each aliasItems as alias}
              <label class="checkbox-item">
                <input
                  type="checkbox"
                  checked={selectedAliases.has(alias.name)}
                  onchange={() => {
                    if (selectedAliases.has(alias.name)) {
                      selectedAliases.delete(alias.name)
                      selectedAliases = new Set(selectedAliases)
                    } else {
                      selectedAliases.add(alias.name)
                      selectedAliases = new Set(selectedAliases)
                    }
                  }}
                />
                <div class="checkbox-content">
                  <span class="checkbox-name">{alias.name}</span>
                  <span class="checkbox-meta">{getAliasSource(alias)}</span>
                </div>
              </label>
            {/each}
          </div>
        </div>
      {/if}

      {#if groupItems.length > 0}
        <div class="field">
          <span>Select groups</span>
          <div class="checkbox-list">
            {#each groupItems as group}
              <Tooltip text={groupDetails.get(group.name) || []} position="cursor" class="checkbox-tooltip">
                <label class="checkbox-item">
                  <input
                    type="checkbox"
                    checked={selectedGroups.has(group.name)}
                    onchange={() => {
                      if (selectedGroups.has(group.name)) {
                        selectedGroups.delete(group.name)
                        selectedGroups = new Set(selectedGroups)
                      } else {
                        selectedGroups.add(group.name)
                        selectedGroups = new Set(selectedGroups)
                      }
                    }}
                  />
                  <div class="checkbox-content">
                    <span class="checkbox-name">{group.name}</span>
                    <span class="checkbox-meta">
                      {group.repo_count} repo{group.repo_count !== 1 ? 's' : ''}
                      {#if group.description}
                        · {group.description}
                      {/if}
                    </span>
                  </div>
                </label>
              </Tooltip>
            {/each}
          </div>
        </div>
      {/if}

      {#if aliasItems.length === 0 && groupItems.length === 0}
        <div class="hint">No aliases or groups configured. Add them in Settings.</div>
      {/if}

      <Button variant="primary" onclick={handleAddItems} disabled={loading} class="action-btn">
        {loading ? 'Adding…' : 'Add'}
      </Button>
    </div>
  {:else if mode === 'archive'}
    <div class="form">
      <div class="hint">Archiving hides the workspace but keeps files on disk.</div>
      <label class="field">
        <span>Reason (optional)</span>
        <input bind:this={nameInput} bind:value={archiveReason} placeholder="paused" />
      </label>
      <Button variant="danger" onclick={handleArchive} disabled={loading} class="action-btn">
        {loading ? 'Archiving…' : 'Archive'}
      </Button>
    </div>
  {:else if mode === 'remove-workspace'}
    <div class="form">
      <div class="hint">
        This only removes the workspace registration. Files and worktrees stay on disk.
      </div>
      <Button variant="danger" onclick={handleRemoveWorkspace} disabled={loading} class="action-btn">
        {loading ? 'Removing…' : 'Remove workspace'}
      </Button>
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
        <Alert variant="warning">
          Repo status is still loading. Destructive deletes may be blocked if the repo is dirty.
        </Alert>
      {/if}
      {#if repo?.dirty && (removeDeleteWorktree || removeDeleteLocal)}
        <Alert variant="warning">
          Uncommitted changes detected. Destructive deletes will be blocked until the repo is clean.
        </Alert>
      {/if}
      <Button variant="danger" onclick={handleRemoveRepo} disabled={loading} class="action-btn">
        {loading ? 'Removing…' : 'Remove repo'}
      </Button>
    </div>
  {/if}
</Modal>

<style>
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

  :global(.action-btn) {
    width: fit-content;
  }

  .hint {
    font-size: 12px;
    color: var(--muted);
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

  /* Checkbox list styles */
  .checkbox-list {
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    max-height: 180px;
    overflow-y: auto;
  }

  .checkbox-item {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 10px 12px;
    cursor: pointer;
    transition: background var(--transition-fast);
  }

  .checkbox-item:hover {
    background: rgba(255, 255, 255, 0.03);
  }

  .checkbox-item:not(:last-child) {
    border-bottom: 1px solid var(--border);
  }

  /* When checkbox-item is wrapped in a Tooltip, apply border to wrapper instead */
  :global(.checkbox-tooltip:not(:last-child)) .checkbox-item {
    border-bottom: 1px solid var(--border);
  }

  :global(.checkbox-tooltip:last-child) .checkbox-item {
    border-bottom: none;
  }

  .checkbox-item input[type="checkbox"] {
    appearance: none;
    -webkit-appearance: none;
    width: 16px;
    height: 16px;
    min-width: 16px;
    min-height: 16px;
    margin-top: 3px;
    flex-shrink: 0;
    background: var(--panel-soft);
    border: 1.5px solid var(--border);
    border-radius: 3px;
    cursor: pointer;
    display: grid;
    place-content: center;
    transition: border-color var(--transition-fast), background var(--transition-fast);
  }

  .checkbox-item input[type="checkbox"]:hover {
    border-color: var(--accent);
  }

  .checkbox-item input[type="checkbox"]:checked {
    background: var(--accent);
    border-color: var(--accent);
  }

  .checkbox-item input[type="checkbox"]::before {
    content: '';
    width: 8px;
    height: 8px;
    transform: scale(0);
    transition: transform 0.1s ease-in-out;
    clip-path: polygon(14% 44%, 0 65%, 50% 100%, 100% 16%, 80% 0%, 43% 62%);
    background: #0a0f14;
  }

  .checkbox-item input[type="checkbox"]:checked::before {
    transform: scale(1);
  }

  .checkbox-content {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    flex: 1;
  }

  .checkbox-name {
    font-weight: 500;
    font-size: 14px;
    color: var(--text);
  }

  .checkbox-meta {
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
