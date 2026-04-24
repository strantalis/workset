import { Clipboard, Events } from '@wailsio/runtime';
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { installTerminalClipboardBridge } from './terminalClipboardBridge';

const appendWailsRuntimeScript = (): HTMLScriptElement => {
	const script = document.createElement('script');
	script.src = '/wails/custom.js';
	document.head.appendChild(script);
	return script;
};

const setNavigatorPlatform = (platform: string): void => {
	Object.defineProperty(window.navigator, 'platform', {
		configurable: true,
		value: platform,
	});
};

const createIsolatedDocument = (): Document => {
	const iframe = document.createElement('iframe');
	document.body.appendChild(iframe);
	return iframe.contentDocument ?? document;
};

describe('installTerminalClipboardBridge', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		vi.mocked(Clipboard.SetText).mockResolvedValue(undefined);
		vi.mocked(Clipboard.Text).mockResolvedValue('');
		setNavigatorPlatform('');
	});

	afterEach(() => {
		document.head.querySelectorAll('script[src*="/wails/"]').forEach((script) => script.remove());
		document.body.replaceChildren();
	});

	it('mirrors terminal copy through the Wails clipboard when runtime scripts are present', async () => {
		appendWailsRuntimeScript();
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const originalCopy = vi.fn();
		const terminal = {
			selectionManager: {
				copyToClipboard: originalCopy,
			},
		};

		installTerminalClipboardBridge(terminal as never);
		terminal.selectionManager.copyToClipboard?.('copied text', '<b>copied text</b>');
		await Promise.resolve();

		expect(setClipboardText).toHaveBeenCalledWith('copied text');
		expect(originalCopy).not.toHaveBeenCalled();
	});

	it('does not install twice for the same terminal instance', async () => {
		appendWailsRuntimeScript();
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const originalCopy = vi.fn();
		const terminal = {
			selectionManager: {
				copyToClipboard: originalCopy,
			},
		};

		installTerminalClipboardBridge(terminal as never);
		installTerminalClipboardBridge(terminal as never);
		terminal.selectionManager.copyToClipboard?.('once');
		await Promise.resolve();

		expect(setClipboardText).toHaveBeenCalledTimes(1);
		expect(originalCopy).not.toHaveBeenCalled();
	});

	it('falls back to browser copy when the native clipboard bridge fails', async () => {
		appendWailsRuntimeScript();
		const setClipboardText = vi
			.mocked(Clipboard.SetText)
			.mockRejectedValueOnce(new Error('clipboard unavailable'));
		const originalCopy = vi.fn();
		const terminal = {
			selectionManager: {
				copyToClipboard: originalCopy,
			},
		};

		installTerminalClipboardBridge(terminal as never);
		terminal.selectionManager.copyToClipboard?.('fallback text', '<b>fallback text</b>');
		await Promise.resolve();
		await Promise.resolve();

		expect(setClipboardText).toHaveBeenCalledWith('fallback text');
		expect(originalCopy).toHaveBeenCalledWith('fallback text', '<b>fallback text</b>');
	});

	it('preserves the terminal copy path and routes clipboard writes through Wails', async () => {
		appendWailsRuntimeScript();
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const copyToClipboard = vi.fn();
		const copySelection = vi.fn(() => false);
		const getSelection = vi.fn(() => 'plain fallback selection');
		const terminal = {
			selectionManager: {
				copySelection,
				copyToClipboard,
				getSelection,
			},
		};

		installTerminalClipboardBridge(terminal as never);
		const copied = terminal.selectionManager.copySelection?.();
		await Promise.resolve();

		expect(copied).toBe(true);
		expect(getSelection).toHaveBeenCalledTimes(1);
		expect(copySelection).not.toHaveBeenCalled();
		expect(setClipboardText).toHaveBeenCalledWith('plain fallback selection');
		expect(copyToClipboard).not.toHaveBeenCalled();
	});

	it('does not re-enter the terminal formatter path when no plain-text selection is available', async () => {
		appendWailsRuntimeScript();
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const copySelection = vi.fn(() => true);
		const terminal = {
			selectionManager: {
				copySelection,
				copyToClipboard: vi.fn(),
				getSelection: vi.fn(() => ''),
			},
		};

		installTerminalClipboardBridge(terminal as never);
		const copied = terminal.selectionManager.copySelection?.();
		await Promise.resolve();

		expect(copied).toBe(false);
		expect(copySelection).not.toHaveBeenCalled();
		expect(setClipboardText).not.toHaveBeenCalled();
	});

	it('does not re-enter the terminal formatter path when getSelection throws', async () => {
		appendWailsRuntimeScript();
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const copySelection = vi.fn(() => true);
		const terminal = {
			selectionManager: {
				copySelection,
				copyToClipboard: vi.fn(),
				getSelection: vi.fn(() => {
					throw new Error('selection failed');
				}),
			},
		};

		installTerminalClipboardBridge(terminal as never);
		const copied = terminal.selectionManager.copySelection?.();
		await Promise.resolve();

		expect(copied).toBe(false);
		expect(copySelection).not.toHaveBeenCalled();
		expect(setClipboardText).not.toHaveBeenCalled();
	});

	it('pastes from the native Wails clipboard on paste events', async () => {
		appendWailsRuntimeScript();
		const readClipboardText = vi.mocked(Clipboard.Text).mockResolvedValueOnce('native pasted text');
		const host = document.createElement('div');
		const textarea = document.createElement('textarea');
		host.appendChild(textarea);
		const paste = vi.fn();
		const terminal = {
			element: host,
			textarea,
			paste,
			selectionManager: {
				copyToClipboard: vi.fn(),
				copySelection: vi.fn(),
				getSelection: vi.fn(() => ''),
			},
		};

		installTerminalClipboardBridge(terminal as never);

		const event = new Event('paste', { bubbles: true, cancelable: true });
		Object.defineProperty(event, 'clipboardData', {
			value: {
				getData: vi.fn(() => ''),
			},
		});
		host.dispatchEvent(event);
		await Promise.resolve();

		expect(readClipboardText).toHaveBeenCalledTimes(1);
		expect(paste).toHaveBeenCalledWith('native pasted text');
		expect(event.defaultPrevented).toBe(true);
	});

	it('pastes from the native Wails clipboard on the terminal paste shortcut', async () => {
		appendWailsRuntimeScript();
		const readClipboardText = vi
			.mocked(Clipboard.Text)
			.mockResolvedValueOnce('shortcut pasted text');
		const host = document.createElement('div');
		const textarea = document.createElement('textarea');
		host.appendChild(textarea);
		const paste = vi.fn();
		const terminal = {
			element: host,
			textarea,
			paste,
			selectionManager: {
				copyToClipboard: vi.fn(),
				copySelection: vi.fn(),
				getSelection: vi.fn(() => ''),
			},
		};

		installTerminalClipboardBridge(terminal as never);

		const event = new KeyboardEvent('keydown', {
			bubbles: true,
			cancelable: true,
			code: 'KeyV',
			key: 'v',
			ctrlKey: true,
			shiftKey: true,
		});
		host.dispatchEvent(event);
		await Promise.resolve();

		expect(readClipboardText).toHaveBeenCalledTimes(1);
		expect(paste).toHaveBeenCalledWith('shortcut pasted text');
		expect(event.defaultPrevented).toBe(true);
	});

	it('copies through the native Wails clipboard on the mac terminal copy shortcut', async () => {
		appendWailsRuntimeScript();
		setNavigatorPlatform('MacIntel');
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const host = document.createElement('div');
		const textarea = document.createElement('textarea');
		host.appendChild(textarea);
		const terminal = {
			element: host,
			textarea,
			selectionManager: {
				copyToClipboard: vi.fn(),
				copySelection: vi.fn(),
				getSelection: vi.fn(() => 'shortcut copied text'),
			},
		};

		installTerminalClipboardBridge(terminal as never);

		const event = new KeyboardEvent('keydown', {
			bubbles: true,
			cancelable: true,
			code: 'KeyC',
			key: 'c',
			metaKey: true,
		});
		host.dispatchEvent(event);
		await Promise.resolve();

		expect(setClipboardText).toHaveBeenCalledWith('shortcut copied text');
		expect(event.defaultPrevented).toBe(true);
	});

	it('copies through the active terminal when mac copy is captured at the document boundary', async () => {
		appendWailsRuntimeScript();
		setNavigatorPlatform('MacIntel');
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const host = document.createElement('div');
		host.dataset.active = 'true';
		const textarea = document.createElement('textarea');
		host.appendChild(textarea);
		document.body.appendChild(host);
		const terminal = {
			element: host,
			textarea,
			selectionManager: {
				copyToClipboard: vi.fn(),
				copySelection: vi.fn(),
				getSelection: vi.fn(() => 'document copied text'),
			},
		};

		installTerminalClipboardBridge(terminal as never);

		const event = new KeyboardEvent('keydown', {
			bubbles: true,
			cancelable: true,
			code: 'KeyC',
			key: 'c',
			metaKey: true,
		});
		document.dispatchEvent(event);
		await Promise.resolve();

		expect(setClipboardText).toHaveBeenCalledWith('document copied text');
		expect(event.defaultPrevented).toBe(true);
	});

	it('handles native copy events for the active terminal without a keydown event', async () => {
		appendWailsRuntimeScript();
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const host = document.createElement('div');
		host.dataset.active = 'true';
		document.body.appendChild(host);
		const terminal = {
			element: host,
			selectionManager: {
				copyToClipboard: vi.fn(),
				copySelection: vi.fn(),
				getSelection: vi.fn(() => 'native command copied text'),
			},
		};

		installTerminalClipboardBridge(terminal as never);

		const event = new Event('copy', { bubbles: true, cancelable: true });
		document.dispatchEvent(event);
		await Promise.resolve();

		expect(setClipboardText).toHaveBeenCalledWith('native command copied text');
		expect(event.defaultPrevented).toBe(true);
	});

	it('routes the Wails native copy command to the active terminal selection', async () => {
		appendWailsRuntimeScript();
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const isolatedDocument = createIsolatedDocument();
		const host = isolatedDocument.createElement('div');
		host.dataset.active = 'true';
		isolatedDocument.body.appendChild(host);
		const terminal = {
			element: host,
			getSelection: vi.fn(() => 'wails command copied text'),
			selectionManager: {
				copyToClipboard: vi.fn(),
			},
		};

		installTerminalClipboardBridge(terminal as never);
		const nativeCopyHandler = vi
			.mocked(Events.On)
			.mock.calls.find(([eventName]) => eventName === 'workset:native-copy-command')?.[1];
		nativeCopyHandler?.({ name: 'workset:native-copy-command', data: undefined });
		await Promise.resolve();

		expect(setClipboardText).toHaveBeenCalledWith('wails command copied text');
	});

	it('falls back to DOM copy for Wails native copy commands in editable controls', () => {
		appendWailsRuntimeScript();
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const isolatedDocument = createIsolatedDocument();
		const host = isolatedDocument.createElement('div');
		host.dataset.active = 'true';
		isolatedDocument.body.appendChild(host);
		const input = isolatedDocument.createElement('input');
		isolatedDocument.body.appendChild(input);
		input.focus();
		const execCommand = vi.fn(() => true);
		Object.defineProperty(isolatedDocument, 'execCommand', {
			configurable: true,
			value: execCommand,
		});
		const terminal = {
			element: host,
			getSelection: vi.fn(() => ''),
			selectionManager: {
				copyToClipboard: vi.fn(),
			},
		};

		installTerminalClipboardBridge(terminal as never);
		const nativeCopyHandler = vi
			.mocked(Events.On)
			.mock.calls.find(([eventName]) => eventName === 'workset:native-copy-command')?.[1];
		nativeCopyHandler?.({ name: 'workset:native-copy-command', data: undefined });

		expect(setClipboardText).not.toHaveBeenCalled();
		expect(execCommand).toHaveBeenCalledWith('copy');
	});

	it('does not steal mac copy from editable controls outside the terminal', async () => {
		appendWailsRuntimeScript();
		setNavigatorPlatform('MacIntel');
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const host = document.createElement('div');
		host.dataset.active = 'true';
		document.body.appendChild(host);
		const input = document.createElement('input');
		document.body.appendChild(input);
		const terminal = {
			element: host,
			selectionManager: {
				copyToClipboard: vi.fn(),
				copySelection: vi.fn(),
				getSelection: vi.fn(() => 'terminal selection'),
			},
		};

		installTerminalClipboardBridge(terminal as never);

		const event = new KeyboardEvent('keydown', {
			bubbles: true,
			cancelable: true,
			code: 'KeyC',
			key: 'c',
			metaKey: true,
		});
		input.dispatchEvent(event);
		await Promise.resolve();

		expect(setClipboardText).not.toHaveBeenCalled();
		expect(event.defaultPrevented).toBe(false);
	});

	it('does not consume the mac terminal copy shortcut when there is no selection', async () => {
		appendWailsRuntimeScript();
		setNavigatorPlatform('MacIntel');
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const host = document.createElement('div');
		const textarea = document.createElement('textarea');
		host.appendChild(textarea);
		const terminal = {
			element: host,
			textarea,
			selectionManager: {
				copyToClipboard: vi.fn(),
				copySelection: vi.fn(),
				getSelection: vi.fn(() => ''),
			},
		};

		installTerminalClipboardBridge(terminal as never);

		const event = new KeyboardEvent('keydown', {
			bubbles: true,
			cancelable: true,
			code: 'KeyC',
			key: 'c',
			metaKey: true,
		});
		host.dispatchEvent(event);
		await Promise.resolve();

		expect(setClipboardText).not.toHaveBeenCalled();
		expect(event.defaultPrevented).toBe(false);
	});

	it('leaves browser-only runs untouched', async () => {
		const setClipboardText = vi.mocked(Clipboard.SetText);
		const originalCopy = vi.fn();
		const terminal = {
			selectionManager: {
				copyToClipboard: originalCopy,
			},
		};

		installTerminalClipboardBridge(terminal as never);
		terminal.selectionManager.copyToClipboard?.('browser copy');
		await Promise.resolve();

		expect(setClipboardText).not.toHaveBeenCalled();
		expect(originalCopy).toHaveBeenCalledWith('browser copy');
	});
});
