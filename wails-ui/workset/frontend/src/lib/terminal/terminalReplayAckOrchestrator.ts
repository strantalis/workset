export type ReplayState = 'idle' | 'replaying' | 'live';

export type ReplayOutputChunk = {
	data: string;
	bytes: number;
};

type TerminalOutputPayload = {
	data: string;
	bytes?: number;
};

type TerminalReplayAckOrchestratorDeps<TKittyEvent> = {
	enqueueOutput: (id: string, data: string, bytes: number) => void;
	flushOutput: (id: string, scheduled: boolean) => void;
	forceRedraw: (id: string) => void;
	hasTerminalHandle: (id: string) => boolean;
	applyKittyEvent: (id: string, event: TKittyEvent) => Promise<void>;
	getWorkspaceId: (id: string) => string;
	getTerminalId: (id: string) => string;
	ack: (workspaceId: string, terminalId: string, bytes: number) => Promise<void>;
	setTimeoutFn: (callback: () => void, timeoutMs: number) => number;
	clearTimeoutFn: (handle: number) => void;
	countBytes: (data: string) => number;
	recordBytesIn: (id: string, bytes: number) => void;
	noteOutputActivity: (id: string) => void;
	ackBatchBytes: number;
	ackFlushDelayMs: number;
	initialStreamCredit: number;
};

export const createTerminalReplayAckOrchestrator = <TKittyEvent>(
	deps: TerminalReplayAckOrchestratorDeps<TKittyEvent>,
) => {
	const replayState = new Map<string, ReplayState>();
	const pendingReplayOutput = new Map<string, ReplayOutputChunk[]>();
	const pendingReplayKitty = new Map<string, TKittyEvent[]>();
	const pendingAckBytes = new Map<string, number>();
	const initialCreditMap = new Map<string, number>();
	const initialCreditSent = new Set<string>();
	const ackTimers = new Map<string, number>();

	const clearAckTimer = (id: string): void => {
		const timer = ackTimers.get(id);
		if (!timer) return;
		deps.clearTimeoutFn(timer);
		ackTimers.delete(id);
	};

	const flushAck = (id: string): void => {
		const bytes = pendingAckBytes.get(id);
		if (!bytes || bytes <= 0) return;
		pendingAckBytes.delete(id);
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		void deps.ack(workspaceId, terminalId, bytes).catch(() => undefined);
	};

	const grantInitialCredit = (id: string): void => {
		if (initialCreditSent.has(id)) return;
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		initialCreditSent.add(id);
		const credit = initialCreditMap.get(id) ?? deps.initialStreamCredit;
		void deps.ack(workspaceId, terminalId, credit).catch(() => {
			initialCreditSent.delete(id);
		});
	};

	const scheduleAck = (id: string): void => {
		if (ackTimers.has(id)) return;
		ackTimers.set(
			id,
			deps.setTimeoutFn(() => {
				ackTimers.delete(id);
				flushAck(id);
			}, deps.ackFlushDelayMs),
		);
	};

	const recordAckBytes = (id: string, bytes: number): void => {
		const total = (pendingAckBytes.get(id) ?? 0) + bytes;
		pendingAckBytes.set(id, total);
		if (total >= deps.ackBatchBytes) {
			flushAck(id);
			return;
		}
		scheduleAck(id);
	};

	const setReplayState = (id: string, state: ReplayState): void => {
		replayState.set(id, state);
		if (state !== 'live') return;
		const pending = pendingReplayOutput.get(id) ?? [];
		if (pending.length > 0) {
			pendingReplayOutput.delete(id);
			for (const chunk of pending) {
				deps.enqueueOutput(id, chunk.data, chunk.bytes);
			}
		}
		const kitty = pendingReplayKitty.get(id) ?? [];
		if (kitty.length > 0 && deps.hasTerminalHandle(id)) {
			pendingReplayKitty.delete(id);
			for (const event of kitty) {
				void deps.applyKittyEvent(id, event);
			}
		}
		deps.flushOutput(id, true);
		deps.forceRedraw(id);
		grantInitialCredit(id);
		flushAck(id);
	};

	const handleTerminalData = (id: string, payload: TerminalOutputPayload): void => {
		const bytes =
			payload.bytes && payload.bytes > 0 ? payload.bytes : deps.countBytes(payload.data);
		const isLive = replayState.get(id) === 'live';
		if (!isLive) {
			const pending = pendingReplayOutput.get(id) ?? [];
			pending.push({ data: payload.data, bytes });
			pendingReplayOutput.set(id, pending);
			return;
		}
		deps.enqueueOutput(id, payload.data, bytes);
		recordAckBytes(id, bytes);
		deps.recordBytesIn(id, bytes);
		deps.noteOutputActivity(id);
	};

	const handleTerminalKitty = (id: string, event: TKittyEvent): void => {
		const isLive = replayState.get(id) === 'live';
		if (!isLive || !deps.hasTerminalHandle(id)) {
			const pending = pendingReplayKitty.get(id) ?? [];
			pending.push(event);
			pendingReplayKitty.set(id, pending);
			return;
		}
		void deps.applyKittyEvent(id, event);
	};

	const resetSession = (id: string): void => {
		replayState.set(id, 'idle');
		pendingReplayOutput.delete(id);
		pendingReplayKitty.delete(id);
		pendingAckBytes.delete(id);
		initialCreditMap.delete(id);
		initialCreditSent.delete(id);
		clearAckTimer(id);
	};

	const destroy = (id: string): void => {
		clearAckTimer(id);
		replayState.delete(id);
		pendingReplayOutput.delete(id);
		pendingReplayKitty.delete(id);
		pendingAckBytes.delete(id);
		initialCreditMap.delete(id);
		initialCreditSent.delete(id);
	};

	return {
		pendingReplayOutput,
		pendingReplayKitty,
		initialCreditMap,
		setReplayState,
		handleTerminalData,
		handleTerminalKitty,
		resetSession,
		destroy,
	};
};
