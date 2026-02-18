export type TerminalViewportResizeHandle = {
	terminal: {
		element?: {
			parentElement: Element | null;
		} | null;
		buffer: {
			active: {
				baseY: number;
				viewportY: number;
			};
		};
		scrollToBottom: () => void;
		focus: () => void;
	};
	fitAddon: {
		fit: () => void;
	};
};

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
	resizeDebounceMs?: number;
	setTimeoutFn?: (callback: () => void, timeoutMs: number) => number;
	clearTimeoutFn?: (handle: number) => void;
	createResizeObserver?: (onResize: () => void) => TerminalResizeObserver;
};

export const createTerminalViewportResizeController = <T extends TerminalViewportResizeHandle>(
	options: TerminalViewportResizeControllerOptions<T>,
) => {
	const resizeObservers = new Map<string, TerminalResizeObserver>();
	const resizeTimers = new Map<string, number>();
	const focusRetryTimers = new Map<string, number>();
	const focusRefitTimers = new Map<string, number>();
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

	const hasRenderableViewport = (handle: T): boolean => {
		const parent = handle.terminal.element?.parentElement;
		if (!parent) {
			return true;
		}
		if (!(parent instanceof HTMLElement)) {
			return true;
		}
		if (!parent.isConnected) {
			return false;
		}
		return parent.clientWidth > 0 && parent.clientHeight > 0;
	};

	const fitTerminal = (id: string, resizeSession: boolean): void => {
		const handle = options.getHandle(id);
		if (!handle) return;
		if (!hasRenderableViewport(handle)) return;
		handle.fitAddon.fit();
		options.resizeOverlay(handle);
		options.forceRedraw(id);
		if (!resizeSession) return;
		options.resizeToFit(id, handle);
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
			fitTerminal(id, options.hasStarted(id));
			handle.terminal.focus();
			clearTimerMap(focusRefitTimers, id);
			focusRefitTimers.set(
				id,
				setTimeoutFn(() => {
					focusRefitTimers.delete(id);
					fitTerminal(id, options.hasStarted(id));
				}, 0),
			);
			return;
		}
		if (focusRetryTimers.has(id)) return;
		focusRetryTimers.set(
			id,
			setTimeoutFn(() => {
				focusRetryTimers.delete(id);
				const current = options.getHandle(id);
				if (!current) return;
				fitTerminal(id, options.hasStarted(id));
				current.terminal.focus();
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
		clearTimerMap(focusRetryTimers, id);
		clearTimerMap(focusRefitTimers, id);
		detachResizeObserver(id);
	};

	return {
		fitTerminal,
		attachResizeObserver,
		detachResizeObserver,
		focusTerminal,
		scrollToBottom,
		isAtBottom,
		destroy,
	};
};
