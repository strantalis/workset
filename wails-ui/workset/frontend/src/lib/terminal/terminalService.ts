import type { Writable } from 'svelte/store';
import { terminalTransport } from './terminalTransport';
import { createTerminalInstance } from './terminalRenderer';
import { TerminalStateStore } from './terminalStateStore';
import { createTerminalLifecycle } from './terminalLifecycle';
import { createTerminalStreamOrchestrator } from './terminalStreamOrchestrator';
import { createTerminalResizeBridge } from './terminalResizeBridge';
import { createTerminalAttachState } from './terminalAttachRendererState';
import {
	createTerminalEventSubscriptions,
	type TerminalPayload,
} from './terminalEventSubscriptions';
import { type TerminalInstanceHandle } from './terminalInstanceManager';
import { createTerminalInputOrchestrator } from './terminalInputOrchestrator';
import { createTerminalViewportResizeController } from './terminalViewportResizeController';
import { createTerminalContextRegistry } from './terminalContextRegistry';
import { createTerminalSessionCoordinator } from './terminalSessionCoordinator';
import { createTerminalResourceLifecycle } from './terminalResourceLifecycle';
import { createTerminalServiceExports } from './terminalServiceExports';
import { createTerminalInstanceOrchestration } from './terminalInstanceOrchestration';
import { createTerminalDebugState } from './terminalDebugState';
import {
	clearLocalTerminalDebugPreference,
	createTerminalSessionBridge,
	createTerminalResourceLifecycleDeps,
	createTerminalSessionTransport,
	createTerminalSyncControllerDeps,
	ensureTerminalGlobals,
} from './terminalServiceDeps';
import { createTerminalServiceState } from './terminalServiceState';
import { createTerminalServiceRuntime } from './terminalServiceRuntime';

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
		lastOutputAt: number;
		lastCprAt: number;
	};
};

type TerminalHandle = TerminalInstanceHandle;

const terminalHandles = new Map<string, TerminalHandle>();
const terminalContextRegistry = createTerminalContextRegistry();
const terminalStores = new TerminalStateStore<TerminalViewState>();
const DISPOSE_TTL_MS = 10 * 60 * 1000;
const STREAM_REORDER_DELAY_MS = 8;
const STREAM_REORDER_FORCE_FLUSH_THRESHOLD = 24;
let globalsInitialized = false;
const terminalServiceState = createTerminalServiceState();
const { pendingInput } = terminalServiceState;
const streamFlushTimers = new Map<string, number>();

const buildTerminalKey = terminalContextRegistry.buildTerminalKey;

const buildState = (id: string): TerminalViewState => {
	const lifecycleState = lifecycle.getSnapshot(id);
	return {
		...lifecycleState,
		debugEnabled: terminalDebugState.isDebugEnabled(),
		debugStats: terminalServiceState.getStatsSnapshot(id),
	};
};

const ensureStore = (id: string): Writable<TerminalViewState> =>
	terminalStores.ensure(id, buildState);
const emitState = (id: string): void => terminalStores.emit(id, buildState);
const emitAllStates = (): void => terminalStores.emitAll(buildState);

const terminalDebugState = createTerminalDebugState({
	emitAllStates,
});

const lifecycle = createTerminalLifecycle({
	emitState,
	emitAllStates,
});

const terminalAttachState = createTerminalAttachState({
	disposeAfterMs: DISPOSE_TTL_MS,
	onDispose: (id) => terminalResourceLifecycle.disposeTerminalResources(id),
	setTimeoutFn: (callback, timeoutMs) => window.setTimeout(callback, timeoutMs),
	clearTimeoutFn: (handle) => window.clearTimeout(handle),
});

const getContext = terminalContextRegistry.getContext;
const ensureContext = terminalContextRegistry.ensureContext;
const getWorkspaceId = terminalContextRegistry.getWorkspaceId;
const getTerminalId = terminalContextRegistry.getTerminalId;

