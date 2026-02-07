import type { Writable } from 'svelte/store';
import { terminalTransport } from './terminalTransport';
import { createTerminalInstance } from './terminalRenderer';
import { createTerminalWebLinksSync } from './terminalWebLinks';
import { TerminalStateStore } from './terminalStateStore';
import { stripMouseReports } from './inputFilter';
import { createTerminalLifecycle } from './terminalLifecycle';
import { createTerminalStreamOrchestrator } from './terminalStreamOrchestrator';
import { createTerminalResizeBridge } from './terminalResizeBridge';
import { createTerminalRenderHealth, hasVisibleTerminalContent } from './terminalRenderHealth';
import {
	createTerminalAttachState,
	createTerminalRendererAddonState,
} from './terminalAttachRendererState';
import {
	createTerminalEventSubscriptions,
	type TerminalKittyPayload,
	type TerminalPayload,
} from './terminalEventSubscriptions';
import { createTerminalModeBootstrapCoordinator } from './terminalModeBootstrapCoordinator';
import { createTerminalReplayAckOrchestrator } from './terminalReplayAckOrchestrator';
import { type TerminalInstanceHandle } from './terminalInstanceManager';
import { createTerminalOutputBuffer } from './terminalOutputBuffer';
import {
	createKittyState,
	createTerminalKittyController,
	type KittyEventPayload,
	type KittyOverlay,
	type KittyState,
} from './terminalKittyImageController';
import { createTerminalInputOrchestrator } from './terminalInputOrchestrator';
import { createTerminalViewportResizeController } from './terminalViewportResizeController';
import { createTerminalContextRegistry } from './terminalContextRegistry';
import { createTerminalSessionCoordinator } from './terminalSessionCoordinator';
import { createTerminalResourceLifecycle } from './terminalResourceLifecycle';
import { createTerminalServiceExports } from './terminalServiceExports';
import { createTerminalInstanceOrchestration } from './terminalInstanceOrchestration';

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

type TerminalHandle = TerminalInstanceHandle<KittyState> & {
	kittyOverlay?: KittyOverlay;
};

const terminalHandles = new Map<string, TerminalHandle>();
const terminalContextRegistry = createTerminalContextRegistry();
const terminalStores = new TerminalStateStore<TerminalViewState>();
const DISPOSE_TTL_MS = 10 * 60 * 1000;

const bootstrapHandled = new Map<string, boolean>();
const bootstrapFetchTimers = new Map<string, number>();
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

let debugEnabled = false;
let debugOverlayPreference: 'on' | 'off' | '' = '';

let suppressMouseUntil: Record<string, number> = {};
let mouseInputTail: Record<string, string> = {};

const ACK_BATCH_BYTES = 32 * 1024;
const ACK_FLUSH_DELAY_MS = 25;
const INITIAL_STREAM_CREDIT = 256 * 1024;
const HEALTH_TIMEOUT_MS = 1200;
const textEncoder = typeof TextEncoder !== 'undefined' ? new TextEncoder() : null;
let globalsInitialized = false;

const buildTerminalKey = terminalContextRegistry.buildTerminalKey;

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

const disposeTerminalResources = (id: string): void => {
	terminalResourceLifecycle.disposeTerminalResources(id);
};

const terminalAttachState = createTerminalAttachState({
	disposeAfterMs: DISPOSE_TTL_MS,
	onDispose: (id) => {
		disposeTerminalResources(id);
	},
	setTimeoutFn: (callback, timeoutMs) => window.setTimeout(callback, timeoutMs),
	clearTimeoutFn: (handle) => window.clearTimeout(handle),
});

const getContext = terminalContextRegistry.getContext;
const ensureContext = terminalContextRegistry.ensureContext;
const getWorkspaceId = terminalContextRegistry.getWorkspaceId;
const getTerminalId = terminalContextRegistry.getTerminalId;

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

const resetSessionState = (id: string): void => {
	terminalResourceLifecycle.resetSessionState(id);
};

const resetTerminalInstance = (id: string): void => {
	terminalResourceLifecycle.resetTerminalInstance(id);
};

