import type { Terminal } from '@xterm/xterm';
import type { ClipboardAddon } from '@xterm/addon-clipboard';
import type { FitAddon } from '@xterm/addon-fit';
import type { Unicode11Addon } from '@xterm/addon-unicode11';
import type { WebLinksAddon } from '@xterm/addon-web-links';
import type { WebglAddon } from '@xterm/addon-webgl';

type Disposable = { dispose: () => void };

export type TerminalInstanceHandle<TKittyState> = {
	terminal: Terminal;
	fitAddon: FitAddon;
	dataDisposable: Disposable;
	binaryDisposable?: Disposable;
	container: HTMLDivElement;
	kittyState: TKittyState;
	kittyDisposables?: Disposable[];
	oscDisposables?: Disposable[];
	clipboardAddon?: ClipboardAddon;
	unicode11Addon?: Unicode11Addon;
	webLinksAddon?: WebLinksAddon;
	webglAddon?: WebglAddon;
};

type TerminalInstanceManagerDeps<TKittyState> = {
	terminalHandles: Map<string, TerminalInstanceHandle<TKittyState>>;
	createTerminalInstance: () => Terminal;
	createFitAddon: () => FitAddon;
	createUnicode11Addon: () => Unicode11Addon;
	createClipboardAddon: () => ClipboardAddon;
	createKittyState: () => TKittyState;
	createHostContainer?: () => HTMLDivElement;
	syncTerminalWebLinks: (id: string) => void;
	registerOscHandlers: (id: string, terminal: Terminal) => Disposable[];
	ensureMode: (id: string) => void;
	onShiftEnter: (id: string) => void;
	onData: (id: string, data: string) => void;
	onBinary: (id: string, data: string) => void;
	onRender: (id: string) => void;
	attachOpen: (input: {
		id: string;
		handle: TerminalInstanceHandle<TKittyState>;
		container: HTMLDivElement | null;
		active: boolean;
	}) => void;
};

const createDefaultHostContainer = (): HTMLDivElement => {
	const host = document.createElement('div');
	host.className = 'terminal-instance';
	return host;
};

export const createTerminalInstanceManager = <TKittyState>(
	deps: TerminalInstanceManagerDeps<TKittyState>,
) => {
	const bindDataListeners = (id: string, handle: TerminalInstanceHandle<TKittyState>): void => {
		if (handle.dataDisposable) {
			handle.dataDisposable.dispose();
		}
		if (handle.binaryDisposable) {
			handle.binaryDisposable.dispose();
		}
		handle.dataDisposable = handle.terminal.onData((data) => {
			deps.onData(id, data);
		});
		handle.binaryDisposable = handle.terminal.onBinary((data) => {
			if (!data) return;
			deps.onBinary(id, data);
		});
	};

	return {
		attach: (id: string, container: HTMLDivElement | null, active: boolean) => {
			let handle = deps.terminalHandles.get(id);
			if (!handle) {
				const terminal = deps.createTerminalInstance();
				const fitAddon = deps.createFitAddon();
				terminal.loadAddon(fitAddon);
				const unicode11Addon = deps.createUnicode11Addon();
				terminal.loadAddon(unicode11Addon);
				terminal.unicode.activeVersion = '11';
				const clipboardAddon = deps.createClipboardAddon();
				terminal.loadAddon(clipboardAddon);
				terminal.attachCustomKeyEventHandler((event) => {
					if (event.key === 'Enter' && event.shiftKey) {
						deps.onShiftEnter(id);
						return false;
					}
					return true;
				});
				const dataDisposable = terminal.onData((data) => {
					deps.onData(id, data);
				});
				const binaryDisposable = terminal.onBinary((data) => {
					if (!data) return;
					deps.onBinary(id, data);
				});
				terminal.onRender(() => {
					deps.onRender(id);
				});
				const createHost = deps.createHostContainer ?? createDefaultHostContainer;
				handle = {
					terminal,
					fitAddon,
					dataDisposable,
					binaryDisposable,
					container: createHost(),
					kittyState: deps.createKittyState(),
					clipboardAddon,
					unicode11Addon,
				};
				deps.terminalHandles.set(id, handle);
				deps.syncTerminalWebLinks(id);
				handle.oscDisposables = deps.registerOscHandlers(id, terminal);
				deps.ensureMode(id);
			}

			bindDataListeners(id, handle);
			deps.attachOpen({ id, handle, container, active });
			return handle;
		},

		dispose: (id: string): void => {
			const handle = deps.terminalHandles.get(id);
			if (!handle) return;
			handle.dataDisposable?.dispose();
			handle.binaryDisposable?.dispose();
			if (handle.oscDisposables) {
				for (const disposable of handle.oscDisposables) {
					disposable.dispose();
				}
			}
			if (handle.kittyDisposables) {
				for (const disposable of handle.kittyDisposables) {
					disposable.dispose();
				}
			}
			handle.clipboardAddon?.dispose();
			handle.webLinksAddon?.dispose();
			handle.unicode11Addon?.dispose();
			handle.webglAddon?.dispose();
			handle.terminal.dispose();
			deps.terminalHandles.delete(id);
		},
	};
};
