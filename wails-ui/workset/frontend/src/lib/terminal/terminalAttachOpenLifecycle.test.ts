import { describe, expect, it, vi } from 'vitest';
import {
	createTerminalAttachOpenLifecycle,
	type TerminalAttachOpenHandle,
} from './terminalAttachOpenLifecycle';

const createHandle = (): TerminalAttachOpenHandle => {
	const host = document.createElement('div');
	host.className = 'terminal-instance';
	const terminal: TerminalAttachOpenHandle['terminal'] = {
		element: null,
		open: vi.fn((container: HTMLElement) => {
			terminal.element = { parentElement: container };
		}),
		focus: vi.fn(),
	};
	return {
		terminal,
		container: host,
		opened: false,
	};
};

const setElementSize = (element: HTMLElement, width: number, height: number): void => {
	Object.defineProperty(element, 'clientWidth', {
		configurable: true,
		get: () => width,
	});
	Object.defineProperty(element, 'clientHeight', {
		configurable: true,
		get: () => height,
	});
};

describe('terminalAttachOpenLifecycle', () => {
	it('mounts a single host, opens terminal, fits, and focuses when active', () => {
		const container = document.createElement('div') as HTMLDivElement;
		document.body.append(container);
		container.append(document.createElement('div'));
		const handle = createHandle();
		setElementSize(container, 800, 600);
		setElementSize(handle.container, 800, 600);
		const fitTerminal = vi.fn();
		const flushOutput = vi.fn();
		const markAttached = vi.fn();
		const lifecycle = createTerminalAttachOpenLifecycle({
			fitTerminal,
			flushOutput,
			markAttached,
		});

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container,
			active: true,
		});

		expect(container.childElementCount).toBe(1);
		expect(container.firstElementChild).toBe(handle.container);
		expect(handle.terminal.open).toHaveBeenCalledTimes(1);
		expect(fitTerminal).toHaveBeenCalledWith('ws::term');
		expect(handle.terminal.focus).toHaveBeenCalledTimes(1);
		expect(flushOutput).toHaveBeenCalledWith('ws::term', false);
		expect(markAttached).toHaveBeenCalledWith('ws::term');
	});

	it('does not reopen when host is already mounted and inactive', () => {
		const container = document.createElement('div') as HTMLDivElement;
		document.body.append(container);
		const handle = createHandle();
		setElementSize(container, 800, 600);
		setElementSize(handle.container, 800, 600);
		handle.terminal.element = { parentElement: handle.container };
		handle.opened = true;
		handle.openWindow = container.ownerDocument.defaultView;
		container.replaceChildren(handle.container);
		const fitTerminal = vi.fn();
		const flushOutput = vi.fn();
		const markAttached = vi.fn();
		const lifecycle = createTerminalAttachOpenLifecycle({
			fitTerminal,
			flushOutput,
			markAttached,
		});

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container,
			active: false,
		});

		expect(handle.terminal.open).not.toHaveBeenCalled();
		expect(handle.terminal.focus).not.toHaveBeenCalled();
		expect(fitTerminal).toHaveBeenCalledWith('ws::term');
	});

	it('focuses on inactive -> active transition', () => {
		const container = document.createElement('div') as HTMLDivElement;
		document.body.append(container);
		const handle = createHandle();
		setElementSize(container, 800, 600);
		setElementSize(handle.container, 800, 600);
		handle.terminal.element = { parentElement: handle.container };
		handle.opened = true;
		handle.openWindow = container.ownerDocument.defaultView;
		handle.container.setAttribute('data-active', 'false');
		container.replaceChildren(handle.container);
		const fitTerminal = vi.fn();
		const flushOutput = vi.fn();
		const markAttached = vi.fn();
		const lifecycle = createTerminalAttachOpenLifecycle({
			fitTerminal,
			flushOutput,
			markAttached,
		});

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container,
			active: true,
		});

		expect(handle.terminal.open).not.toHaveBeenCalled();
		expect(handle.terminal.focus).toHaveBeenCalledTimes(1);
	});

	it('keeps one host in destination container on reattach and does not reopen same-window host', () => {
		const firstContainer = document.createElement('div') as HTMLDivElement;
		const secondContainer = document.createElement('div') as HTMLDivElement;
		document.body.append(firstContainer, secondContainer);
		const handle = createHandle();
		setElementSize(secondContainer, 800, 600);
		setElementSize(handle.container, 800, 600);
		handle.terminal.element = { parentElement: handle.container };
		handle.opened = true;
		handle.openWindow = firstContainer.ownerDocument.defaultView;
		firstContainer.replaceChildren(handle.container);
		const fitTerminal = vi.fn();
		const lifecycle = createTerminalAttachOpenLifecycle({
			fitTerminal,
			flushOutput: vi.fn(),
			markAttached: vi.fn(),
		});

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container: secondContainer,
			active: true,
		});

		expect(secondContainer.childElementCount).toBe(1);
		expect(secondContainer.firstElementChild).toBe(handle.container);
		expect(handle.terminal.open).not.toHaveBeenCalled();
		expect(fitTerminal).toHaveBeenCalledTimes(1);
	});

	it('moves host across different windows without reopening terminal', () => {
		const container = document.createElement('div') as HTMLDivElement;
		const handle = createHandle();
		handle.terminal.element = { parentElement: handle.container };
		handle.opened = true;
		handle.openWindow = {} as unknown as Window;
		container.replaceChildren(handle.container);
		const lifecycle = createTerminalAttachOpenLifecycle({
			fitTerminal: vi.fn(),
			flushOutput: vi.fn(),
			markAttached: vi.fn(),
		});

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container,
			active: false,
		});

		expect(handle.terminal.open).not.toHaveBeenCalled();
		expect(handle.openWindow).toBe(container.ownerDocument.defaultView);
	});

	it('skips fit when host/container are not renderable yet', () => {
		const container = document.createElement('div') as HTMLDivElement;
		const handle = createHandle();
		setElementSize(container, 0, 0);
		setElementSize(handle.container, 0, 0);
		const fitTerminal = vi.fn();
		const traceAttach = vi.fn();
		const lifecycle = createTerminalAttachOpenLifecycle({
			fitTerminal,
			flushOutput: vi.fn(),
			markAttached: vi.fn(),
			traceAttach,
		});

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container,
			active: true,
		});

		expect(fitTerminal).not.toHaveBeenCalled();
		expect(traceAttach).toHaveBeenCalledWith(
			'ws::term',
			'attach_fit_skip_not_renderable',
			expect.objectContaining({
				containerWidth: 0,
				containerHeight: 0,
			}),
		);
	});
});
