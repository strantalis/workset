export type TerminalDebugStats = {
	bytesIn: number;
	bytesOut: number;
	lastOutputAt: number;
	lastCprAt: number;
};

type TerminalServiceStateInput = {
	now?: () => number;
	maxBufferedOutputBytesPerTerminal?: number;
};

export type TerminalBufferedOutputChunk = {
	seq: number;
	bytes: number;
	chunk: Uint8Array;
};

type TerminalBufferedOutputState = {
	totalBytes: number;
	chunks: TerminalBufferedOutputChunk[];
};

type BufferOutputResult = {
	bufferedChunks: number;
	bufferedBytes: number;
	droppedChunks: number;
	droppedBytes: number;
};

export type TerminalOrderedStreamChunk = {
	seq: number;
	bytes: number;
	chunk: Uint8Array;
	receivedAt: number;
};

type TerminalOrderedStreamState = {
	lastSeq: number;
	chunks: TerminalOrderedStreamChunk[];
};

type EnqueueOrderedStreamResult = {
	queuedChunks: number;
	queuedBytes: number;
	droppedStaleChunks: number;
	droppedDuplicateChunks: number;
};

type ConsumeOrderedStreamResult = {
	chunks: TerminalBufferedOutputChunk[];
	droppedStaleChunks: number;
};

type OrderedStreamSnapshot = {
	queuedChunks: number;
	queuedBytes: number;
	firstSeq: number;
	lastSeq: number;
	lastDeliveredSeq: number;
};

const DEFAULT_MAX_BUFFERED_OUTPUT_BYTES_PER_TERMINAL = 512 * 1024;

const createDefaultStats = (): TerminalDebugStats => ({
	bytesIn: 0,
	bytesOut: 0,
	lastOutputAt: 0,
	lastCprAt: 0,
});