const runtime = createTerminalServiceRuntime<TerminalHandle>({
	lifecycle,
	terminalServiceState,
	terminalDebugState,
	emitState,
	getWorkspaceId,
	getTerminalId,
	terminalTransport,
	terminalHandles,
});

const getToken = (name: string, fallback: string): string =>
	getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;

const terminalSessionBridge = createTerminalSessionBridge(
	() => terminalSessionCoordinator,
	() => terminalEventSubscriptions.ensureListeners(),
);
const {
	ensureSessionActive,
	beginTerminal,
	loadTerminalDefaults,
	refreshSessiondStatus,
	initTerminal,
} = terminalSessionBridge;

const terminalInputOrchestrator = createTerminalInputOrchestrator({
	ensureSessionActive: (id) => ensureSessionActive(id),
	hasStarted: (id) => lifecycle.hasStarted(id),
	appendPendingInput: (id, data) => pendingInput.set(id, (pendingInput.get(id) ?? '') + data),
	recordOutputBytes: (id, bytes) =>
		runtime.updateStats(id, (stats) => {
			stats.bytesOut += bytes;
		}),
	getWorkspaceId,
	getTerminalId,
	isContextActive: (id) => getContext(id)?.active ?? false,
	isTerminalFocused: (id) => {
		const handle = terminalHandles.get(id);
		if (!handle) return false;
		if (typeof document === 'undefined') return true;
		const activeElement = document.activeElement;
		if (!activeElement) return false;
		return handle.container.contains(activeElement);
	},
	write: (workspaceId, terminalId, data) => terminalTransport.write(workspaceId, terminalId, data),
	markStopped: (id) => lifecycle.markStopped(id),
	trace: (id, event, details) => runtime.logDebug(id, event, details),
});

const sendInput = (id: string, data: string): void => terminalInputOrchestrator.sendInput(id, data);

const terminalResizeBridge = createTerminalResizeBridge({
	getWorkspaceId,
	getTerminalId,
	resize: (workspaceId, terminalId, cols, rows) =>
		terminalTransport.resize(workspaceId, terminalId, cols, rows),
});

const terminalViewportResizeController = createTerminalViewportResizeController<TerminalHandle>({
	getHandle: (id) => terminalHandles.get(id),
	hasStarted: (id) => lifecycle.hasStarted(id),
	forceRedraw: runtime.forceRedraw,
	resizeToFit: (id, handle) => terminalResizeBridge.resizeToFit(id, handle),
	resizeOverlay: (_handle) => undefined,
});

const terminalInstanceOrchestration = createTerminalInstanceOrchestration({
	terminalHandles,
	createTerminalInstance: (fontSize) =>
		createTerminalInstance({
			fontSize,
			getToken,
		}),
	setRenderer: (id, renderer) => lifecycle.setRenderer(id, renderer),
	setRendererMode: (id, mode) => lifecycle.setRendererMode(id, mode),
	setStatusAndMessage: (id, status, message) => lifecycle.setStatusAndMessage(id, status, message),
	setHealth: (id, state, message) => runtime.setHealth(id, state, message),
	emitState,
	setInput: (id, value) => lifecycle.setInput(id, value),
	sendInput,
	captureCpr: runtime.captureCpr,
	fitTerminal: (id, started) => terminalViewportResizeController.fitTerminal(id, started),
	hasStarted: (id) => lifecycle.hasStarted(id),
	flushOutput: (id, writeAll) => flushBufferedOutput(id, writeAll),
	markAttached: (id) => terminalAttachState.markAttached(id),
});

const terminalFontSizeController = terminalInstanceOrchestration.terminalFontSizeController;
const terminalInstanceManager = terminalInstanceOrchestration.terminalInstanceManager;
const attachTerminal = terminalInstanceOrchestration.attachTerminal;

const terminalResourceLifecycle = createTerminalResourceLifecycle<TerminalHandle>(
	createTerminalResourceLifecycleDeps({
		terminalViewportResizeController,
		terminalStores,
		terminalServiceState,
		terminalResizeBridge,
		lifecycle,
		terminalAttachState,
		terminalInstanceManager,
		terminalHandles,
	}),
);

