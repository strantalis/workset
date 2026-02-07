import type { TerminalLifecyclePayload } from './terminalEventSubscriptions';
import type { TerminalModesState } from './terminalLifecycle';
import type { TerminalSyncControllerDependencies } from './terminalSyncController';
import type { TerminalTransport } from './terminalTransport';

type TerminalContextLike = {
	workspaceId?: string;
	terminalId?: string;
};

type SessionTransportAdapter = Pick<
	TerminalTransport,
	'fetchStatus' | 'fetchBootstrap' | 'start' | 'write' | 'fetchSettings' | 'fetchSessiondStatus'
>;

type ResourceLifecycleDepsInput<THandle> = {
	bootstrapFetchTimers: Map<string, number>;
	clearTimeoutMap: (map: Map<string, number>, id: string) => void;
	terminalStreamOrchestrator: {
		clearReattachTimer: (id: string) => void;
	};
	terminalViewportResizeController: {
		destroy: (id: string) => void;
	};
	terminalStores: {
		delete: (id: string) => void;
	};
	terminalOutputBuffer: {
		clear: (id: string) => void;
	};
	replayAckOrchestrator: {
		destroy: (id: string) => void;
		resetSession: (id: string) => void;
	};
	bootstrapHandled: Map<string, boolean>;
	terminalServiceState: {
		deleteStats: (id: string) => void;
		deletePendingInput: (id: string) => void;
	};
	terminalResizeBridge: {
		clear: (id: string) => void;
	};
	renderHealth: {
		release: (id: string) => void;
		clearSession: (id: string) => void;
	};
	lifecycle: {
		deleteState: (id: string) => void;
		dropHealthCheck: (id: string) => void;
		setMode: (id: string, mode: TerminalModesState) => void;
	};
	terminalMouseState: {
		clearSuppression: (id: string) => void;
		clearTail: (id: string) => void;
		noteSuppress: (id: string, durationMs: number) => void;
	};
	terminalAttachState: {
		release: (id: string) => void;
	};
	terminalInstanceManager: {
		dispose: (id: string) => void;
	};
	terminalHandles: Map<string, THandle>;
	terminalKittyController: {
		resizeOverlay: (handle: THandle) => void;
	};
	terminalRendererAddonState: {
		load: (id: string, handle: THandle) => Promise<void> | void;
	};
};

type ModeBootstrapDepsInput<TKittyEvent> = {
	buildTerminalKey: (workspaceId: string, terminalId: string) => string;
	terminalContextRegistry: {
		getContext: (key: string) => TerminalContextLike | null;
	};
	logDebug: (id: string, event: string, details: Record<string, unknown>) => void;
	lifecycle: {
		markInput: (id: string) => void;
		getStatus: (id: string) => string;
		setStatusAndMessage: (id: string, status: string, message: string) => void;
		applyLifecyclePayload: (id: string, payload: TerminalLifecyclePayload) => void;
		setMode: (id: string, mode: TerminalModesState) => void;
	};
	bootstrapHandled: Map<string, boolean>;
	replayAckOrchestrator: {
		setReplayState: (id: string, state: 'idle' | 'replaying' | 'live') => void;
		pendingReplayKitty: Map<string, TKittyEvent[]>;
		initialCreditMap: Map<string, number>;
		pendingReplayOutput: Map<string, Array<{ bytes: number }>>;
	};
	terminalOutputBuffer: {
		enqueueOutput: (id: string, data: string, bytes: number) => void;
	};
	countBytes: (data: string) => number;
	terminalHandles: Map<string, unknown>;
	terminalKittyController: {
		applyEvent: (id: string, event: TKittyEvent) => Promise<void>;
	};
	setHealth: (id: string, state: 'unknown' | 'checking' | 'ok' | 'stale', message?: string) => void;
	initialStreamCredit: number;
	renderHealth: {
		scheduleBootstrapHealthCheck: (id: string, replayBytes: number) => void;
	};
	emitState: (id: string) => void;
	syncTerminalWebLinks: (id: string) => void;
};

