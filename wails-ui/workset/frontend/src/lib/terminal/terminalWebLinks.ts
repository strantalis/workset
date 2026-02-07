import type { WebLinksAddon } from '@xterm/addon-web-links';
import type { Terminal } from '@xterm/xterm';
import { syncWebLinksForMode } from './terminalRenderer';

type TerminalWebLinksHandle = {
	terminal: Terminal;
	webLinksAddon?: WebLinksAddon;
};

type TerminalWebLinksDependencies = {
	getHandle: (id: string) => TerminalWebLinksHandle | undefined;
	isMouseModeActive: (id: string) => boolean;
	openURL: (url: string) => void;
};

export const createTerminalWebLinksSync = (deps: TerminalWebLinksDependencies) => {
	return (id: string): void => {
		const handle = deps.getHandle(id);
		if (!handle) return;
		handle.webLinksAddon = syncWebLinksForMode({
			terminal: handle.terminal,
			webLinksAddon: handle.webLinksAddon,
			mouseActive: deps.isMouseModeActive(id),
			openURL: deps.openURL,
		});
	};
};