const updateStatsLastOutput = (id: string): void => {
	updateStats(id, (stats) => {
		stats.lastOutputAt = Date.now();
	});
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

const terminalViewportResizeController = createTerminalViewportResizeController<TerminalHandle>({
	getHandle: (id) => terminalHandles.get(id),
	hasStarted: (id) => lifecycle.hasStarted(id),
	forceRedraw,
	resizeToFit: (id, handle) => {
		terminalResizeBridge.resizeToFit(id, handle);
	},
	resizeOverlay: (handle) => {
		terminalKittyController.resizeOverlay(handle);
	},
	logDebug,
});

const renderHealth = createTerminalRenderHealth({
	getHandle: (id) => terminalHandles.get(id),
	reopenWithPreservedViewport: (id, handle) => {
		if (!handle.container) return;
		const viewport = terminalViewportResizeController.captureViewport(handle.terminal);
		handle.terminal.open(handle.container);
		terminalViewportResizeController.fitWithPreservedViewport(handle, viewport);
		terminalResizeBridge.nudgeRedraw(id, handle);
	},
	fitWithPreservedViewport: (_id, handle) => {
		terminalViewportResizeController.fitWithPreservedViewport(handle);
	},
	nudgeRedraw: (id, handle) => {
		terminalResizeBridge.nudgeRedraw(id, handle);
	},
	logDebug,
});

const terminalOutputBuffer = createTerminalOutputBuffer({
	canWrite: (id) => terminalHandles.has(id),
	writeChunk: (id, data) => {
		const handle = terminalHandles.get(id);
		if (!handle) return;
		handle.terminal.write(data, () => {
			renderHealth.noteRender(id);
		});
	},
	onChunkFlushed: (id) => {
		updateStatsLastOutput(id);
	},
	requestAnimationFrameFn: (callback) => requestAnimationFrame(callback),
});

const replayAckOrchestrator = createTerminalReplayAckOrchestrator<KittyEventPayload>({
	enqueueOutput: terminalOutputBuffer.enqueueOutput,
	flushOutput: terminalOutputBuffer.flushOutput,
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
	await terminalSessionCoordinator.ensureSessionActive(id);
};

const beginTerminal = async (id: string, quiet = false): Promise<void> => {
	await terminalSessionCoordinator.beginTerminal(id, quiet);
};

const ensureStream = async (id: string): Promise<void> => {
	await terminalSessionCoordinator.ensureStream(id);
};

const loadTerminalDefaults = async (): Promise<void> => {
	await terminalSessionCoordinator.loadTerminalDefaults();
};

const refreshSessiondStatus = async (): Promise<void> => {
	await terminalSessionCoordinator.refreshSessiondStatus();
};

const initTerminal = async (id: string): Promise<void> => {
	await terminalSessionCoordinator.initTerminal(id, {
		ensureListeners: () => {
			terminalEventSubscriptions.ensureListeners();
		},
	});
};

const handleSessiondRestarted = (): void => {
	terminalSessionCoordinator.handleSessiondRestarted();
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

const terminalInstanceOrchestration = createTerminalInstanceOrchestration<
	KittyState,
	TerminalHandle
>({
	terminalHandles,
	createTerminalInstance: (fontSize) =>
		createTerminalInstance({
			fontSize,
			getToken,
		}),
	createKittyState,
	syncTerminalWebLinks,
	ensureMode: (id) => lifecycle.ensureMode(id),
	setInput: (id, value) => lifecycle.setInput(id, value),
	beginTerminal: (id) => beginTerminal(id),
	sendInput,
	captureCpr,
	noteRender: (id) => renderHealth.noteRender(id),
	getToken,
	getHandle: (id) => terminalHandles.get(id),
	fitTerminal: (id, started) => terminalViewportResizeController.fitTerminal(id, started),
	hasStarted: (id) => lifecycle.hasStarted(id),
	ensureOverlay: (id) => terminalKittyController.ensureOverlay(id),
	loadRendererAddon: (id, handle) => terminalRendererAddonState.load(id, handle),
	fitWithPreservedViewport: (handle) =>
		terminalViewportResizeController.fitWithPreservedViewport(handle),
	resizeToFit: (id, handle) => terminalResizeBridge.resizeToFit(id, handle),
	scheduleFitStabilization: (id, reason) =>
		terminalViewportResizeController.scheduleFitStabilization(id, reason),
	flushOutput: terminalOutputBuffer.flushOutput,
	markAttached: (id) => terminalAttachState.markAttached(id),
});

const terminalFontSizeController = terminalInstanceOrchestration.terminalFontSizeController;
const terminalInstanceManager = terminalInstanceOrchestration.terminalInstanceManager;
const attachTerminal = terminalInstanceOrchestration.attachTerminal;

const terminalResourceLifecycle = createTerminalResourceLifecycle<TerminalHandle>({
	clearBootstrapFetchTimer: (id) => clearTimeoutMap(bootstrapFetchTimers, id),
	clearReattachTimer: (id) => terminalStreamOrchestrator.clearReattachTimer(id),
	destroyViewportState: (id) => terminalViewportResizeController.destroy(id),
	clearTerminalStore: (id) => terminalStores.delete(id),
	clearOutputBuffer: (id) => terminalOutputBuffer.clear(id),
	destroyReplayState: (id) => replayAckOrchestrator.destroy(id),
	resetReplaySession: (id) => replayAckOrchestrator.resetSession(id),
	deleteBootstrapHandled: (id) => bootstrapHandled.delete(id),
	deleteStats: (id) => statsMap.delete(id),
	deletePendingInput: (id) => pendingInput.delete(id),
	clearResizeState: (id) => terminalResizeBridge.clear(id),
	releaseRenderHealth: (id) => renderHealth.release(id),
	clearRenderHealthSession: (id) => renderHealth.clearSession(id),
	deleteLifecycleState: (id) => lifecycle.deleteState(id),
	dropHealthCheck: (id) => lifecycle.dropHealthCheck(id),
	clearMouseSuppression: (id) => {
		suppressMouseUntil = deleteRecordKey(suppressMouseUntil, id);
	},
	clearMouseTail: (id) => {
		if (!mouseInputTail[id]) return;
		mouseInputTail = { ...mouseInputTail, [id]: '' };
	},
	releaseAttachState: (id) => terminalAttachState.release(id),
	disposeTerminalInstance: (id) => terminalInstanceManager.dispose(id),
	getHandle: (id) => terminalHandles.get(id),
	resizeOverlay: (handle) => terminalKittyController.resizeOverlay(handle),
	setMode: (id, mode) => lifecycle.setMode(id, mode),
	loadRendererAddon: (id, handle) => terminalRendererAddonState.load(id, handle),
	noteMouseSuppress,
});

const terminalModeBootstrapCoordinator = createTerminalModeBootstrapCoordinator<KittyEventPayload>({
	buildTerminalKey,
	getContext: (key) => terminalContextRegistry.getContext(key),
	logDebug,
	markInput: (id) => {
		lifecycle.markInput(id);
	},
	bootstrapHandled,
	setReplayState: replayAckOrchestrator.setReplayState,
	enqueueOutput: terminalOutputBuffer.enqueueOutput,
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

const terminalSessionCoordinator = createTerminalSessionCoordinator({
	lifecycle,
	getWorkspaceId,
	getTerminalId,
	getContext,
	attachTerminal,
	terminalIds: () => terminalContextRegistry.keys(),
	transport: {
		fetchStatus: (workspaceId, terminalId) =>
			terminalTransport.fetchStatus(workspaceId, terminalId),
		fetchBootstrap: (workspaceId, terminalId) =>
			terminalTransport.fetchBootstrap(workspaceId, terminalId),
		start: (workspaceId, terminalId) => terminalTransport.start(workspaceId, terminalId),
		write: (workspaceId, terminalId, data) =>
			terminalTransport.write(workspaceId, terminalId, data),
		fetchSettings: () => terminalTransport.fetchSettings(),
		fetchSessiondStatus: () => terminalTransport.fetchSessiondStatus(),
	},
	setHealth,
	emitState,
	pendingInput,
	bootstrapHandled,
	bootstrapFetchTimers,
	replaySetState: replayAckOrchestrator.setReplayState,
	logDebug,
	handleBootstrapPayload: terminalModeBootstrapCoordinator.handleBootstrapPayload,
	handleBootstrapDonePayload: terminalModeBootstrapCoordinator.handleBootstrapDonePayload,
	resetSessionState,
	resetTerminalInstance,
	noteMouseSuppress,
	writeStartFailureMessage: (id, message) => {
		const handle = terminalHandles.get(id);
		handle?.terminal.write(`\r\n[workset] failed to start terminal: ${message}`);
	},
	getDebugOverlayPreference: () => debugOverlayPreference,
	setDebugOverlayPreference: (value) => {
		debugOverlayPreference = value;
	},
	clearLocalDebugPreference: () => {
		if (typeof localStorage === 'undefined') return;
		try {
			localStorage.removeItem('worksetTerminalDebug');
		} catch {
			// Ignore storage failures.
		}
	},
	syncDebugEnabled,
});

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

const terminalServiceExports = createTerminalServiceExports<TerminalViewState, TerminalHandle>({
	loadTerminalDefaults,
	buildTerminalKey,
	ensureStore,
	syncControllerDeps: {
		ensureGlobals,
		buildTerminalKey,
		ensureContext,
		getLastWorkspaceId: (key) => terminalContextRegistry.getLastWorkspaceId(key),
		setLastWorkspaceId: (key, workspaceId) =>
			terminalContextRegistry.setLastWorkspaceId(key, workspaceId),
		deleteContext: (key) => terminalContextRegistry.deleteContext(key),
		attachTerminal,
		attachResizeObserver: (id, container) =>
			terminalViewportResizeController.attachResizeObserver(id, container),
		detachResizeObserver: (id) => terminalViewportResizeController.detachResizeObserver(id),
		scheduleFitStabilization: (id, reason) =>
			terminalViewportResizeController.scheduleFitStabilization(id, reason),
		scheduleReattachCheck: (id, reason) =>
			terminalStreamOrchestrator.scheduleReattachCheck(id, reason),
		syncTerminalStream: (id) => terminalStreamOrchestrator.syncTerminalStream(id),
		fitTerminal: (id, started) => terminalViewportResizeController.fitTerminal(id, started),
		hasStarted: (id) => lifecycle.hasStarted(id),
		forceRedraw,
		getHandle: (id) => terminalHandles.get(id),
		hasVisibleTerminalContent: (handle) => hasVisibleTerminalContent(handle.terminal),
		nudgeRedraw: (id, handle) => terminalResizeBridge.nudgeRedraw(id, handle),
		markDetached: (id) => terminalAttachState.markDetached(id),
		stopTerminal: (workspaceId, terminalId) => terminalTransport.stop(workspaceId, terminalId),
		disposeTerminalResources,
		beginTerminal,
		requestHealthCheck,
		focusTerminal: (id) => terminalViewportResizeController.focusTerminal(id),
		scrollToBottom: (id) => terminalViewportResizeController.scrollToBottom(id),
		isAtBottom: (id) => terminalViewportResizeController.isAtBottom(id),
	},
});

export const refreshTerminalDefaults = terminalServiceExports.refreshTerminalDefaults;
export const getTerminalStore = terminalServiceExports.getTerminalStore;
export const syncTerminal = terminalServiceExports.syncTerminal;
export const detachTerminal = terminalServiceExports.detachTerminal;
export const closeTerminal = terminalServiceExports.closeTerminal;
export const restartTerminal = terminalServiceExports.restartTerminal;
export const retryHealthCheck = terminalServiceExports.retryHealthCheck;
export const focusTerminalInstance = terminalServiceExports.focusTerminalInstance;
export const scrollTerminalToBottom = terminalServiceExports.scrollTerminalToBottom;
export const isTerminalAtBottom = terminalServiceExports.isTerminalAtBottom;

export const shutdownTerminalService = (): void => terminalEventSubscriptions.cleanupListeners();

// Font size controls (VS Code style Cmd/Ctrl +/-)
export const increaseFontSize = (): void => terminalFontSizeController.increaseFontSize();
export const decreaseFontSize = (): void => terminalFontSizeController.decreaseFontSize();
export const resetFontSize = (): void => terminalFontSizeController.resetFontSize();

export const getCurrentFontSize = (): number => terminalFontSizeController.getCurrentFontSize();
