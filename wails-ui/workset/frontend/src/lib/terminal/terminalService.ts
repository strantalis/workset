import { Terminal, type ITheme } from '@xterm/xterm';
import { ClipboardAddon } from '@xterm/addon-clipboard';
import { FitAddon } from '@xterm/addon-fit';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import type { Readable, Writable } from 'svelte/store';
import { terminalTransport } from './terminalTransport';
import { createTerminalInstance } from './terminalRenderer';
import { createTerminalWebLinksSync } from './terminalWebLinks';
import { TerminalStateStore } from './terminalStateStore';
import { stripMouseReports } from './inputFilter';
import { createTerminalLifecycle } from './terminalLifecycle';
import { captureViewportSnapshot, resolveViewportTargetLine } from './viewport';
import { createTerminalStreamOrchestrator } from './terminalStreamOrchestrator';
import { createTerminalResizeBridge } from './terminalResizeBridge';
import { createTerminalRenderHealth, hasVisibleTerminalContent } from './terminalRenderHealth';
import {
	createTerminalAttachState,
	createTerminalRendererAddonState,
} from './terminalAttachRendererState';
import { createTerminalAttachOpenLifecycle } from './terminalAttachOpenLifecycle';
import {
	createTerminalEventSubscriptions,
	type TerminalKittyPayload,
	type TerminalPayload,
} from './terminalEventSubscriptions';
import { createTerminalModeBootstrapCoordinator } from './terminalModeBootstrapCoordinator';
import { createTerminalReplayAckOrchestrator } from './terminalReplayAckOrchestrator';
import {
	createTerminalInstanceManager,
	type TerminalInstanceHandle,
} from './terminalInstanceManager';
import {
	createKittyState,
	createTerminalKittyController,
	type KittyEventPayload,
	type KittyOverlay,
	type KittyState,
} from './terminalKittyImageController';
import { createTerminalInputOrchestrator } from './terminalInputOrchestrator';

export type TerminalViewState = {
	status: string;
	message: string;
	health: 'unknown' | 'checking' | 'ok' | 'stale';
	healthMessage: string;
	renderer: 'unknown' | 'webgl';
	rendererMode: 'webgl';
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
};

type TerminalContext = {
	terminalKey: string;
	workspaceId: string;
	workspaceName: string;
	terminalId: string;
	container: HTMLDivElement | null;
	active: boolean;
	lastWorkspaceId: string;
};

type TerminalKey = string;
type ClipboardSelection = string;

type OutputChunk = {
	data: string;
	bytes: number;
};

type TerminalHandle = TerminalInstanceHandle<KittyState> & {
	kittyOverlay?: KittyOverlay;
};

const terminalHandles = new Map<string, TerminalHandle>();
const terminalContexts = new Map<string, TerminalContext>();
const terminalStores = new TerminalStateStore<TerminalViewState>();
const DISPOSE_TTL_MS = 10 * 60 * 1000;

// Font size configuration
const DEFAULT_FONT_SIZE = 13;
const MIN_FONT_SIZE = 8;
const MAX_FONT_SIZE = 28;
const FONT_SIZE_STEP = 1;
const FONT_SIZE_STORAGE_KEY = 'worksetTerminalFontSize';

const clampFontSize = (value: number): number =>
	Math.min(MAX_FONT_SIZE, Math.max(MIN_FONT_SIZE, value));

const loadInitialFontSize = (): number => {
	if (typeof localStorage === 'undefined') return DEFAULT_FONT_SIZE;
	try {
		const stored = localStorage.getItem(FONT_SIZE_STORAGE_KEY);
		if (!stored) return DEFAULT_FONT_SIZE;
		const parsed = Number.parseInt(stored, 10);
		if (Number.isNaN(parsed)) return DEFAULT_FONT_SIZE;
		return clampFontSize(parsed);
	} catch {
		return DEFAULT_FONT_SIZE;
	}
};

const persistFontSize = (): void => {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.setItem(FONT_SIZE_STORAGE_KEY, String(currentFontSize));
	} catch {
		// Ignore storage failures.
	}
};

let currentFontSize = loadInitialFontSize();

const outputQueues = new Map<
	string,
	{ chunks: OutputChunk[]; bytes: number; scheduled: boolean }
>();
const bootstrapHandled = new Map<string, boolean>();
const bootstrapFetchTimers = new Map<string, number>();
const focusTimers = new Map<string, number>();
const lastDims = new Map<string, { cols: number; rows: number }>();
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
const pendingInput = new Map<string, string>();
const fitStabilizers = new Map<string, number>();
const resizeObservers = new Map<string, ResizeObserver>();
const resizeTimers = new Map<string, number>();

let debugEnabled = false;
let debugOverlayPreference: 'on' | 'off' | '' = '';

let suppressMouseUntil: Record<string, number> = {};
let mouseInputTail: Record<string, string> = {};

const OUTPUT_FLUSH_BUDGET = 128 * 1024;
const OUTPUT_BACKLOG_LIMIT = 512 * 1024;
const ACK_BATCH_BYTES = 32 * 1024;
const ACK_FLUSH_DELAY_MS = 25;
const INITIAL_STREAM_CREDIT = 256 * 1024;
const RESIZE_DEBOUNCE_MS = 100;
const HEALTH_TIMEOUT_MS = 1200;
const STARTUP_OUTPUT_TIMEOUT_MS = 2000;
const MAX_CLIPBOARD_BYTES = 1024 * 1024;
const textEncoder = typeof TextEncoder !== 'undefined' ? new TextEncoder() : null;
const textDecoder = typeof TextDecoder !== 'undefined' ? new TextDecoder() : null;
let globalsInitialized = false;

const buildTerminalKey = (workspaceId: string, terminalId: string): TerminalKey => {
	const workspace = workspaceId?.trim();
	const terminal = terminalId?.trim();
	if (!workspace || !terminal) return '';
	return `${workspace}::${terminal}`;
};

