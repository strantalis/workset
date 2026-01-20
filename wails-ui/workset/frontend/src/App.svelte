<script lang="ts">
  import {onMount} from 'svelte'
  import {
    activeRepo,
    activeRepoId,
    activeWorkspace,
    activeWorkspaceId,
    clearRepo,
    loadWorkspaces,
    loadingWorkspaces,
    selectRepo,
    selectWorkspace,
    workspaceError,
    workspaces
  } from './lib/state'
  import EmptyState from './lib/components/EmptyState.svelte'
  import RepoDiff from './lib/components/RepoDiff.svelte'
  import SettingsPanel from './lib/components/SettingsPanel.svelte'
  import TerminalPane from './lib/components/TerminalPane.svelte'
  import WorkspaceActionModal from './lib/components/WorkspaceActionModal.svelte'
  import WorkspaceTree from './lib/components/WorkspaceTree.svelte'

  $: hasWorkspace = $activeWorkspace !== null
  $: hasRepo = $activeRepo !== null
  $: hasWorkspaces = $workspaces.length > 0
  let settingsOpen = false
  let sidebarCollapsed = false
  let actionOpen = false
  let actionContext: {
    mode:
      | 'create'
      | 'rename'
      | 'add-repo'
      | 'remotes'
      | 'archive'
      | 'remove-workspace'
      | 'remove-repo'
      | null
    workspaceId: string | null
    repoName: string | null
  } = {
    mode: null,
    workspaceId: null,
    repoName: null
  }

  const openAction = (
    mode:
      | 'create'
      | 'rename'
      | 'add-repo'
      | 'remotes'
      | 'archive'
      | 'remove-workspace'
      | 'remove-repo',
    workspaceId: string | null,
    repoName: string | null
  ): void => {
    actionContext = {mode, workspaceId, repoName}
    actionOpen = true
  }

  onMount(() => {
    void loadWorkspaces()
  })
</script>

