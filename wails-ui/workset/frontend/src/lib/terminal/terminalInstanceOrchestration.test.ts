import { describe, expect, it, vi } from 'vitest';
import { createTerminalInstanceOrchestration } from './terminalInstanceOrchestration';
import type { TerminalInstanceHandle } from './terminalInstanceManager';

const createHandle = (cursorBlink: boolean): TerminalInstanceHandle => ({
	terminal: {
		options: {
			fontSize: 13,
			cursorBlink,
		},
	} as never,
	fitAddon: {
		dispose: vi.fn(),
		fit: vi.fn(),
		proposeDimensions: vi.fn(),
	},
	container: document.createElement('div') as HTMLDivElement,
	dataDisposable: {
		dispose: vi.fn(),
	},
});

describe('terminalInstanceOrchestration', () => {
	it('keeps cursor blink enabled only for active terminals', () => {
		const terminalHandles = new Map<string, TerminalInstanceHandle>();
		const orchestration = createTerminalInstanceOrchestration({
			terminalHandles,
			createTerminalInstance: async () =>
				({
					options: {
						fontSize: 13,
						cursorBlink: true,
					},
				}) as never,
			setStatusAndMessage: vi.fn(),
			setHealth: vi.fn(),
			emitState: vi.fn(),
			setInput: vi.fn(),
			sendInput: vi.fn(),
			sendProtocolResponse: vi.fn(),
			captureCpr: vi.fn(),
			fitTerminal: vi.fn(),
			hasStarted: vi.fn(() => true),
			flushOutput: vi.fn(),
			markAttached: vi.fn(),
		});

		const activeHandle = createHandle(true);
		activeHandle.container.setAttribute('data-active', 'true');
		terminalHandles.set('ws::active', activeHandle);

		const inactiveHandle = createHandle(true);
		inactiveHandle.container.setAttribute('data-active', 'false');
		terminalHandles.set('ws::inactive', inactiveHandle);

		orchestration.terminalFontSizeController.setCursorBlink(false);
		expect(activeHandle.terminal.options.cursorBlink).toBe(false);
		expect(inactiveHandle.terminal.options.cursorBlink).toBe(false);

		orchestration.terminalFontSizeController.setCursorBlink(true);
		expect(activeHandle.terminal.options.cursorBlink).toBe(true);
		expect(inactiveHandle.terminal.options.cursorBlink).toBe(false);

		inactiveHandle.container.setAttribute('data-active', 'true');
		orchestration.terminalFontSizeController.setCursorBlink(false);
		orchestration.terminalFontSizeController.setCursorBlink(true);
		expect(inactiveHandle.terminal.options.cursorBlink).toBe(true);
	});
});
