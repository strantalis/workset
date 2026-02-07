import type { StripResult } from './inputFilter';

type TerminalMouseMode = {
	mouse: boolean;
};

type TerminalInputOrchestratorDeps = {
	shouldSuppressMouseInput: (id: string, data: string) => boolean;
	getMode: (id: string) => TerminalMouseMode;
	filterMouseReports: (data: string, mode: TerminalMouseMode, tail: string) => StripResult;
	getMouseTail: (id: string) => string;
	setMouseTail: (id: string, tail: string) => void;
	ensureSessionActive: (id: string) => Promise<void>;
	hasStarted: (id: string) => boolean;
	appendPendingInput: (id: string, data: string) => void;
	recordOutputBytes: (id: string, bytes: number) => void;
	getWorkspaceId: (id: string) => string;
	getTerminalId: (id: string) => string;
	write: (workspaceId: string, terminalId: string, data: string) => Promise<void>;
	markStopped: (id: string) => void;
	resetTerminalInstance: (id: string) => void;
	beginTerminal: (id: string, quiet?: boolean) => Promise<void>;
	writeFailureMessage: (id: string, message: string) => void;
};

const RECOVERY_ERROR_MARKERS = ['session not found', 'terminal not started', 'terminal not found'];

const isRecoverableWriteError = (error: unknown): boolean => {
	if (typeof error === 'string') {
		return RECOVERY_ERROR_MARKERS.some((marker) => error.includes(marker));
	}
	if (error instanceof Error) {
		return RECOVERY_ERROR_MARKERS.some((marker) => error.message.includes(marker));
	}
	return false;
};

export const createTerminalInputOrchestrator = (deps: TerminalInputOrchestratorDeps) => {
	const sendInput = (id: string, data: string): void => {
		if (deps.shouldSuppressMouseInput(id, data)) {
			return;
		}
		const previousTail = deps.getMouseTail(id);
		const mode = deps.getMode(id);
		const mouseResult = deps.filterMouseReports(data, mode, previousTail);
		if (mouseResult.tail !== previousTail) {
			deps.setMouseTail(id, mouseResult.tail);
		}
		const filtered = mouseResult.filtered;
		if (!filtered) {
			return;
		}
		void deps.ensureSessionActive(id);
		if (!deps.hasStarted(id)) {
			deps.appendPendingInput(id, filtered);
			return;
		}
		deps.recordOutputBytes(id, filtered.length);
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		void deps.write(workspaceId, terminalId, filtered).catch((error: unknown) => {
			deps.appendPendingInput(id, filtered);
			deps.markStopped(id);
			if (isRecoverableWriteError(error)) {
				deps.resetTerminalInstance(id);
				void deps.beginTerminal(id, true);
			}
			deps.writeFailureMessage(id, String(error));
		});
	};

	return {
		sendInput,
	};
};
