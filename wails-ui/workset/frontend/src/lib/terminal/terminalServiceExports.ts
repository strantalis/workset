import type { Readable, Writable } from 'svelte/store';
import {
	createTerminalSyncController,
	type TerminalSyncControllerDependencies,
} from './terminalSyncController';
import type { TerminalSnapshotLike } from './terminalEmulatorContracts';

type TerminalServiceExportDependencies<TState> = {
	loadTerminalDefaults: () => Promise<void>;
	buildTerminalKey: (workspaceId: string, terminalId: string) => string;
	ensureStore: (id: string) => Writable<TState>;
	syncControllerDeps: TerminalSyncControllerDependencies;
	captureSnapshot: (id: string) => TerminalSnapshotLike | null;
};

export const createTerminalServiceExports = <TState>(
	deps: TerminalServiceExportDependencies<TState>,
) => {
	const syncController = createTerminalSyncController(deps.syncControllerDeps);

	const refreshTerminalDefaults = (): Promise<void> => deps.loadTerminalDefaults();

	const getTerminalStore = (workspaceId: string, terminalId: string): Readable<TState> => {
		const key = deps.buildTerminalKey(workspaceId, terminalId);
		if (!key) {
			return deps.ensureStore('');
		}
		return deps.ensureStore(key);
	};

	return {
		refreshTerminalDefaults,
		getTerminalStore,
		syncTerminal: syncController.syncTerminal,
		detachTerminal: syncController.detachTerminal,
		closeTerminal: syncController.closeTerminal,
		focusTerminalInstance: syncController.focusTerminalInstance,
		scrollTerminalToBottom: syncController.scrollTerminalToBottom,
		isTerminalAtBottom: syncController.isTerminalAtBottom,
		captureTerminalSnapshot: (
			workspaceId: string,
			terminalId: string,
		): TerminalSnapshotLike | null => {
			const key = deps.buildTerminalKey(workspaceId, terminalId);
			if (!key) {
				return null;
			}
			return deps.captureSnapshot(key);
		},
	};
};
