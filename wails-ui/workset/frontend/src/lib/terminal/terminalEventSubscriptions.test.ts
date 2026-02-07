import { describe, expect, it, vi } from 'vitest';
import {
	EVENT_SESSIOND_RESTARTED,
	EVENT_TERMINAL_BOOTSTRAP,
	EVENT_TERMINAL_BOOTSTRAP_DONE,
	EVENT_TERMINAL_DATA,
	EVENT_TERMINAL_KITTY,
	EVENT_TERMINAL_LIFECYCLE,
	EVENT_TERMINAL_MODES,
} from '../events';
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
		const onTerminalBootstrap = vi.fn();
		const onTerminalBootstrapDone = vi.fn();
		const onTerminalLifecycle = vi.fn();
		const onTerminalModes = vi.fn();
		const onTerminalKitty = vi.fn();
		const onSessiondRestarted = vi.fn();
		const subscriptions = createTerminalEventSubscriptions({
			subscribeEvent,
			buildTerminalKey: (workspaceId, terminalId) => `${workspaceId}::${terminalId}`,
			isWorkspaceMismatch: (_key, workspaceId) => workspaceId === 'mismatch',
			onTerminalData,
			onTerminalBootstrap,
			onTerminalBootstrapDone,
			onTerminalLifecycle,
			onTerminalModes,
			onTerminalKitty,
			onSessiondRestarted,
		});

		subscriptions.ensureListeners();
		subscriptions.ensureListeners();

		expect(subscribedEvents).toHaveLength(7);
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
		handlers.get(EVENT_TERMINAL_BOOTSTRAP)?.({
			workspaceId: 'ws',
			terminalId: 'term',
			source: 'event',
		});
		handlers.get(EVENT_TERMINAL_BOOTSTRAP_DONE)?.({ workspaceId: 'ws', terminalId: 'term' });
		handlers.get(EVENT_TERMINAL_LIFECYCLE)?.({
			workspaceId: 'ws',
			terminalId: 'term',
			status: 'started',
		});
		handlers.get(EVENT_TERMINAL_MODES)?.({ workspaceId: 'ws', terminalId: 'term', mouse: true });
		handlers.get(EVENT_TERMINAL_KITTY)?.({
			workspaceId: 'ws',
			terminalId: 'term',
			event: { kind: 'snapshot' },
		});
		handlers.get(EVENT_SESSIOND_RESTARTED)?.();

		expect(onTerminalData).toHaveBeenCalledTimes(1);
		expect(onTerminalData).toHaveBeenCalledWith(
			'ws::term',
			expect.objectContaining({ workspaceId: 'ws', terminalId: 'term' }),
		);
		expect(onTerminalBootstrap).toHaveBeenCalledTimes(1);
		expect(onTerminalBootstrapDone).toHaveBeenCalledTimes(1);
		expect(onTerminalLifecycle).toHaveBeenCalledWith(
			'ws::term',
			expect.objectContaining({ status: 'started' }),
		);
		expect(onTerminalModes).toHaveBeenCalledWith(
			'ws::term',
			expect.objectContaining({ mouse: true }),
		);
		expect(onTerminalKitty).toHaveBeenCalledWith(
			'ws::term',
			expect.objectContaining({ event: { kind: 'snapshot' } }),
		);
		expect(onSessiondRestarted).toHaveBeenCalledTimes(1);
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
			onTerminalBootstrap: () => undefined,
			onTerminalBootstrapDone: () => undefined,
			onTerminalLifecycle: () => undefined,
			onTerminalModes: () => undefined,
			onTerminalKitty: () => undefined,
			onSessiondRestarted: () => undefined,
		});

		subscriptions.ensureListeners();
		expect(subscribedEvents).toHaveLength(7);

		subscriptions.cleanupListeners();
		expect(unsubscribeFns).toHaveLength(7);
		for (const unsubscribe of unsubscribeFns) {
			expect(unsubscribe).toHaveBeenCalledTimes(1);
		}

		subscribedEvents.length = 0;
		subscriptions.ensureListeners();
		expect(subscribedEvents).toHaveLength(7);
	});
});