type SyncControllerDepsInput<THandle> = {
	ensureGlobals: () => void;
	buildTerminalKey: (workspaceId: string, terminalId: string) => string;
	ensureContext: TerminalSyncControllerDependencies<THandle>['ensureContext'];
	terminalContextRegistry: {
		getLastWorkspaceId: (key: string) => string;
		setLastWorkspaceId: (key: string, workspaceId: string) => void;
		deleteContext: (key: string) => void;
	};
	attachTerminal: (id: string, container: HTMLDivElement | null, active: boolean) => unknown;
	terminalViewportResizeController: {
		attachResizeObserver: (id: string, container: HTMLDivElement) => void;
		detachResizeObserver: (id: string) => void;
		scheduleFitStabilization: (id: string, reason: string) => void;
		fitTerminal: (id: string, started: boolean) => void;
		focusTerminal: (id: string) => void;
		scrollToBottom: (id: string) => void;
		isAtBottom: (id: string) => boolean;
	};
	terminalStreamOrchestrator: {
		scheduleReattachCheck: (id: string, reason: string) => void;
		syncTerminalStream: (id: string) => void;
	};
	lifecycle: {
		hasStarted: (id: string) => boolean;
	};
	forceRedraw: (id: string) => void;
	terminalHandles: Map<string, THandle>;
	hasVisibleTerminalContent: (handle: THandle) => boolean;
	terminalResizeBridge: {
		nudgeRedraw: (id: string, handle: THandle) => void;
	};
	terminalAttachState: {
		markDetached: (id: string) => void;
	};
	terminalTransport: {
		stop: (workspaceId: string, terminalId: string) => Promise<void>;
	};
	terminalResourceLifecycle: {
		disposeTerminalResources: (id: string) => void;
	};
	beginTerminal: (id: string, quiet?: boolean) => Promise<void>;
	requestHealthCheck: (id: string) => void;
};

export const createTerminalSessionTransport = (transport: SessionTransportAdapter) => ({
	fetchStatus: (workspaceId: string, terminalId: string) =>
		transport.fetchStatus(workspaceId, terminalId),
	fetchBootstrap: (workspaceId: string, terminalId: string) =>
		transport.fetchBootstrap(workspaceId, terminalId),
	start: (workspaceId: string, terminalId: string) => transport.start(workspaceId, terminalId),
	write: (workspaceId: string, terminalId: string, data: string) =>
		transport.write(workspaceId, terminalId, data),
	fetchSettings: () => transport.fetchSettings(),
	fetchSessiondStatus: () => transport.fetchSessiondStatus(),
});

export const createTerminalResourceLifecycleDeps = <THandle>(
	input: ResourceLifecycleDepsInput<THandle>,
) => ({
	clearBootstrapFetchTimer: (id: string) => input.clearTimeoutMap(input.bootstrapFetchTimers, id),
	clearReattachTimer: (id: string) => input.terminalStreamOrchestrator.clearReattachTimer(id),
	destroyViewportState: (id: string) => input.terminalViewportResizeController.destroy(id),
	clearTerminalStore: (id: string) => input.terminalStores.delete(id),
	clearOutputBuffer: (id: string) => input.terminalOutputBuffer.clear(id),
	destroyReplayState: (id: string) => input.replayAckOrchestrator.destroy(id),
	resetReplaySession: (id: string) => input.replayAckOrchestrator.resetSession(id),
	deleteBootstrapHandled: (id: string) => input.bootstrapHandled.delete(id),
	deleteStats: (id: string) => input.terminalServiceState.deleteStats(id),
	deletePendingInput: (id: string) => input.terminalServiceState.deletePendingInput(id),
	clearResizeState: (id: string) => input.terminalResizeBridge.clear(id),
	releaseRenderHealth: (id: string) => input.renderHealth.release(id),
	clearRenderHealthSession: (id: string) => input.renderHealth.clearSession(id),
	deleteLifecycleState: (id: string) => input.lifecycle.deleteState(id),
	dropHealthCheck: (id: string) => input.lifecycle.dropHealthCheck(id),
	clearMouseSuppression: (id: string) => input.terminalMouseState.clearSuppression(id),
	clearMouseTail: (id: string) => input.terminalMouseState.clearTail(id),
	releaseAttachState: (id: string) => input.terminalAttachState.release(id),
	disposeTerminalInstance: (id: string) => input.terminalInstanceManager.dispose(id),
	getHandle: (id: string) => input.terminalHandles.get(id),
	resizeOverlay: (handle: THandle) => input.terminalKittyController.resizeOverlay(handle),
	setMode: (id: string, mode: TerminalModesState) => input.lifecycle.setMode(id, mode),
	loadRendererAddon: (id: string, handle: THandle) =>
		input.terminalRendererAddonState.load(id, handle),
	noteMouseSuppress: (id: string, durationMs: number) =>
		input.terminalMouseState.noteSuppress(id, durationMs),
});

