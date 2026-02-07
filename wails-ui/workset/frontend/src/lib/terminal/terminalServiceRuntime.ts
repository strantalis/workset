import type { TerminalDebugStats } from './terminalServiceState';

type RuntimeHandle = {
	terminal: {
		rows: number;
		refresh: (start: number, end: number) => void;
	};
};

type RuntimeDeps<THandle extends RuntimeHandle> = {
	lifecycle: {
		setHealth: (
			id: string,
			state: 'unknown' | 'checking' | 'ok' | 'stale',
			message?: string,
		) => void;
		requestHealthCheck: (
			id: string,
			options: {
				timeoutMs: number;
			},
		) => void;
	};
	terminalServiceState: {
		updateStats: (id: string, update: (stats: TerminalDebugStats) => void) => void;
		markLastOutput: (id: string) => void;
		markCprResponse: (id: string, data: string) => boolean;
	};
	terminalDebugState: {
		syncDebugEnabled: () => void;
		isDebugEnabled: () => boolean;
	};
	emitState: (id: string) => void;
	getWorkspaceId: (id: string) => string;
	getTerminalId: (id: string) => string;
	terminalTransport: {
		logDebug: (
			workspaceId: string,
			terminalId: string,
			event: string,
			details: string,
		) => Promise<void> | void;
	};
	terminalHandles: Map<string, THandle>;
	healthTimeoutMs: number;
};

export const createTerminalServiceRuntime = <THandle extends RuntimeHandle>(
	deps: RuntimeDeps<THandle>,
) => {
	const setHealth = (
		id: string,
		state: 'unknown' | 'checking' | 'ok' | 'stale',
		message = '',
	): void => {
		deps.lifecycle.setHealth(id, state, message);
	};

	const updateStats = (id: string, update: (stats: TerminalDebugStats) => void): void => {
		deps.terminalServiceState.updateStats(id, update);
		if (deps.terminalDebugState.isDebugEnabled()) {
			deps.emitState(id);
		}
	};

	const updateStatsLastOutput = (id: string): void => {
		deps.terminalServiceState.markLastOutput(id);
		if (deps.terminalDebugState.isDebugEnabled()) {
			deps.emitState(id);
		}
	};

	const captureCpr = (id: string, data: string): void => {
		if (!deps.terminalServiceState.markCprResponse(id, data)) return;
		if (deps.terminalDebugState.isDebugEnabled()) {
			deps.emitState(id);
		}
	};

	const logDebug = (id: string, event: string, details: Record<string, unknown>): void => {
		deps.terminalDebugState.syncDebugEnabled();
		if (!deps.terminalDebugState.isDebugEnabled()) return;
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		void deps.terminalTransport.logDebug(workspaceId, terminalId, event, JSON.stringify(details));
	};

	const forceRedraw = (id: string): void => {
		const handle = deps.terminalHandles.get(id);
		if (!handle) return;
		handle.terminal.refresh(0, handle.terminal.rows - 1);
	};

	const requestHealthCheck = (id: string): void => {
		deps.lifecycle.requestHealthCheck(id, { timeoutMs: deps.healthTimeoutMs });
	};

	return {
		setHealth,
		updateStats,
		updateStatsLastOutput,
		captureCpr,
		logDebug,
		forceRedraw,
		requestHealthCheck,
	};
};
