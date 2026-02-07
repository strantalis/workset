import {
	EVENT_SESSIOND_RESTARTED,
	EVENT_TERMINAL_BOOTSTRAP,
	EVENT_TERMINAL_BOOTSTRAP_DONE,
	EVENT_TERMINAL_DATA,
	EVENT_TERMINAL_KITTY,
	EVENT_TERMINAL_LIFECYCLE,
	EVENT_TERMINAL_MODES,
} from '../events';

export type TerminalPayload = {
	workspaceId: string;
	terminalId: string;
	data: string;
	bytes?: number;
};

export type TerminalBootstrapPayload = {
	workspaceId: string;
	terminalId: string;
	snapshot?: string;
	snapshotSource?: string;
	kitty?: { images?: unknown[]; placements?: unknown[] } | null;
	backlog?: string;
	backlogSource?: string;
	backlogTruncated?: boolean;
	nextOffset?: number;
	source?: string;
	altScreen?: boolean;
	mouse?: boolean;
	mouseSGR?: boolean;
	mouseEncoding?: string;
	safeToReplay?: boolean;
	initialCredit?: number;
};

export type TerminalBootstrapDonePayload = {
	workspaceId: string;
	terminalId: string;
};

export type TerminalLifecyclePayload = {
	workspaceId: string;
	terminalId: string;
	status: string;
	message?: string;
};

export type TerminalModesPayload = {
	workspaceId: string;
	terminalId: string;
	altScreen?: boolean;
	mouse?: boolean;
	mouseSGR?: boolean;
	mouseEncoding?: string;
};

export type TerminalKittyPayload = {
	workspaceId: string;
	terminalId: string;
	event: {
		kind: string;
	};
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
	onTerminalBootstrap: (payload: TerminalBootstrapPayload) => void;
	onTerminalBootstrapDone: (payload: TerminalBootstrapDonePayload) => void;
	onTerminalLifecycle: (id: string, payload: TerminalLifecyclePayload) => void;
	onTerminalModes: (id: string, payload: TerminalModesPayload) => void;
	onTerminalKitty: (id: string, payload: TerminalKittyPayload) => void;
	onSessiondRestarted: () => void;
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
		register<TerminalBootstrapPayload>(EVENT_TERMINAL_BOOTSTRAP, (payload) => {
			deps.onTerminalBootstrap(payload);
		});
		register<TerminalBootstrapDonePayload>(EVENT_TERMINAL_BOOTSTRAP_DONE, (payload) => {
			deps.onTerminalBootstrapDone(payload);
		});
		register<TerminalLifecyclePayload>(EVENT_TERMINAL_LIFECYCLE, (payload) => {
			const id = resolveTerminalKey(deps, payload);
			if (!id) return;
			deps.onTerminalLifecycle(id, payload);
		});
		register<TerminalModesPayload>(EVENT_TERMINAL_MODES, (payload) => {
			const id = resolveTerminalKey(deps, payload);
			if (!id) return;
			deps.onTerminalModes(id, payload);
		});
		register<TerminalKittyPayload>(EVENT_TERMINAL_KITTY, (payload) => {
			const id = resolveTerminalKey(deps, payload);
			if (!id) return;
			deps.onTerminalKitty(id, payload);
		});
		register<void>(EVENT_SESSIOND_RESTARTED, () => {
			deps.onSessiondRestarted();
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