export const createTerminalModeBootstrapCoordinatorDeps = <TKittyEvent>(
	input: ModeBootstrapDepsInput<TKittyEvent>,
) => ({
	buildTerminalKey: input.buildTerminalKey,
	getContext: (key: string) => input.terminalContextRegistry.getContext(key),
	logDebug: input.logDebug,
	markInput: (id: string) => input.lifecycle.markInput(id),
	bootstrapHandled: input.bootstrapHandled,
	setReplayState: input.replayAckOrchestrator.setReplayState,
	enqueueOutput: input.terminalOutputBuffer.enqueueOutput,
	countBytes: input.countBytes,
	pendingReplayKitty: input.replayAckOrchestrator.pendingReplayKitty,
	hasTerminalHandle: (id: string) => input.terminalHandles.has(id),
	applyKittyEvent: (id: string, event: TKittyEvent) =>
		input.terminalKittyController.applyEvent(id, event),
	setHealth: input.setHealth,
	initialCreditMap: input.replayAckOrchestrator.initialCreditMap,
	initialStreamCredit: input.initialStreamCredit,
	pendingReplayOutput: input.replayAckOrchestrator.pendingReplayOutput,
	getStatus: (id: string) => input.lifecycle.getStatus(id),
	setStatusAndMessage: (id: string, status: string, message: string) =>
		input.lifecycle.setStatusAndMessage(id, status, message),
	scheduleBootstrapHealthCheck: (id: string, replayBytes: number) =>
		input.renderHealth.scheduleBootstrapHealthCheck(id, replayBytes),
	emitState: input.emitState,
	applyLifecyclePayload: (id: string, payload: TerminalLifecyclePayload) =>
		input.lifecycle.applyLifecyclePayload(id, payload),
	setMode: (id: string, mode: TerminalModesState) => input.lifecycle.setMode(id, mode),
	syncTerminalWebLinks: input.syncTerminalWebLinks,
});

