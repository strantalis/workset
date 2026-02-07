type TerminalLineLike = {
	translateToString: () => string;
};

type TerminalBufferLike = {
	length: number;
	getLine: (index: number) => TerminalLineLike | undefined;
};

type TerminalLike = {
	open: (container: HTMLDivElement) => void;
	refresh: (start: number, end: number) => void;
	rows: number;
	buffer: {
		active: TerminalBufferLike;
	};
};

export type TerminalRenderHealthHandle = {
	terminal: TerminalLike;
	container?: HTMLDivElement | null;
};

type RenderStats = {
	lastRenderAt: number;
	renderCount: number;
};

type TerminalRenderHealthDeps<THandle extends TerminalRenderHealthHandle> = {
	getHandle: (id: string) => THandle | undefined;
	reopenWithPreservedViewport: (id: string, handle: THandle) => void;
	fitWithPreservedViewport: (id: string, handle: THandle) => void;
	nudgeRedraw: (id: string, handle: THandle) => void;
	logDebug: (id: string, event: string, details: Record<string, unknown>) => void;
	setTimeoutFn?: (callback: () => void, delayMs: number) => number;
	clearTimeoutFn?: (timeoutId: number) => void;
	nowFn?: () => number;
	renderCheckDelayMs?: number;
	renderRecoveryDelayMs?: number;
};

const DEFAULT_RENDER_CHECK_DELAY_MS = 350;
const DEFAULT_RENDER_RECOVERY_DELAY_MS = 150;

export const hasVisibleTerminalContent = (terminal: TerminalLike): boolean => {
	const buffer = terminal.buffer.active;
	if (buffer.length === 0) return false;
	const line = buffer.getLine(buffer.length - 1);
	return !!line && line.translateToString().trim().length > 0;
};

const defaultRenderStats = (): RenderStats => ({ lastRenderAt: 0, renderCount: 0 });

export const createTerminalRenderHealth = <THandle extends TerminalRenderHealthHandle>(
	deps: TerminalRenderHealthDeps<THandle>,
) => {
	const renderStats = new Map<string, RenderStats>();
	const pendingChecks = new Map<string, number>();
	const renderCheckLogged = new Set<string>();
	const reopenAttempted = new Set<string>();
	const setTimeoutFn =
		deps.setTimeoutFn ?? ((callback, delayMs) => window.setTimeout(callback, delayMs));
	const clearTimeoutFn = deps.clearTimeoutFn ?? ((timeoutId) => window.clearTimeout(timeoutId));
	const nowFn = deps.nowFn ?? (() => Date.now());
	const renderCheckDelayMs = deps.renderCheckDelayMs ?? DEFAULT_RENDER_CHECK_DELAY_MS;
	const renderRecoveryDelayMs = deps.renderRecoveryDelayMs ?? DEFAULT_RENDER_RECOVERY_DELAY_MS;

	const forceRedraw = (handle: THandle): void => {
		handle.terminal.refresh(0, handle.terminal.rows - 1);
	};

	const getOrCreateStats = (id: string): RenderStats => {
		let stats = renderStats.get(id);
		if (!stats) {
			stats = defaultRenderStats();
			renderStats.set(id, stats);
		}
		return stats;
	};

	const scheduleRenderCheck = (id: string): void => {
		const existing = pendingChecks.get(id);
		if (existing) {
			clearTimeoutFn(existing);
		}
		getOrCreateStats(id);
		pendingChecks.set(
			id,
			setTimeoutFn(() => {
				pendingChecks.delete(id);
				const stats = renderStats.get(id) ?? defaultRenderStats();
				if (nowFn() - stats.lastRenderAt < renderCheckDelayMs) return;
				if (!renderCheckLogged.has(id)) {
					renderCheckLogged.add(id);
					deps.logDebug(id, 'render_stall', { lastRenderAt: stats.lastRenderAt });
				}
				const handle = deps.getHandle(id);
				if (!handle) return;
				if (handle.container && !reopenAttempted.has(id)) {
					reopenAttempted.add(id);
					try {
						deps.reopenWithPreservedViewport(id, handle);
					} catch {
						// Best-effort re-open.
					}
				}
				forceRedraw(handle);
				setTimeoutFn(() => {
					const current = deps.getHandle(id);
					if (!current) return;
					if (!hasVisibleTerminalContent(current.terminal)) {
						deps.nudgeRedraw(id, current);
					}
				}, renderRecoveryDelayMs);
			}, renderCheckDelayMs),
		);
	};

	const noteRender = (id: string): void => {
		const stats = getOrCreateStats(id);
		stats.lastRenderAt = nowFn();
		stats.renderCount += 1;
		renderStats.set(id, stats);
	};

	const noteOutputActivity = (id: string): void => {
		const stats = getOrCreateStats(id);
		if (nowFn() - stats.lastRenderAt > renderCheckDelayMs) {
			scheduleRenderCheck(id);
		}
	};

	const scheduleBootstrapHealthCheck = (id: string, payloadBytes: number): void => {
		if (!id || payloadBytes <= 0 || pendingChecks.has(id)) return;
		const startedAt = nowFn();
		pendingChecks.set(
			id,
			setTimeoutFn(() => {
				pendingChecks.delete(id);
				const handle = deps.getHandle(id);
				if (!handle) return;
				const stats = renderStats.get(id);
				if (stats && stats.lastRenderAt >= startedAt) return;
				deps.fitWithPreservedViewport(id, handle);
				setTimeoutFn(() => {
					const updated = renderStats.get(id);
					if (updated && updated.lastRenderAt >= startedAt) return;
					if (!hasVisibleTerminalContent(handle.terminal)) {
						deps.nudgeRedraw(id, handle);
					}
					deps.logDebug(id, 'render_health_check', {
						rendered: updated ? updated.lastRenderAt >= startedAt : false,
					});
				}, renderRecoveryDelayMs);
			}, renderCheckDelayMs),
		);
	};

	const clearSession = (id: string): void => {
		const pending = pendingChecks.get(id);
		if (pending) {
			clearTimeoutFn(pending);
		}
		pendingChecks.delete(id);
		renderStats.delete(id);
	};

	const release = (id: string): void => {
		clearSession(id);
		renderCheckLogged.delete(id);
		reopenAttempted.delete(id);
	};

	return {
		noteRender,
		noteOutputActivity,
		scheduleBootstrapHealthCheck,
		clearSession,
		release,
	};
};
