type TerminalSyncInput = {
	workspaceId: string;
	workspaceName: string;
	terminalId: string;
	container: HTMLDivElement | null;
	active: boolean;
};

type TerminalSyncContext = {
	terminalKey: string;
	workspaceId: string;
	workspaceName: string;
	terminalId: string;
	container: HTMLDivElement | null;
	active: boolean;
	lastWorkspaceId: string;
};

export type TerminalSyncControllerDependencies<THandle> = {
	ensureGlobals: () => void;
	buildTerminalKey: (workspaceId: string, terminalId: string) => string;
	ensureContext: (input: TerminalSyncContext) => TerminalSyncContext;
	getLastWorkspaceId: (key: string) => string;
	setLastWorkspaceId: (key: string, workspaceId: string) => void;
	deleteContext: (key: string) => void;
	attachTerminal: (id: string, container: HTMLDivElement | null, active: boolean) => unknown;
	attachResizeObserver: (id: string, container: HTMLDivElement) => void;
	detachResizeObserver: (id: string) => void;
	scheduleFitStabilization: (id: string, reason: string) => void;
	scheduleReattachCheck: (id: string, reason: string) => void;
	syncTerminalStream: (id: string) => void;
	fitTerminal: (id: string, started: boolean) => void;
	hasStarted: (id: string) => boolean;
	forceRedraw: (id: string) => void;
	getHandle: (id: string) => THandle | undefined;
	hasVisibleTerminalContent: (handle: THandle) => boolean;
	nudgeRedraw: (id: string, handle: THandle) => void;
	markDetached: (id: string) => void;
	stopTerminal: (workspaceId: string, terminalId: string) => Promise<void>;
	disposeTerminalResources: (id: string) => void;
	beginTerminal: (id: string, quiet?: boolean) => Promise<void>;
	requestHealthCheck: (id: string) => void;
	focusTerminal: (id: string) => void;
	scrollToBottom: (id: string) => void;
	isAtBottom: (id: string) => boolean;
};

export const createTerminalSyncController = <THandle>(
	deps: TerminalSyncControllerDependencies<THandle>,
) => {
	const syncTerminal = (input: TerminalSyncInput): void => {
		if (!input.terminalId || !input.workspaceId) return;
		deps.ensureGlobals();
		const terminalKey = deps.buildTerminalKey(input.workspaceId, input.terminalId);
		if (!terminalKey) return;
		const context = deps.ensureContext({
			terminalKey,
			workspaceId: input.workspaceId,
			workspaceName: input.workspaceName,
			terminalId: input.terminalId,
			container: input.container,
			active: input.active,
			lastWorkspaceId: deps.getLastWorkspaceId(terminalKey),
		});
		if (context.lastWorkspaceId && context.lastWorkspaceId !== context.workspaceId) {
			deps.scheduleFitStabilization(context.terminalKey, 'workspace_switch');
			deps.scheduleReattachCheck(context.terminalKey, 'workspace_switch');
		}
		deps.setLastWorkspaceId(terminalKey, context.workspaceId);
		if (input.container) {
			deps.attachTerminal(terminalKey, input.container, input.active);
			deps.attachResizeObserver(terminalKey, input.container);
			if (input.active) {
				requestAnimationFrame(() => {
					deps.fitTerminal(terminalKey, deps.hasStarted(terminalKey));
					deps.forceRedraw(terminalKey);
					const handle = deps.getHandle(terminalKey);
					if (handle && !deps.hasVisibleTerminalContent(handle)) {
						deps.nudgeRedraw(terminalKey, handle);
					}
				});
			}
		}
		deps.syncTerminalStream(terminalKey);
	};

	const detachTerminal = (workspaceId: string, terminalId: string): void => {
		const terminalKey = deps.buildTerminalKey(workspaceId, terminalId);
		if (!terminalKey) return;
		deps.markDetached(terminalKey);
		deps.detachResizeObserver(terminalKey);
	};

	const closeTerminal = async (workspaceId: string, terminalId: string): Promise<void> => {
		const terminalKey = deps.buildTerminalKey(workspaceId, terminalId);
		if (!terminalKey) return;
		try {
			await deps.stopTerminal(workspaceId, terminalId);
		} catch {
			// Ignore failures.
		}
		deps.disposeTerminalResources(terminalKey);
		deps.deleteContext(terminalKey);
	};

	const restartTerminal = async (workspaceId: string, terminalId: string): Promise<void> => {
		const terminalKey = deps.buildTerminalKey(workspaceId, terminalId);
		if (!terminalKey) return;
		await deps.beginTerminal(terminalKey);
	};

	const retryHealthCheck = (workspaceId: string, terminalId: string): void => {
		const terminalKey = deps.buildTerminalKey(workspaceId, terminalId);
		if (!terminalKey) return;
		deps.requestHealthCheck(terminalKey);
	};

	const focusTerminalInstance = (workspaceId: string, terminalId: string): void => {
		const terminalKey = deps.buildTerminalKey(workspaceId, terminalId);
		if (!terminalKey) return;
		deps.focusTerminal(terminalKey);
	};

	const scrollTerminalToBottom = (workspaceId: string, terminalId: string): void => {
		const terminalKey = deps.buildTerminalKey(workspaceId, terminalId);
		if (!terminalKey) return;
		deps.scrollToBottom(terminalKey);
	};

	const isTerminalAtBottom = (workspaceId: string, terminalId: string): boolean => {
		const terminalKey = deps.buildTerminalKey(workspaceId, terminalId);
		if (!terminalKey) return true;
		return deps.isAtBottom(terminalKey);
	};

	return {
		syncTerminal,
		detachTerminal,
		closeTerminal,
		restartTerminal,
		retryHealthCheck,
		focusTerminalInstance,
		scrollTerminalToBottom,
		isTerminalAtBottom,
	};
};

export type { TerminalSyncInput };
