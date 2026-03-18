import { type TerminalAttachOpenHandle } from './terminalEmulatorContracts';

export type { TerminalAttachOpenHandle } from './terminalEmulatorContracts';

type TerminalAttachOpenLifecycleDeps = {
	fitTerminal: (id: string) => void;
	flushOutput: (id: string, writeAll: boolean) => void;
	markAttached: (id: string) => void;
	traceAttach?: (id: string, event: string, details: Record<string, unknown>) => void;
};

export const createTerminalAttachOpenLifecycle = <THandle extends TerminalAttachOpenHandle>(
	deps: TerminalAttachOpenLifecycleDeps,
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

			const movedAcrossWindows =
				handle.openWindow !== undefined && handle.openWindow !== currentWindow;
			const needsOpen = handle.opened !== true;

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
				handle.opened = true;
				// Disable the canvas-rendered scrollbar for a cleaner look.
				const renderer = (handle.terminal as { renderer?: { renderScrollbarEnabled?: boolean } })
					.renderer;
				if (renderer) {
					renderer.renderScrollbarEnabled = false;
				}
			}
			handle.openWindow = currentWindow;

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
