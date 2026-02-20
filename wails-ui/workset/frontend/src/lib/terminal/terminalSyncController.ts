type TerminalSyncInput = {
	workspaceId: string;
	terminalId: string;
	container: HTMLDivElement | null;
	active: boolean;
	source?: string;
};

type TerminalDetachOptions = {
	force?: boolean;
};

type TerminalSyncContext = {
	terminalKey: string;
	workspaceId: string;
	terminalId: string;
	container: HTMLDivElement | null;
	active: boolean;
};

type TerminalAttachTraceMeta = {
	reason: string;
	source?: string;
};

export type TerminalSyncControllerDependencies = {
	ensureGlobals: () => void;
	buildTerminalKey: (workspaceId: string, terminalId: string) => string;
	ensureContext: (input: TerminalSyncContext) => TerminalSyncContext;
	hasContext?: (key: string) => boolean;
	deleteContext: (key: string) => void;
	attachTerminal: (id: string, container: HTMLDivElement | null, active: boolean) => unknown;
	attachResizeObserver: (id: string, container: HTMLDivElement) => void;
	detachResizeObserver: (id: string) => void;
	syncTerminalStream: (id: string) => void;
	markDetached: (id: string) => void;
	stopTerminal: (workspaceId: string, terminalId: string) => Promise<void>;
	disposeTerminalResources: (id: string) => void;
	focusTerminal: (id: string) => void;
	scrollToBottom: (id: string) => void;
	isAtBottom: (id: string) => boolean;
	trace?: (id: string, event: string, details: Record<string, unknown>) => void;
};

type TerminalAttachmentState = {
	container: HTMLDivElement;
	active: boolean;
};

