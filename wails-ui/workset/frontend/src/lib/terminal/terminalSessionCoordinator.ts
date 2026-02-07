type TerminalCoordinatorContext = {
	container: HTMLDivElement | null;
	active: boolean;
};

type TerminalStatusPayload = {
	active?: boolean | null;
};

type TerminalBootstrapPayload = {
	workspaceId: string;
	terminalId: string;
	source?: string;
	snapshotSource?: string;
	backlogSource?: string;
	backlogTruncated?: boolean;
};

type TerminalSettingsPayload = {
	defaults?: {
		terminalDebugOverlay?: string;
	};
};

type TerminalSessiondStatusPayload = {
	available?: boolean | null;
};

type TerminalCoordinatorLifecycle = {
	hasStarted: (id: string) => boolean;
	hasStartInFlight: (id: string) => boolean;
	isSessiondAvailable: () => boolean | null;
	markStarted: (id: string) => void;
	setStatusAndMessage: (id: string, status: string, message: string) => void;
	setInput: (id: string, value: boolean) => void;
	ensureRendererDefaults: (id: string) => void;
	markStartInFlight: (id: string) => void;
	clearStartInFlight: (id: string) => void;
	scheduleStartupTimeout: (
		id: string,
		options: {
			timeoutMs: number;
			onTimeout?: () => void;
		},
	) => void;
	clearStartupTimeout: (id: string) => void;
	dropHealthCheck: (id: string) => void;
	nextInitToken: (id: string) => number;
	isCurrentInitToken: (id: string, token: number) => boolean;
	isSessiondChecked: () => boolean;
	setSessiondStatus: (available: boolean | null) => void;
	resetSessiondChecked: () => void;
	clearSessionFlags: (id: string) => void;
};

type TerminalCoordinatorDependencies = {
	lifecycle: TerminalCoordinatorLifecycle;
	getWorkspaceId: (id: string) => string;
	getTerminalId: (id: string) => string;
	getContext: (id: string) => TerminalCoordinatorContext | null;
	attachTerminal: (id: string, container: HTMLDivElement | null, active: boolean) => unknown;
	terminalIds: () => IterableIterator<string>;
	transport: {
		fetchStatus: (workspaceId: string, terminalId: string) => Promise<TerminalStatusPayload | null>;
		fetchBootstrap: (workspaceId: string, terminalId: string) => Promise<TerminalBootstrapPayload>;
		start: (workspaceId: string, terminalId: string) => Promise<void>;
		write: (workspaceId: string, terminalId: string, data: string) => Promise<void>;
		fetchSettings: () => Promise<TerminalSettingsPayload | null>;
		fetchSessiondStatus: () => Promise<TerminalSessiondStatusPayload | null>;
	};
	setHealth: (id: string, state: 'unknown' | 'checking' | 'ok' | 'stale', message?: string) => void;
	emitState: (id: string) => void;
	pendingInput: Map<string, string>;
	bootstrapHandled: Map<string, boolean>;
	bootstrapFetchTimers: Map<string, number>;
	replaySetState: (id: string, state: 'idle' | 'replaying' | 'live') => void;
	logDebug: (id: string, event: string, details: Record<string, unknown>) => void;
	handleBootstrapPayload: (payload: TerminalBootstrapPayload) => void;
	handleBootstrapDonePayload: (payload: { workspaceId: string; terminalId: string }) => void;
	resetSessionState: (id: string) => void;
	resetTerminalInstance: (id: string) => void;
	noteMouseSuppress: (id: string, durationMs: number) => void;
	writeStartFailureMessage: (id: string, message: string) => void;
	getDebugOverlayPreference: () => 'on' | 'off' | '';
	setDebugOverlayPreference: (value: 'on' | 'off' | '') => void;
	clearLocalDebugPreference: () => void;
	syncDebugEnabled: () => void;
	bootstrapFetchDelayMs?: number;
};

