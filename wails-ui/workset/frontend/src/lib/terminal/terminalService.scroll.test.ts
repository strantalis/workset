import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type TerminalScenario = {
	baseY: number;
	viewportY: number;
	nextBaseY: number;
};

const terminalScenarios: TerminalScenario[] = [];
const createdTerminals: MockTerminal[] = [];

class MockTerminal {
	public buffer = {
		active: {
			baseY: 0,
			viewportY: 0,
			length: 0,
			getLine: () => ({ translateToString: () => '' }),
		},
	};
	public options: Record<string, unknown>;
	public parser = {
		registerOscHandler: () => ({ dispose: () => undefined }),
	};
	public unicode = { activeVersion: '' };
	public element: { parentElement: HTMLElement | null } | null = null;
	public rows = 24;
	public scrollToBottom = vi.fn(() => {
		this.buffer.active.viewportY = this.buffer.active.baseY;
	});
	public scrollToLine = vi.fn((line: number) => {
		this.buffer.active.viewportY = line;
	});
	public refresh = vi.fn();
	public write = vi.fn((_data: string, cb?: () => void) => cb?.());
	public focus = vi.fn();
	public clear = vi.fn();
	public reset = vi.fn();
	private scenario: TerminalScenario;

	constructor(options: Record<string, unknown>) {
		this.options = options;
		this.scenario = terminalScenarios.shift() ?? { baseY: 0, viewportY: 0, nextBaseY: 0 };
		this.buffer.active.baseY = this.scenario.baseY;
		this.buffer.active.viewportY = this.scenario.viewportY;
		this.buffer.active.length = Math.max(this.buffer.active.baseY + 1, 1);
		createdTerminals.push(this);
	}

	loadAddon(addon: { activate?: (terminal: MockTerminal) => void }): void {
		addon.activate?.(this);
	}

	attachCustomKeyEventHandler(): void {}

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

	applyFit(): void {
		this.buffer.active.baseY = this.scenario.nextBaseY;
		this.buffer.active.length = Math.max(this.buffer.active.baseY + 1, 1);
	}
}

class MockFitAddon {
	private terminal: MockTerminal | null = null;

	activate(terminal: MockTerminal): void {
		this.terminal = terminal;
	}

	fit(): void {
		this.terminal?.applyFit();
	}

	proposeDimensions(): { cols: number; rows: number } {
		return { cols: 80, rows: 24 };
	}
}

class NoopAddon {
	activate(): void {}
	dispose(): void {}
}

const runtimeMock = {
	BrowserOpenURL: vi.fn(),
	EventsOn: vi.fn(),
	EventsOff: vi.fn(),
};

const appMock = {
	AckWorkspaceTerminal: vi.fn().mockResolvedValue(undefined),
	ResizeWorkspaceTerminal: vi.fn().mockResolvedValue(undefined),
	StartWorkspaceTerminal: vi.fn().mockResolvedValue(undefined),
	WriteWorkspaceTerminal: vi.fn().mockResolvedValue(undefined),
};

const apiMock = {
	fetchSessiondStatus: vi.fn().mockResolvedValue({ available: false }),
	fetchSettings: vi.fn().mockResolvedValue({ defaults: {} }),
	fetchTerminalBootstrap: vi.fn().mockResolvedValue({
		workspaceId: 'ws',
		terminalId: 'term',
	}),
	fetchWorkspaceTerminalStatus: vi.fn().mockResolvedValue({ active: false }),
	logTerminalDebug: vi.fn().mockResolvedValue(undefined),
	stopWorkspaceTerminal: vi.fn().mockResolvedValue(undefined),
};

vi.mock('@xterm/xterm', () => ({
	Terminal: MockTerminal,
}));

vi.mock('@xterm/addon-fit', () => ({
	FitAddon: MockFitAddon,
}));

vi.mock('@xterm/addon-clipboard', () => ({
	ClipboardAddon: NoopAddon,
}));

vi.mock('@xterm/addon-unicode11', () => ({
	Unicode11Addon: NoopAddon,
}));

vi.mock('@xterm/addon-web-links', () => ({
	WebLinksAddon: NoopAddon,
}));

vi.mock('@xterm/addon-webgl', () => ({
	WebglAddon: NoopAddon,
}));

vi.mock('../../../wailsjs/runtime/runtime', () => runtimeMock);
vi.mock('../../../wailsjs/go/main/App', () => appMock);
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

describe('terminalService fit viewport preservation', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.resetModules();
		vi.clearAllMocks();
		terminalScenarios.length = 0;
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

	it('preserves relative viewport position when user scrolled up', async () => {
		terminalScenarios.push({ baseY: 120, viewportY: 90, nextBaseY: 260 });
		const service = await loadService();
		const container = document.createElement('div') as HTMLDivElement;

		service.syncTerminal({
			workspaceId: 'ws',
			workspaceName: 'demo',
			terminalId: 'term',
			container,
			active: false,
		});

		expect(createdTerminals).toHaveLength(1);
		const terminal = createdTerminals[0];
		expect(terminal.scrollToLine).toHaveBeenCalledWith(230);
		expect(terminal.scrollToBottom).not.toHaveBeenCalled();
	});

	it('keeps follow mode at bottom after fit', async () => {
		terminalScenarios.push({ baseY: 120, viewportY: 120, nextBaseY: 260 });
		const service = await loadService();
		const container = document.createElement('div') as HTMLDivElement;

		service.syncTerminal({
			workspaceId: 'ws',
			workspaceName: 'demo',
			terminalId: 'term',
			container,
			active: false,
		});

		expect(createdTerminals).toHaveLength(1);
		const terminal = createdTerminals[0];
		expect(terminal.scrollToBottom).toHaveBeenCalled();
		expect(terminal.scrollToLine).not.toHaveBeenCalled();
	});
});
