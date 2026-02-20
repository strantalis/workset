import type { ITerminalAddon, Terminal } from '@xterm/xterm';
import type { FitAddon } from '@xterm/addon-fit';

type Disposable = { dispose: () => void };
type TerminalRenderer = 'unknown' | 'webgl';
type WebglAddonLike = ITerminalAddon & {
	onContextLoss?: (listener: () => void) => Disposable;
	onChangeTextureAtlas?: (listener: (canvas: HTMLCanvasElement) => void) => Disposable;
	clearTextureAtlas?: () => void;
};
type SearchAddonLike = ITerminalAddon;
type WebLinksAddonLike = ITerminalAddon;
type ImageAddonLike = ITerminalAddon;

export type TerminalInstanceHandle = {
	terminal: Terminal;
	fitAddon: FitAddon;
	webglAddon?: WebglAddonLike;
	searchAddon?: SearchAddonLike;
	webLinksAddon?: WebLinksAddonLike;
	imageAddon?: ImageAddonLike;
	webglContextLossDisposable?: Disposable;
	webglAtlasChangeDisposable?: Disposable;
	webglAtlasChangeCount?: number;
	webglInitFailed?: boolean;
	webglInitError?: string;
	renderer: TerminalRenderer;
	dataDisposable: Disposable;
	container: HTMLDivElement;
};

