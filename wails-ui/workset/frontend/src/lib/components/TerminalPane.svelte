<script lang="ts">
  import {onDestroy, onMount, untrack} from 'svelte'
  import {Terminal} from '@xterm/xterm'
  import {FitAddon} from '@xterm/addon-fit'
  import {WebglAddon} from '@xterm/addon-webgl'
  import '@xterm/xterm/css/xterm.css'
  import {EventsOn, EventsOff} from '../../../wailsjs/runtime/runtime'
  import {fetchAgentAvailability, fetchSettings, fetchTerminalBacklog} from '../api'
  import {
    ResizeWorkspaceTerminal,
    StartWorkspaceTerminal,
    WriteWorkspaceTerminal
  } from '../../../wailsjs/go/main/App'
  import type {AgentOption} from '../types'
  import Modal from './Modal.svelte'
  import AgentSelector from './AgentSelector.svelte'

  interface Props {
    workspaceId: string;
    workspaceName: string;
    active?: boolean;
  }

  let { workspaceId, workspaceName, active = true }: Props = $props();

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
  const backlogLoaded = new Set<string>()
  const backlogLoading = new Set<string>()
  const pendingBacklogOutput = new Map<string, string[]>()
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
  let rendererPreference = $state<'auto' | 'webgl' | 'canvas'>('auto')
  let statusMap: Record<string, string> = $state({})
  let messageMap: Record<string, string> = $state({})
  let inputMap: Record<string, boolean> = $state({})
  let healthMap: Record<string, 'unknown' | 'checking' | 'ok' | 'stale'> = $state({})
  let healthMessageMap: Record<string, string> = $state({})
  let rendererMap: Record<string, 'unknown' | 'webgl' | 'canvas'> = $state({})
  let rendererModeMap: Record<string, 'auto' | 'webgl' | 'canvas'> = $state({})
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
  let activeRenderer =
    $derived(workspaceId ? rendererMap[workspaceId] ?? 'unknown' : 'unknown')
  let activeRendererMode =
    $derived(workspaceId ? rendererModeMap[workspaceId] ?? 'auto' : 'auto')
  let hasUserInput = $derived(workspaceId ? inputMap[workspaceId] ?? false : false)
  const listeners = new Set<string>()
  const OUTPUT_FLUSH_BUDGET = 128 * 1024
  const OUTPUT_BACKLOG_LIMIT = 512 * 1024
  const RESIZE_DEBOUNCE_MS = 100
  const HEALTH_TIMEOUT_MS = 1200
  const STARTUP_OUTPUT_TIMEOUT_MS = 2000

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
      const renderer = settings.defaults?.terminalRenderer?.trim().toLowerCase()
      if (agent) {
        selectedAgent = agent
      }
      if (renderer === 'auto' || renderer === 'webgl' || renderer === 'canvas') {
        rendererPreference = renderer
      }
    } catch {
      selectedAgent = selectedAgent || 'codex'
      rendererPreference = rendererPreference || 'auto'
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

  const BASE_FONT_SIZE = 12
  const BASE_LINE_HEIGHT = 1.333

  const computeLineHeight = (fontSize: number, desired: number): number => {
    const dpr =
      typeof window !== 'undefined' && Number.isFinite(window.devicePixelRatio)
        ? window.devicePixelRatio
        : 1
    const target = Math.round(fontSize * desired * dpr) / dpr
    return Number((target / fontSize).toFixed(3))
  }

  const loadRendererAddon = async (
    terminal: Terminal,
    id: string,
    mode: 'auto' | 'webgl' | 'canvas'
  ): Promise<void> => {
    rendererModeMap = {...rendererModeMap, [id]: mode}
    if (mode === 'canvas') {
      rendererMap = {...rendererMap, [id]: 'canvas'}
      return
    }
    try {
      if (typeof document !== 'undefined' && document.fonts?.ready) {
        await document.fonts.ready
      }
    } catch {
      // Font readiness is best-effort; continue if unavailable.
    }

    try {
      const webglAddon = new WebglAddon(true)
      webglAddon.onContextLoss(() => {
        webglAddon.dispose()
        rendererMap = {...rendererMap, [id]: 'canvas'}
      })
      terminal.loadAddon(webglAddon)
      rendererMap = {...rendererMap, [id]: 'webgl'}
    } catch {
      rendererMap = {...rendererMap, [id]: 'canvas'}
      // Canvas renderer remains as default.
    }
  }

  const createTerminal = (): Terminal => {
    const themeBackground = getToken('--panel-strong', '#111c29')
    const themeForeground = getToken('--text', '#eef3f9')
    const themeCursor = getToken('--accent', '#2d8cff')
    const themeSelection = getToken('--accent', '#2d8cff')
    const fontMono = getToken('--font-mono', '"JetBrains Mono", Menlo, Consolas, monospace')

    return new Terminal({
      fontFamily: fontMono,
      fontSize: BASE_FONT_SIZE,
      // Keep fontSize * lineHeight * dpr an integer to avoid subpixel row artifacts.
      lineHeight: computeLineHeight(BASE_FONT_SIZE, BASE_LINE_HEIGHT),
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
      terminal.attachCustomKeyEventHandler((event) => {
        if (event.key === 'Enter' && event.shiftKey) {
          inputMap = {...inputMap, [id]: true}
          void beginTerminal(id)
          // Codex CLI expects Ctrl+J (LF) for newline-in-input.
          sendInput(id, '\x0a')
          return false
        }
        return true
      })
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
        void loadRendererAddon(handle.terminal, id, rendererPreference)
      }
      handle.container.setAttribute('data-active', 'true')
      handle.fitAddon.fit()
      const dims = handle.fitAddon.proposeDimensions()
      if (dims) {
        void ResizeWorkspaceTerminal(id, dims.cols, dims.rows).catch(() => undefined)
      }
      if (active) {
        handle.terminal.focus()
      }
      flushOutput(id, false)
    }
    return handle
  }

  const ensureListener = (): void => {
    if (!listeners.has('terminal:data')) {
      const handler = (payload: {workspaceId: string; data: string}): void => {
        const handle = terminals.get(payload.workspaceId)
        if (!handle) return
        if (!backlogLoaded.has(payload.workspaceId)) {
          const pending = pendingBacklogOutput.get(payload.workspaceId) ?? []
          pending.push(payload.data)
          pendingBacklogOutput.set(payload.workspaceId, pending)
          return
        }
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
    void loadBacklog(id)
    if (token !== initCounter) return
    pendingHealthCheck.delete(id)
    if (!startedSessions.has(id) && !startInFlight.has(id)) {
      statusMap = {...statusMap, [id]: 'standby'}
      messageMap = {...messageMap, [id]: ''}
      setHealth(id, 'unknown')
      if (!rendererMap[id]) {
        rendererMap = {...rendererMap, [id]: 'unknown'}
      }
      if (!rendererModeMap[id]) {
        rendererModeMap = {...rendererModeMap, [id]: rendererPreference}
      }
      inputMap = {...inputMap, [id]: false}
    }
  }

  const restartTerminal = async (): Promise<void> => {
    if (!workspaceId) return
    await beginTerminal(workspaceId)
  }

  const loadBacklog = async (id: string): Promise<void> => {
    if (!id || backlogLoaded.has(id) || backlogLoading.has(id)) {
      return
    }
    backlogLoading.add(id)
    try {
      const result = await fetchTerminalBacklog(id, -1)
      if (result?.data) {
        const handle = terminals.get(id)
        if (handle) {
          handle.terminal.write(result.data)
        }
      }
    } catch {
      // Backlog is best-effort; fall through to live stream.
    } finally {
      backlogLoaded.add(id)
      backlogLoading.delete(id)
      const pending = pendingBacklogOutput.get(id)
      if (pending && pending.length > 0) {
        pendingBacklogOutput.delete(id)
        enqueueOutput(id, pending.join(''))
      }
    }
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
        handle.terminal.options.lineHeight = computeLineHeight(BASE_FONT_SIZE, BASE_LINE_HEIGHT)
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
          class="icon-btn"
          type="button"
          title="Show agent launcher"
          onclick={() => {
            if (!workspaceId) return
            inputMap = {...inputMap, [workspaceId]: false}
          }}
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <rect x="4" y="4" width="6" height="6" />
            <rect x="14" y="4" width="6" height="6" />
            <rect x="4" y="14" width="6" height="6" />
            <rect x="14" y="14" width="6" height="6" />
          </svg>
        </button>
      {/if}
      <div
        class="renderer-status"
        class:fallback={activeRenderer === 'canvas' && activeRendererMode !== 'canvas'}
        class:forced={activeRenderer === 'canvas' && activeRendererMode === 'canvas'}
        title="Terminal renderer"
      >
        {#if activeRenderer === 'webgl'}
          Renderer: WebGL
        {:else if activeRenderer === 'canvas'}
          {#if activeRendererMode === 'canvas'}
            Renderer: Canvas (forced)
          {:else}
            Renderer: Canvas (fallback)
          {/if}
        {:else}
          Renderer: Unknown
        {/if}
      </div>
      <div class="health-status" class:ok={activeHealth === 'ok'} class:stale={activeHealth === 'stale'} class:checking={activeHealth === 'checking'}>
        <span class="health-dot"></span>
        <span class="health-label">
          {activeHealth === 'ok' ? 'Healthy' : activeHealth === 'checking' ? 'Checking' : activeHealth === 'stale' ? 'Stale' : 'Unknown'}
        </span>
      </div>
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
        <div class="agent-launcher-overlay">
          <Modal title="Launch Agent" subtitle="Runs inside this terminal" size="sm">
            <AgentSelector
              agents={agentOptions}
              selected={selectedAgent}
              availability={agentAvailability}
              {availabilityStatus}
              onSelect={(id) => (selectedAgent = id)}
            />
            {#if availabilityStatus === 'ready' && !selectedAgentAvailable}
              <div class="agent-warning">Install {selectedAgent} to launch this agent.</div>
            {:else if availabilityStatus === 'error'}
              <div class="agent-warning">Unable to check agent availability.</div>
            {/if}
            {#snippet footer()}
              <button
                class="primary full-width"
                type="button"
                onclick={startAgent}
                disabled={!selectedAgentAvailable}
              >
                Start {agentOptions.find((a) => a.id === selectedAgent)?.label ?? 'Agent'}
              </button>
              <button
                class="ghost-link"
                type="button"
                onclick={() => {
                  if (!workspaceId) return
                  inputMap = {...inputMap, [workspaceId]: true}
                  void beginTerminal(workspaceId)
                }}
              >
                Use terminal without agent
              </button>
              <div class="agent-hint">Change default in Settings → Session</div>
            {/snippet}
          </Modal>
        </div>
      {/if}
    </div>
  </div>
</section>

<style>
  .terminal {
    display: flex;
    flex-direction: column;
    gap: 8px;
    height: 100%;
  }

  .terminal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    min-height: 32px;
    gap: 8px;
  }

  .title {
    font-size: 14px;
    font-weight: 500;
    color: var(--muted);
  }

  .meta {
    display: none;
  }

  .terminal-body {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 8px;
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 8px;
    min-height: 0;
  }

  .terminal-actions {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .renderer-status {
    font-size: 11px;
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 2px 8px;
    background: rgba(255, 255, 255, 0.02);
    letter-spacing: 0.02em;
  }

  .renderer-status.fallback {
    color: var(--warning);
    border-color: color-mix(in srgb, var(--warning) 50%, var(--border));
    background: color-mix(in srgb, var(--warning) 12%, transparent);
  }

  .renderer-status.forced {
    color: var(--muted);
    border-color: var(--border);
    background: rgba(255, 255, 255, 0.02);
  }

  .icon-btn {
    width: 28px;
    height: 28px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: rgba(255, 255, 255, 0.02);
    color: var(--muted);
    cursor: pointer;
    display: grid;
    place-items: center;
    transition: border-color var(--transition-fast), color var(--transition-fast);
  }

  .icon-btn:hover {
    border-color: var(--accent);
    color: var(--text);
  }

  .icon-btn svg {
    width: 14px;
    height: 14px;
    stroke: currentColor;
    stroke-width: 2;
    fill: none;
  }

  .health-status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--muted);
  }

  .health-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--muted);
  }

  .health-status.ok .health-dot {
    background: var(--success);
  }

  .health-status.stale .health-dot {
    background: var(--warning);
  }

  .health-status.checking .health-dot {
    background: var(--accent);
    animation: pulse 1.5s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
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
    position: relative;
  }

  .terminal-mount {
    position: absolute;
    inset: 8px;
    z-index: 1;
  }

  .agent-launcher-overlay {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    z-index: 2;
    pointer-events: auto;
    background: radial-gradient(
      circle at center,
      rgba(9, 15, 26, 0.65),
      rgba(9, 15, 26, 0.25) 55%,
      transparent 70%
    );
  }

  .agent-warning {
    font-size: 11px;
    color: var(--warning);
    text-align: center;
  }

  .full-width {
    width: 100%;
  }

  .primary {
    background: var(--accent);
    color: #081018;
    border: none;
    padding: 10px 16px;
    border-radius: var(--radius-md);
    font-weight: 600;
    font-size: 14px;
    cursor: pointer;
    transition:
      background var(--transition-fast),
      transform var(--transition-fast);
  }

  .primary:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 85%, white);
  }

  .primary:active:not(:disabled) {
    transform: scale(0.98);
  }

  .primary:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .ghost-link {
    background: transparent;
    border: none;
    color: var(--muted);
    font-size: 12px;
    cursor: pointer;
    padding: 4px 8px;
    transition: color var(--transition-fast);
  }

  .ghost-link:hover {
    color: var(--text);
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
