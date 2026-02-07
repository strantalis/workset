import { captureViewportSnapshot, resolveViewportTargetLine } from './viewport';

export type TerminalViewportResizeHandle = {
	terminal: {
		buffer: {
			active: {
				baseY: number;
				viewportY: number;
			};
		};
		scrollToBottom: () => void;
		scrollToLine: (line: number) => void;
		focus: () => void;
	};
	fitAddon: {
		fit: () => void;
		proposeDimensions: () => { cols: number; rows: number } | undefined;
	};
};

type TerminalViewportSnapshot = ReturnType<typeof captureViewportSnapshot>;

type TerminalResizeObserver = {
	observe: (target: Element) => void;
	disconnect: () => void;
};

type TerminalViewportResizeControllerOptions<T extends TerminalViewportResizeHandle> = {
	getHandle: (id: string) => T | undefined;
	hasStarted: (id: string) => boolean;
	forceRedraw: (id: string) => void;
	resizeToFit: (id: string, handle: T) => void;
	resizeOverlay: (handle: T) => void;
	logDebug?: (id: string, event: string, details: Record<string, unknown>) => void;
	resizeDebounceMs?: number;
	setTimeoutFn?: (callback: () => void, timeoutMs: number) => number;
	clearTimeoutFn?: (handle: number) => void;
	createResizeObserver?: (onResize: () => void) => TerminalResizeObserver;
};

export const createTerminalViewportResizeController = <T extends TerminalViewportResizeHandle>(
	options: TerminalViewportResizeControllerOptions<T>,
) => {
	const fitStabilizers = new Map<string, number>();
	const resizeObservers = new Map<string, TerminalResizeObserver>();
	const resizeTimers = new Map<string, number>();
	const focusTimers = new Map<string, number>();
	const lastDims = new Map<string, { cols: number; rows: number }>();
	const resizeDebounceMs = options.resizeDebounceMs ?? 100;
	const setTimeoutFn =
		options.setTimeoutFn ??
		((callback: () => void, timeoutMs: number) => window.setTimeout(callback, timeoutMs));
	const clearTimeoutFn =
		options.clearTimeoutFn ?? ((handle: number) => window.clearTimeout(handle));
	const createResizeObserver =
		options.createResizeObserver ??
		((onResize: () => void) => new ResizeObserver(() => onResize()));

	const clearTimerMap = (map: Map<string, number>, id: string): void => {
		const timer = map.get(id);
		if (!timer) return;
		clearTimeoutFn(timer);
		map.delete(id);
	};

	const captureViewport = (terminal: T['terminal']): TerminalViewportSnapshot => {
		const buffer = terminal.buffer.active;
		return captureViewportSnapshot({
			baseY: buffer.baseY,
			viewportY: buffer.viewportY,
		});
	};

	const restoreViewport = (terminal: T['terminal'], viewport: TerminalViewportSnapshot): void => {
		const targetLine = resolveViewportTargetLine(viewport, terminal.buffer.active.baseY);
		if (targetLine === null) {
			terminal.scrollToBottom();
			return;
		}
		terminal.scrollToLine(targetLine);
	};

	const fitWithPreservedViewport = (
		handle: T,
		viewport = captureViewport(handle.terminal),
	): void => {
		handle.fitAddon.fit();
		restoreViewport(handle.terminal, viewport);
		options.resizeOverlay(handle);
	};

	const fitTerminal = (id: string, resizeSession: boolean): void => {
		const handle = options.getHandle(id);
		if (!handle) return;
		fitWithPreservedViewport(handle);
		options.forceRedraw(id);
		if (!resizeSession) return;
		options.resizeToFit(id, handle);
	};

	const scheduleFitStabilization = (id: string, reason: string): void => {
		clearTimerMap(fitStabilizers, id);
		let attempts = 0;
		let stableCount = 0;
		const run = (): void => {
			fitStabilizers.delete(id);
			const handle = options.getHandle(id);
			if (!handle) return;
			const dims = handle.fitAddon.proposeDimensions();
			if (!dims || dims.cols <= 0 || dims.rows <= 0) {
				attempts += 1;
				if (attempts < 6) {
					fitStabilizers.set(id, setTimeoutFn(run, 80 + attempts * 20));
				}
				return;
			}
			const prev = lastDims.get(id);
			if (prev && prev.cols === dims.cols && prev.rows === dims.rows) {
				stableCount += 1;
			} else {
				stableCount = 0;
			}
			lastDims.set(id, { cols: dims.cols, rows: dims.rows });
			fitTerminal(id, options.hasStarted(id));
			if (stableCount < 2 && attempts < 5) {
				attempts += 1;
				fitStabilizers.set(id, setTimeoutFn(run, 80 + attempts * 30));
			}
		};
		fitStabilizers.set(id, setTimeoutFn(run, 60));
		options.logDebug?.(id, 'fit', { reason });
	};

	const attachResizeObserver = (id: string, container: HTMLDivElement | null): void => {
		const existing = resizeObservers.get(id);
		if (existing) {
			existing.disconnect();
			resizeObservers.delete(id);
		}
		if (!container) return;
		const observer = createResizeObserver(() => {
			const existingTimer = resizeTimers.get(id);
			if (existingTimer) {
				clearTimeoutFn(existingTimer);
			}
			resizeTimers.set(
				id,
				setTimeoutFn(() => {
					resizeTimers.delete(id);
					const handle = options.getHandle(id);
					if (!handle) return;
					fitTerminal(id, options.hasStarted(id));
				}, resizeDebounceMs),
			);
		});
		observer.observe(container);
		resizeObservers.set(id, observer);
	};

	const detachResizeObserver = (id: string): void => {
		const observer = resizeObservers.get(id);
		if (observer) {
			observer.disconnect();
			resizeObservers.delete(id);
		}
		clearTimerMap(resizeTimers, id);
	};

	const focusTerminal = (id: string): void => {
		if (!id) return;
		const handle = options.getHandle(id);
		if (handle) {
			handle.terminal.focus();
			return;
		}
		if (focusTimers.has(id)) return;
		focusTimers.set(
			id,
			setTimeoutFn(() => {
				focusTimers.delete(id);
				const current = options.getHandle(id);
				current?.terminal.focus();
			}, 0),
		);
	};

	const scrollToBottom = (id: string): void => {
		const handle = options.getHandle(id);
		if (!handle) return;
		handle.terminal.scrollToBottom();
	};

	const isAtBottom = (id: string): boolean => {
		const handle = options.getHandle(id);
		if (!handle) return true;
		const buffer = handle.terminal.buffer.active;
		return buffer.baseY === buffer.viewportY;
	};

	const destroy = (id: string): void => {
		clearTimerMap(focusTimers, id);
		clearTimerMap(fitStabilizers, id);
		detachResizeObserver(id);
		lastDims.delete(id);
	};

	return {
		captureViewport,
		restoreViewport,
		fitWithPreservedViewport,
		fitTerminal,
		scheduleFitStabilization,
		attachResizeObserver,
		detachResizeObserver,
		focusTerminal,
		scrollToBottom,
		isAtBottom,
		destroy,
	};
};
