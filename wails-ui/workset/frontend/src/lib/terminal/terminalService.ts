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
import { createTerminalDebugState } from './terminalDebugState';
import { createTerminalMouseState } from './terminalMouseState';
import {
	clearLocalTerminalDebugPreference,
	createTerminalSessionBridge,
	createTerminalModeBootstrapCoordinatorDeps,
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

const ACK_BATCH_BYTES = 32 * 1024;
const ACK_FLUSH_DELAY_MS = 25;
const INITIAL_STREAM_CREDIT = 256 * 1024;
const HEALTH_TIMEOUT_MS = 1200;
const textEncoder = typeof TextEncoder !== 'undefined' ? new TextEncoder() : null;
let globalsInitialized = false;
const terminalServiceState = createTerminalServiceState({
	textEncoder,
});
const { bootstrapHandled, bootstrapFetchTimers, pendingInput } = terminalServiceState;

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

const terminalMouseState = createTerminalMouseState();

const lifecycle = createTerminalLifecycle({
	emitState,
	emitAllStates,
});

const syncTerminalWebLinks = createTerminalWebLinksSync({
	getHandle: (id) => terminalHandles.get(id),
	isMouseModeActive: (id) => lifecycle.getMode(id).mouse,
	openURL: (url) => void terminalTransport.openURL(url),
});

const clearTimeoutMap = (map: Map<string, number>, id: string): void => {
	const timer = map.get(id);
	if (!timer) return;
	window.clearTimeout(timer);
	map.delete(id);
};

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
const countBytes = terminalServiceState.countBytes;

const runtime = createTerminalServiceRuntime<TerminalHandle>({
	lifecycle,
	terminalServiceState,
	terminalDebugState,
	emitState,
	getWorkspaceId,
	getTerminalId,
	terminalTransport,
	terminalHandles,
	healthTimeoutMs: HEALTH_TIMEOUT_MS,
});

const getToken = (name: string, fallback: string): string =>
	getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;

const terminalKittyController = createTerminalKittyController<TerminalHandle>({
	getHandle: (id) => terminalHandles.get(id),
});

const terminalSessionBridge = createTerminalSessionBridge(
	() => terminalSessionCoordinator,
	() => terminalEventSubscriptions.ensureListeners(),
);
const {
	ensureSessionActive,
	beginTerminal,
	ensureStream,
	loadTerminalDefaults,
	refreshSessiondStatus,
	initTerminal,
	handleSessiondRestarted,
} = terminalSessionBridge;

const terminalInputOrchestrator = createTerminalInputOrchestrator({
	shouldSuppressMouseInput: (id, data) => terminalMouseState.shouldSuppressInput(id, data),
	getMode: (id) => lifecycle.getMode(id),
	filterMouseReports: stripMouseReports,
	getMouseTail: (id) => terminalMouseState.getTail(id),
	setMouseTail: (id, tail) => terminalMouseState.setTail(id, tail),
	ensureSessionActive: (id) => ensureSessionActive(id),
	hasStarted: (id) => lifecycle.hasStarted(id),
	appendPendingInput: (id, data) => pendingInput.set(id, (pendingInput.get(id) ?? '') + data),
	recordOutputBytes: (id, bytes) =>
		runtime.updateStats(id, (stats) => {
			stats.bytesOut += bytes;
		}),
	getWorkspaceId,
	getTerminalId,
	write: (workspaceId, terminalId, data) => terminalTransport.write(workspaceId, terminalId, data),
	markStopped: (id) => lifecycle.markStopped(id),
	resetTerminalInstance: (id) => resetTerminalInstance(id),
	beginTerminal: (id, quiet) => beginTerminal(id, quiet),
	writeFailureMessage: (id, message) => {
		const handle = terminalHandles.get(id);
		handle?.terminal.write(`\r\n[workset] write failed: ${message}`);
	},
});

const sendInput = (id: string, data: string): void => terminalInputOrchestrator.sendInput(id, data);
const resetSessionState = (id: string): void => terminalResourceLifecycle.resetSessionState(id);
const resetTerminalInstance = (id: string): void =>
	terminalResourceLifecycle.resetTerminalInstance(id);

const terminalResizeBridge = createTerminalResizeBridge({
	getWorkspaceId,
	getTerminalId,
	resize: (workspaceId, terminalId, cols, rows) =>
		terminalTransport.resize(workspaceId, terminalId, cols, rows),
	logDebug: runtime.logDebug,
});

const terminalViewportResizeController = createTerminalViewportResizeController<TerminalHandle>({
	getHandle: (id) => terminalHandles.get(id),
	hasStarted: (id) => lifecycle.hasStarted(id),
	forceRedraw: runtime.forceRedraw,
	resizeToFit: (id, handle) => terminalResizeBridge.resizeToFit(id, handle),
	resizeOverlay: (handle) => terminalKittyController.resizeOverlay(handle),
	logDebug: runtime.logDebug,
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
	fitWithPreservedViewport: (_id, handle) =>
		terminalViewportResizeController.fitWithPreservedViewport(handle),
	nudgeRedraw: (id, handle) => terminalResizeBridge.nudgeRedraw(id, handle),
	logDebug: runtime.logDebug,
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
	onChunkFlushed: (id) => runtime.updateStatsLastOutput(id),
	requestAnimationFrameFn: (callback) => requestAnimationFrame(callback),
});

const replayAckOrchestrator = createTerminalReplayAckOrchestrator<KittyEventPayload>({
	enqueueOutput: terminalOutputBuffer.enqueueOutput,
	flushOutput: terminalOutputBuffer.flushOutput,
	forceRedraw: runtime.forceRedraw,
	hasTerminalHandle: (id) => terminalHandles.has(id),
	applyKittyEvent: (id, event) => terminalKittyController.applyEvent(id, event),
	getWorkspaceId,
	getTerminalId,
	ack: (workspaceId, terminalId, bytes) => terminalTransport.ack(workspaceId, terminalId, bytes),
	setTimeoutFn: (callback, timeoutMs) => window.setTimeout(callback, timeoutMs),
	clearTimeoutFn: (handle) => window.clearTimeout(handle),
	countBytes,
	recordBytesIn: (id, bytes) =>
		runtime.updateStats(id, (stats) => {
			stats.bytesIn += bytes;
		}),
	noteOutputActivity: (id) => renderHealth.noteOutputActivity(id),
	ackBatchBytes: ACK_BATCH_BYTES,
	ackFlushDelayMs: ACK_FLUSH_DELAY_MS,
	initialStreamCredit: INITIAL_STREAM_CREDIT,
});

const terminalRendererAddonState = createTerminalRendererAddonState({
	setRendererMode: (id, mode) => {
		lifecycle.setRendererMode(id, mode);
	},
	setRenderer: (id, renderer) => {
		lifecycle.setRenderer(id, renderer);
	},
	onRendererUnavailable: (id, error) => {
		lifecycle.setStatusAndMessage(id, 'error', 'WebGL renderer unavailable.');
		runtime.setHealth(id, 'stale', 'WebGL renderer unavailable.');
		runtime.logDebug(id, 'renderer_webgl_failed', { error: String(error) });
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
	captureCpr: runtime.captureCpr,
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

const terminalResourceLifecycle = createTerminalResourceLifecycle<TerminalHandle>(
	createTerminalResourceLifecycleDeps({
		bootstrapFetchTimers,
		clearTimeoutMap,
		terminalStreamOrchestrator: {
			clearReattachTimer: (id) => terminalStreamOrchestrator.clearReattachTimer(id),
		},
		terminalViewportResizeController,
		terminalStores,
		terminalOutputBuffer,
		replayAckOrchestrator,
		bootstrapHandled,
		terminalServiceState,
		terminalResizeBridge,
		renderHealth,
		lifecycle,
		terminalMouseState,
		terminalAttachState,
		terminalInstanceManager,
		terminalHandles,
		terminalKittyController,
		terminalRendererAddonState,
	}),
);

const terminalModeBootstrapCoordinator = createTerminalModeBootstrapCoordinator<KittyEventPayload>(
	createTerminalModeBootstrapCoordinatorDeps({
		buildTerminalKey,
		terminalContextRegistry,
		logDebug: runtime.logDebug,
		lifecycle,
		bootstrapHandled,
		replayAckOrchestrator,
		terminalOutputBuffer,
		countBytes,
		terminalHandles,
		terminalKittyController,
		setHealth: runtime.setHealth,
		initialStreamCredit: INITIAL_STREAM_CREDIT,
		renderHealth,
		emitState,
		syncTerminalWebLinks,
	}),
);

const handleTerminalDataEvent = (id: string, payload: TerminalPayload): void => {
	lifecycle.markInput(id);
	replayAckOrchestrator.handleTerminalData(id, payload);
};
const handleTerminalKittyEvent = (id: string, payload: TerminalKittyPayload): void =>
	replayAckOrchestrator.handleTerminalKitty(id, payload.event);

const terminalEventSubscriptions = createTerminalEventSubscriptions({
	subscribeEvent: (event, handler) => terminalTransport.onEvent(event, handler),
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
	transport: createTerminalSessionTransport(terminalTransport),
	setHealth: runtime.setHealth,
	emitState,
	pendingInput,
	bootstrapHandled,
	bootstrapFetchTimers,
	replaySetState: replayAckOrchestrator.setReplayState,
	logDebug: runtime.logDebug,
	handleBootstrapPayload: terminalModeBootstrapCoordinator.handleBootstrapPayload,
	handleBootstrapDonePayload: terminalModeBootstrapCoordinator.handleBootstrapDonePayload,
	resetSessionState,
	resetTerminalInstance,
	noteMouseSuppress: (id, durationMs) => terminalMouseState.noteSuppress(id, durationMs),
	writeStartFailureMessage: (id, message) => {
		const handle = terminalHandles.get(id);
		handle?.terminal.write(`\r\n[workset] failed to start terminal: ${message}`);
	},
	getDebugOverlayPreference: () => terminalDebugState.getDebugOverlayPreference(),
	setDebugOverlayPreference: (value) => terminalDebugState.setDebugOverlayPreference(value),
	clearLocalDebugPreference: clearLocalTerminalDebugPreference,
	syncDebugEnabled: () => terminalDebugState.syncDebugEnabled(),
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
	logDebug: runtime.logDebug,
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

const terminalServiceExports = createTerminalServiceExports<TerminalViewState, TerminalHandle>({
	loadTerminalDefaults,
	buildTerminalKey,
	ensureStore,
	syncControllerDeps: createTerminalSyncControllerDeps({
		ensureGlobals,
		buildTerminalKey,
		ensureContext,
		terminalContextRegistry,
		attachTerminal,
		terminalViewportResizeController,
		terminalStreamOrchestrator,
		lifecycle,
		forceRedraw: runtime.forceRedraw,
		terminalHandles,
		hasVisibleTerminalContent: (handle) => hasVisibleTerminalContent(handle.terminal),
		terminalResizeBridge,
		terminalAttachState,
		terminalTransport,
		terminalResourceLifecycle,
		beginTerminal,
		requestHealthCheck: runtime.requestHealthCheck,
	}),
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
