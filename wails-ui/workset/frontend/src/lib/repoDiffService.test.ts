import { beforeEach, describe, expect, it, vi } from 'vitest';
import { EVENT_REPO_DIFF_SUMMARY } from './events';

const eventsOn = vi.fn();
const eventsOff = vi.fn();

vi.mock('../../wailsjs/runtime/runtime', () => ({
	EventsOn: eventsOn,
	EventsOff: eventsOff,
}));

const loadService = async () => {
	const mod = await import('./repoDiffService');
	return mod;
};

beforeEach(() => {
	vi.resetModules();
	eventsOn.mockReset();
	eventsOff.mockReset();
});

describe('subscribeRepoDiffEvent', () => {
	it('uses the EventsOn unsubscribe callback for teardown', async () => {
		const unsubscribe = vi.fn();
		eventsOn.mockImplementation(() => unsubscribe);

		const { subscribeRepoDiffEvent } = await loadService();
		const stop = subscribeRepoDiffEvent(EVENT_REPO_DIFF_SUMMARY, () => {});
		expect(eventsOn).toHaveBeenCalledTimes(1);

		stop();
		expect(unsubscribe).toHaveBeenCalledTimes(1);
		expect(eventsOff).not.toHaveBeenCalled();
	});

	it('keeps the shared listener until the last handler unsubscribes', async () => {
		const unsubscribe = vi.fn();
		eventsOn.mockImplementation(() => unsubscribe);

		const { subscribeRepoDiffEvent } = await loadService();
		const stopA = subscribeRepoDiffEvent(EVENT_REPO_DIFF_SUMMARY, () => {});
		const stopB = subscribeRepoDiffEvent(EVENT_REPO_DIFF_SUMMARY, () => {});

		stopA();
		expect(unsubscribe).not.toHaveBeenCalled();

		stopB();
		expect(unsubscribe).toHaveBeenCalledTimes(1);
		expect(eventsOff).not.toHaveBeenCalled();
	});
});
