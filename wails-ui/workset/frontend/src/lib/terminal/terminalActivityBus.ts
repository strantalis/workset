export type TerminalActivityEvent = {
	workspaceId: string;
};

type TerminalActivityHandler = (event: TerminalActivityEvent) => void;

const handlers = new Set<TerminalActivityHandler>();

export const emitTerminalActivity = (workspaceId: string | null | undefined): void => {
	const id = workspaceId?.trim();
	if (!id) return;
	for (const handler of handlers) {
		handler({ workspaceId: id });
	}
};

export const subscribeTerminalActivity = (handler: TerminalActivityHandler): (() => void) => {
	handlers.add(handler);
	return () => {
		handlers.delete(handler);
	};
};
