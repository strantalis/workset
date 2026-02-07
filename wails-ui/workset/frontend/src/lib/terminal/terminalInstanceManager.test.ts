import { describe, expect, it, vi } from 'vitest';
import {
	createTerminalInstanceManager,
	type TerminalInstanceHandle,
} from './terminalInstanceManager';

type Disposable = { dispose: ReturnType<typeof vi.fn> };
type ListenerState = {
	dataCallbacks: Array<(data: string) => void>;
	binaryCallbacks: Array<(data: string) => void>;
	renderCallbacks: Array<() => void>;
	dataDisposables: Disposable[];
	binaryDisposables: Disposable[];
};

const castHandleField = <T>(value: unknown): T => value as T;

const createTerminalMock = (state: ListenerState) => ({
	loadAddon: vi.fn(),
	attachCustomKeyEventHandler: vi.fn(),
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
	onRender: vi.fn((callback: () => void) => {
		state.renderCallbacks.push(callback);
		return { dispose: vi.fn() };
	}),
	dispose: vi.fn(),
	unicode: { activeVersion: '' },
});

describe('terminalInstanceManager', () => {
	it('creates and attaches a new terminal handle with expected listener wiring', () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle<object>>();
		const dataCallbacks: Array<(data: string) => void> = [];
		const binaryCallbacks: Array<(data: string) => void> = [];
		const renderCallbacks: Array<() => void> = [];
		const dataDisposables: Disposable[] = [];
		const binaryDisposables: Disposable[] = [];
		const terminal = createTerminalMock({
			dataCallbacks,
			binaryCallbacks,
			renderCallbacks,
			dataDisposables,
			binaryDisposables,
		});
		const createTerminalInstance = vi.fn(() => terminal);
		const fitAddon = { fit: vi.fn(), proposeDimensions: vi.fn() };
		const unicode11Addon = { dispose: vi.fn() };
		const clipboardAddon = { dispose: vi.fn() };
		const onShiftEnter = vi.fn();
		const onData = vi.fn();
		const onBinary = vi.fn();
		const onRender = vi.fn();
		const attachOpen = vi.fn();
		const syncTerminalWebLinks = vi.fn();
		const registerOscHandlers = vi.fn(() => [{ dispose: vi.fn() }]);
		const ensureMode = vi.fn();
		const manager = createTerminalInstanceManager<object>({
			terminalHandles,
			createTerminalInstance: () =>
				castHandleField<TerminalInstanceHandle<object>['terminal']>(createTerminalInstance()),
			createFitAddon: () => castHandleField<TerminalInstanceHandle<object>['fitAddon']>(fitAddon),
			createUnicode11Addon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle<object>['unicode11Addon']>>(
					unicode11Addon,
				),
			createClipboardAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle<object>['clipboardAddon']>>(
					clipboardAddon,
				),
			createKittyState: () => ({ ready: true }),
			syncTerminalWebLinks,
			registerOscHandlers,
			ensureMode,
			onShiftEnter,
			onData,
			onBinary,
			onRender,
			attachOpen,
		});
		const container = document.createElement('div') as HTMLDivElement;

		const handle = manager.attach('ws::term', container, true);

		expect(createTerminalInstance).toHaveBeenCalledTimes(1);
		expect(terminal.loadAddon).toHaveBeenNthCalledWith(1, fitAddon);
		expect(terminal.loadAddon).toHaveBeenNthCalledWith(2, unicode11Addon);
		expect(terminal.loadAddon).toHaveBeenNthCalledWith(3, clipboardAddon);
		expect(terminal.unicode.activeVersion).toBe('11');
		expect(syncTerminalWebLinks).toHaveBeenCalledWith('ws::term');
		expect(registerOscHandlers).toHaveBeenCalledWith('ws::term', terminal);
		expect(ensureMode).toHaveBeenCalledWith('ws::term');
		expect(terminal.onData).toHaveBeenCalledTimes(2);
		expect(terminal.onBinary).toHaveBeenCalledTimes(2);
		expect(dataDisposables[0].dispose).toHaveBeenCalledTimes(1);
		expect(binaryDisposables[0].dispose).toHaveBeenCalledTimes(1);
		expect(attachOpen).toHaveBeenCalledWith({ id: 'ws::term', handle, container, active: true });
		expect(terminalHandles.get('ws::term')).toBe(handle);

		const keyHandler = terminal.attachCustomKeyEventHandler.mock.calls[0]?.[0] as
			| ((event: KeyboardEvent) => boolean)
			| undefined;
		expect(keyHandler).toBeTypeOf('function');
		expect(keyHandler?.({ key: 'Enter', shiftKey: true } as KeyboardEvent)).toBe(false);
		expect(onShiftEnter).toHaveBeenCalledWith('ws::term');
		expect(keyHandler?.({ key: 'a', shiftKey: false } as KeyboardEvent)).toBe(true);

		dataCallbacks[1]?.('hello');
		binaryCallbacks[1]?.('');
		binaryCallbacks[1]?.('\u0001');
		renderCallbacks[0]?.();
		expect(onData).toHaveBeenCalledWith('ws::term', 'hello');
		expect(onBinary).toHaveBeenCalledTimes(1);
		expect(onBinary).toHaveBeenCalledWith('ws::term', '\u0001');
		expect(onRender).toHaveBeenCalledWith('ws::term');
	});

	it('reuses an existing handle and rebinds data listeners on reattach', () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle<object>>();
		const dataCallbacks: Array<(data: string) => void> = [];
		const binaryCallbacks: Array<(data: string) => void> = [];
		const renderCallbacks: Array<() => void> = [];
		const dataDisposables: Disposable[] = [];
		const binaryDisposables: Disposable[] = [];
		const terminal = createTerminalMock({
			dataCallbacks,
			binaryCallbacks,
			renderCallbacks,
			dataDisposables,
			binaryDisposables,
		});
		const createTerminalInstance = vi.fn(() => terminal);
		const manager = createTerminalInstanceManager<object>({
			terminalHandles,
			createTerminalInstance: () =>
				castHandleField<TerminalInstanceHandle<object>['terminal']>(createTerminalInstance()),
			createFitAddon: () =>
				castHandleField<TerminalInstanceHandle<object>['fitAddon']>({
					fit: vi.fn(),
					proposeDimensions: vi.fn(),
				}),
			createUnicode11Addon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle<object>['unicode11Addon']>>({
					dispose: vi.fn(),
				}),
			createClipboardAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle<object>['clipboardAddon']>>({
					dispose: vi.fn(),
				}),
			createKittyState: () => ({ mode: 'test' }),
			syncTerminalWebLinks: vi.fn(),
			registerOscHandlers: vi.fn(() => []),
			ensureMode: vi.fn(),
			onShiftEnter: vi.fn(),
			onData: vi.fn(),
			onBinary: vi.fn(),
			onRender: vi.fn(),
			attachOpen: vi.fn(),
		});
		const container = document.createElement('div') as HTMLDivElement;

		manager.attach('ws::term', container, false);
		manager.attach('ws::term', container, true);

		expect(createTerminalInstance).toHaveBeenCalledTimes(1);
		expect(terminal.onData).toHaveBeenCalledTimes(3);
		expect(terminal.onBinary).toHaveBeenCalledTimes(3);
		expect(dataDisposables[1].dispose).toHaveBeenCalledTimes(1);
		expect(binaryDisposables[1].dispose).toHaveBeenCalledTimes(1);
	});

	it('disposes terminal resources in order and removes handle', () => {
		const calls: string[] = [];
		const terminalHandles = new Map<string, TerminalInstanceHandle<object>>();
		terminalHandles.set('ws::term', {
			terminal: castHandleField<TerminalInstanceHandle<object>['terminal']>({
				dispose: vi.fn(() => calls.push('terminal')),
			}),
			fitAddon: castHandleField<TerminalInstanceHandle<object>['fitAddon']>({}),
			dataDisposable: { dispose: vi.fn(() => calls.push('data')) },
			binaryDisposable: { dispose: vi.fn(() => calls.push('binary')) },
			container: document.createElement('div') as HTMLDivElement,
			kittyState: {},
			oscDisposables: [{ dispose: vi.fn(() => calls.push('osc1')) }],
			kittyDisposables: [{ dispose: vi.fn(() => calls.push('kitty1')) }],
			clipboardAddon: castHandleField<
				NonNullable<TerminalInstanceHandle<object>['clipboardAddon']>
			>({
				dispose: vi.fn(() => calls.push('clipboard')),
			}),
			webLinksAddon: castHandleField<NonNullable<TerminalInstanceHandle<object>['webLinksAddon']>>({
				dispose: vi.fn(() => calls.push('links')),
			}),
			unicode11Addon: castHandleField<
				NonNullable<TerminalInstanceHandle<object>['unicode11Addon']>
			>({
				dispose: vi.fn(() => calls.push('unicode')),
			}),
			webglAddon: castHandleField<NonNullable<TerminalInstanceHandle<object>['webglAddon']>>({
				dispose: vi.fn(() => calls.push('webgl')),
			}),
		});
		const manager = createTerminalInstanceManager<object>({
			terminalHandles,
			createTerminalInstance: () =>
				castHandleField<TerminalInstanceHandle<object>['terminal']>(vi.fn()),
			createFitAddon: () => castHandleField<TerminalInstanceHandle<object>['fitAddon']>(vi.fn()),
			createUnicode11Addon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle<object>['unicode11Addon']>>(vi.fn()),
			createClipboardAddon: () =>
				castHandleField<NonNullable<TerminalInstanceHandle<object>['clipboardAddon']>>(vi.fn()),
			createKittyState: () => ({ status: 'ok' }),
			syncTerminalWebLinks: vi.fn(),
			registerOscHandlers: vi.fn(() => []),
			ensureMode: vi.fn(),
			onShiftEnter: vi.fn(),
			onData: vi.fn(),
			onBinary: vi.fn(),
			onRender: vi.fn(),
			attachOpen: vi.fn(),
		});

		manager.dispose('ws::term');
		manager.dispose('missing');

		expect(calls).toEqual([
			'data',
			'binary',
			'osc1',
			'kitty1',
			'clipboard',
			'links',
			'unicode',
			'webgl',
			'terminal',
		]);
		expect(terminalHandles.has('ws::term')).toBe(false);
	});
});
