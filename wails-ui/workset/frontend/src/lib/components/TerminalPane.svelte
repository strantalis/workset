<script lang="ts">
  import {onDestroy, onMount} from 'svelte'
  import {Terminal} from '@xterm/xterm'
  import {FitAddon} from '@xterm/addon-fit'
  import '@xterm/xterm/css/xterm.css'
  import {EventsOn, EventsOff} from '../../../wailsjs/runtime/runtime'
  import {
    ResizeWorkspaceTerminal,
    StartWorkspaceTerminal,
    WriteWorkspaceTerminal
  } from '../../../wailsjs/go/main/App'

  export let workspaceId: string
  export let workspaceName: string

  let terminalContainer: HTMLDivElement | null = null
  let resizeObserver: ResizeObserver | null = null

  type TerminalHandle = {
    terminal: Terminal
    fitAddon: FitAddon
    dataDisposable: {dispose: () => void}
    container: HTMLDivElement
  }

  const terminals = new Map<string, TerminalHandle>()
  let initCounter = 0
  const startedSessions = new Set<string>()
  let resizeScheduled = false
  let statusMap: Record<string, string> = {}
  let messageMap: Record<string, string> = {}

  $: activeStatus = workspaceId ? statusMap[workspaceId] ?? '' : ''
  $: activeMessage = workspaceId ? messageMap[workspaceId] ?? '' : ''
  const listeners = new Set<string>()

  const getToken = (name: string, fallback: string): string => {
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
    return value || fallback
  }

  const createTerminal = (): Terminal => {
    const themeBackground = getToken('--panel-strong', '#111c29')
    const themeForeground = getToken('--text', '#eef3f9')
    const themeCursor = getToken('--accent', '#2d8cff')
    const themeSelection = getToken('--accent', '#2d8cff')
    const fontMono = getToken('--font-mono', 'Menlo, Consolas, monospace')

    return new Terminal({
      fontFamily: fontMono,
      fontSize: 12,
      lineHeight: 1.4,
      cursorBlink: true,
      scrollback: 4000,
      theme: {
        background: themeBackground,
        foreground: themeForeground,
        cursor: themeCursor,
        selectionBackground: themeSelection
      }
    })
  }

  const attachTerminal = (id: string, name: string): TerminalHandle => {
    let handle = terminals.get(id)
    if (!handle) {
      const terminal = createTerminal()
      const fitAddon = new FitAddon()
      terminal.loadAddon(fitAddon)
      const dataDisposable = terminal.onData((data) => {
        if (!startedSessions.has(id)) {
          void StartWorkspaceTerminal(id)
            .then(() => {
              startedSessions.add(id)
              return WriteWorkspaceTerminal(id, data)
            })
            .catch((error) => {
              terminal.write(`\r\n[workset] write failed: ${String(error)}`)
              terminal.write('\r\n$ ')
            })
          return
        }
        void WriteWorkspaceTerminal(id, data).catch((error) => {
          terminal.write(`\r\n[workset] write failed: ${String(error)}`)
          terminal.write('\r\n$ ')
        })
      })
      terminal.writeln(`Workset terminal — ${name}`)
      terminal.writeln('Workspace scope. Use "cd <repo>" before repo commands.')
      terminal.write('$ ')
      const container = document.createElement('div')
      container.className = 'terminal-instance'
      handle = {terminal, fitAddon, dataDisposable, container}
      terminals.set(id, handle)
    }
    if (terminalContainer) {
      terminalContainer.querySelectorAll('.terminal-instance').forEach((node) => {
        node.setAttribute('data-active', 'false')
      })
      if (!terminalContainer.contains(handle.container)) {
        terminalContainer.appendChild(handle.container)
        handle.terminal.open(handle.container)
      }
      handle.container.setAttribute('data-active', 'true')
      handle.fitAddon.fit()
      const dims = handle.fitAddon.proposeDimensions()
      if (dims) {
        void ResizeWorkspaceTerminal(id, dims.cols, dims.rows).catch(() => undefined)
      }
      handle.terminal.focus()
    }
    return handle
  }

  const ensureListener = (): void => {
    if (!listeners.has('terminal:data')) {
      const handler = (payload: {workspaceId: string; data: string}): void => {
        const term = terminals.get(payload.workspaceId)
        term?.terminal.write(payload.data)
      }
      EventsOn('terminal:data', handler)
      listeners.add('terminal:data')
    }
    if (!listeners.has('terminal:lifecycle')) {
      const handler = (payload: {
        workspaceId: string
        status: 'started' | 'closed' | 'error' | 'idle'
        message?: string
      }): void => {
        if (payload.status === 'started') {
          startedSessions.add(payload.workspaceId)
          statusMap = {...statusMap, [payload.workspaceId]: 'ready'}
          messageMap = {...messageMap, [payload.workspaceId]: ''}
          return
        }
        if (payload.status === 'closed') {
          startedSessions.delete(payload.workspaceId)
          statusMap = {...statusMap, [payload.workspaceId]: 'closed'}
          return
        }
        if (payload.status === 'idle') {
          startedSessions.delete(payload.workspaceId)
          statusMap = {...statusMap, [payload.workspaceId]: 'idle'}
          return
        }
        if (payload.status === 'error') {
          startedSessions.delete(payload.workspaceId)
          statusMap = {...statusMap, [payload.workspaceId]: 'error'}
          messageMap = {
            ...messageMap,
            [payload.workspaceId]: payload.message ?? 'Terminal error'
          }
          const term = terminals.get(payload.workspaceId)
          if (term && payload.message) {
            term.terminal.write(`\r\n[workset] ${payload.message}`)
            term.terminal.write('\r\n$ ')
          }
        }
      }
      EventsOn('terminal:lifecycle', handler)
      listeners.add('terminal:lifecycle')
    }
  }

  const cleanupListeners = (): void => {
    if (listeners.has('terminal:data')) {
      EventsOff('terminal:data')
      listeners.delete('terminal:data')
    }
    if (listeners.has('terminal:lifecycle')) {
      EventsOff('terminal:lifecycle')
      listeners.delete('terminal:lifecycle')
    }
  }

  const initTerminal = async (): Promise<void> => {
    if (!workspaceId) return
    const token = ++initCounter
    ensureListener()
    const handle = attachTerminal(workspaceId, workspaceName)
    try {
      statusMap = {...statusMap, [workspaceId]: 'starting'}
      await StartWorkspaceTerminal(workspaceId)
      startedSessions.add(workspaceId)
      statusMap = {...statusMap, [workspaceId]: 'ready'}
      messageMap = {...messageMap, [workspaceId]: ''}
    } catch (error) {
      if (token !== initCounter) return
      statusMap = {...statusMap, [workspaceId]: 'error'}
      messageMap = {
        ...messageMap,
        [workspaceId]: String(error)
      }
      handle.terminal.write(`\r\n[workset] failed to start terminal: ${String(error)}`)
      handle.terminal.write('\r\n$ ')
    }
  }

  const restartTerminal = async (): Promise<void> => {
    if (!workspaceId) return
    statusMap = {...statusMap, [workspaceId]: 'starting'}
    messageMap = {...messageMap, [workspaceId]: ''}
    try {
      await StartWorkspaceTerminal(workspaceId)
      startedSessions.add(workspaceId)
      statusMap = {...statusMap, [workspaceId]: 'ready'}
    } catch (error) {
      statusMap = {...statusMap, [workspaceId]: 'error'}
      messageMap = {...messageMap, [workspaceId]: String(error)}
    }
  }

  onMount(() => {
    if (!terminalContainer) return
    resizeObserver = new ResizeObserver(() => {
      if (!workspaceId || resizeScheduled) return
      resizeScheduled = true
      requestAnimationFrame(() => {
        resizeScheduled = false
        const handle = terminals.get(workspaceId)
        if (!handle) return
        handle.fitAddon.fit()
        if (!startedSessions.has(workspaceId)) return
        const dims = handle.fitAddon.proposeDimensions()
        if (dims) {
          void ResizeWorkspaceTerminal(workspaceId, dims.cols, dims.rows).catch(() => undefined)
        }
      })
    })
    resizeObserver.observe(terminalContainer)
    void initTerminal()
  })

  $: if (workspaceId && terminalContainer) {
    void initTerminal()
  }

  onDestroy(() => {
    resizeObserver?.disconnect()
    cleanupListeners()
  })
