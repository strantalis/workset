import { Terminal, type ITerminalOptions, type ITheme } from '@xterm/xterm';

type TokenResolver = (name: string, fallback: string) => string;

export const createTerminalInstance = (input: {
	fontSize: number;
	getToken: TokenResolver;
}): Terminal => {
	const themeBackground = input.getToken('--panel-strong', '#111c29');
	const themeForeground = input.getToken('--text', '#eef3f9');
	const themeCursor = input.getToken('--accent', '#2d8cff');
	const themeSelection = input.getToken('--accent', '#2d8cff');
	const fontMono = input.getToken('--font-mono', '"JetBrains Mono", Menlo, Consolas, monospace');

	const initOptions: ITerminalOptions = {
		rescaleOverlappingGlyphs: true,
		scrollback: 4000,
		fontFamily: fontMono,
		fontWeight: '400',
		fontWeightBold: '600',
		fontSize: input.fontSize,
		lineHeight: 1,
		letterSpacing: 0,
		cursorBlink: true,
		cursorInactiveStyle: 'none',
		theme: {
			background: themeBackground,
			foreground: themeForeground,
			cursor: themeCursor,
			selectionBackground: themeSelection,
		} as ITheme,
	};

	return new Terminal(initOptions);
};
