export type MockTerminalControllerEntry = {
	workspaceId: string;
	terminalId: string;
};

export const mockTerminalControllerTracker = {
	mounts: [] as MockTerminalControllerEntry[],
	destroys: [] as MockTerminalControllerEntry[],
};

export const resetMockTerminalControllerTracker = (): void => {
	mockTerminalControllerTracker.mounts = [];
	mockTerminalControllerTracker.destroys = [];
};

export const recordMockTerminalControllerMount = (entry: MockTerminalControllerEntry): void => {
	mockTerminalControllerTracker.mounts = [...mockTerminalControllerTracker.mounts, entry];
};

export const recordMockTerminalControllerDestroy = (entry: MockTerminalControllerEntry): void => {
	mockTerminalControllerTracker.destroys = [...mockTerminalControllerTracker.destroys, entry];
};
