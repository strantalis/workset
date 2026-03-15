import type {
	TerminalDisposable,
	TerminalLike,
	TerminalSnapshotLike,
} from './terminalEmulatorContracts';

type SnapshotReadyMeta = {
	requestedOffset?: number;
};

type SnapshotHandle = {
	terminal: TerminalLike;
	opened?: boolean;
};

type TerminalSnapshotManagerDeps = {
	terminalHandles: Map<string, SnapshotHandle>;
	getOffset: (id: string) => number;
	canPublish: (id: string) => boolean;
	publish: (id: string, snapshot: TerminalSnapshotLike, awaitAck?: boolean) => Promise<void> | void;
	logDebug?: (id: string, event: string, details: Record<string, unknown>) => void;
	beforeRestore?: (id: string, snapshot: TerminalSnapshotLike) => void;
	afterRestore?: (id: string, snapshot: TerminalSnapshotLike) => void;
};

const OUTPUT_SETTLE_MS = 100;
const DEFAULT_NORMAL_TAIL_ROWS = 200;

export const createTerminalSnapshotManager = (deps: TerminalSnapshotManagerDeps) => {
	const disposables = new Map<string, TerminalDisposable[]>();
	const publishTimers = new Map<string, ReturnType<typeof setTimeout>>();
	const pendingSnapshots = new Map<
		string,
		{ snapshot: TerminalSnapshotLike; ready: SnapshotReadyMeta }
	>();
	const restoring = new Set<string>();

	const clearTimer = (id: string): void => {
		const timer = publishTimers.get(id);
		if (!timer) return;
		clearTimeout(timer);
		publishTimers.delete(id);
	};

	const publishSnapshot = async (id: string, reason: string, awaitAck = false): Promise<void> => {
		clearTimer(id);
		if (restoring.has(id) || !deps.canPublish(id)) {
			return;
		}
		const handle = deps.terminalHandles.get(id);
		if (!handle || handle.opened !== true) {
			return;
		}
		let snapshot: TerminalSnapshotLike | undefined;
		try {
			snapshot = handle.terminal.serializeState?.({
				nextOffset: deps.getOffset(id),
				normalTailRows: DEFAULT_NORMAL_TAIL_ROWS,
			});
		} catch (error) {
			deps.logDebug?.(id, 'terminal_snapshot_publish_skip', {
				reason,
				error: String(error),
			});
			return;
		}
		if (!snapshot) {
			return;
		}
		try {
			await deps.publish(id, snapshot, awaitAck);
		} catch (error) {
			deps.logDebug?.(id, 'terminal_snapshot_publish_error', {
				reason,
				error: String(error),
			});
			return;
		}
		deps.logDebug?.(id, 'terminal_snapshot_publish', {
			reason,
			nextOffset: snapshot.nextOffset,
			activeBuffer: snapshot.activeBuffer,
			rows: snapshot.rows,
			cols: snapshot.cols,
			normalTail: snapshot.normalTail.length,
			normalScreen: snapshot.normalScreen?.length ?? 0,
		});
	};

	const scheduleSnapshot = (id: string, reason: string): void => {
		if (restoring.has(id)) {
			return;
		}
		clearTimer(id);
		publishTimers.set(
			id,
			setTimeout(() => {
				publishTimers.delete(id);
				void publishSnapshot(id, reason);
			}, OUTPUT_SETTLE_MS),
		);
	};

	const restoreSnapshot = async (
		id: string,
		snapshot: TerminalSnapshotLike,
		ready: SnapshotReadyMeta = {},
	): Promise<void> => {
		const handle = deps.terminalHandles.get(id);
		if (!handle?.terminal.restoreState || handle.opened !== true) {
			pendingSnapshots.set(id, { snapshot, ready });
			return;
		}
		clearTimer(id);
		restoring.add(id);
		try {
			deps.beforeRestore?.(id, snapshot);
			await handle.terminal.restoreState(snapshot);
			deps.afterRestore?.(id, snapshot);
			deps.logDebug?.(id, 'terminal_snapshot_restore_ok', {
				nextOffset: snapshot.nextOffset,
				activeBuffer: snapshot.activeBuffer,
			});
		} finally {
			restoring.delete(id);
		}
	};

	const register = (id: string, handle: SnapshotHandle): void => {
		if (!disposables.has(id)) {
			const listeners: TerminalDisposable[] = [];
			if (handle.terminal.onScroll) {
				listeners.push(
					handle.terminal.onScroll(() => {
						scheduleSnapshot(id, 'scroll');
					}),
				);
			}
			if (handle.terminal.onResize) {
				listeners.push(
					handle.terminal.onResize(() => {
						void publishSnapshot(id, 'resize');
					}),
				);
			}
			disposables.set(id, listeners);
		}
		const pending = pendingSnapshots.get(id);
		if (pending) {
			pendingSnapshots.delete(id);
			void restoreSnapshot(id, pending.snapshot, pending.ready);
		}
	};

	const clear = (id: string): void => {
		clearTimer(id);
		pendingSnapshots.delete(id);
		restoring.delete(id);
		const listeners = disposables.get(id) ?? [];
		for (const listener of listeners) {
			listener.dispose();
		}
		disposables.delete(id);
	};

	const flushAll = (reason: string): void => {
		for (const id of deps.terminalHandles.keys()) {
			void publishSnapshot(id, reason);
		}
	};

	return {
		register,
		scheduleFromOutput: (id: string): void => {
			scheduleSnapshot(id, 'output');
		},
		restore: restoreSnapshot,
		flush: (id: string, reason: string, awaitAck = false): Promise<void> =>
			publishSnapshot(id, reason, awaitAck),
		flushAll,
		clear,
	};
};
