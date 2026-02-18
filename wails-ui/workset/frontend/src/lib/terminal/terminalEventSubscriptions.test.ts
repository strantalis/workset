import { describe, expect, it, vi } from 'vitest';
import { EVENT_TERMINAL_DATA } from '../events';
import { createTerminalEventSubscriptions } from './terminalEventSubscriptions';

describe('terminalEventSubscriptions', () => {
	it('registers listeners once and routes payloads by terminal key', () => {
		const handlers = new Map<string, (payload?: unknown) => void>();
		const subscribedEvents: string[] = [];
		const subscribeEvent = <T>(event: string, handler: (payload: T) => void): (() => void) => {
			subscribedEvents.push(event);
			handlers.set(event, handler as (payload?: unknown) => void);
			return vi.fn();
		};
		const onTerminalData = vi.fn();
		const subscriptions = createTerminalEventSubscriptions({
			subscribeEvent,
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			isWorkspaceMismatch: (_key, workspaceId) => workspaceId === 'mismatch',
			onTerminalData,
		});

		subscriptions.ensureListeners();
		subscriptions.ensureListeners();

		expect(subscribedEvents).toHaveLength(1);
		handlers.get(EVENT_TERMINAL_DATA)?.({
			workspaceId: 'ws',
			terminalId: 'term',
			data: 'echo hello',
		});
		handlers.get(EVENT_TERMINAL_DATA)?.({
			workspaceId: 'mismatch',
			terminalId: 'term',
			data: 'ignored',
		});
		expect(onTerminalData).toHaveBeenCalledTimes(1);
		expect(onTerminalData).toHaveBeenCalledWith(
			'ws::term',
			expect.objectContaining({ workspaceId: 'ws', terminalId: 'term' }),
		);
	});

	it('cleans up listeners and allows re-registering', () => {
		const unsubscribeFns: Array<() => void> = [];
		const subscribedEvents: string[] = [];
		const subscribeEvent = <T>(_event: string, _handler: (payload: T) => void): (() => void) => {
			subscribedEvents.push(_event);
			const unsubscribe = vi.fn();
			unsubscribeFns.push(unsubscribe);
			return unsubscribe;
		};
		const subscriptions = createTerminalEventSubscriptions({
			subscribeEvent,
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			isWorkspaceMismatch: () => false,
			onTerminalData: () => undefined,
		});

		subscriptions.ensureListeners();
		expect(subscribedEvents).toHaveLength(1);

		subscriptions.cleanupListeners();
		expect(unsubscribeFns).toHaveLength(1);
		for (const unsubscribe of unsubscribeFns) {
			expect(unsubscribe).toHaveBeenCalledTimes(1);
		}

		subscribedEvents.length = 0;
		subscriptions.ensureListeners();
		expect(subscribedEvents).toHaveLength(1);
	});
});
