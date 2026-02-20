import { describe, expect, it, vi } from 'vitest';
import { createTerminalStreamOrchestrator } from './terminalStreamOrchestrator';

describe('terminalStreamOrchestrator', () => {
	it('begins terminal for attached sessions during sync', async () => {
		const order: string[] = [];
		let token = 0;
		const orchestrator = createTerminalStreamOrchestrator({
			initTerminal: async () => {
				order.push('initTerminal');
			},
			getContext: () => ({ container: document.createElement('div'), active: true }),
			beginTerminal: async () => {
				order.push('beginTerminal');
			},
			nextSyncToken: () => {
				token += 1;
				return token;
			},
			isCurrentSyncToken: (_id, currentToken) => currentToken === token,
			emitState: () => {
				order.push('emitState');
			},
		});

		orchestrator.syncTerminalStream('ws::term');
		await Promise.resolve();
		await Promise.resolve();

		expect(order).toEqual(['initTerminal', 'beginTerminal', 'emitState']);
	});

	it('starts terminal quietly for inactive panes', async () => {
		const beginTerminal = vi.fn(async () => undefined);
		let token = 0;
		const orchestrator = createTerminalStreamOrchestrator({
			initTerminal: async () => undefined,
			getContext: () => ({ container: document.createElement('div'), active: false }),
			beginTerminal,
			nextSyncToken: () => {
				token += 1;
				return token;
			},
			isCurrentSyncToken: (_id, currentToken) => currentToken === token,
			emitState: () => undefined,
		});

		orchestrator.syncTerminalStream('ws::term');
		await Promise.resolve();
		await Promise.resolve();

		expect(beginTerminal).toHaveBeenCalledTimes(1);
		expect(beginTerminal).toHaveBeenCalledWith('ws::term', true);
	});

	it('coalesces overlapping sync runs per terminal', async () => {
		let latestToken = 0;
		let initCalls = 0;
		let releaseFirstInit: () => void = () => undefined;
		const beginTerminal = vi.fn(async () => undefined);
		const emitState = vi.fn();
		const trace = vi.fn();
		const orchestrator = createTerminalStreamOrchestrator({
			initTerminal: async () => {
				initCalls += 1;
				if (initCalls !== 1) return;
				await new Promise<void>((resolve) => {
					releaseFirstInit = resolve;
				});
			},
			getContext: () => ({ container: document.createElement('div'), active: true }),
			beginTerminal,
			nextSyncToken: () => {
				latestToken += 1;
				return latestToken;
			},
			isCurrentSyncToken: (_id, token) => token === latestToken,
			emitState,
			trace,
		});

		orchestrator.syncTerminalStream('ws::term');
		orchestrator.syncTerminalStream('ws::term');
		releaseFirstInit();
		await Promise.resolve();
		await Promise.resolve();
		await Promise.resolve();
		await Promise.resolve();
		await new Promise((resolve) => {
			setTimeout(resolve, 0);
		});

		expect(initCalls).toBe(2);
		expect(beginTerminal).toHaveBeenCalledTimes(2);
		expect(emitState).toHaveBeenCalledTimes(2);
		expect(trace).toHaveBeenCalledWith(
			'ws::term',
			'stream_sync_coalesced',
			expect.objectContaining({ coalescedCount: 1 }),
		);
		expect(trace).toHaveBeenCalledWith(
			'ws::term',
			'stream_sync_done',
			expect.objectContaining({ token: 2 }),
		);
	});
});