const defaultStats = () => ({
	bytesIn: 0,
	bytesOut: 0,
	backlog: 0,
	lastOutputAt: 0,
	lastCprAt: 0,
});

const resolveDebugEnabled = (): boolean => {
	if (debugOverlayPreference === 'on') return true;
	if (debugOverlayPreference === 'off') return false;
	if (typeof localStorage !== 'undefined') {
		return localStorage.getItem('worksetTerminalDebug') === '1';
	}
	return false;
};

const syncDebugEnabled = (): void => {
	const next = resolveDebugEnabled();
	if (next === debugEnabled) return;
	debugEnabled = next;
	emitAllStates();
};

const getClipboardPayloadBytes = (value: string): number => {
	if (!value) return 0;
	if (textEncoder) {
		return textEncoder.encode(value).length;
	}
	return value.length;
};

const encodeClipboardText = (value: string): string => {
	if (!value) return '';
	if (typeof btoa === 'function') {
		if (textEncoder) {
			const bytes = textEncoder.encode(value);
			let binary = '';
			for (const byte of bytes) {
				binary += String.fromCharCode(byte);
			}
			return btoa(binary);
		}
		try {
			return btoa(value);
		} catch {
			return '';
		}
	}
	return '';
};

const decodeClipboardText = (value: string): string => {
	if (!value) return '';
	const sanitized = value.replace(/\s+/g, '').replace(/-/g, '+').replace(/_/g, '/');
	const padding = sanitized.length % 4;
	const normalized = padding ? sanitized.padEnd(sanitized.length + (4 - padding), '=') : sanitized;
	try {
		if (typeof atob === 'function') {
			const binary = atob(normalized);
			if (textDecoder) {
				const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
				return textDecoder.decode(bytes);
			}
			return binary;
		}
	} catch {
		// Ignore invalid base64.
	}
	return '';
};

const createClipboardBase64 = (): {
	encodeText: (data: string) => string;
	decodeText: (data: string) => string;
} => ({
	encodeText: encodeClipboardText,
	decodeText: decodeClipboardText,
});

const getRuntimeClipboard = (): ((text: string) => Promise<boolean>) | null => {
	if (typeof window === 'undefined') return null;
	const runtime = (
		window as Window & {
			runtime?: { ClipboardSetText?: (text: string) => Promise<boolean> };
		}
	).runtime;
	if (!runtime?.ClipboardSetText) return null;
	return runtime.ClipboardSetText.bind(runtime);
};

const createClipboardProvider = (): {
	readText: (selection: ClipboardSelection) => Promise<string>;
	writeText: (selection: ClipboardSelection, text: string) => Promise<void>;
} => ({
	readText: async (_selection) => '',
	writeText: async (_selection, text) => {
		if (!text) return;
		if (getClipboardPayloadBytes(text) > MAX_CLIPBOARD_BYTES) return;
		const runtimeClipboard = getRuntimeClipboard();
		if (runtimeClipboard) {
			try {
				const ok = await runtimeClipboard(text);
				if (ok) return;
			} catch {
				// Fall back to browser clipboard.
			}
		}
		if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) return;
		try {
			await navigator.clipboard.writeText(text);
		} catch {
			// Ignore clipboard failures (permissions or missing API).
		}
	},
});

const buildState = (id: string): TerminalViewState => {
	const stats = statsMap.get(id) ?? defaultStats();
	const lifecycleState = lifecycle.getSnapshot(id);
	return {
		...lifecycleState,
		debugEnabled,
		debugStats: { ...stats },
	};
};

const ensureStore = (id: string): Writable<TerminalViewState> => {
	return terminalStores.ensure(id, buildState);
};

const emitState = (id: string): void => {
	terminalStores.emit(id, buildState);
};

const emitAllStates = (): void => {
	terminalStores.emitAll(buildState);
};

const lifecycle = createTerminalLifecycle({
	emitState,
	emitAllStates,
});

const syncTerminalWebLinks = createTerminalWebLinksSync({
	getHandle: (id) => terminalHandles.get(id),
	isMouseModeActive: (id) => lifecycle.getMode(id).mouse,
	openURL: (url) => {
		void terminalTransport.openURL(url);
	},
});

const subscribeEvent = <T>(event: string, handler: (payload: T) => void): (() => void) =>
	terminalTransport.onEvent(event, handler);

const clearTimeoutMap = (map: Map<string, number>, id: string): void => {
	const timer = map.get(id);
	if (!timer) return;
	window.clearTimeout(timer);
	map.delete(id);
};

const deleteRecordKey = <T>(record: Record<string, T>, id: string): Record<string, T> => {
	if (!Object.prototype.hasOwnProperty.call(record, id)) return record;
	const next = { ...record };
	delete next[id];
	return next;
};

const destroyTerminalState = (id: string): void => {
	if (!id) return;
	clearTimeoutMap(focusTimers, id);
	clearTimeoutMap(bootstrapFetchTimers, id);
	clearTimeoutMap(fitStabilizers, id);
	terminalStreamOrchestrator.clearReattachTimer(id);
	clearTimeoutMap(resizeTimers, id);
	resetSessionState(id);
	terminalStores.delete(id);
	outputQueues.delete(id);
	replayAckOrchestrator.destroy(id);
	bootstrapHandled.delete(id);
	lastDims.delete(id);
	statsMap.delete(id);
	pendingInput.delete(id);
	terminalResizeBridge.clear(id);
	renderHealth.release(id);
	lifecycle.deleteState(id);
	const observer = resizeObservers.get(id);
	if (observer) {
		observer.disconnect();
	}
	resizeObservers.delete(id);
	suppressMouseUntil = deleteRecordKey(suppressMouseUntil, id);
	mouseInputTail = deleteRecordKey(mouseInputTail, id);
};

