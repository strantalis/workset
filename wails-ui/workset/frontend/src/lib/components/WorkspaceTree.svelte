<script lang="ts">
  import type {Workspace} from '../types'

  export let workspaces: Workspace[] = []
  export let activeWorkspaceId: string | null = null
  export let activeRepoId: string | null = null
  export let onSelectWorkspace: (workspaceId: string) => void
  export let onSelectRepo: (repoId: string) => void
  export let onCreateWorkspace: () => void
  export let onAddRepo: (workspaceId: string) => void
  export let onManageWorkspace: (workspaceId: string, action: 'rename' | 'archive' | 'remove') => void
  export let onManageRepo: (
    workspaceId: string,
    repoId: string,
    action: 'remotes' | 'remove'
  ) => void
  export let sidebarCollapsed = false
  export let onToggleSidebar: () => void

  let collapsed: Record<string, boolean> = {}
  let workspaceMenu: string | null = null
  let repoMenu: string | null = null

  const isCollapsed = (workspaceId: string): boolean => collapsed[workspaceId] ?? false

  const toggleWorkspace = (workspaceId: string): void => {
    collapsed = {...collapsed, [workspaceId]: !isCollapsed(workspaceId)}
  }

  const repoIsDirty = (added: number, removed: number, dirty: boolean): boolean =>
    dirty || added + removed > 0

  $: visibleWorkspaces = workspaces.filter((workspace) => !workspace.archived)
</script>

<div class:collapsed={sidebarCollapsed} class="tree">
  <div class="tree-header">
    <button class="icon-button" type="button" on:click={onToggleSidebar} aria-label="Collapse sidebar">
      {#if sidebarCollapsed}
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <rect x="3" y="4" width="14" height="16" rx="2" ry="2" />
          <path d="M13 8l4 4-4 4" />
        </svg>
      {:else}
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <rect x="7" y="4" width="14" height="16" rx="2" ry="2" />
          <path d="M11 8l-4 4 4 4" />
        </svg>
      {/if}
    </button>
    <span class="title" class:collapsed={sidebarCollapsed}>Workspaces</span>
    <button class="icon-button" type="button" on:click={onCreateWorkspace} aria-label="Create workspace">
      +
    </button>
  </div>
  {#each visibleWorkspaces as workspace}
    <div class="workspace">
      <div class="workspace-row">
        <button
          class="toggle"
          aria-label="Toggle workspace"
          on:click|stopPropagation={() => toggleWorkspace(workspace.id)}
          type="button"
        >
          {#if isCollapsed(workspace.id)}▸{:else}▾{/if}
        </button>
        <button
          class:active={workspace.id === activeWorkspaceId}
          class="workspace-button"
          on:click={() => onSelectWorkspace(workspace.id)}
          type="button"
        >
          <span class="name">{workspace.name}</span>
          <span class="count">{workspace.repos.length}</span>
        </button>
        <div class="menu">
          <button
            class="icon-button"
            type="button"
            aria-label="Workspace actions"
            on:click={() => (workspaceMenu = workspaceMenu === workspace.id ? null : workspace.id)}
          >
            ⋯
          </button>
          {#if workspaceMenu === workspace.id}
            <div class="menu-card">
              <button
                type="button"
                on:click={() => {
                  workspaceMenu = null
                  onAddRepo(workspace.id)
                }}
              >
                Add repo
              </button>
              <button
                type="button"
                on:click={() => {
                  workspaceMenu = null
                  onManageWorkspace(workspace.id, 'rename')
                }}
              >
                Rename
              </button>
              <button
                type="button"
                on:click={() => {
                  workspaceMenu = null
                  onManageWorkspace(workspace.id, 'archive')
                }}
              >
                Archive
              </button>
              <button
                type="button"
                on:click={() => {
                  workspaceMenu = null
                  onManageWorkspace(workspace.id, 'remove')
                }}
              >
                Remove
              </button>
            </div>
          {/if}
        </div>
      </div>
      {#if !isCollapsed(workspace.id)}
        <div class="repos">
          {#each workspace.repos as repo}
            <div class="repo-row">
              <button
                class:active={workspace.id === activeWorkspaceId && repo.id === activeRepoId}
                class="repo-button"
                on:click={() => {
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
                  {#if repo.missing}
                    <span class="status missing">missing</span>
                  {:else if repoIsDirty(repo.diff.added, repo.diff.removed, repo.dirty)}
                    {#if repo.diff.added + repo.diff.removed > 0}
                      <span class="status diffstat"><span class="add">+{repo.diff.added}</span><span class="sep">/</span><span class="del">-{repo.diff.removed}</span></span>
                    {:else}
                      <span class="status dirty">dirty</span>
                    {/if}
                  {/if}
                </span>
              </button>
              <div class="repo-actions">
                <button
                  class="icon-button"
                  type="button"
                  aria-label="Repo actions"
                  on:click={() => (repoMenu = repoMenu === repo.id ? null : repo.id)}
                >
                  ⋯
                </button>
                {#if repoMenu === repo.id}
                  <div class="menu-card">
                    <button
                      type="button"
                      on:click={() => {
                        repoMenu = null
                        onManageRepo(workspace.id, repo.name, 'remotes')
                      }}
                    >
                      Remotes
                    </button>
                    <button
                      type="button"
                      on:click={() => {
                        repoMenu = null
                        onManageRepo(workspace.id, repo.name, 'remove')
                      }}
                    >
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
    gap: 6px;
    padding: 10px 0 12px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.07);
  }

  .workspace-row {
    display: grid;
    grid-template-columns: 20px 1fr 28px;
    align-items: center;
    gap: 6px;
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
    padding: 8px 12px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    position: relative;
    transition: border-color var(--transition-normal), background var(--transition-normal);
    z-index: 1;
  }

  .workspace-button:hover:not(.active) {
    border-color: var(--border);
    background: rgba(255, 255, 255, 0.02);
  }

  .workspace-button .name {
    font-size: 15px;
    font-weight: 600;
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
    gap: 4px;
    padding-left: 28px;
  }

  .repo-row {
    display: grid;
    grid-template-columns: 1fr 32px;
    align-items: center;
    gap: 6px;
  }

  .repo-button {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    background: none;
    border: 1px solid transparent;
    color: var(--text);
    padding: 6px 10px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
    transition: border-color var(--transition-fast), background var(--transition-fast);
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
  }

  .tree.collapsed .workspace-button,
  .tree.collapsed .repo-button {
    justify-content: center;
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

  .tree.collapsed .workspace-row {
    grid-template-columns: 28px 1fr;
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
  }

  .menu-card button {
    background: none;
    border: none;
    color: var(--text);
    text-align: left;
    padding: 6px 8px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background var(--transition-fast);
  }

  .menu-card button:hover {
    background: rgba(255, 255, 255, var(--opacity-subtle));
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
    gap: 8px;
    font-size: 12px;
    color: var(--muted);
  }

  .status {
    font-weight: 600;
  }

  .dirty {
    color: var(--warning);
  }

  .missing {
    color: var(--danger);
  }

  .diffstat .add {
    color: var(--success);
  }

  .diffstat .del {
    color: var(--danger);
  }

  .diffstat .sep {
    color: var(--muted);
  }
</style>
