import { describe, expect, it, vi } from 'vitest';
import {
	createTerminalInstanceManager,
	type TerminalInstanceHandle,
} from './terminalInstanceManager';

type Disposable = { dispose: ReturnType<typeof vi.fn> };
type ListenerState = {
	dataCallbacks: Array<(data: string) => void>;
	binaryCallbacks: Array<(data: string) => void>;
	dataDisposables: Disposable[];
	binaryDisposables: Disposable[];
};

const castHandleField = <T>(value: unknown): T => value as T;

const createTerminalMock = (state: ListenerState) => ({
	loadAddon: vi.fn(),
	attachCustomWheelEventHandler: vi.fn(),
	onData: vi.fn((callback: (data: string) => void) => {
		const disposable = { dispose: vi.fn() };
		state.dataCallbacks.push(callback);
		state.dataDisposables.push(disposable);
		return disposable;
	}),
	onBinary: vi.fn((callback: (data: string) => void) => {
		const disposable = { dispose: vi.fn() };
		state.binaryCallbacks.push(callback);
		state.binaryDisposables.push(disposable);
		return disposable;
	}),
	onRender: vi.fn(() => ({ dispose: vi.fn() })),
	dispose: vi.fn(),
});

describe('terminalInstanceManager', () => {
	it('creates and attaches a new terminal handle with expected listener wiring', () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const dataCallbacks: Array<(data: string) => void> = [];
		const binaryCallbacks: Array<(data: string) => void> = [];
		const dataDisposables: Disposable[] = [];
		const binaryDisposables: Disposable[] = [];
		const terminal = createTerminalMock({
			dataCallbacks,
			binaryCallbacks,
			dataDisposables,
			binaryDisposables,
		});
		const createTerminalInstance = vi.fn(() => terminal);
		const fitAddon = { fit: vi.fn(), proposeDimensions: vi.fn() };
		const searchAddon = { activate: vi.fn(), dispose: vi.fn() };
		const webLinksAddon = { activate: vi.fn(), dispose: vi.fn() };
		const imageAddon = { activate: vi.fn(), dispose: vi.fn() };
		const webglAddon = {
			activate: vi.fn(),
			dispose: vi.fn(),
			onContextLoss: vi.fn(() => ({ dispose: vi.fn() })),
		};
		const onData = vi.fn();
		const onRendererResolved = vi.fn();
		const attachOpen = vi.fn();
		const manager = createTerminalInstanceManager({
			terminalHandles,
			createTerminalInstance: () =>
				castHandleField<TerminalInstanceHandle['terminal']>(createTerminalInstance()),
			createFitAddon: () => castHandleField<TerminalInstanceHandle['fitAddon']>(fitAddon),
			createSearchAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['searchAddon']>>(searchAddon),
			createWebLinksAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['webLinksAddon']>>(webLinksAddon),
			createImageAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['imageAddon']>>(imageAddon),
			createWebglAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['webglAddon']>>(webglAddon),
			onData,
			onRendererResolved,
			attachOpen,
		});
		const container = document.createElement('div') as HTMLDivElement;

		const handle = manager.attach('ws::term', container, true);

		expect(createTerminalInstance).toHaveBeenCalledTimes(1);
		expect(terminal.loadAddon).toHaveBeenNthCalledWith(1, fitAddon);
		expect(terminal.loadAddon).toHaveBeenNthCalledWith(2, searchAddon);
		expect(terminal.loadAddon).toHaveBeenNthCalledWith(3, webLinksAddon);
		expect(terminal.loadAddon).toHaveBeenNthCalledWith(4, imageAddon);
		expect(terminal.loadAddon).toHaveBeenNthCalledWith(5, webglAddon);
		expect(terminal.attachCustomWheelEventHandler).toHaveBeenCalledTimes(1);
		expect(terminal.onData).toHaveBeenCalledTimes(1);
		expect(terminal.onBinary).toHaveBeenCalledTimes(1);
		expect(attachOpen).toHaveBeenCalledWith({ id: 'ws::term', handle, container, active: true });
		expect(onRendererResolved).toHaveBeenCalledWith('ws::term', 'webgl');
		expect(terminalHandles.get('ws::term')).toBe(handle);
		dataCallbacks[0]?.('hello');
		expect(onData).toHaveBeenCalledWith('ws::term', 'hello');
		binaryCallbacks[0]?.('\x9b');
		expect(onData).toHaveBeenCalledWith('ws::term', '\x9b');
	});

	it('reuses an existing handle and avoids rebinding data listeners on reattach', () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const dataCallbacks: Array<(data: string) => void> = [];
		const binaryCallbacks: Array<(data: string) => void> = [];
		const dataDisposables: Disposable[] = [];
		const binaryDisposables: Disposable[] = [];
		const terminal = createTerminalMock({
			dataCallbacks,
			binaryCallbacks,
			dataDisposables,
			binaryDisposables,
		});
		const createTerminalInstance = vi.fn(() => terminal);
		const manager = createTerminalInstanceManager({
			terminalHandles,
			createTerminalInstance: () =>
				castHandleField<TerminalInstanceHandle['terminal']>(createTerminalInstance()),
			createFitAddon: () =>
				castHandleField<TerminalInstanceHandle['fitAddon']>({
					fit: vi.fn(),
					proposeDimensions: vi.fn(),
				}),
			createSearchAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['searchAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createWebLinksAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['webLinksAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createImageAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['imageAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createWebglAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['webglAddon']>>({
					dispose: vi.fn(),
					activate: vi.fn(),
					onContextLoss: vi.fn(() => ({ dispose: vi.fn() })),
				}),
			onData: vi.fn(),
			attachOpen: vi.fn(),
		});
		const container = document.createElement('div') as HTMLDivElement;

		manager.attach('ws::term', container, false);
		manager.attach('ws::term', container, true);

		expect(createTerminalInstance).toHaveBeenCalledTimes(1);
		expect(terminal.onData).toHaveBeenCalledTimes(1);
		expect(terminal.onBinary).toHaveBeenCalledTimes(1);
		expect(terminal.loadAddon).toHaveBeenCalledTimes(5);
		expect(dataDisposables[0].dispose).not.toHaveBeenCalled();
		expect(binaryDisposables[0].dispose).not.toHaveBeenCalled();
	});

	it('still opens/attaches terminal host when webgl initialization fails', () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const state: ListenerState = {
			dataCallbacks: [],
			binaryCallbacks: [],
			dataDisposables: [],
			binaryDisposables: [],
		};
		const terminal = createTerminalMock(state);
		const attachOpen = vi.fn();
		const onRendererResolved = vi.fn();
		const onRendererError = vi.fn();
		const manager = createTerminalInstanceManager({
			terminalHandles,
			createTerminalInstance: () => castHandleField<TerminalInstanceHandle['terminal']>(terminal),
			createFitAddon: () =>
				castHandleField<TerminalInstanceHandle['fitAddon']>({
					fit: vi.fn(),
					proposeDimensions: vi.fn(),
				}),
			createSearchAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['searchAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createWebLinksAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['webLinksAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createImageAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['imageAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createWebglAddon: () => {
				throw new Error('webgl unavailable');
			},
			onData: vi.fn(),
			attachOpen,
			onRendererResolved,
			onRendererError,
		});
		const container = document.createElement('div') as HTMLDivElement;

		const handle = manager.attach('ws::term', container, true);

		expect(attachOpen).toHaveBeenCalledWith({ id: 'ws::term', handle, container, active: true });
		expect(onRendererResolved).toHaveBeenCalledWith('ws::term', 'unknown');
		expect(onRendererError).toHaveBeenCalledWith('ws::term', 'webgl unavailable');
	});

	it('skips webgl initialization entirely when disabled', () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const state: ListenerState = {
			dataCallbacks: [],
			binaryCallbacks: [],
			dataDisposables: [],
			binaryDisposables: [],
		};
		const terminal = createTerminalMock(state);
		const createWebglAddon = vi.fn(() =>
			castHandleField<NonNullable<TerminalInstanceHandle['webglAddon']>>({
				dispose: vi.fn(),
				activate: vi.fn(),
			}),
		);
		const onRendererResolved = vi.fn();
		const onRendererError = vi.fn();
		const manager = createTerminalInstanceManager({
			terminalHandles,
			enableWebgl: false,
			createTerminalInstance: () => castHandleField<TerminalInstanceHandle['terminal']>(terminal),
			createFitAddon: () =>
				castHandleField<TerminalInstanceHandle['fitAddon']>({
					fit: vi.fn(),
					proposeDimensions: vi.fn(),
				}),
			createSearchAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['searchAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createWebLinksAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['webLinksAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createImageAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['imageAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createWebglAddon,
			onData: vi.fn(),
			attachOpen: vi.fn(),
			onRendererResolved,
			onRendererError,
		});
		const container = document.createElement('div') as HTMLDivElement;

		const handle = manager.attach('ws::term', container, true);

		expect(createWebglAddon).not.toHaveBeenCalled();
		expect(handle.renderer).toBe('unknown');
		expect(onRendererResolved).toHaveBeenCalledWith('ws::term', 'unknown');
		expect(onRendererError).not.toHaveBeenCalled();
	});

	it('skips image addon initialization when disabled', () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const state: ListenerState = {
			dataCallbacks: [],
			binaryCallbacks: [],
			dataDisposables: [],
			binaryDisposables: [],
		};
		const terminal = createTerminalMock(state);
		const imageAddon = { activate: vi.fn(), dispose: vi.fn() };
		const createImageAddon = vi.fn(() =>
			castHandleField<NonNullable<TerminalInstanceHandle['imageAddon']>>(imageAddon),
		);
		const manager = createTerminalInstanceManager({
			terminalHandles,
			enableImageAddon: false,
			createTerminalInstance: () => castHandleField<TerminalInstanceHandle['terminal']>(terminal),
			createFitAddon: () =>
				castHandleField<TerminalInstanceHandle['fitAddon']>({
					fit: vi.fn(),
					proposeDimensions: vi.fn(),
				}),
			createSearchAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['searchAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createWebLinksAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['webLinksAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createImageAddon,
			createWebglAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['webglAddon']>>({
					dispose: vi.fn(),
					activate: vi.fn(),
					onContextLoss: vi.fn(() => ({ dispose: vi.fn() })),
				}),
			onData: vi.fn(),
			attachOpen: vi.fn(),
		});
		const container = document.createElement('div') as HTMLDivElement;

		const handle = manager.attach('ws::term', container, true);

		expect(createImageAddon).not.toHaveBeenCalled();
		expect(handle.imageAddon).toBeUndefined();
		expect(terminal.loadAddon).toHaveBeenCalledTimes(4);
	});

	it('disposes terminal resources in order and removes handle', () => {
		const calls: string[] = [];
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		terminalHandles.set('ws::term', {
			terminal: castHandleField<TerminalInstanceHandle['terminal']>({
				dispose: vi.fn(() => calls.push('terminal')),
			}),
			fitAddon: castHandleField<TerminalInstanceHandle['fitAddon']>({}),
			renderer: 'webgl',
			dataDisposable: { dispose: vi.fn(() => calls.push('data')) },
			container: document.createElement('div') as HTMLDivElement,
		});
		const manager = createTerminalInstanceManager({
			terminalHandles,
			createTerminalInstance: () => castHandleField<TerminalInstanceHandle['terminal']>(vi.fn()),
			createFitAddon: () => castHandleField<TerminalInstanceHandle['fitAddon']>(vi.fn()),
			createSearchAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['searchAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createWebLinksAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['webLinksAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createImageAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['imageAddon']>>({
					activate: vi.fn(),
					dispose: vi.fn(),
				}),
			createWebglAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle['webglAddon']>>({
					dispose: vi.fn(),
					activate: vi.fn(),
				}),
			onData: vi.fn(),
			attachOpen: vi.fn(),
		});

		manager.dispose('ws::term');
		manager.dispose('missing');

		expect(calls).toEqual(['data', 'terminal']);
		expect(terminalHandles.has('ws::term')).toBe(false);
	});
});
