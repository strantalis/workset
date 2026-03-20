import { getContext, setContext } from 'svelte';

export type WorkspaceActionCallbacks = {
	createWorkspace: () => void;
	createThread: (worksetId: string) => void;
	addRepo: (worksetId: string) => void;
	removeThread: (threadId: string) => void;
};

export const WORKSPACE_ACTIONS_KEY = Symbol('workspaceActions');

export function provideWorkspaceActions(actions: WorkspaceActionCallbacks): void {
	setContext(WORKSPACE_ACTIONS_KEY, actions);
}

export function useWorkspaceActions(): WorkspaceActionCallbacks {
	return getContext<WorkspaceActionCallbacks>(WORKSPACE_ACTIONS_KEY);
}