export const createTerminalSessionCoordinator = (deps: TerminalCoordinatorDependencies) => {
	const maybeFetchBootstrap = async (id: string, reason: string): Promise<void> => {
		if (!id) return;
		if (deps.bootstrapHandled.get(id)) return;
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		try {
			const payload = await deps.transport.fetchBootstrap(workspaceId, terminalId);
			if (deps.bootstrapHandled.get(id)) return;
			deps.handleBootstrapPayload(payload);
			deps.handleBootstrapDonePayload({ workspaceId, terminalId });
			deps.logDebug(id, 'bootstrap_fetch', {
				reason,
				source: payload.source,
				snapshotSource: payload.snapshotSource,
				backlogSource: payload.backlogSource,
				backlogTruncated: payload.backlogTruncated,
			});
		} catch (error) {
			deps.logDebug(id, 'bootstrap_fetch_failed', { reason, error: String(error) });
		}
	};

	const refreshSessiondStatus = async (): Promise<void> => {
		try {
			const status = await deps.transport.fetchSessiondStatus();
			deps.lifecycle.setSessiondStatus(status?.available ?? false);
		} catch {
			deps.lifecycle.setSessiondStatus(false);
		}
	};

	const ensureSessionActive = async (id: string): Promise<void> => {
		if (deps.lifecycle.hasStarted(id) || deps.lifecycle.hasStartInFlight(id)) return;
		if (deps.lifecycle.isSessiondAvailable() !== true) return;
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		try {
			const status = await deps.transport.fetchStatus(workspaceId, terminalId);
			if (status?.active) {
				deps.lifecycle.markStarted(id);
				deps.lifecycle.setStatusAndMessage(id, 'ready', '');
				deps.setHealth(id, 'ok', 'Session resumed.');
				deps.lifecycle.setInput(id, true);
				deps.lifecycle.ensureRendererDefaults(id);
				deps.emitState(id);
			}
		} catch {
			// Ignore.
		}
	};

	const beginTerminal = async (id: string, quiet = false): Promise<void> => {
		if (!id || deps.lifecycle.hasStarted(id) || deps.lifecycle.hasStartInFlight(id)) return;
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		deps.lifecycle.markStartInFlight(id);
		deps.resetSessionState(id);
		deps.bootstrapHandled.set(id, false);
		deps.replaySetState(id, 'replaying');
		if (!quiet) {
			deps.lifecycle.setStatusAndMessage(id, 'starting', 'Waiting for shell output…');
			deps.setHealth(id, 'unknown');
			deps.lifecycle.setInput(id, false);
			deps.lifecycle.scheduleStartupTimeout(id, {
				timeoutMs: 2000,
				onTimeout: () => {
					deps.pendingInput.delete(id);
				},
			});
			deps.emitState(id);
		}
		try {
			await deps.transport.start(workspaceId, terminalId);
			deps.lifecycle.markStarted(id);
			const queued = deps.pendingInput.get(id);
			if (queued) {
				deps.pendingInput.delete(id);
				await deps.transport.write(workspaceId, terminalId, queued);
			}
			const existingTimer = deps.bootstrapFetchTimers.get(id);
			if (existingTimer) {
				window.clearTimeout(existingTimer);
			}
			deps.bootstrapFetchTimers.set(
				id,
				window.setTimeout(() => {
					deps.bootstrapFetchTimers.delete(id);
					if (!deps.bootstrapHandled.get(id)) {
						void maybeFetchBootstrap(id, 'bootstrap_timeout');
					}
				}, deps.bootstrapFetchDelayMs ?? 200),
			);
		} catch (error) {
			deps.lifecycle.setStatusAndMessage(id, 'error', String(error));
			deps.setHealth(id, 'stale', 'Failed to start terminal.');
			deps.lifecycle.clearStartupTimeout(id);
			deps.pendingInput.delete(id);
			deps.writeStartFailureMessage(id, String(error));
			deps.emitState(id);
		} finally {
			deps.lifecycle.clearStartInFlight(id);
		}
	};

	const ensureStream = async (id: string): Promise<void> => {
		if (!id || deps.lifecycle.hasStartInFlight(id)) return;
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		deps.lifecycle.markStartInFlight(id);
		try {
			await deps.transport.start(workspaceId, terminalId);
			deps.lifecycle.markStarted(id);
		} catch (error) {
			deps.logDebug(id, 'ensure_stream_failed', { error: String(error) });
		} finally {
			deps.lifecycle.clearStartInFlight(id);
		}
	};

	const initTerminal = async (
		id: string,
		options: {
			ensureListeners: () => void;
		},
	): Promise<void> => {
		if (!id) return;
		const token = deps.lifecycle.nextInitToken(id);
		options.ensureListeners();
		if (!deps.lifecycle.isSessiondChecked()) {
			await refreshSessiondStatus();
		}
		const ctx = deps.getContext(id);
		deps.attachTerminal(id, ctx?.container ?? null, ctx?.active ?? false);
		let resumed = false;
		if (deps.lifecycle.isSessiondAvailable() === true) {
			try {
				const workspaceId = deps.getWorkspaceId(id);
				const terminalId = deps.getTerminalId(id);
				if (workspaceId && terminalId) {
					const status = await deps.transport.fetchStatus(workspaceId, terminalId);
					resumed = status?.active ?? false;
				}
			} catch {
				resumed = false;
			}
		}
		if (resumed) {
			await beginTerminal(id, true);
			deps.lifecycle.setInput(id, true);
			deps.lifecycle.setStatusAndMessage(id, 'ready', '');
			deps.setHealth(id, 'ok', 'Session resumed.');
			deps.lifecycle.ensureRendererDefaults(id);
			deps.emitState(id);
			return;
		}
		if (!deps.lifecycle.isCurrentInitToken(id, token)) return;
		deps.lifecycle.dropHealthCheck(id);
		if (!deps.lifecycle.hasStarted(id) && !deps.lifecycle.hasStartInFlight(id)) {
			deps.lifecycle.setStatusAndMessage(id, 'standby', '');
			deps.setHealth(id, 'unknown');
			deps.lifecycle.ensureRendererDefaults(id);
			deps.lifecycle.setInput(id, false);
			deps.emitState(id);
		}
	};

	const handleSessiondRestarted = (): void => {
		deps.lifecycle.resetSessiondChecked();
		void (async () => {
			await refreshSessiondStatus();
			if (deps.lifecycle.isSessiondAvailable() !== true) return;
			for (const id of deps.terminalIds()) {
				deps.lifecycle.clearSessionFlags(id);
				deps.resetTerminalInstance(id);
				deps.resetSessionState(id);
				deps.noteMouseSuppress(id, 4000);
				void beginTerminal(id, true);
			}
		})();
	};

	const loadTerminalDefaults = async (): Promise<void> => {
		let nextDebugPreference = deps.getDebugOverlayPreference();
		try {
			const settings = await deps.transport.fetchSettings();
			const rawPreference = settings?.defaults?.terminalDebugOverlay ?? '';
			const normalizedPreference = rawPreference.toLowerCase().trim();
			if (normalizedPreference === 'on' || normalizedPreference === 'off') {
				nextDebugPreference = normalizedPreference;
			}
		} catch {
			// Keep existing preference on load failure.
		}
		deps.setDebugOverlayPreference(nextDebugPreference);
		if (nextDebugPreference === 'off') {
			deps.clearLocalDebugPreference();
		}
		deps.syncDebugEnabled();
	};

	return {
		loadTerminalDefaults,
		refreshSessiondStatus,
		ensureSessionActive,
		beginTerminal,
		ensureStream,
		initTerminal,
		handleSessiondRestarted,
	};
};
