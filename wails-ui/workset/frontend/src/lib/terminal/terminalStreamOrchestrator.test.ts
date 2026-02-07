import { describe, expect, it, vi } from 'vitest';
import { createTerminalStreamOrchestrator } from './terminalStreamOrchestrator';

describe('terminalStreamOrchestrator', () => {
	it('schedules one reattach timer per terminal key', () => {
		vi.useFakeTimers();
		const ensureSessionActive = vi.fn(async () => undefined);
		const logDebug = vi.fn();
		const orchestrator = createTerminalStreamOrchestrator({
			ensureSessionActive,
			initTerminal: async () => undefined,
			getContext: () => null,
			hasStarted: () => false,
			getStatus: () => 'standby',
			ensureStream: async () => undefined,
			beginTerminal: async () => undefined,
			emitState: () => undefined,
			logDebug,
		});

		orchestrator.scheduleReattachCheck('ws::term', 'first');
		orchestrator.scheduleReattachCheck('ws::term', 'second');
		vi.advanceTimersByTime(239);
		expect(ensureSessionActive).not.toHaveBeenCalled();
		vi.advanceTimersByTime(1);
		expect(ensureSessionActive).toHaveBeenCalledTimes(1);
		expect(ensureSessionActive).toHaveBeenCalledWith('ws::term');
		expect(logDebug).toHaveBeenCalledTimes(2);
		vi.useRealTimers();
	});

	it('clears a pending reattach timer', () => {
		vi.useFakeTimers();
		const ensureSessionActive = vi.fn(async () => undefined);
		const orchestrator = createTerminalStreamOrchestrator({
			ensureSessionActive,
			initTerminal: async () => undefined,
			getContext: () => null,
			hasStarted: () => false,
			getStatus: () => 'standby',
			ensureStream: async () => undefined,
			beginTerminal: async () => undefined,
			emitState: () => undefined,
			logDebug: () => undefined,
		});

		orchestrator.scheduleReattachCheck('ws::term', 'detach');
		orchestrator.clearReattachTimer('ws::term');
		vi.advanceTimersByTime(500);
		expect(ensureSessionActive).not.toHaveBeenCalled();
		vi.useRealTimers();
	});

	it('ensures stream for started sessions during sync', async () => {
		const order: string[] = [];
		const orchestrator = createTerminalStreamOrchestrator({
			ensureSessionActive: async () => {
				order.push('ensureSessionActive');
			},
			initTerminal: async () => {
				order.push('initTerminal');
			},
			getContext: () => ({ container: document.createElement('div'), active: true }),
			hasStarted: () => true,
			getStatus: () => 'ready',
			ensureStream: async () => {
				order.push('ensureStream');
			},
			beginTerminal: async () => {
				order.push('beginTerminal');
			},
			emitState: () => {
				order.push('emitState');
			},
			logDebug: () => undefined,
		});

		orchestrator.syncTerminalStream('ws::term');
		await Promise.resolve();
		await Promise.resolve();

		expect(order).toEqual(['initTerminal', 'ensureStream', 'ensureSessionActive', 'emitState']);
	});

	it('starts standby sessions when not started during sync', async () => {
		const beginTerminal = vi.fn(async () => undefined);
		const ensureStream = vi.fn(async () => undefined);
		const orchestrator = createTerminalStreamOrchestrator({
			ensureSessionActive: async () => undefined,
			initTerminal: async () => undefined,
			getContext: () => ({ container: document.createElement('div'), active: false }),
			hasStarted: () => false,
			getStatus: () => 'standby',
			ensureStream,
			beginTerminal,
			emitState: () => undefined,
			logDebug: () => undefined,
		});

		orchestrator.syncTerminalStream('ws::term');
		await Promise.resolve();
		await Promise.resolve();

		expect(beginTerminal).toHaveBeenCalledTimes(1);
		expect(beginTerminal).toHaveBeenCalledWith('ws::term', true);
		expect(ensureStream).not.toHaveBeenCalled();
	});
});
