export type ResettableTerminalHandle = {
	terminal: {
		reset: () => void;
		clear: () => void;
		scrollToBottom: () => void;
	};
	fitAddon: {
		fit: () => void;
	};
};

type TerminalResourceLifecycleDependencies<THandle extends ResettableTerminalHandle> = {
	destroyViewportState: (id: string) => void;
	clearTerminalStore: (id: string) => void;
	deleteStats: (id: string) => void;
	deletePendingInput: (id: string) => void;
	deletePendingOutput: (id: string) => void;
	clearResizeState: (id: string) => void;
	deleteLifecycleState: (id: string) => void;
	dropHealthCheck: (id: string) => void;
	releaseAttachState: (id: string) => void;
	disposeTerminalInstance: (id: string) => void;
	getHandle: (id: string) => THandle | undefined;
};

export const createTerminalResourceLifecycle = <THandle extends ResettableTerminalHandle>(
	deps: TerminalResourceLifecycleDependencies<THandle>,
) => {
	const resetSessionState = (id: string): void => {
		deps.dropHealthCheck(id);
		deps.clearResizeState(id);
	};

	const resetTerminalInstance = (id: string): void => {
		const handle = deps.getHandle(id);
		if (!handle) return;
		handle.terminal.reset();
		handle.terminal.clear();
		handle.terminal.scrollToBottom();
		handle.fitAddon.fit();
	};

	const destroyTerminalState = (id: string): void => {
		if (!id) return;
		deps.destroyViewportState(id);
		resetSessionState(id);
		deps.clearTerminalStore(id);
		deps.deleteStats(id);
		deps.deletePendingInput(id);
		deps.deletePendingOutput(id);
		deps.clearResizeState(id);
		deps.deleteLifecycleState(id);
	};

	const disposeTerminalResources = (id: string): void => {
		if (!id) return;
		deps.releaseAttachState(id);
		deps.disposeTerminalInstance(id);
		destroyTerminalState(id);
	};

	return {
		resetSessionState,
		resetTerminalInstance,
		destroyTerminalState,
		disposeTerminalResources,
	};
};
