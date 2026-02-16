import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
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
};

export const createTerminalInstanceOrchestration = (
	deps: TerminalInstanceOrchestrationDependencies,
) => {
	const terminalAttachOpenLifecycle = createTerminalAttachOpenLifecycle<TerminalInstanceHandle>({
		fitTerminal: (id) => {
			deps.fitTerminal(id, deps.hasStarted(id));
		},
		flushOutput: deps.flushOutput,
		markAttached: deps.markAttached,
		traceAttach: deps.traceAttach,
		nudgeRenderer: (id, handle, opened) => {
			const refresh = (): void => {
				const end = Math.max(0, handle.terminal.rows - 1);
				handle.terminal.refresh(0, end);
			};
			const resetAtlas = (): void => {
				try {
					handle.webglAddon?.clearTextureAtlas?.();
				} catch {
					// Keep rendering even if texture atlas reset is unavailable.
				}
			};
			// Always reset atlas on attach. When panes are detached/re-attached the WebGL
			// atlas can hold stale glyph quads until the next interaction-triggered redraw.
			resetAtlas();
			refresh();
			window.requestAnimationFrame(() => {
				deps.fitTerminal(id, deps.hasStarted(id));
				resetAtlas();
				refresh();
				window.setTimeout(
					() => {
						deps.fitTerminal(id, deps.hasStarted(id));
						resetAtlas();
						refresh();
					},
					opened ? 24 : 40,
				);
			});
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
		createTerminalInstance: () =>
			deps.createTerminalInstance(terminalFontSizeController.getCurrentFontSize()),
		createFitAddon: () => new FitAddon(),
		createWebglAddon: () => new WebglAddon(),
		onData: (id, data) => {
			deps.setInput(id, true);
			deps.captureCpr(id, data);
			deps.sendInput(id, data);
		},
		onRendererResolved: (id, renderer) => {
			deps.setRenderer(id, renderer);
			deps.setRendererMode(id, 'webgl');
			if (renderer === 'webgl') {
				deps.setHealth(id, 'ok', 'Session active.');
			}
			deps.emitState(id);
		},
		onRendererError: (id, message) => {
			deps.setStatusAndMessage(id, 'error', message);
			deps.setHealth(id, 'stale', message);
			deps.emitState(id);
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
