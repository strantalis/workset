import { describe, expect, it, vi } from 'vitest';
import { createTerminalSnapshotManager } from './terminalSnapshotManager';
import type { TerminalLike, TerminalSnapshotLike } from './terminalEmulatorContracts';

const createSnapshot = (): TerminalSnapshotLike => ({
	version: 1,
	nextOffset: 22,
	cols: 80,
	rows: 24,
	activeBuffer: 'normal',
	normalViewportY: 0,
	cursor: {
		x: 0,
		y: 0,
		visible: true,
	},
	modes: {
		dec: [],
		ansi: [],
	},
	normalTail: ['hello'],
	normalScreen: ['hello'],
});

const createTerminal = () =>
	({
		cols: 80,
		rows: 24,
		buffer: {
			active: {
				baseY: 0,
				viewportY: 0,
			},
		},
		element: null,
		open: vi.fn(),
		focus: vi.fn(),
		scrollToBottom: vi.fn(),
		clear: vi.fn(),
		reset: vi.fn(),
		write: vi.fn(),
		dispose: vi.fn(),
		onData: vi.fn(() => ({ dispose: vi.fn() })),
		options: {
			fontSize: 12,
		},
		restoreState: vi.fn(async () => undefined),
		serializeState: vi.fn(),
	}) satisfies TerminalLike;

describe('terminalSnapshotManager', () => {
	it('restores snapshots even when attach resumes from a nonzero offset', async () => {
		const terminal = createTerminal();
		const terminalHandles = new Map([
			[
				'ws::term',
				{
					terminal,
					opened: true,
				},
			],
		]);
		const beforeRestore = vi.fn();
		const afterRestore = vi.fn();
		const manager = createTerminalSnapshotManager({
			terminalHandles,
			getOffset: () => 0,
			canPublish: () => true,
			publish: vi.fn(),
			beforeRestore,
			afterRestore,
		});

		const snapshot = createSnapshot();
		await manager.restore('ws::term', snapshot, { requestedOffset: 10 });

		expect(beforeRestore).toHaveBeenCalledWith('ws::term', snapshot);
		expect(terminal.restoreState).toHaveBeenCalledWith(snapshot);
		expect(afterRestore).toHaveBeenCalledWith('ws::term', snapshot);
	});

	it('forwards awaitAck when flushing snapshots', async () => {
		const terminal = createTerminal();
		const snapshot = createSnapshot();
		terminal.serializeState = vi.fn(() => snapshot);
		const publish = vi.fn(async () => undefined);
		const manager = createTerminalSnapshotManager({
			terminalHandles: new Map([
				[
					'ws::term',
					{
						terminal,
						opened: true,
					},
				],
			]),
			getOffset: () => 22,
			canPublish: () => true,
			publish,
		});

		await manager.flush('ws::term', 'workspace_popout', true);

		expect(publish).toHaveBeenCalledWith('ws::term', snapshot, true);
	});

	it('queues restore until the terminal is opened', async () => {
		const terminal = createTerminal();
		const handle = {
			terminal,
			opened: false,
		};
		const terminalHandles = new Map([['ws::term', handle]]);
		const manager = createTerminalSnapshotManager({
			terminalHandles,
			getOffset: () => 0,
			canPublish: () => true,
			publish: vi.fn(),
		});

		const snapshot = createSnapshot();
		await manager.restore('ws::term', snapshot, { requestedOffset: 10 });
		expect(terminal.restoreState).not.toHaveBeenCalled();

		handle.opened = true;
		manager.register('ws::term', handle);
		expect(terminal.restoreState).toHaveBeenCalledWith(snapshot);
	});

	it('skips snapshot publish when the terminal is not open', async () => {
		const terminal = createTerminal();
		terminal.serializeState = vi.fn(() => createSnapshot());
		const publish = vi.fn(async () => undefined);
		const manager = createTerminalSnapshotManager({
			terminalHandles: new Map([
				[
					'ws::term',
					{
						terminal,
						opened: false,
					},
				],
			]),
			getOffset: () => 22,
			canPublish: () => true,
			publish,
		});

		await manager.flush('ws::term', 'detach', true);

		expect(terminal.serializeState).not.toHaveBeenCalled();
		expect(publish).not.toHaveBeenCalled();
	});
});