const disposeTerminalResources = (id: string): void => {
	if (!id) return;
	terminalAttachState.release(id);
	terminalInstanceManager.dispose(id);
	destroyTerminalState(id);
};

const terminalAttachState = createTerminalAttachState({
	disposeAfterMs: DISPOSE_TTL_MS,
	onDispose: (id) => {
		disposeTerminalResources(id);
	},
	setTimeoutFn: (callback, timeoutMs) => window.setTimeout(callback, timeoutMs),
	clearTimeoutFn: (handle) => window.clearTimeout(handle),
});

const getContext = (key: TerminalKey): TerminalContext | null => {
	return terminalContexts.get(key) ?? null;
};

const ensureContext = (input: TerminalContext): TerminalContext => {
	const existing = terminalContexts.get(input.terminalKey);
	if (!existing) {
		terminalContexts.set(input.terminalKey, input);
		return input;
	}
	const next = { ...existing, ...input, terminalKey: input.terminalKey };
	terminalContexts.set(input.terminalKey, next);
	return next;
};

const getWorkspaceId = (key: TerminalKey): string => {
	return terminalContexts.get(key)?.workspaceId ?? '';
};

const getTerminalId = (key: TerminalKey): string => {
	return terminalContexts.get(key)?.terminalId ?? '';
};

const setHealth = (
	id: string,
	state: 'unknown' | 'checking' | 'ok' | 'stale',
	message = '',
): void => {
	lifecycle.setHealth(id, state, message);
};

const updateStats = (
	id: string,
	update: (stats: TerminalViewState['debugStats']) => void,
): void => {
	const stats = statsMap.get(id) ?? defaultStats();
	update(stats);
	statsMap.set(id, stats);
	if (debugEnabled) {
		emitState(id);
	}
};

const countBytes = (data: string): number => {
	if (textEncoder) {
		return textEncoder.encode(data).length;
	}
	return data.length;
};

const normalizeHex = (value: string | undefined | null): string | null => {
	if (typeof value !== 'string') return null;
	let hex = value.trim();
	if (!hex) return null;
	if (hex.startsWith('#')) {
		hex = hex.slice(1);
	}
	if (hex.length === 3) {
		hex = hex
			.split('')
			.map((item) => item + item)
			.join('');
	}
	if (hex.length !== 6) return null;
	return hex;
};

const toOscRgb = (value: string | undefined | null): string | null => {
	const hex = normalizeHex(value);
	if (!hex) return null;
	const r = hex.slice(0, 2);
	const g = hex.slice(2, 4);
	const b = hex.slice(4, 6);
	return `rgb:${r}/${g}/${b}`;
};

const defaultAnsiPalette = [
	'#000000',
	'#cd3131',
	'#0dbc79',
	'#e5e510',
	'#2472c8',
	'#bc3fbc',
	'#11a8cd',
	'#e5e5e5',
	'#666666',
	'#f14c4c',
	'#23d18b',
	'#f5f543',
	'#3b8eea',
	'#d670d6',
	'#29b8db',
	'#ffffff',
];

const themePalette = (theme: ITheme): string[] => {
	return [
		theme.black ?? defaultAnsiPalette[0],
		theme.red ?? defaultAnsiPalette[1],
		theme.green ?? defaultAnsiPalette[2],
		theme.yellow ?? defaultAnsiPalette[3],
		theme.blue ?? defaultAnsiPalette[4],
		theme.magenta ?? defaultAnsiPalette[5],
		theme.cyan ?? defaultAnsiPalette[6],
		theme.white ?? defaultAnsiPalette[7],
		theme.brightBlack ?? defaultAnsiPalette[8],
		theme.brightRed ?? defaultAnsiPalette[9],
		theme.brightGreen ?? defaultAnsiPalette[10],
		theme.brightYellow ?? defaultAnsiPalette[11],
		theme.brightBlue ?? defaultAnsiPalette[12],
		theme.brightMagenta ?? defaultAnsiPalette[13],
		theme.brightCyan ?? defaultAnsiPalette[14],
		theme.brightWhite ?? defaultAnsiPalette[15],
	];
};

const resolveThemeColor = (value: string | undefined, fallback: string): string | null => {
	return toOscRgb(value ?? fallback);
};

const resolveAnsiColor = (terminal: Terminal, index: number): string | null => {
	const theme = terminal.options.theme ?? {};
	if (index < 16) {
		const value = themePalette(theme)[index];
		return value ? toOscRgb(value) : null;
	}
	if (index >= 16 && theme.extendedAnsi && theme.extendedAnsi[index - 16]) {
		return toOscRgb(theme.extendedAnsi[index - 16]);
	}
	return null;
};

const getToken = (name: string, fallback: string): string => {
	const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
	return value || fallback;
};

const terminalKittyController = createTerminalKittyController<TerminalHandle>({
	getHandle: (id) => terminalHandles.get(id),
});

const noteMouseSuppress = (id: string, durationMs: number): void => {
	suppressMouseUntil = { ...suppressMouseUntil, [id]: Date.now() + durationMs };
};

const shouldSuppressMouseInput = (id: string, data: string): boolean => {
	const until = suppressMouseUntil[id];
	if (!until || Date.now() >= until) {
		return false;
	}
	return data.includes('\x1b[<');
};

const terminalInputOrchestrator = createTerminalInputOrchestrator({
	shouldSuppressMouseInput,
	getMode: (id) => lifecycle.getMode(id),
	filterMouseReports: stripMouseReports,
	getMouseTail: (id) => mouseInputTail[id] ?? '',
	setMouseTail: (id, tail) => {
		mouseInputTail = { ...mouseInputTail, [id]: tail };
	},
	ensureSessionActive: (id) => ensureSessionActive(id),
	hasStarted: (id) => lifecycle.hasStarted(id),
	appendPendingInput: (id, data) => {
		pendingInput.set(id, (pendingInput.get(id) ?? '') + data);
	},
	recordOutputBytes: (id, bytes) => {
		updateStats(id, (stats) => {
			stats.bytesOut += bytes;
		});
	},
	getWorkspaceId,
	getTerminalId,
	write: (workspaceId, terminalId, data) => terminalTransport.write(workspaceId, terminalId, data),
	markStopped: (id) => lifecycle.markStopped(id),
	resetTerminalInstance: (id) => {
		resetTerminalInstance(id);
	},
	beginTerminal: (id, quiet) => beginTerminal(id, quiet),
	writeFailureMessage: (id, message) => {
		const handle = terminalHandles.get(id);
		handle?.terminal.write(`\r\n[workset] write failed: ${message}`);
	},
});

