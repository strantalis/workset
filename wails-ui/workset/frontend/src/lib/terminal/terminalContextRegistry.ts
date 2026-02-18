export type TerminalContext = {
	terminalKey: string;
	workspaceId: string;
	terminalId: string;
	container: HTMLDivElement | null;
	active: boolean;
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
		deleteContext,
		keys,
	};
};