const decodeBase64ToBytes = (value: string | undefined): Uint8Array => {
	if (!value) return new Uint8Array();
	if (typeof atob !== 'function') return new Uint8Array();
	const binary = atob(value);
	return Uint8Array.from(binary, (char) => char.charCodeAt(0));
};

const summarizeChunk = (chunk: Uint8Array, limit = 48): Record<string, unknown> => {
	const max = Math.min(chunk.length, limit);
	let esc = 0;
	let c1 = 0;
	let ctrl = 0;
	const headHex: string[] = [];
	for (let i = 0; i < chunk.length; i += 1) {
		const b = chunk[i];
		if (b === 0x1b) esc += 1;
		if (b >= 0x80 && b <= 0x9f) c1 += 1;
		if (b < 0x20 && b !== 0x0a && b !== 0x0d && b !== 0x09) ctrl += 1;
		if (i < max) {
			headHex.push(b.toString(16).padStart(2, '0'));
		}
	}
	return {
		bytes: chunk.length,
		esc,
		c1,
		ctrl,
		headHex: headHex.join(' '),
		truncated: chunk.length > max,
	};
};

const resolvePayloadBytes = (payload: TerminalPayload, chunkLength: number): number => {
	return payload.bytes && payload.bytes > 0 ? payload.bytes : chunkLength;
};

const writeChunkToHandle = (
	id: string,
	handle: TerminalHandle,
	seq: number,
	chunk: Uint8Array,
	bytes: number,
	source: 'stream' | 'buffer',
): void => {
	runtime.logDebug(id, 'frontend_output_chunk', {
		seq,
		source,
		...summarizeChunk(chunk),
	});
	runtime.updateStats(id, (stats) => {
		stats.bytesIn += bytes;
	});
	handle.terminal.write(chunk, () => {
		runtime.updateStatsLastOutput(id);
	});
};

const clearStreamFlushTimer = (id: string): void => {
	const timer = streamFlushTimers.get(id);
	if (timer === undefined) return;
	window.clearTimeout(timer);
	streamFlushTimers.delete(id);
};

const clearStreamOrderingState = (id: string): void => {
	clearStreamFlushTimer(id);
	terminalServiceState.resetOrderedStream(id);
};

const flushOrderedStreamOutput = (id: string, force: boolean): void => {
	const handle = terminalHandles.get(id);
	if (!handle) return;
	const flushed = terminalServiceState.consumeOrderedStreamChunks(id, {
		force,
		minAgeMs: STREAM_REORDER_DELAY_MS,
	});
	if (flushed.chunks.length === 0) return;
	runtime.logDebug(id, 'frontend_output_flush_ordered', {
		force,
		chunks: flushed.chunks.length,
		firstSeq: flushed.chunks[0]?.seq ?? 0,
		lastSeq: flushed.chunks[flushed.chunks.length - 1]?.seq ?? 0,
		droppedStaleChunks: flushed.droppedStaleChunks,
	});
	for (const item of flushed.chunks) {
		writeChunkToHandle(id, handle, item.seq, item.chunk, item.bytes, 'stream');
	}
};

const scheduleStreamFlush = (id: string): void => {
	if (streamFlushTimers.has(id)) return;
	streamFlushTimers.set(
		id,
		window.setTimeout(() => {
			streamFlushTimers.delete(id);
			flushOrderedStreamOutput(id, true);
		}, STREAM_REORDER_DELAY_MS),
	);
};