const sendInput = (id: string, data: string): void => {
	terminalInputOrchestrator.sendInput(id, data);
};

const captureTerminalViewport = (terminal: Terminal) => {
	const buffer = terminal.buffer.active;
	return captureViewportSnapshot({
		baseY: buffer.baseY,
		viewportY: buffer.viewportY,
	});
};

const restoreTerminalViewport = (
	terminal: Terminal,
	viewport: ReturnType<typeof captureTerminalViewport>,
): void => {
	const targetLine = resolveViewportTargetLine(viewport, terminal.buffer.active.baseY);
	if (targetLine === null) {
		terminal.scrollToBottom();
		return;
	}
	terminal.scrollToLine(targetLine);
};

const fitWithPreservedViewport = (
	handle: TerminalHandle,
	viewport = captureTerminalViewport(handle.terminal),
): void => {
	handle.fitAddon.fit();
	restoreTerminalViewport(handle.terminal, viewport);
	terminalKittyController.resizeOverlay(handle);
};

const attachResizeObserver = (id: string, container: HTMLDivElement | null): void => {
	const existing = resizeObservers.get(id);
	if (existing) {
		existing.disconnect();
		resizeObservers.delete(id);
	}
	if (!container) return;
	const observer = new ResizeObserver(() => {
		const existingTimer = resizeTimers.get(id);
		if (existingTimer) {
			window.clearTimeout(existingTimer);
		}
		resizeTimers.set(
			id,
			window.setTimeout(() => {
				resizeTimers.delete(id);
				const handle = terminalHandles.get(id);
				if (!handle) return;
				fitTerminal(id, lifecycle.hasStarted(id));
			}, RESIZE_DEBOUNCE_MS),
		);
	});
	observer.observe(container);
	resizeObservers.set(id, observer);
};

const fitTerminal = (id: string, resizeSession: boolean): void => {
	const handle = terminalHandles.get(id);
	if (!handle) return;
	fitWithPreservedViewport(handle);
	forceRedraw(id);
	if (!resizeSession) return;
	terminalResizeBridge.resizeToFit(id, handle);
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
		const handle = terminalHandles.get(id);
		if (!handle) return;
		const dims = handle.fitAddon.proposeDimensions();
		if (!dims || dims.cols <= 0 || dims.rows <= 0) {
			attempts += 1;
			if (attempts < 6) {
				fitStabilizers.set(id, window.setTimeout(run, 80 + attempts * 20));
			}
			return;
		}
		const prev = lastDims.get(id);
		if (prev && prev.cols === dims.cols && prev.rows === dims.rows) {
			stableCount += 1;
		} else {
			stableCount = 0;
		}
		lastDims.set(id, { cols: dims.cols, rows: dims.rows });
		fitTerminal(id, lifecycle.hasStarted(id));
		if (stableCount < 2 && attempts < 5) {
			attempts += 1;
			fitStabilizers.set(id, window.setTimeout(run, 80 + attempts * 30));
		}
	};
	fitStabilizers.set(id, window.setTimeout(run, 60));
	logDebug(id, 'fit', { reason });
};

const focusTerminal = (id: string): void => {
	if (!id) return;
	const handle = terminalHandles.get(id);
	if (handle) {
		handle.terminal.focus();
		return;
	}
	if (focusTimers.has(id)) return;
	focusTimers.set(
		id,
		window.setTimeout(() => {
			focusTimers.delete(id);
			const current = terminalHandles.get(id);
			current?.terminal.focus();
		}, 0),
	);
};

const resetSessionState = (id: string): void => {
	replayAckOrchestrator.resetSession(id);
	outputQueues.delete(id);
	bootstrapHandled.delete(id);
	const bootstrapTimer = bootstrapFetchTimers.get(id);
	if (bootstrapTimer) {
		window.clearTimeout(bootstrapTimer);
	}
	bootstrapFetchTimers.delete(id);
	lifecycle.dropHealthCheck(id);
	renderHealth.clearSession(id);
	terminalResizeBridge.clear(id);
	if (mouseInputTail[id]) {
		mouseInputTail = { ...mouseInputTail, [id]: '' };
	}
};

const resetTerminalInstance = (id: string): void => {
	const handle = terminalHandles.get(id);
	if (!handle) return;
	handle.terminal.reset();
	handle.terminal.clear();
	handle.terminal.scrollToBottom();
	handle.fitAddon.fit();
	terminalKittyController.resizeOverlay(handle);
	lifecycle.setMode(id, { altScreen: false, mouse: false, mouseSGR: false, mouseEncoding: 'x10' });
	if (mouseInputTail[id]) {
		mouseInputTail = { ...mouseInputTail, [id]: '' };
	}
	noteMouseSuppress(id, 2500);
	void terminalRendererAddonState.load(id, handle);
};

const updateStatsLastOutput = (id: string): void => {
	updateStats(id, (stats) => {
		stats.lastOutputAt = Date.now();
	});
};

const scheduleStartupTimeout = (id: string): void => {
	lifecycle.scheduleStartupTimeout(id, {
		timeoutMs: STARTUP_OUTPUT_TIMEOUT_MS,
		onTimeout: () => {
			pendingInput.delete(id);
		},
	});
};

