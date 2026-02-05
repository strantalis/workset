import {
	Terminal,
	type ITerminalInitOnlyOptions,
	type ITerminalOptions,
	type ITheme,
} from '@xterm/xterm';
import { ClipboardAddon } from '@xterm/addon-clipboard';
import { FitAddon } from '@xterm/addon-fit';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { WebglAddon } from '@xterm/addon-webgl';
import { writable, type Readable, type Writable } from 'svelte/store';
import { BrowserOpenURL, EventsOff, EventsOn } from '../../../wailsjs/runtime/runtime';
import {
	AckWorkspaceTerminal,
	ResizeWorkspaceTerminal,
	StartWorkspaceTerminal,
	WriteWorkspaceTerminal,
} from '../../../wailsjs/go/main/App';
import {
	fetchSessiondStatus,
	fetchSettings,
	fetchTerminalBootstrap,
	fetchWorkspaceTerminalStatus,
	logTerminalDebug,
	stopWorkspaceTerminal,
} from '../api';
import { stripMouseReports } from './inputFilter';

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
type EventHandler<T> = (payload: T) => void;
type EventRegistryEntry = {
	handlers: Set<EventHandler<unknown>>;
	bound: boolean;
};

type OutputChunk = {
	data: string;
	bytes: number;
};

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
	clipboardAddon?: ClipboardAddon;
	unicode11Addon?: Unicode11Addon;
	webLinksAddon?: WebLinksAddon;
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
		images?: KittyEventPayload['image'][];
		placements?: KittyEventPayload['placement'][];
	};
};

type TerminalPayload = {
	workspaceId: string;
	terminalId: string;
	data: string;
	bytes?: number;
};

type TerminalBootstrapPayload = {
	workspaceId: string;
	terminalId: string;
	snapshot?: string;
	snapshotSource?: string;
	kitty?: { images?: unknown[]; placements?: unknown[] } | null;
	backlog?: string;
	backlogSource?: string;
	backlogTruncated?: boolean;
	nextOffset?: number;
	source?: string;
	altScreen?: boolean;
	mouse?: boolean;
	mouseSGR?: boolean;
	mouseEncoding?: string;
	safeToReplay?: boolean;
	initialCredit?: number;
};

type TerminalBootstrapDonePayload = {
	workspaceId: string;
	terminalId: string;
};

type TerminalLifecyclePayload = {
	workspaceId: string;
	terminalId: string;
	status: string;
	message?: string;
};

type TerminalModesPayload = {
	workspaceId: string;
	terminalId: string;
	altScreen?: boolean;
	mouse?: boolean;
	mouseSGR?: boolean;
	mouseEncoding?: string;
};

type TerminalKittyPayload = {
	workspaceId: string;
	terminalId: string;
	event: KittyEventPayload;
};

const terminalHandles = new Map<string, TerminalHandle>();
const terminalContexts = new Map<string, TerminalContext>();
const terminalStores = new Map<string, Writable<TerminalViewState>>();
const eventRegistry = new Map<string, EventRegistryEntry>();
const listeners = new Set<string>();
const unsubscribeHandlers: Array<() => void> = [];

const attachedTerminals = new Set<string>();
const disposeTimers = new Map<string, number>();
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
const replayState = new Map<string, 'idle' | 'replaying' | 'live'>();
const pendingReplayOutput = new Map<string, OutputChunk[]>();
const pendingReplayKitty = new Map<string, KittyEventPayload[]>();
const bootstrapHandled = new Map<string, boolean>();
const bootstrapFetchTimers = new Map<string, number>();
const focusTimers = new Map<string, number>();
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
const initialCreditSent = new Set<string>();
const ackTimers = new Map<string, number>();
const fitStabilizers = new Map<string, number>();
const reattachTimers = new Map<string, number>();
const resizeObservers = new Map<string, ResizeObserver>();
const resizeTimers = new Map<string, number>();
const initTokens = new Map<string, number>();
const startedSessions = new Set<string>();

let rendererPreference = 'webgl' as const;
let sessiondAvailable: boolean | null = null;
let sessiondChecked = false;
let debugEnabled = false;
let debugOverlayPreference: 'on' | 'off' | '' = '';

let statusMap: Record<string, string> = {};
let messageMap: Record<string, string> = {};
let inputMap: Record<string, boolean> = {};
let healthMap: Record<string, 'unknown' | 'checking' | 'ok' | 'stale'> = {};
let healthMessageMap: Record<string, string> = {};
let suppressMouseUntil: Record<string, number> = {};
let mouseInputTail: Record<string, string> = {};
let rendererMap: Record<string, 'unknown' | 'webgl'> = {};
let rendererModeMap: Record<string, 'webgl'> = {};
let modeMap: Record<
	string,
	{ altScreen: boolean; mouse: boolean; mouseSGR: boolean; mouseEncoding: string }
> = {};

const OUTPUT_FLUSH_BUDGET = 128 * 1024;
const OUTPUT_BACKLOG_LIMIT = 512 * 1024;
const ACK_BATCH_BYTES = 32 * 1024;
const ACK_FLUSH_DELAY_MS = 25;
const INITIAL_STREAM_CREDIT = 256 * 1024;
const RESIZE_DEBOUNCE_MS = 100;
const HEALTH_TIMEOUT_MS = 1200;
const STARTUP_OUTPUT_TIMEOUT_MS = 2000;
const RENDER_CHECK_DELAY_MS = 350;
const RENDER_RECOVERY_DELAY_MS = 150;
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

const openHttpsLink = (event: MouseEvent, uri: string): void => {
	if (!uri) return;
	if (!event?.ctrlKey && !event?.metaKey) return;
	try {
		const parsed = new URL(uri);
		if (parsed.protocol !== 'https:') return;
		BrowserOpenURL(parsed.toString());
		event.preventDefault();
	} catch {
		// Ignore invalid URLs.
	}
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

const createWebLinksAddon = (): WebLinksAddon =>
	new WebLinksAddon((event, uri) => {
		openHttpsLink(event, uri);
	});

const syncWebLinksForMode = (id: string): void => {
	const handle = terminalHandles.get(id);
	if (!handle) return;
	const mouseActive = modeMap[id]?.mouse ?? false;
	if (mouseActive) {
		if (handle.webLinksAddon) {
			handle.webLinksAddon.dispose();
			handle.webLinksAddon = undefined;
		}
		return;
	}
	if (!handle.webLinksAddon) {
		handle.webLinksAddon = createWebLinksAddon();
		handle.terminal.loadAddon(handle.webLinksAddon);
	}
};

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
	return {
		status: statusMap[id] ?? '',
		message: messageMap[id] ?? '',
		health: healthMap[id] ?? 'unknown',
		healthMessage: healthMessageMap[id] ?? '',
		renderer: rendererMap[id] ?? 'unknown',
		rendererMode: rendererModeMap[id] ?? 'webgl',
		sessiondAvailable,
		sessiondChecked,
		debugEnabled,
		debugStats: { ...stats },
	};
};

