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
