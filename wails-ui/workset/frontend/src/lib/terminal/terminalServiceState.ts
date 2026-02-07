export type TerminalDebugStats = {
	bytesIn: number;
	bytesOut: number;
	backlog: number;
	lastOutputAt: number;
	lastCprAt: number;
};

type TerminalServiceStateInput = {
	textEncoder?: TextEncoder | null;
	now?: () => number;
};

const createDefaultStats = (): TerminalDebugStats => ({
	bytesIn: 0,
	bytesOut: 0,
	backlog: 0,
	lastOutputAt: 0,
	lastCprAt: 0,
});

export const createTerminalServiceState = (input: TerminalServiceStateInput = {}) => {
	const bootstrapHandled = new Map<string, boolean>();
	const bootstrapFetchTimers = new Map<string, number>();
	const pendingInput = new Map<string, string>();
	const statsMap = new Map<string, TerminalDebugStats>();
	const now = input.now ?? Date.now;
	const textEncoder =
		input.textEncoder ?? (typeof TextEncoder !== 'undefined' ? new TextEncoder() : null);

	const getStats = (id: string): TerminalDebugStats => statsMap.get(id) ?? createDefaultStats();

	const getStatsSnapshot = (id: string): TerminalDebugStats => ({ ...getStats(id) });

	const updateStats = (id: string, update: (stats: TerminalDebugStats) => void): void => {
		const stats = getStats(id);
		update(stats);
		statsMap.set(id, stats);
	};

	const countBytes = (data: string): number =>
		textEncoder ? textEncoder.encode(data).length : data.length;

	const markLastOutput = (id: string): void => {
		updateStats(id, (stats) => {
			stats.lastOutputAt = now();
		});
	};

	const markCprResponse = (id: string, data: string): boolean => {
		const cprIndex = data.indexOf('\x1b[');
		if (cprIndex < 0) return false;
		const match = data.slice(cprIndex + 2).match(/^(\d+);(\d+)R/);
		if (!match) return false;
		updateStats(id, (stats) => {
			stats.lastCprAt = now();
		});
		return true;
	};

	const deleteStats = (id: string): void => {
		statsMap.delete(id);
	};

	const deletePendingInput = (id: string): void => {
		pendingInput.delete(id);
	};

	return {
		bootstrapHandled,
		bootstrapFetchTimers,
		pendingInput,
		getStatsSnapshot,
		updateStats,
		countBytes,
		markLastOutput,
		markCprResponse,
		deleteStats,
		deletePendingInput,
	};
};
