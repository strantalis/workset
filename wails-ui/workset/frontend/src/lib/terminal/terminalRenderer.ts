import { Terminal, type ITerminalOptions, type ITheme } from '@strantalis/workset-ghostty-web';
import { ensureGhosttyInitialized } from './ghosttyRuntime';

type TokenResolver = (name: string, fallback: string) => string;

export const createTerminalInstance = async (input: {
	fontSize: number;
	cursorBlink: boolean;
	getToken: TokenResolver;
	openLink?: (url: string, event: MouseEvent) => void | Promise<void>;
}): Promise<Terminal> => {
	await ensureGhosttyInitialized();

	const themeBackground = input.getToken('--panel-strong', '#111c29');
	const themeForeground = input.getToken('--text', '#eef3f9');
	const themeCursor = input.getToken('--accent', '#2d8cff');
	const themeSelectionBackground = input.getToken(
		'--terminal-selection-background',
		'rgba(39, 94, 168, 0.45)',
	);
	const themeSelectionForeground = input.getToken(
		'--terminal-selection-foreground',
		themeForeground,
	);
	// Keep terminal glyph rendering on conservative system monospace fonts to
	// reduce block/box glyph seam artifacts in fallback render paths.
	const fontMono = input.getToken(
		'--font-mono-terminal',
		'"SF Mono", Menlo, Monaco, Consolas, monospace',
	);

	const initOptions: ITerminalOptions = {
		scrollback: 4000,
		smoothScrollDuration: 0,
		fontFamily: fontMono,
		fontSize: input.fontSize,
		cursorStyle: 'bar',
		cursorBlink: input.cursorBlink,
		openLink: input.openLink,
		theme: {
			background: themeBackground,
			foreground: themeForeground,
			cursor: themeCursor,
			selectionBackground: themeSelectionBackground,
			selectionForeground: themeSelectionForeground,
			// ANSI 16-color palette (indices 0-15)
			black: '#1c2b3a',
			red: '#ef4444',
			green: '#86c442',
			yellow: '#f59e0b',
			blue: '#2d8cff',
			magenta: '#8b8aed',
			cyan: '#56b6c2',
			white: '#a3b5c9',
			brightBlack: '#3a4f63',
			brightRed: '#f87171',
			brightGreen: '#a3d977',
			brightYellow: '#fbbf24',
			brightBlue: '#60a5fa',
			brightMagenta: '#a78bfa',
			brightCyan: '#67d4e0',
			brightWhite: '#f2f6fb',
		} as ITheme,
	};

	return new Terminal(initOptions);
};
