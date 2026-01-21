<script lang="ts">
  import type {Workspace} from '../types'
  import {clickOutside} from '../actions/clickOutside'

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
    action: 'remotes' | 'remove'
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
</script>

<div class:collapsed={sidebarCollapsed} class="tree">
  <div class="tree-header">
    <button class="icon-button" type="button" onclick={onToggleSidebar} aria-label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}>
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
    <button class="icon-button" type="button" onclick={onCreateWorkspace} aria-label="Create workspace">
      +
    </button>
  </div>
  {#each visibleWorkspaces as workspace}
    <div class="workspace">
      <div class="workspace-row">
        <button
          class="toggle"
          aria-label="Toggle workspace"
          onclick={(event) => {
            event.stopPropagation()
            toggleWorkspace(workspace.id)
          }}
          type="button"
        >
          {#if collapsed[workspace.id]}▸{:else}▾{/if}
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
          <button
            class="icon-button"
            type="button"
            aria-label="Workspace actions"
            onclick={() => (workspaceMenu = workspaceMenu === workspace.id ? null : workspace.id)}
          >
            ⋯
          </button>
          {#if workspaceMenu === workspace.id}
            <div class="menu-card" use:clickOutside={() => (workspaceMenu = null)}>
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
            </div>
          {/if}
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
                  {#if repo.branch}
                    <span class="branch">{repo.branch}</span>
                  {/if}
                  {#if repo.statusKnown === false}
                    <span class="status-dot unknown" title="Status pending">●</span>
                  {:else if repo.missing}
                    <span class="status-dot missing" title="Missing">●</span>
                  {:else if repo.diff.added + repo.diff.removed > 0}
                    <span class="status-dot changes" title="+{repo.diff.added}/-{repo.diff.removed}">●</span>
                  {:else if repo.dirty}
                    <span class="status-dot modified" title="Modified">●</span>
                  {:else}
                    <span class="status-dot clean" title="Clean">●</span>
                  {/if}
                </span>
              </button>
              <div class="repo-actions">
                <button
                  class="icon-button"
                  type="button"
                  aria-label="Repo actions"
                  onclick={() => (repoMenu = repoMenu === repo.id ? null : repo.id)}
                >
                  ⋯
                </button>
                {#if repoMenu === repo.id}
                  <div class="menu-card" use:clickOutside={() => (repoMenu = null)}>
                    <button
                      type="button"
                      onclick={() => {
                        repoMenu = null
                        onManageRepo(workspace.id, repo.name, 'remotes')
                      }}
                    >
                      <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="18" cy="18" r="3" /><circle cx="6" cy="6" r="3" /><path d="M6 21V9a9 9 0 0 0 9 9" /></svg>
                      Remotes
                    </button>
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
                  </div>
                {/if}
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
    gap: 12px;
  }

  .tree-header {
    display: grid;
    grid-template-columns: 28px 1fr 28px;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
    padding: 0 12px;
  }

  .tree-header .title.collapsed {
    opacity: 0;
  }

  .workspace {
    display: flex;
    flex-direction: column;
    gap: 4px;
    background: var(--panel-soft);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    padding: 8px;
    margin: 4px 8px;
  }

  .workspace-row {
    display: grid;
    grid-template-columns: 20px 1fr 28px;
    align-items: center;
    gap: 4px;
  }

  .toggle {
    background: none;
    border: none;
    color: var(--muted);
    cursor: pointer;
    font-size: 14px;
    position: relative;
    z-index: 3;
    padding: 2px 4px;
    pointer-events: auto;
  }

  .workspace-button {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: none;
    border: 1px solid transparent;
    color: var(--text);
    padding: 6px 8px;
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
    gap: 4px;
  }

  .repo-button {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    background: none;
    border: 1px solid transparent;
    color: var(--text);
    padding: 6px 8px;
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
    width: 32px;
    height: 32px;
    border-radius: var(--radius-sm);
    background: var(--accent-subtle);
    color: var(--accent);
    font-weight: 600;
    font-size: 14px;
    place-items: center;
  }

  .tree.collapsed .initial {
    display: grid;
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
    padding: 4px;
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
  .tree.collapsed .tree-header .icon-button:last-child {
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

  .menu-card {
    position: absolute;
    right: 0;
    top: 28px;
    background: var(--panel-strong);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 6px;
    display: grid;
    gap: 4px;
    z-index: 5;
    min-width: 140px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
  }

  .menu-card button {
    display: flex;
    align-items: center;
    gap: 8px;
    background: none;
    border: none;
    color: var(--text);
    text-align: left;
    padding: 8px 12px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background var(--transition-fast);
  }

  .menu-card button svg {
    width: 14px;
    height: 14px;
    stroke: currentColor;
    stroke-width: 1.6;
    fill: none;
    flex-shrink: 0;
  }

  .menu-card button:hover {
    background: rgba(255, 255, 255, 0.06);
  }

  .menu-card button.danger {
    color: var(--danger);
  }

  .menu-card button.danger:hover {
    background: rgba(var(--danger-rgb, 239, 68, 68), 0.15);
  }

  .icon-button {
    width: 28px;
    height: 28px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: rgba(255, 255, 255, 0.02);
    color: var(--text);
    cursor: pointer;
    display: grid;
    place-items: center;
    transition: border-color var(--transition-fast), background var(--transition-fast);
  }

  .icon-button:hover {
    border-color: var(--accent);
    background: rgba(255, 255, 255, 0.04);
  }

  .icon-button svg {
    width: 16px;
    height: 16px;
    stroke: currentColor;
    stroke-width: 1.6;
    fill: none;
  }

  .meta {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    color: var(--muted);
    white-space: nowrap;
    flex-shrink: 0;
  }

  .status-dot {
    font-size: 8px;
    line-height: 1;
  }

  .status-dot.missing {
    color: var(--danger);
  }

  .status-dot.unknown {
    color: var(--muted);
  }

  .status-dot.modified {
    color: var(--warning);
  }

  .status-dot.changes {
    color: var(--accent);
  }

  .status-dot.clean {
    color: var(--success);
  }
</style>
