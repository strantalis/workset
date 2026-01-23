<script lang="ts">
  import {onMount, tick} from 'svelte'
  import {
    activeWorkspaceId,
    clearRepo,
    clearWorkspace,
    loadWorkspaces,
    selectWorkspace,
    workspaces
  } from '../state'
  import {
    addRepo,
    archiveWorkspace,
    createWorkspace,
    removeRepo,
    removeWorkspace,
    renameWorkspace,
    unarchiveWorkspace,
  } from '../api'
  import type {Repo, Workspace} from '../types'

  interface Props {
    onClose: () => void;
    initialWorkspaceId?: string | null;
    initialRepoName?: string | null;
    initialSection?: 'create' | 'rename' | 'repo' | null;
  }

  let {
    onClose,
    initialWorkspaceId = null,
    initialRepoName = null,
    initialSection = null
  }: Props = $props();

  let selectedWorkspaceId: string | null = $state(null)
  let showArchived = $state(false)

  let createName = $state('')
  let createPath = $state('')
  let createError: string | null = $state(null)
  let createSuccess: string | null = $state(null)
  let creating = $state(false)
  let workspaceError: string | null = $state(null)
  let createInput: HTMLInputElement | null = $state(null)

  let addSource = $state('')
  let addName = $state('')
  let addRepoDir = $state('')
  let addError: string | null = $state(null)
  let addSuccess: string | null = $state(null)
  let adding = $state(false)
  let addSourceInput: HTMLInputElement | null = $state(null)

  let selectedRepoName: string | null = $state(null)

  let renameName = $state('')
  let renameError: string | null = $state(null)
  let renameSuccess: string | null = $state(null)
  let renaming = $state(false)
  let lastSelectedId: string | null = $state(null)
  let lastSelectedRepoName: string | null = $state(null)
  let renameInput: HTMLInputElement | null = $state(null)

  let confirmWorkspaceRemove: string | null = $state(null)
  let confirmRepoRemove: {workspaceId: string; repoName: string} | null = $state(null)
  let removeRepoDeleteWorktree = $state(false)
  let removeRepoDeleteLocal = $state(false)
  let working = false

  const selectManagerWorkspace = (id: string): void => {
    selectedWorkspaceId = id
    confirmWorkspaceRemove = null
    confirmRepoRemove = null
    removeRepoDeleteWorktree = false
    removeRepoDeleteLocal = false
    addError = null
    addSuccess = null
    workspaceError = null
  }

  let managerWorkspaces = $derived($workspaces)
  let activeWorkspaces = $derived(managerWorkspaces.filter((workspace) => !workspace.archived))
  let archivedWorkspaces = $derived(managerWorkspaces.filter((workspace) => workspace.archived))
  let selectedWorkspace =
    $derived(managerWorkspaces.find((workspace) => workspace.id === selectedWorkspaceId) ?? null)
  $effect(() => {
    if (!selectedWorkspaceId && managerWorkspaces.length > 0) {
      selectedWorkspaceId = $activeWorkspaceId ?? managerWorkspaces[0]?.id ?? null
    }
  });
  $effect(() => {
    if (selectedWorkspace && selectedWorkspace.id !== lastSelectedId) {
      renameName = selectedWorkspace.name
      renameError = null
      renameSuccess = null
      lastSelectedId = selectedWorkspace.id
      selectedRepoName = selectedWorkspace.repos[0]?.name ?? null
      lastSelectedRepoName = null
    }
  });
  $effect(() => {
    if (selectedWorkspace && selectedRepoName) {
      const exists = selectedWorkspace.repos.some((repo) => repo.name === selectedRepoName)
      if (!exists) {
        selectedRepoName = selectedWorkspace.repos[0]?.name ?? null
        lastSelectedRepoName = null
      }
    }
  });
  let selectedRepo =
    $derived(selectedWorkspace?.repos.find((repo) => repo.name === selectedRepoName) ?? null)
  $effect(() => {
    if (selectedRepo && selectedRepo.name !== lastSelectedRepoName) {
      lastSelectedRepoName = selectedRepo.name
    }
  });

  const formatError = (err: unknown, fallback: string): string => {
    if (err instanceof Error) {
      return err.message
    }
    if (typeof err === 'string') {
      return err
    }
    if (err && typeof err === 'object' && 'message' in err) {
      const message = (err as {message?: string}).message
      if (typeof message === 'string') {
        return message
      }
    }
    return fallback
  }

  const handleCreate = async (): Promise<void> => {
    if (creating) return
    createError = null
    createSuccess = null
    const name = createName.trim()
    if (!name) {
      createError = 'Workspace name is required.'
      return
    }
    creating = true
    try {
      const result = await createWorkspace(name, createPath.trim())
      createName = ''
      createPath = ''
      createSuccess = `Created ${result.workspace.name}.`
      await loadWorkspaces(true)
      selectWorkspace(result.workspace.name)
      selectedWorkspaceId = result.workspace.name
    } catch (err) {
      createError = formatError(err, 'Failed to create workspace.')
    } finally {
      creating = false
    }
  }

  const handleArchive = async (workspace: Workspace): Promise<void> => {
    if (working) return
    workspaceError = null
    working = true
    try {
      await archiveWorkspace(workspace.id, '')
      await loadWorkspaces(true)
      if ($activeWorkspaceId === workspace.id) {
        clearWorkspace()
      }
      if (selectedWorkspaceId === workspace.id) {
        selectedWorkspaceId = null
      }
    } catch (err) {
      workspaceError = formatError(err, 'Failed to archive workspace.')
    } finally {
      working = false
    }
  }

  const handleUnarchive = async (workspace: Workspace): Promise<void> => {
    if (working) return
    workspaceError = null
    working = true
    try {
      await unarchiveWorkspace(workspace.id)
      await loadWorkspaces(true)
      selectedWorkspaceId = workspace.id
    } catch (err) {
      workspaceError = formatError(err, 'Failed to unarchive workspace.')
    } finally {
      working = false
    }
  }

  const handleRemoveWorkspace = async (workspace: Workspace): Promise<void> => {
    if (working) return
    workspaceError = null
    working = true
    try {
      await removeWorkspace(workspace.id)
      await loadWorkspaces(true)
      if ($activeWorkspaceId === workspace.id) {
        clearWorkspace()
      }
      if (selectedWorkspaceId === workspace.id) {
        selectedWorkspaceId = null
      }
    } catch (err) {
      workspaceError = formatError(err, 'Failed to remove workspace.')
    } finally {
      confirmWorkspaceRemove = null
      working = false
    }
  }

  const handleAddRepo = async (): Promise<void> => {
    if (adding || !selectedWorkspace) return
    addError = null
    addSuccess = null
    const source = addSource.trim()
    if (!source) {
      addError = 'Repo source is required.'
      return
    }
    adding = true
    try {
      await addRepo(selectedWorkspace.id, source, addName.trim(), addRepoDir.trim())
      addSource = ''
      addName = ''
      addRepoDir = ''
      addSuccess = 'Repo added.'
      await loadWorkspaces(true)
    } catch (err) {
      addError = formatError(err, 'Failed to add repo.')
    } finally {
      adding = false
    }
  }

  const handleRemoveRepo = async (workspace: Workspace, repo: Repo): Promise<void> => {
    if (working) return
    working = true
    try {
      await removeRepo(workspace.id, repo.name, removeRepoDeleteWorktree, removeRepoDeleteLocal)
      await loadWorkspaces(true)
      if ($activeWorkspaceId === workspace.id) {
        clearRepo()
      }
    } catch (err) {
      addError = formatError(err, 'Failed to remove repo.')
    } finally {
      confirmRepoRemove = null
      removeRepoDeleteWorktree = false
      removeRepoDeleteLocal = false
      working = false
    }
  }

  const handleRename = async (): Promise<void> => {
    if (renaming || !selectedWorkspace) return
    renameError = null
    renameSuccess = null
    const nextName = renameName.trim()
    if (!nextName) {
      renameError = 'New name is required.'
      return
    }
    if (nextName === selectedWorkspace.name) {
      renameSuccess = 'Name is unchanged.'
      return
    }
    renaming = true
    try {
      const currentId = selectedWorkspace.id
      await renameWorkspace(currentId, nextName)
      await loadWorkspaces(true)
      if ($activeWorkspaceId === currentId) {
        selectWorkspace(nextName)
      }
      selectedWorkspaceId = nextName
      renameSuccess = `Renamed to ${nextName}.`
    } catch (err) {
      renameError = formatError(err, 'Failed to rename workspace.')
    } finally {
      renaming = false
    }
  }

  onMount(() => {
    void loadWorkspaces(true)
    if (!selectedWorkspaceId) {
      selectedWorkspaceId = initialWorkspaceId ?? $activeWorkspaceId ?? $workspaces[0]?.id ?? null
    }
    if (initialRepoName) {
      selectedRepoName = initialRepoName
    }
    void tick().then(() => {
      if (initialSection === 'create') {
        createInput?.focus()
      } else if (initialSection === 'rename') {
        renameInput?.focus()
      } else if (initialSection === 'repo') {
        addSourceInput?.focus()
      }
    })
  })
