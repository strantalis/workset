import { ClipboardAddon } from '@xterm/addon-clipboard';
import { FitAddon } from '@xterm/addon-fit';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import type { Terminal } from '@xterm/xterm';
import { createTerminalAttachOpenLifecycle } from './terminalAttachOpenLifecycle';
import { createTerminalFontSizeController } from './terminalFontSizeController';
import {
	createTerminalInstanceManager,
	type TerminalInstanceHandle,
} from './terminalInstanceManager';
import {
	createTerminalClipboardBase64,
	createTerminalClipboardProvider,
} from './terminalClipboard';
import { registerTerminalOscHandlers } from './terminalOscHandlers';

type TerminalInstanceOrchestrationDependencies<
	TKittyState,
	THandle extends TerminalInstanceHandle<TKittyState>,
> = {
	terminalHandles: Map<string, THandle>;
	createTerminalInstance: (fontSize: number) => Terminal;
	createKittyState: () => TKittyState;
	syncTerminalWebLinks: (id: string) => void;
	ensureMode: (id: string) => void;
	setInput: (id: string, value: boolean) => void;
	beginTerminal: (id: string) => Promise<void>;
	sendInput: (id: string, data: string) => void;
	captureCpr: (id: string, data: string) => void;
	noteRender: (id: string) => void;
	getToken: (name: string, fallback: string) => string;
	getHandle: (id: string) => THandle | undefined;
	fitTerminal: (id: string, started: boolean) => void;
	hasStarted: (id: string) => boolean;
	ensureOverlay: (id: string) => void;
	loadRendererAddon: (id: string, handle: THandle) => Promise<void> | void;
	fitWithPreservedViewport: (handle: THandle) => void;
	resizeToFit: (id: string, handle: THandle) => void;
	scheduleFitStabilization: (id: string, reason: string) => void;
	flushOutput: (id: string, writeAll: boolean) => void;
	markAttached: (id: string) => void;
};

export const createTerminalInstanceOrchestration = <
	TKittyState,
	THandle extends TerminalInstanceHandle<TKittyState>,
>(
	deps: TerminalInstanceOrchestrationDependencies<TKittyState, THandle>,
) => {
	const terminalAttachOpenLifecycle = createTerminalAttachOpenLifecycle({
		getHandle: deps.getHandle,
		ensureOverlay: (_handle, id) => {
			deps.ensureOverlay(id);
		},
		loadRendererAddon: deps.loadRendererAddon,
		fitWithPreservedViewport: (handle) => {
			deps.fitWithPreservedViewport(handle);
		},
		resizeToFit: (id, handle) => {
			deps.resizeToFit(id, handle);
		},
		scheduleFitStabilization: deps.scheduleFitStabilization,
		flushOutput: deps.flushOutput,
		markAttached: deps.markAttached,
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

	const terminalInstanceManager = createTerminalInstanceManager<TKittyState>({
		terminalHandles: deps.terminalHandles,
		createTerminalInstance: () =>
			deps.createTerminalInstance(terminalFontSizeController.getCurrentFontSize()),
		createFitAddon: () => new FitAddon(),
		createUnicode11Addon: () => new Unicode11Addon(),
		createClipboardAddon: () =>
			new ClipboardAddon(createTerminalClipboardBase64(), createTerminalClipboardProvider()),
		createKittyState: deps.createKittyState,
		syncTerminalWebLinks: deps.syncTerminalWebLinks,
		registerOscHandlers: (id, terminal) =>
			registerTerminalOscHandlers(id, terminal, {
				sendInput: deps.sendInput,
				getToken: deps.getToken,
			}),
		ensureMode: deps.ensureMode,
		onShiftEnter: (id) => {
			deps.setInput(id, true);
			void deps.beginTerminal(id);
			deps.sendInput(id, '\x0a');
		},
		onData: (id, data) => {
			deps.setInput(id, true);
			void deps.beginTerminal(id);
			deps.captureCpr(id, data);
			deps.sendInput(id, data);
		},
		onBinary: (id, data) => {
			deps.setInput(id, true);
			void deps.beginTerminal(id);
			deps.sendInput(id, data);
		},
		onRender: deps.noteRender,
		attachOpen: ({ id, handle, container, active }) => {
			terminalAttachOpenLifecycle.attach({ id, handle: handle as THandle, container, active });
		},
	});

	const attachTerminal = (
		id: string,
		container: HTMLDivElement | null,
		active: boolean,
	): THandle => {
		return terminalInstanceManager.attach(id, container, active) as THandle;
	};

	return {
		terminalFontSizeController,
		terminalInstanceManager,
		attachTerminal,
	};
};
