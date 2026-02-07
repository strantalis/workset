type TerminalFontSizeControllerDeps = {
	onFontSizeChange: (fontSize: number) => void;
	storageKey?: string;
	defaultFontSize?: number;
	minFontSize?: number;
	maxFontSize?: number;
	step?: number;
};

const clamp = (value: number, min: number, max: number): number =>
	Math.min(max, Math.max(min, value));

export const createTerminalFontSizeController = (deps: TerminalFontSizeControllerDeps) => {
	const storageKey = deps.storageKey ?? 'worksetTerminalFontSize';
	const defaultFontSize = deps.defaultFontSize ?? 13;
	const minFontSize = deps.minFontSize ?? 8;
	const maxFontSize = deps.maxFontSize ?? 28;
	const step = deps.step ?? 1;

	const loadInitial = (): number => {
		if (typeof localStorage === 'undefined') return defaultFontSize;
		try {
			const stored = localStorage.getItem(storageKey);
			if (!stored) return defaultFontSize;
			const parsed = Number.parseInt(stored, 10);
			if (Number.isNaN(parsed)) return defaultFontSize;
			return clamp(parsed, minFontSize, maxFontSize);
		} catch {
			return defaultFontSize;
		}
	};

	const persist = (value: number): void => {
		if (typeof localStorage === 'undefined') return;
		try {
			localStorage.setItem(storageKey, String(value));
		} catch {
			// Ignore storage failures.
		}
	};

	let currentFontSize = loadInitial();

	const apply = (next: number): void => {
		if (next === currentFontSize) return;
		currentFontSize = next;
		persist(currentFontSize);
		deps.onFontSizeChange(currentFontSize);
	};

	return {
		getCurrentFontSize: (): number => currentFontSize,
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
