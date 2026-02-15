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
	markStopped: (id: string) => void;
	setStatusAndMessage: (id: string, status: string, message: string) => void;
	setInput: (id: string, value: boolean) => void;
	ensureRendererDefaults: (id: string) => void;
	markStartInFlight: (id: string) => void;
	clearStartInFlight: (id: string) => void;
	clearStartupTimeout: (id: string) => void;
	dropHealthCheck: (id: string) => void;
	setSessiondStatus: (available: boolean | null) => void;
};

type TerminalCoordinatorDependencies = {
	lifecycle: TerminalCoordinatorLifecycle;
	getWorkspaceId: (id: string) => string;
	getTerminalId: (id: string) => string;
	transport: {
		start: (workspaceId: string, terminalId: string) => Promise<void>;
		write: (workspaceId: string, terminalId: string, data: string) => Promise<void>;
		fetchSettings: () => Promise<TerminalSettingsPayload | null>;
		fetchSessiondStatus: () => Promise<TerminalSessiondStatusPayload | null>;
	};
	setHealth: (id: string, state: 'unknown' | 'checking' | 'ok' | 'stale', message?: string) => void;
	emitState: (id: string) => void;
	pendingInput: Map<string, string>;
	logDebug: (id: string, event: string, details: Record<string, unknown>) => void;
	resetSessionState: (id: string) => void;
	writeStartFailureMessage: (id: string, message: string) => void;
	getDebugOverlayPreference: () => 'on' | 'off' | '';
	setDebugOverlayPreference: (value: 'on' | 'off' | '') => void;
	clearLocalDebugPreference: () => void;
	syncDebugEnabled: () => void;
};

export const createTerminalSessionCoordinator = (deps: TerminalCoordinatorDependencies) => {
	const log = (id: string, event: string, details: Record<string, unknown>): void => {
		deps.logDebug(id, event, details);
	};

	const refreshSessiondStatus = async (): Promise<void> => {
		try {
			const status = await deps.transport.fetchSessiondStatus();
			deps.lifecycle.setSessiondStatus(status?.available ?? false);
		} catch {
			deps.lifecycle.setSessiondStatus(false);
		}
	};

	const beginTerminal = async (id: string, quiet = false): Promise<void> => {
		if (!id) return;
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) {
			log(id, 'session_begin_skip_missing_ids', {
				hasWorkspaceId: Boolean(workspaceId),
				hasTerminalId: Boolean(terminalId),
			});
			return;
		}
		if (deps.lifecycle.hasStartInFlight(id)) {
			log(id, 'session_begin_skip_in_flight', { quiet });
			return;
		}
		if (deps.lifecycle.hasStarted(id)) {
			if (quiet) {
				log(id, 'session_begin_skip_started', { quiet });
				return;
			}
			log(id, 'session_begin_reassert_start', { quiet, workspaceId, terminalId });
			try {
				await deps.transport.start(workspaceId, terminalId);
				deps.lifecycle.setInput(id, true);
				deps.lifecycle.setStatusAndMessage(id, 'ready', '');
				deps.setHealth(id, 'ok', 'Session active.');
				deps.lifecycle.ensureRendererDefaults(id);
				deps.emitState(id);
				log(id, 'session_begin_reassert_ok', { quiet });
				return;
			} catch (error) {
				// Fall through to full start path if the backend no longer has the session.
				deps.lifecycle.markStopped(id);
				log(id, 'session_begin_reassert_error', {
					quiet,
					error: String(error),
				});
			}
		}
		if (deps.lifecycle.hasStarted(id)) {
			log(id, 'session_begin_skip_started', { quiet });
			return;
		}

		log(id, 'session_begin_start', {
			quiet,
			workspaceId,
			terminalId,
		});

		deps.lifecycle.markStartInFlight(id);
		deps.resetSessionState(id);
		if (!quiet) {
			deps.lifecycle.setStatusAndMessage(id, 'starting', 'Waiting for shell output…');
			deps.setHealth(id, 'unknown');
			deps.lifecycle.setInput(id, false);
			deps.emitState(id);
		}

		try {
			log(id, 'session_begin_transport_start', {});
			await deps.transport.start(workspaceId, terminalId);
			deps.lifecycle.markStarted(id);
			deps.lifecycle.setInput(id, true);
			deps.lifecycle.setStatusAndMessage(id, 'ready', '');
			deps.setHealth(id, 'ok', 'Session active.');
			deps.lifecycle.ensureRendererDefaults(id);
			log(id, 'session_begin_transport_ok', {});
			const queued = deps.pendingInput.get(id);
			if (queued) {
				log(id, 'session_begin_flush_pending', {
					bytes: queued.length,
				});
				deps.pendingInput.delete(id);
				try {
					await deps.transport.write(workspaceId, terminalId, queued);
					log(id, 'session_begin_flush_pending_ok', {
						bytes: queued.length,
					});
				} catch {
					deps.pendingInput.set(id, queued + (deps.pendingInput.get(id) ?? ''));
					log(id, 'session_begin_flush_pending_error', {
						bytes: queued.length,
					});
				}
			}
			deps.emitState(id);
		} catch (error) {
			deps.lifecycle.setStatusAndMessage(id, 'error', String(error));
			deps.setHealth(id, 'stale', 'Failed to start terminal.');
			deps.pendingInput.delete(id);
			deps.writeStartFailureMessage(id, String(error));
			deps.emitState(id);
			log(id, 'session_begin_error', {
				error: String(error),
			});
		} finally {
			deps.lifecycle.clearStartupTimeout(id);
			deps.lifecycle.clearStartInFlight(id);
			log(id, 'session_begin_done', {});
		}
	};

	const ensureSessionActive = async (id: string): Promise<void> => {
		await beginTerminal(id, true);
	};

	const initTerminal = async (
		id: string,
		options: {
			ensureListeners: () => void;
		},
	): Promise<void> => {
		if (!id) return;
		log(id, 'session_init_start', {});
		options.ensureListeners();
		deps.lifecycle.dropHealthCheck(id);
		deps.lifecycle.ensureRendererDefaults(id);
		if (!deps.lifecycle.hasStarted(id) && !deps.lifecycle.hasStartInFlight(id)) {
			deps.lifecycle.setStatusAndMessage(id, 'standby', '');
			deps.setHealth(id, 'unknown');
			deps.lifecycle.setInput(id, false);
			deps.emitState(id);
		}
		log(id, 'session_init_done', {});
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
		initTerminal,
	};
};