export const createTerminalSyncControllerDeps = <THandle>(
	input: SyncControllerDepsInput<THandle>,
): TerminalSyncControllerDependencies<THandle> => ({
	ensureGlobals: input.ensureGlobals,
	buildTerminalKey: input.buildTerminalKey,
	ensureContext: input.ensureContext,
	getLastWorkspaceId: (key: string) => input.terminalContextRegistry.getLastWorkspaceId(key),
	setLastWorkspaceId: (key: string, workspaceId: string) =>
		input.terminalContextRegistry.setLastWorkspaceId(key, workspaceId),
	deleteContext: (key: string) => input.terminalContextRegistry.deleteContext(key),
	attachTerminal: input.attachTerminal,
	attachResizeObserver: (id: string, container: HTMLDivElement) =>
		input.terminalViewportResizeController.attachResizeObserver(id, container),
	detachResizeObserver: (id: string) =>
		input.terminalViewportResizeController.detachResizeObserver(id),
	scheduleFitStabilization: (id: string, reason: string) =>
		input.terminalViewportResizeController.scheduleFitStabilization(id, reason),
	scheduleReattachCheck: (id: string, reason: string) =>
		input.terminalStreamOrchestrator.scheduleReattachCheck(id, reason),
	syncTerminalStream: (id: string) => input.terminalStreamOrchestrator.syncTerminalStream(id),
	fitTerminal: (id: string, started: boolean) =>
		input.terminalViewportResizeController.fitTerminal(id, started),
	hasStarted: (id: string) => input.lifecycle.hasStarted(id),
	forceRedraw: input.forceRedraw,
	getHandle: (id: string) => input.terminalHandles.get(id),
	hasVisibleTerminalContent: input.hasVisibleTerminalContent,
	nudgeRedraw: (id: string, handle: THandle) => input.terminalResizeBridge.nudgeRedraw(id, handle),
	markDetached: (id: string) => input.terminalAttachState.markDetached(id),
	stopTerminal: (workspaceId: string, terminalId: string) =>
		input.terminalTransport.stop(workspaceId, terminalId),
	disposeTerminalResources: (id: string) =>
		input.terminalResourceLifecycle.disposeTerminalResources(id),
	beginTerminal: (id: string, quiet?: boolean) => input.beginTerminal(id, quiet),
	requestHealthCheck: (id: string) => input.requestHealthCheck(id),
	focusTerminal: (id: string) => input.terminalViewportResizeController.focusTerminal(id),
	scrollToBottom: (id: string) => input.terminalViewportResizeController.scrollToBottom(id),
	isAtBottom: (id: string) => input.terminalViewportResizeController.isAtBottom(id),
});

type SessionCoordinatorBridge = {
	ensureSessionActive: (id: string) => Promise<void>;
	beginTerminal: (id: string, quiet?: boolean) => Promise<void>;
	ensureStream: (id: string) => Promise<void>;
	loadTerminalDefaults: () => Promise<void>;
	refreshSessiondStatus: () => Promise<void>;
	initTerminal: (
		id: string,
		options: {
			ensureListeners: () => void;
		},
	) => Promise<void>;
	handleSessiondRestarted: () => void;
};

export const createTerminalSessionBridge = (
	getCoordinator: () => SessionCoordinatorBridge,
	ensureListeners: () => void,
) => ({
	ensureSessionActive: (id: string): Promise<void> => getCoordinator().ensureSessionActive(id),
	beginTerminal: (id: string, quiet = false): Promise<void> =>
		getCoordinator().beginTerminal(id, quiet),
	ensureStream: (id: string): Promise<void> => getCoordinator().ensureStream(id),
	loadTerminalDefaults: (): Promise<void> => getCoordinator().loadTerminalDefaults(),
	refreshSessiondStatus: (): Promise<void> => getCoordinator().refreshSessiondStatus(),
	initTerminal: (id: string): Promise<void> =>
		getCoordinator().initTerminal(id, {
			ensureListeners,
		}),
	handleSessiondRestarted: (): void => getCoordinator().handleSessiondRestarted(),
});

type EnsureTerminalGlobalsDeps = {
	isInitialized: () => boolean;
	markInitialized: () => void;
	loadTerminalDefaults: () => Promise<void>;
	refreshSessiondStatus: () => Promise<void>;
	onFocus: (callback: () => void) => void;
	forEachAttached: (callback: (id: string) => void) => void;
	ensureSessionActive: (id: string) => Promise<void>;
};

export const ensureTerminalGlobals = (deps: EnsureTerminalGlobalsDeps): void => {
	if (deps.isInitialized()) return;
	deps.markInitialized();
	void deps.loadTerminalDefaults();
	void deps.refreshSessiondStatus();
	deps.onFocus(() => {
		deps.forEachAttached((id) => {
			void deps.ensureSessionActive(id);
		});
	});
};

export const clearLocalTerminalDebugPreference = (): void => {
	if (typeof localStorage === 'undefined') return;
	try {
		localStorage.removeItem('worksetTerminalDebug');
	} catch {
		// Ignore storage failures.
	}
};