const clearStartupTimeout = (id: string): void => {
	lifecycle.clearStartupTimeout(id);
};

const logDebug = (id: string, event: string, details: Record<string, unknown>): void => {
	syncDebugEnabled();
	if (!debugEnabled) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	void terminalTransport.logDebug(workspaceId, terminalId, event, JSON.stringify(details));
};

const terminalResizeBridge = createTerminalResizeBridge({
	getWorkspaceId,
	getTerminalId,
	resize: (workspaceId, terminalId, cols, rows) =>
		terminalTransport.resize(workspaceId, terminalId, cols, rows),
	logDebug,
});

const forceRedraw = (id: string): void => {
	const handle = terminalHandles.get(id);
	if (!handle) return;
	handle.terminal.refresh(0, handle.terminal.rows - 1);
};

const renderHealth = createTerminalRenderHealth({
	getHandle: (id) => terminalHandles.get(id),
	reopenWithPreservedViewport: (id, handle) => {
		if (!handle.container) return;
		const viewport = captureTerminalViewport(handle.terminal);
		handle.terminal.open(handle.container);
		fitWithPreservedViewport(handle, viewport);
		terminalResizeBridge.nudgeRedraw(id, handle);
	},
	fitWithPreservedViewport: (_id, handle) => {
		fitWithPreservedViewport(handle);
	},
	nudgeRedraw: (id, handle) => {
		terminalResizeBridge.nudgeRedraw(id, handle);
	},
	logDebug,
});

const enqueueOutput = (id: string, data: string, bytes: number): void => {
	const queue = outputQueues.get(id) ?? { chunks: [], bytes: 0, scheduled: false };
	queue.chunks.push({ data, bytes });
	queue.bytes += bytes;
	outputQueues.set(id, queue);
	if (!queue.scheduled) {
		queue.scheduled = true;
		requestAnimationFrame(() => flushOutput(id, true));
	}
};

const flushOutput = (id: string, scheduled: boolean): void => {
	const queue = outputQueues.get(id);
	if (!queue) return;
	if (queue.scheduled !== scheduled) return;
	queue.scheduled = false;
	const handle = terminalHandles.get(id);
	if (!handle) return;
	let budget = OUTPUT_FLUSH_BUDGET;
	while (queue.chunks.length > 0 && budget > 0) {
		const chunk = queue.chunks.shift();
		if (!chunk) break;
		budget -= chunk.bytes;
		handle.terminal.write(chunk.data, () => {
			renderHealth.noteRender(id);
		});
		updateStatsLastOutput(id);
	}
	queue.bytes = queue.chunks.reduce((sum, chunk) => sum + chunk.bytes, 0);
	if (queue.bytes > OUTPUT_BACKLOG_LIMIT) {
		queue.chunks.splice(0, Math.floor(queue.chunks.length / 2));
		queue.bytes = queue.chunks.reduce((sum, chunk) => sum + chunk.bytes, 0);
	}
	if (queue.chunks.length > 0 && !queue.scheduled) {
		queue.scheduled = true;
		requestAnimationFrame(() => flushOutput(id, true));
	}
};

const replayAckOrchestrator = createTerminalReplayAckOrchestrator<KittyEventPayload>({
	enqueueOutput,
	flushOutput,
	forceRedraw,
	hasTerminalHandle: (id) => terminalHandles.has(id),
	applyKittyEvent: (id, event) => terminalKittyController.applyEvent(id, event),
	getWorkspaceId,
	getTerminalId,
	ack: (workspaceId, terminalId, bytes) => terminalTransport.ack(workspaceId, terminalId, bytes),
	setTimeoutFn: (callback, timeoutMs) => window.setTimeout(callback, timeoutMs),
	clearTimeoutFn: (handle) => window.clearTimeout(handle),
	countBytes,
	recordBytesIn: (id, bytes) => {
		updateStats(id, (stats) => {
			stats.bytesIn += bytes;
		});
	},
	noteOutputActivity: (id) => {
		renderHealth.noteOutputActivity(id);
	},
	ackBatchBytes: ACK_BATCH_BYTES,
	ackFlushDelayMs: ACK_FLUSH_DELAY_MS,
	initialStreamCredit: INITIAL_STREAM_CREDIT,
});

const requestHealthCheck = (id: string): void => {
	lifecycle.requestHealthCheck(id, { timeoutMs: HEALTH_TIMEOUT_MS });
};

const captureCpr = (id: string, data: string): void => {
	const cprIndex = data.indexOf('\x1b[');
	if (cprIndex < 0) return;
	const match = data.slice(cprIndex + 2).match(/^(\d+);(\d+)R/);
	if (!match) return;
	updateStats(id, (stats) => {
		stats.lastCprAt = Date.now();
	});
};

const ensureSessionActive = async (id: string): Promise<void> => {
	if (lifecycle.hasStarted(id) || lifecycle.hasStartInFlight(id)) return;
	if (lifecycle.isSessiondAvailable() !== true) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	try {
		const status = await terminalTransport.fetchStatus(workspaceId, terminalId);
		if (status?.active) {
			lifecycle.markStarted(id);
			lifecycle.setStatusAndMessage(id, 'ready', '');
			setHealth(id, 'ok', 'Session resumed.');
			lifecycle.setInput(id, true);
			lifecycle.ensureRendererDefaults(id);
			emitState(id);
		}
	} catch {
		// Ignore.
	}
};

const maybeFetchBootstrap = async (id: string, reason: string): Promise<void> => {
	if (!id) return;
	if (bootstrapHandled.get(id)) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	try {
		const payload = await terminalTransport.fetchBootstrap(workspaceId, terminalId);
		if (bootstrapHandled.get(id)) return;
		terminalModeBootstrapCoordinator.handleBootstrapPayload(payload);
		terminalModeBootstrapCoordinator.handleBootstrapDonePayload({ workspaceId, terminalId });
		logDebug(id, 'bootstrap_fetch', {
			reason,
			source: payload.source,
			snapshotSource: payload.snapshotSource,
			backlogSource: payload.backlogSource,
			backlogTruncated: payload.backlogTruncated,
		});
	} catch (error) {
		logDebug(id, 'bootstrap_fetch_failed', { reason, error: String(error) });
	}
};

