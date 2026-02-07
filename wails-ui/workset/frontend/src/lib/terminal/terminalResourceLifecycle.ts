import type { TerminalModesState } from './terminalLifecycle';

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
	clearBootstrapFetchTimer: (id: string) => void;
	clearReattachTimer: (id: string) => void;
	destroyViewportState: (id: string) => void;
	clearTerminalStore: (id: string) => void;
	clearOutputBuffer: (id: string) => void;
	destroyReplayState: (id: string) => void;
	resetReplaySession: (id: string) => void;
	deleteBootstrapHandled: (id: string) => void;
	deleteStats: (id: string) => void;
	deletePendingInput: (id: string) => void;
	clearResizeState: (id: string) => void;
	releaseRenderHealth: (id: string) => void;
	clearRenderHealthSession: (id: string) => void;
	deleteLifecycleState: (id: string) => void;
	dropHealthCheck: (id: string) => void;
	clearMouseSuppression: (id: string) => void;
	clearMouseTail: (id: string) => void;
	releaseAttachState: (id: string) => void;
	disposeTerminalInstance: (id: string) => void;
	getHandle: (id: string) => THandle | undefined;
	resizeOverlay: (handle: THandle) => void;
	setMode: (id: string, mode: TerminalModesState) => void;
	loadRendererAddon: (id: string, handle: THandle) => Promise<void> | void;
	noteMouseSuppress: (id: string, durationMs: number) => void;
};

const defaultMode: TerminalModesState = {
	altScreen: false,
	mouse: false,
	mouseSGR: false,
	mouseEncoding: 'x10',
};

export const createTerminalResourceLifecycle = <THandle extends ResettableTerminalHandle>(
	deps: TerminalResourceLifecycleDependencies<THandle>,
) => {
	const resetSessionState = (id: string): void => {
		deps.resetReplaySession(id);
		deps.clearOutputBuffer(id);
		deps.deleteBootstrapHandled(id);
		deps.clearBootstrapFetchTimer(id);
		deps.dropHealthCheck(id);
		deps.clearRenderHealthSession(id);
		deps.clearResizeState(id);
		deps.clearMouseTail(id);
	};

	const resetTerminalInstance = (id: string): void => {
		const handle = deps.getHandle(id);
		if (!handle) return;
		handle.terminal.reset();
		handle.terminal.clear();
		handle.terminal.scrollToBottom();
		handle.fitAddon.fit();
		deps.resizeOverlay(handle);
		deps.setMode(id, defaultMode);
		deps.clearMouseTail(id);
		deps.noteMouseSuppress(id, 2500);
		void deps.loadRendererAddon(id, handle);
	};

	const destroyTerminalState = (id: string): void => {
		if (!id) return;
		deps.clearBootstrapFetchTimer(id);
		deps.clearReattachTimer(id);
		deps.destroyViewportState(id);
		resetSessionState(id);
		deps.clearTerminalStore(id);
		deps.clearOutputBuffer(id);
		deps.destroyReplayState(id);
		deps.deleteBootstrapHandled(id);
		deps.deleteStats(id);
		deps.deletePendingInput(id);
		deps.clearResizeState(id);
		deps.releaseRenderHealth(id);
		deps.deleteLifecycleState(id);
		deps.clearMouseSuppression(id);
		deps.clearMouseTail(id);
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
