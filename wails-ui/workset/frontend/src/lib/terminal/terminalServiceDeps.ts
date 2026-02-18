import type { TerminalSyncControllerDependencies } from './terminalSyncController';
import type { TerminalTransport } from './terminalTransport';

type SessionTransportAdapter = Pick<
	TerminalTransport,
	'start' | 'write' | 'fetchSettings' | 'fetchSessiondStatus'
>;

type ResourceLifecycleDepsInput<THandle> = {
	terminalViewportResizeController: {
		destroy: (id: string) => void;
	};
	terminalStores: {
		delete: (id: string) => void;
	};
	terminalServiceState: {
		deleteStats: (id: string) => void;
		deletePendingInput: (id: string) => void;
		deletePendingOutput: (id: string) => void;
	};
	terminalResizeBridge: {
		clear: (id: string) => void;
	};
	lifecycle: {
		deleteState: (id: string) => void;
		dropHealthCheck: (id: string) => void;
	};
	terminalAttachState: {
		release: (id: string) => void;
	};
	terminalInstanceManager: {
		dispose: (id: string) => void;
	};
	terminalHandles: Map<string, THandle>;
};

type SyncControllerDepsInput = {
	ensureGlobals: () => void;
	buildTerminalKey: (workspaceId: string, terminalId: string) => string;
	ensureContext: TerminalSyncControllerDependencies['ensureContext'];
	trace?: TerminalSyncControllerDependencies['trace'];
	terminalContextRegistry: {
		getContext: (key: string) => unknown;
		deleteContext: (key: string) => void;
	};
	attachTerminal: (id: string, container: HTMLDivElement | null, active: boolean) => unknown;
	terminalViewportResizeController: {
		attachResizeObserver: (id: string, container: HTMLDivElement) => void;
		detachResizeObserver: (id: string) => void;
		fitTerminal: (id: string, started: boolean) => void;
		focusTerminal: (id: string) => void;
		scrollToBottom: (id: string) => void;
		isAtBottom: (id: string) => boolean;
	};
	terminalStreamOrchestrator: {
		syncTerminalStream: (id: string) => void;
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
};

export const createTerminalSessionTransport = (transport: SessionTransportAdapter) => ({
	start: (workspaceId: string, terminalId: string) => transport.start(workspaceId, terminalId),
	write: (workspaceId: string, terminalId: string, data: string) =>
		transport.write(workspaceId, terminalId, data),
	fetchSettings: () => transport.fetchSettings(),
	fetchSessiondStatus: () => transport.fetchSessiondStatus(),
});

export const createTerminalResourceLifecycleDeps = <THandle>(
	input: ResourceLifecycleDepsInput<THandle>,
) => ({
	destroyViewportState: (id: string) => input.terminalViewportResizeController.destroy(id),
	clearTerminalStore: (id: string) => input.terminalStores.delete(id),
	deleteStats: (id: string) => input.terminalServiceState.deleteStats(id),
	deletePendingInput: (id: string) => input.terminalServiceState.deletePendingInput(id),
	deletePendingOutput: (id: string) => input.terminalServiceState.deletePendingOutput(id),
	clearResizeState: (id: string) => input.terminalResizeBridge.clear(id),
	deleteLifecycleState: (id: string) => input.lifecycle.deleteState(id),
	dropHealthCheck: (id: string) => input.lifecycle.dropHealthCheck(id),
	releaseAttachState: (id: string) => input.terminalAttachState.release(id),
	disposeTerminalInstance: (id: string) => input.terminalInstanceManager.dispose(id),
	getHandle: (id: string) => input.terminalHandles.get(id),
});

export const createTerminalSyncControllerDeps = (
	input: SyncControllerDepsInput,
): TerminalSyncControllerDependencies => ({
	ensureGlobals: input.ensureGlobals,
	buildTerminalKey: input.buildTerminalKey,
	ensureContext: input.ensureContext,
	hasContext: (key: string) => Boolean(input.terminalContextRegistry.getContext(key)),
	deleteContext: (key: string) => input.terminalContextRegistry.deleteContext(key),
	attachTerminal: input.attachTerminal,
	attachResizeObserver: (id: string, container: HTMLDivElement) =>
		input.terminalViewportResizeController.attachResizeObserver(id, container),
	detachResizeObserver: (id: string) =>
		input.terminalViewportResizeController.detachResizeObserver(id),
	syncTerminalStream: (id: string) => input.terminalStreamOrchestrator.syncTerminalStream(id),
	markDetached: (id: string) => input.terminalAttachState.markDetached(id),
	stopTerminal: (workspaceId: string, terminalId: string) =>
		input.terminalTransport.stop(workspaceId, terminalId),
	disposeTerminalResources: (id: string) =>
		input.terminalResourceLifecycle.disposeTerminalResources(id),
	focusTerminal: (id: string) => input.terminalViewportResizeController.focusTerminal(id),
	scrollToBottom: (id: string) => input.terminalViewportResizeController.scrollToBottom(id),
	isAtBottom: (id: string) => input.terminalViewportResizeController.isAtBottom(id),
	trace: input.trace,
});

type SessionCoordinatorBridge = {
	ensureSessionActive: (id: string) => Promise<void>;
	beginTerminal: (id: string, quiet?: boolean) => Promise<void>;
	loadTerminalDefaults: () => Promise<void>;
	refreshSessiondStatus: () => Promise<void>;
	initTerminal: (
		id: string,
		options: {
			ensureListeners: () => void;
		},
	) => Promise<void>;
};

export const createTerminalSessionBridge = (
	getCoordinator: () => SessionCoordinatorBridge,
	ensureListeners: () => void,
) => ({
	ensureSessionActive: (id: string): Promise<void> => getCoordinator().ensureSessionActive(id),
	beginTerminal: (id: string, quiet = false): Promise<void> =>
		getCoordinator().beginTerminal(id, quiet),
	loadTerminalDefaults: (): Promise<void> => getCoordinator().loadTerminalDefaults(),
	refreshSessiondStatus: (): Promise<void> => getCoordinator().refreshSessiondStatus(),
	initTerminal: (id: string): Promise<void> =>
		getCoordinator().initTerminal(id, {
			ensureListeners,
		}),
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
