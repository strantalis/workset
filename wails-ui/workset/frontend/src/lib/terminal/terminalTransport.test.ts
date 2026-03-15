import { beforeEach, describe, expect, it, vi } from 'vitest';

const terminalLayoutMocks = vi.hoisted(() => ({
	fetchTerminalBootstrap: vi.fn(async () => ({
		sessionId: 'session-1',
		owner: 'window-1',
		canWrite: true,
		running: true,
		currentOffset: 0,
		transport: 'socket',
	})),
	logTerminalDebug: vi.fn(),
	stopWorkspaceTerminal: vi.fn(),
}));

vi.mock('../api/settings', () => ({
	fetchSessiondStatus: vi.fn(),
	fetchSettings: vi.fn(),
}));
vi.mock('../api/terminal-layout', () => terminalLayoutMocks);
vi.mock('@wailsio/runtime', () => ({
	Browser: {
		OpenURL: vi.fn(),
	},
}));

const storage = new Map<string, string>();

describe('terminalTransport', () => {
	beforeEach(() => {
		vi.resetModules();
		vi.stubGlobal('localStorage', {
			getItem: (key: string) => storage.get(key) ?? null,
			setItem: (key: string, value: string) => {
				storage.set(key, value);
			},
			removeItem: (key: string) => {
				storage.delete(key);
			},
		});
		storage.clear();
		terminalLayoutMocks.fetchTerminalBootstrap.mockClear();
		terminalLayoutMocks.logTerminalDebug.mockClear();
	});

	it('does not emit transport debug logs when terminal debug is disabled', async () => {
		const { terminalTransport } = await import('./terminalTransport');

		await terminalTransport.start('ws-1', 'term-1');

		expect(terminalLayoutMocks.logTerminalDebug).not.toHaveBeenCalled();
		expect(terminalLayoutMocks.fetchTerminalBootstrap).toHaveBeenCalledTimes(1);
	});

	it('emits transport debug logs when terminal debug is enabled', async () => {
		storage.set('worksetTerminalDebug', '1');
		const { terminalTransport } = await import('./terminalTransport');

		await terminalTransport.start('ws-1', 'term-1');

		expect(terminalLayoutMocks.logTerminalDebug).toHaveBeenCalled();
		expect(terminalLayoutMocks.logTerminalDebug).toHaveBeenCalledWith(
			'ws-1',
			'term-1',
			expect.any(String),
			expect.any(String),
		);
	});
});
