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
  import {
    activeWorkspaceId,
    clearRepo,
    clearWorkspace,
    loadWorkspaces,
    refreshWorkspacesStatus,
    selectWorkspace,
    workspaces
  } from '../state'
  import type {Alias, GroupSummary, Repo, Workspace} from '../types'
  import {
    generateWorkspaceName,
    generateAlternatives,
    deriveRepoName,
    isRepoSource,
    looksLikeUrl,
    looksLikePath
  } from '../names'
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

  // Create mode: smart single input
  let primaryInput = $state('')           // URL, path, or workspace name
  let customizeName = $state('')          // Override for generated name
  let createExpanded = $state(false)      // Show customize section
  let alternatives: string[] = $state([]) // Alternative name suggestions

  let renameName = $state('')

  let addSource = $state('')
  let aliasItems: Alias[] = $state([])
  let groupItems: GroupSummary[] = $state([])
  let groupDetails: Map<string, string[]> = $state(new Map()) // group name -> repo names

  // Selection state for create mode expanded section and add-repo mode
  let selectedAliases: Set<string> = $state(new Set())
  let selectedGroups: Set<string> = $state(new Set())

  // Create mode: derived state
  let detectedRepoName = $derived(deriveRepoName(primaryInput))
  let inputIsSource = $derived(isRepoSource(primaryInput))

  // Get the first selected alias name for auto-generation when no primary input
  let firstSelectedAlias = $derived(
    selectedAliases.size > 0 ? Array.from(selectedAliases)[0] : null
  )

  // Source for name generation: URL/path repo name, or first selected alias
  let nameSource = $derived(detectedRepoName || firstSelectedAlias)

  let generatedName = $derived(
    nameSource ? generateWorkspaceName(nameSource) : null
  )

  // Final name: custom override > generated > plain text input
  let finalName = $derived(
    customizeName || generatedName || primaryInput.trim()
  )

  // Show name customization when we have a generated name (from URL/path or alias)
  let showNameCustomization = $derived(!!nameSource)

  // Regenerate alternatives when name source changes
  $effect(() => {
    if (nameSource) {
      alternatives = generateAlternatives(nameSource, 2)
    } else {
      alternatives = []
    }
  })

  function regenerateName(): void {
    if (nameSource) {
      customizeName = generateWorkspaceName(nameSource)
    }
  }

  function selectAlternative(name: string): void {
    customizeName = name
  }

  // Helper to get alias display info
  const getAliasSource = (alias: Alias): string => alias.url || alias.path || ''

  let archiveReason = $state('')
  let removeDeleteWorktree = $state(false)
  let removeDeleteLocal = $state(false)
  let removeDeleteFiles = $state(false)
  let removeForceDelete = $state(false)
  let removeConfirmText = $state('')
  let removeRepoConfirmText = $state('')
  let removeRepoStatusRequested = $state(false)
  let removeRepoStatusRefreshing = $state(false)

  const removeConfirmValid = $derived(
    !removeDeleteFiles || removeConfirmText.trim().toUpperCase() === 'DELETE'
  )
  const removeRepoConfirmRequired = $derived(removeDeleteWorktree || removeDeleteLocal)
  const removeRepoConfirmValid = $derived(
    !removeRepoConfirmRequired || removeRepoConfirmText.trim().toUpperCase() === 'DELETE'
  )
  const removeRepoStatus = $derived(
    workspaceId && repoName
      ? $workspaces.find((entry) => entry.id === workspaceId)?.repos.find((entry) => entry.name === repoName) ?? null
      : null
  )

  $effect(() => {
    if (!removeDeleteFiles && removeForceDelete) {
      removeForceDelete = false
    }
    if (!removeDeleteFiles && removeConfirmText) {
      removeConfirmText = ''
    }
    if (!removeRepoConfirmRequired && removeRepoConfirmText) {
      removeRepoConfirmText = ''
    }
    if (!removeRepoConfirmRequired) {
      removeRepoStatusRequested = false
    }
  })

  $effect(() => {
    if (removeRepoConfirmRequired && !removeRepoStatusRequested) {
      removeRepoStatusRequested = true
      removeRepoStatusRefreshing = true
      void (async () => {
        await refreshWorkspacesStatus(true)
        removeRepoStatusRefreshing = false
      })()
    }
  })

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
    if (!finalName) {
      error = 'Enter a repo URL, path, or workspace name.'
      return
    }
    loading = true
    error = null
    try {
      const repos: string[] = []

      // If primary input is URL/path, add it as first repo
      if (inputIsSource) {
        repos.push(primaryInput.trim())
      }

      // Add any selected aliases (from expanded section)
      if (createExpanded) {
        for (const alias of selectedAliases) {
          repos.push(alias)
        }
      }

      // Groups from expanded section
      const groups = createExpanded ? Array.from(selectedGroups) : []

      const result = await createWorkspace(
        finalName,
        '',
        repos.length > 0 ? repos : undefined,
        groups.length > 0 ? groups : undefined
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
      if (removeDeleteFiles && !removeConfirmValid) {
        error = 'Type DELETE to confirm file deletion.'
        return
      }
      await removeWorkspace(workspaceId, {
        deleteFiles: removeDeleteFiles,
        force: removeForceDelete
      })
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
      if (!removeRepoConfirmValid) {
        error = 'Type DELETE to confirm repo deletion.'
        return
      }
      await removeRepo(workspace.id, repo.name, removeDeleteWorktree, removeDeleteLocal)
      await loadWorkspaces(true)
      if (get(activeWorkspaceId) === workspace.id) {
        clearRepo()
      }
      onClose()
    } catch (err) {
      error = formatError(err, 'Failed to remove repo.')
    } finally {
      removeRepoConfirmText = ''
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
        <span>Repo URL, path, or workspace name</span>
        <div class="inline">
          <input
            bind:this={nameInput}
            bind:value={primaryInput}
            placeholder="git@github.com:org/repo.git"
            autocapitalize="off"
            autocorrect="off"
            spellcheck="false"
          />
          <Button variant="ghost" size="sm" onclick={async () => {
            try {
              const path = await openDirectoryDialog('Select repo directory', primaryInput.trim())
              if (path) primaryInput = path
            } catch (err) {
              error = formatError(err, 'Failed to open directory picker.')
            }
          }}>Browse</Button>
        </div>
      </label>

      {#if finalName}
        <div class="feedback-box">
          {#if generatedName}
            <span class="feedback-check">✓</span>
            <span class="feedback-text">
              Will create workspace "<strong>{customizeName || generatedName}</strong>"
              {#if !inputIsSource && selectedAliases.size > 0}
                with {selectedAliases.size} repo{selectedAliases.size !== 1 ? 's' : ''}
              {/if}
            </span>
            <button class="refresh-btn" type="button" onclick={regenerateName} title="Generate new name">
              ↻
            </button>
          {:else}
            <span class="feedback-check">✓</span>
            <span class="feedback-text">
              Will create empty workspace "<strong>{primaryInput.trim()}</strong>"
            </span>
          {/if}
        </div>
      {/if}

      <button
        class="collapsible-header"
        type="button"
        onclick={() => createExpanded = !createExpanded}
      >
        <span class="chevron" class:expanded={createExpanded}>▸</span>
        {inputIsSource ? 'Customize name or add more repos' : 'Add repos from aliases or groups'}
      </button>

      {#if createExpanded}
        <div class="collapsible-content">
          {#if showNameCustomization}
            <label class="field">
              <span>Workspace name</span>
              <div class="inline">
                <input
                  bind:value={customizeName}
                  placeholder={generatedName ?? ''}
                  autocapitalize="off"
                  autocorrect="off"
                  spellcheck="false"
                />
                <Button variant="ghost" size="sm" onclick={regenerateName}>↻ New</Button>
              </div>
            </label>

            {#if alternatives.length > 0}
              <div class="suggestions">
                Suggestions:
                {#each alternatives as alt, i}
                  {#if i > 0} · {/if}
                  <button
                    type="button"
                    class="suggestion-btn"
                    onclick={() => selectAlternative(alt)}
                  >{alt}</button>
                {/each}
              </div>
            {/if}
          {/if}

          {#if aliasItems.length > 0}
            <div class="field">
              <span>{inputIsSource ? 'Additional repos' : 'Select aliases'}</span>
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

      <Button variant="primary" onclick={handleCreate} disabled={loading || !finalName} class="action-btn">
        {loading ? 'Creating…' : 'Create'}
      </Button>
    </div>
  {:else if mode === 'rename'}
    <div class="form">
      <label class="field">
        <span>New name</span>
        <input
          bind:this={nameInput}
          bind:value={renameName}
          placeholder="acme"
          autocapitalize="off"
          autocorrect="off"
          spellcheck="false"
        />
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
            autocapitalize="off"
            autocorrect="off"
            spellcheck="false"
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
        <input
          bind:this={nameInput}
          bind:value={archiveReason}
          placeholder="paused"
          autocapitalize="off"
          autocorrect="off"
          spellcheck="false"
        />
      </label>
      <Button variant="danger" onclick={handleArchive} disabled={loading} class="action-btn">
        {loading ? 'Archiving…' : 'Archive'}
      </Button>
    </div>
  {:else if mode === 'remove-workspace'}
    <div class="form">
      <div class="hint">Remove workspace registration only by default.</div>
      <label class="option">
        <input type="checkbox" bind:checked={removeDeleteFiles} />
        <span>Also delete workspace files and worktrees</span>
      </label>
      {#if removeDeleteFiles}
        <div class="hint">Deletes the workspace directory and removes all worktrees.</div>
        <label class="field">
          <span>Type DELETE to confirm</span>
          <input
            bind:value={removeConfirmText}
            placeholder="DELETE"
            autocapitalize="off"
            autocorrect="off"
            spellcheck="false"
          />
        </label>
        <label class="option">
          <input type="checkbox" bind:checked={removeForceDelete} />
          <span>Force delete (skip safety checks)</span>
        </label>
        {#if removeForceDelete}
          <Alert variant="warning">
            Force delete bypasses dirty/unmerged checks and may delete uncommitted work.
          </Alert>
        {/if}
      {/if}
      <Button
        variant="danger"
        onclick={handleRemoveWorkspace}
        disabled={loading || !removeConfirmValid}
        class="action-btn"
      >
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
      {#if removeRepoConfirmRequired}
        <label class="field">
          <span>Type DELETE to confirm</span>
          <input
            bind:value={removeRepoConfirmText}
            placeholder="DELETE"
            autocapitalize="off"
            autocorrect="off"
            spellcheck="false"
          />
        </label>
      {/if}
      {#if removeDeleteWorktree || removeDeleteLocal}
        <div class="hint">Destructive deletes are permanent and cannot be undone.</div>
      {/if}
      {#if removeRepoStatusRefreshing}
        <Alert variant="warning">Fetching repo status…</Alert>
      {:else if removeRepoStatus?.statusKnown === false && (removeDeleteWorktree || removeDeleteLocal)}
        <Alert variant="warning">
          Repo status unknown. Destructive deletes may be blocked if the repo is dirty.
        </Alert>
      {/if}
      {#if removeRepoStatus?.dirty && (removeDeleteWorktree || removeDeleteLocal)}
        <Alert variant="warning">
          Uncommitted changes detected. Destructive deletes will be blocked until the repo is clean.
        </Alert>
      {/if}
      <Button
        variant="danger"
        onclick={handleRemoveRepo}
        disabled={loading || !removeRepoConfirmValid}
        class="action-btn"
      >
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

  /* Feedback box for create mode */
  .feedback-box {
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 10px 12px;
    font-size: 13px;
  }

  .feedback-check {
    color: var(--success, #4ade80);
    font-size: 14px;
    flex-shrink: 0;
  }

  .feedback-text {
    flex: 1;
    color: var(--text);
  }

  .feedback-text strong {
    font-weight: 600;
  }

  .refresh-btn {
    background: transparent;
    border: none;
    color: var(--muted);
    cursor: pointer;
    padding: 4px 8px;
    font-size: 14px;
    border-radius: var(--radius-sm);
    transition: color var(--transition-fast), background var(--transition-fast);
  }

  .refresh-btn:hover {
    color: var(--text);
    background: rgba(255, 255, 255, 0.05);
  }

  /* Suggestions */
  .suggestions {
    font-size: 12px;
    color: var(--muted);
    margin-top: -4px;
  }

  .suggestion-btn {
    background: transparent;
    border: none;
    color: var(--accent);
    cursor: pointer;
    padding: 0;
    font-size: 12px;
    transition: opacity var(--transition-fast);
  }

  .suggestion-btn:hover {
    opacity: 0.8;
    text-decoration: underline;
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
