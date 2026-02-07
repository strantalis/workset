import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	createTerminalViewportResizeController,
	type TerminalViewportResizeHandle,
} from './terminalViewportResizeController';

type Dimensions = { cols: number; rows: number } | undefined;

const createHandle = (input: {
	baseY: number;
	viewportY: number;
	nextBaseY?: number;
	nextDimensions?: () => Dimensions;
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
		scrollToLine: vi.fn((line: number) => {
			buffer.active.viewportY = line;
		}),
		focus: vi.fn(),
	};
	const fitAddon: TerminalViewportResizeHandle['fitAddon'] = {
		fit: vi.fn(() => {
			buffer.active.baseY = input.nextBaseY ?? input.baseY;
		}),
		proposeDimensions: vi.fn(() => input.nextDimensions?.() ?? { cols: 80, rows: 24 }),
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

	it('fits with preserved viewport when user is scrolled up', () => {
		const handle = createHandle({ baseY: 120, viewportY: 90, nextBaseY: 260 });
		const controller = createTerminalViewportResizeController({
			getHandle: () => handle,
			hasStarted: () => true,
			forceRedraw: vi.fn(),
			resizeToFit: vi.fn(),
			resizeOverlay: vi.fn(),
		});

		controller.fitWithPreservedViewport(handle);

		expect(handle.fitAddon.fit).toHaveBeenCalledTimes(1);
		expect(handle.terminal.scrollToLine).toHaveBeenCalledWith(90);
		expect(handle.terminal.scrollToBottom).not.toHaveBeenCalled();
	});

	it('fits in follow mode at bottom', () => {
		const handle = createHandle({ baseY: 120, viewportY: 120, nextBaseY: 260 });
		const controller = createTerminalViewportResizeController({
			getHandle: () => handle,
			hasStarted: () => true,
			forceRedraw: vi.fn(),
			resizeToFit: vi.fn(),
			resizeOverlay: vi.fn(),
		});

		controller.fitWithPreservedViewport(handle);

		expect(handle.terminal.scrollToBottom).toHaveBeenCalledTimes(1);
		expect(handle.terminal.scrollToLine).not.toHaveBeenCalled();
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

	it('stabilizes fit dimensions with retries and logs reason', () => {
		const id = 'ws::term';
		const dimsQueue: Dimensions[] = [
			undefined,
			{ cols: 0, rows: 24 },
			{ cols: 120, rows: 40 },
			{ cols: 120, rows: 40 },
			{ cols: 120, rows: 40 },
		];
		const handle = createHandle({
			baseY: 10,
			viewportY: 10,
			nextDimensions: () => dimsQueue.shift() ?? { cols: 120, rows: 40 },
		});
		const resizeToFit = vi.fn();
		const forceRedraw = vi.fn();
		const logDebug = vi.fn();
		const controller = createTerminalViewportResizeController({
			getHandle: (key) => (key === id ? handle : undefined),
			hasStarted: () => true,
			forceRedraw,
			resizeToFit,
			resizeOverlay: vi.fn(),
			logDebug,
		});

		controller.scheduleFitStabilization(id, 'open');
		vi.runAllTimers();

		expect(handle.fitAddon.fit).toHaveBeenCalledTimes(3);
		expect(forceRedraw).toHaveBeenCalledTimes(3);
		expect(resizeToFit).toHaveBeenCalledTimes(3);
		expect(logDebug).toHaveBeenCalledWith(id, 'fit', { reason: 'open' });
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