const beginTerminal = async (id: string, quiet = false): Promise<void> => {
	if (!id || lifecycle.hasStarted(id) || lifecycle.hasStartInFlight(id)) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	lifecycle.markStartInFlight(id);
	resetSessionState(id);
	bootstrapHandled.set(id, false);
	replayAckOrchestrator.setReplayState(id, 'replaying');
	if (!quiet) {
		lifecycle.setStatusAndMessage(id, 'starting', 'Waiting for shell output…');
		setHealth(id, 'unknown');
		lifecycle.setInput(id, false);
		scheduleStartupTimeout(id);
		emitState(id);
	}
	try {
		await terminalTransport.start(workspaceId, terminalId);
		lifecycle.markStarted(id);
		const queued = pendingInput.get(id);
		if (queued) {
			pendingInput.delete(id);
			await terminalTransport.write(workspaceId, terminalId, queued);
		}
		const existingTimer = bootstrapFetchTimers.get(id);
		if (existingTimer) {
			window.clearTimeout(existingTimer);
		}
		bootstrapFetchTimers.set(
			id,
			window.setTimeout(() => {
				bootstrapFetchTimers.delete(id);
				if (!bootstrapHandled.get(id)) {
					void maybeFetchBootstrap(id, 'bootstrap_timeout');
				}
			}, 200),
		);
	} catch (error) {
		lifecycle.setStatusAndMessage(id, 'error', String(error));
		setHealth(id, 'stale', 'Failed to start terminal.');
		clearStartupTimeout(id);
		pendingInput.delete(id);
		const handle = terminalHandles.get(id);
		handle?.terminal.write(`\r\n[workset] failed to start terminal: ${String(error)}`);
		emitState(id);
	} finally {
		lifecycle.clearStartInFlight(id);
	}
};

const ensureStream = async (id: string): Promise<void> => {
	if (!id || lifecycle.hasStartInFlight(id)) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	lifecycle.markStartInFlight(id);
	try {
		await terminalTransport.start(workspaceId, terminalId);
		lifecycle.markStarted(id);
	} catch (error) {
		logDebug(id, 'ensure_stream_failed', { error: String(error) });
	} finally {
		lifecycle.clearStartInFlight(id);
	}
};

const loadTerminalDefaults = async (): Promise<void> => {
	let nextDebugPreference = debugOverlayPreference;
	try {
		const settings = await terminalTransport.fetchSettings();
		const rawPreference = settings?.defaults?.terminalDebugOverlay ?? '';
		const normalizedPreference = rawPreference.toLowerCase().trim();
		if (normalizedPreference === 'on' || normalizedPreference === 'off') {
			nextDebugPreference = normalizedPreference;
		}
	} catch {
		// Keep existing preference on load failure.
	}
	debugOverlayPreference = nextDebugPreference;
	if (debugOverlayPreference === 'off' && typeof localStorage !== 'undefined') {
		try {
			localStorage.removeItem('worksetTerminalDebug');
		} catch {
			// Ignore storage failures.
		}
	}
	syncDebugEnabled();
};

