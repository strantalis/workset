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
		const handle = createHandle();
		handle.terminal.element = { parentElement: handle.container };
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
		const handle = createHandle();
		handle.terminal.element = { parentElement: handle.container };
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
		const handle = createHandle();
		handle.terminal.element = { parentElement: handle.container };
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
});
