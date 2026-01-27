<script lang="ts">
  import type {Workspace} from '../types'
  import IconButton from './ui/IconButton.svelte'
  import DropdownMenu from './ui/DropdownMenu.svelte'

  interface Props {
    workspaces?: Workspace[];
    activeWorkspaceId?: string | null;
    activeRepoId?: string | null;
    onSelectWorkspace: (workspaceId: string) => void;
    onSelectRepo: (repoId: string) => void;
    onCreateWorkspace: () => void;
    onAddRepo: (workspaceId: string) => void;
    onManageWorkspace: (workspaceId: string, action: 'rename' | 'archive' | 'remove') => void;
    onManageRepo: (
    workspaceId: string,
    repoId: string,
    action: 'remove'
  ) => void;
    sidebarCollapsed?: boolean;
    onToggleSidebar: () => void;
  }

  let {
    workspaces = [],
    activeWorkspaceId = null,
    activeRepoId = null,
    onSelectWorkspace,
    onSelectRepo,
    onCreateWorkspace,
    onAddRepo,
    onManageWorkspace,
    onManageRepo,
    sidebarCollapsed = false,
    onToggleSidebar
  }: Props = $props();

  let collapsed: Record<string, boolean> = $state({})
  let workspaceMenu: string | null = $state(null)
  let repoMenu: string | null = $state(null)

  const isCollapsed = (workspaceId: string): boolean => collapsed[workspaceId] ?? false

  const toggleWorkspace = (workspaceId: string): void => {
    collapsed = {...collapsed, [workspaceId]: !isCollapsed(workspaceId)}
  }

  let visibleWorkspaces = $derived(workspaces.filter((workspace) => !workspace.archived))

  let searchQuery = $state('')

  let filteredWorkspaces = $derived.by(() => {
    const query = searchQuery.toLowerCase().trim()
    if (!query) return visibleWorkspaces

    return visibleWorkspaces
      .map(ws => ({
        ...ws,
        repos: ws.repos.filter(r => r.name.toLowerCase().includes(query))
      }))
      .filter(ws =>
        ws.name.toLowerCase().includes(query) || ws.repos.length > 0
      )
  })
</script>