const ensureStore = (id: string): Writable<TerminalViewState> => {
	let store = terminalStores.get(id);
	if (!store) {
		store = writable(buildState(id));
		terminalStores.set(id, store);
	}
	return store;
};

const emitState = (id: string): void => {
	const store = ensureStore(id);
	store.set(buildState(id));
};

const emitAllStates = (): void => {
	for (const id of terminalStores.keys()) {
		emitState(id);
	}
};

const subscribeEvent = <T>(event: string, handler: EventHandler<T>): (() => void) => {
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
		if (current.handlers.size !== 0) {
			return;
		}
		if (current.bound) {
			EventsOff(event);
		}
		eventRegistry.delete(event);
	};
};

const scheduleTerminalDispose = (id: string): void => {
	if (!id) return;
	if (attachedTerminals.has(id)) return;
	if (disposeTimers.has(id)) return;
	const timer = window.setTimeout(() => {
		disposeTimers.delete(id);
		if (attachedTerminals.has(id)) return;
		disposeTerminalResources(id);
	}, DISPOSE_TTL_MS);
	disposeTimers.set(id, timer);
};

const cancelTerminalDispose = (id: string): void => {
	const timer = disposeTimers.get(id);
	if (!timer) return;
	window.clearTimeout(timer);
	disposeTimers.delete(id);
};

const markTerminalAttached = (id: string): void => {
	if (!id) return;
	attachedTerminals.add(id);
	cancelTerminalDispose(id);
};

const markTerminalDetached = (id: string): void => {
	if (!id) return;
	attachedTerminals.delete(id);
	scheduleTerminalDispose(id);
};

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
	clearTimeoutMap(startupTimers, id);
	clearTimeoutMap(bootstrapFetchTimers, id);
	clearTimeoutMap(pendingRenderCheck, id);
	clearTimeoutMap(ackTimers, id);
	clearTimeoutMap(fitStabilizers, id);
	clearTimeoutMap(reattachTimers, id);
	clearTimeoutMap(resizeTimers, id);
	clearTimeoutMap(pendingHealthCheck, id);
	resetSessionState(id);
	terminalStores.delete(id);
	outputQueues.delete(id);
	replayState.delete(id);
	pendingReplayOutput.delete(id);
	pendingReplayKitty.delete(id);
	bootstrapHandled.delete(id);
	lastDims.delete(id);
	statsMap.delete(id);
	renderStatsMap.delete(id);
	pendingInput.delete(id);
	pendingRedraw.delete(id);
	renderCheckLogged.delete(id);
	reopenAttempted.delete(id);
	pendingAckBytes.delete(id);
	initialCreditMap.delete(id);
	initialCreditSent.delete(id);
	startInFlight.delete(id);
	startedSessions.delete(id);
	initTokens.delete(id);
	const observer = resizeObservers.get(id);
	if (observer) {
		observer.disconnect();
	}
	resizeObservers.delete(id);
	statusMap = deleteRecordKey(statusMap, id);
	messageMap = deleteRecordKey(messageMap, id);
	inputMap = deleteRecordKey(inputMap, id);
	healthMap = deleteRecordKey(healthMap, id);
	healthMessageMap = deleteRecordKey(healthMessageMap, id);
	rendererMap = deleteRecordKey(rendererMap, id);
	rendererModeMap = deleteRecordKey(rendererModeMap, id);
	modeMap = deleteRecordKey(modeMap, id);
	suppressMouseUntil = deleteRecordKey(suppressMouseUntil, id);
	mouseInputTail = deleteRecordKey(mouseInputTail, id);
};

const disposeTerminalResources = (id: string): void => {
	if (!id) return;
	cancelTerminalDispose(id);
	attachedTerminals.delete(id);
	const handle = terminalHandles.get(id);
	if (handle) {
		handle.dataDisposable?.dispose();
		handle.binaryDisposable?.dispose();
		if (handle.oscDisposables) {
			for (const disposable of handle.oscDisposables) {
				disposable.dispose();
			}
		}
		if (handle.kittyDisposables) {
			for (const disposable of handle.kittyDisposables) {
				disposable.dispose();
			}
		}
		handle.clipboardAddon?.dispose();
		handle.webLinksAddon?.dispose();
		handle.unicode11Addon?.dispose();
		handle.webglAddon?.dispose();
		handle.terminal.dispose();
	}
	terminalHandles.delete(id);
	destroyTerminalState(id);
};

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
	healthMap = { ...healthMap, [id]: state };
	if (message) {
		healthMessageMap = { ...healthMessageMap, [id]: message };
	} else if (healthMessageMap[id]) {
		healthMessageMap = { ...healthMessageMap, [id]: '' };
	}
	emitState(id);
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

const createKittyState = (): KittyState => ({
	images: new Map(),
	placements: new Map(),
});

const createKittyOverlay = (): KittyOverlay => {
	const underlay = document.createElement('canvas');
	const overlay = document.createElement('canvas');
	const ctxUnder = underlay.getContext('2d');
	const ctxOver = overlay.getContext('2d');
	if (!ctxUnder || !ctxOver) {
		throw new Error('Unable to initialize kitty overlay canvas.');
	}
	underlay.className = 'kitty-underlay';
	overlay.className = 'kitty-overlay';
	return {
		underlay,
		overlay,
		ctxUnder,
		ctxOver,
		cellWidth: 0,
		cellHeight: 0,
		dpr: window.devicePixelRatio || 1,
		renderScheduled: false,
	};
};

const ensureKittyOverlay = (handle: TerminalHandle, id: string): void => {
	if (!handle.kittyOverlay) {
		try {
			handle.kittyOverlay = createKittyOverlay();
			handle.container.append(handle.kittyOverlay.underlay, handle.kittyOverlay.overlay);
		} catch {
			handle.kittyOverlay = undefined;
			return;
		}
	}
	resizeKittyOverlay(handle);
	scheduleKittyRender(id);
};