const refreshSessiondStatus = async (): Promise<void> => {
	try {
		const status = await terminalTransport.fetchSessiondStatus();
		lifecycle.setSessiondStatus(status?.available ?? false);
	} catch {
		lifecycle.setSessiondStatus(false);
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

const terminalRendererAddonState = createTerminalRendererAddonState({
	setRendererMode: (id, mode) => {
		lifecycle.setRendererMode(id, mode);
	},
	setRenderer: (id, renderer) => {
		lifecycle.setRenderer(id, renderer);
	},
	onRendererUnavailable: (id, error) => {
		lifecycle.setStatusAndMessage(id, 'error', 'WebGL renderer unavailable.');
		setHealth(id, 'stale', 'WebGL renderer unavailable.');
		logDebug(id, 'renderer_webgl_failed', { error: String(error) });
	},
	onComplete: (id) => {
		emitState(id);
	},
});

const terminalAttachOpenLifecycle = createTerminalAttachOpenLifecycle({
	getHandle: (id) => terminalHandles.get(id),
	ensureOverlay: (_handle, id) => {
		terminalKittyController.ensureOverlay(id);
	},
	loadRendererAddon: (id, handle) => terminalRendererAddonState.load(id, handle),
	fitWithPreservedViewport: (handle) => {
		fitWithPreservedViewport(handle);
	},
	resizeToFit: (id, handle) => {
		terminalResizeBridge.resizeToFit(id, handle);
	},
	scheduleFitStabilization,
	flushOutput,
	markAttached: (id) => {
		terminalAttachState.markAttached(id);
	},
});

const terminalInstanceManager = createTerminalInstanceManager<KittyState>({
	terminalHandles,
	createTerminalInstance: () =>
		createTerminalInstance({
			fontSize: currentFontSize,
			getToken,
		}),
	createFitAddon: () => new FitAddon(),
	createUnicode11Addon: () => new Unicode11Addon(),
	createClipboardAddon: () =>
		new ClipboardAddon(createClipboardBase64(), createClipboardProvider()),
	createKittyState,
	syncTerminalWebLinks,
	registerOscHandlers,
	ensureMode: (id) => {
		lifecycle.ensureMode(id);
	},
	onShiftEnter: (id) => {
		lifecycle.setInput(id, true);
		void beginTerminal(id);
		sendInput(id, '\x0a');
	},
	onData: (id, data) => {
		lifecycle.setInput(id, true);
		void beginTerminal(id);
		captureCpr(id, data);
		sendInput(id, data);
	},
	onBinary: (id, data) => {
		lifecycle.setInput(id, true);
		void beginTerminal(id);
		sendInput(id, data);
	},
	onRender: (id) => {
		renderHealth.noteRender(id);
	},
	attachOpen: ({ id, handle, container, active }) => {
		terminalAttachOpenLifecycle.attach({ id, handle, container, active });
	},
});

const attachTerminal = (
	id: string,
	container: HTMLDivElement | null,
	active: boolean,
): TerminalHandle => {
	return terminalInstanceManager.attach(id, container, active);
};

const terminalModeBootstrapCoordinator = createTerminalModeBootstrapCoordinator<KittyEventPayload>({
	buildTerminalKey,
	getContext: (key) => terminalContexts.get(key) ?? null,
	logDebug,
	markInput: (id) => {
		lifecycle.markInput(id);
	},
	bootstrapHandled,
	setReplayState: replayAckOrchestrator.setReplayState,
	enqueueOutput,
	countBytes,
	pendingReplayKitty: replayAckOrchestrator.pendingReplayKitty,
	hasTerminalHandle: (id) => terminalHandles.has(id),
	applyKittyEvent: (id, event) => terminalKittyController.applyEvent(id, event),
	setHealth,
	initialCreditMap: replayAckOrchestrator.initialCreditMap,
	initialStreamCredit: INITIAL_STREAM_CREDIT,
	pendingReplayOutput: replayAckOrchestrator.pendingReplayOutput,
	getStatus: (id) => lifecycle.getStatus(id),
	setStatusAndMessage: (id, status, message) => {
		lifecycle.setStatusAndMessage(id, status, message);
	},
	scheduleBootstrapHealthCheck: (id, replayBytes) => {
		renderHealth.scheduleBootstrapHealthCheck(id, replayBytes);
	},
	emitState,
	applyLifecyclePayload: (id, payload) => {
		lifecycle.applyLifecyclePayload(id, payload);
	},
	setMode: (id, mode) => {
		lifecycle.setMode(id, mode);
	},
	syncTerminalWebLinks,
});

const handleTerminalDataEvent = (id: string, payload: TerminalPayload): void => {
	lifecycle.markInput(id);
	replayAckOrchestrator.handleTerminalData(id, payload);
};

const handleTerminalKittyEvent = (id: string, payload: TerminalKittyPayload): void => {
	replayAckOrchestrator.handleTerminalKitty(id, payload.event);
};

const handleSessiondRestarted = (): void => {
	lifecycle.resetSessiondChecked();
	void (async () => {
		await refreshSessiondStatus();
		if (lifecycle.isSessiondAvailable() !== true) return;
		for (const id of terminalContexts.keys()) {
			lifecycle.clearSessionFlags(id);
			resetTerminalInstance(id);
			resetSessionState(id);
			noteMouseSuppress(id, 4000);
			void beginTerminal(id, true);
		}
	})();
};

const terminalEventSubscriptions = createTerminalEventSubscriptions({
	subscribeEvent,
	buildTerminalKey,
	isWorkspaceMismatch: terminalModeBootstrapCoordinator.isWorkspaceMismatch,
	onTerminalData: handleTerminalDataEvent,
	onTerminalBootstrap: terminalModeBootstrapCoordinator.handleBootstrapPayload,
	onTerminalBootstrapDone: terminalModeBootstrapCoordinator.handleBootstrapDonePayload,
	onTerminalLifecycle: terminalModeBootstrapCoordinator.handleTerminalLifecyclePayload,
	onTerminalModes: terminalModeBootstrapCoordinator.handleTerminalModesPayload,
	onTerminalKitty: handleTerminalKittyEvent,
	onSessiondRestarted: handleSessiondRestarted,
});

const initTerminal = async (id: string): Promise<void> => {
	if (!id) return;
	const token = lifecycle.nextInitToken(id);
	terminalEventSubscriptions.ensureListeners();
	if (!lifecycle.isSessiondChecked()) {
		await refreshSessiondStatus();
	}
	const ctx = getContext(id);
	attachTerminal(id, ctx?.container ?? null, ctx?.active ?? false);
	let resumed = false;
	if (lifecycle.isSessiondAvailable() === true) {
		try {
			const workspaceId = getWorkspaceId(id);
			const terminalId = getTerminalId(id);
			if (workspaceId && terminalId) {
				const status = await terminalTransport.fetchStatus(workspaceId, terminalId);
				resumed = status?.active ?? false;
			}
		} catch {
			resumed = false;
		}
	}
	if (resumed) {
		await beginTerminal(id, true);
		lifecycle.setInput(id, true);
		lifecycle.setStatusAndMessage(id, 'ready', '');
		setHealth(id, 'ok', 'Session resumed.');
		lifecycle.ensureRendererDefaults(id);
		emitState(id);
		return;
	}
	if (!lifecycle.isCurrentInitToken(id, token)) return;
	lifecycle.dropHealthCheck(id);
	if (!lifecycle.hasStarted(id) && !lifecycle.hasStartInFlight(id)) {
		lifecycle.setStatusAndMessage(id, 'standby', '');
		setHealth(id, 'unknown');
		lifecycle.ensureRendererDefaults(id);
		lifecycle.setInput(id, false);
		emitState(id);
	}
};

const terminalStreamOrchestrator = createTerminalStreamOrchestrator({
	ensureSessionActive,
	initTerminal,
	getContext,
	hasStarted: (id) => lifecycle.hasStarted(id),
	getStatus: (id) => lifecycle.getStatus(id),
	ensureStream,
	beginTerminal,
	emitState,
	logDebug,
});

const ensureGlobals = (): void => {
	if (globalsInitialized) return;
	globalsInitialized = true;
	void loadTerminalDefaults();
	void refreshSessiondStatus();
	window.addEventListener('focus', () => {
		terminalAttachState.forEachAttached((id) => {
			void ensureSessionActive(id);
		});
	});
};

export const refreshTerminalDefaults = async (): Promise<void> => {
	await loadTerminalDefaults();
};

export const getTerminalStore = (
	workspaceId: string,
	terminalId: string,
): Readable<TerminalViewState> => {
	const key = buildTerminalKey(workspaceId, terminalId);
	if (!key) {
		return ensureStore('');
	}
	return ensureStore(key);
};

export const syncTerminal = (input: {
	workspaceId: string;
	workspaceName: string;
	terminalId: string;
	container: HTMLDivElement | null;
	active: boolean;
}): void => {
	if (!input.terminalId || !input.workspaceId) return;
	ensureGlobals();
	const terminalKey = buildTerminalKey(input.workspaceId, input.terminalId);
	if (!terminalKey) return;
	const context = ensureContext({
		terminalKey,
		workspaceId: input.workspaceId,
		workspaceName: input.workspaceName,
		terminalId: input.terminalId,
		container: input.container,
		active: input.active,
		lastWorkspaceId: terminalContexts.get(terminalKey)?.lastWorkspaceId ?? '',
	});
	if (context.lastWorkspaceId && context.lastWorkspaceId !== context.workspaceId) {
		scheduleFitStabilization(context.terminalKey, 'workspace_switch');
		terminalStreamOrchestrator.scheduleReattachCheck(context.terminalKey, 'workspace_switch');
	}
	context.lastWorkspaceId = context.workspaceId;
	if (input.container) {
		attachTerminal(terminalKey, input.container, input.active);
		attachResizeObserver(terminalKey, input.container);
		if (input.active) {
			requestAnimationFrame(() => {
				fitTerminal(terminalKey, lifecycle.hasStarted(terminalKey));
				forceRedraw(terminalKey);
				const handle = terminalHandles.get(terminalKey);
				if (handle && !hasVisibleTerminalContent(handle.terminal)) {
					terminalResizeBridge.nudgeRedraw(terminalKey, handle);
				}
			});
		}
	}
	terminalStreamOrchestrator.syncTerminalStream(terminalKey);
};

export const detachTerminal = (workspaceId: string, terminalId: string): void => {
	const terminalKey = buildTerminalKey(workspaceId, terminalId);
	if (!terminalKey) return;
	terminalAttachState.markDetached(terminalKey);
	const observer = resizeObservers.get(terminalKey);
	if (observer) {
		observer.disconnect();
		resizeObservers.delete(terminalKey);
	}
	const timer = resizeTimers.get(terminalKey);
	if (timer) {
		window.clearTimeout(timer);
		resizeTimers.delete(terminalKey);
	}
};

export const closeTerminal = async (workspaceId: string, terminalId: string): Promise<void> => {
	const terminalKey = buildTerminalKey(workspaceId, terminalId);
	if (!terminalKey) return;
	try {
		await terminalTransport.stop(workspaceId, terminalId);
	} catch {
		// Ignore failures.
	}
	disposeTerminalResources(terminalKey);
	terminalContexts.delete(terminalKey);
};

export const restartTerminal = async (workspaceId: string, terminalId: string): Promise<void> => {
	const terminalKey = buildTerminalKey(workspaceId, terminalId);
	if (!terminalKey) return;
	await beginTerminal(terminalKey);
};

export const retryHealthCheck = (workspaceId: string, terminalId: string): void => {
	const terminalKey = buildTerminalKey(workspaceId, terminalId);
	if (!terminalKey) return;
	requestHealthCheck(terminalKey);
};

export const focusTerminalInstance = (workspaceId: string, terminalId: string): void => {
	const terminalKey = buildTerminalKey(workspaceId, terminalId);
	if (!terminalKey) return;
	focusTerminal(terminalKey);
};

export const scrollTerminalToBottom = (workspaceId: string, terminalId: string): void => {
	const terminalKey = buildTerminalKey(workspaceId, terminalId);
	if (!terminalKey) return;
	const handle = terminalHandles.get(terminalKey);
	if (!handle) return;
	handle.terminal.scrollToBottom();
};

export const isTerminalAtBottom = (workspaceId: string, terminalId: string): boolean => {
	const terminalKey = buildTerminalKey(workspaceId, terminalId);
	if (!terminalKey) return true;
	const handle = terminalHandles.get(terminalKey);
	if (!handle) return true;
	const buffer = handle.terminal.buffer.active;
	return buffer.baseY === buffer.viewportY;
};

export const shutdownTerminalService = (): void => {
	terminalEventSubscriptions.cleanupListeners();
};

// Font size controls (VS Code style Cmd/Ctrl +/-)
const applyFontSizeToAllTerminals = (): void => {
	for (const [id, handle] of terminalHandles.entries()) {
		handle.terminal.options.fontSize = currentFontSize;
		// Refit terminal to recalculate dimensions with new font size.
		try {
			fitTerminal(id, lifecycle.hasStarted(id));
		} catch {
			// Ignore fit errors for terminals not attached to DOM.
		}
	}
};

export const increaseFontSize = (): void => {
	if (currentFontSize < MAX_FONT_SIZE) {
		currentFontSize = Math.min(currentFontSize + FONT_SIZE_STEP, MAX_FONT_SIZE);
		persistFontSize();
		applyFontSizeToAllTerminals();
	}
};

export const decreaseFontSize = (): void => {
	if (currentFontSize > MIN_FONT_SIZE) {
		currentFontSize = Math.max(currentFontSize - FONT_SIZE_STEP, MIN_FONT_SIZE);
		persistFontSize();
		applyFontSizeToAllTerminals();
	}
};

export const resetFontSize = (): void => {
	if (currentFontSize !== DEFAULT_FONT_SIZE) {
		currentFontSize = DEFAULT_FONT_SIZE;
		persistFontSize();
		applyFontSizeToAllTerminals();
	}
};

export const getCurrentFontSize = (): number => currentFontSize;
