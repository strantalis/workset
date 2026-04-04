export type DebugOverlayPreference = 'on' | 'off' | '';
export type LifecycleLogPreference = 'on' | 'off' | '';

type TerminalDebugStateDeps = {
	emitAllStates: () => void;
};

export const createTerminalDebugState = (deps: TerminalDebugStateDeps) => {
	let debugEnabled = false;
	let debugOverlayPreference: DebugOverlayPreference = '';
	let lifecycleLogPreference: LifecycleLogPreference = '';

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
	const isLifecycleLoggingEnabled = (): boolean => lifecycleLogPreference === 'on' || debugEnabled;

	const getDebugOverlayPreference = (): DebugOverlayPreference => debugOverlayPreference;
	const getLifecycleLogPreference = (): LifecycleLogPreference => lifecycleLogPreference;

	const setDebugOverlayPreference = (value: DebugOverlayPreference): void => {
		debugOverlayPreference = value;
	};

	const setLifecycleLogPreference = (value: LifecycleLogPreference): void => {
		lifecycleLogPreference = value;
	};

	return {
		syncDebugEnabled,
		isDebugEnabled,
		isLifecycleLoggingEnabled,
		getDebugOverlayPreference,
		getLifecycleLogPreference,
		setDebugOverlayPreference,
		setLifecycleLogPreference,
	};
};