<div class:collapsed={sidebarCollapsed} class="tree">
  <div class="tree-header">
    <IconButton
      label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
      onclick={onToggleSidebar}
    >
      {#if sidebarCollapsed}
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M9 18l6-6-6-6" />
        </svg>
      {:else}
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M15 18l-6-6 6-6" />
        </svg>
      {/if}
    </IconButton>
    <span class="title" class:collapsed={sidebarCollapsed}>Workspaces</span>
    <IconButton label="Create workspace" onclick={onCreateWorkspace}>
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M12 5v14m-7-7h14" />
      </svg>
    </IconButton>
  </div>
  {#if !sidebarCollapsed}
    <div class="search-bar">
      <svg class="search-icon" viewBox="0 0 24 24" aria-hidden="true">
        <circle cx="11" cy="11" r="8" />
        <path d="m21 21-4.35-4.35" />
      </svg>
      <input
        type="text"
        placeholder="Filter workspaces..."
        bind:value={searchQuery}
        class="search-input"
      />
      {#if searchQuery}
        <button class="clear-btn" onclick={() => searchQuery = ''} type="button" aria-label="Clear search">
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M18 6 6 18M6 6l12 12" />
          </svg>
        </button>
      {/if}
    </div>
  {/if}
  {#each filteredWorkspaces as workspace}
    <div class="workspace">
      <div class="workspace-row">
        <button
          class="toggle"
          class:expanded={!collapsed[workspace.id]}
          aria-label="Toggle workspace"
          onclick={(event) => {
            event.stopPropagation()
            toggleWorkspace(workspace.id)
          }}
          type="button"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M9 18l6-6-6-6" />
          </svg>
        </button>
        <button
          class:active={workspace.id === activeWorkspaceId}
          class="workspace-button"
          onclick={() => onSelectWorkspace(workspace.id)}
          type="button"
        >
          <span class="initial">{workspace.name.charAt(0).toUpperCase()}</span>
          <span class="name">{workspace.name}</span>
          <span class="count">{workspace.repos.length}</span>
        </button>
        <div class="menu">
          <IconButton
            label="Workspace actions"
            onclick={() => (workspaceMenu = workspaceMenu === workspace.id ? null : workspace.id)}
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <circle cx="5" cy="12" r="1.5" fill="currentColor" stroke="none" />
              <circle cx="12" cy="12" r="1.5" fill="currentColor" stroke="none" />
              <circle cx="19" cy="12" r="1.5" fill="currentColor" stroke="none" />
            </svg>
          </IconButton>
          <DropdownMenu
            open={workspaceMenu === workspace.id}
            onClose={() => (workspaceMenu = null)}
          >
            <button
              type="button"
              onclick={() => {
                workspaceMenu = null
                onAddRepo(workspace.id)
              }}
            >
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14m-7-7h14" /></svg>
              Add repo
            </button>
            <button
              type="button"
              onclick={() => {
                workspaceMenu = null
                onManageWorkspace(workspace.id, 'rename')
              }}
            >
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" /><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" /></svg>
              Rename
            </button>
            <button
              type="button"
              onclick={() => {
                workspaceMenu = null
                onManageWorkspace(workspace.id, 'archive')
              }}
            >
              <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="2" y="4" width="20" height="5" rx="1" /><path d="M4 9v9a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9" /><path d="M10 13h4" /></svg>
              Archive
            </button>
            <button
              class="danger"
              type="button"
              onclick={() => {
                workspaceMenu = null
                onManageWorkspace(workspace.id, 'remove')
              }}
            >
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg>
              Remove
            </button>
          </DropdownMenu>
        </div>
      </div>
      {#if !collapsed[workspace.id]}
        <div class="repos">
          {#each workspace.repos as repo}
            <div class="repo-row">
              <button
                class:active={workspace.id === activeWorkspaceId && repo.id === activeRepoId}
                class="repo-button"
                onclick={() => {
                  onSelectWorkspace(workspace.id)
                  onSelectRepo(repo.id)
                }}
                type="button"
              >
                <span class="repo-name">{repo.name}</span>
                  <span class="meta">
                  {#if repo.remote || repo.defaultBranch}
                    <span class="branch">
                      {repo.remote && repo.defaultBranch
                        ? `${repo.remote}/${repo.defaultBranch}`
                        : repo.defaultBranch ?? repo.remote}
                    </span>
                  {/if}
                  {#if repo.statusKnown === false}
                    <svg class="status-dot unknown" viewBox="0 0 6 6" role="img" aria-label="Status pending">
                      <title>Status pending</title>
                      <circle cx="3" cy="3" r="3" />
                    </svg>
                  {:else if repo.missing}
                    <svg class="status-dot missing" viewBox="0 0 6 6" role="img" aria-label="Missing">
                      <title>Missing</title>
                      <circle cx="3" cy="3" r="3" />
                    </svg>
                  {:else if repo.diff.added + repo.diff.removed > 0}
                    <svg class="status-dot changes" viewBox="0 0 6 6" role="img" aria-label="+{repo.diff.added}/-{repo.diff.removed}">
                      <title>+{repo.diff.added}/-{repo.diff.removed}</title>
                      <circle cx="3" cy="3" r="3" />
                    </svg>
                  {:else if repo.dirty}
                    <svg class="status-dot modified" viewBox="0 0 6 6" role="img" aria-label="Modified">
                      <title>Modified</title>
                      <circle cx="3" cy="3" r="3" />
                    </svg>
                  {:else}
                    <svg class="status-dot clean" viewBox="0 0 6 6" role="img" aria-label="Clean">
                      <title>Clean</title>
                      <circle cx="3" cy="3" r="3" />
                    </svg>
                  {/if}
                </span>
              </button>
              <div class="repo-actions">
                <IconButton
                  label="Repo actions"
                  onclick={() => (repoMenu = repoMenu === repo.id ? null : repo.id)}
                >
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <circle cx="5" cy="12" r="1.5" fill="currentColor" stroke="none" />
                    <circle cx="12" cy="12" r="1.5" fill="currentColor" stroke="none" />
                    <circle cx="19" cy="12" r="1.5" fill="currentColor" stroke="none" />
                  </svg>
                </IconButton>
                <DropdownMenu
                  open={repoMenu === repo.id}
                  onClose={() => (repoMenu = null)}
                >
                  <button
                    class="danger"
                    type="button"
                    onclick={() => {
                      repoMenu = null
                      onManageRepo(workspace.id, repo.name, 'remove')
                    }}
                  >
                    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg>
                    Remove
                  </button>
                </DropdownMenu>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/each}
</div>

<style>
  .tree {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .search-bar {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-2) var(--space-3);
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    margin: 0 var(--space-2);
  }

  .search-icon {
    width: 14px;
    height: 14px;
    stroke: var(--muted);
    stroke-width: 2;
    fill: none;
    flex-shrink: 0;
  }

  .search-input {
    flex: 1;
    background: none;
    border: none;
    color: var(--text);
    font-size: 13px;
    outline: none;
    min-width: 0;
  }

  .search-input::placeholder {
    color: var(--muted);
  }

  .clear-btn {
    background: none;
    border: none;
    padding: 2px;
    cursor: pointer;
    color: var(--muted);
    display: grid;
    place-items: center;
  }

  .clear-btn:hover {
    color: var(--text);
  }

  .clear-btn svg {
    width: 14px;
    height: 14px;
    stroke: currentColor;
    stroke-width: 2;
    fill: none;
  }

  .tree-header {
    display: grid;
    grid-template-columns: 28px 1fr 28px;
    align-items: center;
    gap: var(--space-2);
    font-size: 13px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
    padding: 0 var(--space-3);
  }

  .tree-header .title.collapsed {
    opacity: 0;
  }

  .workspace {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: var(--space-2);
    margin: var(--space-1) var(--space-2);
    transition: border-color var(--transition-normal), background var(--transition-normal);
  }

  .workspace:hover {
    border-color: color-mix(in srgb, var(--border) 70%, var(--accent) 30%);
    background: linear-gradient(
      135deg,
      var(--panel-soft) 0%,
      color-mix(in srgb, var(--panel-soft) 95%, var(--accent) 5%) 100%
    );
  }

  .workspace-row {
    display: grid;
    grid-template-columns: 20px 1fr 28px;
    align-items: center;
    gap: var(--space-1);
  }

  .toggle {
    background: none;
    border: none;
    color: var(--muted);
    cursor: pointer;
    position: relative;
    z-index: 3;
    padding: 2px 4px;
    pointer-events: auto;
    display: grid;
    place-items: center;
  }

  .toggle svg {
    width: 14px;
    height: 14px;
    stroke: currentColor;
    stroke-width: 1.6;
    fill: none;
    transition: transform var(--transition-fast);
  }

  .toggle.expanded svg {
    transform: rotate(90deg);
  }

  .workspace-button {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: none;
    border: 1px solid transparent;
    color: var(--text);
    padding: 6px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    position: relative;
    transition: border-color var(--transition-normal), background var(--transition-normal);
    z-index: 1;
    min-width: 0;
  }

  .workspace-button:hover:not(.active) {
    border-color: var(--border);
    background: rgba(255, 255, 255, 0.02);
  }

  .workspace-button .name {
    font-size: 14px;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .workspace-button.active {
    background: var(--panel);
    border: 1px solid var(--accent);
    box-shadow: inset 3px 0 0 var(--accent);
  }

  .workspace-button .count {
    color: var(--muted);
    font-size: 12px;
  }

  .repos {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-left: 24px;
  }

  .repo-row {
    display: grid;
    grid-template-columns: 1fr 28px;
    align-items: center;
    gap: var(--space-1);
  }

  .repo-button {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    background: none;
    border: 1px solid transparent;
    color: var(--text);
    padding: 6px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    transition: border-color var(--transition-fast), background var(--transition-fast);
    min-width: 0;
  }

  .repo-button:hover {
    border-color: var(--border);
    background: rgba(255, 255, 255, 0.02);
  }

  .repo-button.active {
    background: var(--accent-subtle);
    border-color: var(--accent-soft);
  }

  .repo-name {
    font-size: 13px;
    font-weight: 500;
  }

  .initial {
    display: none;
    width: 36px;
    height: 36px;
    border-radius: var(--radius-sm);
    background: var(--accent-subtle);
    color: var(--accent);
    font-weight: 600;
    font-size: 14px;
    place-items: center;
    border: 1px solid transparent;
    transition:
      transform var(--transition-fast),
      border-color var(--transition-fast),
      background var(--transition-fast);
  }

  .tree.collapsed .initial {
    display: grid;
  }

  .tree.collapsed .initial:hover {
    transform: scale(1.05);
    border-color: var(--accent-soft);
    background: color-mix(in srgb, var(--accent-subtle) 80%, var(--accent) 20%);
  }

  .tree.collapsed .workspace-button .name,
  .tree.collapsed .workspace-button .count,
  .tree.collapsed .repo-name,
  .tree.collapsed .meta,
  .tree.collapsed .branch {
    display: none;
  }

  .tree.collapsed .repos,
  .tree.collapsed .repo-actions,
  .tree.collapsed .menu {
    display: none;
  }

  .tree.collapsed .toggle {
    display: none;
  }

  .tree.collapsed .workspace {
    padding: 6px;
    margin: 2px 6px;
  }

  .tree.collapsed .workspace-row {
    grid-template-columns: 1fr;
  }

  .tree.collapsed .workspace-button {
    padding: var(--space-1);
    justify-content: center;
  }

  .tree.collapsed .workspace-button.active {
    box-shadow: none;
    border-color: var(--accent);
  }

  .tree.collapsed .tree-header {
    grid-template-columns: 1fr;
    justify-items: center;
    padding: 0 6px;
  }

  .tree.collapsed .tree-header .title,
  .tree.collapsed .tree-header > :global(*:last-child) {
    display: none;
  }

  .menu {
    position: relative;
    width: 28px;
    display: grid;
    place-items: center;
  }

  .repo-actions {
    position: relative;
    display: grid;
    place-items: center;
    width: 28px;
    padding-left: 0;
  }

  .meta {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-size: 11px;
    color: var(--muted);
    white-space: nowrap;
    flex-shrink: 0;
  }

  .status-dot {
    width: 6px;
    height: 6px;
    flex-shrink: 0;
  }

  .status-dot.missing {
    fill: var(--danger);
  }

  .status-dot.unknown {
    fill: var(--muted);
  }

  .status-dot.modified {
    fill: var(--warning);
  }

  .status-dot.changes {
    fill: var(--accent);
  }

  .status-dot.clean {
    fill: var(--success);
  }
</style>
