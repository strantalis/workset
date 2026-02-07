import { Terminal, type ITerminalOptions, type ITheme } from '@xterm/xterm';
import { WebLinksAddon } from '@xterm/addon-web-links';

type TokenResolver = (name: string, fallback: string) => string;
type TerminalInitOptions = ITerminalOptions & {
	allowProposedApi?: boolean;
};

const openHttpsLink = (event: MouseEvent, uri: string, openURL: (url: string) => void): void => {
	if (!uri) return;
	if (!event?.ctrlKey && !event?.metaKey) return;
	try {
		const parsed = new URL(uri);
		if (parsed.protocol !== 'https:') return;
		openURL(parsed.toString());
		event.preventDefault();
	} catch {
		// Ignore invalid URLs.
	}
};

export const createWebLinksAddon = (openURL: (url: string) => void): WebLinksAddon =>
	new WebLinksAddon((event: MouseEvent, uri: string) => {
		openHttpsLink(event, uri, openURL);
	});

export const createTerminalInstance = (input: {
	fontSize: number;
	getToken: TokenResolver;
}): Terminal => {
	const themeBackground = input.getToken('--panel-strong', '#111c29');
	const themeForeground = input.getToken('--text', '#eef3f9');
	const themeCursor = input.getToken('--accent', '#2d8cff');
	const themeSelection = input.getToken('--accent', '#2d8cff');
	const fontMono = input.getToken('--font-mono', '"JetBrains Mono", Menlo, Consolas, monospace');

	const initOptions: TerminalInitOptions = {
		allowProposedApi: true,
		scrollback: 4000,
		fontFamily: fontMono,
		fontSize: input.fontSize,
		lineHeight: 1.4,
		cursorBlink: true,
		theme: {
			background: themeBackground,
			foreground: themeForeground,
			cursor: themeCursor,
			selectionBackground: themeSelection,
		} as ITheme,
	};

	return new Terminal(initOptions);
};
