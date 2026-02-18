import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createTerminalAttachState } from './terminalAttachRendererState';

describe('terminalAttachRendererState', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('schedules terminal disposal on detach and cancels it on reattach', () => {
		const onDispose = vi.fn();
		const attachState = createTerminalAttachState({
			disposeAfterMs: 1000,
			onDispose,
			setTimeoutFn: (callback, timeoutMs) => setTimeout(callback, timeoutMs),
			clearTimeoutFn: (handle) => clearTimeout(handle),
		});

		attachState.markAttached('ws::term');
		attachState.markDetached('ws::term');

		vi.advanceTimersByTime(900);
		expect(onDispose).not.toHaveBeenCalled();

		attachState.markAttached('ws::term');
		vi.advanceTimersByTime(200);
		expect(onDispose).not.toHaveBeenCalled();

		attachState.markDetached('ws::term');
		vi.advanceTimersByTime(1000);
		expect(onDispose).toHaveBeenCalledTimes(1);
		expect(onDispose).toHaveBeenCalledWith('ws::term');
	});
});
