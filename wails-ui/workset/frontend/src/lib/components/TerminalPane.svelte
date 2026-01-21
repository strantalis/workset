<script lang="ts">
  import {onDestroy, onMount, untrack} from 'svelte'
  import {Terminal} from '@xterm/xterm'
  import {FitAddon} from '@xterm/addon-fit'
  import '@xterm/xterm/css/xterm.css'
  import {EventsOn, EventsOff} from '../../../wailsjs/runtime/runtime'
  import {fetchAgentAvailability, fetchSettings} from '../api'
  import {
    ResizeWorkspaceTerminal,
    StartWorkspaceTerminal,
    WriteWorkspaceTerminal
  } from '../../../wailsjs/go/main/App'

  interface Props {
    workspaceId: string;
    workspaceName: string;
  }

  let { workspaceId, workspaceName }: Props = $props();

  let terminalContainer: HTMLDivElement | null = $state(null)
  let resizeObserver: ResizeObserver | null = null

  type TerminalHandle = {
    terminal: Terminal
    fitAddon: FitAddon
    dataDisposable: {dispose: () => void}
    container: HTMLDivElement
  }

  const terminals = new Map<string, TerminalHandle>()
  const outputQueues = new Map<string, {chunks: string[]; bytes: number; scheduled: boolean}>()
  const lastDims = new Map<string, {cols: number; rows: number}>()
  const startupTimers = new Map<string, number>()
  const startInFlight = new Set<string>()
  const statsMap = new Map<string, {
    bytesIn: number
    bytesOut: number
    backlog: number
    lastOutputAt: number
    lastCprAt: number
  }>()
  const pendingInput = new Map<string, string>()
  const pendingHealthCheck = new Map<string, number>()
  let initCounter = 0
  const startedSessions = new Set<string>()
  let resizeScheduled = false
  let resizeTimer: number | null = null
  let debugInterval: number | null = null
  let statusMap: Record<string, string> = $state({})
  let messageMap: Record<string, string> = $state({})
  let inputMap: Record<string, boolean> = $state({})
  let healthMap: Record<string, 'unknown' | 'checking' | 'ok' | 'stale'> = $state({})
  let healthMessageMap: Record<string, string> = $state({})
  let debugEnabled = $state(false)
  let debugStats = $state({
    bytesIn: 0,
    bytesOut: 0,
    backlog: 0,
    lastOutputAt: 0,
    lastCprAt: 0
  })

  let activeStatus = $derived(workspaceId ? statusMap[workspaceId] ?? '' : '')
  let activeMessage = $derived(workspaceId ? messageMap[workspaceId] ?? '' : '')
  let activeHealth = $derived(workspaceId ? healthMap[workspaceId] ?? 'unknown' : 'unknown')
  let activeHealthMessage =
    $derived(workspaceId ? healthMessageMap[workspaceId] ?? '' : '')
  let hasUserInput = $derived(workspaceId ? inputMap[workspaceId] ?? false : false)
  const listeners = new Set<string>()
  const OUTPUT_FLUSH_BUDGET = 128 * 1024
  const OUTPUT_BACKLOG_LIMIT = 512 * 1024
  const RESIZE_DEBOUNCE_MS = 100
  const HEALTH_TIMEOUT_MS = 1200
  const STARTUP_OUTPUT_TIMEOUT_MS = 2000

  type AgentOption = {
    id: string
    label: string
    command: string
  }

  const agentOptions: AgentOption[] = [
    {id: 'codex', label: 'Codex', command: 'codex'},
    {id: 'claude', label: 'Claude Code', command: 'claude'},
    {id: 'opencode', label: 'OpenCode', command: 'opencode'},
    {id: 'pi', label: 'Pi', command: 'pi'},
    {id: 'cursor', label: 'Cursor Agent', command: 'cursor agent'}
  ]

  let selectedAgent = $state('codex')
  let agentAvailability: Record<string, boolean> = $state({})
  let availabilityStatus = $state<'idle' | 'loading' | 'ready' | 'error'>('idle')
  let availabilityMessage = $state('')

  let selectedAgentAvailable = $derived(
    availabilityStatus !== 'ready' ? true : agentAvailability[selectedAgent] ?? false
  )

  const getToken = (name: string, fallback: string): string => {
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
    return value || fallback
  }

  const loadAgentDefault = async (): Promise<void> => {
    try {
      const settings = await fetchSettings()
      const agent = settings.defaults?.agent?.trim()
      if (agent) {
        selectedAgent = agent
      }
    } catch {
      selectedAgent = selectedAgent || 'codex'
    }
  }

  const loadAgentAvailability = async (): Promise<void> => {
    availabilityStatus = 'loading'
    availabilityMessage = ''
    try {
      const availability = await fetchAgentAvailability()
      agentAvailability = availability ?? {}
      availabilityStatus = 'ready'
    } catch (error) {
      availabilityStatus = 'error'
      availabilityMessage = String(error)
    }
  }

  const resolveAgentCommand = (): string => {
    const option = agentOptions.find((entry) => entry.id === selectedAgent)
    return option?.command ?? selectedAgent
  }

  const startAgent = (): void => {
    if (!workspaceId) return
    if (!selectedAgentAvailable) return
    const command = resolveAgentCommand()
    if (!command) return
    inputMap = {...inputMap, [workspaceId]: true}
    void beginTerminal(workspaceId)
    sendInput(workspaceId, `${command}\n`)
  }

  const beginTerminal = async (id: string): Promise<void> => {
    if (!id || startedSessions.has(id) || startInFlight.has(id)) return
    startInFlight.add(id)
    statusMap = {...statusMap, [id]: 'starting'}
    messageMap = {...messageMap, [id]: 'Waiting for shell output…'}
    setHealth(id, 'unknown')
    inputMap = {...inputMap, [id]: false}
    scheduleStartupTimeout(id)
    try {
      await StartWorkspaceTerminal(id)
      startedSessions.add(id)
      requestHealthCheck(id)
      const queued = pendingInput.get(id)
      if (queued) {
        pendingInput.delete(id)
        await WriteWorkspaceTerminal(id, queued)
      }
    } catch (error) {
      statusMap = {...statusMap, [id]: 'error'}
      messageMap = {...messageMap, [id]: String(error)}
      setHealth(id, 'stale', 'Failed to start terminal.')
      clearStartupTimeout(id)
      pendingInput.delete(id)
      const handle = terminals.get(id)
      handle?.terminal.write(`\r\n[workset] failed to start terminal: ${String(error)}`)
    } finally {
      startInFlight.delete(id)
    }
  }

  const updateStats = (id: string, updater: (stats: {
    bytesIn: number
    bytesOut: number
    backlog: number
    lastOutputAt: number
    lastCprAt: number
  }) => void): void => {
    const existing =
      statsMap.get(id) ?? {bytesIn: 0, bytesOut: 0, backlog: 0, lastOutputAt: 0, lastCprAt: 0}
    updater(existing)
    statsMap.set(id, existing)
  }

  const setHealth = (id: string, state: 'unknown' | 'checking' | 'ok' | 'stale', message = ''): void => {
    healthMap = {...healthMap, [id]: state}
    healthMessageMap = {...healthMessageMap, [id]: message}
  }

  const clearStartupTimeout = (id: string): void => {
    const timer = startupTimers.get(id)
    if (timer) {
      window.clearTimeout(timer)
      startupTimers.delete(id)
    }
  }

  const scheduleStartupTimeout = (id: string): void => {
    clearStartupTimeout(id)
    const timer = window.setTimeout(() => {
      if (statusMap[id] === 'starting') {
        messageMap = {
          ...messageMap,
          [id]: 'Shell has not produced output yet. Check your shell init scripts.'
        }
      }
    }, STARTUP_OUTPUT_TIMEOUT_MS)
    startupTimers.set(id, timer)
  }

  const sendInput = (id: string, data: string): void => {
    if (!startedSessions.has(id)) {
      pendingInput.set(id, (pendingInput.get(id) ?? '') + data)
      return
    }
    updateStats(id, (stats) => {
      stats.bytesOut += data.length
    })
    void WriteWorkspaceTerminal(id, data).catch((error) => {
      pendingInput.set(id, (pendingInput.get(id) ?? '') + data)
      startedSessions.delete(id)
      const handle = terminals.get(id)
      handle?.terminal.write(`\r\n[workset] write failed: ${String(error)}`)
    })
  }

  const noteCpr = (id: string): void => {
    updateStats(id, (stats) => {
      stats.lastCprAt = Date.now()
    })
    if (healthMap[id] === 'checking' || healthMap[id] === 'unknown') {
      setHealth(id, 'ok', 'CPR received.')
    }
  }

  const captureCpr = (id: string, data: string): boolean => {
    const matches = data.match(/\x1b\[\??\d+;\d+R/g)
    if (!matches) return false
    noteCpr(id)
    return true
  }

  const enqueueOutput = (id: string, data: string): void => {
    updateStats(id, (stats) => {
      stats.bytesIn += data.length
      stats.lastOutputAt = Date.now()
    })
    const queue = outputQueues.get(id) ?? {chunks: [], bytes: 0, scheduled: false}
    queue.chunks.push(data)
    queue.bytes += data.length
    outputQueues.set(id, queue)
    updateStats(id, (stats) => {
      stats.backlog = queue.bytes
    })
    if (statusMap[id] === 'starting') {
      statusMap = {...statusMap, [id]: 'ready'}
      messageMap = {...messageMap, [id]: ''}
      clearStartupTimeout(id)
    }

    const isActive = id === workspaceId
    if (queue.bytes >= OUTPUT_BACKLOG_LIMIT) {
      flushOutput(id, true)
      return
    }
    if (isActive && queue.bytes >= OUTPUT_FLUSH_BUDGET) {
      flushOutput(id, false)
      return
    }
    if (!queue.scheduled) {
      queue.scheduled = true
      requestAnimationFrame(() => {
        queue.scheduled = false
        flushOutput(id, false)
      })
    }
  }

  const flushOutput = (id: string, force: boolean): void => {
    const queue = outputQueues.get(id)
    if (!queue || queue.bytes === 0) return
    const handle = terminals.get(id)
    if (!handle) return
    const isActive = id === workspaceId
    if (!isActive && !force) return

    let size = 0
    let count = 0
    while (count < queue.chunks.length && size + queue.chunks[count].length <= OUTPUT_FLUSH_BUDGET) {
      size += queue.chunks[count].length
      count += 1
    }
    if (count === 0 && queue.chunks.length > 0) {
      size = queue.chunks[0].length
      count = 1
    }
    const output = queue.chunks.slice(0, count).join('')
    queue.chunks = queue.chunks.slice(count)
    queue.bytes = Math.max(0, queue.bytes - size)
    updateStats(id, (stats) => {
      stats.backlog = queue.bytes
    })
    if (output) {
      handle.terminal.write(output)
    }
    if (queue.chunks.length === 0) {
      outputQueues.delete(id)
    } else if (isActive) {
      requestAnimationFrame(() => flushOutput(id, false))
    }
  }

  const requestHealthCheck = (id: string): void => {
    const handle = terminals.get(id)
    if (!handle) return
    setHealth(id, 'checking', 'Waiting for CPR…')
    const startedAt = Date.now()
    pendingHealthCheck.set(id, startedAt)
    handle.terminal.write('\x1b[6n')
    window.setTimeout(() => {
      if (healthMap[id] === 'checking') {
        setHealth(id, 'stale', 'No CPR response.')
      }
      pendingHealthCheck.delete(id)
    }, HEALTH_TIMEOUT_MS)
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
        inputMap = {...inputMap, [id]: true}
        void beginTerminal(id)
        const sawCpr = captureCpr(id, data)
        if (sawCpr) {
          const pendingAt = pendingHealthCheck.get(id)
          if (pendingAt && Date.now() - pendingAt <= HEALTH_TIMEOUT_MS * 2) {
            pendingHealthCheck.delete(id)
            const stripped = data.replace(/\x1b\[\??\d+;\d+R/g, '')
            if (stripped) {
              sendInput(id, stripped)
            }
            return
          }
        }
        sendInput(id, data)
      })
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
      flushOutput(id, false)
    }
    return handle
  }

  const ensureListener = (): void => {
    if (!listeners.has('terminal:data')) {
      const handler = (payload: {workspaceId: string; data: string}): void => {
        const handle = terminals.get(payload.workspaceId)
        if (!handle) return
        enqueueOutput(payload.workspaceId, payload.data)
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
          statusMap = {...statusMap, [payload.workspaceId]: 'starting'}
          messageMap = {...messageMap, [payload.workspaceId]: 'Waiting for shell output…'}
          inputMap = {...inputMap, [payload.workspaceId]: false}
          scheduleStartupTimeout(payload.workspaceId)
          setHealth(payload.workspaceId, 'unknown')
          return
        }
        if (payload.status === 'closed') {
          startedSessions.delete(payload.workspaceId)
          statusMap = {...statusMap, [payload.workspaceId]: 'closed'}
          setHealth(payload.workspaceId, 'stale', 'Terminal closed.')
          clearStartupTimeout(payload.workspaceId)
          return
        }
        if (payload.status === 'idle') {
          startedSessions.delete(payload.workspaceId)
          statusMap = {...statusMap, [payload.workspaceId]: 'idle'}
          setHealth(payload.workspaceId, 'stale', 'Terminal idle.')
          clearStartupTimeout(payload.workspaceId)
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
          }
          setHealth(payload.workspaceId, 'stale', payload.message ?? 'Terminal error.')
          clearStartupTimeout(payload.workspaceId)
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

  const initTerminal = async (id: string, name: string): Promise<void> => {
    if (!id) return
    const token = ++initCounter
    ensureListener()
    attachTerminal(id, name)
    if (token !== initCounter) return
    pendingHealthCheck.delete(id)
    if (!startedSessions.has(id) && !startInFlight.has(id)) {
      statusMap = {...statusMap, [id]: 'standby'}
      messageMap = {...messageMap, [id]: ''}
      setHealth(id, 'unknown')
      inputMap = {...inputMap, [id]: false}
    }
  }

  const restartTerminal = async (): Promise<void> => {
    if (!workspaceId) return
    await beginTerminal(workspaceId)
  }

  onMount(() => {
    if (!terminalContainer) return
    debugEnabled =
      typeof localStorage !== 'undefined' && localStorage.getItem('worksetTerminalDebug') === '1'
    void loadAgentDefault()
    void loadAgentAvailability()
    resizeObserver = new ResizeObserver(() => {
      if (!workspaceId || resizeScheduled) return
      resizeScheduled = true
      if (resizeTimer) {
        window.clearTimeout(resizeTimer)
      }
      resizeTimer = window.setTimeout(() => {
        resizeScheduled = false
        const handle = terminals.get(workspaceId)
        if (!handle) return
        handle.fitAddon.fit()
        if (!startedSessions.has(workspaceId)) return
        const dims = handle.fitAddon.proposeDimensions()
        if (dims) {
          const prev = lastDims.get(workspaceId)
          if (!prev || prev.cols !== dims.cols || prev.rows !== dims.rows) {
            lastDims.set(workspaceId, {cols: dims.cols, rows: dims.rows})
            void ResizeWorkspaceTerminal(workspaceId, dims.cols, dims.rows).catch(() => undefined)
          }
        }
      }, RESIZE_DEBOUNCE_MS)
    })
    resizeObserver.observe(terminalContainer)

    if (debugEnabled) {
      debugInterval = window.setInterval(() => {
        if (!workspaceId) return
        const stats =
          statsMap.get(workspaceId) ?? {bytesIn: 0, bytesOut: 0, backlog: 0, lastOutputAt: 0, lastCprAt: 0}
        debugStats = {...stats}
      }, 1000)
    }
  })

  $effect(() => {
    if (!workspaceId || !terminalContainer) return
    const id = workspaceId
    const name = workspaceName
    untrack(() => {
      void initTerminal(id, name)
    })
  })

  onDestroy(() => {
    resizeObserver?.disconnect()
    if (resizeTimer) {
      window.clearTimeout(resizeTimer)
    }
    for (const timer of startupTimers.values()) {
      window.clearTimeout(timer)
    }
    startupTimers.clear()
    if (debugInterval) {
      window.clearInterval(debugInterval)
    }
    cleanupListeners()
  })
</script>

<section class="terminal">
  <header class="terminal-header">
    <div>
      <div class="title">Workspace terminal</div>
      <div class="meta">Workspace: {workspaceName}</div>
    </div>
    <div class="terminal-actions">
      {#if hasUserInput}
        <button
          class="ghost agent-reset"
          type="button"
          onclick={() => {
            if (!workspaceId) return
            inputMap = {...inputMap, [workspaceId]: false}
          }}
        >
          Show launcher
        </button>
      {/if}
      <span
        class="health-pill"
        class:ok={activeHealth === 'ok'}
        class:stale={activeHealth === 'stale'}
        class:checking={activeHealth === 'checking'}
      >
        {activeHealth === 'ok'
          ? 'Healthy'
          : activeHealth === 'checking'
            ? 'Checking…'
            : activeHealth === 'stale'
              ? 'Needs sync'
              : 'Unknown'}
      </span>
    </div>
  </header>
  <div class="terminal-body">
    {#if activeStatus && activeStatus !== 'ready' && activeStatus !== 'standby'}
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
          {:else if activeStatus === 'standby'}
            Terminal is ready to start.
          {/if}
          {#if activeMessage}
            <span class="status-message">{activeMessage}</span>
          {/if}
        </div>
        <button class="restart" onclick={restartTerminal} type="button">Restart</button>
      </div>
    {/if}
    {#if activeHealthMessage && activeHealth !== 'ok'}
      <div class="terminal-status subtle">
        <div class="status-text">
          {activeHealthMessage}
        </div>
        <button
          class="restart"
          type="button"
          onclick={() => workspaceId && requestHealthCheck(workspaceId)}
        >
          Retry check
        </button>
      </div>
    {/if}
    {#if debugEnabled}
      <div class="terminal-debug">
        <div>bytes in: {debugStats.bytesIn}</div>
        <div>bytes out: {debugStats.bytesOut}</div>
        <div>backlog: {debugStats.backlog}</div>
        <div>last output: {debugStats.lastOutputAt ? new Date(debugStats.lastOutputAt).toLocaleTimeString() : '—'}</div>
        <div>last cpr: {debugStats.lastCprAt ? new Date(debugStats.lastCprAt).toLocaleTimeString() : '—'}</div>
      </div>
    {/if}
    <div class="terminal-surface">
      <div class="terminal-mount" bind:this={terminalContainer}></div>
      {#if activeStatus !== 'starting' && !hasUserInput}
        <div class="agent-launcher">
          <div class="agent-card">
            <div class="agent-title">Start agent</div>
            <div class="agent-subtitle">Runs inside this terminal.</div>
            <div class="agent-controls">
              <label class="agent-select">
                <span>Default</span>
                <select bind:value={selectedAgent}>
                  {#each agentOptions as option}
                    <option value={option.id}>
                      {option.label}
                      {#if availabilityStatus === 'ready' && agentAvailability[option.id] === false}
                        (missing)
                      {/if}
                    </option>
                  {/each}
                </select>
              </label>
              <button
                class="primary agent-start"
                type="button"
                onclick={startAgent}
                disabled={!selectedAgentAvailable}
              >
                Start
              </button>
            </div>
            {#if availabilityStatus === 'ready' && !selectedAgentAvailable}
              <div class="agent-missing">Install {selectedAgent} to launch this agent.</div>
            {:else if availabilityStatus === 'error'}
              <div class="agent-missing">Unable to check agent availability.</div>
            {/if}
            <button
              class="ghost agent-skip"
              type="button"
              onclick={() => {
                if (!workspaceId) return
                inputMap = {...inputMap, [workspaceId]: true}
                void beginTerminal(workspaceId)
              }}
            >
              Use terminal without agent
            </button>
            <div class="agent-hint">Change default in Settings → Session.</div>
          </div>
        </div>
      {/if}
    </div>
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
    align-items: center;
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

  .terminal-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .agent-reset {
    font-size: 12px;
  }

  .health-pill {
    font-size: 11px;
    padding: 4px 8px;
    border-radius: 999px;
    border: 1px solid var(--border);
    background: rgba(255, 255, 255, 0.04);
    color: var(--muted);
  }

  .health-pill.ok {
    border-color: var(--success-soft);
    color: var(--success);
    background: var(--success-subtle);
  }

  .health-pill.stale {
    border-color: var(--warning-soft);
    color: var(--warning);
    background: var(--warning-subtle);
  }

  .health-pill.checking {
    border-color: var(--border);
    color: var(--text);
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

  .terminal-status.subtle {
    background: var(--panel-soft);
    border-color: var(--border);
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

  .terminal-debug {
    font-size: 11px;
    color: var(--muted);
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 6px 12px;
    border: 1px dashed var(--border);
    border-radius: var(--radius-sm);
    padding: 8px;
    background: rgba(255, 255, 255, 0.02);
  }

  .terminal-surface {
    flex: 1;
    background: var(--panel-strong);
    border-radius: 10px;
    overflow: hidden;
    border: 1px solid var(--border);
    position: relative;
  }

  .terminal-mount {
    position: absolute;
    inset: 0;
    z-index: 1;
  }

  .agent-launcher {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    z-index: 2;
    pointer-events: auto;
    background: radial-gradient(circle at center, rgba(9, 15, 26, 0.55), rgba(9, 15, 26, 0.15) 55%, transparent 70%);
  }

  .agent-card {
    width: min(360px, 70%);
    padding: 20px 22px;
    border-radius: 16px;
    border: 1px solid rgba(255, 255, 255, 0.12);
    background: rgba(11, 18, 30, 0.85);
    box-shadow: 0 16px 50px rgba(0, 0, 0, 0.45);
    text-align: center;
    display: grid;
    gap: 12px;
  }

  .agent-title {
    font-size: 18px;
    font-weight: 600;
  }

  .agent-subtitle {
    font-size: 12px;
    color: var(--muted);
  }

  .agent-controls {
    display: grid;
    gap: 10px;
  }

  .agent-select {
    display: grid;
    gap: 6px;
    text-align: left;
    font-size: 11px;
    color: var(--muted);
  }

  .agent-select span {
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .agent-select select {
    width: 100%;
    padding: 8px 10px;
    border-radius: var(--radius-sm);
    border: 1px solid rgba(255, 255, 255, 0.08);
    background: rgba(255, 255, 255, 0.03);
    color: var(--text);
    font-size: 13px;
  }

  .agent-start {
    width: 100%;
    justify-self: center;
  }

  .agent-missing {
    font-size: 11px;
    color: var(--warning);
  }

  .agent-skip {
    justify-self: center;
    font-size: 12px;
  }

  .agent-hint {
    font-size: 11px;
    color: var(--muted);
  }

  :global(.terminal-instance) {
    position: absolute;
    inset: 0;
    opacity: 0;
    pointer-events: none;
    z-index: 1;
  }

  :global(.terminal-instance[data-active='true']) {
    opacity: 1;
    pointer-events: auto;
  }
</style>
