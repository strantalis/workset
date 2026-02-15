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

	constructor(options: Record<string, unknown>) {
		this.options = options;
		createdTerminals.push(this);
	}

	loadAddon(addon: { activate?: (terminal: MockTerminal) => void }): void {
		addon.activate?.(this);
	}

	onData(): { dispose: () => void } {
		return { dispose: () => undefined };
	}

	onBinary(): { dispose: () => void } {
		return { dispose: () => undefined };
	}

	onRender(): { dispose: () => void } {
		return { dispose: () => undefined };
	}

	open(container: HTMLElement): void {
		this.element = { parentElement: container };
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
});
