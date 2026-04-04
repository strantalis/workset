import { describe, expect, it, vi } from 'vitest';
import { createTerminalDebugState } from './terminalDebugState';

describe('terminalDebugState', () => {
	it('enables lifecycle logging without enabling the visual overlay', () => {
		const state = createTerminalDebugState({
			emitAllStates: vi.fn(),
		});

		state.setLifecycleLogPreference('on');
		state.setDebugOverlayPreference('off');
		state.syncDebugEnabled();

		expect(state.isLifecycleLoggingEnabled()).toBe(true);
		expect(state.isDebugEnabled()).toBe(false);
	});
});
