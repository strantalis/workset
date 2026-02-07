import { describe, expect, it, vi } from 'vitest';
import { createTerminalModeBootstrapCoordinator } from './terminalModeBootstrapCoordinator';

type TestKittyEvent = {
	kind: string;
	snapshot?: {
		images?: Array<{
			id: string;
		}>;
		placements?: Array<{
			id: number;
			imageId: string;
			row: number;
			col: number;
			rows: number;
			cols: number;
		}>;
	};
};

const createCoordinator = () => {
	const bootstrapHandled = new Map<string, boolean>();
	const pendingReplayOutput = new Map<string, Array<{ bytes: number }>>();
	const pendingReplayKitty = new Map<string, TestKittyEvent[]>();
	const initialCreditMap = new Map<string, number>();
	const contextByKey = new Map<string, { workspaceId: string; terminalId: string }>();
	contextByKey.set('ws::term', { workspaceId: 'ws', terminalId: 'term' });

	const callOrder: string[] = [];
	const deps = {
		buildTerminalKey: (workspaceId: string, terminalId: string) => `${workspaceId}::${terminalId}`,
		getContext: (key: string) => contextByKey.get(key) ?? null,
		logDebug: vi.fn(),
		markInput: vi.fn(() => {
			callOrder.push('markInput');
		}),
		bootstrapHandled,
		setReplayState: vi.fn(() => {
			callOrder.push('setReplayState');
		}),
		enqueueOutput: vi.fn(),
		countBytes: (value: string) => value.length,
		pendingReplayKitty,
		hasTerminalHandle: vi.fn(() => false),
		applyKittyEvent: vi.fn(async () => undefined),
		setHealth: vi.fn(),
		initialCreditMap,
		initialStreamCredit: 512,
		pendingReplayOutput,
		getStatus: vi.fn(() => 'starting'),
		setStatusAndMessage: vi.fn(() => {
			callOrder.push('setStatusAndMessage');
		}),
		scheduleBootstrapHealthCheck: vi.fn(() => {
			callOrder.push('scheduleBootstrapHealthCheck');
		}),
		emitState: vi.fn(() => {
			callOrder.push('emitState');
		}),
		applyLifecyclePayload: vi.fn(),
		setMode: vi.fn(() => {
			callOrder.push('setMode');
		}),
		syncTerminalWebLinks: vi.fn(() => {
			callOrder.push('syncTerminalWebLinks');
		}),
	};

	const coordinator = createTerminalModeBootstrapCoordinator<TestKittyEvent>(deps);
	return {
		coordinator,
		deps,
		callOrder,
		contextByKey,
		bootstrapHandled,
		pendingReplayOutput,
		pendingReplayKitty,
		initialCreditMap,
	};
};

describe('terminalModeBootstrapCoordinator', () => {
	it('detects workspace mismatches from active context', () => {
		const { coordinator, deps } = createCoordinator();

		expect(coordinator.isWorkspaceMismatch('ws::term', 'other', 'term')).toBe(true);
		expect(deps.logDebug).toHaveBeenCalledWith(
			'ws::term',
			'workspace_mismatch',
			expect.objectContaining({
				payloadWorkspaceId: 'other',
				payloadTerminalId: 'term',
				contextWorkspaceId: 'ws',
				contextTerminalId: 'term',
			}),
		);
		expect(coordinator.isWorkspaceMismatch('ws::term', 'ws', 'term')).toBe(false);
	});

	it('marks bootstrap handled and skips replay when safeToReplay is false', () => {
		const { coordinator, deps, bootstrapHandled } = createCoordinator();

		coordinator.handleBootstrapPayload({
			workspaceId: 'ws',
			terminalId: 'term',
			safeToReplay: false,
		});

		expect(deps.markInput).toHaveBeenCalledWith('ws::term');
		expect(deps.setReplayState).toHaveBeenCalledWith('ws::term', 'live');
		expect(bootstrapHandled.get('ws::term')).toBe(true);
		expect(deps.enqueueOutput).not.toHaveBeenCalled();
		expect(deps.applyKittyEvent).not.toHaveBeenCalled();
	});

	it('queues replay output and kitty snapshots before terminal handle is present', () => {
		const { coordinator, deps, bootstrapHandled, pendingReplayKitty, initialCreditMap } =
			createCoordinator();

		coordinator.handleBootstrapPayload({
			workspaceId: 'ws',
			terminalId: 'term',
			snapshot: 'snapshot',
			backlog: 'backlog',
			kitty: {
				images: [{ id: 'img-1' }],
				placements: [{ id: 1, imageId: 'img-1', row: 0, col: 0, rows: 1, cols: 1 }],
			},
			backlogTruncated: true,
		});

		expect(deps.setReplayState).toHaveBeenCalledWith('ws::term', 'replaying');
		expect(deps.enqueueOutput).toHaveBeenNthCalledWith(1, 'ws::term', 'snapshot', 8);
		expect(deps.enqueueOutput).toHaveBeenNthCalledWith(2, 'ws::term', 'backlog', 7);
		expect(pendingReplayKitty.get('ws::term')).toEqual([
			{
				kind: 'snapshot',
				snapshot: {
					images: [{ id: 'img-1' }],
					placements: [{ id: 1, imageId: 'img-1', row: 0, col: 0, rows: 1, cols: 1 }],
				},
			},
		]);
		expect(deps.setHealth).toHaveBeenCalledWith(
			'ws::term',
			'ok',
			'Backlog truncated; showing latest output.',
		);
		expect(initialCreditMap.get('ws::term')).toBe(512);
		expect(bootstrapHandled.get('ws::term')).toBe(true);
		expect(deps.logDebug).toHaveBeenCalledWith(
			'ws::term',
			'bootstrap',
			expect.objectContaining({ backlogTruncated: true }),
		);
	});

	it('keeps bootstrap-done ordering intact', () => {
		const { coordinator, deps, callOrder, pendingReplayOutput } = createCoordinator();
		pendingReplayOutput.set('ws::term', [{ bytes: 3 }, { bytes: 5 }]);

		coordinator.handleBootstrapDonePayload({
			workspaceId: 'ws',
			terminalId: 'term',
		});

		expect(deps.scheduleBootstrapHealthCheck).toHaveBeenCalledWith('ws::term', 8);
		expect(callOrder).toEqual([
			'markInput',
			'setStatusAndMessage',
			'scheduleBootstrapHealthCheck',
			'setReplayState',
			'emitState',
		]);
	});

	it('applies mode payload before syncing web links', () => {
		const { coordinator, deps, callOrder } = createCoordinator();

		coordinator.handleTerminalModesPayload('ws::term', {
			workspaceId: 'ws',
			terminalId: 'term',
			mouse: true,
		});

		expect(deps.setMode).toHaveBeenCalledWith('ws::term', {
			altScreen: false,
			mouse: true,
			mouseSGR: false,
			mouseEncoding: 'x10',
		});
		expect(callOrder).toEqual(['setMode', 'syncTerminalWebLinks']);
	});
});
