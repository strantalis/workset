import { describe, expect, it, vi } from 'vitest';
import { createTerminalResizeBridge, type TerminalResizeHandle } from './terminalResizeBridge';

const createHandle = (cols: number, rows: number) => {
	const write = vi.fn();
	const handle: TerminalResizeHandle = {
		terminal: {
			write,
		},
		fitAddon: {
			proposeDimensions: vi.fn(() => ({ cols, rows })),
		},
	};
	return { handle, write };
};

describe('terminalResizeBridge', () => {
	it('resizes to fit current dimensions', () => {
		const resize = vi.fn().mockResolvedValue(undefined);
		const bridge = createTerminalResizeBridge({
			getWorkspaceId: () => 'ws',
			getTerminalId: () => 'term',
			resize,
		});
		const { handle } = createHandle(120, 40);

		bridge.resizeToFit('ws::term', handle);

		expect(resize).toHaveBeenCalledTimes(1);
		expect(resize).toHaveBeenCalledWith('ws', 'term', 120, 40);
	});

	it('nudges redraw once while pending and restores dimensions after delay', () => {
		vi.useFakeTimers();
		const resize = vi.fn().mockResolvedValue(undefined);
		const logDebug = vi.fn();
		const bridge = createTerminalResizeBridge({
			getWorkspaceId: () => 'ws',
			getTerminalId: () => 'term',
			resize,
			logDebug,
		});
		const { handle } = createHandle(1, 0);

		bridge.nudgeRedraw('ws::term', handle);
		bridge.nudgeRedraw('ws::term', handle);
		expect(resize).toHaveBeenCalledTimes(1);
		expect(resize).toHaveBeenCalledWith('ws', 'term', 3, 1);
		expect(logDebug).toHaveBeenCalledWith('ws::term', 'redraw_nudge', {
			cols: 2,
			rows: 1,
			nudgeCols: 3,
		});

		vi.advanceTimersByTime(60);
		expect(resize).toHaveBeenCalledTimes(2);
		expect(resize).toHaveBeenLastCalledWith('ws', 'term', 2, 1);

		bridge.nudgeRedraw('ws::term', handle);
		expect(resize).toHaveBeenCalledTimes(3);
		expect(resize).toHaveBeenLastCalledWith('ws', 'term', 3, 1);
		vi.useRealTimers();
	});

	it('falls back to local write when transport ids are unavailable', () => {
		const resize = vi.fn().mockResolvedValue(undefined);
		const bridge = createTerminalResizeBridge({
			getWorkspaceId: () => '',
			getTerminalId: () => 'term',
			resize,
		});
		const { handle, write } = createHandle(80, 24);

		bridge.nudgeRedraw('ws::term', handle);

		expect(resize).not.toHaveBeenCalled();
		expect(write).toHaveBeenCalledTimes(1);
		expect(write).toHaveBeenCalledWith('');
	});
});
