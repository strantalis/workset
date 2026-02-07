type FitDimensions = { cols: number; rows: number };

type TerminalLike = {
	write: (data: string) => void;
};

type FitAddonLike = {
	proposeDimensions: () => FitDimensions | undefined;
};

export type TerminalResizeHandle = {
	terminal: TerminalLike;
	fitAddon: FitAddonLike;
};

type TerminalResizeBridgeDeps = {
	getWorkspaceId: (id: string) => string;
	getTerminalId: (id: string) => string;
	resize: (workspaceId: string, terminalId: string, cols: number, rows: number) => Promise<unknown>;
	logDebug?: (id: string, event: string, details: Record<string, unknown>) => void;
	setTimeoutFn?: (callback: () => void, delayMs: number) => ReturnType<typeof setTimeout>;
	nudgeDelayMs?: number;
};

const DEFAULT_NUDGE_DELAY_MS = 60;

export const createTerminalResizeBridge = (deps: TerminalResizeBridgeDeps) => {
	const pendingRedraw = new Set<string>();
	const setTimeoutFn =
		deps.setTimeoutFn ?? ((callback, delayMs) => window.setTimeout(callback, delayMs));
	const nudgeDelayMs = deps.nudgeDelayMs ?? DEFAULT_NUDGE_DELAY_MS;

	const resizeToFit = (id: string, handle: TerminalResizeHandle | undefined): void => {
		if (!handle) return;
		const dims = handle.fitAddon.proposeDimensions();
		if (!dims) return;
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		void deps.resize(workspaceId, terminalId, dims.cols, dims.rows).catch(() => undefined);
	};

	const nudgeRedraw = (id: string, handle: TerminalResizeHandle | undefined): void => {
		if (!handle || pendingRedraw.has(id)) return;
		const dims = handle.fitAddon.proposeDimensions();
		if (!dims) return;
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) {
			handle.terminal.write('');
			return;
		}
		const cols = Math.max(2, dims.cols);
		const rows = Math.max(1, dims.rows);
		const nudgeCols = cols + 1;
		pendingRedraw.add(id);
		void deps.resize(workspaceId, terminalId, nudgeCols, rows).catch(() => undefined);
		deps.logDebug?.(id, 'redraw_nudge', { cols, rows, nudgeCols });
		setTimeoutFn(() => {
			void deps.resize(workspaceId, terminalId, cols, rows).catch(() => undefined);
			pendingRedraw.delete(id);
		}, nudgeDelayMs);
	};

	const clear = (id: string): void => {
		pendingRedraw.delete(id);
	};

	return {
		resizeToFit,
		nudgeRedraw,
		clear,
	};
};
