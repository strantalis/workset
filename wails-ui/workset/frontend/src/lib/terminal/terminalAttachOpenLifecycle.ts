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
	nudgeRenderer?: (id: string, handle: THandle, rebuildAtlas: boolean) => void;
	traceAttach?: (id: string, event: string, details: Record<string, unknown>) => void;
};

export const createTerminalAttachOpenLifecycle = <THandle extends TerminalAttachOpenHandle>(
	deps: TerminalAttachOpenLifecycleDeps<THandle>,
) => {
	const attachSequences = new Map<string, number>();

	return {
		attach: (input: {
			id: string;
			handle: THandle;
			container: HTMLDivElement | null;
			active: boolean;
		}): void => {
			const { id, handle, container, active } = input;
			if (!container) return;
			const attachSeq = (attachSequences.get(id) ?? 0) + 1;
			attachSequences.set(id, attachSeq);

			const currentWindow = container.ownerDocument?.defaultView ?? null;
			const wasConnected = handle.container.isConnected;
			const wasActive = handle.container.getAttribute('data-active') === 'true';
			const containerChanged = container.firstElementChild !== handle.container;

			// KISS: exactly one mounted terminal host per pane container.
			if (containerChanged) {
				container.replaceChildren(handle.container);
			}
			handle.container.setAttribute('data-terminal-key', id);

			const terminalElement = handle.terminal.element;
			const movedAcrossWindows =
				handle.openWindow !== undefined && handle.openWindow !== currentWindow;
			const needsOpen =
				!terminalElement ||
				terminalElement.parentElement !== handle.container ||
				movedAcrossWindows;

			deps.traceAttach?.(id, 'attach_open_start', {
				attachSeq,
				active,
				wasConnected,
				movedAcrossWindows,
				needsOpen,
				containerConnected: container.isConnected,
				containerWidth: container.clientWidth,
				containerHeight: container.clientHeight,
			});

			if (needsOpen) {
				handle.container.replaceChildren();
				handle.terminal.open(handle.container);
				handle.openWindow = currentWindow;
			} else if (handle.openWindow === undefined) {
				handle.openWindow = currentWindow;
			}

			handle.container.setAttribute('data-active', active ? 'true' : 'false');
			const isRenderable =
				container.isConnected &&
				container.clientWidth > 0 &&
				container.clientHeight > 0 &&
				handle.container.isConnected &&
				handle.container.clientWidth > 0 &&
				handle.container.clientHeight > 0;
			if (isRenderable) {
				deps.fitTerminal(id);
			} else {
				deps.traceAttach?.(id, 'attach_fit_skip_not_renderable', {
					attachSeq,
					active,
					containerConnected: container.isConnected,
					containerWidth: container.clientWidth,
					containerHeight: container.clientHeight,
					hostConnected: handle.container.isConnected,
					hostWidth: handle.container.clientWidth,
					hostHeight: handle.container.clientHeight,
				});
			}

			const shouldNudgeRenderer = needsOpen || !wasConnected || active;
			if (shouldNudgeRenderer && isRenderable) {
				// Rebuild the WebGL texture atlas whenever the terminal transitions from
				// inactive→active or disconnected→connected. Without this, stale glyph
				// quads from a prior render surface linger after tab/pane switches and
				// produce corrupted glyphs until the next interaction-triggered redraw.
				const rebuildAtlas = needsOpen || !wasConnected || !wasActive || containerChanged;
				deps.traceAttach?.(id, 'attach_nudge', {
					attachSeq,
					active,
					needsOpen,
					wasConnected,
					rebuildAtlas,
				});
				deps.nudgeRenderer?.(id, handle, rebuildAtlas);
			} else {
				deps.traceAttach?.(id, 'attach_nudge_skip', {
					attachSeq,
					active,
					needsOpen,
					wasConnected,
					isRenderable,
				});
			}

			if (active && (needsOpen || !wasActive || !wasConnected)) {
				handle.terminal.focus();
			}

			deps.traceAttach?.(id, 'attach_open_done', {
				attachSeq,
				active,
				needsOpen,
				wasActive,
				wasConnected,
				hostConnected: handle.container.isConnected,
				hostWidth: handle.container.clientWidth,
				hostHeight: handle.container.clientHeight,
			});
			deps.flushOutput(id, false);
			deps.markAttached(id);
		},
	};
};