export const createTerminalServiceState = (input: TerminalServiceStateInput = {}) => {
	const pendingInput = new Map<string, string>();
	const statsMap = new Map<string, TerminalDebugStats>();
	const pendingOutput = new Map<string, TerminalBufferedOutputState>();
	const orderedStreamOutput = new Map<string, TerminalOrderedStreamState>();
	const now = input.now ?? Date.now;
	const maxBufferedOutputBytesPerTerminal =
		input.maxBufferedOutputBytesPerTerminal ?? DEFAULT_MAX_BUFFERED_OUTPUT_BYTES_PER_TERMINAL;

	const getStats = (id: string): TerminalDebugStats => statsMap.get(id) ?? createDefaultStats();

	const getStatsSnapshot = (id: string): TerminalDebugStats => ({ ...getStats(id) });

	const getBufferedOutputState = (id: string): TerminalBufferedOutputState => {
		const existing = pendingOutput.get(id);
		if (existing) return existing;
		const created: TerminalBufferedOutputState = {
			totalBytes: 0,
			chunks: [],
		};
		pendingOutput.set(id, created);
		return created;
	};

	const getOrderedStreamState = (id: string): TerminalOrderedStreamState => {
		const existing = orderedStreamOutput.get(id);
		if (existing) return existing;
		const created: TerminalOrderedStreamState = {
			lastSeq: 0,
			chunks: [],
		};
		orderedStreamOutput.set(id, created);
		return created;
	};

	const updateStats = (id: string, update: (stats: TerminalDebugStats) => void): void => {
		const stats = getStats(id);
		update(stats);
		statsMap.set(id, stats);
	};

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

	const bufferOutputChunk = (
		id: string,
		value: TerminalBufferedOutputChunk,
	): BufferOutputResult => {
		const state = getBufferedOutputState(id);
		state.chunks.push(value);
		state.totalBytes += value.chunk.length;

		let droppedChunks = 0;
		let droppedBytes = 0;
		while (state.chunks.length > 1 && state.totalBytes > maxBufferedOutputBytesPerTerminal) {
			const dropped = state.chunks.shift();
			if (!dropped) break;
			droppedChunks += 1;
			droppedBytes += dropped.chunk.length;
			state.totalBytes -= dropped.chunk.length;
		}

		if (state.totalBytes <= 0 || state.chunks.length === 0) {
			pendingOutput.delete(id);
			return {
				bufferedChunks: 0,
				bufferedBytes: 0,
				droppedChunks,
				droppedBytes,
			};
		}

		pendingOutput.set(id, state);
		return {
			bufferedChunks: state.chunks.length,
			bufferedBytes: state.totalBytes,
			droppedChunks,
			droppedBytes,
		};
	};

	const consumeBufferedOutput = (id: string): TerminalBufferedOutputChunk[] => {
		const state = pendingOutput.get(id);
		if (!state || state.chunks.length === 0) {
			pendingOutput.delete(id);
			return [];
		}
		pendingOutput.delete(id);
		return state.chunks;
	};

	const getBufferedOutputSnapshot = (
		id: string,
	): { bufferedChunks: number; bufferedBytes: number } => {
		const state = pendingOutput.get(id);
		if (!state) {
			return {
				bufferedChunks: 0,
				bufferedBytes: 0,
			};
		}
		return {
			bufferedChunks: state.chunks.length,
			bufferedBytes: state.totalBytes,
		};
	};

	const deletePendingOutput = (id: string): void => {
		pendingOutput.delete(id);
	};

	const enqueueOrderedStreamChunk = (
		id: string,
		value: TerminalOrderedStreamChunk,
	): EnqueueOrderedStreamResult => {
		const state = getOrderedStreamState(id);
		if (value.seq > 0 && state.lastSeq > 0 && value.seq <= state.lastSeq) {
			const queuedBytes = state.chunks.reduce((sum, item) => sum + item.bytes, 0);
			return {
				queuedChunks: state.chunks.length,
				queuedBytes,
				droppedStaleChunks: 1,
				droppedDuplicateChunks: 0,
			};
		}
		if (value.seq > 0 && state.chunks.some((item) => item.seq === value.seq)) {
			const queuedBytes = state.chunks.reduce((sum, item) => sum + item.bytes, 0);
			return {
				queuedChunks: state.chunks.length,
				queuedBytes,
				droppedStaleChunks: 0,
				droppedDuplicateChunks: 1,
			};
		}
		const insertAt = state.chunks.findIndex((item) => item.seq > value.seq);
		if (insertAt < 0) {
			state.chunks.push(value);
		} else {
			state.chunks.splice(insertAt, 0, value);
		}
		orderedStreamOutput.set(id, state);
		const queuedBytes = state.chunks.reduce((sum, item) => sum + item.bytes, 0);
		return {
			queuedChunks: state.chunks.length,
			queuedBytes,
			droppedStaleChunks: 0,
			droppedDuplicateChunks: 0,
		};
	};

	const consumeOrderedStreamChunks = (
		id: string,
		options: {
			force?: boolean;
			minAgeMs?: number;
		} = {},
	): ConsumeOrderedStreamResult => {
		const state = orderedStreamOutput.get(id);
		if (!state || state.chunks.length === 0) {
			return {
				chunks: [],
				droppedStaleChunks: 0,
			};
		}

		const minAgeMs = options.minAgeMs ?? 0;
		if (!options.force && minAgeMs > 0) {
			const oldest = state.chunks[0];
			if (now() - oldest.receivedAt < minAgeMs) {
				return {
					chunks: [],
					droppedStaleChunks: 0,
				};
			}
		}

		const ready: TerminalBufferedOutputChunk[] = [];
		let droppedStaleChunks = 0;
		let cursor = 0;
		while (cursor < state.chunks.length) {
			const item = state.chunks[cursor];
			if (item.seq > 0) {
				if (state.lastSeq > 0) {
					if (item.seq <= state.lastSeq) {
						droppedStaleChunks += 1;
						cursor += 1;
						continue;
					}
					const expectedSeq = state.lastSeq + 1;
					// Preserve strict in-order delivery for control sequences.
					// If there is a gap, keep newer chunks queued until missing seq arrives.
					if (item.seq !== expectedSeq) {
						if (!options.force) {
							break;
						}
						// Forced flush mode is a recovery path: if one seq is dropped,
						// move forward so output rendering does not deadlock indefinitely.
						droppedStaleChunks += item.seq - expectedSeq;
						state.lastSeq = item.seq - 1;
					}
					if (item.seq !== state.lastSeq + 1) {
						break;
					}
				}
				ready.push({
					seq: item.seq,
					bytes: item.bytes,
					chunk: item.chunk,
				});
				state.lastSeq = item.seq;
				cursor += 1;
				continue;
			}
			ready.push({
				seq: item.seq,
				bytes: item.bytes,
				chunk: item.chunk,
			});
			cursor += 1;
		}
		state.chunks = state.chunks.slice(cursor);
		orderedStreamOutput.set(id, state);
		return {
			chunks: ready,
			droppedStaleChunks,
		};
	};

	const getOrderedStreamSnapshot = (id: string): OrderedStreamSnapshot => {
		const state = orderedStreamOutput.get(id);
		if (!state || state.chunks.length === 0) {
			return {
				queuedChunks: 0,
				queuedBytes: 0,
				firstSeq: 0,
				lastSeq: 0,
				lastDeliveredSeq: state?.lastSeq ?? 0,
			};
		}
		let queuedBytes = 0;
		for (const chunk of state.chunks) {
			queuedBytes += chunk.bytes;
		}
		return {
			queuedChunks: state.chunks.length,
			queuedBytes,
			firstSeq: state.chunks[0]?.seq ?? 0,
			lastSeq: state.chunks[state.chunks.length - 1]?.seq ?? 0,
			lastDeliveredSeq: state.lastSeq,
		};
	};

	const resetOrderedStream = (id: string): void => {
		orderedStreamOutput.delete(id);
	};

	return {
		pendingInput,
		getStatsSnapshot,
		updateStats,
		markLastOutput,
		markCprResponse,
		bufferOutputChunk,
		consumeBufferedOutput,
		getBufferedOutputSnapshot,
		deleteStats,
		deletePendingInput,
		deletePendingOutput,
		enqueueOrderedStreamChunk,
		consumeOrderedStreamChunks,
		getOrderedStreamSnapshot,
		resetOrderedStream,
	};
};
