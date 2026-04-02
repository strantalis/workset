import { closeWorkspacePopout, listWorkspacePopouts, openWorkspacePopout } from '../api/workspaces';
import type { Workspace } from '../types';

export type PopoutManager = {
	/** Map of workspace ID → window name for open popouts. */
	readonly openPopouts: Record<string, string>;
	/** Whether a popout operation is in progress. */
	readonly busy: boolean;
	/** Check if a workspace (or any thread in its workset) is popped out. */
	isWorkspacePoppedOut: (workspaceId: string | null | undefined) => boolean;
	/** Update popout state for a workspace. */
	updateState: (workspaceId: string, windowName: string, open: boolean) => void;
	/** Load initial popout state from backend. */
	loadState: () => Promise<void>;
	/** Open or close a workspace popout. */
	handlePopout: (workspaceId: string, open: boolean) => Promise<void>;
	/** Resolve which workspace ID should be used for a popout (handles workset threads). */
	resolvePopoutWorkspaceId: (workspaceId: string) => string;
};

type PopoutManagerOptions = {
	/** Whether this app instance is itself running in a popout window. */
	popoutMode: boolean;
	/** Get all current workspaces for thread resolution. */
	getWorksetThreads: (workspaceId: string) => Workspace[];
};

export function createPopoutManager(options: PopoutManagerOptions): PopoutManager {
	const { getWorksetThreads } = options;

	let openPopouts = $state<Record<string, string>>({});
	let busy = $state(false);

	const resolvePopoutWorkspaceId = (workspaceId: string): string => {
		const id = workspaceId.trim();
		if (!id) return '';
		const threads = getWorksetThreads(id);
		if (threads.length === 0) return id;
		for (const thread of threads) {
			if (openPopouts[thread.id] !== undefined) {
				return thread.id;
			}
		}
		return threads[0]?.id ?? id;
	};

	const isWorkspacePoppedOut = (workspaceId: string | null | undefined): boolean => {
		if (!workspaceId) return false;
		const popoutWorkspaceId = resolvePopoutWorkspaceId(workspaceId);
		if (!popoutWorkspaceId) return false;
		return openPopouts[popoutWorkspaceId] !== undefined;
	};

	const updateState = (workspaceId: string, windowName: string, open: boolean): void => {
		const id = workspaceId.trim();
		if (!id) return;
		if (open) {
			openPopouts = { ...openPopouts, [id]: windowName };
			return;
		}
		if (openPopouts[id] === undefined) return;
		const next = { ...openPopouts };
		delete next[id];
		openPopouts = next;
	};

	const loadState = async (): Promise<void> => {
		try {
			const states = await listWorkspacePopouts();
			const next: Record<string, string> = {};
			for (const state of states) {
				if (!state.open || !state.workspaceId) continue;
				next[state.workspaceId] = state.windowName;
			}
			openPopouts = next;
		} catch {
			// ignore state probe failures
		}
	};

	const handlePopout = async (workspaceId: string, open: boolean): Promise<void> => {
		if (!workspaceId || busy) return;
		const popoutWorkspaceId = resolvePopoutWorkspaceId(workspaceId);
		if (!popoutWorkspaceId) return;
		busy = true;
		try {
			if (open) {
				const state = await openWorkspacePopout(popoutWorkspaceId);
				updateState(state.workspaceId, state.windowName, state.open);
			} else {
				await closeWorkspacePopout(popoutWorkspaceId);
				updateState(popoutWorkspaceId, '', false);
			}
		} catch {
			// ignore popout action errors in UI
		} finally {
			busy = false;
		}
	};

	return {
		get openPopouts() {
			return openPopouts;
		},
		get busy() {
			return busy;
		},
		isWorkspacePoppedOut,
		updateState,
		loadState,
		handlePopout,
		resolvePopoutWorkspaceId,
	};
}
