import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const createdTerminals: MockTerminal[] = [];

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
	public refresh = vi.fn();
	public write = vi.fn((_data: string, cb?: () => void) => cb?.());
	public focus = vi.fn();
	public clear = vi.fn();
	public reset = vi.fn();
	public attachCustomWheelEventHandler = vi.fn((handler: (event: WheelEvent) => boolean) => {
		this.customWheelEventHandler = handler;
	});
	public customWheelEventHandler: ((event: WheelEvent) => boolean) | null = null;
	public onDataCallback: ((data: string) => void) | null = null;
	public onBinaryCallback: ((data: string) => void) | null = null;

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

	onBinary(callback: (data: string) => void): { dispose: () => void } {
		this.onBinaryCallback = callback;
		return { dispose: () => undefined };
	}

	onRender(): { dispose: () => void } {
		return { dispose: () => undefined };
	}

	open(container: HTMLElement): void {
		this.element = { parentElement: container };
	}

	emitData(data: string): void {
		this.onDataCallback?.(data);
	}

	emitBinary(data: string): void {
		this.onBinaryCallback?.(data);
	}
}

class MockFitAddon {
	activate(_terminal: MockTerminal): void {}
	fit = vi.fn();
	proposeDimensions(): { cols: number; rows: number } {
		return { cols: 80, rows: 24 };
	}
}

const runtimeMock = {
	Browser: {
		OpenURL: vi.fn(),
	},
	Events: {
		On: vi.fn(),
		Off: vi.fn(),
	},
};

const appMock = {
	ResizeWorkspaceTerminalForWindowName: vi.fn().mockResolvedValue(undefined),
	StartWorkspaceTerminalForWindowName: vi.fn().mockResolvedValue(undefined),
	WriteWorkspaceTerminalForWindowName: vi.fn().mockResolvedValue(undefined),
};

const apiMock = {
	fetchSessiondStatus: vi.fn().mockResolvedValue({ available: false }),
	fetchSettings: vi.fn().mockResolvedValue({ defaults: {} }),
	fetchTerminalBootstrap: vi.fn().mockResolvedValue({
		workspaceId: 'ws',
		terminalId: 'term',
	}),
	logTerminalDebug: vi.fn().mockResolvedValue(undefined),
	stopWorkspaceTerminal: vi.fn().mockResolvedValue(undefined),
};

vi.mock('@xterm/xterm', () => ({
	Terminal: MockTerminal,
}));

vi.mock('@xterm/addon-fit', () => ({
	FitAddon: MockFitAddon,
}));
vi.mock('@xterm/addon-image', () => ({
	ImageAddon: class MockImageAddon {
		activate(): void {}
		dispose(): void {}
	},
}));
vi.mock('@xterm/addon-search', () => ({
	SearchAddon: class MockSearchAddon {
		activate(): void {}
		dispose(): void {}
	},
}));
vi.mock('@xterm/addon-web-links', () => ({
	WebLinksAddon: class MockWebLinksAddon {
		activate(): void {}
		dispose(): void {}
	},
}));

vi.mock('@wailsio/runtime', () => runtimeMock);
vi.mock('../../../bindings/workset/app', () => appMock);
vi.mock('../api', () => apiMock);

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
		Object.defineProperty(globalThis, 'ResizeObserver', {
			value: MockResizeObserver,
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
		await Promise.resolve();
		await Promise.resolve();

		expect(createdTerminals).toHaveLength(1);
		const terminal = createdTerminals[0];
		expect(terminal.scrollToBottom).not.toHaveBeenCalled();
	});

	it('attaches a custom wheel handler that consumes browser scroll', async () => {
		const service = await loadService();
		const container = document.createElement('div') as HTMLDivElement;

		service.syncTerminal({
			workspaceId: 'ws',
			terminalId: 'term',
			container,
			active: true,
		});
		await vi.waitFor(() => {
			expect(appMock.StartWorkspaceTerminalForWindowName).toHaveBeenCalled();
		});

		expect(createdTerminals).toHaveLength(1);
		const terminal = createdTerminals[0];
		expect(terminal.attachCustomWheelEventHandler).toHaveBeenCalledTimes(1);
		expect(terminal.customWheelEventHandler).toBeTypeOf('function');

		const preventDefault = vi.fn();
		const stopPropagation = vi.fn();
		const handled = terminal.customWheelEventHandler?.({
			preventDefault,
			stopPropagation,
		} as unknown as WheelEvent);
		expect(handled).toBe(true);
		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(stopPropagation).toHaveBeenCalledTimes(1);
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
			expect(appMock.StartWorkspaceTerminalForWindowName).toHaveBeenCalled();
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
			expect(appMock.WriteWorkspaceTerminalForWindowName).toHaveBeenCalledWith(
				'ws',
				'term',
				'\x1b[<64;10;10M',
				expect.any(String),
			);
		});
	});

	it('refits attached terminals on window focus so handoff does not require manual resize', async () => {
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
				expect(appMock.StartWorkspaceTerminalForWindowName).toHaveBeenCalled();
			});
			await vi.runAllTimersAsync();
			expect(appMock.ResizeWorkspaceTerminalForWindowName).not.toHaveBeenCalled();
			appMock.ResizeWorkspaceTerminalForWindowName.mockClear();

			document.body.appendChild(container);
			window.dispatchEvent(new Event('focus'));
			await vi.runAllTimersAsync();

			expect(appMock.ResizeWorkspaceTerminalForWindowName.mock.calls.length).toBeGreaterThan(0);
		} finally {
			container.remove();
			restoreClientMetrics();
		}
	});
});