const flushBufferedOutput = (id: string, writeAll: boolean): void => {
	const handle = terminalHandles.get(id);
	if (!handle) return;
	const queued = terminalServiceState.consumeBufferedOutput(id);
	if (queued.length === 0) return;
	queued.sort((left, right) => left.seq - right.seq);
	let totalBytes = 0;
	for (const item of queued) {
		totalBytes += item.bytes;
	}
	runtime.logDebug(id, 'frontend_output_flushed_buffer', {
		chunks: queued.length,
		bytes: totalBytes,
		writeAll,
	});
	for (const item of queued) {
		writeChunkToHandle(id, handle, item.seq, item.chunk, item.bytes, 'buffer');
	}
};

const writeTerminalDataDirect = (id: string, payload: TerminalPayload): void => {
	if (!payload.dataB64 || payload.dataB64.length === 0) return;
	const chunk = decodeBase64ToBytes(payload.dataB64);
	if (chunk.length === 0) return;

	const seq = payload.seq ?? 0;
	const bytes = resolvePayloadBytes(payload, chunk.length);
	const handle = terminalHandles.get(id);
	if (!handle) {
		const buffered = terminalServiceState.bufferOutputChunk(id, {
			seq,
			bytes,
			chunk,
		});
		runtime.logDebug(id, 'frontend_output_buffered_no_handle', {
			seq,
			...summarizeChunk(chunk),
			bufferedChunks: buffered.bufferedChunks,
			bufferedBytes: buffered.bufferedBytes,
			droppedChunks: buffered.droppedChunks,
			droppedBytes: buffered.droppedBytes,
		});
		if (buffered.droppedChunks > 0) {
			runtime.logDebug(id, 'frontend_output_dropped_no_handle', {
				seq,
				bytes,
				reason: 'buffer_limit',
				droppedChunks: buffered.droppedChunks,
				droppedBytes: buffered.droppedBytes,
			});
		}
		return;
	}
	const buffered = terminalServiceState.getBufferedOutputSnapshot(id);
	if (buffered.bufferedChunks > 0) {
		flushBufferedOutput(id, false);
	}
	const ordered = terminalServiceState.enqueueOrderedStreamChunk(id, {
		seq,
		bytes,
		chunk,
		receivedAt: Date.now(),
	});
	runtime.logDebug(id, 'frontend_output_enqueued_ordered', {
		seq,
		queuedChunks: ordered.queuedChunks,
		queuedBytes: ordered.queuedBytes,
		droppedStaleChunks: ordered.droppedStaleChunks,
		droppedDuplicateChunks: ordered.droppedDuplicateChunks,
	});
	if (ordered.queuedChunks >= STREAM_REORDER_FORCE_FLUSH_THRESHOLD) {
		clearStreamFlushTimer(id);
		flushOrderedStreamOutput(id, true);
		return;
	}
	scheduleStreamFlush(id);
};

const resetSessionState = (id: string): void => {
	clearStreamOrderingState(id);
	terminalResourceLifecycle.resetSessionState(id);
	const buffered = terminalServiceState.consumeBufferedOutput(id);
	if (buffered.length === 0) return;
	let bytes = 0;
	for (const item of buffered) {
		bytes += item.bytes;
	}
	runtime.logDebug(id, 'frontend_output_drop_on_session_reset', {
		chunks: buffered.length,
		bytes,
	});
};

const handleTerminalDataEvent = (id: string, payload: TerminalPayload): void => {
	lifecycle.markInput(id);
	if (lifecycle.getStatus(id) !== 'ready') {
		lifecycle.setStatusAndMessage(id, 'ready', '');
		runtime.setHealth(id, 'ok', 'Session active.');
		emitState(id);
	}
	writeTerminalDataDirect(id, payload);
};

const terminalEventSubscriptions = createTerminalEventSubscriptions({
	subscribeEvent: (event, handler) => terminalTransport.onEvent(event, handler),
	buildTerminalKey,
	isWorkspaceMismatch: (id, payloadWorkspaceId, payloadTerminalId) => {
		const context = getContext(id);
		if (!context) return true;
		if (payloadWorkspaceId && context.workspaceId !== payloadWorkspaceId) return true;
		if (payloadTerminalId && context.terminalId !== payloadTerminalId) return true;
		return false;
	},
	onTerminalData: handleTerminalDataEvent,
});

