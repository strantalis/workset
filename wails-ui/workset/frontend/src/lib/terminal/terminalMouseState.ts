export const createTerminalMouseState = () => {
	let suppressMouseUntil: Record<string, number> = {};
	let mouseInputTail: Record<string, string> = {};

	const noteSuppress = (id: string, durationMs: number): void => {
		suppressMouseUntil = { ...suppressMouseUntil, [id]: Date.now() + durationMs };
	};

	const shouldSuppressInput = (id: string, data: string): boolean => {
		const until = suppressMouseUntil[id];
		if (!until || Date.now() >= until) {
			return false;
		}
		return data.includes('\x1b[<');
	};

	const getTail = (id: string): string => mouseInputTail[id] ?? '';

	const setTail = (id: string, tail: string): void => {
		mouseInputTail = { ...mouseInputTail, [id]: tail };
	};

	const clearSuppression = (id: string): void => {
		if (!Object.prototype.hasOwnProperty.call(suppressMouseUntil, id)) return;
		const next = { ...suppressMouseUntil };
		delete next[id];
		suppressMouseUntil = next;
	};

	const clearTail = (id: string): void => {
		if (!mouseInputTail[id]) return;
		mouseInputTail = { ...mouseInputTail, [id]: '' };
	};

	return {
		noteSuppress,
		shouldSuppressInput,
		getTail,
		setTail,
		clearSuppression,
		clearTail,
	};
};
