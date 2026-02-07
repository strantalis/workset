import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createTerminalReplayAckOrchestrator } from './terminalReplayAckOrchestrator';

const createOrchestrator = (overrides?: {
	hasTerminalHandle?: (id: string) => boolean;
	ackBatchBytes?: number;
	ackFlushDelayMs?: number;
}) => {
	const enqueueOutput = vi.fn();
	const flushOutput = vi.fn();
	const forceRedraw = vi.fn();
	const applyKittyEvent = vi.fn(async () => undefined);
	const ack = vi.fn(async () => undefined);
	const recordBytesIn = vi.fn();
	const noteOutputActivity = vi.fn();
	const hasTerminalHandle = vi.fn(overrides?.hasTerminalHandle ?? (() => true));

	const orchestrator = createTerminalReplayAckOrchestrator<{ kind: string }>({
		enqueueOutput,
		flushOutput,
		forceRedraw,
		hasTerminalHandle,
		applyKittyEvent,
		getWorkspaceId: () => 'ws',
		getTerminalId: () => 'term',
		ack,
		setTimeoutFn: (callback, timeoutMs) => window.setTimeout(callback, timeoutMs),
		clearTimeoutFn: (handle) => window.clearTimeout(handle),
		countBytes: (data) => data.length,
		recordBytesIn,
		noteOutputActivity,
		ackBatchBytes: overrides?.ackBatchBytes ?? 1024,
		ackFlushDelayMs: overrides?.ackFlushDelayMs ?? 25,
		initialStreamCredit: 2048,
	});

	return {
		orchestrator,
		enqueueOutput,
		flushOutput,
		forceRedraw,
		applyKittyEvent,
		ack,
		recordBytesIn,
		noteOutputActivity,
		hasTerminalHandle,
	};
};

describe('terminalReplayAckOrchestrator', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('replays buffered output and kitty snapshot when stream becomes live', () => {
		const { orchestrator, enqueueOutput, flushOutput, forceRedraw, applyKittyEvent, ack } =
			createOrchestrator();
		const id = 'ws::term';
		orchestrator.initialCreditMap.set(id, 8192);
		orchestrator.setReplayState(id, 'replaying');
		orchestrator.handleTerminalData(id, { data: 'snapshot' });
		orchestrator.handleTerminalKitty(id, { kind: 'snapshot' });

		orchestrator.setReplayState(id, 'live');

		expect(enqueueOutput).toHaveBeenCalledWith(id, 'snapshot', 8);
		expect(applyKittyEvent).toHaveBeenCalledWith(id, { kind: 'snapshot' });
		expect(flushOutput).toHaveBeenCalledWith(id, true);
		expect(forceRedraw).toHaveBeenCalledWith(id);
		expect(ack).toHaveBeenCalledWith('ws', 'term', 8192);
	});

	it('records ack credit for live data and flushes immediately at threshold', () => {
		const { orchestrator, ack, enqueueOutput, recordBytesIn, noteOutputActivity } =
			createOrchestrator({ ackBatchBytes: 10 });
		const id = 'ws::term';
		orchestrator.setReplayState(id, 'live');
		ack.mockClear();

		orchestrator.handleTerminalData(id, { data: 'hello', bytes: 12 });

		expect(enqueueOutput).toHaveBeenCalledWith(id, 'hello', 12);
		expect(recordBytesIn).toHaveBeenCalledWith(id, 12);
		expect(noteOutputActivity).toHaveBeenCalledWith(id);
		expect(ack).toHaveBeenCalledWith('ws', 'term', 12);
	});

	it('batches delayed acknowledgements below the threshold', () => {
		const { orchestrator, ack } = createOrchestrator({ ackBatchBytes: 100, ackFlushDelayMs: 25 });
		const id = 'ws::term';
		orchestrator.setReplayState(id, 'live');
		ack.mockClear();

		orchestrator.handleTerminalData(id, { data: '12345', bytes: 30 });
		orchestrator.handleTerminalData(id, { data: 'abcde', bytes: 20 });

		expect(ack).not.toHaveBeenCalled();
		vi.advanceTimersByTime(24);
		expect(ack).not.toHaveBeenCalled();
		vi.advanceTimersByTime(1);
		expect(ack).toHaveBeenCalledTimes(1);
		expect(ack).toHaveBeenCalledWith('ws', 'term', 50);
	});

	it('clears pending ack timers when resetting session state', () => {
		const { orchestrator, ack } = createOrchestrator({ ackBatchBytes: 100, ackFlushDelayMs: 25 });
		const id = 'ws::term';
		orchestrator.setReplayState(id, 'live');
		ack.mockClear();
		orchestrator.handleTerminalData(id, { data: 'abcde', bytes: 5 });

		orchestrator.resetSession(id);
		vi.advanceTimersByTime(50);

		expect(ack).not.toHaveBeenCalled();
	});
});
