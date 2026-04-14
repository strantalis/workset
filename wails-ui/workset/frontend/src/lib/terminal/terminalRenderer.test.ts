import { beforeEach, describe, expect, it, vi } from 'vitest';

const terminalCtor = vi.fn();

vi.mock('@strantalis/workset-ghostty-web', () => ({
	Terminal: function MockTerminal(options: unknown) {
		terminalCtor(options);
	},
}));

vi.mock('./ghosttyRuntime', () => ({
	ensureGhosttyInitialized: vi.fn(async () => undefined),
}));

describe('createTerminalInstance', () => {
	beforeEach(() => {
		terminalCtor.mockClear();
	});

	it('uses the packaged ghostty-web defaults and terminal theme colors', async () => {
		const { createTerminalInstance } = await import('./terminalRenderer');
		const openLink = vi.fn();

		await createTerminalInstance({
			fontSize: 14,
			cursorBlink: false,
			getToken: (_name, fallback) => fallback,
			openLink,
		});

		const options = terminalCtor.mock.calls[0]?.[0];

		expect(options).toEqual(
			expect.objectContaining({
				smoothScrollDuration: 0,
				theme: expect.objectContaining({
					background: '#111c29',
					foreground: '#eef3f9',
					cursor: '#2d8cff',
					black: '#1c2b3a',
					red: '#ef4444',
					green: '#86c442',
					yellow: '#f59e0b',
					blue: '#2d8cff',
					magenta: '#8b8aed',
					cyan: '#56b6c2',
					white: '#a3b5c9',
					brightBlack: '#3a4f63',
					brightRed: '#f87171',
					brightGreen: '#a3d977',
					brightYellow: '#fbbf24',
					brightBlue: '#60a5fa',
					brightMagenta: '#a78bfa',
					brightCyan: '#67d4e0',
					brightWhite: '#f2f6fb',
				}),
				fontSize: 14,
				fontFamily: '"SF Mono", Menlo, Monaco, Consolas, monospace',
				cursorStyle: 'bar',
				cursorBlink: false,
				openLink,
			}),
		);
	});
});
