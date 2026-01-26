<script lang="ts">
  import {onDestroy, onMount, untrack} from 'svelte'
  import {Terminal} from '@xterm/xterm'
  import {FitAddon} from '@xterm/addon-fit'
  import {WebglAddon} from '@xterm/addon-webgl'
  import '@xterm/xterm/css/xterm.css'
  import {EventsOn, EventsOff} from '../../../wailsjs/runtime/runtime'
  import {
    fetchAgentAvailability,
    fetchSettings,
    fetchSessiondStatus,
    fetchWorkspaceTerminalStatus,
    fetchTerminalBootstrap,
    logTerminalDebug
  } from '../api'
  import {
    ResizeWorkspaceTerminal,
    StartWorkspaceTerminal,
    WriteWorkspaceTerminal
  } from '../../../wailsjs/go/main/App'
  import {stripMouseReports} from '../terminal/inputFilter'
  import {encodeWheel, type MouseEncoding} from '../terminal/mouse'
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
    binaryDisposable?: {dispose: () => void}
    container: HTMLDivElement
    kittyState: KittyState
    kittyOverlay?: KittyOverlay
    kittyDisposables?: {dispose: () => void}[]
    webglAddon?: WebglAddon
  }

  type KittyImage = {
    id: string
    format: string
    width: number
    height: number
    data: Uint8Array
    bitmap?: ImageBitmap
    decoding?: Promise<void>
  }

  type KittyPlacement = {
    id: number
    imageId: string
    row: number
    col: number
    rows: number
    cols: number
    x: number
    y: number
    z: number
  }

  type KittyState = {
    images: Map<string, KittyImage>
    placements: Map<string, KittyPlacement>
  }

  type KittyOverlay = {
    underlay: HTMLCanvasElement
    overlay: HTMLCanvasElement
    ctxUnder: CanvasRenderingContext2D
    ctxOver: CanvasRenderingContext2D
    cellWidth: number
    cellHeight: number
    dpr: number
    renderScheduled: boolean
  }

  type KittyEventPayload = {
    kind: string
    image?: {
      id: string
      format?: string
      width?: number
      height?: number
    data?: string | number[] | Uint8Array
    }
    placement?: {
      id: number
      imageId: string
      row: number
      col: number
      rows: number
      cols: number
      x?: number
      y?: number
      z?: number
    }
    delete?: {
      all?: boolean
      imageId?: string
      placementId?: number
    }
    snapshot?: {
      images?: Array<{
        id: string
        format?: string
        width?: number
        height?: number
        data?: string | number[] | Uint8Array
      }>
      placements?: Array<{
        id: number
        imageId: string
        row: number
        col: number
        rows: number
        cols: number
        x?: number
        y?: number
        z?: number
      }>
    }
  }

  const terminals = new Map<string, TerminalHandle>()
  const outputQueues = new Map<string, {chunks: string[]; bytes: number; scheduled: boolean}>()
  const replayState = new Map<string, 'idle' | 'replaying' | 'live'>()
  const replayLoading = new Set<string>()
  const pendingReplayOutput = new Map<string, string[]>()
  const pendingReplayKitty = new Map<string, KittyEventPayload[]>()
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
  const renderStatsMap = new Map<string, {lastRenderAt: number; renderCount: number}>()
  const pendingInput = new Map<string, string>()
  const pendingHealthCheck = new Map<string, number>()
  const pendingRenderCheck = new Map<string, number>()
  const pendingRedraw = new Set<string>()
  let initCounter = 0
  const startedSessions = new Set<string>()
  let resizeScheduled = false
  let resizeTimer: number | null = null
  let debugInterval: number | null = null
  let rendererPreference = $state<'auto' | 'webgl' | 'canvas'>('auto')
  let sessiondAvailable = $state<boolean | null>(null)
  let sessiondChecked = $state(false)
  let statusMap: Record<string, string> = $state({})
  let messageMap: Record<string, string> = $state({})
  let inputMap: Record<string, boolean> = $state({})
  let healthMap: Record<string, 'unknown' | 'checking' | 'ok' | 'stale'> = $state({})
  let healthMessageMap: Record<string, string> = $state({})
  let suppressMouseUntil: Record<string, number> = $state({})
  let mouseInputTail: Record<string, string> = $state({})
  let rendererMap: Record<string, 'unknown' | 'webgl' | 'canvas'> = $state({})
  let rendererModeMap: Record<string, 'auto' | 'webgl' | 'canvas'> = $state({})
  let modeMap: Record<
    string,
    {altScreen: boolean; mouse: boolean; mouseSGR: boolean; mouseEncoding: string}
  > = $state({})
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

  const clamp = (value: number, min: number, max: number): number => {
    if (value < min) return min
    if (value > max) return max
    return value
  }

  const resolveCellSize = (handle: TerminalHandle, terminal: Terminal): {width: number; height: number} => {
    if (handle.kittyOverlay?.cellWidth && handle.kittyOverlay?.cellHeight) {
      return {width: handle.kittyOverlay.cellWidth, height: handle.kittyOverlay.cellHeight}
    }
    if (!handle.container) {
      return {width: 0, height: 0}
    }
    const rect = handle.container.getBoundingClientRect()
    const cols = Math.max(terminal.cols, 1)
    const rows = Math.max(terminal.rows, 1)
    return {width: rect.width / cols, height: rect.height / rows}
  }

  const handleWheel = (id: string, terminal: Terminal, event: WheelEvent): boolean => {
    const modes = modeMap[id]
    if (statusMap[id] !== 'ready' || startInFlight.has(id) || !startedSessions.has(id)) {
      return true
    }
    if (!modes?.mouse) {
      return true
    }
    const handle = terminals.get(id)
    if (!handle?.container) {
      return true
    }
    const rect = handle.container.getBoundingClientRect()
    const {width, height} = resolveCellSize(handle, terminal)
    if (width <= 0 || height <= 0) {
      return true
    }
    const col = clamp(Math.floor((event.clientX - rect.left) / width) + 1, 1, terminal.cols)
    const row = clamp(Math.floor((event.clientY - rect.top) / height) + 1, 1, terminal.rows)
    const button = event.deltaY < 0 ? 64 : 65
    const encoding = (modes.mouseEncoding || (modes.mouseSGR ? 'sgr' : 'x10')) as MouseEncoding
    sendInput(id, encodeWheel({button, col, row, encoding}))
    event.preventDefault()
    return false
  }
  const RESIZE_DEBOUNCE_MS = 100
  const HEALTH_TIMEOUT_MS = 1200
  const STARTUP_OUTPUT_TIMEOUT_MS = 2000
  const RENDER_CHECK_DELAY_MS = 350
  const RENDER_RECOVERY_DELAY_MS = 150

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

  const beginTerminal = async (id: string, quiet = false): Promise<void> => {
    if (!id || startedSessions.has(id) || startInFlight.has(id)) return
    startInFlight.add(id)
    resetSessionState(id)
    if (!quiet) {
      statusMap = {...statusMap, [id]: 'starting'}
      messageMap = {...messageMap, [id]: 'Waiting for shell output…'}
      setHealth(id, 'unknown')
      inputMap = {...inputMap, [id]: false}
      scheduleStartupTimeout(id)
    }
    try {
      await StartWorkspaceTerminal(id)
      startedSessions.add(id)
      const queued = pendingInput.get(id)
      if (queued) {
        pendingInput.delete(id)
        await WriteWorkspaceTerminal(id, queued)
      }
      await loadBootstrap(id)
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

  const shouldSuppressMouseInput = (id: string, data: string): boolean => {
    const until = suppressMouseUntil[id]
    if (!until || Date.now() >= until) {
      return false
    }
    return data.includes('\x1b[<')
  }

  const noteMouseSuppress = (id: string, durationMs: number): void => {
    suppressMouseUntil = {...suppressMouseUntil, [id]: Date.now() + durationMs}
  }

  const sendInput = (id: string, data: string): void => {
    if (shouldSuppressMouseInput(id, data)) {
      return
    }
    const modes = modeMap[id] ?? {altScreen: false, mouse: false, mouseSGR: false, mouseEncoding: 'x10'}
    const mouseResult = stripMouseReports(data, modes, mouseInputTail[id] ?? '')
    if (mouseResult.tail !== (mouseInputTail[id] ?? '')) {
      mouseInputTail = {...mouseInputTail, [id]: mouseResult.tail}
    }
    const filtered = mouseResult.filtered
    if (!filtered) {
      return
    }
    if (!startedSessions.has(id)) {
      pendingInput.set(id, (pendingInput.get(id) ?? '') + filtered)
      return
    }
    updateStats(id, (stats) => {
      stats.bytesOut += filtered.length
    })
    void WriteWorkspaceTerminal(id, filtered).catch((error) => {
      pendingInput.set(id, (pendingInput.get(id) ?? '') + filtered)
      startedSessions.delete(id)
      if (
        typeof error === 'string' &&
        (error.includes('session not found') || error.includes('terminal not started'))
      ) {
        resetTerminalInstance(id)
        void beginTerminal(id, true)
      }
      if (error instanceof Error) {
        const message = error.message
        if (message.includes('session not found') || message.includes('terminal not started')) {
          resetTerminalInstance(id)
          void beginTerminal(id, true)
        }
      }
      const handle = terminals.get(id)
      handle?.terminal.write(`\r\n[workset] write failed: ${String(error)}`)
    })
  }

  const resetSessionState = (id: string): void => {
    replayState.set(id, 'idle')
    replayLoading.delete(id)
    pendingReplayOutput.delete(id)
    pendingReplayKitty.delete(id)
    outputQueues.delete(id)
    pendingHealthCheck.delete(id)
    const renderTimer = pendingRenderCheck.get(id)
    if (renderTimer) {
      window.clearTimeout(renderTimer)
    }
    pendingRenderCheck.delete(id)
    renderStatsMap.delete(id)
    pendingRedraw.delete(id)
    if (mouseInputTail[id]) {
      mouseInputTail = {...mouseInputTail, [id]: ''}
    }
  }

  const resetTerminalInstance = (id: string): void => {
    const handle = terminals.get(id)
    if (!handle) return
    handle.terminal.reset()
    handle.terminal.clear()
    handle.terminal.scrollToBottom()
    handle.fitAddon.fit()
    resizeKittyOverlay(handle)
    modeMap = {
      ...modeMap,
      [id]: {altScreen: false, mouse: false, mouseSGR: false, mouseEncoding: 'x10'}
    }
    if (mouseInputTail[id]) {
      mouseInputTail = {...mouseInputTail, [id]: ''}
    }
    noteMouseSuppress(id, 2500)
    void loadRendererAddon(handle, id, rendererModeMap[id] ?? rendererPreference)
  }

  const setReplayState = (id: string, state: 'idle' | 'replaying' | 'live'): void => {
    replayState.set(id, state)
    if (state !== 'live') {
      return
    }
    const kittyEvents = pendingReplayKitty.get(id)
    if (kittyEvents && kittyEvents.length > 0) {
      pendingReplayKitty.delete(id)
      for (const event of kittyEvents) {
        void applyKittyEvent(id, event)
      }
    }
    const pending = pendingReplayOutput.get(id)
    if (pending && pending.length > 0) {
      pendingReplayOutput.delete(id)
      enqueueOutput(id, pending.join(''))
    }
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
    if (healthMap[id] === 'checking' || healthMap[id] === 'unknown') {
      setHealth(id, 'ok', 'Output received.')
    }
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
    setHealth(id, 'checking', 'Waiting for output…')
    const startedAt = Date.now()
    pendingHealthCheck.set(id, startedAt)
    window.setTimeout(() => {
      if (healthMap[id] === 'checking') {
        const stats =
          statsMap.get(id) ??
          {bytesIn: 0, bytesOut: 0, backlog: 0, lastOutputAt: 0, lastCprAt: 0}
        if (stats.lastOutputAt >= startedAt) {
          setHealth(id, 'ok', 'Output received.')
        } else {
          setHealth(id, 'unknown', 'No output yet.')
        }
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

  const logDebug = (id: string, event: string, details?: Record<string, unknown>): void => {
    if (!debugEnabled) return
    const payload = details ? JSON.stringify(details) : ''
    void logTerminalDebug(id, event, payload)
  }

  const noteRender = (id: string): void => {
    const stats = renderStatsMap.get(id) ?? {lastRenderAt: 0, renderCount: 0}
    stats.lastRenderAt = Date.now()
    stats.renderCount += 1
    renderStatsMap.set(id, stats)
  }

  const hasVisibleContent = (terminal: Terminal): boolean => {
    const buffer = terminal.buffer.active
    const rows = terminal.rows
    for (let i = 0; i < rows; i += 1) {
      const line = buffer.getLine(i)
      if (!line) continue
      const text = line.translateToString(true)
      if (text.trim().length > 0) {
        return true
      }
    }
    return false
  }

  const forceCanvasRenderer = (id: string, handle: TerminalHandle): void => {
    if (handle.webglAddon) {
      try {
        handle.webglAddon.dispose()
      } catch {
        // Ignore disposal failures.
      }
      handle.webglAddon = undefined
    }
    rendererMap = {...rendererMap, [id]: 'canvas'}
    logDebug(id, 'renderer_fallback', {
      mode: rendererModeMap[id] ?? 'auto',
      renderer: 'canvas'
    })
  }

  const nudgeTerminalRedraw = (id: string): void => {
    if (pendingRedraw.has(id)) return
    const handle = terminals.get(id)
    if (!handle) return
    const dims = handle.fitAddon.proposeDimensions()
    if (!dims) return
    const cols = Math.max(2, dims.cols)
    const rows = Math.max(1, dims.rows)
    const nudgeCols = cols + 1
    pendingRedraw.add(id)
    void ResizeWorkspaceTerminal(id, nudgeCols, rows).catch(() => undefined)
    logDebug(id, 'redraw_nudge', {cols, rows, nudgeCols})
    window.setTimeout(() => {
      void ResizeWorkspaceTerminal(id, cols, rows).catch(() => undefined)
      pendingRedraw.delete(id)
    }, 60)
  }

  const scheduleRenderHealthCheck = (id: string, payloadBytes: number): void => {
    if (!id || payloadBytes <= 0 || pendingRenderCheck.has(id)) return
    const startedAt = Date.now()
    const timer = window.setTimeout(() => {
      pendingRenderCheck.delete(id)
      const handle = terminals.get(id)
      if (!handle) return
      const stats = renderStatsMap.get(id)
      if (stats && stats.lastRenderAt >= startedAt) {
        return
      }
      handle.fitAddon.fit()
      window.setTimeout(() => {
        const updated = renderStatsMap.get(id)
        if (updated && updated.lastRenderAt >= startedAt) {
          return
        }
        if (rendererMap[id] === 'webgl' && rendererModeMap[id] !== 'webgl') {
          forceCanvasRenderer(id, handle)
          handle.fitAddon.fit()
        }
        if (!hasVisibleContent(handle.terminal)) {
          nudgeTerminalRedraw(id)
        }
        logDebug(id, 'render_health_check', {
          rendered: updated ? updated.lastRenderAt >= startedAt : false,
          renderer: rendererMap[id] ?? 'unknown'
        })
      }, RENDER_RECOVERY_DELAY_MS)
    }, RENDER_CHECK_DELAY_MS)
    pendingRenderCheck.set(id, timer)
  }

  const createKittyState = (): KittyState => ({
    images: new Map(),
    placements: new Map()
  })

  const kittyPlacementKey = (imageId: string, placementId: number): string => `${imageId}:${placementId}`

  const decodeBase64 = (input: string | number[] | Uint8Array): Uint8Array => {
    if (!input) return new Uint8Array()
    if (input instanceof Uint8Array) {
      return input
    }
    if (Array.isArray(input)) {
      return Uint8Array.from(input)
    }
    const binary = atob(input)
    const output = new Uint8Array(binary.length)
    for (let i = 0; i < binary.length; i += 1) {
      output[i] = binary.charCodeAt(i)
    }
    return output
  }

  const decodeKittyImage = async (image: KittyImage): Promise<void> => {
    if (image.bitmap || image.decoding) {
      if (image.decoding) {
        await image.decoding
      }
      return
    }
    image.decoding = (async () => {
      try {
        if (image.format === 'png' || image.format === '') {
          const bytes = image.data.slice()
          const blob = new Blob([bytes], {type: 'image/png'})
          image.bitmap = await createImageBitmap(blob)
          return
        }
        const channels = image.format === 'rgba' ? 4 : 3
        if (image.width <= 0 || image.height <= 0) {
          return
        }
        const expected = image.width * image.height * channels
        if (image.data.length < expected) {
          return
        }
        const canvas = document.createElement('canvas')
        canvas.width = image.width
        canvas.height = image.height
        const ctx = canvas.getContext('2d')
        if (!ctx) return
        const imageData = ctx.createImageData(image.width, image.height)
        if (channels === 4) {
          imageData.data.set(image.data.subarray(0, expected))
        } else {
          let src = 0
          for (let i = 0; i < imageData.data.length; i += 4) {
            imageData.data[i] = image.data[src]
            imageData.data[i + 1] = image.data[src + 1]
            imageData.data[i + 2] = image.data[src + 2]
            imageData.data[i + 3] = 255
            src += 3
          }
        }
        ctx.putImageData(imageData, 0, 0)
        image.bitmap = await createImageBitmap(canvas)
      } catch {
        // Best-effort; keep rendering text if bitmap fails to decode.
      }
    })()
    await image.decoding
  }

  const ensureKittyOverlay = (handle: TerminalHandle, id: string): void => {
    if (handle.kittyOverlay || !handle.container) return
    const underlay = document.createElement('canvas')
    underlay.className = 'kitty-layer kitty-underlay'
    const overlay = document.createElement('canvas')
    overlay.className = 'kitty-layer kitty-overlay'
    handle.container.appendChild(underlay)
    handle.container.appendChild(overlay)
    const ctxUnder = underlay.getContext('2d')
    const ctxOver = overlay.getContext('2d')
    if (!ctxUnder || !ctxOver) return
    const kittyOverlay: KittyOverlay = {
      underlay,
      overlay,
      ctxUnder,
      ctxOver,
      cellWidth: 0,
      cellHeight: 0,
      dpr: window.devicePixelRatio || 1,
      renderScheduled: false
    }
    handle.kittyOverlay = kittyOverlay
    resizeKittyOverlay(handle)
    scheduleKittyRender(id)
    const disposables: {dispose: () => void}[] = []
    disposables.push(
      handle.terminal.onRender(() => {
        scheduleKittyRender(id)
      })
    )
    disposables.push(
      handle.terminal.onScroll(() => {
        scheduleKittyRender(id)
      })
    )
    disposables.push(
      handle.terminal.onWriteParsed(() => {
        scheduleKittyRender(id)
      })
    )
    disposables.push(
      handle.terminal.onResize(() => {
        resizeKittyOverlay(handle)
        scheduleKittyRender(id)
      })
    )
    const maybeDimensions = (handle.terminal as unknown as {
      onDimensionsChange?: (fn: (dimensions: {css: {cell: {width: number; height: number}}}) => void) => {
        dispose: () => void
      }
    }).onDimensionsChange
    if (maybeDimensions) {
      disposables.push(
        maybeDimensions((dimensions) => {
          if (!handle.kittyOverlay) return
          handle.kittyOverlay.cellWidth = dimensions.css.cell.width
          handle.kittyOverlay.cellHeight = dimensions.css.cell.height
          scheduleKittyRender(id)
        })
      )
    }
    handle.kittyDisposables = disposables
  }

  const resizeKittyOverlay = (handle: TerminalHandle): void => {
    if (!handle.kittyOverlay || !handle.container) return
    const rect = handle.container.getBoundingClientRect()
    const dpr = window.devicePixelRatio || 1
    handle.kittyOverlay.dpr = dpr
    for (const canvas of [handle.kittyOverlay.underlay, handle.kittyOverlay.overlay]) {
      canvas.width = Math.max(1, Math.floor(rect.width * dpr))
      canvas.height = Math.max(1, Math.floor(rect.height * dpr))
      canvas.style.width = `${rect.width}px`
      canvas.style.height = `${rect.height}px`
    }
    handle.kittyOverlay.ctxUnder.setTransform(dpr, 0, 0, dpr, 0, 0)
    handle.kittyOverlay.ctxOver.setTransform(dpr, 0, 0, dpr, 0, 0)
  }

  const scheduleKittyRender = (id: string): void => {
    const handle = terminals.get(id)
    if (!handle || !handle.kittyOverlay) return
    if (handle.kittyOverlay.renderScheduled) return
    handle.kittyOverlay.renderScheduled = true
    requestAnimationFrame(() => {
      if (!handle.kittyOverlay) return
      handle.kittyOverlay.renderScheduled = false
      renderKitty(id)
    })
  }

  const renderKitty = (id: string): void => {
    const handle = terminals.get(id)
    if (!handle || !handle.kittyOverlay) return
    const overlay = handle.kittyOverlay
    const state = handle.kittyState
    const width = overlay.overlay.width / overlay.dpr
    const height = overlay.overlay.height / overlay.dpr
    overlay.ctxUnder.clearRect(0, 0, width, height)
    overlay.ctxOver.clearRect(0, 0, width, height)
    if (state.images.size === 0 || state.placements.size === 0) return
    const cellWidth = overlay.cellWidth || width / handle.terminal.cols
    const cellHeight = overlay.cellHeight || height / handle.terminal.rows
    if (!Number.isFinite(cellWidth) || !Number.isFinite(cellHeight) || cellWidth <= 0 || cellHeight <= 0) return
    const placements = Array.from(state.placements.values()).sort((a, b) => a.z - b.z)
    for (const placement of placements) {
      const image = state.images.get(placement.imageId)
      if (!image || !image.bitmap) {
        continue
      }
      const x = placement.col * cellWidth + placement.x
      const y = placement.row * cellHeight + placement.y
      const imageWidth = image.width || image.bitmap.width
      const imageHeight = image.height || image.bitmap.height
      const drawWidth = placement.cols > 0 ? placement.cols * cellWidth : imageWidth
      const drawHeight = placement.rows > 0 ? placement.rows * cellHeight : imageHeight
      if (drawWidth <= 0 || drawHeight <= 0) continue
      const ctx = placement.z < 0 ? overlay.ctxUnder : overlay.ctxOver
      ctx.drawImage(image.bitmap, x, y, drawWidth, drawHeight)
    }
  }

  const applyKittyEvent = async (id: string, event: KittyEventPayload): Promise<void> => {
    const handle = terminals.get(id)
    if (!handle) return
    const state = handle.kittyState
    if (event.kind === 'snapshot' && event.snapshot) {
      state.images.clear()
      state.placements.clear()
      for (const image of event.snapshot.images ?? []) {
        if (!image?.id) continue
        const kittyImage: KittyImage = {
          id: image.id,
          format: image.format ?? 'png',
          width: image.width ?? 0,
          height: image.height ?? 0,
          data: decodeBase64(image.data ?? '')
        }
        state.images.set(kittyImage.id, kittyImage)
        await decodeKittyImage(kittyImage)
      }
      for (const placement of event.snapshot.placements ?? []) {
        const kittyPlacement: KittyPlacement = {
          id: placement.id,
          imageId: placement.imageId,
          row: placement.row,
          col: placement.col,
          rows: placement.rows,
          cols: placement.cols,
          x: placement.x ?? 0,
          y: placement.y ?? 0,
          z: placement.z ?? 0
        }
        state.placements.set(kittyPlacementKey(kittyPlacement.imageId, kittyPlacement.id), kittyPlacement)
      }
      scheduleKittyRender(id)
      return
    }
    if (event.kind === 'delete' && event.delete) {
      if (event.delete.all) {
        state.images.clear()
        state.placements.clear()
      } else if (event.delete.imageId) {
        state.images.delete(event.delete.imageId)
        for (const [key, placement] of state.placements.entries()) {
          if (placement.imageId === event.delete.imageId) {
            state.placements.delete(key)
          }
        }
      } else if (event.delete.placementId && event.delete.imageId) {
        state.placements.delete(kittyPlacementKey(event.delete.imageId, event.delete.placementId))
      }
      scheduleKittyRender(id)
      return
    }
    if (event.kind === 'image' && event.image) {
      const kittyImage: KittyImage = {
        id: event.image.id,
        format: event.image.format ?? 'png',
        width: event.image.width ?? 0,
        height: event.image.height ?? 0,
        data: decodeBase64(event.image.data ?? '')
      }
      state.images.set(kittyImage.id, kittyImage)
      await decodeKittyImage(kittyImage)
      scheduleKittyRender(id)
      return
    }
    if (event.kind === 'placement' && event.placement) {
      const kittyPlacement: KittyPlacement = {
        id: event.placement.id,
        imageId: event.placement.imageId,
        row: event.placement.row,
        col: event.placement.col,
        rows: event.placement.rows,
        cols: event.placement.cols,
        x: event.placement.x ?? 0,
        y: event.placement.y ?? 0,
        z: event.placement.z ?? 0
      }
      state.placements.set(kittyPlacementKey(kittyPlacement.imageId, kittyPlacement.id), kittyPlacement)
      scheduleKittyRender(id)
    }
  }

  const loadRendererAddon = async (
    handle: TerminalHandle,
    id: string,
    mode: 'auto' | 'webgl' | 'canvas'
  ): Promise<void> => {
    rendererModeMap = {...rendererModeMap, [id]: mode}
    if (mode === 'canvas') {
      forceCanvasRenderer(id, handle)
      return
    }
    if (handle.webglAddon) {
      try {
        handle.webglAddon.dispose()
      } catch {
        // Best-effort cleanup.
      }
      handle.webglAddon = undefined
    }
    try {
      if (typeof document !== 'undefined' && document.fonts?.ready) {
        await document.fonts.ready
      }
    } catch {
      // Font readiness is best-effort; continue if unavailable.
    }

    try {
      const webglAddon = new WebglAddon()
      webglAddon.onContextLoss(() => {
        webglAddon.dispose()
        rendererMap = {...rendererMap, [id]: 'canvas'}
      })
      handle.terminal.loadAddon(webglAddon)
      handle.webglAddon = webglAddon
      rendererMap = {...rendererMap, [id]: 'webgl'}
      logDebug(id, 'renderer_loaded', {mode, renderer: 'webgl'})
    } catch {
      rendererMap = {...rendererMap, [id]: 'canvas'}
      logDebug(id, 'renderer_loaded', {mode, renderer: 'canvas'})
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
      terminal.attachCustomWheelEventHandler((event) => handleWheel(id, terminal, event))
      const dataDisposable = terminal.onData((data) => {
        inputMap = {...inputMap, [id]: true}
        void beginTerminal(id)
        captureCpr(id, data)
        sendInput(id, data)
      })
      const binaryDisposable = terminal.onBinary((data) => {
        if (!data) return
        inputMap = {...inputMap, [id]: true}
        void beginTerminal(id)
        sendInput(id, data)
      })
      terminal.onRender(() => {
        noteRender(id)
      })
      const container = document.createElement('div')
      container.className = 'terminal-instance'
      handle = {
        terminal,
        fitAddon,
        dataDisposable,
        binaryDisposable,
        container,
        kittyState: createKittyState()
      }
      terminals.set(id, handle)
      if (!modeMap[id]) {
        modeMap = {
          ...modeMap,
          [id]: {altScreen: false, mouse: false, mouseSGR: false, mouseEncoding: 'x10'}
        }
      }
    }
    if (terminalContainer) {
      terminalContainer.querySelectorAll('.terminal-instance').forEach((node) => {
        node.setAttribute('data-active', 'false')
      })
      if (!terminalContainer.contains(handle.container)) {
        terminalContainer.appendChild(handle.container)
        handle.terminal.open(handle.container)
        ensureKittyOverlay(handle, id)
        void loadRendererAddon(handle, id, rendererPreference)
        if (typeof document !== 'undefined' && document.fonts?.ready) {
          document.fonts
            .ready
            .then(() => {
              const current = terminals.get(id)
              if (!current) return
              current.terminal.options.lineHeight =
                computeLineHeight(BASE_FONT_SIZE, BASE_LINE_HEIGHT)
              current.fitAddon.fit()
              resizeKittyOverlay(current)
              const updated = current.fitAddon.proposeDimensions()
              if (updated) {
                void ResizeWorkspaceTerminal(id, updated.cols, updated.rows).catch(() => undefined)
              }
            })
            .catch(() => undefined)
        }
      }
      handle.container.setAttribute('data-active', 'true')
      handle.fitAddon.fit()
      resizeKittyOverlay(handle)
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
        if (!inputMap[payload.workspaceId]) {
          inputMap = {...inputMap, [payload.workspaceId]: true}
        }
        if (replayState.get(payload.workspaceId) !== 'live') {
          const pending = pendingReplayOutput.get(payload.workspaceId) ?? []
          pending.push(payload.data)
          pendingReplayOutput.set(payload.workspaceId, pending)
          return
        }
        enqueueOutput(payload.workspaceId, payload.data)
      }
      EventsOn('terminal:data', handler)
      listeners.add('terminal:data')
    }
    if (!listeners.has('terminal:kitty')) {
      const handler = (payload: {workspaceId: string; event: KittyEventPayload}): void => {
        const handle = terminals.get(payload.workspaceId)
        if (!handle) return
        if (!inputMap[payload.workspaceId]) {
          inputMap = {...inputMap, [payload.workspaceId]: true}
        }
        if (replayState.get(payload.workspaceId) !== 'live') {
          const pending = pendingReplayKitty.get(payload.workspaceId) ?? []
          pending.push(payload.event)
          pendingReplayKitty.set(payload.workspaceId, pending)
          return
        }
        void applyKittyEvent(payload.workspaceId, payload.event)
      }
      EventsOn('terminal:kitty', handler)
      listeners.add('terminal:kitty')
    }
    if (!listeners.has('terminal:lifecycle')) {
      const handler = (payload: {
        workspaceId: string
        status: 'started' | 'closed' | 'error' | 'idle'
        message?: string
      }): void => {
        if (payload.status === 'started') {
          const message = payload.message?.toLowerCase() ?? ''
          const isResume =
            message.includes('backlog truncated') || message.includes('session resumed')
          startedSessions.add(payload.workspaceId)
          statusMap = {
            ...statusMap,
            [payload.workspaceId]: isResume ? 'ready' : 'starting'
          }
          messageMap = {
            ...messageMap,
            [payload.workspaceId]: isResume
              ? payload.message ?? ''
              : 'Waiting for shell output…'
          }
          inputMap = {...inputMap, [payload.workspaceId]: isResume}
          if (isResume) {
            clearStartupTimeout(payload.workspaceId)
            setHealth(payload.workspaceId, 'ok', 'Session resumed (TUI state not replayed).')
          } else {
            scheduleStartupTimeout(payload.workspaceId)
            setHealth(payload.workspaceId, 'unknown')
          }
          return
        }
        if (payload.status === 'closed') {
          startedSessions.delete(payload.workspaceId)
          statusMap = {...statusMap, [payload.workspaceId]: 'closed'}
          setHealth(payload.workspaceId, 'stale', 'Terminal closed.')
          clearStartupTimeout(payload.workspaceId)
          resetSessionState(payload.workspaceId)
          return
        }
        if (payload.status === 'idle') {
          startedSessions.delete(payload.workspaceId)
          statusMap = {...statusMap, [payload.workspaceId]: 'idle'}
          setHealth(payload.workspaceId, 'stale', 'Terminal idle.')
          clearStartupTimeout(payload.workspaceId)
          resetSessionState(payload.workspaceId)
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
          resetSessionState(payload.workspaceId)
        }
      }
      EventsOn('terminal:lifecycle', handler)
      listeners.add('terminal:lifecycle')
    }
    if (!listeners.has('terminal:modes')) {
      const handler = (payload: {
        workspaceId: string
        altScreen: boolean
        mouse: boolean
        mouseSGR: boolean
        mouseEncoding?: string
      }): void => {
        modeMap = {
          ...modeMap,
          [payload.workspaceId]: {
            altScreen: payload.altScreen,
            mouse: payload.mouse,
            mouseSGR: payload.mouseSGR,
            mouseEncoding: payload.mouseEncoding ?? (payload.mouseSGR ? 'sgr' : 'x10')
          }
        }
      }
      EventsOn('terminal:modes', handler)
      listeners.add('terminal:modes')
    }
    if (!listeners.has('sessiond:restarted')) {
      const handler = (): void => {
        sessiondChecked = false
        void (async () => {
          if (workspaceId) {
            statusMap = {...statusMap, [workspaceId]: 'starting'}
            messageMap = {
              ...messageMap,
              [workspaceId]: 'Session daemon restarted. Reconnecting…'
            }
            setHealth(workspaceId, 'checking', 'Reconnecting after daemon restart.')
          }
          await refreshSessiondStatus()
          if (!workspaceId || sessiondAvailable !== true) return
          startedSessions.delete(workspaceId)
          startInFlight.delete(workspaceId)
          resetTerminalInstance(workspaceId)
          resetSessionState(workspaceId)
          noteMouseSuppress(workspaceId, 4000)
          void beginTerminal(workspaceId, true)
        })()
      }
      EventsOn('sessiond:restarted', handler)
      listeners.add('sessiond:restarted')
    }
  }

  const cleanupListeners = (): void => {
    if (listeners.has('terminal:data')) {
      EventsOff('terminal:data')
      listeners.delete('terminal:data')
    }
    if (listeners.has('terminal:kitty')) {
      EventsOff('terminal:kitty')
      listeners.delete('terminal:kitty')
    }
    if (listeners.has('terminal:lifecycle')) {
      EventsOff('terminal:lifecycle')
      listeners.delete('terminal:lifecycle')
    }
    if (listeners.has('terminal:modes')) {
      EventsOff('terminal:modes')
      listeners.delete('terminal:modes')
    }
    if (listeners.has('sessiond:restarted')) {
      EventsOff('sessiond:restarted')
      listeners.delete('sessiond:restarted')
    }
  }

  const initTerminal = async (id: string, name: string): Promise<void> => {
    if (!id) return
    const token = ++initCounter
    ensureListener()
    if (!sessiondChecked) {
      await refreshSessiondStatus()
    }
    attachTerminal(id, name)
    let resumed = false
    if (sessiondAvailable === true) {
      try {
        const status = await fetchWorkspaceTerminalStatus(id)
        resumed = status?.active ?? false
      } catch {
        resumed = false
      }
    }
    if (resumed) {
      if (startedSessions.has(id) || startInFlight.has(id)) {
        resetSessionState(id)
        await loadBootstrap(id)
      } else {
        await beginTerminal(id, true)
      }
      inputMap = {...inputMap, [id]: true}
      statusMap = {...statusMap, [id]: 'ready'}
      messageMap = {...messageMap, [id]: ''}
      setHealth(id, 'ok', 'Session resumed.')
      if (!rendererMap[id]) {
        rendererMap = {...rendererMap, [id]: 'unknown'}
      }
      if (!rendererModeMap[id]) {
        rendererModeMap = {...rendererModeMap, [id]: rendererPreference}
      }
      return
    }
    void loadBootstrap(id)
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

  const refreshSessiondStatus = async (): Promise<void> => {
    try {
      const status = await fetchSessiondStatus()
      sessiondAvailable = status?.available ?? false
    } catch {
      sessiondAvailable = false
    } finally {
      sessiondChecked = true
    }
  }

  const loadBootstrap = async (id: string): Promise<void> => {
    if (!id || replayLoading.has(id)) return
    replayLoading.add(id)
    setReplayState(id, 'replaying')
    let replayFinished = false
    const finishReplay = (): void => {
      if (replayFinished) return
      replayFinished = true
      replayLoading.delete(id)
      setReplayState(id, 'live')
    }
    try {
      const result = await fetchTerminalBootstrap(id)
      const snapshotBytes = result?.snapshot?.length ?? 0
      const backlogBytes = result?.backlog?.length ?? 0
      logDebug(id, 'bootstrap', {
        snapshotBytes,
        backlogBytes,
        snapshotSource: result?.snapshotSource ?? '',
        backlogSource: result?.backlogSource ?? '',
        truncated: result?.backlogTruncated ?? false,
        source: result?.source ?? '',
        altScreen: result?.altScreen ?? false,
        mouse: result?.mouse ?? false,
        mouseSGR: result?.mouseSGR ?? false,
        mouseEncoding: result?.mouseEncoding ?? '',
        safeToReplay: result?.safeToReplay ?? false
      })
      if (result) {
        modeMap = {
          ...modeMap,
          [id]: {
            altScreen: result.altScreen ?? false,
            mouse: result.mouse ?? false,
            mouseSGR: result.mouseSGR ?? false,
            mouseEncoding: result.mouseEncoding ?? (result.mouseSGR ? 'sgr' : 'x10')
          }
        }
      }
      const writeAndWait = async (handle: TerminalHandle, data: string): Promise<void> => {
        await new Promise<void>((resolve) => {
          handle.terminal.write(data, () => resolve())
        })
        handle.fitAddon.fit()
        resizeKittyOverlay(handle)
      }
      if (result?.snapshot) {
        const handle = terminals.get(id)
        if (handle) {
          await writeAndWait(handle, result.snapshot)
          handle.terminal.scrollToBottom()
          updateStats(id, (stats) => {
            stats.bytesIn += snapshotBytes
            stats.lastOutputAt = Date.now()
          })
        }
        if (snapshotBytes > 0) {
          nudgeTerminalRedraw(id)
        }
      } else if (result?.backlog) {
        const handle = terminals.get(id)
        if (handle) {
          await writeAndWait(handle, result.backlog)
          handle.terminal.scrollToBottom()
          updateStats(id, (stats) => {
            stats.bytesIn += backlogBytes
            stats.lastOutputAt = Date.now()
          })
        }
        if (result?.backlogTruncated) {
          setHealth(id, 'ok', 'Backlog truncated; showing latest output.')
        }
      }
      if (result?.kitty) {
        void applyKittyEvent(id, {kind: 'snapshot', snapshot: result.kitty})
      }
      if (!inputMap[id]) {
        inputMap = {...inputMap, [id]: true}
      }
      if (statusMap[id] !== 'ready') {
        statusMap = {...statusMap, [id]: 'ready'}
      }
      scheduleRenderHealthCheck(id, snapshotBytes + backlogBytes)
      finishReplay()
    } catch {
      // Bootstrap is best-effort; continue to live stream.
    } finally {
      finishReplay()
    }
  }

  onMount(() => {
    if (!terminalContainer) return
    debugEnabled =
      typeof localStorage !== 'undefined' && localStorage.getItem('worksetTerminalDebug') === '1'
    void loadAgentDefault()
    void loadAgentAvailability()
    void refreshSessiondStatus()
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
        resizeKittyOverlay(handle)
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
        class="daemon-status"
        class:offline={sessiondAvailable === false}
        class:online={sessiondAvailable === true}
        title={sessiondAvailable === true
          ? 'Session daemon active'
          : sessiondAvailable === false
            ? 'Session daemon unavailable (using local shell)'
            : 'Checking session daemon status'}
      >
        {#if sessiondAvailable === true}
          Session: daemon
        {:else if sessiondAvailable === false}
          Session: local
        {:else}
          Session: checking
        {/if}
      </div>
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

  .daemon-status {
    font-size: 11px;
    color: var(--muted);
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 2px 8px;
    background: rgba(255, 255, 255, 0.02);
    letter-spacing: 0.02em;
  }

  .daemon-status.online {
    color: var(--success);
    border-color: color-mix(in srgb, var(--success) 50%, var(--border));
    background: color-mix(in srgb, var(--success) 12%, transparent);
  }

  .daemon-status.offline {
    color: var(--warning);
    border-color: color-mix(in srgb, var(--warning) 50%, var(--border));
    background: color-mix(in srgb, var(--warning) 12%, transparent);
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
    overflow: hidden;
  }

  :global(.terminal-instance .kitty-layer) {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }

  :global(.terminal-instance .kitty-underlay) {
    z-index: 0;
  }

  :global(.terminal-instance .xterm) {
    position: relative;
    z-index: 1;
  }

  :global(.terminal-instance .kitty-overlay) {
    z-index: 2;
  }

  :global(.terminal-instance[data-active='true']) {
    opacity: 1;
    pointer-events: auto;
  }
</style>
