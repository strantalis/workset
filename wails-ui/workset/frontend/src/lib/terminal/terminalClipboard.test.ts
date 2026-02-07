import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	createTerminalClipboardBase64,
	createTerminalClipboardProvider,
} from './terminalClipboard';

describe('terminalClipboard', () => {
	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('round-trips clipboard base64 encoding', () => {
		const clipboard = createTerminalClipboardBase64();
		const input = 'hello μ-world';
		const encoded = clipboard.encodeText(input);
		expect(encoded).not.toBe('');
		expect(clipboard.decodeText(encoded)).toBe(input);
	});

	it('skips writing clipboard payloads larger than the max bytes', async () => {
		Object.defineProperty(navigator, 'clipboard', {
			value: {
				writeText: vi.fn().mockResolvedValue(undefined),
			},
			configurable: true,
		});
		const provider = createTerminalClipboardProvider();
		const oversized = 'a'.repeat(1024 * 1024 + 1);
		await provider.writeText('clipboard', oversized);
		expect(navigator.clipboard.writeText).not.toHaveBeenCalled();
	});

	it('writes to browser clipboard when payload is valid', async () => {
		Object.defineProperty(navigator, 'clipboard', {
			value: {
				writeText: vi.fn().mockResolvedValue(undefined),
			},
			configurable: true,
		});
		const provider = createTerminalClipboardProvider();
		await provider.writeText('clipboard', 'copied');
		expect(navigator.clipboard.writeText).toHaveBeenCalledWith('copied');
	});
});
