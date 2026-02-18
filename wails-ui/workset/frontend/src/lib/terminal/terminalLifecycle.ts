export type TerminalHealthState = 'unknown' | 'checking' | 'ok' | 'stale';
export type TerminalRendererState = 'unknown' | 'webgl';
export type TerminalRendererMode = 'webgl';

export type TerminalLifecycleSnapshot = {
	status: string;
	message: string;
	health: TerminalHealthState;
	healthMessage: string;
	renderer: TerminalRendererState;
	rendererMode: TerminalRendererMode;
	sessiondAvailable: boolean | null;
	sessiondChecked: boolean;
};

type StartupTimeoutOptions = {
	timeoutMs: number;
	onTimeout?: () => void;
};

type HealthCheckOptions = {
	timeoutMs: number;
};

type TerminalLifecycleDependencies = {
	emitState: (id: string) => void;
	emitAllStates: () => void;
	setTimeoutFn?: (handler: () => void, timeoutMs: number) => number;
	clearTimeoutFn?: (timer: number) => void;
};

const deleteRecordKey = <T>(record: Record<string, T>, id: string): Record<string, T> => {
	if (!Object.prototype.hasOwnProperty.call(record, id)) return record;
	const next = { ...record };
	delete next[id];
	return next;
};

export const createTerminalLifecycle = (deps: TerminalLifecycleDependencies) => {
	const setTimeoutFn =
		deps.setTimeoutFn ?? ((handler, timeoutMs) => window.setTimeout(handler, timeoutMs));
	const clearTimeoutFn = deps.clearTimeoutFn ?? ((timer) => window.clearTimeout(timer));

	const startupTimers = new Map<string, number>();
	const pendingHealthCheck = new Map<string, number>();
	const initTokens = new Map<string, number>();
	const startedSessions = new Set<string>();
	const startInFlight = new Set<string>();

	let sessiondAvailable: boolean | null = null;
	let sessiondChecked = false;
	let statusMap: Record<string, string> = {};
	let messageMap: Record<string, string> = {};
	let inputMap: Record<string, boolean> = {};
	let healthMap: Record<string, TerminalHealthState> = {};
	let healthMessageMap: Record<string, string> = {};
	let rendererMap: Record<string, TerminalRendererState> = {};
	let rendererModeMap: Record<string, TerminalRendererMode> = {};

	const clearTimeoutMap = (map: Map<string, number>, id: string): void => {
		const timer = map.get(id);
		if (!timer) return;
		clearTimeoutFn(timer);
		map.delete(id);
	};

	const setHealth = (id: string, state: TerminalHealthState, message = ''): void => {
		healthMap = { ...healthMap, [id]: state };
		if (message) {
			healthMessageMap = { ...healthMessageMap, [id]: message };
		} else if (healthMessageMap[id]) {
			healthMessageMap = { ...healthMessageMap, [id]: '' };
		}
		deps.emitState(id);
	};

	const clearStartupTimeout = (id: string): void => {
		clearTimeoutMap(startupTimers, id);
	};

	const clearHealthCheck = (id: string): void => {
		clearTimeoutMap(pendingHealthCheck, id);
	};

	return {
		getSnapshot: (id: string): TerminalLifecycleSnapshot => ({
			status: statusMap[id] ?? '',
			message: messageMap[id] ?? '',
			health: healthMap[id] ?? 'unknown',
			healthMessage: healthMessageMap[id] ?? '',
			renderer: rendererMap[id] ?? 'unknown',
			rendererMode: rendererModeMap[id] ?? 'webgl',
			sessiondAvailable,
			sessiondChecked,
		}),
		setHealth,
		setStatus: (id: string, status: string): void => {
			statusMap = { ...statusMap, [id]: status };
		},
		setMessage: (id: string, message: string): void => {
			messageMap = { ...messageMap, [id]: message };
		},
		setStatusAndMessage: (id: string, status: string, message: string): void => {
			statusMap = { ...statusMap, [id]: status };
			messageMap = { ...messageMap, [id]: message };
		},
		getStatus: (id: string): string => statusMap[id] ?? '',
		hasInput: (id: string): boolean => inputMap[id] ?? false,
		setInput: (id: string, value: boolean): void => {
			inputMap = { ...inputMap, [id]: value };
		},
		markInput: (id: string): void => {
			if (inputMap[id]) return;
			inputMap = { ...inputMap, [id]: true };
		},
		ensureRendererDefaults: (id: string): void => {
			if (!rendererMap[id]) {
				rendererMap = { ...rendererMap, [id]: 'unknown' };
			}
			if (!rendererModeMap[id]) {
				rendererModeMap = { ...rendererModeMap, [id]: 'webgl' };
			}
		},
		setRenderer: (id: string, renderer: TerminalRendererState): void => {
			rendererMap = { ...rendererMap, [id]: renderer };
		},
		setRendererMode: (id: string, mode: TerminalRendererMode): void => {
			rendererModeMap = { ...rendererModeMap, [id]: mode };
		},
		hasStarted: (id: string): boolean => startedSessions.has(id),
		markStarted: (id: string): void => {
			startedSessions.add(id);
		},
		markStopped: (id: string): void => {
			startedSessions.delete(id);
		},
		getStartedTerminalIds: (): string[] => Array.from(startedSessions),
		hasStartInFlight: (id: string): boolean => startInFlight.has(id),
		markStartInFlight: (id: string): void => {
			startInFlight.add(id);
		},
		clearStartInFlight: (id: string): void => {
			startInFlight.delete(id);
		},
		clearSessionFlags: (id: string): void => {
			startedSessions.delete(id);
			startInFlight.delete(id);
		},
		isSessiondChecked: (): boolean => sessiondChecked,
		isSessiondAvailable: (): boolean | null => sessiondAvailable,
		setSessiondStatus: (available: boolean | null): void => {
			sessiondAvailable = available;
			sessiondChecked = true;
			deps.emitAllStates();
		},
		resetSessiondChecked: (): void => {
			sessiondChecked = false;
		},
		scheduleStartupTimeout: (id: string, options: StartupTimeoutOptions): void => {
			clearStartupTimeout(id);
			startupTimers.set(
				id,
				setTimeoutFn(() => {
					startupTimers.delete(id);
					if (startedSessions.has(id)) return;
					statusMap = { ...statusMap, [id]: 'error' };
					messageMap = { ...messageMap, [id]: 'Terminal startup timed out.' };
					setHealth(id, 'stale', 'Terminal startup timed out.');
					options.onTimeout?.();
					deps.emitState(id);
				}, options.timeoutMs),
			);
		},
		clearStartupTimeout,
		requestHealthCheck: (id: string, options: HealthCheckOptions): void => {
			clearHealthCheck(id);
			setHealth(id, 'checking', 'Checking session health…');
			pendingHealthCheck.set(
				id,
				setTimeoutFn(() => {
					pendingHealthCheck.delete(id);
					if (!startedSessions.has(id)) {
						setHealth(id, 'stale', 'Session not active.');
					}
				}, options.timeoutMs),
			);
		},
		clearHealthCheck,
		dropHealthCheck: (id: string): void => {
			pendingHealthCheck.delete(id);
		},
		nextInitToken: (id: string): number => {
			const token = (initTokens.get(id) ?? 0) + 1;
			initTokens.set(id, token);
			return token;
		},
		isCurrentInitToken: (id: string, token: number): boolean => initTokens.get(id) === token,
		clearInitToken: (id: string): void => {
			initTokens.delete(id);
		},
		deleteState: (id: string): void => {
			if (!id) return;
			clearStartupTimeout(id);
			clearHealthCheck(id);
			startInFlight.delete(id);
			startedSessions.delete(id);
			initTokens.delete(id);
			statusMap = deleteRecordKey(statusMap, id);
			messageMap = deleteRecordKey(messageMap, id);
			inputMap = deleteRecordKey(inputMap, id);
			healthMap = deleteRecordKey(healthMap, id);
			healthMessageMap = deleteRecordKey(healthMessageMap, id);
			rendererMap = deleteRecordKey(rendererMap, id);
			rendererModeMap = deleteRecordKey(rendererModeMap, id);
		},
	};
};