<div class="app">
  <header class="topbar">
    <div class="brand">
      <div class="logo">Workset</div>
      {#if hasWorkspace}
        <div class="context">Workspace: {$activeWorkspace?.name}</div>
      {:else}
        <div class="context">Select a workspace to begin</div>
      {/if}
    </div>
    <div class="actions">
      <button class="ghost" type="button">Search</button>
      <button class="ghost" type="button" on:click={() => (settingsOpen = true)}>
        Settings
      </button>
    </div>
  </header>

  <div class:collapsed={sidebarCollapsed} class="layout">
    <aside class:collapsed={sidebarCollapsed} class="sidebar">
      <WorkspaceTree
        workspaces={$workspaces}
        activeWorkspaceId={$activeWorkspaceId}
        activeRepoId={$activeRepoId}
        onSelectWorkspace={selectWorkspace}
        onSelectRepo={selectRepo}
        sidebarCollapsed={sidebarCollapsed}
        onToggleSidebar={() => (sidebarCollapsed = !sidebarCollapsed)}
        onCreateWorkspace={() => openAction('create', null, null)}
        onAddRepo={(workspaceId) => openAction('add-repo', workspaceId, null)}
        onManageWorkspace={(workspaceId, action) => {
          if (action === 'rename') {
            openAction('rename', workspaceId, null)
          } else if (action === 'archive') {
            openAction('archive', workspaceId, null)
          } else {
            openAction('remove-workspace', workspaceId, null)
          }
        }}
        onManageRepo={(workspaceId, repoName, action) => {
          if (action === 'remotes') {
            openAction('remotes', workspaceId, repoName)
          } else {
            openAction('remove-repo', workspaceId, repoName)
          }
        }}
      />
    </aside>

    <main class="main">
      {#if $loadingWorkspaces}
        <EmptyState title="Loading workspaces" body="Fetching your workspace list." />
      {:else if $workspaceError}
        <section class="error">
          <div class="title">Failed to load workspaces</div>
          <div class="body">{$workspaceError}</div>
          <button class="retry" on:click={() => loadWorkspaces()} type="button">Retry</button>
        </section>
      {:else if !hasWorkspace}
        <EmptyState
          title={hasWorkspaces ? 'No workspace selected' : 'No workspaces found'}
          body={
            hasWorkspaces
              ? 'Pick a workspace on the left or create a new one to start.'
              : 'Create your first workspace to begin.'
          }
        />
      {:else if hasRepo}
        {#key $activeRepoId}
          <RepoDiff
            repo={$activeRepo}
            workspaceId={$activeWorkspaceId ?? ''}
            onClose={clearRepo}
          />
        {/key}
      {:else}
        <TerminalPane
          workspaceId={$activeWorkspace?.id ?? ''}
          workspaceName={$activeWorkspace?.name ?? 'Workspace'}
        />
      {/if}
    </main>
  </div>

  {#if settingsOpen}
    <div
      class="overlay"
      role="button"
      tabindex="0"
      on:click={() => (settingsOpen = false)}
      on:keydown={(event) => {
        if (event.key === 'Escape') settingsOpen = false
      }}
    >
      <div class="overlay-panel" role="presentation" on:click|stopPropagation on:keydown|stopPropagation>
        <SettingsPanel onClose={() => (settingsOpen = false)} />
      </div>
    </div>
  {/if}

  {#if actionOpen}
    <div
      class="overlay"
      role="button"
      tabindex="0"
      on:click={() => (actionOpen = false)}
      on:keydown={(event) => {
        if (event.key === 'Escape') actionOpen = false
      }}
    >
      <div class="overlay-panel" role="presentation" on:click|stopPropagation on:keydown|stopPropagation>
        <WorkspaceActionModal
          onClose={() => (actionOpen = false)}
          mode={actionContext.mode}
          workspaceId={actionContext.workspaceId}
          repoName={actionContext.repoName}
        />
      </div>
    </div>
  {/if}
</div>

<style>
  .app {
    min-height: 100vh;
    display: grid;
    grid-template-rows: auto 1fr;
  }

  .topbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 24px;
    border-bottom: 1px solid var(--border);
    background: var(--panel-strong);
  }

  .brand {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .logo {
    font-size: 18px;
    font-weight: 600;
    font-family: var(--font-display);
    letter-spacing: 0.02em;
  }

  .context {
    font-size: 14px;
    color: var(--muted);
    font-weight: 500;
  }

  .actions {
    display: flex;
    gap: 10px;
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

  .layout {
    display: grid;
    grid-template-columns: 280px 1fr;
    height: 100%;
  }

  .layout.collapsed {
    grid-template-columns: 72px 1fr;
  }

  .sidebar {
    border-right: 1px solid var(--border);
    padding: 20px 12px;
    background: var(--panel);
    transition: width 160ms ease, padding 160ms ease;
  }

  .sidebar.collapsed {
    width: 72px;
    padding: 20px 8px;
  }

  .main {
    padding: 24px;
    overflow: hidden;
  }

  .error {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .error .title {
    font-size: 18px;
    font-weight: 600;
  }

  .error .body {
    color: var(--muted);
    font-size: 14px;
  }

  .retry {
    align-self: flex-start;
    background: var(--accent);
    border: none;
    color: #081018;
    padding: 8px 12px;
    border-radius: var(--radius-md);
    font-weight: 600;
    cursor: pointer;
    transition: background var(--transition-fast), transform var(--transition-fast);
  }

  .retry:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 85%, white);
  }

  .retry:active:not(:disabled) {
    transform: scale(0.98);
  }

  .retry:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(6, 9, 14, 0.78);
    display: grid;
    place-items: center;
    z-index: 20;
    padding: 24px;
    animation: overlayFadeIn var(--transition-normal) ease-out;
  }

  .overlay-panel {
    width: 100%;
    display: flex;
    justify-content: center;
    animation: modalSlideIn 200ms ease-out;
  }

  @keyframes overlayFadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }

  @keyframes modalSlideIn {
    from {
      opacity: 0;
      transform: translateY(-8px) scale(0.98);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  @media (max-width: 720px) {
    .overlay {
      padding: 0;
    }
  }
</style>
