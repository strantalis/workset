type TerminalSyncInput = {
	workspaceId: string;
	terminalId: string;
	container: HTMLDivElement | null;
	active: boolean;
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

type TerminalMountStatus = 'detached' | 'attaching' | 'attached' | 'detaching';

type TerminalMountIntent =
	| {
			type: 'attach';
			generation: number;
			container: HTMLDivElement;
			active: boolean;
	  }
	| {
			type: 'detach';
			generation: number;
	  };

type TerminalMountState = {
	generation: number;
	status: TerminalMountStatus;
	container: HTMLDivElement | null;
	active: boolean;
	pending: TerminalMountIntent | null;
	scheduled: boolean;
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
	const mountStates = new Map<string, TerminalMountState>();

	const scheduleMicrotask = (run: () => void): void => {
		if (typeof queueMicrotask === 'function') {
			queueMicrotask(run);
			return;
		}
		void Promise.resolve().then(run);
	};

	const getMountState = (id: string): TerminalMountState => {
		const existing = mountStates.get(id);
		if (existing) return existing;
		const created: TerminalMountState = {
			generation: 0,
			status: 'detached',
			container: null,
			active: false,
			pending: null,
			scheduled: false,
		};
		mountStates.set(id, created);
		return created;
	};

	const applyMountIntent = (id: string): void => {
		const state = getMountState(id);
		if (state.scheduled) return;
		state.scheduled = true;
		scheduleMicrotask(() => {
			state.scheduled = false;
			const intent = state.pending;
			state.pending = null;
			if (!intent) return;
			if (intent.generation !== state.generation) {
				deps.trace?.(id, 'mount_intent_stale', {
					intentGeneration: intent.generation,
					currentGeneration: state.generation,
					type: intent.type,
				});
				return;
			}
			if (intent.type === 'attach') {
				state.status = 'attaching';
				deps.attachTerminal(id, intent.container, intent.active);
				deps.attachResizeObserver(id, intent.container);
				state.status = 'attached';
				state.container = intent.container;
				state.active = intent.active;
				deps.trace?.(id, 'mount_apply_attach', {
					generation: intent.generation,
					active: intent.active,
				});
			} else {
				state.status = 'detaching';
				deps.markDetached(id);
				deps.detachResizeObserver(id);
				state.status = 'detached';
				state.container = null;
				state.active = false;
				deps.trace?.(id, 'mount_apply_detach', {
					generation: intent.generation,
				});
			}
			if (state.pending) {
				applyMountIntent(id);
			}
		});
	};

	const requestAttach = (id: string, container: HTMLDivElement, active: boolean): void => {
		const state = getMountState(id);
		if (state.status === 'attached' && state.container === container && state.active === active) {
			deps.trace?.(id, 'mount_skip_attached', {
				generation: state.generation,
				active,
			});
			return;
		}
		state.generation += 1;
		state.pending = {
			type: 'attach',
			generation: state.generation,
			container,
			active,
		};
		deps.trace?.(id, 'mount_request_attach', {
			generation: state.generation,
			active,
		});
		applyMountIntent(id);
	};

	const requestDetach = (id: string): void => {
		const state = getMountState(id);
		if (state.status === 'detached' && state.pending?.type !== 'attach') {
			deps.trace?.(id, 'mount_skip_detached', {
				generation: state.generation,
			});
			return;
		}
		state.generation += 1;
		state.pending = {
			type: 'detach',
			generation: state.generation,
		};
		deps.trace?.(id, 'mount_request_detach', {
			generation: state.generation,
		});
		applyMountIntent(id);
	};

	const syncTerminal = (input: TerminalSyncInput): void => {
		if (!input.terminalId || !input.workspaceId) return;
		deps.ensureGlobals();
		const terminalKey = deps.buildTerminalKey(input.workspaceId, input.terminalId);
		if (!terminalKey) return;
		let previous = lastSyncState.get(terminalKey);
		if (previous && deps.hasContext && !deps.hasContext(terminalKey)) {
			lastSyncState.delete(terminalKey);
			mountStates.delete(terminalKey);
			deps.trace?.(terminalKey, 'sync_terminal_clear_stale_state', {});
			previous = undefined;
		}
		const sameIdentity =
			previous !== undefined &&
			previous.workspaceId === input.workspaceId &&
			previous.terminalId === input.terminalId;
		if (
			sameIdentity &&
			previous &&
			previous.container === input.container &&
			previous.active === input.active
		) {
			deps.trace?.(terminalKey, 'sync_terminal_skip_unchanged', {
				hasContainer: Boolean(input.container),
				active: input.active,
			});
			return;
		}

		// During rapid tab/workspace switches the view can briefly report a null
		// container for a terminal that is still mounted/re-attaching. Preserve
		// the last non-null context so in-flight stream sync doesn't race against
		// a transient container-less update.
		if (!input.container && previous?.container) {
			deps.trace?.(terminalKey, 'sync_terminal_no_container_preserve_previous', {
				active: input.active,
				previousActive: previous.active,
			});
			deps.trace?.(terminalKey, 'sync_terminal_skip_stream_no_container', {
				active: input.active,
				hadContainer: true,
			});
			return;
		}

		deps.trace?.(terminalKey, 'sync_terminal_start', {
			workspaceId: input.workspaceId,
			terminalId: input.terminalId,
			hasContainer: Boolean(input.container),
			active: input.active,
		});
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
		if (input.container) {
			deps.trace?.(terminalKey, 'sync_terminal_attach', {
				active: input.active,
			});
			requestAttach(terminalKey, input.container, input.active);
			// Container element churn (same terminal identity + same active state) should
			// only move the mounted host node. Re-running stream/init on every container
			// replacement causes noisy reassert-start loops and focus/input instability.
			if (
				sameIdentity &&
				previous &&
				previous.container !== input.container &&
				previous.active === input.active
			) {
				deps.trace?.(terminalKey, 'sync_terminal_skip_stream_container_churn', {
					active: input.active,
				});
				return;
			}
		} else {
			deps.trace?.(terminalKey, 'sync_terminal_no_container', {
				active: input.active,
			});
			// Do not kick stream/init while there is no host element. Tab switches
			// can briefly produce a null container before the next pane mounts, and
			// that causes attach/init races against the subsequent real attach.
			deps.trace?.(terminalKey, 'sync_terminal_skip_stream_no_container', {
				active: input.active,
				hadContainer: Boolean(previous?.container),
			});
			return;
		}
		deps.syncTerminalStream(terminalKey);
		deps.trace?.(terminalKey, 'sync_terminal_stream_requested', {});
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
		deps.trace?.(terminalKey, 'detach_terminal', {
			workspaceId,
			terminalId,
			force,
		});
		requestDetach(terminalKey);
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
		deps.trace?.(terminalKey, 'detach_terminal_context_retained', {});
	};

	const closeTerminal = async (workspaceId: string, terminalId: string): Promise<void> => {
		const terminalKey = deps.buildTerminalKey(workspaceId, terminalId);
		if (!terminalKey) return;
		deps.trace?.(terminalKey, 'close_terminal_start', {
			workspaceId,
			terminalId,
		});
		requestDetach(terminalKey);
		try {
			await deps.stopTerminal(workspaceId, terminalId);
			deps.trace?.(terminalKey, 'close_terminal_stop_ok', {});
		} catch {
			deps.trace?.(terminalKey, 'close_terminal_stop_error', {});
			// Ignore failures.
		}
		deps.disposeTerminalResources(terminalKey);
		deps.deleteContext(terminalKey);
		lastSyncState.delete(terminalKey);
		mountStates.delete(terminalKey);
		deps.trace?.(terminalKey, 'close_terminal_done', {});
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
