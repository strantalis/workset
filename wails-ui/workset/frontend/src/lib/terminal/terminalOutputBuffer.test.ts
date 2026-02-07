import { describe, expect, it } from 'vitest';
import { createTerminalOutputBuffer } from './terminalOutputBuffer';

describe('createTerminalOutputBuffer', () => {
	it('flushes queued output across animation frames using the configured budget', () => {
		const frames: Array<() => void> = [];
		const writes: string[] = [];
		const flushedBytes: number[] = [];
		const buffer = createTerminalOutputBuffer({
			canWrite: () => true,
			writeChunk: (_id, data) => {
				writes.push(data);
			},
			onChunkFlushed: (_id, chunk) => {
				flushedBytes.push(chunk.bytes);
			},
			requestAnimationFrameFn: (callback) => {
				frames.push(callback);
				return frames.length;
			},
			flushBudgetBytes: 2,
		});

		buffer.enqueueOutput('alpha', 'first', 3);
		buffer.enqueueOutput('alpha', 'second', 3);

		expect(writes).toEqual([]);
		expect(frames).toHaveLength(1);

		const first = frames.shift();
		first?.();

		expect(writes).toEqual(['first']);
		expect(flushedBytes).toEqual([3]);
		expect(frames).toHaveLength(1);

		const second = frames.shift();
		second?.();

		expect(writes).toEqual(['first', 'second']);
		expect(flushedBytes).toEqual([3, 3]);
		expect(frames).toHaveLength(0);
	});

	it('keeps backlog while writes are unavailable and flushes once writes resume', () => {
		const frames: Array<() => void> = [];
		const writes: string[] = [];
		let writable = false;
		const buffer = createTerminalOutputBuffer({
			canWrite: () => writable,
			writeChunk: (_id, data) => {
				writes.push(data);
			},
			requestAnimationFrameFn: (callback) => {
				frames.push(callback);
				return frames.length;
			},
		});

		buffer.enqueueOutput('beta', 'queued', 6);
		expect(frames).toHaveLength(1);
		frames.shift()?.();
		expect(writes).toEqual([]);

		writable = true;
		buffer.flushOutput('beta', false);
		expect(writes).toEqual(['queued']);
	});

	it('drops buffered output when cleared', () => {
		const frames: Array<() => void> = [];
		const writes: string[] = [];
		const buffer = createTerminalOutputBuffer({
			canWrite: () => true,
			writeChunk: (_id, data) => {
				writes.push(data);
			},
			requestAnimationFrameFn: (callback) => {
				frames.push(callback);
				return frames.length;
			},
		});

		buffer.enqueueOutput('gamma', 'one', 3);
		buffer.enqueueOutput('gamma', 'two', 3);
		buffer.clear('gamma');

		frames.shift()?.();
		expect(writes).toEqual([]);
	});
});
