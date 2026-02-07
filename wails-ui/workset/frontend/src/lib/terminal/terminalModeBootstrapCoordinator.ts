import type {
	TerminalBootstrapDonePayload,
	TerminalBootstrapPayload,
	TerminalLifecyclePayload,
	TerminalModesPayload,
} from './terminalEventSubscriptions';

type ReplayState = 'idle' | 'replaying' | 'live';

type TerminalContextLike = {
	workspaceId?: string;
	terminalId?: string;
};

type PendingOutputChunk = {
	bytes: number;
};

type TerminalMode = {
	altScreen: boolean;
	mouse: boolean;
	mouseSGR: boolean;
	mouseEncoding: string;
};

type TerminalKittySnapshot = {
	images?: Array<
		| {
				id: string;
				format?: string;
				width?: number;
				height?: number;
				data?: string | number[] | Uint8Array;
		  }
		| undefined
	>;
	placements?: Array<
		| {
				id: number;
				imageId: string;
				row: number;
				col: number;
				rows: number;
				cols: number;
				x?: number;
				y?: number;
				z?: number;
		  }
		| undefined
	>;
};

type KittySnapshotEvent = {
	kind: string;
	snapshot?: TerminalKittySnapshot;
};

type TerminalHealth = 'unknown' | 'checking' | 'ok' | 'stale';

type TerminalModeBootstrapCoordinatorDeps<TKittyEvent extends KittySnapshotEvent> = {
	buildTerminalKey: (workspaceId: string, terminalId: string) => string;
	getContext: (key: string) => TerminalContextLike | null;
	logDebug: (id: string, event: string, details: Record<string, unknown>) => void;
	markInput: (id: string) => void;
	bootstrapHandled: Map<string, boolean>;
	setReplayState: (id: string, state: ReplayState) => void;
	enqueueOutput: (id: string, data: string, bytes: number) => void;
	countBytes: (data: string) => number;
	pendingReplayKitty: Map<string, TKittyEvent[]>;
	hasTerminalHandle: (id: string) => boolean;
	applyKittyEvent: (id: string, event: TKittyEvent) => Promise<void>;
	setHealth: (id: string, status: TerminalHealth, message?: string) => void;
	initialCreditMap: Map<string, number>;
	initialStreamCredit: number;
	pendingReplayOutput: Map<string, PendingOutputChunk[]>;
	getStatus: (id: string) => string;
	setStatusAndMessage: (id: string, status: string, message: string) => void;
	scheduleBootstrapHealthCheck: (id: string, replayBytes: number) => void;
	emitState: (id: string) => void;
	applyLifecyclePayload: (id: string, payload: TerminalLifecyclePayload) => void;
	setMode: (id: string, mode: TerminalMode) => void;
	syncTerminalWebLinks: (id: string) => void;
};

const coerceKittySnapshot = (
	value: TerminalBootstrapPayload['kitty'],
): TerminalKittySnapshot | null => {
	if (!value) return null;
	const images = Array.isArray(value.images)
		? (value.images as TerminalKittySnapshot['images'])
		: undefined;
	const placements = Array.isArray(value.placements)
		? (value.placements as TerminalKittySnapshot['placements'])
		: undefined;
	if (!images && !placements) return null;
	return { images, placements };
};