const resizeKittyOverlay = (handle: TerminalHandle): void => {
	if (!handle.kittyOverlay || !handle.container) return;
	const rect = handle.container.getBoundingClientRect();
	const dpr = window.devicePixelRatio || 1;
	if (rect.width <= 0 || rect.height <= 0) return;
	handle.kittyOverlay.dpr = dpr;
	handle.kittyOverlay.underlay.width = rect.width * dpr;
	handle.kittyOverlay.underlay.height = rect.height * dpr;
	handle.kittyOverlay.overlay.width = rect.width * dpr;
	handle.kittyOverlay.overlay.height = rect.height * dpr;
	handle.kittyOverlay.underlay.style.width = `${rect.width}px`;
	handle.kittyOverlay.underlay.style.height = `${rect.height}px`;
	handle.kittyOverlay.overlay.style.width = `${rect.width}px`;
	handle.kittyOverlay.overlay.style.height = `${rect.height}px`;
	const cols = Math.max(handle.terminal.cols, 1);
	const rows = Math.max(handle.terminal.rows, 1);
	handle.kittyOverlay.cellWidth = rect.width / cols;
	handle.kittyOverlay.cellHeight = rect.height / rows;
};

const scheduleKittyRender = (id: string): void => {
	const handle = terminalHandles.get(id);
	if (!handle?.kittyOverlay || handle.kittyOverlay.renderScheduled) return;
	handle.kittyOverlay.renderScheduled = true;
	requestAnimationFrame(() => {
		const current = terminalHandles.get(id);
		if (!current?.kittyOverlay) return;
		current.kittyOverlay.renderScheduled = false;
		renderKittyOverlay(id);
	});
};

const renderKittyOverlay = (id: string): void => {
	const handle = terminalHandles.get(id);
	if (!handle?.kittyOverlay) return;
	const overlay = handle.kittyOverlay;
	overlay.ctxUnder.clearRect(0, 0, overlay.underlay.width, overlay.underlay.height);
	overlay.ctxOver.clearRect(0, 0, overlay.overlay.width, overlay.overlay.height);
	if (!handle.kittyState) return;
	for (const placement of handle.kittyState.placements.values()) {
		const image = handle.kittyState.images.get(placement.imageId);
		if (!image || !image.bitmap) continue;
		const target = placement.z >= 0 ? overlay.ctxOver : overlay.ctxUnder;
		const x = (placement.col - 1) * overlay.cellWidth * overlay.dpr;
		const y = (placement.row - 1) * overlay.cellHeight * overlay.dpr;
		const w = placement.cols * overlay.cellWidth * overlay.dpr;
		const h = placement.rows * overlay.cellHeight * overlay.dpr;
		target.drawImage(image.bitmap, x, y, w, h);
	}
};

const clearKittyOverlay = (handle: TerminalHandle): void => {
	if (!handle.kittyOverlay) return;
	handle.kittyOverlay.ctxUnder.clearRect(
		0,
		0,
		handle.kittyOverlay.underlay.width,
		handle.kittyOverlay.underlay.height,
	);
	handle.kittyOverlay.ctxOver.clearRect(
		0,
		0,
		handle.kittyOverlay.overlay.width,
		handle.kittyOverlay.overlay.height,
	);
};

const decodeBase64 = (input: string | number[] | Uint8Array): Uint8Array => {
	if (!input) return new Uint8Array();
	if (input instanceof Uint8Array) {
		return input;
	}
	if (Array.isArray(input)) {
		return Uint8Array.from(input);
	}
	const binary = atob(input);
	const bytes = new Uint8Array(binary.length);
	for (let i = 0; i < binary.length; i += 1) {
		bytes[i] = binary.charCodeAt(i);
	}
	return bytes;
};

const applyKittyEvent = async (id: string, event: KittyEventPayload): Promise<void> => {
	const handle = terminalHandles.get(id);
	if (!handle) return;
	if (!handle.kittyState) {
		handle.kittyState = createKittyState();
	}
	if (event.kind === 'clear') {
		handle.kittyState.images.clear();
		handle.kittyState.placements.clear();
		clearKittyOverlay(handle);
		return;
	}
	if (event.kind === 'snapshot' && event.snapshot) {
		handle.kittyState.images.clear();
		handle.kittyState.placements.clear();
		const images = event.snapshot.images ?? [];
		for (const image of images) {
			if (!image?.id || !image.data) continue;
			const data = decodeBase64(image.data);
			handle.kittyState.images.set(image.id, {
				id: image.id,
				format: image.format ?? 'png',
				width: image.width ?? 0,
				height: image.height ?? 0,
				data,
			});
		}
		const placements = event.snapshot.placements ?? [];
		for (const placement of placements) {
			if (!placement) continue;
			handle.kittyState.placements.set(String(placement.id), {
				id: placement.id ?? 0,
				imageId: placement.imageId ?? '',
				row: placement.row ?? 0,
				col: placement.col ?? 0,
				rows: placement.rows ?? 0,
				cols: placement.cols ?? 0,
				x: placement.x ?? 0,
				y: placement.y ?? 0,
				z: placement.z ?? 0,
			});
		}
	}
	if (event.kind === 'image' && event.image?.id && event.image.data) {
		const data = decodeBase64(event.image.data);
		handle.kittyState.images.set(event.image.id, {
			id: event.image.id,
			format: event.image.format ?? 'png',
			width: event.image.width ?? 0,
			height: event.image.height ?? 0,
			data,
		});
	}
	if (event.kind === 'placement' && event.placement) {
		handle.kittyState.placements.set(String(event.placement.id ?? 0), {
			id: event.placement.id ?? 0,
			imageId: event.placement.imageId ?? '',
			row: event.placement.row ?? 0,
			col: event.placement.col ?? 0,
			rows: event.placement.rows ?? 0,
			cols: event.placement.cols ?? 0,
			x: event.placement.x ?? 0,
			y: event.placement.y ?? 0,
			z: event.placement.z ?? 0,
		});
	}
	if (event.kind === 'delete' && event.delete) {
		if (event.delete.all) {
			handle.kittyState.images.clear();
			handle.kittyState.placements.clear();
		} else {
			if (event.delete.imageId) {
				handle.kittyState.images.delete(event.delete.imageId);
			}
			if (event.delete.placementId) {
				handle.kittyState.placements.delete(String(event.delete.placementId));
			}
		}
	}
	for (const image of handle.kittyState.images.values()) {
		if (image.bitmap || image.decoding) continue;
		if (!image.data || image.data.length === 0) continue;
		const blobData = image.data instanceof Uint8Array ? Uint8Array.from(image.data) : image.data;
		image.decoding = createImageBitmap(new Blob([blobData]))
			.then((bitmap) => {
				image.bitmap = bitmap;
			})
			.catch(() => undefined)
			.finally(() => {
				image.decoding = undefined;
			});
	}
	if (handle.kittyOverlay) {
		resizeKittyOverlay(handle);
	}
	scheduleKittyRender(id);
};

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
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	void WriteWorkspaceTerminal(workspaceId, terminalId, filtered).catch((error: unknown) => {
		pendingInput.set(id, (pendingInput.get(id) ?? '') + filtered);
		startedSessions.delete(id);
		if (
			typeof error === 'string' &&
			(error.includes('session not found') ||
				error.includes('terminal not started') ||
				error.includes('terminal not found'))
		) {
			resetTerminalInstance(id);
			void beginTerminal(id, true);
		}
		if (error instanceof Error) {
			const message = error.message;
			if (
				message.includes('session not found') ||
				message.includes('terminal not started') ||
				message.includes('terminal not found')
			) {
				resetTerminalInstance(id);
				void beginTerminal(id, true);
			}
		}
		const handle = terminalHandles.get(id);
		handle?.terminal.write(`\r\n[workset] write failed: ${String(error)}`);
	});
};