</script>

<div class="panel" role="dialog" aria-modal="true" aria-label="Workspace management">
  <header class="header">
    <div>
      <div class="title">Workspaces</div>
      <div class="subtitle">Create and manage workspace registrations and repos.</div>
    </div>
    <button class="ghost" type="button" onclick={onClose}>Close</button>
  </header>

  <section class="create">
    <div class="section-title">Create workspace</div>
    <div class="form-grid">
      <label class="field">
        <span>Name</span>
        <input
          placeholder="acme"
          bind:this={createInput}
          bind:value={createName}
          onkeydown={(event) => {
            if (event.key === 'Enter') void handleCreate()
          }}
        />
      </label>
      <label class="field span-2">
        <span>Path (optional)</span>
        <input
          placeholder="~/workspaces/acme"
          bind:value={createPath}
          onkeydown={(event) => {
            if (event.key === 'Enter') void handleCreate()
          }}
        />
      </label>
    </div>
    <div class="inline-actions">
      <button class="primary" type="button" onclick={handleCreate} disabled={creating}>
        {creating ? 'Creating…' : 'Create workspace'}
      </button>
      {#if createError}
        <div class="note error">{createError}</div>
      {:else if createSuccess}
        <div class="note success">{createSuccess}</div>
      {/if}
    </div>
  </section>

  <section class="list">
    <div class="list-header">
      <div class="section-title">Workspace list</div>
      <label class="toggle">
        <input type="checkbox" bind:checked={showArchived} />
        <span>Show archived</span>
      </label>
    </div>
    {#if workspaceError}
      <div class="note error">{workspaceError}</div>
    {/if}

    <div class="list-grid">
      <div class="workspace-column">
        {#if activeWorkspaces.length === 0}
          <div class="empty">No active workspaces yet.</div>
        {/if}
        {#each activeWorkspaces as workspace}
          <div class:active={workspace.id === selectedWorkspaceId} class="workspace-card">
            <button class="select" type="button" onclick={() => selectManagerWorkspace(workspace.id)}>
              <div class="name">{workspace.name}</div>
              <div class="path">{workspace.path}</div>
            </button>
            <div class="card-actions">
              <button class="ghost" type="button" onclick={() => selectWorkspace(workspace.id)}>
                Open
              </button>
              <button class="ghost" type="button" onclick={() => handleArchive(workspace)}>
                Archive
              </button>
              {#if confirmWorkspaceRemove === workspace.id}
                <button class="danger" type="button" onclick={() => handleRemoveWorkspace(workspace)}>
                  Confirm remove
                </button>
                <button class="ghost" type="button" onclick={() => (confirmWorkspaceRemove = null)}>
                  Cancel
                </button>
              {:else}
                <button
                  class="ghost"
                  type="button"
                  onclick={() => (confirmWorkspaceRemove = workspace.id)}
                >
                  Remove
                </button>
              {/if}
            </div>
          </div>
        {/each}

        {#if showArchived}
          <div class="divider">Archived</div>
          {#if archivedWorkspaces.length === 0}
            <div class="empty">No archived workspaces.</div>
          {/if}
          {#each archivedWorkspaces as workspace}
            <div class:active={workspace.id === selectedWorkspaceId} class="workspace-card archived">
              <button class="select" type="button" onclick={() => selectManagerWorkspace(workspace.id)}>
                <div class="name">{workspace.name}</div>
                <div class="path">{workspace.path}</div>
                {#if workspace.archivedReason}
                  <div class="reason">{workspace.archivedReason}</div>
                {/if}
              </button>
              <div class="card-actions">
                <button class="ghost" type="button" onclick={() => handleUnarchive(workspace)}>
                  Unarchive
                </button>
                {#if confirmWorkspaceRemove === workspace.id}
                  <button class="danger" type="button" onclick={() => handleRemoveWorkspace(workspace)}>
                    Confirm remove
                  </button>
                  <button class="ghost" type="button" onclick={() => (confirmWorkspaceRemove = null)}>
                    Cancel
                  </button>
                {:else}
                  <button
                    class="ghost"
                    type="button"
                    onclick={() => (confirmWorkspaceRemove = workspace.id)}
                  >
                    Remove
                  </button>
                {/if}
              </div>
            </div>
          {/each}
        {/if}
      </div>

      <div class="details-column">
        {#if selectedWorkspace}
          <div class="details-card">
            <div class="details-header">
              <div>
                <div class="details-title">{selectedWorkspace.name}</div>
                <div class="details-sub">{selectedWorkspace.path}</div>
              </div>
              <div class="pill">{selectedWorkspace.repos.length} repos</div>
            </div>

            <div class="rename">
              <div class="section-title">Rename workspace</div>
              <div class="form-grid">
                <label class="field span-2">
                  <span>New name</span>
                  <input
                    placeholder="acme"
                    bind:this={renameInput}
                    bind:value={renameName}
                    onkeydown={(event) => {
                      if (event.key === 'Enter') void handleRename()
                    }}
                  />
                </label>
              </div>
              <div class="inline-actions">
                <button class="primary" type="button" onclick={handleRename} disabled={renaming}>
                  {renaming ? 'Renaming…' : 'Rename'}
                </button>
                {#if renameError}
                  <div class="note error">{renameError}</div>
                {:else if renameSuccess}
                  <div class="note success">{renameSuccess}</div>
                {/if}
              </div>
              <div class="hint">Renaming updates config and workset.yaml. Files stay in place.</div>
            </div>

            <div class="repo-add">
              <div class="section-title">Add repo</div>
              <div class="form-grid">
                <label class="field span-2">
                  <span>Source</span>
                  <input
                    placeholder="alias, URL, or local path"
                    bind:this={addSourceInput}
                    bind:value={addSource}
                    onkeydown={(event) => {
                      if (event.key === 'Enter') void handleAddRepo()
                    }}
                  />
                </label>
                <label class="field">
                  <span>Repo name (optional)</span>
                  <input placeholder="agent-skills" bind:value={addName} />
                </label>
                <label class="field">
                  <span>Repo dir (optional)</span>
                  <input placeholder="agent-skills" bind:value={addRepoDir} />
                </label>
              </div>
              <div class="inline-actions">
                <button class="primary" type="button" onclick={handleAddRepo} disabled={adding}>
                  {adding ? 'Adding…' : 'Add repo'}
                </button>
                {#if addError}
                  <div class="note error">{addError}</div>
                {:else if addSuccess}
                  <div class="note success">{addSuccess}</div>
                {/if}
              </div>
              <div class="hint">Removes only update the workset config. Files stay on disk.</div>
            </div>

            <div class="repo-list">
              <div class="section-title">Repos</div>
              {#if selectedWorkspace.repos.length === 0}
                <div class="empty">No repos configured yet.</div>
              {/if}
              {#each selectedWorkspace.repos as repo}
                <div class:active={repo.name === selectedRepoName} class="repo-row">
                  <button class="repo-select" type="button" onclick={() => (selectedRepoName = repo.name)}>
                    <div class="repo-name">{repo.name}</div>
                    <div class="repo-path">{repo.path}</div>
                  </button>
                  <div class="card-actions">
                    {#if confirmRepoRemove?.repoName === repo.name}
                      <div class="remove-options">
                        <label class="option">
                          <input type="checkbox" bind:checked={removeRepoDeleteWorktree} />
                          <span>Also delete worktrees for this repo</span>
                        </label>
                        <label class="option">
                          <input type="checkbox" bind:checked={removeRepoDeleteLocal} />
                          <span>Also delete local cache for this repo</span>
                        </label>
                        {#if removeRepoDeleteWorktree || removeRepoDeleteLocal}
                          <div class="hint">Destructive deletes are permanent and cannot be undone.</div>
                        {/if}
                        {#if repo.statusKnown === false && (removeRepoDeleteWorktree || removeRepoDeleteLocal)}
                          <div class="note warning">
                            Repo status is still loading. Destructive deletes may be blocked if the repo is dirty.
                          </div>
                        {/if}
                        {#if repo.dirty && (removeRepoDeleteWorktree || removeRepoDeleteLocal)}
                          <div class="note warning">
                            Uncommitted changes detected. Destructive deletes will be blocked until the repo is clean.
                          </div>
                        {/if}
                      </div>
                      <button
                        class="danger"
                        type="button"
                        onclick={() => handleRemoveRepo(selectedWorkspace, repo)}
                      >
                        Confirm remove
                      </button>
                      <button
                        class="ghost"
                        type="button"
                        onclick={() => {
                          confirmRepoRemove = null
                          removeRepoDeleteWorktree = false
                          removeRepoDeleteLocal = false
                        }}
                      >
                        Cancel
                      </button>
                    {:else}
                      <button
                        class="ghost"
                        type="button"
                        onclick={() => {
                          confirmRepoRemove = {workspaceId: selectedWorkspace.id, repoName: repo.name}
                          removeRepoDeleteWorktree = false
                          removeRepoDeleteLocal = false
                        }}
                      >
                        Remove
                      </button>
                    {/if}
                  </div>
                </div>
              {/each}
            </div>

          </div>
        {:else}
          <div class="details-card empty">
            <div class="details-title">Pick a workspace to manage repos.</div>
            <div class="details-sub">Select a workspace to view repos and add new ones.</div>
          </div>
        {/if}
      </div>
    </div>
  </section>
</div>

<style>
  .panel {
    background: var(--panel-strong);
    border: 1px solid var(--border);
    border-radius: 20px;
    padding: 24px;
    max-width: 1120px;
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 20px;
    box-shadow: 0 30px 80px rgba(6, 10, 16, 0.6);
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .title {
    font-size: 20px;
    font-weight: 600;
  }

  .subtitle {
    color: var(--muted);
    font-size: 13px;
  }

  .ghost {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--border);
    color: var(--text);
    padding: 6px 12px;
    border-radius: 8px;
    cursor: pointer;
  }

  .primary {
    background: var(--accent);
    color: #081018;
    border: none;
    padding: 8px 16px;
    border-radius: 10px;
    font-weight: 600;
    cursor: pointer;
  }

  .danger {
    background: rgba(255, 107, 107, 0.12);
    border: 1px solid rgba(255, 107, 107, 0.5);
    color: #ff9a9a;
    padding: 6px 12px;
    border-radius: 8px;
    cursor: pointer;
  }

  .create,
  .list {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 16px;
  }

  .section-title {
    font-size: 13px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
    font-weight: 600;
  }

  .form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
    margin-top: 12px;
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
    border-radius: 10px;
    color: var(--text);
    padding: 8px 10px;
    font-size: 14px;
  }

  .span-2 {
    grid-column: span 2;
  }

  .inline-actions {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 12px;
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

  .remove-options {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 6px;
  }

  .list-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .toggle {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: var(--muted);
    font-size: 12px;
  }

  .list-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr);
    gap: 16px;
    margin-top: 16px;
  }

  .workspace-column,
  .details-column {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .workspace-card {
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    background: var(--panel-soft);
  }

  .workspace-card.archived {
    border-style: dashed;
    opacity: 0.8;
  }

  .workspace-card.active {
    border-color: var(--accent);
    box-shadow: inset 0 0 0 1px rgba(45, 140, 255, 0.35);
  }

  .select {
    background: none;
    border: none;
    text-align: left;
    cursor: pointer;
    color: inherit;
  }

  .name {
    font-size: 15px;
    font-weight: 600;
  }

  .path,
  .reason {
    font-size: 12px;
    color: var(--muted);
  }

  .reason {
    margin-top: 6px;
  }

  .card-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .divider {
    margin-top: 12px;
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
  }

  .details-card {
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 16px;
    background: var(--panel-soft);
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .details-card.empty {
    align-items: flex-start;
    justify-content: center;
  }

  .details-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .details-title {
    font-size: 18px;
    font-weight: 600;
  }

  .details-sub {
    font-size: 12px;
    color: var(--muted);
  }

  .pill {
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 4px 10px;
    font-size: 12px;
    color: var(--muted);
  }

  .repo-add,
  .repo-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .hint {
    font-size: 12px;
    color: var(--muted);
  }

  .repo-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 10px 12px;
    background: rgba(6, 12, 18, 0.4);
  }

  .repo-row.active {
    border-color: var(--accent);
    box-shadow: inset 0 0 0 1px rgba(45, 140, 255, 0.35);
  }

  .repo-select {
    flex: 1;
    background: none;
    border: none;
    color: inherit;
    text-align: left;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .repo-name {
    font-size: 14px;
    font-weight: 600;
  }

  .repo-path {
    font-size: 12px;
    color: var(--muted);
  }

  .empty {
    font-size: 13px;
    color: var(--muted);
    padding: 8px 0;
  }

  @media (max-width: 1000px) {
    .list-grid {
      grid-template-columns: 1fr;
    }
  }

  @media (max-width: 720px) {
    .panel {
      border-radius: 0;
      height: 100%;
      overflow: auto;
    }
  }
</style>
