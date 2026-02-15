import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	createTerminalViewportResizeController,
	type TerminalViewportResizeHandle,
} from './terminalViewportResizeController';

const createHandle = (input: {
	baseY: number;
	viewportY: number;
}): TerminalViewportResizeHandle => {
	const buffer = {
		active: {
			baseY: input.baseY,
			viewportY: input.viewportY,
		},
	};
	const terminal: TerminalViewportResizeHandle['terminal'] = {
		buffer,
		scrollToBottom: vi.fn(() => {
			buffer.active.viewportY = buffer.active.baseY;
		}),
		focus: vi.fn(),
	};
	const fitAddon: TerminalViewportResizeHandle['fitAddon'] = {
		fit: vi.fn(),
	};
	return {
		terminal,
		fitAddon,
	};
};

type MockObserver = {
	observe: (target: Element) => void;
	disconnect: () => void;
	observeSpy: ReturnType<typeof vi.fn>;
	disconnectSpy: ReturnType<typeof vi.fn>;
	trigger: () => void;
};

const createObserverFactory =
	(observers: MockObserver[]) =>
	(callback: () => void): MockObserver => {
		const observeSpy = vi.fn<(target: Element) => void>();
		const disconnectSpy = vi.fn<() => void>();
		const observer: MockObserver = {
			observe: (target: Element) => {
				observeSpy(target);
			},
			disconnect: () => {
				disconnectSpy();
			},
			observeSpy,
			disconnectSpy,
			trigger: () => {
				callback();
			},
		};
		observers.push(observer);
		return observer;
	};

describe('terminalViewportResizeController', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('fits terminal and resizes pty in one pass', () => {
		const id = 'ws::term';
		const handle = createHandle({ baseY: 10, viewportY: 10 });
		const resizeToFit = vi.fn();
		const forceRedraw = vi.fn();
		const resizeOverlay = vi.fn();
		const controller = createTerminalViewportResizeController({
			getHandle: (key) => (key === id ? handle : undefined),
			hasStarted: () => true,
			forceRedraw,
			resizeToFit,
			resizeOverlay,
		});

		controller.fitTerminal(id, true);

		expect(handle.fitAddon.fit).toHaveBeenCalledTimes(1);
		expect(resizeOverlay).toHaveBeenCalledWith(handle);
		expect(forceRedraw).toHaveBeenCalledTimes(1);
		expect(resizeToFit).toHaveBeenCalledWith(id, handle);
	});

	it('skips fit when terminal host is disconnected or zero-sized', () => {
		const id = 'ws::term';
		const handle = createHandle({ baseY: 10, viewportY: 10 });
		const resizeToFit = vi.fn();
		const forceRedraw = vi.fn();
		const resizeOverlay = vi.fn();
		const hiddenHost = document.createElement('div');
		handle.terminal.element = { parentElement: hiddenHost };
		const controller = createTerminalViewportResizeController({
			getHandle: (key) => (key === id ? handle : undefined),
			hasStarted: () => true,
			forceRedraw,
			resizeToFit,
			resizeOverlay,
		});

		controller.fitTerminal(id, true);

		expect(handle.fitAddon.fit).not.toHaveBeenCalled();
		expect(resizeOverlay).not.toHaveBeenCalled();
		expect(forceRedraw).not.toHaveBeenCalled();
		expect(resizeToFit).not.toHaveBeenCalled();
	});

	it('attaches and detaches resize observers with debounce', () => {
		const id = 'ws::term';
		const handle = createHandle({ baseY: 10, viewportY: 10 });
		const resizeToFit = vi.fn();
		const forceRedraw = vi.fn();
		const observers: MockObserver[] = [];
		const controller = createTerminalViewportResizeController({
			getHandle: (key) => (key === id ? handle : undefined),
			hasStarted: () => true,
			forceRedraw,
			resizeToFit,
			resizeOverlay: vi.fn(),
			resizeDebounceMs: 100,
			createResizeObserver: createObserverFactory(observers),
		});
		const container = document.createElement('div') as HTMLDivElement;

		controller.attachResizeObserver(id, container);
		expect(observers).toHaveLength(1);
		expect(observers[0].observeSpy).toHaveBeenCalledWith(container);

		observers[0].trigger();
		vi.advanceTimersByTime(50);
		observers[0].trigger();
		vi.advanceTimersByTime(99);
		expect(handle.fitAddon.fit).toHaveBeenCalledTimes(0);

		vi.advanceTimersByTime(1);
		expect(handle.fitAddon.fit).toHaveBeenCalledTimes(1);
		expect(forceRedraw).toHaveBeenCalledTimes(1);
		expect(resizeToFit).toHaveBeenCalledTimes(1);

		observers[0].trigger();
		controller.detachResizeObserver(id);
		expect(observers[0].disconnectSpy).toHaveBeenCalledTimes(1);
		vi.runAllTimers();
		expect(handle.fitAddon.fit).toHaveBeenCalledTimes(1);
	});

	it('focuses eagerly or with deferred retry and exposes bottom helpers', () => {
		const state: { handle?: TerminalViewportResizeHandle } = {};
		const controller = createTerminalViewportResizeController({
			getHandle: () => state.handle,
			hasStarted: () => true,
			forceRedraw: vi.fn(),
			resizeToFit: vi.fn(),
			resizeOverlay: vi.fn(),
		});

		controller.focusTerminal('ws::term');
		controller.focusTerminal('ws::term');

		state.handle = createHandle({ baseY: 15, viewportY: 9 });
		vi.runAllTimers();
		expect(state.handle.terminal.focus).toHaveBeenCalledTimes(1);

		controller.focusTerminal('ws::term');
		expect(state.handle.terminal.focus).toHaveBeenCalledTimes(2);

		expect(controller.isAtBottom('ws::term')).toBe(false);
		controller.scrollToBottom('ws::term');
		expect(controller.isAtBottom('ws::term')).toBe(true);
		expect(controller.isAtBottom('missing')).toBe(true);
	});
});