export const createTerminalModeBootstrapCoordinator = <TKittyEvent extends KittySnapshotEvent>(
	deps: TerminalModeBootstrapCoordinatorDeps<TKittyEvent>,
) => {
	const isWorkspaceMismatch = (
		key: string,
		payloadWorkspaceId?: string,
		payloadTerminalId?: string,
	): boolean => {
		if (!payloadWorkspaceId || !payloadTerminalId) return false;
		const context = deps.getContext(key);
		if (!context?.workspaceId || !context.terminalId) return false;
		if (context.workspaceId === payloadWorkspaceId && context.terminalId === payloadTerminalId) {
			return false;
		}
		deps.logDebug(key, 'workspace_mismatch', {
			payloadWorkspaceId,
			payloadTerminalId,
			contextWorkspaceId: context.workspaceId,
			contextTerminalId: context.terminalId,
		});
		return true;
	};

	const handleBootstrapPayload = (payload: TerminalBootstrapPayload): void => {
		const terminalId = payload.terminalId;
		const workspaceId = payload.workspaceId;
		if (!terminalId || !workspaceId) return;
		const id = deps.buildTerminalKey(workspaceId, terminalId);
		if (!id) return;
		if (isWorkspaceMismatch(id, workspaceId, terminalId)) return;
		if (deps.bootstrapHandled.get(id)) {
			deps.logDebug(id, 'bootstrap_duplicate', { source: payload.source ?? 'event' });
			return;
		}
		deps.markInput(id);
		if (payload.safeToReplay === false) {
			deps.setReplayState(id, 'live');
			deps.bootstrapHandled.set(id, true);
			return;
		}
		deps.setReplayState(id, 'replaying');
		if (payload.snapshot) {
			deps.enqueueOutput(id, payload.snapshot, deps.countBytes(payload.snapshot));
		}
		if (payload.backlog) {
			deps.enqueueOutput(id, payload.backlog, deps.countBytes(payload.backlog));
		}
		const kittySnapshot = coerceKittySnapshot(payload.kitty);
		if (kittySnapshot) {
			const kittyEvent = {
				kind: 'snapshot',
				snapshot: kittySnapshot,
			} as TKittyEvent;
			if (!deps.hasTerminalHandle(id)) {
				const pending = deps.pendingReplayKitty.get(id) ?? [];
				pending.push(kittyEvent);
				deps.pendingReplayKitty.set(id, pending);
			} else {
				void deps.applyKittyEvent(id, kittyEvent);
			}
		}
		if (payload.backlogTruncated) {
			deps.setHealth(id, 'ok', 'Backlog truncated; showing latest output.');
		}
		deps.initialCreditMap.set(id, payload.initialCredit ?? deps.initialStreamCredit);
		deps.bootstrapHandled.set(id, true);
		deps.logDebug(id, 'bootstrap', {
			source: payload.source,
			snapshotSource: payload.snapshotSource,
			backlogSource: payload.backlogSource,
			backlogTruncated: payload.backlogTruncated,
		});
	};

	const handleBootstrapDonePayload = (payload: TerminalBootstrapDonePayload): void => {
		const terminalId = payload.terminalId;
		const workspaceId = payload.workspaceId;
		if (!terminalId || !workspaceId) return;
		const id = deps.buildTerminalKey(workspaceId, terminalId);
		if (!id) return;
		if (isWorkspaceMismatch(id, workspaceId, terminalId)) return;
		const pending = deps.pendingReplayOutput.get(id) ?? [];
		const replayBytes = pending.reduce((sum, chunk) => sum + chunk.bytes, 0);
		deps.markInput(id);
		if (deps.getStatus(id) !== 'ready') {
			deps.setStatusAndMessage(id, 'ready', '');
		}
		deps.scheduleBootstrapHealthCheck(id, replayBytes);
		deps.setReplayState(id, 'live');
		deps.emitState(id);
	};

	const handleTerminalLifecyclePayload = (id: string, payload: TerminalLifecyclePayload): void => {
		deps.applyLifecyclePayload(id, payload);
	};

	const handleTerminalModesPayload = (id: string, payload: TerminalModesPayload): void => {
		deps.setMode(id, {
			altScreen: payload.altScreen ?? false,
			mouse: payload.mouse ?? false,
			mouseSGR: payload.mouseSGR ?? false,
			mouseEncoding: payload.mouseEncoding ?? 'x10',
		});
		deps.syncTerminalWebLinks(id);
	};

	return {
		isWorkspaceMismatch,
		handleBootstrapPayload,
		handleBootstrapDonePayload,
		handleTerminalLifecyclePayload,
		handleTerminalModesPayload,
	};
};
