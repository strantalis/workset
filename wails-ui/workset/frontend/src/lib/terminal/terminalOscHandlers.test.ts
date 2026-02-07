import { describe, expect, it, vi } from 'vitest';
import { registerTerminalOscHandlers } from './terminalOscHandlers';

describe('registerTerminalOscHandlers', () => {
	it('responds to OSC 10/11/12 color queries', () => {
		const handlers = new Map<number, (data: string) => boolean>();
		const sendInput = vi.fn();
		const terminal = {
			options: {
				theme: {
					foreground: '#112233',
					background: '#445566',
					cursor: '#778899',
				},
			},
			parser: {
				registerOscHandler: (code: number, handler: (data: string) => boolean) => {
					handlers.set(code, handler);
					return { dispose: () => undefined };
				},
			},
		};

		registerTerminalOscHandlers('t-1', terminal as never, {
			sendInput,
			getToken: (_name, fallback) => fallback,
		});

		expect(handlers.get(10)?.('?')).toBe(true);
		expect(handlers.get(11)?.('?')).toBe(true);
		expect(handlers.get(12)?.('?')).toBe(true);
		expect(sendInput).toHaveBeenCalledWith('t-1', '\x1b]10;rgb:11/22/33\x07');
		expect(sendInput).toHaveBeenCalledWith('t-1', '\x1b]11;rgb:44/55/66\x07');
		expect(sendInput).toHaveBeenCalledWith('t-1', '\x1b]12;rgb:77/88/99\x07');
	});

	it('responds to OSC 4 palette queries when colors are available', () => {
		const handlers = new Map<number, (data: string) => boolean>();
		const sendInput = vi.fn();
		const terminal = {
			options: {
				theme: {
					black: '#000000',
					red: '#cd3131',
					extendedAnsi: ['#abcdef'],
				},
			},
			parser: {
				registerOscHandler: (code: number, handler: (data: string) => boolean) => {
					handlers.set(code, handler);
					return { dispose: () => undefined };
				},
			},
		};

		registerTerminalOscHandlers('t-2', terminal as never, {
			sendInput,
			getToken: (_name, fallback) => fallback,
		});

		expect(handlers.get(4)?.('0;?;16;?')).toBe(true);
		expect(sendInput).toHaveBeenCalledWith('t-2', '\x1b]4;0;rgb:00/00/00\x07');
		expect(sendInput).toHaveBeenCalledWith('t-2', '\x1b]4;16;rgb:ab/cd/ef\x07');
	});
});