export const createTerminalSyncController = (deps: TerminalSyncControllerDependencies) => {
	const lastSyncState = new Map<
		string,
		{
			workspaceId: string;
			terminalId: string;
			container: HTMLDivElement | null;
			active: boolean;
		}
	>();
	const attachmentState = new Map<string, TerminalAttachmentState>();
	const containerOwners = new Map<HTMLDivElement, string>();

	const requestAttach = (
		id: string,
		container: HTMLDivElement,
		active: boolean,
		trace: TerminalAttachTraceMeta,
	): void => {
		const displacedId = containerOwners.get(container);
		if (displacedId && displacedId !== id) {
			requestDetach(displacedId, {
				reason: 'displaced_by_container_reuse',
				source: trace.source,
			});
			const displaced = lastSyncState.get(displacedId);
			if (displaced) {
				lastSyncState.set(displacedId, {
					...displaced,
					container: null,
					active: false,
				});
				deps.ensureContext({
					terminalKey: displacedId,
					workspaceId: displaced.workspaceId,
					terminalId: displaced.terminalId,
					container: null,
					active: false,
				});
			}
			deps.trace?.(displacedId, 'sync_terminal_displaced_by_container_reuse', {
				by: id,
				source: trace.source ?? '',
			});
		}
		const current = attachmentState.get(id);
		if (current && current.container === container && current.active === active) {
			deps.trace?.(id, 'mount_skip_attached', {
				active,
				reason: trace.reason,
				source: trace.source ?? '',
			});
			return;
		}
		deps.attachTerminal(id, container, active);
		deps.attachResizeObserver(id, container);
		attachmentState.set(id, { container, active });
		containerOwners.set(container, id);
		deps.trace?.(id, 'mount_apply_attach', {
			active,
			reason: trace.reason,
			source: trace.source ?? '',
			containerConnected: container.isConnected,
			containerWidth: container.clientWidth,
			containerHeight: container.clientHeight,
		});
	};

	const requestDetach = (
		id: string,
		trace: {
			reason: string;
			source?: string;
		},
	): void => {
		const current = attachmentState.get(id);
		if (!current) {
			deps.trace?.(id, 'mount_skip_detached', {
				reason: trace.reason,
				source: trace.source ?? '',
			});
			return;
		}
		if (containerOwners.get(current.container) === id) {
			containerOwners.delete(current.container);
		}
		deps.markDetached(id);
		deps.detachResizeObserver(id);
		attachmentState.delete(id);
		deps.trace?.(id, 'mount_apply_detach', {
			reason: trace.reason,
			source: trace.source ?? '',
		});
	};

	const syncTerminal = (input: TerminalSyncInput): void => {
		if (!input.terminalId || !input.workspaceId) return;
		deps.ensureGlobals();
		const terminalKey = deps.buildTerminalKey(input.workspaceId, input.terminalId);
		if (!terminalKey) return;
		const source = input.source?.trim() || 'unspecified';
		let previous = lastSyncState.get(terminalKey);
		if (previous && deps.hasContext && !deps.hasContext(terminalKey)) {
			lastSyncState.delete(terminalKey);
			attachmentState.delete(terminalKey);
			deps.trace?.(terminalKey, 'sync_terminal_clear_stale_state', {
				source,
			});
			previous = undefined;
		}

		if (
			previous &&
			previous.workspaceId === input.workspaceId &&
			previous.terminalId === input.terminalId &&
			previous.container === input.container &&
			previous.active === input.active
		) {
			deps.trace?.(terminalKey, 'sync_terminal_skip_unchanged', {
				source,
				hasContainer: Boolean(input.container),
				active: input.active,
			});
			return;
		}

		lastSyncState.set(terminalKey, {
			workspaceId: input.workspaceId,
			terminalId: input.terminalId,
			container: input.container,
			active: input.active,
		});
		deps.ensureContext({
			terminalKey,
			workspaceId: input.workspaceId,
			terminalId: input.terminalId,
			container: input.container,
			active: input.active,
		});

		if (!input.container) {
			deps.trace?.(terminalKey, 'sync_terminal_skip_stream_no_container', {
				source,
				active: input.active,
			});
			return;
		}

		const activeFlipOnSameContainer =
			source === 'controller.active_change' &&
			previous !== undefined &&
			previous.container === input.container &&
			previous.active !== input.active;

		requestAttach(terminalKey, input.container, input.active, {
			reason: 'sync_terminal_attach',
			source,
		});

		if (activeFlipOnSameContainer) {
			const previousActive = previous?.active ?? false;
			deps.trace?.(terminalKey, 'sync_terminal_active_flip_attach', {
				source,
				previousActive,
				active: input.active,
				hasContainer: true,
			});
			if (input.active) {
				deps.focusTerminal(terminalKey);
				deps.syncTerminalStream(terminalKey);
				deps.trace?.(terminalKey, 'sync_terminal_stream_requested', {
					source,
					reason: 'active_flip',
				});
			}
			return;
		}

		if (input.active) {
			deps.focusTerminal(terminalKey);
		}
		deps.syncTerminalStream(terminalKey);
		deps.trace?.(terminalKey, 'sync_terminal_stream_requested', {
			source,
		});
	};

	const detachTerminal = (
		workspaceId: string,
		terminalId: string,
		options?: TerminalDetachOptions,
	): void => {
		const terminalKey = deps.buildTerminalKey(workspaceId, terminalId);
		if (!terminalKey) return;
		const force = options?.force === true;
		const latest = lastSyncState.get(terminalKey);
		if (!force && latest?.container?.isConnected) {
			deps.trace?.(terminalKey, 'detach_terminal_skip_stale', {
				workspaceId,
				terminalId,
			});
			return;
		}
		requestDetach(terminalKey, {
			reason: force ? 'detach_terminal_force' : 'detach_terminal',
			source: 'sync_controller_detach',
		});
		// Keep key/workspace/terminal association stable across tab switches so
		// in-flight init/start calls can resolve against the same terminal entry.
		deps.ensureContext({
			terminalKey,
			workspaceId,
			terminalId,
			container: null,
			active: false,
		});
		lastSyncState.delete(terminalKey);
	};

	const closeTerminal = async (workspaceId: string, terminalId: string): Promise<void> => {
		const terminalKey = deps.buildTerminalKey(workspaceId, terminalId);
		if (!terminalKey) return;
		requestDetach(terminalKey, {
			reason: 'close_terminal',
			source: 'sync_controller_close',
		});
		try {
			await deps.stopTerminal(workspaceId, terminalId);
			deps.trace?.(terminalKey, 'close_terminal_stop_ok', {});
		} catch {
			deps.trace?.(terminalKey, 'close_terminal_stop_error', {});
			// Ignore stop failures when closing stale/missing sessions.
		}
		deps.disposeTerminalResources(terminalKey);
		deps.deleteContext(terminalKey);
		lastSyncState.delete(terminalKey);
		attachmentState.delete(terminalKey);
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
		focusTerminalInstance,
		scrollTerminalToBottom,
		isTerminalAtBottom,
	};
};

export type { TerminalSyncInput };
