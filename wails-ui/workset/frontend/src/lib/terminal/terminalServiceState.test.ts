import { describe, expect, it } from 'vitest';
import { createTerminalServiceState } from './terminalServiceState';

describe('terminalServiceState output buffering', () => {
	it('buffers and consumes output chunks in FIFO order', () => {
		const state = createTerminalServiceState({
			maxBufferedOutputBytesPerTerminal: 32,
		});
		const chunkA = new Uint8Array([0x1b, 0x5b, 0x31]);
		const chunkB = new Uint8Array([0x6f, 0x70, 0x65, 0x6e]);

		const first = state.bufferOutputChunk('ws::term', {
			seq: 10,
			bytes: chunkA.length,
			chunk: chunkA,
		});
		expect(first).toEqual({
			bufferedChunks: 1,
			bufferedBytes: 3,
			droppedChunks: 0,
			droppedBytes: 0,
		});

		const second = state.bufferOutputChunk('ws::term', {
			seq: 11,
			bytes: chunkB.length,
			chunk: chunkB,
		});
		expect(second).toEqual({
			bufferedChunks: 2,
			bufferedBytes: 7,
			droppedChunks: 0,
			droppedBytes: 0,
		});

		const queued = state.consumeBufferedOutput('ws::term');
		expect(queued.map((item) => item.seq)).toEqual([10, 11]);
		expect(queued.map((item) => item.chunk.length)).toEqual([3, 4]);
		expect(state.getBufferedOutputSnapshot('ws::term')).toEqual({
			bufferedChunks: 0,
			bufferedBytes: 0,
		});
	});

	it('drops oldest chunks when buffer exceeds byte limit', () => {
		const state = createTerminalServiceState({
			maxBufferedOutputBytesPerTerminal: 6,
		});

		state.bufferOutputChunk('ws::term', {
			seq: 1,
			bytes: 3,
			chunk: new Uint8Array([0x61, 0x62, 0x63]),
		});
		state.bufferOutputChunk('ws::term', {
			seq: 2,
			bytes: 3,
			chunk: new Uint8Array([0x64, 0x65, 0x66]),
		});
		const result = state.bufferOutputChunk('ws::term', {
			seq: 3,
			bytes: 3,
			chunk: new Uint8Array([0x67, 0x68, 0x69]),
		});

		expect(result).toEqual({
			bufferedChunks: 2,
			bufferedBytes: 6,
			droppedChunks: 1,
			droppedBytes: 3,
		});
		const queued = state.consumeBufferedOutput('ws::term');
		expect(queued.map((item) => item.seq)).toEqual([2, 3]);
	});

	it('retains a single oversize chunk when it is the only buffered output', () => {
		const state = createTerminalServiceState({
			maxBufferedOutputBytesPerTerminal: 4,
		});
		const oversize = new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8]);

		const result = state.bufferOutputChunk('ws::term', {
			seq: 42,
			bytes: oversize.length,
			chunk: oversize,
		});
		expect(result).toEqual({
			bufferedChunks: 1,
			bufferedBytes: 8,
			droppedChunks: 0,
			droppedBytes: 0,
		});

		const queued = state.consumeBufferedOutput('ws::term');
		expect(queued).toHaveLength(1);
		expect(queued[0]?.seq).toBe(42);
		expect(queued[0]?.chunk.length).toBe(8);
	});

	it('reorders stream chunks by seq before flush', () => {
		let now = 1000;
		const state = createTerminalServiceState({
			now: () => now,
		});

		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 12,
			bytes: 3,
			chunk: new Uint8Array([0x63, 0x63, 0x63]),
			receivedAt: now,
		});
		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 10,
			bytes: 3,
			chunk: new Uint8Array([0x61, 0x61, 0x61]),
			receivedAt: now + 1,
		});
		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 11,
			bytes: 3,
			chunk: new Uint8Array([0x62, 0x62, 0x62]),
			receivedAt: now + 2,
		});

		now += 10;
		const flushed = state.consumeOrderedStreamChunks('ws::term', {
			force: true,
			minAgeMs: 5,
		});
		expect(flushed.droppedStaleChunks).toBe(0);
		expect(flushed.chunks.map((item) => item.seq)).toEqual([10, 11, 12]);
	});

	it('drops stale and duplicate ordered stream chunks', () => {
		const state = createTerminalServiceState();

		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 100,
			bytes: 1,
			chunk: new Uint8Array([0x01]),
			receivedAt: 1,
		});
		state.consumeOrderedStreamChunks('ws::term', { force: true });

		const stale = state.enqueueOrderedStreamChunk('ws::term', {
			seq: 99,
			bytes: 1,
			chunk: new Uint8Array([0x02]),
			receivedAt: 2,
		});
		expect(stale.droppedStaleChunks).toBe(1);

		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 101,
			bytes: 1,
			chunk: new Uint8Array([0x03]),
			receivedAt: 3,
		});
		const duplicate = state.enqueueOrderedStreamChunk('ws::term', {
			seq: 101,
			bytes: 1,
			chunk: new Uint8Array([0x04]),
			receivedAt: 4,
		});
		expect(duplicate.droppedDuplicateChunks).toBe(1);
	});

	it('holds ordered stream chunks until they are old enough when not forced', () => {
		let now = 200;
		const state = createTerminalServiceState({
			now: () => now,
		});

		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 1,
			bytes: 1,
			chunk: new Uint8Array([0x61]),
			receivedAt: now,
		});

		const immediate = state.consumeOrderedStreamChunks('ws::term', {
			minAgeMs: 8,
		});
		expect(immediate.chunks).toHaveLength(0);

		now += 9;
		const delayed = state.consumeOrderedStreamChunks('ws::term', {
			minAgeMs: 8,
		});
		expect(delayed.chunks.map((item) => item.seq)).toEqual([1]);
	});

	it('holds across seq gaps in normal mode and recovers on forced flush', () => {
		const state = createTerminalServiceState();

		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 248,
			bytes: 1,
			chunk: new Uint8Array([0x01]),
			receivedAt: 1,
		});
		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 249,
			bytes: 1,
			chunk: new Uint8Array([0x02]),
			receivedAt: 2,
		});
		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 251,
			bytes: 1,
			chunk: new Uint8Array([0x03]),
			receivedAt: 3,
		});

		const first = state.consumeOrderedStreamChunks('ws::term');
		expect(first.chunks.map((item) => item.seq)).toEqual([248, 249]);

		const blocked = state.consumeOrderedStreamChunks('ws::term');
		expect(blocked.chunks).toHaveLength(0);

		const recovered = state.consumeOrderedStreamChunks('ws::term', {
			force: true,
		});
		expect(recovered.chunks.map((item) => item.seq)).toEqual([251]);
		expect(recovered.droppedStaleChunks).toBe(1);

		const stale = state.enqueueOrderedStreamChunk('ws::term', {
			seq: 250,
			bytes: 1,
			chunk: new Uint8Array([0x04]),
			receivedAt: 4,
		});
		expect(stale.droppedStaleChunks).toBe(1);
	});

	it('reports ordered stream snapshot for blocked flush diagnostics', () => {
		const state = createTerminalServiceState();
		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 10,
			bytes: 2,
			chunk: new Uint8Array([0x61, 0x62]),
			receivedAt: 1,
		});
		state.enqueueOrderedStreamChunk('ws::term', {
			seq: 11,
			bytes: 3,
			chunk: new Uint8Array([0x63, 0x64, 0x65]),
			receivedAt: 2,
		});

		const snapshot = state.getOrderedStreamSnapshot('ws::term');
		expect(snapshot).toEqual({
			queuedChunks: 2,
			queuedBytes: 5,
			firstSeq: 10,
			lastSeq: 11,
			lastDeliveredSeq: 0,
		});
	});
});
