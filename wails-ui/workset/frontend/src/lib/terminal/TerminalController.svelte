<script module lang="ts">
	import { EventsOff, EventsOn } from '../../../wailsjs/runtime/runtime';

	type EventHandler<T> = (payload: T) => void;
	type EventRegistryEntry = {
		handlers: Set<EventHandler<unknown>>;
		bound: boolean;
	};

	const eventRegistry = new Map<string, EventRegistryEntry>();

	const subscribeEvent = <T,>(event: string, handler: EventHandler<T>): (() => void) => {
		let entry = eventRegistry.get(event);
		if (!entry) {
			entry = { handlers: new Set(), bound: false };
			eventRegistry.set(event, entry);
		}
		entry.handlers.add(handler as EventHandler<unknown>);
		if (!entry.bound) {
			EventsOn(event, (payload: T) => {
				const current = eventRegistry.get(event);
				if (!current) return;
				for (const registered of current.handlers) {
					registered(payload as unknown);
				}
			});
			entry.bound = true;
		}
		return () => {
			const current = eventRegistry.get(event);
			if (!current) return;
			current.handlers.delete(handler as EventHandler<unknown>);
			if (current.handlers.size === 0 && current.bound) {
				EventsOff(event);
				current.bound = false;
			}
		};
	};
</script>

