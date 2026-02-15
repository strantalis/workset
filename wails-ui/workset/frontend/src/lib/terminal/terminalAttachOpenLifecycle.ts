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
};

type TerminalAttachOpenLifecycleDeps<THandle extends TerminalAttachOpenHandle> = {
	fitTerminal: (id: string) => void;
	flushOutput: (id: string, writeAll: boolean) => void;
	markAttached: (id: string) => void;
	nudgeRenderer?: (id: string, handle: THandle, opened: boolean) => void;
};

export const createTerminalAttachOpenLifecycle = <THandle extends TerminalAttachOpenHandle>(
	deps: TerminalAttachOpenLifecycleDeps<THandle>,
) => {
	return {
		attach: (input: {
			id: string;
			handle: THandle;
			container: HTMLDivElement | null;
			active: boolean;
		}): void => {
			const { id, handle, container, active } = input;
			if (!container) return;
			if (container.firstChild !== handle.container) {
				container.replaceChildren(handle.container);
			}
			const terminalElement = handle.terminal.element;
			const needsOpen = !terminalElement || terminalElement.parentElement !== handle.container;
			const wasActive = handle.container.getAttribute('data-active') === 'true';
			if (needsOpen) {
				handle.container.replaceChildren();
				handle.terminal.open(handle.container);
			}
			handle.container.setAttribute('data-active', active ? 'true' : 'false');
			deps.fitTerminal(id);
			deps.nudgeRenderer?.(id, handle, needsOpen);
			if (active && (needsOpen || !wasActive)) {
				handle.terminal.focus();
			}
			deps.flushOutput(id, false);
			deps.markAttached(id);
		},
	};
};