const terminalSessionCoordinator = createTerminalSessionCoordinator({
	lifecycle,
	getWorkspaceId,
	getTerminalId,
	transport: createTerminalSessionTransport(terminalTransport),
	setHealth: runtime.setHealth,
	emitState,
	pendingInput,
	logDebug: runtime.logDebug,
	resetSessionState,
	writeStartFailureMessage: () => undefined,
	getDebugOverlayPreference: () => terminalDebugState.getDebugOverlayPreference(),
	setDebugOverlayPreference: (value) => terminalDebugState.setDebugOverlayPreference(value),
	clearLocalDebugPreference: clearLocalTerminalDebugPreference,
	syncDebugEnabled: () => terminalDebugState.syncDebugEnabled(),
});

const terminalStreamOrchestrator = createTerminalStreamOrchestrator({
	initTerminal,
	getContext,
	beginTerminal,
	nextSyncToken: (id) => lifecycle.nextInitToken(id),
	isCurrentSyncToken: (id, token) => lifecycle.isCurrentInitToken(id, token),
	emitState,
	trace: (id, event, details) => runtime.logDebug(id, event, details),
});

const ensureGlobals = (): void =>
	ensureTerminalGlobals({
		isInitialized: () => globalsInitialized,
		markInitialized: () => {
			globalsInitialized = true;
		},
		loadTerminalDefaults,
		refreshSessiondStatus,
		onFocus: (callback) => window.addEventListener('focus', callback),
		forEachAttached: (callback) => terminalAttachState.forEachAttached(callback),
		ensureSessionActive,
	});

const terminalServiceExports = createTerminalServiceExports<TerminalViewState>({
	loadTerminalDefaults,
	buildTerminalKey,
	ensureStore,
	syncControllerDeps: createTerminalSyncControllerDeps({
		ensureGlobals,
		buildTerminalKey,
		ensureContext,
		trace: (id, event, details) => runtime.logDebug(id, event, details),
		terminalContextRegistry,
		attachTerminal,
		terminalViewportResizeController,
		terminalStreamOrchestrator,
		terminalAttachState,
		terminalTransport,
		terminalResourceLifecycle: {
			disposeTerminalResources: (id: string) => {
				clearStreamOrderingState(id);
				terminalResourceLifecycle.disposeTerminalResources(id);
			},
		},
	}),
});

export const refreshTerminalDefaults = terminalServiceExports.refreshTerminalDefaults;
export const getTerminalStore = terminalServiceExports.getTerminalStore;
export const syncTerminal = terminalServiceExports.syncTerminal;
export const detachTerminal = terminalServiceExports.detachTerminal;
export const closeTerminal = terminalServiceExports.closeTerminal;
export const focusTerminalInstance = terminalServiceExports.focusTerminalInstance;
export const scrollTerminalToBottom = terminalServiceExports.scrollTerminalToBottom;
export const isTerminalAtBottom = terminalServiceExports.isTerminalAtBottom;

export const releaseWorkspaceTerminals = (workspaceId: string): void => {
	const targetWorkspace = workspaceId.trim();
	if (!targetWorkspace) return;
	for (const key of terminalContextRegistry.keys()) {
		if (getWorkspaceId(key) !== targetWorkspace) continue;
		clearStreamOrderingState(key);
		terminalResourceLifecycle.disposeTerminalResources(key);
		terminalContextRegistry.deleteContext(key);
	}
};

export const shutdownTerminalService = (): void => terminalEventSubscriptions.cleanupListeners();

// Font size controls (VS Code style Cmd/Ctrl +/-)
export const increaseFontSize = (): void => terminalFontSizeController.increaseFontSize();
export const decreaseFontSize = (): void => terminalFontSizeController.decreaseFontSize();
export const resetFontSize = (): void => terminalFontSizeController.resetFontSize();

export const getCurrentFontSize = (): number => terminalFontSizeController.getCurrentFontSize();