<script lang="ts">
	import { onDestroy, onMount, untrack } from 'svelte';
	import {
		Terminal,
		type ITerminalInitOnlyOptions,
		type ITerminalOptions,
		type ITheme,
	} from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import { CanvasAddon } from '@xterm/addon-canvas';
	import { WebglAddon } from '@xterm/addon-webgl';
	import '@xterm/xterm/css/xterm.css';
	import { Environment } from '../../../wailsjs/runtime/runtime';
	import {
		fetchSettings,
		fetchSessiondStatus,
		fetchWorkspaceTerminalStatus,
		logTerminalDebug,
	} from '../api';
	import {
		AckWorkspaceTerminal,
		ResizeWorkspaceTerminal,
		StartWorkspaceTerminal,
		WriteWorkspaceTerminal,
	} from '../../../wailsjs/go/main/App';
	import { stripMouseReports } from '../terminal/inputFilter';
	import { encodeWheel, type MouseEncoding } from '../terminal/mouse';

	interface Props {
		workspaceId: string;
		workspaceName: string;
		terminalId: string;
		active?: boolean;
		terminalContainer?: HTMLDivElement | null;
		onStateChange?: (state: {
			status: string;
			message: string;
			health: 'unknown' | 'checking' | 'ok' | 'stale';
			healthMessage: string;
			renderer: 'unknown' | 'webgl' | 'canvas';
			rendererMode: 'auto' | 'webgl' | 'canvas';
			sessiondAvailable: boolean | null;
			sessiondChecked: boolean;
			debugEnabled: boolean;
			debugStats: {
				bytesIn: number;
				bytesOut: number;
				backlog: number;
				lastOutputAt: number;
				lastCprAt: number;
			};
		}) => void;
	}

	const {
		workspaceId,
		workspaceName,
		terminalId,
		active = true,
		terminalContainer = null,
		onStateChange = undefined,
	}: Props = $props();

	const terminalKey = $derived(terminalId || workspaceId);
	const resolvePayloadId = (payload: { terminalId?: string; workspaceId?: string }): string =>
		payload.terminalId || payload.workspaceId || '';
	const resolveBackendId = (id: string): string => (id === terminalKey ? terminalId : id);

	let resizeObserver: ResizeObserver | null = null;
	let observedContainer: HTMLDivElement | null = null;
	const fitStabilizers = new Map<string, number>();
	const reattachTimers = new Map<string, number>();
	let lastWorkspaceId = $state('');

	type TerminalHandle = {
		terminal: Terminal;
		fitAddon: FitAddon;
		dataDisposable: { dispose: () => void };
		binaryDisposable?: { dispose: () => void };
		container: HTMLDivElement;
		kittyState: KittyState;
		kittyOverlay?: KittyOverlay;
		kittyDisposables?: { dispose: () => void }[];
		oscDisposables?: { dispose: () => void }[];
		canvasAddon?: CanvasAddon;
		webglAddon?: WebglAddon;
	};

	type KittyImage = {
		id: string;
		format: string;
		width: number;
		height: number;
		data: Uint8Array;
		bitmap?: ImageBitmap;
		decoding?: Promise<void>;
	};

	type KittyPlacement = {
		id: number;
		imageId: string;
		row: number;
		col: number;
		rows: number;
		cols: number;
		x: number;
		y: number;
		z: number;
	};

	type KittyState = {
		images: Map<string, KittyImage>;
		placements: Map<string, KittyPlacement>;
	};

	type KittyOverlay = {
		underlay: HTMLCanvasElement;
		overlay: HTMLCanvasElement;
		ctxUnder: CanvasRenderingContext2D;
		ctxOver: CanvasRenderingContext2D;
		cellWidth: number;
		cellHeight: number;
		dpr: number;
		renderScheduled: boolean;
	};

	type OutputChunk = {
		data: string;
		bytes: number;
	};

	type KittyEventPayload = {
		kind: string;
		image?: {
			id: string;
			format?: string;
			width?: number;
			height?: number;
			data?: string | number[] | Uint8Array;
		};
		placement?: {
			id: number;
			imageId: string;
			row: number;
			col: number;
			rows: number;
			cols: number;
			x?: number;
			y?: number;
			z?: number;
		};
		delete?: {
			all?: boolean;
			imageId?: string;
			placementId?: number;
		};
		snapshot?: {
			images?: Array<{
				id: string;
				format?: string;
				width?: number;
				height?: number;
				data?: string | number[] | Uint8Array;
			}>;
			placements?: Array<{
				id: number;
				imageId: string;
				row: number;
				col: number;
				rows: number;
				cols: number;
				x?: number;
				y?: number;
				z?: number;
			}>;
		};
	};

	const terminals = new Map<string, TerminalHandle>();
	const outputQueues = new Map<
		string,
		{ chunks: OutputChunk[]; bytes: number; scheduled: boolean }
	>();
	const replayState = new Map<string, 'idle' | 'replaying' | 'live'>();
	const pendingReplayOutput = new Map<string, OutputChunk[]>();
	const pendingReplayKitty = new Map<string, KittyEventPayload[]>();
	const lastDims = new Map<string, { cols: number; rows: number }>();
	const startupTimers = new Map<string, number>();
	const startInFlight = new Set<string>();
	const statsMap = new Map<
		string,
		{
			bytesIn: number;
			bytesOut: number;
			backlog: number;
			lastOutputAt: number;
			lastCprAt: number;
		}
	>();
	const renderStatsMap = new Map<string, { lastRenderAt: number; renderCount: number }>();
	const pendingInput = new Map<string, string>();
	const pendingHealthCheck = new Map<string, number>();
	const pendingRenderCheck = new Map<string, number>();
	const pendingRedraw = new Set<string>();
	const renderCheckLogged = new Set<string>();
	const reopenAttempted = new Set<string>();
	const pendingAckBytes = new Map<string, number>();
	const initialCreditMap = new Map<string, number>();
	const ackTimers = new Map<string, number>();
	const initialCreditSent = new Set<string>();
	let initCounter = 0;
	const startedSessions = new Set<string>();
	let resizeScheduled = false;
	let resizeTimer: number | null = null;
	let debugInterval: number | null = null;
	let rendererPreference = $state<'auto' | 'webgl' | 'canvas'>('auto');
	let sessiondAvailable = $state<boolean | null>(null);
	let sessiondChecked = $state(false);
	let forceCanvasRendererEnabled = $state(false);
	let statusMap: Record<string, string> = $state({});
	let messageMap: Record<string, string> = $state({});
	let inputMap: Record<string, boolean> = $state({});
	let healthMap: Record<string, 'unknown' | 'checking' | 'ok' | 'stale'> = $state({});
	let healthMessageMap: Record<string, string> = $state({});
	let suppressMouseUntil: Record<string, number> = $state({});
	let mouseInputTail: Record<string, string> = $state({});
	let rendererMap: Record<string, 'unknown' | 'webgl' | 'canvas'> = $state({});
	let rendererModeMap: Record<string, 'auto' | 'webgl' | 'canvas'> = $state({});
	let modeMap: Record<
		string,
		{ altScreen: boolean; mouse: boolean; mouseSGR: boolean; mouseEncoding: string }
	> = $state({});
	let debugEnabled = $state(false);
	let debugStats = $state({
		bytesIn: 0,
		bytesOut: 0,
		backlog: 0,
		lastOutputAt: 0,
		lastCprAt: 0,
	});

	const activeStatus = $derived(terminalKey ? (statusMap[terminalKey] ?? '') : '');
	const activeMessage = $derived(terminalKey ? (messageMap[terminalKey] ?? '') : '');
	const activeHealth = $derived(terminalKey ? (healthMap[terminalKey] ?? 'unknown') : 'unknown');
	const activeHealthMessage = $derived(terminalKey ? (healthMessageMap[terminalKey] ?? '') : '');
	const activeRenderer = $derived(
		terminalKey ? (rendererMap[terminalKey] ?? 'unknown') : 'unknown',
	);
	const activeRendererMode = $derived(
		terminalKey ? (rendererModeMap[terminalKey] ?? 'auto') : 'auto',
	);
	const listeners = new Set<string>();
	const unsubscribeHandlers: Array<() => void> = [];
	$effect(() => {
		if (!onStateChange) return;
		onStateChange({
			status: activeStatus,
			message: activeMessage,
			health: activeHealth,
			healthMessage: activeHealthMessage,
			renderer: activeRenderer,
			rendererMode: activeRendererMode,
			sessiondAvailable,
			sessiondChecked,
			debugEnabled,
			debugStats,
		});
	});
	const OUTPUT_FLUSH_BUDGET = 128 * 1024;
	const OUTPUT_BACKLOG_LIMIT = 512 * 1024;
	const ACK_BATCH_BYTES = 32 * 1024;
	const ACK_FLUSH_DELAY_MS = 25;
	// Fallback when sessiond doesn't advertise the initial credit.
	const INITIAL_STREAM_CREDIT = 256 * 1024;
	const textEncoder = typeof TextEncoder !== 'undefined' ? new TextEncoder() : null;

	const countBytes = (data: string): number => {
		if (textEncoder) {
			return textEncoder.encode(data).length;
		}
		return data.length;
	};

	const clamp = (value: number, min: number, max: number): number => {
		if (value < min) return min;
		if (value > max) return max;
		return value;
	};

	const resolveCellSize = (
		handle: TerminalHandle,
		terminal: Terminal,
	): { width: number; height: number } => {
		if (handle.kittyOverlay?.cellWidth && handle.kittyOverlay?.cellHeight) {
			return { width: handle.kittyOverlay.cellWidth, height: handle.kittyOverlay.cellHeight };
		}
		if (!handle.container) {
			return { width: 0, height: 0 };
		}
		const rect = handle.container.getBoundingClientRect();
		const cols = Math.max(terminal.cols, 1);
		const rows = Math.max(terminal.rows, 1);
		return { width: rect.width / cols, height: rect.height / rows };
	};

	const handleWheel = (id: string, terminal: Terminal, event: WheelEvent): boolean => {
		const modes = modeMap[id];
		if (statusMap[id] !== 'ready' || startInFlight.has(id) || !startedSessions.has(id)) {
			return true;
		}
		if (!modes?.mouse) {
			return true;
		}
		const handle = terminals.get(id);
		if (!handle?.container) {
			return true;
		}
		const rect = handle.container.getBoundingClientRect();
		const { width, height } = resolveCellSize(handle, terminal);
		if (width <= 0 || height <= 0) {
			return true;
		}
		const col = clamp(Math.floor((event.clientX - rect.left) / width) + 1, 1, terminal.cols);
		const row = clamp(Math.floor((event.clientY - rect.top) / height) + 1, 1, terminal.rows);
		const button = event.deltaY < 0 ? 64 : 65;
		const encoding = (modes.mouseEncoding || (modes.mouseSGR ? 'sgr' : 'x10')) as MouseEncoding;
		sendInput(id, encodeWheel({ button, col, row, encoding }));
		event.preventDefault();
		return false;
	};
	const RESIZE_DEBOUNCE_MS = 100;
	const HEALTH_TIMEOUT_MS = 1200;
	const STARTUP_OUTPUT_TIMEOUT_MS = 2000;
	const RENDER_CHECK_DELAY_MS = 350;
	const RENDER_RECOVERY_DELAY_MS = 150;
	let colorParser: CanvasRenderingContext2D | null = null;

	const getToken = (name: string, fallback: string): string => {
		const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
		return value || fallback;
	};

	const normalizeHex = (value: string): string | null => {
		let hex = value.trim();
		if (!hex) return null;
		if (hex.startsWith('#')) {
			hex = hex.slice(1);
		}
		if (hex.length === 3) {
			hex = hex
				.split('')
				.map((ch) => ch + ch)
				.join('');
		}
		if (hex.length !== 6 || /[^0-9a-fA-F]/.test(hex)) {
			return null;
		}
		return hex.toLowerCase();
	};

	const parseCssColor = (value: string): { r: number; g: number; b: number } | null => {
		if (!value) return null;
		if (!colorParser && typeof document !== 'undefined') {
			const canvas = document.createElement('canvas');
			colorParser = canvas.getContext('2d');
		}
		if (!colorParser) return null;
		colorParser.fillStyle = '#000';
		colorParser.fillStyle = value;
		const normalized = colorParser.fillStyle;
		if (normalized.startsWith('#')) {
			const hex = normalizeHex(normalized);
			if (!hex) return null;
			return {
				r: parseInt(hex.slice(0, 2), 16),
				g: parseInt(hex.slice(2, 4), 16),
				b: parseInt(hex.slice(4, 6), 16),
			};
		}
		const match = normalized.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+)/i);
		if (!match) return null;
		return { r: Number(match[1]), g: Number(match[2]), b: Number(match[3]) };
	};

	const toOscRgb = (value: string | undefined): string | null => {
		if (!value) return null;
		const parsed = parseCssColor(value);
		if (!parsed) return null;
		const to16 = (v: number): string => {
			const clamped = Math.max(0, Math.min(255, v));
			return (clamped * 257).toString(16).padStart(4, '0');
		};
		return `rgb:${to16(parsed.r)}/${to16(parsed.g)}/${to16(parsed.b)}`;
	};

	const themePalette = (theme: ITheme): (string | undefined)[] => [
		theme.black,
		theme.red,
		theme.green,
		theme.yellow,
		theme.blue,
		theme.magenta,
		theme.cyan,
		theme.white,
		theme.brightBlack,
		theme.brightRed,
		theme.brightGreen,
		theme.brightYellow,
		theme.brightBlue,
		theme.brightMagenta,
		theme.brightCyan,
		theme.brightWhite,
	];

	const resolveThemeColor = (value: string | undefined, fallback: string): string | null => {
		return toOscRgb(value ?? fallback);
	};

	const resolveAnsiColor = (terminal: Terminal, index: number): string | null => {
		const theme = terminal.options.theme ?? {};
		if (index < 16) {
			const value = themePalette(theme)[index];
			return toOscRgb(value);
		}
		if (index >= 16 && theme.extendedAnsi && theme.extendedAnsi[index - 16]) {
			return toOscRgb(theme.extendedAnsi[index - 16]);
		}
		return null;
	};

	const loadTerminalDefaults = async (): Promise<void> => {
		if (forceCanvasRendererEnabled) {
			rendererPreference = 'canvas';
			return;
		}
		try {
			const settings = await fetchSettings();
			const renderer = settings.defaults?.terminalRenderer?.trim().toLowerCase();
			if (renderer === 'auto' || renderer === 'webgl' || renderer === 'canvas') {
				rendererPreference = renderer;
			}
		} catch {
			rendererPreference = rendererPreference || 'auto';
		}
	};

	const beginTerminal = async (id: string, quiet = false): Promise<void> => {
		if (!id || startedSessions.has(id) || startInFlight.has(id)) return;
		startInFlight.add(id);
		resetSessionState(id);
		setReplayState(id, 'replaying');
		if (!quiet) {
			statusMap = { ...statusMap, [id]: 'starting' };
			messageMap = { ...messageMap, [id]: 'Waiting for shell output…' };
			setHealth(id, 'unknown');
			inputMap = { ...inputMap, [id]: false };
			scheduleStartupTimeout(id);
		}
		try {
			await StartWorkspaceTerminal(workspaceId, resolveBackendId(id));
			startedSessions.add(id);
			const queued = pendingInput.get(id);
			if (queued) {
				pendingInput.delete(id);
				await WriteWorkspaceTerminal(workspaceId, resolveBackendId(id), queued);
			}
		} catch (error) {
			statusMap = { ...statusMap, [id]: 'error' };
			messageMap = { ...messageMap, [id]: String(error) };
			setHealth(id, 'stale', 'Failed to start terminal.');
			clearStartupTimeout(id);
			pendingInput.delete(id);
			const handle = terminals.get(id);
			handle?.terminal.write(`\r\n[workset] failed to start terminal: ${String(error)}`);
		} finally {
			startInFlight.delete(id);
		}
	};

	const ensureSessionActive = async (id: string, quiet = true): Promise<void> => {
		if (!id) return;
		if (startedSessions.has(id) || startInFlight.has(id)) return;
		const status = statusMap[id];
		if (status === 'closed' || status === 'error' || status === 'idle') {
			try {
				await beginTerminal(id, quiet);
			} catch (error) {
				logDebug(id, 'ensure_session_failed', { error: String(error) });
			}
		}
	};

	const updateStats = (
		id: string,
		updater: (stats: {
			bytesIn: number;
			bytesOut: number;
			backlog: number;
			lastOutputAt: number;
			lastCprAt: number;
		}) => void,
	): void => {
		const existing = statsMap.get(id) ?? {
			bytesIn: 0,
			bytesOut: 0,
			backlog: 0,
			lastOutputAt: 0,
			lastCprAt: 0,
		};
		updater(existing);
		statsMap.set(id, existing);
	};

	const grantInitialCredit = (id: string): void => {
		if (initialCreditSent.has(id) || !workspaceId) return;
		initialCreditSent.add(id);
		const backendId = resolveBackendId(id);
		const credit = initialCreditMap.get(id) ?? INITIAL_STREAM_CREDIT;
		void AckWorkspaceTerminal(workspaceId, backendId, credit).catch(() => {
			initialCreditSent.delete(id);
		});
	};

	const flushAck = (id: string): void => {
		const queued = pendingAckBytes.get(id) ?? 0;
		if (queued <= 0 || !workspaceId) return;
		if (replayState.get(id) !== 'live') return;
		if (!startedSessions.has(id)) return;
		pendingAckBytes.set(id, 0);
		const backendId = resolveBackendId(id);
		void AckWorkspaceTerminal(workspaceId, backendId, queued).catch(() => {
			pendingAckBytes.set(id, (pendingAckBytes.get(id) ?? 0) + queued);
			scheduleAckFlush(id);
		});
	};

	const scheduleAckFlush = (id: string): void => {
		if (ackTimers.has(id)) return;
		const timer = window.setTimeout(() => {
			ackTimers.delete(id);
			flushAck(id);
		}, ACK_FLUSH_DELAY_MS);
		ackTimers.set(id, timer);
	};

	const queueAck = (id: string, bytes: number): void => {
		if (bytes <= 0) return;
		const next = (pendingAckBytes.get(id) ?? 0) + bytes;
		pendingAckBytes.set(id, next);
		if (next >= ACK_BATCH_BYTES) {
			flushAck(id);
			return;
		}
		scheduleAckFlush(id);
	};

	const setHealth = (
		id: string,
		state: 'unknown' | 'checking' | 'ok' | 'stale',
		message = '',
	): void => {
		healthMap = { ...healthMap, [id]: state };
		healthMessageMap = { ...healthMessageMap, [id]: message };
	};

	const clearStartupTimeout = (id: string): void => {
		const timer = startupTimers.get(id);
		if (timer) {
			window.clearTimeout(timer);
			startupTimers.delete(id);
		}
	};

	const scheduleStartupTimeout = (id: string): void => {
		clearStartupTimeout(id);
		const timer = window.setTimeout(() => {
			if (statusMap[id] === 'starting') {
				messageMap = {
					...messageMap,
					[id]: 'Shell has not produced output yet. Check your shell init scripts.',
				};
			}
		}, STARTUP_OUTPUT_TIMEOUT_MS);
		startupTimers.set(id, timer);
	};

	const shouldSuppressMouseInput = (id: string, data: string): boolean => {
		const until = suppressMouseUntil[id];
		if (!until || Date.now() >= until) {
			return false;
		}
		return data.includes('\x1b[<');
	};

	const noteMouseSuppress = (id: string, durationMs: number): void => {
		suppressMouseUntil = { ...suppressMouseUntil, [id]: Date.now() + durationMs };
	};

	const sendInput = (id: string, data: string): void => {
		if (shouldSuppressMouseInput(id, data)) {
			return;
		}
		const modes = modeMap[id] ?? {
			altScreen: false,
			mouse: false,
			mouseSGR: false,
			mouseEncoding: 'x10',
		};
		const mouseResult = stripMouseReports(data, modes, mouseInputTail[id] ?? '');
		if (mouseResult.tail !== (mouseInputTail[id] ?? '')) {
			mouseInputTail = { ...mouseInputTail, [id]: mouseResult.tail };
		}
		const filtered = mouseResult.filtered;
		if (!filtered) {
			return;
		}
		void ensureSessionActive(id);
		if (!startedSessions.has(id)) {
			pendingInput.set(id, (pendingInput.get(id) ?? '') + filtered);
			return;
		}
		updateStats(id, (stats) => {
			stats.bytesOut += filtered.length;
		});
		const backendId = resolveBackendId(id);
		void WriteWorkspaceTerminal(workspaceId, backendId, filtered).catch((error: unknown) => {
			pendingInput.set(id, (pendingInput.get(id) ?? '') + filtered);
			startedSessions.delete(id);
			if (
				typeof error === 'string' &&
				(error.includes('session not found') || error.includes('terminal not started'))
			) {
				resetTerminalInstance(id);
				void beginTerminal(id, true);
			}
			if (error instanceof Error) {
				const message = error.message;
				if (message.includes('session not found') || message.includes('terminal not started')) {
					resetTerminalInstance(id);
					void beginTerminal(id, true);
				}
			}
			const handle = terminals.get(id);
			handle?.terminal.write(`\r\n[workset] write failed: ${String(error)}`);
		});
	};

	const resetSessionState = (id: string): void => {
		replayState.set(id, 'idle');
		pendingReplayOutput.delete(id);
		pendingReplayKitty.delete(id);
		outputQueues.delete(id);
		pendingAckBytes.delete(id);
		initialCreditMap.delete(id);
		const ackTimer = ackTimers.get(id);
		if (ackTimer) {
			window.clearTimeout(ackTimer);
		}
		ackTimers.delete(id);
		initialCreditSent.delete(id);
		pendingHealthCheck.delete(id);
		const renderTimer = pendingRenderCheck.get(id);
		if (renderTimer) {
			window.clearTimeout(renderTimer);
		}
		pendingRenderCheck.delete(id);
		renderStatsMap.delete(id);
		pendingRedraw.delete(id);
		if (mouseInputTail[id]) {
			mouseInputTail = { ...mouseInputTail, [id]: '' };
		}
	};

	const resetTerminalInstance = (id: string): void => {
		const handle = terminals.get(id);
		if (!handle) return;
		handle.terminal.reset();
		handle.terminal.clear();
		handle.terminal.scrollToBottom();
		handle.fitAddon.fit();
		resizeKittyOverlay(handle);
		modeMap = {
			...modeMap,
			[id]: { altScreen: false, mouse: false, mouseSGR: false, mouseEncoding: 'x10' },
		};
		if (mouseInputTail[id]) {
			mouseInputTail = { ...mouseInputTail, [id]: '' };
		}
		noteMouseSuppress(id, 2500);
		void loadRendererAddon(handle, id, rendererModeMap[id] ?? rendererPreference);
	};

	const fitTerminal = (id: string, resizeSession: boolean): void => {
		const handle = terminals.get(id);
		if (!handle) return;
		const buffer = handle.terminal.buffer.active;
		const wasAtBottom = buffer.baseY === buffer.viewportY;
		handle.terminal.options.lineHeight = computeLineHeight(BASE_FONT_SIZE, BASE_LINE_HEIGHT);
		handle.fitAddon.fit();
		resizeKittyOverlay(handle);
		if (wasAtBottom) {
			handle.terminal.scrollToBottom();
		}
		forceRedraw(id);
		if (!resizeSession) return;
		const dims = handle.fitAddon.proposeDimensions();
		if (dims) {
			void ResizeWorkspaceTerminal(workspaceId, resolveBackendId(id), dims.cols, dims.rows).catch(
				() => undefined,
			);
		}
	};

	const scheduleFitStabilization = (id: string, reason: string): void => {
		const existing = fitStabilizers.get(id);
		if (existing) {
			window.clearTimeout(existing);
			fitStabilizers.delete(id);
		}
		let attempts = 0;
		let stableCount = 0;
		const run = (): void => {
			fitStabilizers.delete(id);
			const handle = terminals.get(id);
			if (!handle) return;
			const dims = handle.fitAddon.proposeDimensions();
			if (!dims || dims.cols <= 0 || dims.rows <= 0) {
				attempts += 1;
				if (attempts < 6) {
					fitStabilizers.set(id, window.setTimeout(run, 80 + attempts * 20));
				}
				logDebug(id, 'fit_stabilize_skip', { reason, attempts });
				return;
			}
			const prev = lastDims.get(id);
			if (!prev || prev.cols !== dims.cols || prev.rows !== dims.rows) {
				fitTerminal(id, startedSessions.has(id));
				lastDims.set(id, { cols: dims.cols, rows: dims.rows });
				stableCount = 0;
				logDebug(id, 'fit_stabilize', { reason, cols: dims.cols, rows: dims.rows });
			} else {
				stableCount += 1;
			}
			attempts += 1;
			if (attempts < 6 && stableCount < 2) {
				fitStabilizers.set(id, window.setTimeout(run, 90));
			}
		};
		fitStabilizers.set(id, window.setTimeout(run, 0));
	};

	const scheduleReattachCheck = (id: string, reason: string): void => {
		const existing = reattachTimers.get(id);
		if (existing) {
			window.clearTimeout(existing);
			reattachTimers.delete(id);
		}
		reattachTimers.set(
			id,
			window.setTimeout(() => {
				reattachTimers.delete(id);
				const handle = terminals.get(id);
				if (!handle?.container) return;
				const element = handle.terminal.element;
				if (!element) return;
				const containerRect = handle.container.getBoundingClientRect();
				const elementRect = element.getBoundingClientRect();
				const overflowX = elementRect.width - containerRect.width;
				const overflowY = elementRect.height - containerRect.height;
				if (overflowX <= 2 && overflowY <= 2) {
					return;
				}
				try {
					handle.terminal.open(handle.container);
					handle.fitAddon.fit();
					resizeKittyOverlay(handle);
					forceRedraw(id);
					logDebug(id, 'terminal_reattach', {
						reason,
						overflowX: Math.round(overflowX),
						overflowY: Math.round(overflowY),
					});
				} catch {
					// Best-effort reattach.
				}
			}, 220),
		);
	};

	const attachResizeObserver = (): void => {
		if (!resizeObserver) {
			resizeObserver = new ResizeObserver(() => {
				if (!terminalKey || resizeScheduled) return;
				resizeScheduled = true;
				if (resizeTimer) {
					window.clearTimeout(resizeTimer);
				}
				resizeTimer = window.setTimeout(() => {
					resizeScheduled = false;
					const handle = terminals.get(terminalKey);
					if (!handle) return;
					fitTerminal(terminalKey, false);
					if (!startedSessions.has(terminalKey)) return;
					const dims = handle.fitAddon.proposeDimensions();
					if (dims) {
						const prev = lastDims.get(terminalKey);
						if (!prev || prev.cols !== dims.cols || prev.rows !== dims.rows) {
							lastDims.set(terminalKey, { cols: dims.cols, rows: dims.rows });
							void ResizeWorkspaceTerminal(
								workspaceId,
								resolveBackendId(terminalKey),
								dims.cols,
								dims.rows,
							).catch(() => undefined);
						}
					}
				}, RESIZE_DEBOUNCE_MS);
			});
		}
		if (!terminalContainer) {
			if (observedContainer) {
				resizeObserver.disconnect();
				observedContainer = null;
			}
			return;
		}
		if (observedContainer === terminalContainer) return;
		resizeObserver.disconnect();
		resizeObserver.observe(terminalContainer);
		observedContainer = terminalContainer;
	};

	const forceRedraw = (id: string): void => {
		const handle = terminals.get(id);
		if (!handle) return;
		const rows = handle.terminal.rows;
		if (rows > 0) {
			handle.terminal.refresh(0, rows - 1);
		}
	};

	const setReplayState = (id: string, state: 'idle' | 'replaying' | 'live'): void => {
		replayState.set(id, state);
		if (state !== 'live') {
			return;
		}
		const kittyEvents = pendingReplayKitty.get(id);
		if (kittyEvents && kittyEvents.length > 0) {
			pendingReplayKitty.delete(id);
			for (const event of kittyEvents) {
				void applyKittyEvent(id, event);
			}
		}
		const pending = pendingReplayOutput.get(id);
		if (pending && pending.length > 0) {
			pendingReplayOutput.delete(id);
			for (const chunk of pending) {
				enqueueOutput(id, chunk.data, chunk.bytes);
			}
		}
		flushOutput(id, true);
		forceRedraw(id);
		grantInitialCredit(id);
		flushAck(id);
	};

	const noteCpr = (id: string): void => {
		updateStats(id, (stats) => {
			stats.lastCprAt = Date.now();
		});
		if (healthMap[id] === 'checking' || healthMap[id] === 'unknown') {
			setHealth(id, 'ok', 'CPR received.');
		}
	};

	const captureCpr = (id: string, data: string): boolean => {
		// eslint-disable-next-line no-control-regex
		const matches = data.match(/\x1b\[\??\d+;\d+R/g);
		if (!matches) return false;
		noteCpr(id);
		return true;
	};

	const enqueueOutput = (id: string, data: string, bytes?: number): void => {
		const chunkBytes = bytes && bytes > 0 ? bytes : countBytes(data);
		updateStats(id, (stats) => {
			stats.bytesIn += chunkBytes;
			stats.lastOutputAt = Date.now();
		});
		if (healthMap[id] === 'checking' || healthMap[id] === 'unknown') {
			setHealth(id, 'ok', 'Output received.');
		}
		const queue = outputQueues.get(id) ?? { chunks: [], bytes: 0, scheduled: false };
		queue.chunks.push({ data, bytes: chunkBytes });
		queue.bytes += chunkBytes;
		outputQueues.set(id, queue);
		updateStats(id, (stats) => {
			stats.backlog = queue.bytes;
		});
		if (statusMap[id] === 'starting') {
			statusMap = { ...statusMap, [id]: 'ready' };
			messageMap = { ...messageMap, [id]: '' };
			clearStartupTimeout(id);
		}

		const isActive = id === terminalKey;
		if (queue.bytes > OUTPUT_BACKLOG_LIMIT) {
			let droppedBytes = 0;
			while (queue.bytes > OUTPUT_BACKLOG_LIMIT && queue.chunks.length > 0) {
				const dropped = queue.chunks.shift();
				if (!dropped) break;
				queue.bytes = Math.max(0, queue.bytes - dropped.bytes);
				droppedBytes += dropped.bytes;
			}
			updateStats(id, (stats) => {
				stats.backlog = queue.bytes;
			});
			if (droppedBytes > 0) {
				setHealth(id, 'ok', 'Output throttled; dropping oldest output.');
			}
		}
		if (isActive && queue.bytes >= OUTPUT_FLUSH_BUDGET) {
			flushOutput(id, false);
			return;
		}
		if (!queue.scheduled) {
			queue.scheduled = true;
			requestAnimationFrame(() => {
				queue.scheduled = false;
				flushOutput(id, false);
			});
		}
	};

	const flushOutput = (id: string, force: boolean): void => {
		const queue = outputQueues.get(id);
		if (!queue || queue.bytes === 0) return;
		const handle = terminals.get(id);
		if (!handle) return;
		const isActive = id === terminalKey;
		if (!isActive && !force) return;

		let size = 0;
		let count = 0;
		while (count < queue.chunks.length && size + queue.chunks[count].bytes <= OUTPUT_FLUSH_BUDGET) {
			size += queue.chunks[count].bytes;
			count += 1;
		}
		if (count === 0 && queue.chunks.length > 0) {
			size = queue.chunks[0].bytes;
			count = 1;
		}
		const selected = queue.chunks.slice(0, count);
		const output = selected.map((chunk) => chunk.data).join('');
		queue.chunks = queue.chunks.slice(count);
		queue.bytes = Math.max(0, queue.bytes - size);
		updateStats(id, (stats) => {
			stats.backlog = queue.bytes;
		});
		if (output) {
			const beforeRender = renderStatsMap.get(id) ?? { lastRenderAt: 0, renderCount: 0 };
			if (id === terminalKey && handle.container?.getAttribute('data-active') !== 'true') {
				handle.container?.setAttribute('data-active', 'true');
				handle.fitAddon.fit();
				resizeKittyOverlay(handle);
				nudgeTerminalRedraw(id);
			}
			const scheduleRenderCheck = (): void => {
				if (renderCheckLogged.has(id)) return;
				renderCheckLogged.add(id);
				requestAnimationFrame(() => {
					const afterRender = renderStatsMap.get(id) ?? { lastRenderAt: 0, renderCount: 0 };
					const renderedAfterWrite = afterRender.lastRenderAt > beforeRender.lastRenderAt;
					if (renderedAfterWrite) {
						return;
					}
					const refreshFn = (
						handle.terminal as unknown as { refresh?: (start: number, end: number) => void }
					).refresh;
					const canFallback = rendererMap[id] === 'webgl' && rendererModeMap[id] !== 'webgl';
					if (canFallback) {
						forceCanvasRenderer(id, handle);
					}
					handle.fitAddon.fit();
					nudgeTerminalRedraw(id);
					if (typeof refreshFn === 'function') {
						try {
							const end = Math.max(handle.terminal.rows - 1, 0);
							refreshFn.call(handle.terminal, 0, end);
						} catch {
							// Best-effort refresh.
						}
					}
					const renderService = (
						handle.terminal as unknown as {
							_core?: { _renderService?: { refreshRows?: (start: number, end: number) => void } };
						}
					)._core?._renderService;
					const coreRefresh = renderService?.refreshRows;
					if (typeof coreRefresh === 'function') {
						try {
							const end = Math.max(handle.terminal.rows - 1, 0);
							coreRefresh.call(renderService, 0, end);
						} catch {
							// Best-effort refresh.
						}
					}
					if (handle.terminal.element) {
						const element = handle.terminal.element;
						element.style.transform = 'translateZ(0)';
						requestAnimationFrame(() => {
							element.style.transform = '';
						});
					}
					if (id === terminalKey && handle.container) {
						handle.container.setAttribute('data-active', 'false');
						requestAnimationFrame(() => {
							handle.container.setAttribute('data-active', 'true');
							handle.fitAddon.fit();
							nudgeTerminalRedraw(id);
						});
					}
					if (!reopenAttempted.has(id) && handle.container) {
						reopenAttempted.add(id);
						try {
							handle.terminal.open(handle.container);
							handle.fitAddon.fit();
							resizeKittyOverlay(handle);
							nudgeTerminalRedraw(id);
						} catch {
							// Best-effort re-open.
						}
					}
				});
			};
			handle.terminal.write(output, () => {
				scheduleRenderCheck();
				queueAck(id, size);
			});
			scheduleRenderCheck();
		}
		if (queue.chunks.length === 0) {
			outputQueues.delete(id);
		} else if (isActive) {
			requestAnimationFrame(() => flushOutput(id, false));
		}
	};

	const requestHealthCheck = (id: string): void => {
		const handle = terminals.get(id);
		if (!handle) return;
		setHealth(id, 'checking', 'Waiting for output…');
		const startedAt = Date.now();
		pendingHealthCheck.set(id, startedAt);
		window.setTimeout(() => {
			if (healthMap[id] === 'checking') {
				const stats = statsMap.get(id) ?? {
					bytesIn: 0,
					bytesOut: 0,
					backlog: 0,
					lastOutputAt: 0,
					lastCprAt: 0,
				};
				if (stats.lastOutputAt >= startedAt) {
					setHealth(id, 'ok', 'Output received.');
				} else {
					setHealth(id, 'unknown', 'No output yet.');
				}
			}
			pendingHealthCheck.delete(id);
		}, HEALTH_TIMEOUT_MS);
	};

	const BASE_FONT_SIZE = 12;
	const BASE_LINE_HEIGHT = 1.333;

	const computeLineHeight = (fontSize: number, desired: number): number => {
		const dpr =
			typeof window !== 'undefined' && Number.isFinite(window.devicePixelRatio)
				? window.devicePixelRatio
				: 1;
		const target = Math.round(fontSize * desired * dpr) / dpr;
		return Number((target / fontSize).toFixed(3));
	};

	const logDebug = (id: string, event: string, details?: Record<string, unknown>): void => {
		if (!debugEnabled) return;
		const payload = details ? JSON.stringify(details) : '';
		void logTerminalDebug(workspaceId, id, event, payload);
		const consoleRef = typeof globalThis !== 'undefined' ? globalThis.console : undefined;
		if (consoleRef && typeof consoleRef.debug === 'function') {
			consoleRef.debug(`[workset:terminal] ${event}`, { terminal: id, ...details });
		}
	};

	const noteRender = (id: string): void => {
		const stats = renderStatsMap.get(id) ?? { lastRenderAt: 0, renderCount: 0 };
		stats.lastRenderAt = Date.now();
		stats.renderCount += 1;
		renderStatsMap.set(id, stats);
	};

	const hasVisibleContent = (terminal: Terminal): boolean => {
		const buffer = terminal.buffer.active;
		const rows = terminal.rows;
		for (let i = 0; i < rows; i += 1) {
			const line = buffer.getLine(i);
			if (!line) continue;
			const text = line.translateToString(true);
			if (text.trim().length > 0) {
				return true;
			}
		}
		return false;
	};

	const forceCanvasRenderer = (id: string, handle: TerminalHandle): void => {
		if (handle.webglAddon) {
			try {
				handle.webglAddon.dispose();
			} catch {
				// Ignore disposal failures.
			}
			handle.webglAddon = undefined;
		}
		if (!handle.canvasAddon) {
			try {
				const canvasAddon = new CanvasAddon();
				handle.terminal.loadAddon(canvasAddon);
				handle.canvasAddon = canvasAddon;
			} catch {
				// Best-effort; fall back to DOM renderer.
			}
		}
		rendererMap = { ...rendererMap, [id]: 'canvas' };
		logDebug(id, 'renderer_fallback', {
			mode: rendererModeMap[id] ?? 'auto',
			renderer: 'canvas',
		});
	};

	const nudgeTerminalRedraw = (id: string): void => {
		if (pendingRedraw.has(id)) return;
		const handle = terminals.get(id);
		if (!handle) return;
		const dims = handle.fitAddon.proposeDimensions();
		if (!dims) return;
		const cols = Math.max(2, dims.cols);
		const rows = Math.max(1, dims.rows);
		const nudgeCols = cols + 1;
		pendingRedraw.add(id);
		void ResizeWorkspaceTerminal(workspaceId, resolveBackendId(id), nudgeCols, rows).catch(
			() => undefined,
		);
		logDebug(id, 'redraw_nudge', { cols, rows, nudgeCols });
		window.setTimeout(() => {
			void ResizeWorkspaceTerminal(workspaceId, resolveBackendId(id), cols, rows).catch(
				() => undefined,
			);
			pendingRedraw.delete(id);
		}, 60);
	};

	const scheduleRenderHealthCheck = (id: string, payloadBytes: number): void => {
		if (!id || payloadBytes <= 0 || pendingRenderCheck.has(id)) return;
		const startedAt = Date.now();
		const timer = window.setTimeout(() => {
			pendingRenderCheck.delete(id);
			const handle = terminals.get(id);
			if (!handle) return;
			const stats = renderStatsMap.get(id);
			if (stats && stats.lastRenderAt >= startedAt) {
				return;
			}
			handle.fitAddon.fit();
			window.setTimeout(() => {
				const updated = renderStatsMap.get(id);
				if (updated && updated.lastRenderAt >= startedAt) {
					return;
				}
				if (rendererMap[id] === 'webgl' && rendererModeMap[id] !== 'webgl') {
					forceCanvasRenderer(id, handle);
					handle.fitAddon.fit();
				}
				if (!hasVisibleContent(handle.terminal)) {
					nudgeTerminalRedraw(id);
				}
				logDebug(id, 'render_health_check', {
					rendered: updated ? updated.lastRenderAt >= startedAt : false,
					renderer: rendererMap[id] ?? 'unknown',
				});
			}, RENDER_RECOVERY_DELAY_MS);
		}, RENDER_CHECK_DELAY_MS);
		pendingRenderCheck.set(id, timer);
	};

	const createKittyState = (): KittyState => ({
		images: new Map(),
		placements: new Map(),
	});

	const kittyPlacementKey = (imageId: string, placementId: number): string =>
		`${imageId}:${placementId}`;

	const decodeBase64 = (input: string | number[] | Uint8Array): Uint8Array => {
		if (!input) return new Uint8Array();
		if (input instanceof Uint8Array) {
			return input;
		}
		if (Array.isArray(input)) {
			return Uint8Array.from(input);
		}
		const binary = atob(input);
		const output = new Uint8Array(binary.length);
		for (let i = 0; i < binary.length; i += 1) {
			output[i] = binary.charCodeAt(i);
		}
		return output;
	};

	const decodeKittyImage = async (image: KittyImage): Promise<void> => {
		if (image.bitmap || image.decoding) {
			if (image.decoding) {
				await image.decoding;
			}
			return;
		}
		image.decoding = (async () => {
			try {
				if (image.format === 'png' || image.format === '') {
					const bytes = image.data.slice();
					const blob = new Blob([bytes], { type: 'image/png' });
					image.bitmap = await createImageBitmap(blob);
					return;
				}
				const channels = image.format === 'rgba' ? 4 : 3;
				if (image.width <= 0 || image.height <= 0) {
					return;
				}
				const expected = image.width * image.height * channels;
				if (image.data.length < expected) {
					return;
				}
				const canvas = document.createElement('canvas');
				canvas.width = image.width;
				canvas.height = image.height;
				const ctx = canvas.getContext('2d');
				if (!ctx) return;
				const imageData = ctx.createImageData(image.width, image.height);
				if (channels === 4) {
					imageData.data.set(image.data.subarray(0, expected));
				} else {
					let src = 0;
					for (let i = 0; i < imageData.data.length; i += 4) {
						imageData.data[i] = image.data[src];
						imageData.data[i + 1] = image.data[src + 1];
						imageData.data[i + 2] = image.data[src + 2];
						imageData.data[i + 3] = 255;
						src += 3;
					}
				}
				ctx.putImageData(imageData, 0, 0);
				image.bitmap = await createImageBitmap(canvas);
			} catch {
				// Best-effort; keep rendering text if bitmap fails to decode.
			}
		})();
		await image.decoding;
	};

	const ensureKittyOverlay = (handle: TerminalHandle, id: string): void => {
		if (handle.kittyOverlay || !handle.container) return;
		const underlay = document.createElement('canvas');
		underlay.className = 'kitty-layer kitty-underlay';
		const overlay = document.createElement('canvas');
		overlay.className = 'kitty-layer kitty-overlay';
		handle.container.appendChild(underlay);
		handle.container.appendChild(overlay);
		const ctxUnder = underlay.getContext('2d');
		const ctxOver = overlay.getContext('2d');
		if (!ctxUnder || !ctxOver) return;
		const kittyOverlay: KittyOverlay = {
			underlay,
			overlay,
			ctxUnder,
			ctxOver,
			cellWidth: 0,
			cellHeight: 0,
			dpr: window.devicePixelRatio || 1,
			renderScheduled: false,
		};
		handle.kittyOverlay = kittyOverlay;
		resizeKittyOverlay(handle);
		scheduleKittyRender(id);
		const disposables: { dispose: () => void }[] = [];
		disposables.push(
			handle.terminal.onRender(() => {
				scheduleKittyRender(id);
			}),
		);
		disposables.push(
			handle.terminal.onScroll(() => {
				scheduleKittyRender(id);
			}),
		);
		disposables.push(
			handle.terminal.onWriteParsed(() => {
				scheduleKittyRender(id);
			}),
		);
		disposables.push(
			handle.terminal.onResize(() => {
				resizeKittyOverlay(handle);
				scheduleKittyRender(id);
			}),
		);
		const maybeDimensions = (
			handle.terminal as unknown as {
				onDimensionsChange?: (
					fn: (dimensions: { css: { cell: { width: number; height: number } } }) => void,
				) => {
					dispose: () => void;
				};
			}
		).onDimensionsChange;
		if (maybeDimensions) {
			disposables.push(
				maybeDimensions((dimensions) => {
					if (!handle.kittyOverlay) return;
					handle.kittyOverlay.cellWidth = dimensions.css.cell.width;
					handle.kittyOverlay.cellHeight = dimensions.css.cell.height;
					scheduleKittyRender(id);
				}),
			);
		}
		handle.kittyDisposables = disposables;
	};

	const resizeKittyOverlay = (handle: TerminalHandle): void => {
		if (!handle.kittyOverlay || !handle.container) return;
		const rect = handle.container.getBoundingClientRect();
		const dpr = window.devicePixelRatio || 1;
		handle.kittyOverlay.dpr = dpr;
		for (const canvas of [handle.kittyOverlay.underlay, handle.kittyOverlay.overlay]) {
			canvas.width = Math.max(1, Math.floor(rect.width * dpr));
			canvas.height = Math.max(1, Math.floor(rect.height * dpr));
			canvas.style.width = `${rect.width}px`;
			canvas.style.height = `${rect.height}px`;
		}
		handle.kittyOverlay.ctxUnder.setTransform(dpr, 0, 0, dpr, 0, 0);
		handle.kittyOverlay.ctxOver.setTransform(dpr, 0, 0, dpr, 0, 0);
	};

	const scheduleKittyRender = (id: string): void => {
		const handle = terminals.get(id);
		if (!handle || !handle.kittyOverlay) return;
		if (handle.kittyOverlay.renderScheduled) return;
		handle.kittyOverlay.renderScheduled = true;
		requestAnimationFrame(() => {
			if (!handle.kittyOverlay) return;
			handle.kittyOverlay.renderScheduled = false;
			renderKitty(id);
		});
	};

	const renderKitty = (id: string): void => {
		const handle = terminals.get(id);
		if (!handle || !handle.kittyOverlay) return;
		const overlay = handle.kittyOverlay;
		const state = handle.kittyState;
		const width = overlay.overlay.width / overlay.dpr;
		const height = overlay.overlay.height / overlay.dpr;
		overlay.ctxUnder.clearRect(0, 0, width, height);
		overlay.ctxOver.clearRect(0, 0, width, height);
		if (state.images.size === 0 || state.placements.size === 0) return;
		const cellWidth = overlay.cellWidth || width / handle.terminal.cols;
		const cellHeight = overlay.cellHeight || height / handle.terminal.rows;
		if (
			!Number.isFinite(cellWidth) ||
			!Number.isFinite(cellHeight) ||
			cellWidth <= 0 ||
			cellHeight <= 0
		)
			return;
		const placements = Array.from(state.placements.values()).sort((a, b) => a.z - b.z);
		for (const placement of placements) {
			const image = state.images.get(placement.imageId);
			if (!image || !image.bitmap) {
				continue;
			}
			const x = placement.col * cellWidth + placement.x;
			const y = placement.row * cellHeight + placement.y;
			const imageWidth = image.width || image.bitmap.width;
			const imageHeight = image.height || image.bitmap.height;
			const drawWidth = placement.cols > 0 ? placement.cols * cellWidth : imageWidth;
			const drawHeight = placement.rows > 0 ? placement.rows * cellHeight : imageHeight;
			if (drawWidth <= 0 || drawHeight <= 0) continue;
			const ctx = placement.z < 0 ? overlay.ctxUnder : overlay.ctxOver;
			ctx.drawImage(image.bitmap, x, y, drawWidth, drawHeight);
		}
	};

	const applyKittyEvent = async (id: string, event: KittyEventPayload): Promise<void> => {
		const handle = terminals.get(id);
		if (!handle) return;
		const state = handle.kittyState;
		if (event.kind === 'snapshot' && event.snapshot) {
			state.images.clear();
			state.placements.clear();
			for (const image of event.snapshot.images ?? []) {
				if (!image?.id) continue;
				const kittyImage: KittyImage = {
					id: image.id,
					format: image.format ?? 'png',
					width: image.width ?? 0,
					height: image.height ?? 0,
					data: decodeBase64(image.data ?? ''),
				};
				state.images.set(kittyImage.id, kittyImage);
				await decodeKittyImage(kittyImage);
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
					z: placement.z ?? 0,
				};
				state.placements.set(
					kittyPlacementKey(kittyPlacement.imageId, kittyPlacement.id),
					kittyPlacement,
				);
			}
			scheduleKittyRender(id);
			return;
		}
		if (event.kind === 'delete' && event.delete) {
			if (event.delete.all) {
				state.images.clear();
				state.placements.clear();
			} else if (event.delete.placementId && event.delete.imageId) {
				state.placements.delete(kittyPlacementKey(event.delete.imageId, event.delete.placementId));
			} else if (event.delete.imageId) {
				state.images.delete(event.delete.imageId);
				for (const [key, placement] of state.placements.entries()) {
					if (placement.imageId === event.delete.imageId) {
						state.placements.delete(key);
					}
				}
			}
			scheduleKittyRender(id);
			return;
		}
		if (event.kind === 'image' && event.image) {
			const kittyImage: KittyImage = {
				id: event.image.id,
				format: event.image.format ?? 'png',
				width: event.image.width ?? 0,
				height: event.image.height ?? 0,
				data: decodeBase64(event.image.data ?? ''),
			};
			state.images.set(kittyImage.id, kittyImage);
			await decodeKittyImage(kittyImage);
			scheduleKittyRender(id);
			return;
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
				z: event.placement.z ?? 0,
			};
			state.placements.set(
				kittyPlacementKey(kittyPlacement.imageId, kittyPlacement.id),
				kittyPlacement,
			);
			scheduleKittyRender(id);
		}
	};

	const loadRendererAddon = async (
		handle: TerminalHandle,
		id: string,
		mode: 'auto' | 'webgl' | 'canvas',
	): Promise<void> => {
		const resolvedMode = forceCanvasRendererEnabled ? 'canvas' : mode;
		rendererModeMap = { ...rendererModeMap, [id]: resolvedMode };
		if (resolvedMode === 'canvas') {
			const options = handle.terminal.options as {
				allowProposedApi?: boolean;
				rendererType?: string;
			};
			options.allowProposedApi = true;
			options.rendererType = 'canvas';
			forceCanvasRenderer(id, handle);
			return;
		}
		if (handle.canvasAddon) {
			try {
				handle.canvasAddon.dispose();
			} catch {
				// Best-effort cleanup.
			}
			handle.canvasAddon = undefined;
		}
		if (handle.webglAddon) {
			try {
				handle.webglAddon.dispose();
			} catch {
				// Best-effort cleanup.
			}
			handle.webglAddon = undefined;
		}
		try {
			if (typeof document !== 'undefined' && document.fonts?.ready) {
				await document.fonts.ready;
			}
		} catch {
			// Font readiness is best-effort; continue if unavailable.
		}

		try {
			const webglAddon = new WebglAddon();
			webglAddon.onContextLoss(() => {
				webglAddon.dispose();
				rendererMap = { ...rendererMap, [id]: 'canvas' };
			});
			handle.terminal.loadAddon(webglAddon);
			handle.webglAddon = webglAddon;
			rendererMap = { ...rendererMap, [id]: 'webgl' };
			logDebug(id, 'renderer_loaded', { mode, renderer: 'webgl' });
		} catch {
			rendererMap = { ...rendererMap, [id]: 'canvas' };
			logDebug(id, 'renderer_loaded', { mode, renderer: 'canvas' });
			// Canvas renderer remains as default.
		}
	};

	const registerOscHandlers = (id: string, terminal: Terminal): { dispose: () => void }[] => {
		const disposables: { dispose: () => void }[] = [];
		const respond = (payload: string): void => {
			sendInput(id, `\x1b]${payload}\x07`);
		};
		disposables.push(
			terminal.parser.registerOscHandler(10, (data) => {
				if (data !== '?') return false;
				const rgb = resolveThemeColor(
					terminal.options.theme?.foreground,
					getToken('--text', '#eef3f9'),
				);
				if (!rgb) return false;
				respond(`10;${rgb}`);
				return true;
			}),
		);
		disposables.push(
			terminal.parser.registerOscHandler(11, (data) => {
				if (data !== '?') return false;
				const rgb = resolveThemeColor(
					terminal.options.theme?.background,
					getToken('--panel-strong', '#111c29'),
				);
				if (!rgb) return false;
				respond(`11;${rgb}`);
				return true;
			}),
		);
		disposables.push(
			terminal.parser.registerOscHandler(12, (data) => {
				if (data !== '?') return false;
				const rgb = resolveThemeColor(
					terminal.options.theme?.cursor,
					getToken('--accent', '#2d8cff'),
				);
				if (!rgb) return false;
				respond(`12;${rgb}`);
				return true;
			}),
		);
		disposables.push(
			terminal.parser.registerOscHandler(4, (data) => {
				const parts = data.split(';');
				if (parts.length < 2 || parts.length % 2 !== 0) return false;
				const responses: string[] = [];
				for (let i = 0; i < parts.length; i += 2) {
					const index = Number.parseInt(parts[i], 10);
					const query = parts[i + 1];
					if (!Number.isFinite(index) || query !== '?') {
						return false;
					}
					const rgb = resolveAnsiColor(terminal, index);
					if (!rgb) return false;
					responses.push(`4;${index};${rgb}`);
				}
				if (responses.length === 0) return false;
				for (const response of responses) {
					respond(response);
				}
				return true;
			}),
		);
		return disposables;
	};

	const createTerminal = (): Terminal => {
		const themeBackground = getToken('--panel-strong', '#111c29');
		const themeForeground = getToken('--text', '#eef3f9');
		const themeCursor = getToken('--accent', '#2d8cff');
		const themeSelection = getToken('--accent', '#2d8cff');
		const fontMono = getToken('--font-mono', '"JetBrains Mono", Menlo, Consolas, monospace');

		const options: ITerminalOptions & ITerminalInitOnlyOptions & { rendererType?: string } = {
			fontFamily: fontMono,
			fontSize: BASE_FONT_SIZE,
			// Keep fontSize * lineHeight * dpr an integer to avoid subpixel row artifacts.
			lineHeight: computeLineHeight(BASE_FONT_SIZE, BASE_LINE_HEIGHT),
			cursorBlink: true,
			scrollback: 4000,
			allowProposedApi: true,
			rendererType: 'canvas',
			theme: {
				background: themeBackground,
				foreground: themeForeground,
				cursor: themeCursor,
				selectionBackground: themeSelection,
			},
		};
		return new Terminal(options);
	};

	const attachTerminal = (id: string, _name: string): TerminalHandle => {
		let handle = terminals.get(id);
		if (!handle) {
			const terminal = createTerminal();
			const fitAddon = new FitAddon();
			terminal.loadAddon(fitAddon);
			terminal.attachCustomKeyEventHandler((event) => {
				if (event.key === 'Enter' && event.shiftKey) {
					inputMap = { ...inputMap, [id]: true };
					void beginTerminal(id);
					// Codex CLI expects Ctrl+J (LF) for newline-in-input.
					sendInput(id, '\x0a');
					return false;
				}
				return true;
			});
			terminal.attachCustomWheelEventHandler((event) => handleWheel(id, terminal, event));
			const dataDisposable = terminal.onData((data) => {
				inputMap = { ...inputMap, [id]: true };
				void beginTerminal(id);
				captureCpr(id, data);
				sendInput(id, data);
			});
			const binaryDisposable = terminal.onBinary((data) => {
				if (!data) return;
				inputMap = { ...inputMap, [id]: true };
				void beginTerminal(id);
				sendInput(id, data);
			});
			terminal.onRender(() => {
				noteRender(id);
			});
			const container = document.createElement('div');
			container.className = 'terminal-instance';
			handle = {
				terminal,
				fitAddon,
				dataDisposable,
				binaryDisposable,
				container,
				kittyState: createKittyState(),
			};
			handle.oscDisposables = registerOscHandlers(id, terminal);
			terminals.set(id, handle);
			if (!modeMap[id]) {
				modeMap = {
					...modeMap,
					[id]: { altScreen: false, mouse: false, mouseSGR: false, mouseEncoding: 'x10' },
				};
			}
		}
		if (terminalContainer) {
			if (terminalContainer.firstChild !== handle.container) {
				terminalContainer.replaceChildren(handle.container);
			}
			const terminalElement = handle.terminal.element;
			const needsOpen = !terminalElement || terminalElement.parentElement !== handle.container;
			if (needsOpen) {
				handle.container.replaceChildren();
				handle.terminal.open(handle.container);
				ensureKittyOverlay(handle, id);
				void loadRendererAddon(handle, id, rendererPreference);
				if (typeof document !== 'undefined' && document.fonts?.ready) {
					document.fonts.ready
						.then(() => {
							const current = terminals.get(id);
							if (!current) return;
							current.terminal.options.lineHeight = computeLineHeight(
								BASE_FONT_SIZE,
								BASE_LINE_HEIGHT,
							);
							current.fitAddon.fit();
							resizeKittyOverlay(current);
							const updated = current.fitAddon.proposeDimensions();
							if (updated) {
								void ResizeWorkspaceTerminal(
									workspaceId,
									resolveBackendId(id),
									updated.cols,
									updated.rows,
								).catch(() => undefined);
							}
						})
						.catch(() => undefined);
				}
				scheduleFitStabilization(id, 'open');
			}
			handle.container.setAttribute('data-active', 'true');
			handle.fitAddon.fit();
			resizeKittyOverlay(handle);
			const dims = handle.fitAddon.proposeDimensions();
			if (dims) {
				void ResizeWorkspaceTerminal(workspaceId, resolveBackendId(id), dims.cols, dims.rows).catch(
					() => undefined,
				);
			}
			if (active) {
				handle.terminal.focus();
			}
			flushOutput(id, false);
		}
		return handle;
	};

	const ensureListener = (): void => {
		if (!listeners.has('terminal:data')) {
			const handler = (payload: {
				workspaceId: string;
				terminalId: string;
				data: string;
				bytes?: number;
			}): void => {
				const id = resolvePayloadId(payload);
				if (!id) return;
				const handle = terminals.get(id);
				if (!handle) return;
				if (!inputMap[id]) {
					inputMap = { ...inputMap, [id]: true };
				}
				const bytes = payload.bytes && payload.bytes > 0 ? payload.bytes : countBytes(payload.data);
				const replayStateValue = replayState.get(id) ?? 'unknown';
				const isLive = replayStateValue === 'live';
				if (!isLive) {
					const pending = pendingReplayOutput.get(id) ?? [];
					pending.push({ data: payload.data, bytes });
					pendingReplayOutput.set(id, pending);
					return;
				}
				enqueueOutput(id, payload.data, bytes);
			};
			unsubscribeHandlers.push(subscribeEvent('terminal:data', handler));
			listeners.add('terminal:data');
		}
		if (!listeners.has('terminal:kitty')) {
			const handler = (payload: {
				workspaceId: string;
				terminalId: string;
				event: KittyEventPayload;
			}): void => {
				const id = resolvePayloadId(payload);
				if (!id) return;
				const handle = terminals.get(id);
				if (!handle) return;
				if (!inputMap[id]) {
					inputMap = { ...inputMap, [id]: true };
				}
				if (replayState.get(id) !== 'live') {
					const pending = pendingReplayKitty.get(id) ?? [];
					pending.push(payload.event);
					pendingReplayKitty.set(id, pending);
					return;
				}
				void applyKittyEvent(id, payload.event);
			};
			unsubscribeHandlers.push(subscribeEvent('terminal:kitty', handler));
			listeners.add('terminal:kitty');
		}
		if (!listeners.has('terminal:bootstrap')) {
			const handler = (payload: {
				workspaceId: string;
				terminalId: string;
				snapshotSource?: string;
				backlogSource?: string;
				backlogTruncated?: boolean;
				nextOffset?: number;
				altScreen?: boolean;
				mouse?: boolean;
				mouseSGR?: boolean;
				mouseEncoding?: string;
				safeToReplay?: boolean;
				initialCredit?: number;
			}): void => {
				const id = resolvePayloadId(payload);
				if (!id) return;
				setReplayState(id, 'replaying');
				logDebug(id, 'bootstrap', {
					snapshotSource: payload.snapshotSource ?? '',
					backlogSource: payload.backlogSource ?? '',
					truncated: payload.backlogTruncated ?? false,
					altScreen: payload.altScreen ?? false,
					mouse: payload.mouse ?? false,
					mouseSGR: payload.mouseSGR ?? false,
					mouseEncoding: payload.mouseEncoding ?? '',
					safeToReplay: payload.safeToReplay ?? false,
				});
				initialCreditMap.set(id, payload.initialCredit ?? INITIAL_STREAM_CREDIT);
				modeMap = {
					...modeMap,
					[id]: {
						altScreen: payload.altScreen ?? false,
						mouse: payload.mouse ?? false,
						mouseSGR: payload.mouseSGR ?? false,
						mouseEncoding: payload.mouseEncoding ?? (payload.mouseSGR ? 'sgr' : 'x10'),
					},
				};
				if (payload.backlogTruncated) {
					setHealth(id, 'ok', 'Backlog truncated; showing latest output.');
				}
			};
			unsubscribeHandlers.push(subscribeEvent('terminal:bootstrap', handler));
			listeners.add('terminal:bootstrap');
		}
		if (!listeners.has('terminal:bootstrap_done')) {
			const handler = (payload: { workspaceId: string; terminalId: string }): void => {
				const id = resolvePayloadId(payload);
				if (!id) return;
				const pending = pendingReplayOutput.get(id) ?? [];
				const replayBytes = pending.reduce((sum, chunk) => sum + chunk.bytes, 0);
				if (!inputMap[id]) {
					inputMap = { ...inputMap, [id]: true };
				}
				if (statusMap[id] !== 'ready') {
					statusMap = { ...statusMap, [id]: 'ready' };
					messageMap = { ...messageMap, [id]: '' };
				}
				scheduleRenderHealthCheck(id, replayBytes);
				setReplayState(id, 'live');
			};
			unsubscribeHandlers.push(subscribeEvent('terminal:bootstrap_done', handler));
			listeners.add('terminal:bootstrap_done');
		}
		if (!listeners.has('terminal:lifecycle')) {
			const handler = (payload: {
				workspaceId: string;
				terminalId: string;
				status: 'started' | 'closed' | 'error' | 'idle';
				message?: string;
			}): void => {
				const id = resolvePayloadId(payload);
				if (!id) return;
				if (payload.status === 'started') {
					const message = payload.message?.toLowerCase() ?? '';
					const isResume =
						message.includes('backlog truncated') || message.includes('session resumed');
					startedSessions.add(id);
					statusMap = {
						...statusMap,
						[id]: isResume ? 'ready' : 'starting',
					};
					messageMap = {
						...messageMap,
						[id]: isResume ? (payload.message ?? '') : 'Waiting for shell output…',
					};
					inputMap = { ...inputMap, [id]: isResume };
					if (isResume) {
						clearStartupTimeout(id);
						setHealth(id, 'ok', 'Session resumed (TUI state not replayed).');
					} else {
						scheduleStartupTimeout(id);
						setHealth(id, 'unknown');
					}
					return;
				}
				if (payload.status === 'closed') {
					startedSessions.delete(id);
					statusMap = { ...statusMap, [id]: 'closed' };
					setHealth(id, 'stale', 'Terminal closed.');
					clearStartupTimeout(id);
					resetSessionState(id);
					return;
				}
				if (payload.status === 'idle') {
					startedSessions.delete(id);
					statusMap = { ...statusMap, [id]: 'idle' };
					setHealth(id, 'stale', 'Terminal idle.');
					clearStartupTimeout(id);
					resetSessionState(id);
					return;
				}
				if (payload.status === 'error') {
					startedSessions.delete(id);
					statusMap = { ...statusMap, [id]: 'error' };
					messageMap = {
						...messageMap,
						[id]: payload.message ?? 'Terminal error',
					};
					const term = terminals.get(id);
					if (term && payload.message) {
						term.terminal.write(`\r\n[workset] ${payload.message}`);
					}
					setHealth(id, 'stale', payload.message ?? 'Terminal error.');
					clearStartupTimeout(id);
					resetSessionState(id);
				}
			};
			unsubscribeHandlers.push(subscribeEvent('terminal:lifecycle', handler));
			listeners.add('terminal:lifecycle');
		}
		if (!listeners.has('terminal:modes')) {
			const handler = (payload: {
				workspaceId: string;
				terminalId: string;
				altScreen: boolean;
				mouse: boolean;
				mouseSGR: boolean;
				mouseEncoding?: string;
			}): void => {
				const id = resolvePayloadId(payload);
				if (!id) return;
				modeMap = {
					...modeMap,
					[id]: {
						altScreen: payload.altScreen,
						mouse: payload.mouse,
						mouseSGR: payload.mouseSGR,
						mouseEncoding: payload.mouseEncoding ?? (payload.mouseSGR ? 'sgr' : 'x10'),
					},
				};
			};
			unsubscribeHandlers.push(subscribeEvent('terminal:modes', handler));
			listeners.add('terminal:modes');
		}
		if (!listeners.has('sessiond:restarted')) {
			const handler = (): void => {
				sessiondChecked = false;
				void (async () => {
					if (terminalKey) {
						statusMap = { ...statusMap, [terminalKey]: 'starting' };
						messageMap = {
							...messageMap,
							[terminalKey]: 'Session daemon restarted. Reconnecting…',
						};
						setHealth(terminalKey, 'checking', 'Reconnecting after daemon restart.');
					}
					await refreshSessiondStatus();
					if (!terminalKey || sessiondAvailable !== true) return;
					startedSessions.delete(terminalKey);
					startInFlight.delete(terminalKey);
					resetTerminalInstance(terminalKey);
					resetSessionState(terminalKey);
					noteMouseSuppress(terminalKey, 4000);
					void beginTerminal(terminalKey, true);
				})();
			};
			unsubscribeHandlers.push(subscribeEvent('sessiond:restarted', handler));
			listeners.add('sessiond:restarted');
		}
	};

	const cleanupListeners = (): void => {
		for (const unsubscribe of unsubscribeHandlers.splice(0)) {
			unsubscribe();
		}
		listeners.clear();
	};

	const initTerminal = async (id: string, name: string): Promise<void> => {
		if (!id) return;
		const token = ++initCounter;
		ensureListener();
		if (!sessiondChecked) {
			await refreshSessiondStatus();
		}
		attachTerminal(id, name);
		let resumed = false;
		if (sessiondAvailable === true) {
			try {
				const status = await fetchWorkspaceTerminalStatus(workspaceId, resolveBackendId(id));
				resumed = status?.active ?? false;
			} catch {
				resumed = false;
			}
		}
		if (resumed) {
			await beginTerminal(id, true);
			inputMap = { ...inputMap, [id]: true };
			statusMap = { ...statusMap, [id]: 'ready' };
			messageMap = { ...messageMap, [id]: '' };
			setHealth(id, 'ok', 'Session resumed.');
			if (!rendererMap[id]) {
				rendererMap = { ...rendererMap, [id]: 'unknown' };
			}
			if (!rendererModeMap[id]) {
				rendererModeMap = { ...rendererModeMap, [id]: rendererPreference };
			}
			return;
		}
		if (token !== initCounter) return;
		pendingHealthCheck.delete(id);
		if (!startedSessions.has(id) && !startInFlight.has(id)) {
			statusMap = { ...statusMap, [id]: 'standby' };
			messageMap = { ...messageMap, [id]: '' };
			setHealth(id, 'unknown');
			if (!rendererMap[id]) {
				rendererMap = { ...rendererMap, [id]: 'unknown' };
			}
			if (!rendererModeMap[id]) {
				rendererModeMap = { ...rendererModeMap, [id]: rendererPreference };
			}
			inputMap = { ...inputMap, [id]: false };
		}
	};

	const restartTerminal = async (): Promise<void> => {
		if (!workspaceId || !terminalKey) return;
		await beginTerminal(terminalKey);
	};

	const refreshSessiondStatus = async (): Promise<void> => {
		try {
			const status = await fetchSessiondStatus();
			sessiondAvailable = status?.available ?? false;
		} catch {
			sessiondAvailable = false;
		} finally {
			sessiondChecked = true;
		}
	};

	export async function restart(): Promise<void> {
		await restartTerminal();
	}

	export function retryHealthCheck(): void {
		if (!terminalKey) return;
		requestHealthCheck(terminalKey);
	}

	onMount(() => {
		debugEnabled =
			typeof localStorage !== 'undefined' && localStorage.getItem('worksetTerminalDebug') === '1';
		void Environment()
			.then((env: { buildType?: string; platform?: string } | null) => {
				if (env?.buildType === 'production' && env?.platform === 'darwin') {
					forceCanvasRendererEnabled = true;
					rendererPreference = 'canvas';
					for (const [id, handle] of terminals.entries()) {
						const options = handle.terminal.options as {
							allowProposedApi?: boolean;
							rendererType?: string;
						};
						options.allowProposedApi = true;
						options.rendererType = 'canvas';
						void loadRendererAddon(handle, id, 'canvas');
						handle.fitAddon.fit();
						resizeKittyOverlay(handle);
						nudgeTerminalRedraw(id);
					}
				}
			})
			.catch(() => undefined);
		void loadTerminalDefaults();
		void refreshSessiondStatus();
		attachResizeObserver();

		if (debugEnabled) {
			debugInterval = window.setInterval(() => {
				if (!terminalKey) return;
				const stats = statsMap.get(terminalKey) ?? {
					bytesIn: 0,
					bytesOut: 0,
					backlog: 0,
					lastOutputAt: 0,
					lastCprAt: 0,
				};
				debugStats = { ...stats };
			}, 1000);
		}

		const focusHandler = (): void => {
			if (!terminalKey) return;
			void ensureSessionActive(terminalKey);
		};
		window.addEventListener('focus', focusHandler);

		return () => {
			window.removeEventListener('focus', focusHandler);
		};
	});

	$effect(() => {
		if (!terminalKey || !terminalContainer) return;
		const id = terminalKey;
		const name = workspaceName;
		untrack(() => {
			void initTerminal(id, name);
		});
		untrack(() => {
			void ensureSessionActive(id);
		});
	});

	$effect(() => {
		if (!terminalKey || !terminalContainer) return;
		if (lastWorkspaceId === workspaceId) return;
		lastWorkspaceId = workspaceId;
		scheduleFitStabilization(terminalKey, 'workspace_switch');
		scheduleReattachCheck(terminalKey, 'workspace_switch');
	});

	$effect(() => {
		if (!terminalKey) return;
		attachResizeObserver();
		if (terminalContainer) {
			fitTerminal(terminalKey, startedSessions.has(terminalKey));
		}
	});

	$effect(() => {
		if (!terminalKey || !terminalContainer) return;
		if (!active) return;
		const id = terminalKey;
		let cancelled = false;
		requestAnimationFrame(() => {
			requestAnimationFrame(() => {
				if (cancelled) return;
				fitTerminal(id, startedSessions.has(id));
				forceRedraw(id);
				const handle = terminals.get(id);
				if (handle && !hasVisibleContent(handle.terminal)) {
					nudgeTerminalRedraw(id);
				}
			});
		});
		const settleTimer = window.setTimeout(() => {
			if (cancelled) return;
			const handle = terminals.get(id);
			const dims = handle?.fitAddon.proposeDimensions();
			const prev = dims ? lastDims.get(id) : null;
			if (!dims || !prev || prev.cols !== dims.cols || prev.rows !== dims.rows) {
				fitTerminal(id, startedSessions.has(id));
				if (dims) {
					lastDims.set(id, { cols: dims.cols, rows: dims.rows });
				}
			}
			forceRedraw(id);
		}, 140);
		scheduleFitStabilization(id, 'activate');
		const handle = terminals.get(id);
		if (handle) {
			handle.terminal.focus();
		}
		return () => {
			cancelled = true;
			window.clearTimeout(settleTimer);
		};
	});

	$effect(() => {
		if (!terminalKey || !terminalContainer) return;
		if (startedSessions.has(terminalKey) || startInFlight.has(terminalKey)) return;
		if (statusMap[terminalKey] === 'standby') {
			void beginTerminal(terminalKey, !active);
		}
	});

	onDestroy(() => {
		resizeObserver?.disconnect();
		if (resizeTimer) {
			window.clearTimeout(resizeTimer);
		}
		for (const timer of fitStabilizers.values()) {
			window.clearTimeout(timer);
		}
		fitStabilizers.clear();
		for (const timer of reattachTimers.values()) {
			window.clearTimeout(timer);
		}
		reattachTimers.clear();
		for (const timer of startupTimers.values()) {
			window.clearTimeout(timer);
		}
		startupTimers.clear();
		if (debugInterval) {
			window.clearInterval(debugInterval);
		}
		cleanupListeners();
	});
</script>
