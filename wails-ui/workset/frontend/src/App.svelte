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
  import TerminalWorkspace from './lib/components/TerminalWorkspace.svelte'
  import WorkspaceActionModal from './lib/components/WorkspaceActionModal.svelte'
  import WorkspaceTree from './lib/components/WorkspaceTree.svelte'
  import {fetchAppVersion} from './lib/api'
  import type {AppVersion} from './lib/types'

  let hasWorkspace = $derived($activeWorkspace !== null)
  let hasRepo = $derived($activeRepo !== null)
  let hasWorkspaces = $derived($workspaces.length > 0)
  let settingsOpen = $state(false)
  let sidebarCollapsed = $state(false)
  let actionOpen = $state(false)
  let appVersion = $state<AppVersion | null>(null)
  let versionLabel = $derived(
    appVersion
      ? `${appVersion.version}${appVersion.dirty ? '+dirty' : ''} (${appVersion.commit ? appVersion.commit.slice(0, 7) : 'unknown'})`
      : ''
  )
  let versionTitle = $derived(
    appVersion
      ? `${appVersion.version}${appVersion.dirty ? '+dirty' : ''} (${appVersion.commit || 'unknown'})`
      : ''
  )
  let actionContext: {
    mode:
      | 'create'
      | 'rename'
      | 'add-repo'
      | 'archive'
      | 'remove-workspace'
      | 'remove-repo'
      | null
    workspaceId: string | null
    repoName: string | null
  } = $state({
    mode: null,
    workspaceId: null,
    repoName: null
  })

  const openAction = (
    mode:
      | 'create'
      | 'rename'
      | 'add-repo'
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
    void (async () => {
      try {
        appVersion = await fetchAppVersion()
      } catch {
        appVersion = null
      }
    })()
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
      <button class="icon-button" type="button" onclick={() => (settingsOpen = true)} aria-label="Settings">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <circle cx="12" cy="12" r="3" />
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
        </svg>
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
          openAction('remove-repo', workspaceId, repoName)
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
          <button class="retry" onclick={() => loadWorkspaces()} type="button">Retry</button>
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
      {:else}
        <div class="view-stack">
          <div class="view-pane" class:active={!hasRepo} aria-hidden={hasRepo}>
            <TerminalWorkspace
              workspaceId={$activeWorkspace?.id ?? ''}
              workspaceName={$activeWorkspace?.name ?? 'Workspace'}
              active={!hasRepo}
            />
          </div>
          {#if hasRepo}
            <div class="view-pane active" aria-hidden={!hasRepo}>
              {#key $activeRepoId}
                <RepoDiff
                  repo={$activeRepo!}
                  workspaceId={$activeWorkspaceId ?? ''}
                  onClose={clearRepo}
                />
              {/key}
            </div>
          {/if}
        </div>
      {/if}
    </main>
  </div>

  {#if settingsOpen}
    <div
      class="overlay"
      role="button"
      tabindex="0"
      onclick={() => (settingsOpen = false)}
      onkeydown={(event) => {
        if (event.key === 'Escape') settingsOpen = false
      }}
    >
      <div
        class="overlay-panel"
        role="presentation"
        onclick={(event) => event.stopPropagation()}
        onkeydown={(event) => event.stopPropagation()}
      >
        <SettingsPanel onClose={() => (settingsOpen = false)} />
      </div>
    </div>
  {/if}

  {#if actionOpen}
    <div
      class="overlay"
      role="button"
      tabindex="0"
      onclick={() => (actionOpen = false)}
      onkeydown={(event) => {
        if (event.key === 'Escape') actionOpen = false
      }}
    >
      <div
        class="overlay-panel"
        role="presentation"
        onclick={(event) => event.stopPropagation()}
        onkeydown={(event) => event.stopPropagation()}
      >
        <WorkspaceActionModal
          onClose={() => (actionOpen = false)}
          mode={actionContext.mode}
          workspaceId={actionContext.workspaceId}
          repoName={actionContext.repoName}
        />
      </div>
    </div>
  {/if}

  <footer class="app-footer" aria-label="App version">
    {#if appVersion}
      <span class="app-version" title={versionTitle}>{versionLabel}</span>
    {/if}
  </footer>
</div>

<style>
  .app {
    height: 100vh;
    display: grid;
    grid-template-rows: auto 1fr auto;
    overflow: hidden;
  }

  .topbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 24px;
    padding-left: 88px; /* Space for traffic lights */
    border-bottom: 1px solid var(--border);
    background: var(--panel-strong);
    --wails-draggable: drag;
  }

  .brand {
    display: flex;
    flex-direction: column;
    gap: 2px;
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
    --wails-draggable: no-drag;
  }

  .icon-button {
    width: 36px;
    height: 36px;
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
    width: 18px;
    height: 18px;
    stroke: currentColor;
    stroke-width: 1.6;
    fill: none;
  }

  .layout {
    display: grid;
    grid-template-columns: 280px 1fr;
    height: 100%;
    min-height: 0;
  }

  .layout.collapsed {
    grid-template-columns: 72px 1fr;
  }

  .sidebar {
    border-right: 1px solid rgba(255, 255, 255, 0.06);
    padding: 20px 12px;
    background: var(--panel);
    transition: width 160ms ease, padding 160ms ease;
    overflow-y: auto;
    min-height: 0;
  }

  .sidebar.collapsed {
    width: 72px;
    padding: 20px 8px;
  }

  .main {
    padding: 8px;
    overflow: hidden;
    background: transparent; /* Let vibrancy show through */
  }

  .view-stack {
    position: relative;
    height: 100%;
    min-height: 0;
  }

  .view-pane {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    min-height: 0;
    opacity: 0;
    pointer-events: none;
    transition: opacity var(--transition-fast);
  }

  .view-pane.active {
    opacity: 1;
    pointer-events: auto;
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

  .app-footer {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    padding: 10px 16px;
    border-top: 1px solid var(--border);
    background: var(--panel);
    color: var(--muted);
    font-size: 12px;
    --wails-draggable: no-drag;
  }

  .app-version {
    font-family: var(--font-mono);
    letter-spacing: 0.02em;
  }
</style>
