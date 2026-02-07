import type { ITheme, Terminal } from '@xterm/xterm';

type TerminalTokenResolver = (name: string, fallback: string) => string;

type TerminalOscHandlersDeps = {
	sendInput: (id: string, data: string) => void;
	getToken: TerminalTokenResolver;
};

const normalizeHex = (value: string | undefined | null): string | null => {
	if (typeof value !== 'string') return null;
	let hex = value.trim();
	if (!hex) return null;
	if (hex.startsWith('#')) {
		hex = hex.slice(1);
	}
	if (hex.length === 3) {
		hex = hex
			.split('')
			.map((item) => item + item)
			.join('');
	}
	if (hex.length !== 6) return null;
	return hex;
};

const toOscRgb = (value: string | undefined | null): string | null => {
	const hex = normalizeHex(value);
	if (!hex) return null;
	const r = hex.slice(0, 2);
	const g = hex.slice(2, 4);
	const b = hex.slice(4, 6);
	return `rgb:${r}/${g}/${b}`;
};

const defaultAnsiPalette = [
	'#000000',
	'#cd3131',
	'#0dbc79',
	'#e5e510',
	'#2472c8',
	'#bc3fbc',
	'#11a8cd',
	'#e5e5e5',
	'#666666',
	'#f14c4c',
	'#23d18b',
	'#f5f543',
	'#3b8eea',
	'#d670d6',
	'#29b8db',
	'#ffffff',
];

const themePalette = (theme: ITheme): string[] => {
	return [
		theme.black ?? defaultAnsiPalette[0],
		theme.red ?? defaultAnsiPalette[1],
		theme.green ?? defaultAnsiPalette[2],
		theme.yellow ?? defaultAnsiPalette[3],
		theme.blue ?? defaultAnsiPalette[4],
		theme.magenta ?? defaultAnsiPalette[5],
		theme.cyan ?? defaultAnsiPalette[6],
		theme.white ?? defaultAnsiPalette[7],
		theme.brightBlack ?? defaultAnsiPalette[8],
		theme.brightRed ?? defaultAnsiPalette[9],
		theme.brightGreen ?? defaultAnsiPalette[10],
		theme.brightYellow ?? defaultAnsiPalette[11],
		theme.brightBlue ?? defaultAnsiPalette[12],
		theme.brightMagenta ?? defaultAnsiPalette[13],
		theme.brightCyan ?? defaultAnsiPalette[14],
		theme.brightWhite ?? defaultAnsiPalette[15],
	];
};

const resolveThemeColor = (value: string | undefined, fallback: string): string | null => {
	return toOscRgb(value ?? fallback);
};

const resolveAnsiColor = (terminal: Terminal, index: number): string | null => {
	const theme = terminal.options.theme ?? {};
	if (index < 16) {
		const value = themePalette(theme)[index];
		return value ? toOscRgb(value) : null;
	}
	if (index >= 16 && theme.extendedAnsi && theme.extendedAnsi[index - 16]) {
		return toOscRgb(theme.extendedAnsi[index - 16]);
	}
	return null;
};

export const registerTerminalOscHandlers = (
	id: string,
	terminal: Terminal,
	deps: TerminalOscHandlersDeps,
): { dispose: () => void }[] => {
	const disposables: { dispose: () => void }[] = [];
	const respond = (payload: string): void => {
		deps.sendInput(id, `\x1b]${payload}\x07`);
	};
	disposables.push(
		terminal.parser.registerOscHandler(10, (data) => {
			if (data !== '?') return false;
			const rgb = resolveThemeColor(
				terminal.options.theme?.foreground,
				deps.getToken('--text', '#eef3f9'),
			);
			if (!rgb) return false;
			respond(`10;${rgb}`);
			return true;
		}),
	);
	disposables.push(
		terminal.parser.registerOscHandler(11, (data) => {
			if (data !== '?') return false;
			const rgb = resolveThemeColor(
				terminal.options.theme?.background,
				deps.getToken('--panel-strong', '#111c29'),
			);
			if (!rgb) return false;
			respond(`11;${rgb}`);
			return true;
		}),
	);
	disposables.push(
		terminal.parser.registerOscHandler(12, (data) => {
			if (data !== '?') return false;
			const rgb = resolveThemeColor(
				terminal.options.theme?.cursor,
				deps.getToken('--accent', '#2d8cff'),
			);
			if (!rgb) return false;
			respond(`12;${rgb}`);
			return true;
		}),
	);
	disposables.push(
		terminal.parser.registerOscHandler(4, (data) => {
			const parts = data.split(';');
			if (parts.length < 2 || parts.length % 2 !== 0) return false;
			const responses: string[] = [];
			for (let i = 0; i < parts.length; i += 2) {
				const index = Number.parseInt(parts[i], 10);
				const query = parts[i + 1];
				if (!Number.isFinite(index) || query !== '?') {
					return false;
				}
				const rgb = resolveAnsiColor(terminal, index);
				if (!rgb) return false;
				responses.push(`4;${index};${rgb}`);
			}
			if (responses.length === 0) return false;
			for (const response of responses) {
				respond(response);
			}
			return true;
		}),
	);
	return disposables;
};
