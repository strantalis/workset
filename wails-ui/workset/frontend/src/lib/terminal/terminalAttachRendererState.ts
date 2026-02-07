import type { WebglAddon } from '@xterm/addon-webgl';
import type { Terminal } from '@xterm/xterm';
import { loadRendererAddon as defaultLoadRendererAddon } from './terminalRenderer';

type TimeoutHandle = ReturnType<typeof setTimeout>;
type RendererMode = 'webgl';
type RendererType = 'unknown' | 'webgl';

export type AttachRendererHandle = {
	terminal: Terminal;
	webglAddon?: WebglAddon;
};

export const createTerminalAttachState = (input: {
	disposeAfterMs: number;
	onDispose: (id: string) => void;
	setTimeoutFn?: (callback: () => void, timeoutMs: number) => TimeoutHandle;
	clearTimeoutFn?: (handle: TimeoutHandle) => void;
}) => {
	const attachedTerminals = new Set<string>();
	const disposeTimers = new Map<string, TimeoutHandle>();
	const setTimeoutFn =
		input.setTimeoutFn ?? ((callback, timeoutMs) => setTimeout(callback, timeoutMs));
	const clearTimeoutFn = input.clearTimeoutFn ?? ((handle) => clearTimeout(handle));

	const cancelDispose = (id: string): void => {
		const timer = disposeTimers.get(id);
		if (!timer) return;
		clearTimeoutFn(timer);
		disposeTimers.delete(id);
	};

	const scheduleDispose = (id: string): void => {
		if (!id) return;
		if (attachedTerminals.has(id)) return;
		if (disposeTimers.has(id)) return;
		const timer = setTimeoutFn(() => {
			disposeTimers.delete(id);
			if (attachedTerminals.has(id)) return;
			input.onDispose(id);
		}, input.disposeAfterMs);
		disposeTimers.set(id, timer);
	};

	return {
		markAttached: (id: string): void => {
			if (!id) return;
			attachedTerminals.add(id);
			cancelDispose(id);
		},
		markDetached: (id: string): void => {
			if (!id) return;
			attachedTerminals.delete(id);
			scheduleDispose(id);
		},
		release: (id: string): void => {
			if (!id) return;
			cancelDispose(id);
			attachedTerminals.delete(id);
		},
		forEachAttached: (run: (id: string) => void): void => {
			for (const id of attachedTerminals) {
				run(id);
			}
		},
	};
};

export const createTerminalRendererAddonState = (input: {
	setRendererMode: (id: string, mode: RendererMode) => void;
	setRenderer: (id: string, renderer: RendererType) => void;
	onRendererUnavailable: (id: string, error: unknown) => void;
	onComplete: (id: string) => void;
	loadRendererAddon?: typeof defaultLoadRendererAddon;
}) => {
	const loadAddon = input.loadRendererAddon ?? defaultLoadRendererAddon;
	return {
		load: async (id: string, handle: AttachRendererHandle): Promise<void> => {
			handle.webglAddon = await loadAddon({
				terminal: handle.terminal,
				webglAddon: handle.webglAddon,
				setRendererMode: (mode) => {
					input.setRendererMode(id, mode);
				},
				setRenderer: (renderer) => {
					input.setRenderer(id, renderer);
				},
				onRendererUnavailable: (error) => {
					input.onRendererUnavailable(id, error);
				},
				onComplete: () => {
					input.onComplete(id);
				},
			});
		},
	};
};
