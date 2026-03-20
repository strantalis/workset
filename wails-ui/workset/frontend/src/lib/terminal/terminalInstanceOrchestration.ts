import {
	FitAddon,
	OSC8LinkProvider,
	Terminal,
	UrlRegexProvider,
} from '@strantalis/workset-ghostty-web';
import type { TerminalLinkProviderLike, TerminalLinkRange } from './terminalEmulatorContracts';
import { createTerminalAttachOpenLifecycle } from './terminalAttachOpenLifecycle';
import { createTerminalFontSizeController } from './terminalFontSizeController';
import {
	createTerminalInstanceManager,
	type TerminalInstanceHandle,
} from './terminalInstanceManager';

type TerminalInstanceOrchestrationDependencies = {
	terminalHandles: Map<string, TerminalInstanceHandle>;
	createTerminalInstance: (fontSize: number, cursorBlink: boolean) => Promise<Terminal>;
	openURL: (url: string) => Promise<void>;
	setStatusAndMessage: (id: string, status: string, message: string) => void;
	setHealth: (id: string, state: 'unknown' | 'checking' | 'ok' | 'stale', message?: string) => void;
	emitState: (id: string) => void;
	setInput: (id: string, value: boolean) => void;
	sendInput: (id: string, data: string) => void;
	sendProtocolResponse: (id: string, data: string) => void;
	captureCpr: (id: string, data: string) => void;
	fitTerminal: (id: string, started: boolean) => void;
	hasStarted: (id: string) => boolean;
	flushOutput: (id: string, writeAll: boolean) => void;
	markAttached: (id: string) => void;
	traceAttach?: (id: string, event: string, details: Record<string, unknown>) => void;
	traceRenderer?: (id: string, event: string, details: Record<string, unknown>) => void;
};

type LinkProviderSource = {
	provideLinks: (
		y: number,
		callback: (links: { text: string; range: TerminalLinkRange }[] | undefined) => void,
	) => void;
	dispose?: () => void;
};

const wrapLinkProvider = (
	provider: LinkProviderSource,
	openURL: (url: string) => Promise<void>,
): TerminalLinkProviderLike => ({
	provideLinks: (row, callback) => {
		provider.provideLinks(row, (links) => {
			if (!links) {
				callback(undefined);
				return;
			}
			callback(
				links.map((link) => ({
					...link,
					activate: (event: MouseEvent) => {
						event.preventDefault();
						event.stopPropagation();
						void openURL(link.text).catch(() => undefined);
					},
				})),
			);
		});
	},
	dispose: () => {
		provider.dispose?.();
	},
});

const createLinkProviders = (
	terminal: unknown,
	openURL: (url: string) => Promise<void>,
): TerminalLinkProviderLike[] => {
	const osc8Provider = new OSC8LinkProvider(terminal as never);
	const urlProvider = new UrlRegexProvider(terminal as never);
	return [osc8Provider, urlProvider].map((provider) => wrapLinkProvider(provider, openURL));
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

	const applyCursorBlinkToAllTerminals = (cursorBlink: boolean): void => {
		for (const handle of deps.terminalHandles.values()) {
			const active = handle.container.getAttribute('data-active') === 'true';
			handle.terminal.options.cursorBlink = Boolean(active && cursorBlink);
		}
	};

	const terminalFontSizeController = createTerminalFontSizeController({
		onFontSizeChange: applyFontSizeToAllTerminals,
		onCursorBlinkChange: applyCursorBlinkToAllTerminals,
	});

	const terminalInstanceManager = createTerminalInstanceManager({
		terminalHandles: deps.terminalHandles,
		createTerminalInstance: () =>
			deps.createTerminalInstance(
				terminalFontSizeController.getCurrentFontSize(),
				terminalFontSizeController.getCursorBlink(),
			),
		createFitAddon: () => new FitAddon({ scrollbarWidth: 0 }),
		createLinkProviders: (terminal) => createLinkProviders(terminal, deps.openURL),
		onData: (id, data) => {
			deps.setInput(id, true);
			deps.sendInput(id, data);
		},
		onResponse: (id, data) => {
			deps.captureCpr(id, data);
			deps.sendProtocolResponse(id, data);
		},
		onRendererError: (id, message) => {
			deps.traceRenderer?.(id, 'renderer_error', { message });
			deps.setStatusAndMessage(id, 'error', message);
			deps.setHealth(id, 'stale', message);
			deps.emitState(id);
		},
		onRendererDebug: deps.traceRenderer,
		attachOpen: ({ id, handle, container, active }) => {
			handle.terminal.options.cursorBlink = Boolean(
				active && terminalFontSizeController.getCursorBlink(),
			);
			terminalAttachOpenLifecycle.attach({ id, handle, container, active });
		},
	});

	const attachTerminal = (
		id: string,
		container: HTMLDivElement | null,
		active: boolean,
	): Promise<TerminalInstanceHandle> => {
		return terminalInstanceManager.attach(id, container, active);
	};

	return {
		terminalFontSizeController,
		terminalInstanceManager,
		attachTerminal,
	};
};
