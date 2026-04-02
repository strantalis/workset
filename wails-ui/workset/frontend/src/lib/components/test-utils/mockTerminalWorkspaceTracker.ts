export type TerminalWorkspaceLifecycleEntry = {
	workspaceId: string;
	workspaceName: string;
};

export const terminalWorkspaceLifecycle = {
	mounts: [] as TerminalWorkspaceLifecycleEntry[],
	destroys: [] as TerminalWorkspaceLifecycleEntry[],
};

export const resetTerminalWorkspaceLifecycle = (): void => {
	terminalWorkspaceLifecycle.mounts.length = 0;
	terminalWorkspaceLifecycle.destroys.length = 0;
};

export const recordTerminalWorkspaceMount = (entry: TerminalWorkspaceLifecycleEntry): void => {
	terminalWorkspaceLifecycle.mounts.push(entry);
};

export const recordTerminalWorkspaceDestroy = (entry: TerminalWorkspaceLifecycleEntry): void => {
	terminalWorkspaceLifecycle.destroys.push(entry);
};
