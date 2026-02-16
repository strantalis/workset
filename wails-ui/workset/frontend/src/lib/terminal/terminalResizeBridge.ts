type FitDimensions = { cols: number; rows: number };

type FitAddonLike = {
	proposeDimensions: () => FitDimensions | undefined;
};

export type TerminalResizeHandle = {
	fitAddon: FitAddonLike;
};

type TerminalResizeBridgeDeps = {
	getWorkspaceId: (id: string) => string;
	getTerminalId: (id: string) => string;
	resize: (workspaceId: string, terminalId: string, cols: number, rows: number) => Promise<unknown>;
};

export const createTerminalResizeBridge = (deps: TerminalResizeBridgeDeps) => {
	const MIN_STABLE_COLS = 4;
	const MIN_STABLE_ROWS = 2;
	const lastSentDimensions = new Map<string, FitDimensions>();
	const sameDimensions = (left: FitDimensions | undefined, right: FitDimensions): boolean =>
		Boolean(left && left.cols === right.cols && left.rows === right.rows);

	const resizeToFit = (id: string, handle: TerminalResizeHandle | undefined): void => {
		if (!handle) return;
		const dims = handle.fitAddon.proposeDimensions();
		if (!dims) return;
		if (dims.cols < MIN_STABLE_COLS || dims.rows < MIN_STABLE_ROWS) return;
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		const previous = lastSentDimensions.get(id);
		if (sameDimensions(previous, dims)) return;
		lastSentDimensions.set(id, { cols: dims.cols, rows: dims.rows });
		void deps.resize(workspaceId, terminalId, dims.cols, dims.rows).catch(() => {
			// Keep retries possible when resize races terminal startup and the backend
			// reports a transient miss.
			const current = lastSentDimensions.get(id);
			if (sameDimensions(current, dims)) {
				lastSentDimensions.delete(id);
			}
		});
	};

	const clear = (id: string): void => {
		lastSentDimensions.delete(id);
	};

	return {
		resizeToFit,
		clear,
	};
};
