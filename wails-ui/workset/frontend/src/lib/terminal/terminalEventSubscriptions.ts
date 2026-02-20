import { EVENT_TERMINAL_DATA } from '../events';

export type TerminalPayload = {
	workspaceId: string;
	terminalId: string;
	windowName?: string;
	dataB64?: string;
	bytes?: number;
	seq?: number;
};

type TerminalScopedPayload = {
	workspaceId: string;
	terminalId: string;
};

type EventHandler<T> = (payload: T) => void;

type TerminalEventSubscriptionsDeps = {
	subscribeEvent: <T>(event: string, handler: EventHandler<T>) => () => void;
	buildTerminalKey: (workspaceId: string, terminalId: string) => string;
	isWorkspaceMismatch: (
		key: string,
		payloadWorkspaceId?: string,
		payloadTerminalId?: string,
	) => boolean;
	onTerminalData: (id: string, payload: TerminalPayload) => void;
};

const resolveTerminalKey = <T extends TerminalScopedPayload>(
	deps: Pick<TerminalEventSubscriptionsDeps, 'buildTerminalKey' | 'isWorkspaceMismatch'>,
	payload: T,
): string | null => {
	const terminalId = payload.terminalId;
	const workspaceId = payload.workspaceId;
	if (!terminalId || !workspaceId) return null;
	const id = deps.buildTerminalKey(workspaceId, terminalId);
	if (!id) return null;
	if (deps.isWorkspaceMismatch(id, workspaceId, terminalId)) return null;
	return id;
};

export const createTerminalEventSubscriptions = (deps: TerminalEventSubscriptionsDeps) => {
	const listeners = new Set<string>();
	const unsubscribeHandlers: Array<() => void> = [];

	const register = <T>(event: string, handler: EventHandler<T>): void => {
		if (listeners.has(event)) return;
		unsubscribeHandlers.push(deps.subscribeEvent(event, handler));
		listeners.add(event);
	};

	const ensureListeners = (): void => {
		register<TerminalPayload>(EVENT_TERMINAL_DATA, (payload) => {
			const id = resolveTerminalKey(deps, payload);
			if (!id) return;
			deps.onTerminalData(id, payload);
		});
	};

	const cleanupListeners = (): void => {
		for (const unsubscribe of unsubscribeHandlers.splice(0)) {
			unsubscribe();
		}
		listeners.clear();
	};

	return {
		ensureListeners,
		cleanupListeners,
	};
};
