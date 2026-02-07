type OutputChunk = {
	data: string;
	bytes: number;
};

type OutputQueueState = {
	chunks: OutputChunk[];
	bytes: number;
	scheduled: boolean;
};

type TerminalOutputBufferDeps = {
	canWrite: (id: string) => boolean;
	writeChunk: (id: string, data: string, onWritten: () => void) => void;
	onChunkFlushed?: (id: string, chunk: OutputChunk) => void;
	requestAnimationFrameFn: (callback: () => void) => number;
	flushBudgetBytes?: number;
	backlogLimitBytes?: number;
};

const DEFAULT_FLUSH_BUDGET_BYTES = 128 * 1024;
const DEFAULT_BACKLOG_LIMIT_BYTES = 512 * 1024;

export const createTerminalOutputBuffer = (deps: TerminalOutputBufferDeps) => {
	const outputQueues = new Map<string, OutputQueueState>();
	const flushBudgetBytes = deps.flushBudgetBytes ?? DEFAULT_FLUSH_BUDGET_BYTES;
	const backlogLimitBytes = deps.backlogLimitBytes ?? DEFAULT_BACKLOG_LIMIT_BYTES;

	const recalculateBytes = (queue: OutputQueueState): void => {
		queue.bytes = queue.chunks.reduce((sum, chunk) => sum + chunk.bytes, 0);
	};

	const flushOutput = (id: string, scheduled: boolean): void => {
		const queue = outputQueues.get(id);
		if (!queue) return;
		if (queue.scheduled !== scheduled) return;
		queue.scheduled = false;
		if (!deps.canWrite(id)) return;
		let budget = flushBudgetBytes;
		while (queue.chunks.length > 0 && budget > 0) {
			const chunk = queue.chunks.shift();
			if (!chunk) break;
			budget -= chunk.bytes;
			deps.writeChunk(id, chunk.data, () => undefined);
			deps.onChunkFlushed?.(id, chunk);
		}
		recalculateBytes(queue);
		if (queue.bytes > backlogLimitBytes) {
			queue.chunks.splice(0, Math.floor(queue.chunks.length / 2));
			recalculateBytes(queue);
		}
		if (queue.chunks.length > 0 && !queue.scheduled) {
			queue.scheduled = true;
			deps.requestAnimationFrameFn(() => flushOutput(id, true));
		}
	};

	const enqueueOutput = (id: string, data: string, bytes: number): void => {
		const queue = outputQueues.get(id) ?? { chunks: [], bytes: 0, scheduled: false };
		queue.chunks.push({ data, bytes });
		queue.bytes += bytes;
		outputQueues.set(id, queue);
		if (!queue.scheduled) {
			queue.scheduled = true;
			deps.requestAnimationFrameFn(() => flushOutput(id, true));
		}
	};

	const clear = (id: string): void => {
		outputQueues.delete(id);
	};

	return {
		enqueueOutput,
		flushOutput,
		clear,
	};
};
