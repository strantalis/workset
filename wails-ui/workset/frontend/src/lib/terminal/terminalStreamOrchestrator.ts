type TerminalSyncContext = {
	container: HTMLDivElement | null;
	active: boolean;
};

type TerminalStreamOrchestratorDependencies = {
	ensureSessionActive: (id: string) => Promise<void>;
	initTerminal: (id: string) => Promise<void>;
	getContext: (id: string) => TerminalSyncContext | null;
	hasStarted: (id: string) => boolean;
	getStatus: (id: string) => string;
	ensureStream: (id: string) => Promise<void>;
	beginTerminal: (id: string, quiet?: boolean) => Promise<void>;
	emitState: (id: string) => void;
	logDebug: (id: string, event: string, details: Record<string, unknown>) => void;
	setTimeoutFn?: (handler: () => void, timeoutMs: number) => number;
	clearTimeoutFn?: (timer: number) => void;
};

const REATTACH_DELAY_MS = 240;

export const createTerminalStreamOrchestrator = (deps: TerminalStreamOrchestratorDependencies) => {
	const setTimeoutFn =
		deps.setTimeoutFn ?? ((handler, timeoutMs) => window.setTimeout(handler, timeoutMs));
	const clearTimeoutFn = deps.clearTimeoutFn ?? ((timer) => window.clearTimeout(timer));
	const reattachTimers = new Map<string, number>();

	const clearReattachTimer = (id: string): void => {
		const timer = reattachTimers.get(id);
		if (!timer) return;
		clearTimeoutFn(timer);
		reattachTimers.delete(id);
	};

	const scheduleReattachCheck = (id: string, reason: string): void => {
		const existing = reattachTimers.get(id);
		if (existing) {
			clearTimeoutFn(existing);
			reattachTimers.delete(id);
		}
		reattachTimers.set(
			id,
			setTimeoutFn(() => {
				clearReattachTimer(id);
				void deps.ensureSessionActive(id);
			}, REATTACH_DELAY_MS),
		);
		deps.logDebug(id, 'reattach', { reason });
	};

	const syncTerminalStream = (id: string): void => {
		void (async () => {
			await deps.initTerminal(id);
			const current = deps.getContext(id);
			if (current?.container) {
				if (deps.hasStarted(id)) {
					void deps.ensureStream(id);
				} else if (deps.getStatus(id) === 'standby') {
					await deps.beginTerminal(id, !current.active);
				}
			}
			await deps.ensureSessionActive(id);
			deps.emitState(id);
		})();
	};

	return {
		clearReattachTimer,
		scheduleReattachCheck,
		syncTerminalStream,
	};
};
