export type DebugOverlayPreference = 'on' | 'off' | '';

type TerminalDebugStateDeps = {
	emitAllStates: () => void;
};

export const createTerminalDebugState = (deps: TerminalDebugStateDeps) => {
	let debugEnabled = false;
	let debugOverlayPreference: DebugOverlayPreference = '';

	const resolveDebugEnabled = (): boolean => {
		if (debugOverlayPreference === 'on') return true;
		if (debugOverlayPreference === 'off') return false;
		if (typeof localStorage !== 'undefined') {
			return localStorage.getItem('worksetTerminalDebug') === '1';
		}
		return false;
	};

	const syncDebugEnabled = (): void => {
		const next = resolveDebugEnabled();
		if (next === debugEnabled) return;
		debugEnabled = next;
		deps.emitAllStates();
	};

	const isDebugEnabled = (): boolean => debugEnabled;

	const getDebugOverlayPreference = (): DebugOverlayPreference => debugOverlayPreference;

	const setDebugOverlayPreference = (value: DebugOverlayPreference): void => {
		debugOverlayPreference = value;
	};

	return {
		syncDebugEnabled,
		isDebugEnabled,
		getDebugOverlayPreference,
		setDebugOverlayPreference,
	};
};
