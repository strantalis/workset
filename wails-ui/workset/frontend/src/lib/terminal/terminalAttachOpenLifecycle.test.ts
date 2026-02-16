import { describe, expect, it, vi } from 'vitest';
import {
	createTerminalAttachOpenLifecycle,
	type TerminalAttachOpenHandle,
} from './terminalAttachOpenLifecycle';

const createHandle = (): TerminalAttachOpenHandle => {
	const host = document.createElement('div');
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
	};
};

describe('terminalAttachOpenLifecycle', () => {
	it('opens terminal and fits/resizes for the attached container', () => {
		const container = document.createElement('div') as HTMLDivElement;
		const handle = createHandle();
		const fitTerminal = vi.fn();
		const flushOutput = vi.fn();
		const markAttached = vi.fn();
		const nudgeRenderer = vi.fn();
		const lifecycle = createTerminalAttachOpenLifecycle({
			fitTerminal,
			flushOutput,
			markAttached,
			nudgeRenderer,
		});

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container,
			active: true,
		});

		expect(container.firstChild).toBe(handle.container);
		expect(handle.terminal.open).toHaveBeenCalledTimes(1);
		expect(fitTerminal).toHaveBeenCalledWith('ws::term');
		expect(nudgeRenderer).toHaveBeenCalledWith('ws::term', handle, true);
		expect(handle.terminal.focus).toHaveBeenCalledTimes(1);
		expect(flushOutput).toHaveBeenCalledWith('ws::term', false);
		expect(markAttached).toHaveBeenCalledWith('ws::term');
		expect(handle.container.getAttribute('data-active')).toBe('true');
	});

	it('skips open path when terminal is already opened in host container', () => {
		const container = document.createElement('div') as HTMLDivElement;
		document.body.append(container);
		const handle = createHandle();
		handle.terminal.element = { parentElement: handle.container };
		handle.openWindow = container.ownerDocument.defaultView;
		container.replaceChildren(handle.container);
		const fitTerminal = vi.fn();
		const flushOutput = vi.fn();
		const markAttached = vi.fn();
		const nudgeRenderer = vi.fn();
		const lifecycle = createTerminalAttachOpenLifecycle({
			fitTerminal,
			flushOutput,
			markAttached,
			nudgeRenderer,
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
		expect(nudgeRenderer).toHaveBeenCalledWith('ws::term', handle, false);
		expect(flushOutput).toHaveBeenCalledWith('ws::term', false);
		expect(markAttached).toHaveBeenCalledWith('ws::term');
		expect(handle.container.getAttribute('data-active')).toBe('false');
	});

	it('focuses once when transitioning inactive to active without reopening', () => {
		const container = document.createElement('div') as HTMLDivElement;
		document.body.append(container);
		const handle = createHandle();
		handle.terminal.element = { parentElement: handle.container };
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
		expect(handle.container.getAttribute('data-active')).toBe('true');
	});

	it('does not refocus when staying active on container churn', () => {
		const firstContainer = document.createElement('div') as HTMLDivElement;
		const secondContainer = document.createElement('div') as HTMLDivElement;
		document.body.append(firstContainer, secondContainer);
		const handle = createHandle();
		handle.terminal.element = { parentElement: handle.container };
		handle.openWindow = firstContainer.ownerDocument.defaultView;
		handle.container.setAttribute('data-active', 'true');
		firstContainer.replaceChildren(handle.container);
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
			container: secondContainer,
			active: true,
		});

		expect(secondContainer.firstChild).toBe(handle.container);
		expect(handle.terminal.open).not.toHaveBeenCalled();
		expect(handle.terminal.focus).not.toHaveBeenCalled();
	});

	it('refocuses without reopening after detach/reattach in the same window', () => {
		const firstContainer = document.createElement('div') as HTMLDivElement;
		const secondContainer = document.createElement('div') as HTMLDivElement;
		document.body.append(firstContainer, secondContainer);
		const handle = createHandle();
		handle.terminal.element = { parentElement: handle.container };
		handle.openWindow = firstContainer.ownerDocument.defaultView;
		handle.container.setAttribute('data-active', 'true');
		firstContainer.replaceChildren(handle.container);
		const fitTerminal = vi.fn();
		const flushOutput = vi.fn();
		const markAttached = vi.fn();
		const lifecycle = createTerminalAttachOpenLifecycle({
			fitTerminal,
			flushOutput,
			markAttached,
		});

		firstContainer.replaceChildren();
		expect(handle.container.isConnected).toBe(false);

		lifecycle.attach({
			id: 'ws::term',
			handle,
			container: secondContainer,
			active: true,
		});

		expect(handle.terminal.open).not.toHaveBeenCalled();
		expect(handle.terminal.focus).toHaveBeenCalledTimes(1);
		expect(secondContainer.firstChild).toBe(handle.container);
	});

	it('retries fit after detach/reattach in the same window', () => {
		vi.useFakeTimers();
		try {
			const firstContainer = document.createElement('div') as HTMLDivElement;
			const secondContainer = document.createElement('div') as HTMLDivElement;
			document.body.append(firstContainer, secondContainer);
			const handle = createHandle();
			handle.terminal.element = { parentElement: handle.container };
			handle.openWindow = firstContainer.ownerDocument.defaultView;
			handle.container.setAttribute('data-active', 'true');
			firstContainer.replaceChildren(handle.container);
			const fitTerminal = vi.fn();
			const flushOutput = vi.fn();
			const markAttached = vi.fn();
			const lifecycle = createTerminalAttachOpenLifecycle({
				fitTerminal,
				flushOutput,
				markAttached,
			});

			firstContainer.replaceChildren();
			expect(handle.container.isConnected).toBe(false);

			lifecycle.attach({
				id: 'ws::term',
				handle,
				container: secondContainer,
				active: true,
			});

			vi.advanceTimersByTime(600);
			expect(fitTerminal.mock.calls.length).toBeGreaterThan(1);
		} finally {
			vi.useRealTimers();
		}
	});

	it('reopens terminal when moved across browser windows', () => {
		const container = document.createElement('div') as HTMLDivElement;
		const handle = createHandle();
		handle.terminal.element = { parentElement: handle.container };
		handle.openWindow = {} as unknown as Window;
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

		expect(handle.terminal.open).toHaveBeenCalledTimes(1);
		expect(handle.openWindow).toBe(container.ownerDocument.defaultView);
	});

	it('retries fit after reopen so late-mounted popout containers self-render', () => {
		vi.useFakeTimers();
		try {
			const container = document.createElement('div') as HTMLDivElement;
			const handle = createHandle();
			handle.terminal.element = { parentElement: handle.container };
			handle.openWindow = {} as unknown as Window;
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
			expect(handle.terminal.open).toHaveBeenCalledTimes(1);

			vi.advanceTimersByTime(600);
			expect(fitTerminal.mock.calls.length).toBeGreaterThan(1);
		} finally {
			vi.useRealTimers();
		}
	});
});
