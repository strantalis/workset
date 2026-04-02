import type {
	TerminalDisposable,
	TerminalLike,
	TerminalSnapshotLike,
} from './terminalEmulatorContracts';

type SnapshotHandle = {
	terminal: TerminalLike;
	opened?: boolean;
};

type TerminalSnapshotManagerDeps = {
	terminalHandles: Map<string, SnapshotHandle>;
	getOffset: (id: string) => number;
	logDebug?: (id: string, event: string, details: Record<string, unknown>) => void;
	beforeRestore?: (id: string, snapshot: TerminalSnapshotLike) => void;
	afterRestore?: (id: string, snapshot: TerminalSnapshotLike) => void;
};

const DEFAULT_NORMAL_TAIL_ROWS = 200;

export const createTerminalSnapshotManager = (deps: TerminalSnapshotManagerDeps) => {
	const disposables = new Map<string, TerminalDisposable[]>();
	const pendingSnapshots = new Map<string, { snapshot: TerminalSnapshotLike }>();
	const restoring = new Set<string>();

	const restoreSnapshot = async (id: string, snapshot: TerminalSnapshotLike): Promise<void> => {
		const handle = deps.terminalHandles.get(id);
		if (!handle?.terminal.restoreState || handle.opened !== true) {
			pendingSnapshots.set(id, { snapshot });
			return;
		}
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

	const captureSnapshot = (id: string, reason: string): TerminalSnapshotLike | null => {
		const handle = deps.terminalHandles.get(id);
		if (!handle || handle.opened !== true) {
			return null;
		}
		try {
			return (
				handle.terminal.serializeState?.({
					nextOffset: deps.getOffset(id),
					normalTailRows: DEFAULT_NORMAL_TAIL_ROWS,
				}) ?? null
			);
		} catch (error) {
			deps.logDebug?.(id, 'terminal_snapshot_publish_skip', {
				reason,
				error: String(error),
			});
			return null;
		}
	};

	const register = (id: string, _handle: SnapshotHandle): void => {
		disposables.set(id, []);
		const pending = pendingSnapshots.get(id);
		if (pending) {
			pendingSnapshots.delete(id);
			void restoreSnapshot(id, pending.snapshot);
		}
	};

	const clear = (id: string): void => {
		pendingSnapshots.delete(id);
		restoring.delete(id);
		const listeners = disposables.get(id) ?? [];
		for (const listener of listeners) {
			listener.dispose();
		}
		disposables.delete(id);
	};

	return {
		register,
		restore: restoreSnapshot,
		capture: (id: string): TerminalSnapshotLike | null => captureSnapshot(id, 'capture'),
		clear,
	};
};
