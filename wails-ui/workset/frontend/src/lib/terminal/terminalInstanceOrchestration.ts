import { FitAddon } from '@xterm/addon-fit';
import { ImageAddon } from '@xterm/addon-image';
import { SearchAddon } from '@xterm/addon-search';
import { WebglAddon } from '@xterm/addon-webgl';
import { WebLinksAddon } from '@xterm/addon-web-links';
import type { Terminal } from '@xterm/xterm';
import { createTerminalAttachOpenLifecycle } from './terminalAttachOpenLifecycle';
import { createTerminalFontSizeController } from './terminalFontSizeController';
import {
	createTerminalInstanceManager,
	type TerminalInstanceHandle,
} from './terminalInstanceManager';

type TerminalInstanceOrchestrationDependencies = {
	terminalHandles: Map<string, TerminalInstanceHandle>;
	createTerminalInstance: (fontSize: number) => Terminal;
	setRenderer: (id: string, renderer: 'unknown' | 'webgl') => void;
	setRendererMode: (id: string, mode: 'webgl') => void;
	setStatusAndMessage: (id: string, status: string, message: string) => void;
	setHealth: (id: string, state: 'unknown' | 'checking' | 'ok' | 'stale', message?: string) => void;
	emitState: (id: string) => void;
	setInput: (id: string, value: boolean) => void;
	sendInput: (id: string, data: string) => void;
	captureCpr: (id: string, data: string) => void;
	fitTerminal: (id: string, started: boolean) => void;
	hasStarted: (id: string) => boolean;
	flushOutput: (id: string, writeAll: boolean) => void;
	markAttached: (id: string) => void;
	traceAttach?: (id: string, event: string, details: Record<string, unknown>) => void;
	traceRenderer?: (id: string, event: string, details: Record<string, unknown>) => void;
};

export const createTerminalInstanceOrchestration = (
	deps: TerminalInstanceOrchestrationDependencies,
) => {
	// WebGL enabled. Image addon remains off pending separate validation.
	const ENABLE_WEBGL_RENDERER = true;
	const ENABLE_IMAGE_ADDON = false;

	// nudgeRenderer captures this callback and invokes it after manager creation.
	const reinitWebgl = (id: string): void => {
		terminalInstanceManager.reinitWebgl(id);
	};

	const terminalAttachOpenLifecycle = createTerminalAttachOpenLifecycle<TerminalInstanceHandle>({
		fitTerminal: (id) => {
			deps.fitTerminal(id, deps.hasStarted(id));
		},
		flushOutput: deps.flushOutput,
		markAttached: deps.markAttached,
		traceAttach: deps.traceAttach,
		nudgeRenderer: (id, handle, rebuildAtlas) => {
			const refresh = (): void => {
				if (handle.terminal.rows < 1) {
					return;
				}
				const end = Math.max(0, handle.terminal.rows - 1);
				handle.terminal.refresh(0, end);
			};
			if (rebuildAtlas) {
				// Dispose and recreate the WebGL addon so the renderer re-initializes
				// against the current container geometry. clearTextureAtlas() alone is
				// not sufficient: the WebGL canvas pixel dimensions can go stale when
				// the terminal's DOM node moves between containers, causing glyphs to
				// render at wrong pixel offsets until the next scroll-triggered repaint.
				reinitWebgl(id);
			}
			deps.fitTerminal(id, deps.hasStarted(id));
			refresh();
		},
	});

	const applyFontSizeToAllTerminals = (fontSize: number): void => {
		for (const [id, handle] of deps.terminalHandles.entries()) {
			handle.terminal.options.fontSize = fontSize;
			try {
				deps.fitTerminal(id, deps.hasStarted(id));
			} catch {
				// Ignore fit errors for terminals not attached to DOM.
			}
		}
	};

	const terminalFontSizeController = createTerminalFontSizeController({
		onFontSizeChange: applyFontSizeToAllTerminals,
	});

	const terminalInstanceManager = createTerminalInstanceManager({
		terminalHandles: deps.terminalHandles,
		enableWebgl: ENABLE_WEBGL_RENDERER,
		enableImageAddon: ENABLE_IMAGE_ADDON,
		createTerminalInstance: () =>
			deps.createTerminalInstance(terminalFontSizeController.getCurrentFontSize()),
		createFitAddon: () => new FitAddon(),
		createSearchAddon: () => new SearchAddon(),
		createWebLinksAddon: () => new WebLinksAddon(),
		createImageAddon: () => new ImageAddon(),
		createWebglAddon: () => new WebglAddon(),
		onData: (id, data) => {
			deps.setInput(id, true);
			deps.captureCpr(id, data);
			deps.sendInput(id, data);
		},
		onRendererResolved: (id, renderer) => {
			deps.setRenderer(id, renderer);
			deps.setRendererMode(id, 'webgl');
			deps.traceRenderer?.(id, 'renderer_resolved', { renderer });
			if (renderer === 'webgl') {
				deps.setHealth(id, 'ok', 'Session active.');
			}
			deps.emitState(id);
		},
		onRendererError: (id, message) => {
			deps.traceRenderer?.(id, 'renderer_error', { message });
			deps.setStatusAndMessage(id, 'error', message);
			deps.setHealth(id, 'stale', message);
			deps.emitState(id);
		},
		onRendererDebug: (id, event, details) => {
			deps.traceRenderer?.(id, event, details);
		},
		attachOpen: ({ id, handle, container, active }) => {
			terminalAttachOpenLifecycle.attach({ id, handle, container, active });
		},
	});

	const attachTerminal = (
		id: string,
		container: HTMLDivElement | null,
		active: boolean,
	): TerminalInstanceHandle => {
		return terminalInstanceManager.attach(id, container, active);
	};

	return {
		terminalFontSizeController,
		terminalInstanceManager,
		attachTerminal,
	};
};
