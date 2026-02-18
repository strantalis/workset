import type { ITerminalAddon, Terminal } from '@xterm/xterm';
import type { FitAddon } from '@xterm/addon-fit';

type Disposable = { dispose: () => void };
type TerminalRenderer = 'unknown' | 'webgl';
type WebglAddonLike = ITerminalAddon & {
	onContextLoss?: (listener: () => void) => Disposable;
	clearTextureAtlas?: () => void;
};

export type TerminalInstanceHandle = {
	terminal: Terminal;
	fitAddon: FitAddon;
	webglAddon?: WebglAddonLike;
	webglContextLossDisposable?: Disposable;
	webglInitFailed?: boolean;
	webglInitError?: string;
	renderer: TerminalRenderer;
	dataDisposable: Disposable;
	container: HTMLDivElement;
};

type TerminalInstanceManagerDeps = {
	terminalHandles: Map<string, TerminalInstanceHandle>;
	createTerminalInstance: () => Terminal;
	createFitAddon: () => FitAddon;
	createWebglAddon: () => WebglAddonLike;
	createHostContainer?: () => HTMLDivElement;
	onData: (id: string, data: string) => void;
	onRendererResolved?: (id: string, renderer: TerminalRenderer) => void;
	onRendererError?: (id: string, message: string) => void;
	attachOpen: (input: {
		id: string;
		handle: TerminalInstanceHandle;
		container: HTMLDivElement | null;
		active: boolean;
	}) => void;
};

const createDefaultHostContainer = (): HTMLDivElement => {
	const host = document.createElement('div');
	host.className = 'terminal-instance';
	return host;
};

export const createTerminalInstanceManager = (deps: TerminalInstanceManagerDeps) => {
	return {
		attach: (id: string, container: HTMLDivElement | null, active: boolean) => {
			let handle = deps.terminalHandles.get(id);
			if (!handle) {
				const terminal = deps.createTerminalInstance();
				const fitAddon = deps.createFitAddon();
				terminal.loadAddon(fitAddon);
				terminal.attachCustomWheelEventHandler((event) => {
					// Delegate wheel semantics to xterm so alternate-screen TUIs
					// keep receiving native wheel/mouse behavior.
					event.preventDefault();
					event.stopPropagation();
					return true;
				});
				const onDataDisposable = terminal.onData((data) => {
					deps.onData(id, data);
				});
				const onBinaryDisposable = terminal.onBinary((data) => {
					deps.onData(id, data);
				});
				const dataDisposable: Disposable = {
					dispose: () => {
						onDataDisposable.dispose();
						onBinaryDisposable.dispose();
					},
				};
				const createHost = deps.createHostContainer ?? createDefaultHostContainer;
				handle = {
					terminal,
					fitAddon,
					renderer: 'unknown',
					dataDisposable,
					container: createHost(),
				};
				deps.terminalHandles.set(id, handle);
			}
			if (!handle.webglAddon && !handle.webglInitFailed) {
				try {
					const webglAddon = deps.createWebglAddon();
					handle.terminal.loadAddon(webglAddon);
					handle.webglAddon = webglAddon;
					if (webglAddon.onContextLoss) {
						handle.webglContextLossDisposable = webglAddon.onContextLoss(() => {
							handle.renderer = 'unknown';
							handle.webglInitFailed = true;
							handle.webglInitError = 'WebGL context lost';
							deps.onRendererResolved?.(id, 'unknown');
							deps.onRendererError?.(id, 'WebGL context lost');
						});
					}
					handle.renderer = 'webgl';
					handle.webglInitFailed = false;
					handle.webglInitError = undefined;
				} catch (error) {
					handle.renderer = 'unknown';
					handle.webglInitFailed = true;
					handle.webglInitError =
						error instanceof Error ? error.message : 'WebGL renderer initialization failed';
					deps.onRendererResolved?.(id, 'unknown');
					deps.onRendererError?.(id, handle.webglInitError);
				}
			}
			deps.attachOpen({ id, handle, container, active });
			deps.onRendererResolved?.(id, handle.renderer);
			return handle;
		},

		dispose: (id: string): void => {
			const handle = deps.terminalHandles.get(id);
			if (!handle) return;
			handle.dataDisposable?.dispose();
			handle.webglContextLossDisposable?.dispose();
			handle.webglAddon?.dispose();
			handle.terminal.dispose();
			deps.terminalHandles.delete(id);
		},
	};
};
