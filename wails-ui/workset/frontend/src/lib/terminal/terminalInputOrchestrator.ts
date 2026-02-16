type TerminalInputOrchestratorDeps = {
	ensureSessionActive: (id: string) => Promise<void>;
	hasStarted: (id: string) => boolean;
	appendPendingInput: (id: string, data: string) => void;
	recordOutputBytes: (id: string, bytes: number) => void;
	getWorkspaceId: (id: string) => string;
	getTerminalId: (id: string) => string;
	isContextActive?: (id: string) => boolean;
	isTerminalFocused?: (id: string) => boolean;
	write: (workspaceId: string, terminalId: string, data: string) => Promise<void>;
	markStopped: (id: string) => void;
	trace?: (id: string, event: string, details: Record<string, unknown>) => void;
};

export const createTerminalInputOrchestrator = (deps: TerminalInputOrchestratorDeps) => {
	const FOCUS_REPORT_SEQUENCES = ['\x1b[I', '\x1b[O'] as const;

	const stripFocusReports = (data: string): { sanitized: string; removed: number } => {
		if (!data.includes('\x1b[')) {
			return { sanitized: data, removed: 0 };
		}
		let sanitized = data;
		let removed = 0;
		for (const seq of FOCUS_REPORT_SEQUENCES) {
			const parts = sanitized.split(seq);
			if (parts.length <= 1) {
				continue;
			}
			removed += parts.length - 1;
			sanitized = parts.join('');
		}
		return {
			sanitized,
			removed,
		};
	};

	const hasPrintableText = (value: string): boolean => {
		for (let index = 0; index < value.length; index += 1) {
			const code = value.charCodeAt(index);
			if (code >= 0x20 && code <= 0x7e) {
				return true;
			}
		}
		return false;
	};

	const hasMouseSequence = (value: string): boolean => {
		// Legacy X10 mouse reports: ESC [ M Cb Cx Cy
		// Keep these ungated even when focus/context state is stale during popout handoffs.
		for (
			let sequenceStart = value.indexOf('\x1b[M');
			sequenceStart !== -1;
			sequenceStart = value.indexOf('\x1b[M', sequenceStart + 1)
		) {
			if (sequenceStart + 6 <= value.length) {
				return true;
			}
		}

		// Rxvt mouse mode reports: ESC [ Cb ; Cx ; Cy M
		for (
			let sequenceStart = value.indexOf('\x1b[');
			sequenceStart !== -1;
			sequenceStart = value.indexOf('\x1b[', sequenceStart + 1)
		) {
			let index = sequenceStart + 2;
			let separators = 0;
			let digitsInSegment = 0;
			while (index < value.length) {
				const code = value.charCodeAt(index);
				if (code >= 0x30 && code <= 0x39) {
					digitsInSegment += 1;
					index += 1;
					continue;
				}
				const char = value[index];
				if (char === ';') {
					if (digitsInSegment === 0 || separators >= 2) break;
					separators += 1;
					digitsInSegment = 0;
					index += 1;
					continue;
				}
				if (char === 'M' && separators === 2 && digitsInSegment > 0) {
					return true;
				}
				break;
			}
		}

		if (!value.includes('\x1b[<')) return false;
		for (
			let sequenceStart = value.indexOf('\x1b[<');
			sequenceStart !== -1;
			sequenceStart = value.indexOf('\x1b[<', sequenceStart + 1)
		) {
			let index = sequenceStart + 3;
			let separators = 0;
			let digitsInSegment = 0;
			while (index < value.length) {
				const code = value.charCodeAt(index);
				if (code >= 0x30 && code <= 0x39) {
					digitsInSegment += 1;
					index += 1;
					continue;
				}
				const char = value[index];
				if (char === ';') {
					if (digitsInSegment === 0 || separators >= 2) break;
					separators += 1;
					digitsInSegment = 0;
					index += 1;
					continue;
				}
				if ((char === 'm' || char === 'M') && separators === 2 && digitsInSegment > 0) {
					return true;
				}
				break;
			}
		}
		return false;
	};

	const sendInput = (id: string, data: string): void => {
		if (!data) {
			return;
		}
		const stripped = stripFocusReports(data);
		// Focus in/out reports can flood during mount/re-focus churn and show up as
		// literal "^[[I/^[[O" in interactive shells. Keep the PTY input path clean.
		if (stripped.removed > 0) {
			deps.trace?.(id, 'frontend_input_focus_report_dropped', {
				bytes: data.length,
				removed: stripped.removed,
				preview: data.slice(0, 32),
			});
			data = stripped.sanitized;
			if (!data) {
				return;
			}
		}
		const printableInput = hasPrintableText(data);
		const shouldGatePrintableInput = printableInput && !hasMouseSequence(data);
		if (deps.isContextActive && !deps.isContextActive(id) && shouldGatePrintableInput) {
			deps.trace?.(id, 'frontend_input_drop_inactive_context', {
				bytes: data.length,
				preview: data.slice(0, 24),
			});
			return;
		}
		if (deps.isTerminalFocused && !deps.isTerminalFocused(id) && shouldGatePrintableInput) {
			deps.trace?.(id, 'frontend_input_drop_unfocused_terminal', {
				bytes: data.length,
				preview: data.slice(0, 24),
			});
			return;
		}
		if (!deps.hasStarted(id)) {
			void deps.ensureSessionActive(id);
			deps.appendPendingInput(id, data);
			deps.trace?.(id, 'frontend_input_queued', { bytes: data.length, preview: data.slice(0, 24) });
			return;
		}
		deps.recordOutputBytes(id, data.length);
		const workspaceId = deps.getWorkspaceId(id);
		const terminalId = deps.getTerminalId(id);
		if (!workspaceId || !terminalId) return;
		deps.trace?.(id, 'frontend_input_write', { bytes: data.length, preview: data.slice(0, 24) });
		void deps.write(workspaceId, terminalId, data).catch(() => {
			deps.appendPendingInput(id, data);
			deps.markStopped(id);
			void deps.ensureSessionActive(id);
			deps.trace?.(id, 'frontend_input_write_failed', { bytes: data.length });
		});
	};

	return {
		sendInput,
	};
};