type TerminalInstanceManagerDeps = {
	terminalHandles: Map<string, TerminalInstanceHandle>;
	enableWebgl?: boolean;
	enableImageAddon?: boolean;
	createTerminalInstance: () => Terminal;
	createFitAddon: () => FitAddon;
	createWebglAddon: () => WebglAddonLike;
	createSearchAddon: () => SearchAddonLike;
	createWebLinksAddon: () => WebLinksAddonLike;
	createImageAddon: () => ImageAddonLike;
	createHostContainer?: () => HTMLDivElement;
	onData: (id: string, data: string) => void;
	onRendererResolved?: (id: string, renderer: TerminalRenderer) => void;
	onRendererError?: (id: string, message: string) => void;
	onRendererDebug?: (id: string, event: string, details: Record<string, unknown>) => void;
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
	const createDataDisposable = (
		id: string,
		terminal: Terminal,
	): {
		dataDisposable: Disposable;
	} => {
		const onDataDisposable = terminal.onData((data) => {
			deps.onData(id, data);
		});
		const onBinaryDisposable = terminal.onBinary((data) => {
			deps.onData(id, data);
		});
		return {
			dataDisposable: {
				dispose: () => {
					onDataDisposable.dispose();
					onBinaryDisposable.dispose();
				},
			},
		};
	};

	const createHandle = (
		id: string,
		container: HTMLDivElement | null,
		active: boolean,
	): TerminalInstanceHandle => {
		const terminal = deps.createTerminalInstance();
		const fitAddon = deps.createFitAddon();
		const searchAddon = deps.createSearchAddon();
		const webLinksAddon = deps.createWebLinksAddon();
		const imageAddon = deps.enableImageAddon === false ? undefined : deps.createImageAddon();
		terminal.loadAddon(fitAddon);
		terminal.loadAddon(searchAddon);
		terminal.loadAddon(webLinksAddon);
		if (imageAddon) {
			terminal.loadAddon(imageAddon);
		}
		terminal.attachCustomWheelEventHandler((event) => {
			// Delegate wheel semantics to xterm so alternate-screen TUIs
			// keep receiving native wheel/mouse behavior.
			event.preventDefault();
			event.stopPropagation();
			return true;
		});
		const { dataDisposable } = createDataDisposable(id, terminal);
		const createHost = deps.createHostContainer ?? createDefaultHostContainer;
		const handle: TerminalInstanceHandle = {
			terminal,
			fitAddon,
			searchAddon,
			webLinksAddon,
			imageAddon,
			renderer: 'unknown',
			dataDisposable,
			container: createHost(),
		};
		deps.onRendererDebug?.(id, 'terminal_instance_created', {
			hasContainer: Boolean(container),
			active,
		});
		return handle;
	};

	const bindWebglTelemetry = (
		id: string,
		handle: TerminalInstanceHandle,
		webglAddon: WebglAddonLike,
	) => {
		if (webglAddon.onContextLoss) {
			handle.webglContextLossDisposable = webglAddon.onContextLoss(() => {
				deps.onRendererDebug?.(id, 'webgl_context_lost', {});
				handle.renderer = 'unknown';
				handle.webglInitFailed = true;
				handle.webglInitError = 'WebGL context lost';
				deps.onRendererResolved?.(id, 'unknown');
				deps.onRendererError?.(id, 'WebGL context lost');
			});
		}
		if (webglAddon.onChangeTextureAtlas) {
			handle.webglAtlasChangeDisposable = webglAddon.onChangeTextureAtlas((canvas) => {
				const nextCount = (handle.webglAtlasChangeCount ?? 0) + 1;
				handle.webglAtlasChangeCount = nextCount;
				if (nextCount <= 3 || nextCount % 50 === 0) {
					deps.onRendererDebug?.(id, 'webgl_texture_atlas_changed', {
						count: nextCount,
						width: canvas.width,
						height: canvas.height,
					});
				}
			});
		}
	};

	const initializeWebgl = (id: string, handle: TerminalInstanceHandle): void => {
		try {
			const webglAddon = deps.createWebglAddon();
			handle.terminal.loadAddon(webglAddon);
			handle.webglAddon = webglAddon;
			bindWebglTelemetry(id, handle, webglAddon);
			handle.renderer = 'webgl';
			handle.webglInitFailed = false;
			handle.webglInitError = undefined;
			deps.onRendererDebug?.(id, 'webgl_init_success', {});
		} catch (error) {
			handle.renderer = 'unknown';
			handle.webglInitFailed = true;
			handle.webglInitError =
				error instanceof Error ? error.message : 'WebGL renderer initialization failed';
			deps.onRendererDebug?.(id, 'webgl_init_error', {
				message: handle.webglInitError,
			});
			deps.onRendererResolved?.(id, 'unknown');
			deps.onRendererError?.(id, handle.webglInitError);
		}
	};

	const ensureRendererForAttach = (
		id: string,
		handle: TerminalInstanceHandle,
		active: boolean,
		container: HTMLDivElement | null,
	): void => {
		const webglEnabled = deps.enableWebgl !== false;
		if (!webglEnabled) {
			if (!handle.webglInitFailed && !handle.webglAddon) {
				deps.onRendererDebug?.(id, 'webgl_disabled', {
					active,
					hasContainer: Boolean(container),
				});
			}
			handle.renderer = 'unknown';
			handle.webglInitFailed = true;
			handle.webglInitError = 'WebGL disabled';
			return;
		}
		if (!handle.webglAddon && !handle.webglInitFailed) {
			deps.onRendererDebug?.(id, 'webgl_init_start', {
				active,
				hasContainer: Boolean(container),
			});
			initializeWebgl(id, handle);
		}
	};

	return {
		attach: (id: string, container: HTMLDivElement | null, active: boolean) => {
			let handle = deps.terminalHandles.get(id);
			if (!handle) {
				handle = createHandle(id, container, active);
				deps.terminalHandles.set(id, handle);
			}
			ensureRendererForAttach(id, handle, active, container);
			deps.onRendererDebug?.(id, 'terminal_attach_open_request', {
				active,
				hasContainer: Boolean(container),
			});
			deps.attachOpen({ id, handle, container, active });
			deps.onRendererResolved?.(id, handle.renderer);
			return handle;
		},

		reinitWebgl: (id: string): void => {
			const handle = deps.terminalHandles.get(id);
			// Only reinit if a WebGL addon was previously loaded successfully.
			// If the initial setup failed (no GPU support), skip — we'd just fail again.
			if (!handle || deps.enableWebgl === false || !handle.webglAddon) return;
			handle.webglContextLossDisposable?.dispose();
			handle.webglAtlasChangeDisposable?.dispose();
			handle.webglAddon.dispose();
			handle.webglAddon = undefined;
			handle.webglContextLossDisposable = undefined;
			handle.webglAtlasChangeDisposable = undefined;
			handle.webglAtlasChangeCount = undefined;
			handle.renderer = 'unknown';
			deps.onRendererDebug?.(id, 'webgl_reinit_start', {});
			try {
				const webglAddon = deps.createWebglAddon();
				handle.terminal.loadAddon(webglAddon);
				handle.webglAddon = webglAddon;
				if (webglAddon.onContextLoss) {
					handle.webglContextLossDisposable = webglAddon.onContextLoss(() => {
						deps.onRendererDebug?.(id, 'webgl_context_lost', {});
						handle.renderer = 'unknown';
						handle.webglInitFailed = true;
						handle.webglInitError = 'WebGL context lost';
						deps.onRendererResolved?.(id, 'unknown');
						deps.onRendererError?.(id, 'WebGL context lost');
					});
				}
				if (webglAddon.onChangeTextureAtlas) {
					handle.webglAtlasChangeDisposable = webglAddon.onChangeTextureAtlas((canvas) => {
						const nextCount = (handle?.webglAtlasChangeCount ?? 0) + 1;
						if (handle) handle.webglAtlasChangeCount = nextCount;
						if (nextCount <= 3 || nextCount % 50 === 0) {
							deps.onRendererDebug?.(id, 'webgl_texture_atlas_changed', {
								count: nextCount,
								width: canvas.width,
								height: canvas.height,
							});
						}
					});
				}
				handle.renderer = 'webgl';
				handle.webglInitFailed = false;
				handle.webglInitError = undefined;
				deps.onRendererDebug?.(id, 'webgl_reinit_success', {});
			} catch (error) {
				handle.renderer = 'unknown';
				handle.webglInitFailed = true;
				handle.webglInitError =
					error instanceof Error ? error.message : 'WebGL renderer reinitialization failed';
				deps.onRendererDebug?.(id, 'webgl_reinit_error', { message: handle.webglInitError });
				deps.onRendererResolved?.(id, 'unknown');
				deps.onRendererError?.(id, handle.webglInitError);
			}
		},

		dispose: (id: string): void => {
			const handle = deps.terminalHandles.get(id);
			if (!handle) return;
			handle.dataDisposable?.dispose();
			handle.webglContextLossDisposable?.dispose();
			handle.webglAtlasChangeDisposable?.dispose();
			handle.webglAddon?.dispose();
			handle.searchAddon?.dispose();
			handle.webLinksAddon?.dispose();
			handle.imageAddon?.dispose();
			handle.terminal.dispose();
			deps.terminalHandles.delete(id);
			deps.onRendererDebug?.(id, 'terminal_instance_disposed', {});
		},
	};
};
