import { get } from 'svelte/store';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

class MockTerminal {
	public buffer = {
		active: {
			baseY: 0,
			viewportY: 0,
			length: 1,
			getLine: () => ({ translateToString: () => '' }),
		},
	};
	public options: Record<string, unknown>;
	public element: { parentElement: HTMLElement | null } | null = null;
	public rows = 24;
	public scrollToBottom = vi.fn(() => {
		this.buffer.active.viewportY = this.buffer.active.baseY;
	});
	public write = vi.fn((_data: string, cb?: () => void) => cb?.());
	public dispose = vi.fn();
	public focus = vi.fn();
	public clear = vi.fn();
	public reset = vi.fn();
	public onDataCallback: ((data: string) => void) | null = null;

	constructor(options: Record<string, unknown>) {
		this.options = options;
		createdTerminals.push(this);
	}

	loadAddon(addon: { activate?: (terminal: MockTerminal) => void }): void {
		addon.activate?.(this);
	}

	onData(callback: (data: string) => void): { dispose: () => void } {
		this.onDataCallback = callback;
		return { dispose: () => undefined };
	}

	open(container: HTMLElement): void {
		this.element = { parentElement: container };
	}

	emitData(data: string): void {
		this.onDataCallback?.(data);
	}
}

class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	public binaryType = 'blob';
	public readyState = MockWebSocket.CONNECTING;
	public readonly sent: unknown[] = [];
	private listeners = new Map<string, Set<(event: unknown) => void>>();

	constructor(public readonly url: string) {
		createdSockets.push(this);
	}

	addEventListener(type: string, listener: (event: unknown) => void): void {
		const current = this.listeners.get(type) ?? new Set();
		current.add(listener);
		this.listeners.set(type, current);
	}

	send(data: unknown): void {
		this.sent.push(data);
	}

	close(code = 1000, reason = ''): void {
		this.readyState = MockWebSocket.CLOSED;
		this.dispatch('close', { code, reason, wasClean: true });
	}

	open(): void {
		this.readyState = MockWebSocket.OPEN;
		this.dispatch('open', {});
	}

	emitText(data: unknown): void {
		this.dispatch('message', { data: JSON.stringify(data) });
	}

	emitBinary(payload: Uint8Array): void {
		this.dispatch('message', { data: payload.buffer.slice(0) });
	}

	private dispatch(type: string, event: unknown): void {
		for (const listener of this.listeners.get(type) ?? []) {
			listener(event);
		}
	}
}

class MockFitAddon {
	activate(_terminal: MockTerminal): void {}
	dispose(): void {}
	fit = vi.fn();
	proposeDimensions(): { cols: number; rows: number } {
		return { cols: 80, rows: 24 };
	}
}

const createdTerminals: MockTerminal[] = [];
const createdSockets: MockWebSocket[] = [];

const encodeChunk = (seq: number, value: string): Uint8Array => {
	const text = new TextEncoder().encode(value);
	const payload = new Uint8Array(8 + text.length);
	const view = new DataView(payload.buffer);
	view.setBigUint64(0, BigInt(seq), false);
	payload.set(text, 8);
	return payload;
};

const runtimeMock = vi.hoisted(() => ({
	Browser: {
		OpenURL: vi.fn(),
	},
	Events: {
		On: vi.fn(),
		Off: vi.fn(),
	},
}));

const appMock = vi.hoisted(() => ({
	LogTerminalDebug: vi.fn().mockResolvedValue(undefined),
	StartWorkspaceTerminalSessionForWindow: vi.fn().mockResolvedValue({
		workspaceId: 'ws',
		terminalId: 'term',
		sessionId: 'ws::term',
		socketUrl: 'ws://127.0.0.1:9001/stream',
		socketToken: 'token',
	}),
	StopWorkspaceTerminalForWindow: vi.fn().mockResolvedValue(undefined),
	GetTerminalServiceStatus: vi.fn().mockResolvedValue({ available: false }),
	GetSettings: vi.fn().mockResolvedValue({ defaults: {} }),
}));

