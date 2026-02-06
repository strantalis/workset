import { beforeEach, describe, expect, it, vi } from 'vitest';

const eventsOn = vi.fn();
const eventsOff = vi.fn();

vi.mock('../../wailsjs/runtime/runtime', () => ({
	EventsOn: eventsOn,
	EventsOff: eventsOff,
}));

const loadService = async () => {
	const mod = await import('./githubOperationService');
	return mod;
};

beforeEach(() => {
	vi.resetModules();
	eventsOn.mockReset();
	eventsOff.mockReset();
});

describe('subscribeGitHubOperationEvent', () => {
	it('uses the EventsOn unsubscribe callback for teardown', async () => {
		const unsubscribe = vi.fn();
		eventsOn.mockImplementation(() => unsubscribe);

		const { subscribeGitHubOperationEvent } = await loadService();
		const stop = subscribeGitHubOperationEvent(() => {});
		expect(eventsOn).toHaveBeenCalledTimes(1);

		stop();
		expect(unsubscribe).toHaveBeenCalledTimes(1);
		expect(eventsOff).not.toHaveBeenCalled();
	});

	it('keeps the shared listener until the last handler unsubscribes', async () => {
		const unsubscribe = vi.fn();
		eventsOn.mockImplementation(() => unsubscribe);

		const { subscribeGitHubOperationEvent } = await loadService();
		const stopA = subscribeGitHubOperationEvent(() => {});
		const stopB = subscribeGitHubOperationEvent(() => {});

		stopA();
		expect(unsubscribe).not.toHaveBeenCalled();

		stopB();
		expect(unsubscribe).toHaveBeenCalledTimes(1);
		expect(eventsOff).not.toHaveBeenCalled();
	});
});
