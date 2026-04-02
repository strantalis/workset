import type { Writable } from 'svelte/store';
import { terminalTransport } from './terminalTransport';
import { createTerminalInstance } from './terminalRenderer';
import { TerminalStateStore } from './terminalStateStore';
import { createTerminalLifecycle } from './terminalLifecycle';
import { createTerminalStreamOrchestrator } from './terminalStreamOrchestrator';
import { createTerminalResizeBridge } from './terminalResizeBridge';
import { createTerminalAttachState } from './terminalAttachRendererState';
import { type TerminalInstanceHandle } from './terminalInstanceManager';
import { createTerminalInputOrchestrator } from './terminalInputOrchestrator';
import { createTerminalViewportResizeController } from './terminalViewportResizeController';
import { createTerminalContextRegistry } from './terminalContextRegistry';
import { createTerminalSessionCoordinator } from './terminalSessionCoordinator';
import { createTerminalResourceLifecycle } from './terminalResourceLifecycle';
import { createTerminalServiceExports } from './terminalServiceExports';
import { createTerminalInstanceOrchestration } from './terminalInstanceOrchestration';
import { createTerminalDebugState } from './terminalDebugState';
import { createTerminalSnapshotManager } from './terminalSnapshotManager';
import {
	clearLocalTerminalDebugPreference,
	createTerminalSessionBridge,
	createTerminalResourceLifecycleDeps,
	createTerminalSyncControllerDeps,
	ensureTerminalGlobals,
} from './terminalServiceDeps';
import { createTerminalServiceState } from './terminalServiceState';
import { createTerminalServiceRuntime } from './terminalServiceRuntime';
import { createTerminalSocketStream } from './terminalSocketStream';
import { emitTerminalActivity } from './terminalActivityBus';
import type { TerminalSnapshotLike } from './terminalEmulatorContracts';

export type TerminalViewState = {
	status: string;
	message: string;
	health: 'unknown' | 'checking' | 'ok' | 'stale';
	healthMessage: string;
	terminalServiceAvailable: boolean | null;
	terminalServiceChecked: boolean;
	debugEnabled: boolean;
	debugStats: {
		bytesIn: number;
		bytesOut: number;
		lastOutputAt: number;
		lastCprAt: number;
	};
};

type TerminalHandle = TerminalInstanceHandle;
type TerminalPendingWrite = {
	seq: number;
	chunk: Uint8Array;
	bytes: number;
	source: 'stream' | 'buffer';
};
type TerminalWriteQueueState = {
	queue: TerminalPendingWrite[];
	inFlight: boolean;
	scheduled: boolean;
};

const terminalHandles = new Map<string, TerminalHandle>();
const terminalWriteQueues = new Map<string, TerminalWriteQueueState>();
const terminalContextRegistry = createTerminalContextRegistry();
const terminalStores = new TerminalStateStore<TerminalViewState>();
const DISPOSE_TTL_MS = 10 * 60 * 1000;
let globalsInitialized = false;
const terminalServiceState = createTerminalServiceState();
const { pendingInput } = terminalServiceState;
const lastInboundSeq = new Map<string, { seq: number; bytes: number; at: number }>();
const lastStreamOffset = new Map<string, number>();
const consumedStartupSnapshots = new Set<string>();

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

const terminalSessionBridge = createTerminalSessionBridge(() => terminalSessionCoordinator);
const {
	ensureSessionActive,
	beginTerminal,
	loadTerminalDefaults,
	refreshTerminalServiceStatus,
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
	markActivity: (workspaceId) => emitTerminalActivity(workspaceId),
	write: async (workspaceId, terminalId, data) => {
		terminalSocketStream.write(buildTerminalKey(workspaceId, terminalId), data);
	},
	markStopped: (id) => lifecycle.markStopped(id),
	trace: (id, event, details) => runtime.logDebug(id, event, details),
});

const sendInput = (id: string, data: string): void => terminalInputOrchestrator.sendInput(id, data);
const sendProtocolResponse = (id: string, data: string): void =>
	terminalInputOrchestrator.sendProtocolResponse(id, data);

const terminalResizeBridge = createTerminalResizeBridge({
	getWorkspaceId,
	getTerminalId,
	resize: async (workspaceId, terminalId, cols, rows) => {
		terminalSocketStream.resize(buildTerminalKey(workspaceId, terminalId), cols, rows);
	},
});

const terminalViewportResizeController = createTerminalViewportResizeController<TerminalHandle>({
	getHandle: (id) => terminalHandles.get(id),
	hasStarted: (id) => lifecycle.hasStarted(id),
	resizeToFit: (id, handle) => terminalResizeBridge.resizeToFit(id, handle),
	resizeOverlay: (_handle) => undefined,
});

