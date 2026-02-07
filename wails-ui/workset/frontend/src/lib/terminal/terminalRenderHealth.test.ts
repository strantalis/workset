import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createTerminalRenderHealth } from './terminalRenderHealth';

const createHandle = () => {
	let visibleText = '';
	const terminal = {
		open: vi.fn(),
		refresh: vi.fn(),
		rows: 24,
		buffer: {
			active: {
				length: 1,
				getLine: () => ({
					translateToString: () => visibleText,
				}),
			},
		},
	};
	return {
		handle: {
			terminal,
			container: document.createElement('div'),
		},
		setVisibleText: (value: string) => {
			visibleText = value;
		},
	};
};

describe('terminalRenderHealth', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-01-01T00:00:00Z'));
	});

	it('detects stalled output and performs one reopen attempt', () => {
		const { handle } = createHandle();
		const reopenWithPreservedViewport = vi.fn();
		const nudgeRedraw = vi.fn();
		const logDebug = vi.fn();
		const renderHealth = createTerminalRenderHealth({
			getHandle: () => handle,
			reopenWithPreservedViewport,
			fitWithPreservedViewport: () => undefined,
			nudgeRedraw,
			logDebug,
		});

		renderHealth.noteOutputActivity('ws::term');
		vi.advanceTimersByTime(350);

		expect(reopenWithPreservedViewport).toHaveBeenCalledTimes(1);
		expect(handle.terminal.refresh).toHaveBeenCalledTimes(1);
		expect(logDebug).toHaveBeenCalledWith('ws::term', 'render_stall', { lastRenderAt: 0 });

		vi.advanceTimersByTime(150);
		expect(nudgeRedraw).toHaveBeenCalledTimes(1);

		renderHealth.noteOutputActivity('ws::term');
		vi.advanceTimersByTime(350);
		expect(reopenWithPreservedViewport).toHaveBeenCalledTimes(1);
		expect(logDebug.mock.calls.filter((call) => call[1] === 'render_stall')).toHaveLength(1);
	});

	it('runs bootstrap render health recovery when nothing renders', () => {
		const { handle } = createHandle();
		const fitWithPreservedViewport = vi.fn();
		const nudgeRedraw = vi.fn();
		const logDebug = vi.fn();
		const renderHealth = createTerminalRenderHealth({
			getHandle: () => handle,
			reopenWithPreservedViewport: () => undefined,
			fitWithPreservedViewport,
			nudgeRedraw,
			logDebug,
		});

		renderHealth.scheduleBootstrapHealthCheck('ws::term', 1024);
		vi.advanceTimersByTime(350);
		expect(fitWithPreservedViewport).toHaveBeenCalledTimes(1);

		vi.advanceTimersByTime(150);
		expect(nudgeRedraw).toHaveBeenCalledTimes(1);
		expect(logDebug).toHaveBeenCalledWith('ws::term', 'render_health_check', {
			rendered: false,
		});
	});

	it('skips bootstrap recovery when output rendered after bootstrap began', () => {
		const { handle } = createHandle();
		const fitWithPreservedViewport = vi.fn();
		const renderHealth = createTerminalRenderHealth({
			getHandle: () => handle,
			reopenWithPreservedViewport: () => undefined,
			fitWithPreservedViewport,
			nudgeRedraw: () => undefined,
			logDebug: () => undefined,
		});

		renderHealth.scheduleBootstrapHealthCheck('ws::term', 2048);
		vi.advanceTimersByTime(100);
		renderHealth.noteRender('ws::term');
		vi.advanceTimersByTime(250);

		expect(fitWithPreservedViewport).not.toHaveBeenCalled();
	});

	it('clears pending checks and allows release to reset stall guards', () => {
		const { handle } = createHandle();
		const reopenWithPreservedViewport = vi.fn();
		const logDebug = vi.fn();
		const renderHealth = createTerminalRenderHealth({
			getHandle: () => handle,
			reopenWithPreservedViewport,
			fitWithPreservedViewport: () => undefined,
			nudgeRedraw: () => undefined,
			logDebug,
		});

		renderHealth.noteOutputActivity('ws::term');
		renderHealth.clearSession('ws::term');
		vi.advanceTimersByTime(350);
		expect(reopenWithPreservedViewport).not.toHaveBeenCalled();

		renderHealth.noteOutputActivity('ws::term');
		vi.advanceTimersByTime(350);
		expect(reopenWithPreservedViewport).toHaveBeenCalledTimes(1);

		renderHealth.release('ws::term');
		renderHealth.noteOutputActivity('ws::term');
		vi.advanceTimersByTime(350);
		expect(reopenWithPreservedViewport).toHaveBeenCalledTimes(2);
		expect(logDebug.mock.calls.filter((call) => call[1] === 'render_stall')).toHaveLength(2);
	});
});
