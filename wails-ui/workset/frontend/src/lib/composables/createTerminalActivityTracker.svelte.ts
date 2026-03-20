/**
 * Tracks which workspaces have recent terminal activity with TTL-based expiry.
 * Activity marks expire after `ttlMs` milliseconds, with deadline extension
 * support (re-marking before expiry extends the deadline).
 */
export type TerminalActivityTracker = {
	/** Record of active workspace IDs (keyed for O(1) lookup). */
	readonly active: Record<string, true>;
	/** Flat list of active workspace IDs (derived). */
	readonly activeIds: string[];
	/** Mark a workspace as having terminal activity. Starts/extends the expiry timer. */
	mark: (workspaceId: string | null | undefined) => void;
	/** Immediately clear terminal activity for a workspace. */
	clear: (workspaceId: string | null | undefined) => void;
	/** Clean up all timers. Call from onDestroy. */
	destroy: () => void;
};

export function createTerminalActivityTracker(ttlMs: number): TerminalActivityTracker {
	const expiryTimers = new Map<string, number>();
	const deadlines = new Map<string, number>();

	let active = $state<Record<string, true>>({});
	const activeIds = $derived(Object.keys(active));

	const clearTimer = (id: string): void => {
		const timer = expiryTimers.get(id);
		if (timer === undefined) return;
		window.clearTimeout(timer);
		expiryTimers.delete(id);
	};

	const remove = (id: string): void => {
		if (active[id] === undefined) return;
		const next = { ...active };
		delete next[id];
		active = next;
	};

	const scheduleExpiry = (id: string, delayMs: number): void => {
		clearTimer(id);
		const timer = window.setTimeout(
			() => {
				expiryTimers.delete(id);
				const deadline = deadlines.get(id);
				if (deadline === undefined) {
					remove(id);
					return;
				}
				const remainingMs = deadline - Date.now();
				if (remainingMs > 0) {
					scheduleExpiry(id, remainingMs);
					return;
				}
				deadlines.delete(id);
				remove(id);
			},
			Math.max(0, delayMs),
		);
		expiryTimers.set(id, timer);
	};

	const mark = (workspaceId: string | null | undefined): void => {
		const id = workspaceId?.trim() ?? '';
		if (!id) return;
		const expiresAt = Date.now() + ttlMs;
		deadlines.set(id, expiresAt);
		if (active[id] === undefined) {
			active = { ...active, [id]: true };
			scheduleExpiry(id, ttlMs);
			return;
		}
		if (!expiryTimers.has(id)) {
			scheduleExpiry(id, ttlMs);
		}
	};

	const clear = (workspaceId: string | null | undefined): void => {
		const id = workspaceId?.trim() ?? '';
		if (!id) return;
		clearTimer(id);
		deadlines.delete(id);
		remove(id);
	};

	const destroy = (): void => {
		for (const timer of expiryTimers.values()) {
			window.clearTimeout(timer);
		}
		expiryTimers.clear();
		deadlines.clear();
	};

	return {
		get active() {
			return active;
		},
		get activeIds() {
			return activeIds;
		},
		mark,
		clear,
		destroy,
	};
}
