import { describe, expect, it, vi } from 'vitest';
import {
	createTerminalInstanceManager,
	type TerminalInstanceHandle,
} from './terminalInstanceManager';

type Disposable = { dispose: ReturnType<typeof vi.fn> };

type ListenerState = {
	dataCallbacks: Array<(data: string) => void>;
	dataDisposables: Disposable[];
	responseCallbacks: Array<(data: string) => void>;
	responseDisposables: Disposable[];
};

const createTerminalMock = (state: ListenerState) => ({
	loadAddon: vi.fn(),
	registerLinkProvider: vi.fn(),
	onData: vi.fn((callback: (data: string) => void) => {
		const disposable = { dispose: vi.fn() };
		state.dataCallbacks.push(callback);
		state.dataDisposables.push(disposable);
		return disposable;
	}),
	onResponse: vi.fn((callback: (data: string) => void) => {
		const disposable = { dispose: vi.fn() };
		state.responseCallbacks.push(callback);
		state.responseDisposables.push(disposable);
		return disposable;
	}),
	dispose: vi.fn(),
	open: vi.fn(),
	scrollToBottom: vi.fn(),
	focus: vi.fn(),
	write: vi.fn(),
	clear: vi.fn(),
	reset: vi.fn(),
	buffer: {
		active: {
			baseY: 0,
			viewportY: 0,
		},
	},
	options: { fontSize: 12 },
	element: undefined,
});

const createLinkProviderMock = () => ({
	provideLinks: vi.fn(),
	dispose: vi.fn(),
});

const createFitAddon = () => ({
	activate: vi.fn(),
	dispose: vi.fn(),
	fit: vi.fn(),
	proposeDimensions: vi.fn(),
});

const createEventState = () => ({
	dataCallbacks: [] as Array<(data: string) => void>,
	dataDisposables: [] as Disposable[],
	responseCallbacks: [] as Array<(data: string) => void>,
	responseDisposables: [] as Disposable[],
});

