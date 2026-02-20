type TerminalSyncContext = {
	container: HTMLDivElement | null;
	active: boolean;
};

type TerminalStreamOrchestratorDependencies = {
	initTerminal: (id: string) => Promise<void>;
	getContext: (id: string) => TerminalSyncContext | null;
	beginTerminal: (id: string, quiet?: boolean) => Promise<void>;
	nextSyncToken: (id: string) => number;
	isCurrentSyncToken: (id: string, token: number) => boolean;
	emitState: (id: string) => void;
	trace?: (id: string, event: string, details: Record<string, unknown>) => void;
};

export const createTerminalStreamOrchestrator = (deps: TerminalStreamOrchestratorDependencies) => {
	const inFlight = new Set<string>();
	const pendingResync = new Set<string>();
	const coalescedCounts = new Map<string, number>();
	const lastActive = new Map<string, boolean>();

	const runSync = (id: string): void => {
		inFlight.add(id);
		const token = deps.nextSyncToken(id);
		const coalescedCount = coalescedCounts.get(id) ?? 0;
		coalescedCounts.delete(id);
		deps.trace?.(id, 'stream_sync_start', { token, coalescedCount });
		void (async () => {
			try {
				await deps.initTerminal(id);
			} catch (error) {
				deps.trace?.(id, 'stream_sync_init_error', {
					token,
					error: String(error),
				});
				return;
			}
			if (!deps.isCurrentSyncToken(id, token)) {
				deps.trace?.(id, 'stream_sync_stale_after_init', { token });
				return;
			}
			const current = deps.getContext(id);
			deps.trace?.(id, 'stream_sync_context', {
				token,
				hasContainer: Boolean(current?.container),
				active: current?.active ?? false,
			});
			if (current?.container) {
				const wasActive = lastActive.get(id) ?? false;
				const becameActive = current.active && !wasActive;
				lastActive.set(id, current.active);
				const quiet = !becameActive;
				deps.trace?.(id, 'stream_sync_begin_request', {
					token,
					quiet,
					wasActive,
					active: current.active,
					becameActive,
				});
				try {
					await deps.beginTerminal(id, quiet);
				} catch (error) {
					deps.trace?.(id, 'stream_sync_begin_error', {
						token,
						error: String(error),
					});
					return;
				}
			}
			if (!deps.isCurrentSyncToken(id, token)) {
				deps.trace?.(id, 'stream_sync_stale_after_begin', { token });
				return;
			}
			deps.emitState(id);
			deps.trace?.(id, 'stream_sync_done', { token });
		})().finally(() => {
			inFlight.delete(id);
			if (pendingResync.delete(id)) {
				runSync(id);
			}
		});
	};

	const syncTerminalStream = (id: string): void => {
		if (inFlight.has(id)) {
			pendingResync.add(id);
			const coalescedCount = (coalescedCounts.get(id) ?? 0) + 1;
			coalescedCounts.set(id, coalescedCount);
			deps.trace?.(id, 'stream_sync_coalesced', {
				coalescedCount,
			});
			return;
		}
		runSync(id);
	};

	return {
		syncTerminalStream,
	};
};
