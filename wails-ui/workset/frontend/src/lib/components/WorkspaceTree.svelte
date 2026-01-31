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
  let activeDropdown: 'workspace' | 'repo' | null = $state(null)
  let workspaceTrigger: HTMLElement | null = $state(null)
  let repoTrigger: HTMLElement | null = $state(null)

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

  const openWorkspaceMenu = (workspaceId: string, trigger: HTMLElement): void => {
    activeDropdown = 'workspace'
    workspaceMenu = workspaceId
    workspaceTrigger = trigger
  }

  const openRepoMenu = (repoId: string, trigger: HTMLElement): void => {
    activeDropdown = 'repo'
    repoMenu = repoId
    repoTrigger = trigger
  }

  const closeMenus = (): void => {
    workspaceMenu = null
    repoMenu = null
    activeDropdown = null
    workspaceTrigger = null
    repoTrigger = null
  }
</script>

<div class:collapsed={sidebarCollapsed} class="tree">
  <div class="tree-header">
    <button
      class="header-btn collapse-btn"
      type="button"
      aria-label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
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
    </button>
    <span class="title" class:collapsed={sidebarCollapsed}>Workspaces</span>
    <div class="header-spacer"></div>
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
  
  <div class="workspace-list">
    {#each filteredWorkspaces as workspace, index}
      <div class="workspace-item" class:first={index === 0}>
        <div class="workspace-header" class:active={workspace.id === activeWorkspaceId}>
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
            class="workspace-name"
            onclick={() => onSelectWorkspace(workspace.id)}
            type="button"
          >
            <span class="initial">{workspace.name.slice(0, 2).toUpperCase()}</span>
            <span class="name-text">{workspace.name}</span>
            <span class="count">{workspace.repos.length}</span>
          </button>
          
          <div class="workspace-actions">
            <button
              class="menu-trigger"
              type="button"
              aria-label="Workspace actions"
              onclick={(event) => openWorkspaceMenu(workspace.id, event.currentTarget)}
            >
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="5" cy="12" r="1.5" fill="currentColor" stroke="none" />
                <circle cx="12" cy="12" r="1.5" fill="currentColor" stroke="none" />
                <circle cx="19" cy="12" r="1.5" fill="currentColor" stroke="none" />
              </svg>
            </button>
            <DropdownMenu
              open={workspaceMenu === workspace.id}
              onClose={closeMenus}
              position="left"
              trigger={workspaceTrigger}
            >
              <button
                type="button"
                onclick={() => {
                  closeMenus()
                  onAddRepo(workspace.id)
                }}
              >
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14m-7-7h14" /></svg>
                Add repo
              </button>
              <button
                type="button"
                onclick={() => {
                  closeMenus()
                  onManageWorkspace(workspace.id, 'rename')
                }}
              >
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" /><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" /></svg>
                Rename
              </button>
              <button
                type="button"
                onclick={() => {
                  closeMenus()
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
                  closeMenus()
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
          <div class="repo-list">
            {#each workspace.repos as repo}
              <div class="repo-item">
                <button
                  class="repo-button"
                  class:active={workspace.id === activeWorkspaceId && repo.id === activeRepoId}
                  onclick={() => {
                    onSelectWorkspace(workspace.id)
                    onSelectRepo(repo.id)
                  }}
                  type="button"
                >
                  <span class="repo-name">{repo.name}</span>
                  <span class="repo-meta">
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
                  <button
                    class="menu-trigger"
                    type="button"
                    aria-label="Repo actions"
                    onclick={(event) => openRepoMenu(repo.id, event.currentTarget)}
                  >
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                      <circle cx="5" cy="12" r="1.5" fill="currentColor" stroke="none" />
                      <circle cx="12" cy="12" r="1.5" fill="currentColor" stroke="none" />
                      <circle cx="19" cy="12" r="1.5" fill="currentColor" stroke="none" />
                    </svg>
                  </button>
                  <DropdownMenu
                    open={repoMenu === repo.id}
                    onClose={closeMenus}
                    position="left"
                    trigger={repoTrigger}
                  >
                    <button
                      class="danger"
                      type="button"
                      onclick={() => {
                        closeMenus()
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
  {#if !sidebarCollapsed}
    <button class="new-workspace-btn" type="button" onclick={onCreateWorkspace}>
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M12 5v14m-7-7h14" />
      </svg>
      <span>New Workspace</span>
    </button>
  {/if}
</div>

<style>
  .tree {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  .search-bar {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid transparent;
    border-radius: var(--radius-lg);
    margin: 0 var(--space-2);
    transition: all 0.2s ease;
    flex-shrink: 0;
  }

  .search-bar:hover {
    background: rgba(255, 255, 255, 0.05);
  }

  .search-bar:focus-within {
    background: rgba(255, 255, 255, 0.04);
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-subtle);
  }

  .search-icon {
    width: 14px;
    height: 14px;
    stroke: var(--muted);
    stroke-width: 1.5;
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
    transition: color 0.15s ease;
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
    font-size: 12px;
    font-weight: 600;
    color: var(--text);
    padding: 0 var(--space-3) var(--space-3);
    flex-shrink: 0;
  }

  .tree-header .title.collapsed {
    opacity: 0;
  }

  .header-btn {
    width: 24px;
    height: 24px;
    border-radius: var(--radius-sm);
    border: none;
    background: transparent;
    color: var(--muted);
    cursor: pointer;
    display: grid;
    place-items: center;
    padding: 0;
    transition: all 0.15s ease;
    opacity: 0.6;
  }

  .header-btn:hover {
    background: rgba(255, 255, 255, 0.06);
    color: var(--text);
    opacity: 1;
  }

  .header-btn:active {
    transform: scale(0.92);
  }

  .header-btn svg {
    width: 14px;
    height: 14px;
    stroke: currentColor;
    stroke-width: 1.8;
    fill: none;
  }

  .header-spacer {
    width: 24px;
  }

  .new-workspace-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-2);
    margin: var(--space-2);
    background: transparent;
    border: 1px dashed rgba(255, 255, 255, 0.15);
    border-radius: var(--radius-md);
    color: var(--muted);
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s ease;
    flex-shrink: 0;
  }

  .new-workspace-btn:hover {
    background: rgba(255, 255, 255, 0.04);
    border-color: rgba(255, 255, 255, 0.25);
    color: var(--text);
  }

  .new-workspace-btn:active {
    transform: scale(0.98);
  }

  .new-workspace-btn svg {
    width: 14px;
    height: 14px;
    stroke: currentColor;
    stroke-width: 1.8;
    fill: none;
  }

  .workspace-list {
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    overflow-x: visible;
    padding: 0 var(--space-1) var(--space-2);
    flex: 1;
    min-height: 0;
  }

  .workspace-item {
    display: flex;
    flex-direction: column;
    margin-bottom: var(--space-1);
  }

  .workspace-item.first {
    margin-top: var(--space-1);
  }

  .workspace-header {
    display: grid;
    grid-template-columns: 20px 1fr auto;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-2) var(--space-2);
    transition: all 0.15s ease;
    position: relative;
    border-radius: var(--radius-md);
  }

  .workspace-header:hover {
    background: rgba(255, 255, 255, 0.04);
  }

  .workspace-header.active {
    background: var(--accent-subtle);
  }

  .workspace-header.active::before {
    content: '';
    position: absolute;
    left: 0;
    top: 6px;
    bottom: 6px;
    width: 2px;
    background: var(--accent);
    border-radius: 0 1px 1px 0;
  }

  .toggle {
    background: none;
    border: none;
    color: var(--muted);
    cursor: pointer;
    padding: 2px;
    display: grid;
    place-items: center;
    transition: color 0.15s ease;
  }

  .toggle:hover {
    color: var(--text);
  }

  .toggle svg {
    width: 14px;
    height: 14px;
    stroke: currentColor;
    stroke-width: 2;
    fill: none;
    transition: transform 0.2s cubic-bezier(0.34, 1.56, 0.64, 1);
  }

  .toggle.expanded svg {
    transform: rotate(90deg);
  }

  .workspace-name {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    background: none;
    border: none;
    color: var(--text);
    cursor: pointer;
    text-align: left;
    padding: 4px;
    border-radius: var(--radius-sm);
    transition: background 0.15s ease;
    min-width: 0;
  }

  .workspace-name:hover {
    background: rgba(255, 255, 255, 0.02);
  }

  .workspace-header.active .workspace-name:hover {
    background: rgba(255, 255, 255, 0.04);
  }

  .name-text {
    font-size: 13px;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }

  .count {
    color: var(--muted);
    font-size: 11px;
    font-weight: 500;
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
  }

  .workspace-actions {
    position: relative;
    display: grid;
    place-items: center;
    width: 26px;
    opacity: 0;
    transition: opacity 0.15s ease;
    pointer-events: none;
  }

  .workspace-header:hover .workspace-actions {
    opacity: 1;
    pointer-events: auto;
  }

  .menu-trigger {
    width: 24px;
    height: 24px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: rgba(255, 255, 255, 0.02);
    color: var(--text);
    cursor: pointer;
    display: grid;
    place-items: center;
    padding: 0;
    transition: all 0.15s ease;
  }

  .menu-trigger:hover {
    border-color: var(--accent);
    background: rgba(255, 255, 255, 0.04);
  }

  .menu-trigger:active {
    transform: scale(0.95);
  }

  .menu-trigger svg {
    width: 14px;
    height: 14px;
    stroke: currentColor;
    stroke-width: 1.6;
    fill: none;
  }

  .repo-list {
    display: flex;
    flex-direction: column;
    padding-left: 24px;
    padding-top: var(--space-1);
    padding-bottom: var(--space-2);
    gap: 2px;
  }

  .repo-item {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: var(--space-1);
    border-radius: var(--radius-sm);
    transition: background 0.15s ease;
  }

  .repo-item:hover {
    background: rgba(255, 255, 255, 0.03);
  }

  .repo-button {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    background: none;
    border: none;
    color: var(--text);
    padding: 5px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    transition: all 0.15s ease;
    min-width: 0;
  }

  .repo-button.active {
    background: var(--accent-subtle);
  }

  .repo-name {
    font-size: 12px;
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }

  .repo-meta {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex-shrink: 0;
  }

  .branch {
    font-family: var(--font-mono);
    font-size: 9px;
    color: var(--muted);
    opacity: 0.6;
    letter-spacing: 0.02em;
  }

  .status-dot {
    width: 4px;
    height: 4px;
    flex-shrink: 0;
    opacity: 0.8;
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

  .repo-actions {
    position: relative;
    display: grid;
    place-items: center;
    width: 24px;
    padding-right: var(--space-1);
    opacity: 0;
    transition: opacity 0.15s ease;
    pointer-events: none;
  }

  .repo-item:hover .repo-actions {
    opacity: 1;
    pointer-events: auto;
  }

  .initial {
    display: none;
    min-width: 28px;
    width: auto;
    height: 28px;
    padding: 0 4px;
    border-radius: var(--radius-md);
    background: var(--accent-subtle);
    color: var(--accent);
    font-weight: 600;
    font-size: 11px;
    place-items: center;
    flex-shrink: 0;
    letter-spacing: -0.02em;
  }

  /* Collapsed state */
  .tree.collapsed .workspace-list {
    padding: 0 var(--space-1);
  }

  .tree.collapsed .workspace-item {
    border: none;
    border-bottom: 1px solid var(--border);
  }

  .tree.collapsed .workspace-item.first {
    border-top: 1px solid var(--border);
  }

  .tree.collapsed .toggle,
  .tree.collapsed .name-text,
  .tree.collapsed .count,
  .tree.collapsed .repo-list,
  .tree.collapsed .workspace-actions {
    display: none;
  }

  .tree.collapsed .workspace-header {
    grid-template-columns: 1fr;
    justify-items: center;
    padding: var(--space-2);
  }

  .tree.collapsed .workspace-name {
    justify-content: center;
    padding: var(--space-1);
    width: 100%;
  }

  .tree.collapsed .initial {
    display: grid;
  }

  .tree.collapsed .initial:hover {
    transform: scale(1.05);
  }

  .tree.collapsed .workspace-header.active::before {
    left: 2px;
  }

  .tree.collapsed .tree-header {
    grid-template-columns: 1fr;
    justify-items: center;
    padding: 0 var(--space-2);
  }

  .tree.collapsed .tree-header .title,
  .tree.collapsed .tree-header > :global(*:last-child) {
    display: none;
  }

  .tree.collapsed .search-bar {
    display: none;
  }
</style>