</script>

<section class="terminal">
  <header class="terminal-header">
    <div class="title">Workspace terminal</div>
    <div class="meta">Workspace: {workspaceName}</div>
  </header>
  <div class="terminal-body">
    {#if activeStatus && activeStatus !== 'ready'}
      <div class="terminal-status">
        <div class="status-text">
          {#if activeStatus === 'idle'}
            Terminal suspended due to inactivity.
          {:else if activeStatus === 'error'}
            Terminal error.
          {:else if activeStatus === 'closed'}
            Terminal closed.
          {:else if activeStatus === 'starting'}
            Starting terminal…
          {/if}
          {#if activeMessage}
            <span class="status-message">{activeMessage}</span>
          {/if}
        </div>
        <button class="restart" on:click={restartTerminal} type="button">Restart</button>
      </div>
    {/if}
    <div class="tabs">
      <span class="tab active">tab1</span>
      <button class="tab add" type="button" disabled>+</button>
    </div>
    <div class="terminal-surface" bind:this={terminalContainer} />
  </div>
</section>

<style>
  .terminal {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
  }

  .terminal-header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 12px;
  }

  .title {
    font-size: 18px;
    font-weight: 600;
  }

  .meta {
    color: var(--muted);
    font-size: 12px;
  }

  .terminal-body {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 16px;
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-height: 0;
  }

  .terminal-status {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    padding: 10px 12px;
    border-radius: var(--radius-md);
    border: 1px solid var(--warning-soft);
    background: var(--warning-subtle);
    color: var(--text);
    font-size: 12px;
  }

  .status-message {
    margin-left: 8px;
    color: var(--muted);
  }

  .restart {
    background: var(--accent);
    border: none;
    color: #081018;
    padding: 6px 10px;
    border-radius: var(--radius-sm);
    font-weight: 600;
    cursor: pointer;
    transition: background var(--transition-fast), transform var(--transition-fast);
  }

  .restart:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 85%, white);
  }

  .restart:active:not(:disabled) {
    transform: scale(0.98);
  }

  .tabs {
    display: flex;
    gap: 8px;
  }

  .tab {
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid var(--border);
    color: var(--text);
    padding: 4px 10px;
    border-radius: var(--radius-sm);
    font-size: 12px;
    cursor: pointer;
    transition: border-color var(--transition-fast), color var(--transition-fast), background var(--transition-fast);
  }

  .tab:hover:not(.active):not(.add) {
    border-color: var(--muted);
  }

  .tab.add {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .tab.active {
    border-color: var(--accent);
    color: var(--accent);
    background: var(--accent-subtle);
  }

  .terminal-surface {
    flex: 1;
    background: var(--panel-strong);
    border-radius: 10px;
    overflow: hidden;
    border: 1px solid var(--border);
    position: relative;
  }

  :global(.terminal-instance) {
    position: absolute;
    inset: 0;
    opacity: 0;
    pointer-events: none;
  }

  :global(.terminal-instance[data-active='true']) {
    opacity: 1;
    pointer-events: auto;
  }
</style>
