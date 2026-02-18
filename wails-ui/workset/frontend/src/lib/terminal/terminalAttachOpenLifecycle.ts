type TerminalElementLike = {
	parentElement: Element | null;
};

type TerminalLike = {
	element: TerminalElementLike | null | undefined;
	open: (container: HTMLElement) => void;
	focus: () => void;
};

export type TerminalAttachOpenHandle = {
	terminal: TerminalLike;
	container: HTMLDivElement;
	openWindow?: Window | null;
};

type TerminalAttachOpenLifecycleDeps<THandle extends TerminalAttachOpenHandle> = {
	fitTerminal: (id: string) => void;
	flushOutput: (id: string, writeAll: boolean) => void;
	markAttached: (id: string) => void;
	nudgeRenderer?: (id: string, handle: THandle, opened: boolean) => void;
	traceAttach?: (id: string, event: string, details: Record<string, unknown>) => void;
};

export const createTerminalAttachOpenLifecycle = <THandle extends TerminalAttachOpenHandle>(
	deps: TerminalAttachOpenLifecycleDeps<THandle>,
) => {
	const deferredFitTimers = new Map<string, Array<{ win: Window; handle: number }>>();
	const fitRetryDelaysMs = [0, 16, 48, 120, 260, 520];

	const clearDeferredFits = (id: string): void => {
		const timers = deferredFitTimers.get(id);
		if (!timers) return;
		for (const timer of timers) {
			timer.win.clearTimeout(timer.handle);
		}
		deferredFitTimers.delete(id);
	};

	const scheduleDeferredFits = (id: string, fitWindow: Window | null): void => {
		clearDeferredFits(id);
		if (!fitWindow) return;
		const timers: Array<{ win: Window; handle: number }> = [];
		for (const delayMs of fitRetryDelaysMs) {
			const handle = fitWindow.setTimeout(() => {
				deps.fitTerminal(id);
			}, delayMs);
			timers.push({ win: fitWindow, handle });
		}
		deferredFitTimers.set(id, timers);
	};

	return {
		attach: (input: {
			id: string;
			handle: THandle;
			container: HTMLDivElement | null;
			active: boolean;
		}): void => {
			const { id, handle, container, active } = input;
			if (!container) return;
			const currentWindow = container.ownerDocument?.defaultView ?? null;
			const wasConnected = handle.container.isConnected;
			if (container.firstChild !== handle.container) {
				container.replaceChildren(handle.container);
			}
			const terminalElement = handle.terminal.element;
			const movedAcrossWindows =
				handle.openWindow !== undefined && handle.openWindow !== currentWindow;
			const needsOpen =
				!terminalElement ||
				terminalElement.parentElement !== handle.container ||
				movedAcrossWindows;
			deps.traceAttach?.(id, 'attach_open_start', {
				active,
				wasConnected,
				movedAcrossWindows,
				needsOpen,
				currentWindow: currentWindow?.name ?? '',
				openWindow: handle.openWindow?.name ?? '',
				containerConnected: container.isConnected,
				containerWidth: container.clientWidth,
				containerHeight: container.clientHeight,
			});
			const wasActive = handle.container.getAttribute('data-active') === 'true';
			if (needsOpen) {
				handle.container.replaceChildren();
				handle.terminal.open(handle.container);
				handle.openWindow = currentWindow;
			} else if (handle.openWindow === undefined) {
				handle.openWindow = currentWindow;
			}
			handle.container.setAttribute('data-active', active ? 'true' : 'false');
			deps.fitTerminal(id);
			if (needsOpen || !wasConnected) {
				scheduleDeferredFits(id, currentWindow);
			}
			deps.nudgeRenderer?.(id, handle, needsOpen);
			if (active && (needsOpen || !wasActive || !wasConnected)) {
				handle.terminal.focus();
			}
			deps.traceAttach?.(id, 'attach_open_done', {
				active,
				needsOpen,
				wasActive,
				wasConnected,
				hostConnected: handle.container.isConnected,
				hostWidth: handle.container.clientWidth,
				hostHeight: handle.container.clientHeight,
				parentWindow: handle.container.ownerDocument?.defaultView?.name ?? '',
			});
			deps.flushOutput(id, false);
			deps.markAttached(id);
		},
	};
};