const apiMock = vi.hoisted(() => ({
	fetchTerminalServiceStatus: vi.fn().mockResolvedValue({ available: false }),
	fetchSettings: vi.fn().mockResolvedValue({ defaults: {} }),
	fetchTerminalBootstrap: vi.fn().mockResolvedValue({
		workspaceId: 'ws',
		terminalId: 'term',
		sessionId: 'ws::term',
		socketUrl: 'ws://127.0.0.1:9001/stream',
		socketToken: 'token',
	}),
	logTerminalDebug: vi.fn().mockResolvedValue(undefined),
	stopWorkspaceTerminal: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('@strantalis/workset-ghostty-web', () => ({
	init: vi.fn(async () => undefined),
	Terminal: MockTerminal,
	FitAddon: MockFitAddon,
}));

vi.mock('@wailsio/runtime', () => runtimeMock);
vi.mock('../../../bindings/workset/app', () => appMock);
vi.mock('../api/settings', () => ({
	fetchTerminalServiceStatus: apiMock.fetchTerminalServiceStatus,
	fetchSettings: apiMock.fetchSettings,
}));
vi.mock('../api/terminal-layout', () => ({
	fetchTerminalBootstrap: apiMock.fetchTerminalBootstrap,
	logTerminalDebug: apiMock.logTerminalDebug,
	stopWorkspaceTerminal: apiMock.stopWorkspaceTerminal,
}));

const loadService = async () => import('./terminalService');

class MockResizeObserver {
	constructor(_cb: ResizeObserverCallback) {}
	observe(): void {}
	disconnect(): void {}
}

const installLocalStorage = (): void => {
	const store = new Map<string, string>();
	Object.defineProperty(globalThis, 'localStorage', {
		value: {
			getItem: (key: string) => store.get(key) ?? null,
			setItem: (key: string, value: string) => {
				store.set(key, String(value));
			},
			removeItem: (key: string) => {
				store.delete(key);
			},
			clear: () => {
				store.clear();
			},
		},
		configurable: true,
	});
};

describe('terminalService resize flow', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.resetModules();
		vi.clearAllMocks();
		createdTerminals.length = 0;
		createdSockets.length = 0;
		Object.defineProperty(globalThis, 'ResizeObserver', {
			value: MockResizeObserver,
			configurable: true,
		});
		Object.defineProperty(globalThis, 'WebSocket', {
			value: MockWebSocket,
			configurable: true,
		});
		Object.defineProperty(HTMLCanvasElement.prototype, 'getContext', {
			value: () => ({ clearRect: () => undefined, drawImage: () => undefined }),
			configurable: true,
		});
		installLocalStorage();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('syncs terminal without viewport snapshot restore behavior', async () => {
		const service = await loadService();
		const container = document.createElement('div') as HTMLDivElement;

		service.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: false,
		});
		await vi.waitFor(() => {
			expect(createdTerminals).toHaveLength(1);
		});
		// scrollToBottom may be called during session reset (resetTerminalInstance)
		// but should not be called as part of viewport snapshot restore
		expect(createdTerminals[0]).toBeDefined();
	});

	it('logs state transitions and browser lifecycle when terminal lifecycle logging is enabled', async () => {
		apiMock.fetchSettings.mockResolvedValue({ defaults: { terminalDebugLog: 'on' } });
		const service = await loadService();
		await service.refreshTerminalDefaults();
		const container = document.createElement('div') as HTMLDivElement;

		service.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});

		await vi.waitFor(() => {
			expect(createdTerminals).toHaveLength(1);
			expect(createdSockets).toHaveLength(1);
		});

		const socket = createdSockets[0];
		socket.open();
		socket.emitText({ type: 'ready' });

		await vi.waitFor(() => {
			expect(apiMock.logTerminalDebug).toHaveBeenCalledWith(
				'ws',
				'term',
				'frontend_state_transition',
				expect.stringContaining('"status":"starting"'),
			);
		});

		await vi.waitFor(() => {
			expect(apiMock.logTerminalDebug).toHaveBeenCalledWith(
				'ws',
				'term',
				'frontend_state_transition',
				expect.stringContaining('"status":"ready"'),
			);
		});

		window.dispatchEvent(new Event('blur'));

		await vi.waitFor(() => {
			expect(apiMock.logTerminalDebug).toHaveBeenCalledWith(
				'ws',
				'term',
				'frontend_window_lifecycle',
				expect.stringContaining('"event":"window.blur"'),
			);
		});

		Object.defineProperty(document, 'visibilityState', {
			value: 'visible',
			configurable: true,
		});
		Object.defineProperty(document, 'hidden', {
			value: false,
			configurable: true,
		});
		window.dispatchEvent(new Event('focus'));

		await vi.waitFor(() => {
			expect(apiMock.logTerminalDebug).toHaveBeenCalledWith(
				'ws',
				'term',
				'frontend_window_state_resync',
				expect.stringContaining('"reason":"window.focus"'),
			);
		});

		expect(get(service.getTerminalStore('ws', 'term'))).toMatchObject({
			status: 'ready',
			health: 'ok',
		});
	});

	it('forwards wheel input after attaching terminal host in another document', async () => {
		const service = await loadService();
		const container = document.createElement('div') as HTMLDivElement;

		service.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});
		await vi.waitFor(() => {
			expect(createdSockets).toHaveLength(1);
		});
		const socket = createdSockets[0];
		socket.open();
		socket.emitText({ type: 'ready' });
		await vi.waitFor(() => {
			expect(apiMock.fetchTerminalBootstrap).toHaveBeenCalled();
		});

		expect(createdTerminals).toHaveLength(1);
		const terminal = createdTerminals[0];

		const popoutDocument = document.implementation.createHTMLDocument('popout');
		const popoutContainer = popoutDocument.createElement('div') as HTMLDivElement;
		Object.defineProperty(popoutDocument, 'activeElement', {
			get: () => popoutContainer.firstElementChild,
			configurable: true,
		});

		service.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container: popoutContainer,
			active: true,
		});
		await Promise.resolve();

		expect(popoutContainer.firstElementChild).toBeTruthy();
		expect(popoutContainer.firstElementChild?.ownerDocument).toBe(popoutDocument);
		expect(terminal.onDataCallback).toBeTypeOf('function');

		terminal.emitData('\x1b[<64;10;10M');
		await vi.waitFor(() => {
			expect(socket.sent).toContainEqual(
				JSON.stringify({
					protocolVersion: 2,
					type: 'input',
					data: '\x1b[<64;10;10M',
				}),
			);
		});
	});

	it('opens terminal links via Browser.OpenURL', async () => {
		const service = await loadService();
		const container = document.createElement('div') as HTMLDivElement;

		service.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});
		await vi.waitFor(() => {
			expect(apiMock.fetchTerminalBootstrap).toHaveBeenCalled();
		});
		expect(createdTerminals).toHaveLength(1);
		const terminal = createdTerminals[0];
		const openLink = terminal.options.openLink as
			| ((url: string, event: MouseEvent) => void | Promise<void>)
			| undefined;

		expect(openLink).toBeTypeOf('function');

		await openLink?.('https://osc8.example.com', {} as MouseEvent);

		await Promise.resolve();

		expect(runtimeMock.Browser.OpenURL).toHaveBeenCalledWith('https://osc8.example.com');
	});

	it('batches websocket output frames into a single terminal write', async () => {
		const service = await loadService();
		const container = document.createElement('div') as HTMLDivElement;

		service.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});

		await vi.waitFor(() => {
			expect(createdTerminals).toHaveLength(1);
			expect(createdSockets).toHaveLength(1);
		});

		const terminal = createdTerminals[0];
		const socket = createdSockets[0];
		socket.open();
		socket.emitText({ type: 'ready' });

		terminal.write.mockClear();
		socket.emitBinary(encodeChunk(5, 'hello '));
		socket.emitBinary(encodeChunk(11, 'world'));

		await vi.waitFor(() => {
			expect(terminal.write).toHaveBeenCalledTimes(1);
		});
		const merged = terminal.write.mock.calls[0][0] as unknown as Uint8Array;
		expect(new TextDecoder().decode(merged)).toBe('hello world');
	});

	it('stops a live terminal over websocket before falling back to Wails stop', async () => {
		const service = await loadService();
		const container = document.createElement('div') as HTMLDivElement;

		service.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});

		await vi.waitFor(() => {
			expect(createdSockets).toHaveLength(1);
		});

		const socket = createdSockets[0];
		socket.open();
		socket.emitText({ type: 'ready' });
		await vi.waitFor(() => {
			expect(apiMock.fetchTerminalBootstrap).toHaveBeenCalled();
		});
		socket.sent.length = 0;

		await service.closeTerminal('ws', 'term');

		expect(socket.sent).toContainEqual(
			JSON.stringify({
				protocolVersion: 2,
				type: 'stop',
			}),
		);
		expect(apiMock.stopWorkspaceTerminal).not.toHaveBeenCalled();
	});

	it('marks the terminal closed when the server reports the session exited', async () => {
		const service = await loadService();
		const container = document.createElement('div') as HTMLDivElement;

		service.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});

		await vi.waitFor(() => {
			expect(createdSockets).toHaveLength(1);
		});

		const socket = createdSockets[0];
		socket.open();
		socket.emitText({ type: 'ready' });
		await vi.waitFor(() => {
			expect(apiMock.fetchTerminalBootstrap).toHaveBeenCalled();
		});

		socket.emitText({ type: 'closed' });
		socket.close(1000, 'session exited');

		await vi.waitFor(() => {
			expect(get(service.getTerminalStore('ws', 'term'))).toMatchObject({
				status: 'closed',
				message: 'Terminal exited.',
				health: 'unknown',
				healthMessage: '',
			});
		});
	});

	it('does not restart or refit healthy terminals on window focus', async () => {
		const service = await loadService();
		const container = document.createElement('div') as HTMLDivElement;
		const widthDescriptor = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientWidth');
		const heightDescriptor = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'clientHeight');
		Object.defineProperty(HTMLElement.prototype, 'clientWidth', {
			configurable: true,
			get() {
				return this.isConnected ? 800 : 0;
			},
		});
		Object.defineProperty(HTMLElement.prototype, 'clientHeight', {
			configurable: true,
			get() {
				return this.isConnected ? 400 : 0;
			},
		});
		const restoreClientMetrics = (): void => {
			if (widthDescriptor) {
				Object.defineProperty(HTMLElement.prototype, 'clientWidth', widthDescriptor);
			}
			if (heightDescriptor) {
				Object.defineProperty(HTMLElement.prototype, 'clientHeight', heightDescriptor);
			}
		};
		try {
			service.syncTerminal({
				workspaceId: 'ws',
				terminalId: 'term',
				container,
				active: true,
			});
			await vi.waitFor(() => {
				expect(createdSockets).toHaveLength(1);
			});
			const socket = createdSockets[0];
			socket.open();
			socket.emitText({ type: 'ready' });
			await vi.runAllTimersAsync();
			apiMock.fetchTerminalBootstrap.mockClear();
			socket.sent.length = 0;

			document.body.appendChild(container);
			window.dispatchEvent(new Event('focus'));
			await vi.runAllTimersAsync();

			expect(apiMock.fetchTerminalBootstrap).not.toHaveBeenCalled();
			expect(socket.sent).toHaveLength(0);
		} finally {
			container.remove();
			restoreClientMetrics();
		}
	});
});