const createTerminal = (): Terminal => {
	const themeBackground = getToken('--panel-strong', '#111c29');
	const themeForeground = getToken('--text', '#eef3f9');
	const themeCursor = getToken('--accent', '#2d8cff');
	const themeSelection = getToken('--accent', '#2d8cff');
	const fontMono = getToken('--font-mono', '"JetBrains Mono", Menlo, Consolas, monospace');

	const options: ITerminalOptions & ITerminalInitOnlyOptions & { rendererType?: string } = {
		fontFamily: fontMono,
		fontSize: currentFontSize,
		lineHeight: 1.4,
		cursorBlink: true,
		scrollback: 4000,
		allowProposedApi: true,
		theme: {
			background: themeBackground,
			foreground: themeForeground,
			cursor: themeCursor,
			selectionBackground: themeSelection,
		},
	};
	return new Terminal(options);
};

const attachTerminal = (
	id: string,
	container: HTMLDivElement | null,
	active: boolean,
): TerminalHandle => {
	let handle = terminalHandles.get(id);
	if (!handle) {
		const terminal = createTerminal();
		const fitAddon = new FitAddon();
		terminal.loadAddon(fitAddon);
		const unicode11Addon = new Unicode11Addon();
		terminal.loadAddon(unicode11Addon);
		terminal.unicode.activeVersion = '11';
		const clipboardAddon = new ClipboardAddon(createClipboardBase64(), createClipboardProvider());
		terminal.loadAddon(clipboardAddon);
		terminal.attachCustomKeyEventHandler((event) => {
			if (event.key === 'Enter' && event.shiftKey) {
				inputMap = { ...inputMap, [id]: true };
				void beginTerminal(id);
				sendInput(id, '\x0a');
				return false;
			}
			return true;
		});
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
		const host = document.createElement('div');
		host.className = 'terminal-instance';
		handle = {
			terminal,
			fitAddon,
			dataDisposable,
			binaryDisposable,
			container: host,
			kittyState: createKittyState(),
			clipboardAddon,
			unicode11Addon,
		};
		terminalHandles.set(id, handle);
		syncWebLinksForMode(id);
		handle.oscDisposables = registerOscHandlers(id, terminal);
		if (!modeMap[id]) {
			modeMap = {
				...modeMap,
				[id]: { altScreen: false, mouse: false, mouseSGR: false, mouseEncoding: 'x10' },
			};
		}
	}
	if (handle.dataDisposable) {
		handle.dataDisposable.dispose();
	}
	if (handle.binaryDisposable) {
		handle.binaryDisposable.dispose();
	}
	handle.dataDisposable = handle.terminal.onData((data) => {
		inputMap = { ...inputMap, [id]: true };
		void beginTerminal(id);
		captureCpr(id, data);
		sendInput(id, data);
	});
	handle.binaryDisposable = handle.terminal.onBinary((data) => {
		if (!data) return;
		inputMap = { ...inputMap, [id]: true };
		void beginTerminal(id);
		sendInput(id, data);
	});
	if (container) {
		if (container.firstChild !== handle.container) {
			container.replaceChildren(handle.container);
		}
		const terminalElement = handle.terminal.element;
		const needsOpen = !terminalElement || terminalElement.parentElement !== handle.container;
		if (needsOpen) {
			handle.container.replaceChildren();
			handle.terminal.open(handle.container);
			ensureKittyOverlay(handle, id);
			void loadRendererAddon(handle, id);
			if (typeof document !== 'undefined' && document.fonts?.ready) {
				document.fonts.ready
					.then(() => {
						const current = terminalHandles.get(id);
						if (!current) return;
						current.fitAddon.fit();
						resizeKittyOverlay(current);
						const updated = current.fitAddon.proposeDimensions();
						if (updated) {
							const workspaceId = getWorkspaceId(id);
							const terminalId = getTerminalId(id);
							if (workspaceId && terminalId) {
								void ResizeWorkspaceTerminal(
									workspaceId,
									terminalId,
									updated.cols,
									updated.rows,
								).catch(() => undefined);
							}
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
			const workspaceId = getWorkspaceId(id);
			const terminalId = getTerminalId(id);
			if (workspaceId && terminalId) {
				void ResizeWorkspaceTerminal(workspaceId, terminalId, dims.cols, dims.rows).catch(
					() => undefined,
				);
			}
		}
		if (active) {
			handle.terminal.focus();
		}
		flushOutput(id, false);
		markTerminalAttached(id);
	}
	return handle;
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
				fitTerminal(id, startedSessions.has(id));
			}, RESIZE_DEBOUNCE_MS),
		);
	});
	observer.observe(container);
	resizeObservers.set(id, observer);
};

