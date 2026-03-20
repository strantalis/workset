type TerminalFontSizeControllerDeps = {
	onFontSizeChange: (fontSize: number) => void;
	onCursorBlinkChange: (cursorBlink: boolean) => void;
	defaultFontSize?: number;
	minFontSize?: number;
	maxFontSize?: number;
	step?: number;
	defaultCursorBlink?: boolean;
};

export const DEFAULT_TERMINAL_FONT_SIZE = 13;
export const MIN_TERMINAL_FONT_SIZE = 8;
export const MAX_TERMINAL_FONT_SIZE = 28;

const clamp = (value: number, min: number, max: number): number =>
	Math.min(max, Math.max(min, value));

export const createTerminalFontSizeController = (deps: TerminalFontSizeControllerDeps) => {
	const defaultFontSize = deps.defaultFontSize ?? DEFAULT_TERMINAL_FONT_SIZE;
	const minFontSize = deps.minFontSize ?? MIN_TERMINAL_FONT_SIZE;
	const maxFontSize = deps.maxFontSize ?? MAX_TERMINAL_FONT_SIZE;
	const step = deps.step ?? 1;
	const defaultCursorBlink = deps.defaultCursorBlink ?? true;
	let currentFontSize = defaultFontSize;
	let currentCursorBlink = defaultCursorBlink;

	const apply = (next: number): void => {
		if (next === currentFontSize) return;
		currentFontSize = next;
		deps.onFontSizeChange(currentFontSize);
	};

	const applyCursorBlink = (next: boolean): void => {
		if (next === currentCursorBlink) return;
		currentCursorBlink = next;
		deps.onCursorBlinkChange(currentCursorBlink);
	};

	return {
		getCurrentFontSize: (): number => currentFontSize,
		getCursorBlink: (): boolean => currentCursorBlink,
		setFontSize: (next: number): void => {
			apply(clamp(next, minFontSize, maxFontSize));
		},
		setCursorBlink: (next: boolean): void => {
			applyCursorBlink(next);
		},
		increaseFontSize: (): void => {
			apply(Math.min(currentFontSize + step, maxFontSize));
		},
		decreaseFontSize: (): void => {
			apply(Math.max(currentFontSize - step, minFontSize));
		},
		resetFontSize: (): void => {
			apply(defaultFontSize);
		},
	};
};
