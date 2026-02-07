export type TerminalContext = {
	terminalKey: string;
	workspaceId: string;
	workspaceName: string;
	terminalId: string;
	container: HTMLDivElement | null;
	active: boolean;
	lastWorkspaceId: string;
};

export const createTerminalContextRegistry = () => {
	const terminalContexts = new Map<string, TerminalContext>();

	const buildTerminalKey = (workspaceId: string, terminalId: string): string => {
		const workspace = workspaceId?.trim();
		const terminal = terminalId?.trim();
		if (!workspace || !terminal) return '';
		return `${workspace}::${terminal}`;
	};

	const getContext = (key: string): TerminalContext | null => {
		return terminalContexts.get(key) ?? null;
	};

	const ensureContext = (input: TerminalContext): TerminalContext => {
		const existing = terminalContexts.get(input.terminalKey);
		if (!existing) {
			terminalContexts.set(input.terminalKey, input);
			return input;
		}
		const next = { ...existing, ...input, terminalKey: input.terminalKey };
		terminalContexts.set(input.terminalKey, next);
		return next;
	};

	const getWorkspaceId = (key: string): string => {
		return terminalContexts.get(key)?.workspaceId ?? '';
	};

	const getTerminalId = (key: string): string => {
		return terminalContexts.get(key)?.terminalId ?? '';
	};

	const getLastWorkspaceId = (key: string): string => {
		return terminalContexts.get(key)?.lastWorkspaceId ?? '';
	};

	const setLastWorkspaceId = (key: string, workspaceId: string): void => {
		const existing = terminalContexts.get(key);
		if (!existing) return;
		terminalContexts.set(key, { ...existing, lastWorkspaceId: workspaceId });
	};

	const deleteContext = (key: string): void => {
		terminalContexts.delete(key);
	};

	const keys = (): IterableIterator<string> => terminalContexts.keys();

	return {
		buildTerminalKey,
		getContext,
		ensureContext,
		getWorkspaceId,
		getTerminalId,
		getLastWorkspaceId,
		setLastWorkspaceId,
		deleteContext,
		keys,
	};
};
