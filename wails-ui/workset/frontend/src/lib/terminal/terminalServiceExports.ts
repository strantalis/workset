import type { Readable, Writable } from 'svelte/store';
import {
	createTerminalSyncController,
	type TerminalSyncControllerDependencies,
} from './terminalSyncController';

type TerminalServiceExportDependencies<TState, THandle> = {
	loadTerminalDefaults: () => Promise<void>;
	buildTerminalKey: (workspaceId: string, terminalId: string) => string;
	ensureStore: (id: string) => Writable<TState>;
	syncControllerDeps: TerminalSyncControllerDependencies<THandle>;
};

export const createTerminalServiceExports = <TState, THandle>(
	deps: TerminalServiceExportDependencies<TState, THandle>,
) => {
	const syncController = createTerminalSyncController<THandle>(deps.syncControllerDeps);

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
		restartTerminal: syncController.restartTerminal,
		retryHealthCheck: syncController.retryHealthCheck,
		focusTerminalInstance: syncController.focusTerminalInstance,
		scrollTerminalToBottom: syncController.scrollTerminalToBottom,
		isTerminalAtBottom: syncController.isTerminalAtBottom,
	};
};
