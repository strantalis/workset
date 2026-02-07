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
	getHandle: (id: string) => THandle | undefined;
	ensureOverlay: (handle: THandle, id: string) => void;
	loadRendererAddon: (id: string, handle: THandle) => void | Promise<void>;
	fitWithPreservedViewport: (handle: THandle) => void;
	resizeToFit: (id: string, handle: THandle) => void;
	scheduleFitStabilization: (id: string, reason: string) => void;
	flushOutput: (id: string, writeAll: boolean) => void;
	markAttached: (id: string) => void;
	resolveFontsReady?: () => Promise<unknown> | undefined;
};

const resolveFontsReady = (): Promise<unknown> | undefined => {
	if (typeof document === 'undefined') return undefined;
	return document.fonts?.ready;
};

export const createTerminalAttachOpenLifecycle = <THandle extends TerminalAttachOpenHandle>(
	deps: TerminalAttachOpenLifecycleDeps<THandle>,
) => {
	const getFontsReady = deps.resolveFontsReady ?? resolveFontsReady;

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
			if (needsOpen) {
				handle.container.replaceChildren();
				handle.terminal.open(handle.container);
				deps.ensureOverlay(handle, id);
				void deps.loadRendererAddon(id, handle);
				const fontsReady = getFontsReady();
				if (fontsReady) {
					fontsReady
						.then(() => {
							const current = deps.getHandle(id);
							if (!current) return;
							deps.fitWithPreservedViewport(current);
							deps.resizeToFit(id, current);
						})
						.catch(() => undefined);
				}
				deps.scheduleFitStabilization(id, 'open');
			}
			handle.container.setAttribute('data-active', 'true');
			deps.fitWithPreservedViewport(handle);
			deps.resizeToFit(id, handle);
			if (active) {
				handle.terminal.focus();
			}
			deps.flushOutput(id, false);
			deps.markAttached(id);
		},
	};
};
