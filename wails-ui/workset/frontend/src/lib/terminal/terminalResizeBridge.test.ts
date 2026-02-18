import { describe, expect, it, vi } from 'vitest';
import { createTerminalResizeBridge, type TerminalResizeHandle } from './terminalResizeBridge';

const createHandle = (cols: number, rows: number) => {
	const handle: TerminalResizeHandle = {
		fitAddon: {
			proposeDimensions: vi.fn(() => ({ cols, rows })),
		},
	};
	return { handle };
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

	it('ignores transient tiny dimensions from hidden layout churn', () => {
		const resize = vi.fn().mockResolvedValue(undefined);
		const bridge = createTerminalResizeBridge({
			getWorkspaceId: () => 'ws',
			getTerminalId: () => 'term',
			resize,
		});
		const { handle } = createHandle(2, 1);

		bridge.resizeToFit('ws::term', handle);

		expect(resize).not.toHaveBeenCalled();
	});

	it('does not resend identical dimensions repeatedly', () => {
		const resize = vi.fn().mockResolvedValue(undefined);
		const bridge = createTerminalResizeBridge({
			getWorkspaceId: () => 'ws',
			getTerminalId: () => 'term',
			resize,
		});
		const { handle } = createHandle(133, 40);

		bridge.resizeToFit('ws::term', handle);
		bridge.resizeToFit('ws::term', handle);

		expect(resize).toHaveBeenCalledTimes(1);
	});

	it('allows same dimensions again after clear', () => {
		const resize = vi.fn().mockResolvedValue(undefined);
		const bridge = createTerminalResizeBridge({
			getWorkspaceId: () => 'ws',
			getTerminalId: () => 'term',
			resize,
		});
		const { handle } = createHandle(133, 40);

		bridge.resizeToFit('ws::term', handle);
		bridge.clear('ws::term');
		bridge.resizeToFit('ws::term', handle);

		expect(resize).toHaveBeenCalledTimes(2);
	});

	it('retries identical dimensions after a failed resize', async () => {
		const resize = vi
			.fn()
			.mockRejectedValueOnce(new Error('terminal not started'))
			.mockResolvedValue(undefined);
		const bridge = createTerminalResizeBridge({
			getWorkspaceId: () => 'ws',
			getTerminalId: () => 'term',
			resize,
		});
		const { handle } = createHandle(120, 40);

		bridge.resizeToFit('ws::term', handle);
		await Promise.resolve();
		bridge.resizeToFit('ws::term', handle);

		expect(resize).toHaveBeenCalledTimes(2);
	});
});