const fitTerminal = (id: string, resizeSession: boolean): void => {
	const handle = terminalHandles.get(id);
	if (!handle) return;
	const buffer = handle.terminal.buffer.active;
	const wasAtBottom = buffer.baseY === buffer.viewportY;
	handle.fitAddon.fit();
	resizeKittyOverlay(handle);
	if (wasAtBottom) {
		handle.terminal.scrollToBottom();
	}
	forceRedraw(id);
	if (!resizeSession) return;
	const dims = handle.fitAddon.proposeDimensions();
	if (dims) {
		const workspaceId = getWorkspaceId(id);
		const terminalId = getTerminalId(id);
		if (workspaceId && terminalId) {
			void ResizeWorkspaceTerminal(workspaceId, terminalId, dims.cols, dims.rows).catch(
				() => undefined,
			);
		}
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
		fitTerminal(id, startedSessions.has(id));
		if (stableCount < 2 && attempts < 5) {
			attempts += 1;
			fitStabilizers.set(id, window.setTimeout(run, 80 + attempts * 30));
		}
	};
	fitStabilizers.set(id, window.setTimeout(run, 60));
	logDebug(id, 'fit', { reason });
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
			clearReattachTimer(id);
			void ensureSessionActive(id);
		}, 240),
	);
	logDebug(id, 'reattach', { reason });
};

const clearReattachTimer = (id: string): void => {
	const timer = reattachTimers.get(id);
	if (timer) {
		window.clearTimeout(timer);
		reattachTimers.delete(id);
	}
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

const setReplayState = (id: string, state: 'idle' | 'replaying' | 'live'): void => {
	replayState.set(id, state);
	if (state !== 'live') {
		return;
	}
	const pending = pendingReplayOutput.get(id) ?? [];
	if (pending.length > 0) {
		pendingReplayOutput.delete(id);
		for (const chunk of pending) {
			enqueueOutput(id, chunk.data, chunk.bytes);
		}
	}
	const kitty = pendingReplayKitty.get(id) ?? [];
	if (kitty.length > 0) {
		if (terminalHandles.has(id)) {
			pendingReplayKitty.delete(id);
			for (const event of kitty) {
				void applyKittyEvent(id, event);
			}
		}
	}
	flushOutput(id, true);
	forceRedraw(id);
	grantInitialCredit(id);
	flushAck(id);
};

const resetSessionState = (id: string): void => {
	replayState.set(id, 'idle');
	pendingReplayOutput.delete(id);
	pendingReplayKitty.delete(id);
	outputQueues.delete(id);
	pendingAckBytes.delete(id);
	initialCreditMap.delete(id);
	initialCreditSent.delete(id);
	bootstrapHandled.delete(id);
	const bootstrapTimer = bootstrapFetchTimers.get(id);
	if (bootstrapTimer) {
		window.clearTimeout(bootstrapTimer);
	}
	bootstrapFetchTimers.delete(id);
	const ackTimer = ackTimers.get(id);
	if (ackTimer) {
		window.clearTimeout(ackTimer);
	}
	ackTimers.delete(id);
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
	const handle = terminalHandles.get(id);
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
	void loadRendererAddon(handle, id);
};

const updateStatsLastOutput = (id: string): void => {
	updateStats(id, (stats) => {
		stats.lastOutputAt = Date.now();
	});
};

const noteRender = (id: string): void => {
	const stats = renderStatsMap.get(id) ?? { lastRenderAt: 0, renderCount: 0 };
	stats.lastRenderAt = Date.now();
	stats.renderCount += 1;
	renderStatsMap.set(id, stats);
};

const scheduleStartupTimeout = (id: string): void => {
	const existing = startupTimers.get(id);
	if (existing) {
		window.clearTimeout(existing);
	}
	startupTimers.set(
		id,
		window.setTimeout(() => {
			startupTimers.delete(id);
			if (startedSessions.has(id)) return;
			statusMap = { ...statusMap, [id]: 'error' };
			messageMap = { ...messageMap, [id]: 'Terminal startup timed out.' };
			setHealth(id, 'stale', 'Terminal startup timed out.');
			pendingInput.delete(id);
			emitState(id);
		}, STARTUP_OUTPUT_TIMEOUT_MS),
	);
};

const clearStartupTimeout = (id: string): void => {
	const timer = startupTimers.get(id);
	if (!timer) return;
	window.clearTimeout(timer);
	startupTimers.delete(id);
};

const logDebug = (id: string, event: string, details: Record<string, unknown>): void => {
	syncDebugEnabled();
	if (!debugEnabled) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	void logTerminalDebug(workspaceId, terminalId, event, JSON.stringify(details));
};

const forceRedraw = (id: string): void => {
	const handle = terminalHandles.get(id);
	if (!handle) return;
	handle.terminal.refresh(0, handle.terminal.rows - 1);
};

const nudgeTerminalRedraw = (id: string): void => {
	if (pendingRedraw.has(id)) return;
	const handle = terminalHandles.get(id);
	if (!handle) return;
	const dims = handle.fitAddon.proposeDimensions();
	if (!dims) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) {
		handle.terminal.write('');
		return;
	}
	const cols = Math.max(2, dims.cols);
	const rows = Math.max(1, dims.rows);
	const nudgeCols = cols + 1;
	pendingRedraw.add(id);
	void ResizeWorkspaceTerminal(workspaceId, terminalId, nudgeCols, rows).catch(() => undefined);
	logDebug(id, 'redraw_nudge', { cols, rows, nudgeCols });
	window.setTimeout(() => {
		void ResizeWorkspaceTerminal(workspaceId, terminalId, cols, rows).catch(() => undefined);
		pendingRedraw.delete(id);
	}, 60);
};

const hasVisibleContent = (terminal: Terminal): boolean => {
	const buffer = terminal.buffer.active;
	if (buffer.length === 0) return false;
	const line = buffer.getLine(buffer.length - 1);
	return !!line && line.translateToString().trim().length > 0;
};

const isWorkspaceMismatch = (
	key: string,
	payloadWorkspaceId?: string,
	payloadTerminalId?: string,
): boolean => {
	if (!payloadWorkspaceId || !payloadTerminalId) return false;
	const context = terminalContexts.get(key);
	if (!context?.workspaceId || !context.terminalId) return false;
	if (context.workspaceId === payloadWorkspaceId && context.terminalId === payloadTerminalId) {
		return false;
	}
	logDebug(key, 'workspace_mismatch', {
		payloadWorkspaceId,
		payloadTerminalId,
		contextWorkspaceId: context.workspaceId,
		contextTerminalId: context.terminalId,
	});
	return true;
};

const setReplayStateDone = (id: string): void => {
	setReplayState(id, 'live');
};

