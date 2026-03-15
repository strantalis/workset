import { type ITerminalOptions, type ITheme, Terminal } from 'ghostty-web';
import { ensureGhosttyInitialized } from './ghosttyRuntime';

type TokenResolver = (name: string, fallback: string) => string;

export const createTerminalInstance = async (input: {
	fontSize: number;
	getToken: TokenResolver;
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
		cursorBlink: true,
		theme: {
			background: themeBackground,
			foreground: themeForeground,
			cursor: themeCursor,
			selectionBackground: themeSelectionBackground,
			selectionForeground: themeSelectionForeground,
		} as ITheme,
	};

	return new Terminal(initOptions);
};