describe('terminalInstanceManager', () => {
	it('creates a terminal handle and wires listeners', async () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const listenerState = createEventState();
		const createTerminalInstance = vi.fn(() => Promise.resolve(createTerminalMock(listenerState)));
		const fitAddon = createFitAddon();
		const createFitAddonMock = vi.fn(() => fitAddon);
		const provider1 = createLinkProviderMock();
		const provider2 = createLinkProviderMock();
		const createLinkProviders = vi.fn(() => [provider1, provider2]);
		const onData = vi.fn();
		const onResponse = vi.fn();
		const onRendererError = vi.fn();
		const attachOpen = vi.fn();

		const manager = createTerminalInstanceManager({
			terminalHandles,
			createTerminalInstance,
			createFitAddon: createFitAddonMock,
			createLinkProviders,
			onData,
			onResponse,
			onRendererError,
			attachOpen,
		});

		const container = document.createElement('div') as HTMLDivElement;
		const handle = await manager.attach('ws::term', container, true);

		expect(createTerminalInstance).toHaveBeenCalledTimes(1);
		expect(createFitAddonMock).toHaveBeenCalledTimes(1);
		expect(handle.terminal.loadAddon).toHaveBeenCalledWith(fitAddon);
		expect(handle.terminal.registerLinkProvider).toHaveBeenCalledTimes(2);
		expect(handle.terminal.registerLinkProvider).toHaveBeenNthCalledWith(1, provider1);
		expect(handle.terminal.registerLinkProvider).toHaveBeenNthCalledWith(2, provider2);
		expect(handle.terminal.onData).toHaveBeenCalledTimes(1);
		expect(handle.terminal.onResponse).toHaveBeenCalledTimes(1);
		expect(attachOpen).toHaveBeenCalledWith({
			id: 'ws::term',
			handle,
			container,
			active: true,
		});
		expect(onRendererError).not.toHaveBeenCalled();

		listenerState.dataCallbacks[0]?.('hello');
		expect(onData).toHaveBeenCalledWith('ws::term', 'hello');
		listenerState.responseCallbacks[0]?.('\x1b[1;1R');
		expect(onResponse).toHaveBeenCalledWith('ws::term', '\x1b[1;1R');
	});

	it('reuses a terminal handle without duplicating data listeners', async () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const listenerState = createEventState();
		const createTerminalInstance = vi.fn(() => Promise.resolve(createTerminalMock(listenerState)));
		const fitAddon = createFitAddon();
		const createFitAddonMock = vi.fn(() => fitAddon);
		const manager = createTerminalInstanceManager({
			terminalHandles,
			createTerminalInstance,
			createFitAddon: createFitAddonMock,
			createLinkProviders: () => [createLinkProviderMock()],
			onData: vi.fn(),
			attachOpen: vi.fn(),
		});

		const container = document.createElement('div') as HTMLDivElement;
		await manager.attach('ws::term', container, false);
		await manager.attach('ws::term', container, true);

		expect(createTerminalInstance).toHaveBeenCalledTimes(1);
		expect(listenerState.dataCallbacks).toHaveLength(1);
		expect(listenerState.responseCallbacks).toHaveLength(1);
		expect(listenerState.dataDisposables[0]?.dispose).not.toHaveBeenCalled();
		expect(listenerState.responseDisposables[0]?.dispose).not.toHaveBeenCalled();
		expect(terminalHandles.get('ws::term')?.terminal.registerLinkProvider).toHaveBeenCalledTimes(1);
	});

	it('deduplicates concurrent attach calls while creating a terminal', async () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const listenerState = createEventState();
		const createTerminalInstance = vi.fn(() => Promise.resolve(createTerminalMock(listenerState)));
		const manager = createTerminalInstanceManager({
			terminalHandles,
			createTerminalInstance,
			createFitAddon: createFitAddon,
			createLinkProviders: () => [createLinkProviderMock()],
			onData: vi.fn(),
			attachOpen: vi.fn(),
		});
		const container = document.createElement('div') as HTMLDivElement;

		await Promise.all([
			manager.attach('ws::term', container, true),
			manager.attach('ws::term', container, true),
		]);

		expect(createTerminalInstance).toHaveBeenCalledTimes(1);
	});

	it('reports renderer errors when link provider registration fails', async () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const listenerState = createEventState();
		const terminal = createTerminalMock(listenerState);
		terminal.registerLinkProvider = vi.fn(() => {
			throw new Error('link provider failed');
		});
		const createTerminalInstance = vi.fn(() => Promise.resolve(terminal));
		const container = document.createElement('div') as HTMLDivElement;
		const onRendererError = vi.fn();
		const managerWithErrors = createTerminalInstanceManager({
			terminalHandles,
			createTerminalInstance,
			createFitAddon: createFitAddon,
			createLinkProviders: () => [createLinkProviderMock()],
			onData: vi.fn(),
			onRendererError,
			attachOpen: vi.fn(),
		});

		await managerWithErrors.attach('ws::term', container, true);

		expect(onRendererError).toHaveBeenCalledWith('ws::term', 'link provider failed');
	});

	it('disposes terminal resources in order and removes handle', async () => {
		const calls: string[] = [];
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		terminalHandles.set('ws::term', {
			terminal: {
				cols: 80,
				rows: 24,
				dispose: vi.fn(() => calls.push('terminal')),
				scrollToBottom: vi.fn(),
				focus: vi.fn(),
				open: vi.fn(),
				write: vi.fn(),
				clear: vi.fn(),
				reset: vi.fn(),
				loadAddon: vi.fn(),
				onData: vi.fn(),
				onResponse: vi.fn(),
				buffer: {
					active: {
						baseY: 0,
						viewportY: 0,
					},
				},
				options: { fontSize: 12 },
				element: undefined,
			},
			fitAddon: createFitAddon(),
			linkProviders: [
				{
					provideLinks: vi.fn(),
					dispose: vi.fn(() => calls.push('link-provider')),
				},
			],
			dataDisposable: { dispose: vi.fn(() => calls.push('data')) },
			container: document.createElement('div') as HTMLDivElement,
		});
		const manager = createTerminalInstanceManager({
			terminalHandles,
			createTerminalInstance: vi.fn(),
			createFitAddon: createFitAddon,
			onData: vi.fn(),
			attachOpen: vi.fn(),
		});

		manager.dispose('ws::term');
		manager.dispose('missing');

		expect(calls).toEqual(['data', 'link-provider', 'terminal']);
		expect(terminalHandles.has('ws::term')).toBe(false);
	});

	it('drops a failed attach-open handle so the next attach can recreate it', async () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const firstListenerState = createEventState();
		const secondListenerState = createEventState();
		const firstTerminal = createTerminalMock(firstListenerState);
		const secondTerminal = createTerminalMock(secondListenerState);
		const createTerminalInstance = vi
			.fn()
			.mockResolvedValueOnce(firstTerminal)
			.mockResolvedValueOnce(secondTerminal);
		const attachOpen = vi
			.fn()
			.mockRejectedValueOnce(new Error('Failed to open terminal: boom'))
			.mockResolvedValueOnce(undefined);
		const manager = createTerminalInstanceManager({
			terminalHandles,
			createTerminalInstance,
			createFitAddon: createFitAddon,
			onData: vi.fn(),
			attachOpen,
		});
		const container = document.createElement('div') as HTMLDivElement;

		await expect(manager.attach('ws::term', container, true)).rejects.toThrow(
			'Failed to open terminal: boom',
		);
		expect(firstTerminal.dispose).toHaveBeenCalledTimes(1);
		expect(terminalHandles.has('ws::term')).toBe(false);

		const recovered = await manager.attach('ws::term', container, true);
		expect(createTerminalInstance).toHaveBeenCalledTimes(2);
		expect(recovered.terminal).toBe(secondTerminal);
	});
});