const coerceKittySnapshot = (
	value: TerminalBootstrapPayload['kitty'],
): KittyEventPayload['snapshot'] | null => {
	if (!value) return null;
	const images = Array.isArray(value.images)
		? (value.images as KittyEventPayload['image'][])
		: undefined;
	const placements = Array.isArray(value.placements)
		? (value.placements as KittyEventPayload['placement'][])
		: undefined;
	if (!images && !placements) return null;
	return { images, placements };
};

const handleBootstrapPayload = (payload: TerminalBootstrapPayload): void => {
	const terminalId = payload.terminalId;
	const workspaceId = payload.workspaceId;
	if (!terminalId || !workspaceId) return;
	const id = buildTerminalKey(workspaceId, terminalId);
	if (!id) return;
	if (isWorkspaceMismatch(id, workspaceId, terminalId)) return;
	if (bootstrapHandled.get(id)) {
		logDebug(id, 'bootstrap_duplicate', { source: payload.source ?? 'event' });
		return;
	}
	if (!inputMap[id]) {
		inputMap = { ...inputMap, [id]: true };
	}
	if (payload.safeToReplay === false) {
		setReplayStateDone(id);
		bootstrapHandled.set(id, true);
		return;
	}
	setReplayState(id, 'replaying');
	if (payload.snapshot) {
		enqueueOutput(id, payload.snapshot, countBytes(payload.snapshot));
	}
	if (payload.backlog) {
		enqueueOutput(id, payload.backlog, countBytes(payload.backlog));
	}
	const kittySnapshot = coerceKittySnapshot(payload.kitty);
	if (kittySnapshot) {
		const kittyEvent: KittyEventPayload = {
			kind: 'snapshot',
			snapshot: kittySnapshot,
		};
		if (!terminalHandles.has(id)) {
			const pending = pendingReplayKitty.get(id) ?? [];
			pending.push(kittyEvent);
			pendingReplayKitty.set(id, pending);
		} else {
			void applyKittyEvent(id, kittyEvent);
		}
	}
	if (payload.backlogTruncated) {
		setHealth(id, 'ok', 'Backlog truncated; showing latest output.');
	}
	initialCreditMap.set(id, payload.initialCredit ?? INITIAL_STREAM_CREDIT);
	bootstrapHandled.set(id, true);
	logDebug(id, 'bootstrap', {
		source: payload.source,
		snapshotSource: payload.snapshotSource,
		backlogSource: payload.backlogSource,
		backlogTruncated: payload.backlogTruncated,
	});
};

const handleBootstrapDonePayload = (payload: TerminalBootstrapDonePayload): void => {
	const terminalId = payload.terminalId;
	const workspaceId = payload.workspaceId;
	if (!terminalId || !workspaceId) return;
	const id = buildTerminalKey(workspaceId, terminalId);
	if (!id) return;
	if (isWorkspaceMismatch(id, workspaceId, terminalId)) return;
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
	setReplayStateDone(id);
	emitState(id);
};

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
			noteRender(id);
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

const scheduleAck = (id: string): void => {
	if (ackTimers.has(id)) return;
	ackTimers.set(
		id,
		window.setTimeout(() => {
			ackTimers.delete(id);
			flushAck(id);
		}, ACK_FLUSH_DELAY_MS),
	);
};

const flushAck = (id: string): void => {
	const bytes = pendingAckBytes.get(id);
	if (!bytes || bytes <= 0) return;
	pendingAckBytes.delete(id);
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	void AckWorkspaceTerminal(workspaceId, terminalId, bytes).catch(() => undefined);
};

const grantInitialCredit = (id: string): void => {
	if (initialCreditSent.has(id)) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	initialCreditSent.add(id);
	const credit = initialCreditMap.get(id) ?? INITIAL_STREAM_CREDIT;
	void AckWorkspaceTerminal(workspaceId, terminalId, credit).catch(() => {
		initialCreditSent.delete(id);
	});
};

const recordAckBytes = (id: string, bytes: number): void => {
	const total = (pendingAckBytes.get(id) ?? 0) + bytes;
	pendingAckBytes.set(id, total);
	if (total >= ACK_BATCH_BYTES) {
		flushAck(id);
		return;
	}
	scheduleAck(id);
};

const noteRenderStats = (id: string): void => {
	let stats = renderStatsMap.get(id);
	if (!stats) {
		stats = { lastRenderAt: 0, renderCount: 0 };
		renderStatsMap.set(id, stats);
	}
	const now = Date.now();
	if (now - stats.lastRenderAt > RENDER_CHECK_DELAY_MS) {
		scheduleRenderCheck(id);
	}
};