const terminalInstanceOrchestration = createTerminalInstanceOrchestration({
	terminalHandles,
	createTerminalInstance: (fontSize, cursorBlink) =>
		createTerminalInstance({
			fontSize,
			cursorBlink,
			getToken,
			openLink: (url) => terminalTransport.openURL(url),
		}),
	setStatusAndMessage: (id, status, message) => lifecycle.setStatusAndMessage(id, status, message),
	setHealth: (id, state, message) => runtime.setHealth(id, state, message),
	emitState,
	setInput: (id, value) => lifecycle.setInput(id, value),
	sendInput,
	sendProtocolResponse,
	captureCpr: runtime.captureCpr,
	fitTerminal: (id, started) => terminalViewportResizeController.fitTerminal(id, started),
	hasStarted: (id) => lifecycle.hasStarted(id),
	flushOutput: (id, writeAll) => {
		flushBufferedOutput(id, writeAll);
		scheduleTerminalWriteQueue(id);
	},
	markAttached: (id) => terminalAttachState.markAttached(id),
	traceAttach: (id, event, details) => runtime.logDebug(id, event, details),
	traceRenderer: (id, event, details) => runtime.logDebug(id, event, details),
});

const terminalFontSizeController = terminalInstanceOrchestration.terminalFontSizeController;
const terminalInstanceManager = terminalInstanceOrchestration.terminalInstanceManager;
const attachTerminal = async (
	id: string,
	container: HTMLDivElement | null,
	active: boolean,
	initialSnapshot?: TerminalSnapshotLike | null,
): Promise<TerminalHandle> => {
	const handle = await terminalInstanceOrchestration.attachTerminal(id, container, active);
	terminalSnapshotManager.register(id, handle);
	if (initialSnapshot && !consumedStartupSnapshots.has(id)) {
		consumedStartupSnapshots.add(id);
		await terminalSnapshotManager.restore(id, initialSnapshot);
	}
	scheduleTerminalWriteQueue(id);
	return handle;
};

const baseTerminalResourceLifecycle = createTerminalResourceLifecycle<TerminalHandle>(
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

const getTerminalWriteQueueState = (id: string): TerminalWriteQueueState => {
	const existing = terminalWriteQueues.get(id);
	if (existing) return existing;
	const created: TerminalWriteQueueState = {
		queue: [],
		inFlight: false,
		scheduled: false,
	};
	terminalWriteQueues.set(id, created);
	return created;
};

const concatChunks = (chunks: Uint8Array[]): Uint8Array => {
	if (chunks.length === 1) return chunks[0];
	let total = 0;
	for (const chunk of chunks) {
		total += chunk.length;
	}
	const merged = new Uint8Array(total);
	let offset = 0;
	for (const chunk of chunks) {
		merged.set(chunk, offset);
		offset += chunk.length;
	}
	return merged;
};

const scheduleTerminalWriteQueue = (id: string): void => {
	const state = terminalWriteQueues.get(id);
	if (!state || state.inFlight || state.scheduled) return;
	state.scheduled = true;
	terminalWriteQueues.set(id, state);
	queueMicrotask(() => {
		const current = terminalWriteQueues.get(id);
		if (!current) return;
		current.scheduled = false;
		terminalWriteQueues.set(id, current);
		pumpTerminalWriteQueue(id);
	});
};

const clearTerminalWriteQueue = (id: string): void => {
	terminalWriteQueues.delete(id);
};

const pumpTerminalWriteQueue = (id: string): void => {
	const state = terminalWriteQueues.get(id);
	if (!state || state.inFlight) return;
	const next = state.queue[0];
	if (!next) {
		terminalWriteQueues.delete(id);
		return;
	}
	const handle = terminalHandles.get(id);
	if (!handle || handle.opened !== true) return;

	const batch = state.queue.slice();
	const chunks = batch.map((item) => item.chunk);
	const mergedChunk = concatChunks(chunks);
	let mergedBytes = 0;
	for (const item of batch) {
		mergedBytes += item.bytes;
	}
	const first = batch[0];
	const last = batch[batch.length - 1];

	state.inFlight = true;
	terminalWriteQueues.set(id, state);
	runtime.logDebug(id, 'frontend_output_chunk_batch', {
		firstSeq: first.seq,
		lastSeq: last.seq,
		chunks: batch.length,
		bytes: mergedBytes,
		sources: Array.from(new Set(batch.map((item) => item.source))),
		...summarizeChunk(mergedChunk),
	});
	runtime.updateStats(id, (stats) => {
		stats.bytesIn += mergedBytes;
	});
	handle.terminal.write(mergedChunk, () => {
		runtime.updateStatsLastOutput(id);
		const current = terminalWriteQueues.get(id);
		if (!current) return;
		if (current.queue.length > 0) {
			current.queue.splice(0, batch.length);
		}
		current.inFlight = false;
		if (current.queue.length === 0) {
			terminalWriteQueues.delete(id);
		} else {
			terminalWriteQueues.set(id, current);
			scheduleTerminalWriteQueue(id);
		}
	});
};

const writeChunkToHandle = (
	id: string,
	seq: number,
	chunk: Uint8Array,
	bytes: number,
	source: 'stream' | 'buffer',
): void => {
	const state = getTerminalWriteQueueState(id);
	state.queue.push({
		seq,
		chunk,
		bytes,
		source,
	});
	terminalWriteQueues.set(id, state);
	scheduleTerminalWriteQueue(id);
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
		writeChunkToHandle(id, item.seq, item.chunk, item.bytes, 'buffer');
	}
};