const scheduleRenderCheck = (id: string): void => {
	const existing = pendingRenderCheck.get(id);
	if (existing) {
		window.clearTimeout(existing);
	}
	let stats = renderStatsMap.get(id);
	if (!stats) {
		stats = { lastRenderAt: 0, renderCount: 0 };
		renderStatsMap.set(id, stats);
	}
	pendingRenderCheck.set(
		id,
		window.setTimeout(() => {
			pendingRenderCheck.delete(id);
			const stats = renderStatsMap.get(id) ?? { lastRenderAt: 0, renderCount: 0 };
			const now = Date.now();
			if (now - stats.lastRenderAt < RENDER_CHECK_DELAY_MS) return;
			if (!renderCheckLogged.has(id)) {
				renderCheckLogged.add(id);
				logDebug(id, 'render_stall', { lastRenderAt: stats.lastRenderAt });
			}
			const handle = terminalHandles.get(id);
			if (handle?.container && !reopenAttempted.has(id)) {
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
			forceRedraw(id);
			window.setTimeout(() => {
				const current = terminalHandles.get(id);
				if (!current) return;
				if (!hasVisibleContent(current.terminal)) {
					nudgeTerminalRedraw(id);
				}
			}, RENDER_RECOVERY_DELAY_MS);
		}, RENDER_CHECK_DELAY_MS),
	);
};

const scheduleRenderHealthCheck = (id: string, payloadBytes: number): void => {
	if (!id || payloadBytes <= 0 || pendingRenderCheck.has(id)) return;
	const startedAt = Date.now();
	pendingRenderCheck.set(
		id,
		window.setTimeout(() => {
			pendingRenderCheck.delete(id);
			const handle = terminalHandles.get(id);
			if (!handle) return;
			const stats = renderStatsMap.get(id);
			if (stats && stats.lastRenderAt >= startedAt) return;
			handle.fitAddon.fit();
			resizeKittyOverlay(handle);
			window.setTimeout(() => {
				const updated = renderStatsMap.get(id);
				if (updated && updated.lastRenderAt >= startedAt) return;
				if (!hasVisibleContent(handle.terminal)) {
					nudgeTerminalRedraw(id);
				}
				logDebug(id, 'render_health_check', {
					rendered: updated ? updated.lastRenderAt >= startedAt : false,
				});
			}, RENDER_RECOVERY_DELAY_MS);
		}, RENDER_CHECK_DELAY_MS),
	);
};

const requestHealthCheck = (id: string): void => {
	const existing = pendingHealthCheck.get(id);
	if (existing) {
		window.clearTimeout(existing);
	}
	setHealth(id, 'checking', 'Checking session health…');
	pendingHealthCheck.set(
		id,
		window.setTimeout(() => {
			pendingHealthCheck.delete(id);
			if (!startedSessions.has(id)) {
				setHealth(id, 'stale', 'Session not active.');
			}
		}, HEALTH_TIMEOUT_MS),
	);
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
	if (startedSessions.has(id) || startInFlight.has(id)) return;
	if (sessiondAvailable !== true) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	try {
		const status = await fetchWorkspaceTerminalStatus(workspaceId, terminalId);
		if (status?.active) {
			startedSessions.add(id);
			statusMap = { ...statusMap, [id]: 'ready' };
			messageMap = { ...messageMap, [id]: '' };
			setHealth(id, 'ok', 'Session resumed.');
			inputMap = { ...inputMap, [id]: true };
			if (!rendererMap[id]) {
				rendererMap = { ...rendererMap, [id]: 'unknown' };
			}
			if (!rendererModeMap[id]) {
				rendererModeMap = { ...rendererModeMap, [id]: rendererPreference };
			}
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
		const payload = await fetchTerminalBootstrap(workspaceId, terminalId);
		if (bootstrapHandled.get(id)) return;
		handleBootstrapPayload(payload);
		handleBootstrapDonePayload({ workspaceId, terminalId });
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
	if (!id || startedSessions.has(id) || startInFlight.has(id)) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	startInFlight.add(id);
	resetSessionState(id);
	bootstrapHandled.set(id, false);
	setReplayState(id, 'replaying');
	if (!quiet) {
		statusMap = { ...statusMap, [id]: 'starting' };
		messageMap = { ...messageMap, [id]: 'Waiting for shell output…' };
		setHealth(id, 'unknown');
		inputMap = { ...inputMap, [id]: false };
		scheduleStartupTimeout(id);
		emitState(id);
	}
	try {
		await StartWorkspaceTerminal(workspaceId, terminalId);
		startedSessions.add(id);
		const queued = pendingInput.get(id);
		if (queued) {
			pendingInput.delete(id);
			await WriteWorkspaceTerminal(workspaceId, terminalId, queued);
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
		statusMap = { ...statusMap, [id]: 'error' };
		messageMap = { ...messageMap, [id]: String(error) };
		setHealth(id, 'stale', 'Failed to start terminal.');
		clearStartupTimeout(id);
		pendingInput.delete(id);
		const handle = terminalHandles.get(id);
		handle?.terminal.write(`\r\n[workset] failed to start terminal: ${String(error)}`);
		emitState(id);
	} finally {
		startInFlight.delete(id);
	}
};

const ensureStream = async (id: string): Promise<void> => {
	if (!id || startInFlight.has(id)) return;
	const workspaceId = getWorkspaceId(id);
	const terminalId = getTerminalId(id);
	if (!workspaceId || !terminalId) return;
	startInFlight.add(id);
	try {
		await StartWorkspaceTerminal(workspaceId, terminalId);
		startedSessions.add(id);
	} catch (error) {
		logDebug(id, 'ensure_stream_failed', { error: String(error) });
	} finally {
		startInFlight.delete(id);
	}
};

const loadTerminalDefaults = async (): Promise<void> => {
	rendererPreference = 'webgl';
	let nextDebugPreference = debugOverlayPreference;
	try {
		const settings = await fetchSettings();
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
		const status = await fetchSessiondStatus();
		sessiondAvailable = status?.available ?? false;
	} catch {
		sessiondAvailable = false;
	} finally {
		sessiondChecked = true;
		emitAllStates();
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

const loadRendererAddon = async (handle: TerminalHandle, id: string): Promise<void> => {
	rendererModeMap = { ...rendererModeMap, [id]: 'webgl' };
	try {
		if (!handle.webglAddon) {
			handle.webglAddon = new WebglAddon();
			handle.terminal.loadAddon(handle.webglAddon);
		}
		rendererMap = { ...rendererMap, [id]: 'webgl' };
	} catch (error) {
		rendererMap = { ...rendererMap, [id]: 'unknown' };
		statusMap = { ...statusMap, [id]: 'error' };
		messageMap = { ...messageMap, [id]: 'WebGL renderer unavailable.' };
		setHealth(id, 'stale', 'WebGL renderer unavailable.');
		logDebug(id, 'renderer_webgl_failed', { error: String(error) });
	}
	emitState(id);
};

const ensureListener = (): void => {
	if (!listeners.has('terminal:data')) {
		const handler = (payload: TerminalPayload): void => {
			const terminalId = payload.terminalId;
			const workspaceId = payload.workspaceId;
			if (!terminalId || !workspaceId) return;
			const id = buildTerminalKey(workspaceId, terminalId);
			if (!id) return;
			if (isWorkspaceMismatch(id, workspaceId, terminalId)) return;
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
			recordAckBytes(id, bytes);
			updateStats(id, (stats) => {
				stats.bytesIn += bytes;
			});
			noteRenderStats(id);
		};
		unsubscribeHandlers.push(subscribeEvent('terminal:data', handler));
		listeners.add('terminal:data');
	}
	if (!listeners.has('terminal:bootstrap')) {
		const handler = (payload: TerminalBootstrapPayload): void => {
			handleBootstrapPayload(payload);
		};
		unsubscribeHandlers.push(subscribeEvent('terminal:bootstrap', handler));
		listeners.add('terminal:bootstrap');
	}
	if (!listeners.has('terminal:bootstrap_done')) {
		const handler = (payload: TerminalBootstrapDonePayload): void => {
			handleBootstrapDonePayload(payload);
		};
		unsubscribeHandlers.push(subscribeEvent('terminal:bootstrap_done', handler));
		listeners.add('terminal:bootstrap_done');
	}
	if (!listeners.has('terminal:lifecycle')) {
		const handler = (payload: TerminalLifecyclePayload): void => {
			const terminalId = payload.terminalId;
			const workspaceId = payload.workspaceId;
			if (!terminalId || !workspaceId) return;
			const id = buildTerminalKey(workspaceId, terminalId);
			if (!id) return;
			if (isWorkspaceMismatch(id, workspaceId, terminalId)) return;
			if (payload.status === 'started') {
				startedSessions.add(id);
				statusMap = { ...statusMap, [id]: 'ready' };
				messageMap = { ...messageMap, [id]: '' };
				inputMap = { ...inputMap, [id]: true };
				clearStartupTimeout(id);
				setHealth(id, 'ok', 'Session started.');
				emitState(id);
				return;
			}
			if (payload.status === 'closed') {
				startedSessions.delete(id);
				statusMap = { ...statusMap, [id]: 'closed' };
				setHealth(id, 'stale', 'Session closed.');
				emitState(id);
				return;
			}
			if (payload.status === 'idle') {
				startedSessions.delete(id);
				statusMap = { ...statusMap, [id]: 'idle' };
				setHealth(id, 'unknown');
				emitState(id);
				return;
			}
			if (payload.status === 'error') {
				startedSessions.delete(id);
				statusMap = { ...statusMap, [id]: 'error' };
				messageMap = {
					...messageMap,
					[id]: payload.message ?? 'Terminal error',
				};
				setHealth(id, 'stale', payload.message ?? 'Terminal error');
				emitState(id);
			}
		};
		unsubscribeHandlers.push(subscribeEvent('terminal:lifecycle', handler));
		listeners.add('terminal:lifecycle');
	}
	if (!listeners.has('terminal:modes')) {
		const handler = (payload: TerminalModesPayload): void => {
			const terminalId = payload.terminalId;
			const workspaceId = payload.workspaceId;
			if (!terminalId || !workspaceId) return;
			const id = buildTerminalKey(workspaceId, terminalId);
			if (!id) return;
			if (isWorkspaceMismatch(id, workspaceId, terminalId)) return;
			modeMap = {
				...modeMap,
				[id]: {
					altScreen: payload.altScreen ?? false,
					mouse: payload.mouse ?? false,
					mouseSGR: payload.mouseSGR ?? false,
					mouseEncoding: payload.mouseEncoding ?? 'x10',
				},
			};
			syncWebLinksForMode(id);
		};
		unsubscribeHandlers.push(subscribeEvent('terminal:modes', handler));
		listeners.add('terminal:modes');
	}
	if (!listeners.has('terminal:kitty')) {
		const handler = (payload: TerminalKittyPayload): void => {
			const terminalId = payload.terminalId;
			const workspaceId = payload.workspaceId;
			if (!terminalId || !workspaceId) return;
			const id = buildTerminalKey(workspaceId, terminalId);
			if (!id) return;
			if (isWorkspaceMismatch(id, workspaceId, terminalId)) return;
			const replayStateValue = replayState.get(id) ?? 'unknown';
			const isLive = replayStateValue === 'live';
			if (!isLive || !terminalHandles.has(id)) {
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
	if (!listeners.has('sessiond:restarted')) {
		const handler = (): void => {
			sessiondChecked = false;
			void (async () => {
				await refreshSessiondStatus();
				if (sessiondAvailable !== true) return;
				for (const id of terminalContexts.keys()) {
					startedSessions.delete(id);
					startInFlight.delete(id);
					resetTerminalInstance(id);
					resetSessionState(id);
					noteMouseSuppress(id, 4000);
					void beginTerminal(id, true);
				}
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

const initTerminal = async (id: string): Promise<void> => {
	if (!id) return;
	const token = (initTokens.get(id) ?? 0) + 1;
	initTokens.set(id, token);
	ensureListener();
	if (!sessiondChecked) {
		await refreshSessiondStatus();
	}
	const ctx = getContext(id);
	attachTerminal(id, ctx?.container ?? null, ctx?.active ?? false);
	let resumed = false;
	if (sessiondAvailable === true) {
		try {
			const workspaceId = getWorkspaceId(id);
			const terminalId = getTerminalId(id);
			if (workspaceId && terminalId) {
				const status = await fetchWorkspaceTerminalStatus(workspaceId, terminalId);
				resumed = status?.active ?? false;
			}
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
		emitState(id);
		return;
	}
	if (token !== initTokens.get(id)) return;
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
		emitState(id);
	}
};

const ensureGlobals = (): void => {
	if (globalsInitialized) return;
	globalsInitialized = true;
	void loadTerminalDefaults();
	void refreshSessiondStatus();
	window.addEventListener('focus', () => {
		for (const id of attachedTerminals) {
			void ensureSessionActive(id);
		}
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
		scheduleReattachCheck(context.terminalKey, 'workspace_switch');
	}
	context.lastWorkspaceId = context.workspaceId;
	if (input.container) {
		attachTerminal(terminalKey, input.container, input.active);
		attachResizeObserver(terminalKey, input.container);
		if (input.active) {
			requestAnimationFrame(() => {
				fitTerminal(terminalKey, startedSessions.has(terminalKey));
				forceRedraw(terminalKey);
				const handle = terminalHandles.get(terminalKey);
				if (handle && !hasVisibleContent(handle.terminal)) {
					nudgeTerminalRedraw(terminalKey);
				}
			});
		}
	}
	void (async () => {
		await initTerminal(terminalKey);
		const current = terminalContexts.get(terminalKey);
		if (current?.container) {
			if (startedSessions.has(terminalKey)) {
				void ensureStream(terminalKey);
			} else if (statusMap[terminalKey] === 'standby') {
				await beginTerminal(terminalKey, !current.active);
			}
		}
		await ensureSessionActive(terminalKey);
		emitState(terminalKey);
	})();
};

export const detachTerminal = (workspaceId: string, terminalId: string): void => {
	const terminalKey = buildTerminalKey(workspaceId, terminalId);
	if (!terminalKey) return;
	markTerminalDetached(terminalKey);
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
		await stopWorkspaceTerminal(workspaceId, terminalId);
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
	cleanupListeners();
};

// Font size controls (VS Code style Cmd/Ctrl +/-)
const applyFontSizeToAllTerminals = (): void => {
	for (const [id, handle] of terminalHandles.entries()) {
		handle.terminal.options.fontSize = currentFontSize;
		// Refit terminal to recalculate dimensions with new font size.
		try {
			fitTerminal(id, startedSessions.has(id));
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