const writeTerminalChunkDirect = (
	id: string,
	seq: number,
	chunk: Uint8Array,
	bytes: number,
): void => {
	if (chunk.length === 0) return;
	const normalizedBytes = bytes > 0 ? bytes : chunk.length;
	const now = Date.now();
	const previous = lastInboundSeq.get(id);
	if (previous && seq <= previous.seq) {
		runtime.logDebug(id, 'frontend_output_seq_stale_received', {
			seq,
			bytes: normalizedBytes,
			previousSeq: previous.seq,
			previousBytes: previous.bytes,
			deltaMs: now - previous.at,
		});
		return;
	}
	lastInboundSeq.set(id, {
		seq,
		bytes: normalizedBytes,
		at: now,
	});
	lastStreamOffset.set(id, seq);
	const handle = terminalHandles.get(id);
	if (!handle) {
		const buffered = terminalServiceState.bufferOutputChunk(id, {
			seq,
			bytes: normalizedBytes,
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
				bytes: normalizedBytes,
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
	writeChunkToHandle(id, seq, chunk, normalizedBytes, 'stream');
};

const resetSessionState = (id: string): void => {
	clearTerminalWriteQueue(id);
	lastInboundSeq.delete(id);
	lastStreamOffset.delete(id);
	baseTerminalResourceLifecycle.resetSessionState(id);
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

const terminalSocketStream = createTerminalSocketStream({
	logDebug: (id, event, details) => runtime.logDebug(id, event, details),
	onReady: (id) => {
		runtime.logDebug(id, 'frontend_socket_ready', {});
	},
	onChunk: (id, nextOffset, chunk) => {
		const bytes = chunk.length;
		runtime.logDebug(id, 'frontend_output_event_received', {
			seq: nextOffset,
			bytes,
			source: 'terminal-service-websocket',
		});
		lifecycle.markInput(id);
		const workspaceId = getWorkspaceId(id);
		if (workspaceId) {
			emitTerminalActivity(workspaceId);
		}
		if (lifecycle.getStatus(id) !== 'ready') {
			lifecycle.setStatusAndMessage(id, 'ready', '');
			runtime.setHealth(id, 'ok', 'Session active.');
			emitState(id);
		}
		writeTerminalChunkDirect(id, nextOffset, chunk, bytes);
	},
	onError: (id, error) => {
		runtime.logDebug(id, 'frontend_socket_error', { error });
	},
	onClosed: (id, details) => {
		runtime.logDebug(id, 'frontend_socket_closed', details);
		if (details.intentional) {
			return;
		}
		if (details.serverClosed || details.reason === 'subscriber closed') {
			lifecycle.markStopped(id);
			lifecycle.setInput(id, false);
			lifecycle.setStatusAndMessage(id, 'closed', 'Terminal exited.');
			runtime.setHealth(id, 'unknown', '');
			emitState(id);
			return;
		}
		lifecycle.markStopped(id);
		lifecycle.setInput(id, false);
		lifecycle.setStatusAndMessage(id, 'error', 'Terminal stream disconnected.');
		runtime.setHealth(id, 'stale', 'Terminal stream disconnected.');
		emitState(id);
	},
});

const terminalResourceLifecycle = {
	resetSessionState: (id: string): void => {
		terminalSnapshotManager.clear(id);
		consumedStartupSnapshots.delete(id);
		terminalSocketStream.disconnect(id);
		clearTerminalWriteQueue(id);
		lastStreamOffset.delete(id);
		baseTerminalResourceLifecycle.resetSessionState(id);
	},
	disposeTerminalResources: (id: string): void => {
		terminalSnapshotManager.clear(id);
		consumedStartupSnapshots.delete(id);
		terminalSocketStream.disconnect(id);
		clearTerminalWriteQueue(id);
		lastStreamOffset.delete(id);
		baseTerminalResourceLifecycle.disposeTerminalResources(id);
	},
};

const terminalSnapshotManager = createTerminalSnapshotManager({
	terminalHandles,
	getOffset: (id) => lastStreamOffset.get(id) ?? 0,
	logDebug: (id, event, details) => runtime.logDebug(id, event, details),
	beforeRestore: (id, snapshot) => {
		clearTerminalWriteQueue(id);
		terminalServiceState.deletePendingOutput(id);
		lastInboundSeq.set(id, {
			seq: snapshot.nextOffset,
			bytes: 0,
			at: Date.now(),
		});
		lastStreamOffset.set(id, snapshot.nextOffset);
	},
	afterRestore: (id) => {
		terminalViewportResizeController.fitTerminal(id, true);
	},
});

const terminalSessionCoordinator = createTerminalSessionCoordinator({
	lifecycle,
	getWorkspaceId,
	getTerminalId,
	transport: {
		start: (workspaceId, terminalId) => terminalTransport.start(workspaceId, terminalId),
		write: async (workspaceId, terminalId, data) => {
			terminalSocketStream.write(buildTerminalKey(workspaceId, terminalId), data);
		},
		fetchSettings: () => terminalTransport.fetchSettings(),
		fetchTerminalServiceStatus: () => terminalTransport.fetchTerminalServiceStatus(),
	},
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
	getCurrentTerminalFontSize: () => terminalFontSizeController.getCurrentFontSize(),
	setCurrentTerminalFontSize: (value) => terminalFontSizeController.setFontSize(value),
	getCurrentCursorBlink: () => terminalFontSizeController.getCursorBlink(),
	setCurrentCursorBlink: (value) => terminalFontSizeController.setCursorBlink(value),
	onSessionReady: async (id, descriptor) => {
		try {
			await terminalSocketStream.connect(id, descriptor);
		} catch (error) {
			runtime.logDebug(id, 'frontend_socket_connect_failed', {
				error: String(error),
			});
			lifecycle.markStopped(id);
			lifecycle.setInput(id, false);
			lifecycle.setStatusAndMessage(id, 'error', String(error));
			runtime.setHealth(id, 'stale', 'Failed to attach terminal stream.');
			emitState(id);
			throw error;
		}
		terminalViewportResizeController.fitTerminal(id, true);
	},
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
		refreshTerminalServiceStatus,
		onFocus: (callback) => window.addEventListener('focus', callback),
		forEachAttached: (callback) => terminalAttachState.forEachAttached(callback),
		ensureSessionActive: async (id) => ensureSessionActive(id),
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
		terminalAttachState: {
			markDetached: (id: string) => {
				terminalAttachState.markDetached(id);
			},
		},
		stopTerminal: async (workspaceId: string, terminalId: string) => {
			const id = buildTerminalKey(workspaceId, terminalId);
			if (terminalSocketStream.hasLiveConnection(id)) {
				terminalSocketStream.stop(id);
				return;
			}
			await terminalTransport.stop(workspaceId, terminalId);
		},
		terminalResourceLifecycle: {
			disposeTerminalResources: (id: string) => {
				terminalResourceLifecycle.disposeTerminalResources(id);
			},
		},
	}),
	captureSnapshot: (id) => terminalSnapshotManager.capture(id),
});

export const refreshTerminalDefaults = terminalServiceExports.refreshTerminalDefaults;
export const getTerminalStore = terminalServiceExports.getTerminalStore;
export const syncTerminal = terminalServiceExports.syncTerminal;
export const detachTerminal = terminalServiceExports.detachTerminal;
export const closeTerminal = terminalServiceExports.closeTerminal;
export const focusTerminalInstance = terminalServiceExports.focusTerminalInstance;
export const scrollTerminalToBottom = terminalServiceExports.scrollTerminalToBottom;
export const isTerminalAtBottom = terminalServiceExports.isTerminalAtBottom;
export const captureTerminalSnapshot = terminalServiceExports.captureTerminalSnapshot;

export const releaseWorkspaceTerminals = (workspaceId: string): void => {
	const targetWorkspace = workspaceId.trim();
	if (!targetWorkspace) return;
	for (const key of terminalContextRegistry.keys()) {
		if (getWorkspaceId(key) !== targetWorkspace) continue;
		terminalResourceLifecycle.disposeTerminalResources(key);
		consumedStartupSnapshots.delete(key);
		terminalContextRegistry.deleteContext(key);
	}
};

export const shutdownTerminalService = (): void => {
	terminalSocketStream.disconnectAll();
};

export const increaseFontSize = (): void => terminalFontSizeController.increaseFontSize();
export const decreaseFontSize = (): void => terminalFontSizeController.decreaseFontSize();
export const resetFontSize = (): void => terminalFontSizeController.resetFontSize();

export const getCurrentFontSize = (): number => terminalFontSizeController.getCurrentFontSize();
