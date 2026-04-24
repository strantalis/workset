import { Clipboard, Events } from '@wailsio/runtime';

type TerminalSelectionManagerLike = {
	copyToClipboard?: (text: string, html?: string) => void;
	copySelection?: () => boolean;
	getSelection?: () => string;
};

type TerminalWithClipboardBridge = {
	copySelection?: () => boolean;
	getSelection?: () => string;
	hasSelection?: () => boolean;
	onSelectionChange?: (callback: () => void) => { dispose: () => void };
	selectionManager?: TerminalSelectionManagerLike;
	paste?: (text: string) => void;
	element?: HTMLElement;
	textarea?: HTMLTextAreaElement;
	__worksetClipboardBridgeInstalled?: boolean;
};

type ClipboardBridgeDebugDetails = Record<string, unknown>;
type ClipboardBridgeDebugLogger = (event: string, details: ClipboardBridgeDebugDetails) => void;

type ClipboardBridgeOptions = {
	logDebug?: ClipboardBridgeDebugLogger;
};

const NATIVE_COPY_COMMAND_EVENT = 'workset:native-copy-command';
const terminalsByDocument = new WeakMap<Document, Set<TerminalWithClipboardBridge>>();
const documentsWithCopyBridge = new WeakSet<Document>();
const loggersByTerminal = new WeakMap<TerminalWithClipboardBridge, ClipboardBridgeDebugLogger>();
const terminalsWithSelectionDebug = new WeakSet<TerminalWithClipboardBridge>();
const terminalsWithMouseDebug = new WeakSet<TerminalWithClipboardBridge>();

const logClipboardDebug = (
	terminal: TerminalWithClipboardBridge | null | undefined,
	event: string,
	details: ClipboardBridgeDebugDetails = {},
): void => {
	const logDebug = terminal ? loggersByTerminal.get(terminal) : undefined;
	if (!logDebug) {
		return;
	}
	logDebug(`terminal_clipboard_${event}`, details);
};

const isMacPlatform = (): boolean =>
	typeof navigator !== 'undefined' &&
	/(Mac|iPhone|iPod|iPad)/i.test(navigator.platform || navigator.userAgent);

const isTerminalPasteShortcut = (event: KeyboardEvent): boolean => {
	if (event.code !== 'KeyV') {
		return false;
	}

	if (isMacPlatform()) {
		return event.metaKey && !event.ctrlKey && !event.altKey && !event.shiftKey;
	}

	return event.ctrlKey && event.shiftKey && !event.metaKey && !event.altKey;
};

const isTerminalCopyShortcut = (event: KeyboardEvent): boolean => {
	if (event.code !== 'KeyC') {
		return false;
	}

	if (isMacPlatform()) {
		return event.metaKey && !event.ctrlKey && !event.altKey && !event.shiftKey;
	}

	return event.ctrlKey && event.shiftKey && !event.metaKey && !event.altKey;
};

const getTerminalDocument = (terminal: TerminalWithClipboardBridge): Document | null => {
	return (
		terminal.element?.ownerDocument ??
		terminal.textarea?.ownerDocument ??
		(typeof document === 'undefined' ? null : document)
	);
};

const getTargetElement = (target: EventTarget | null): Element | null => {
	if (!target || !('nodeType' in target)) {
		return null;
	}

	const node = target as Node;
	if (node.nodeType === Node.ELEMENT_NODE) {
		return node as Element;
	}
	return node.parentElement;
};

const isEditableTarget = (target: EventTarget | null): boolean => {
	const element = getTargetElement(target);
	if (!element) {
		return false;
	}
	return Boolean(
		element.closest('input, textarea, select, [contenteditable=""], [contenteditable="true"]'),
	);
};

const isTargetInsideTerminal = (
	terminal: TerminalWithClipboardBridge,
	target: EventTarget | null,
): boolean => {
	const element = getTargetElement(target);
	if (!element) {
		return false;
	}
	return Boolean(
		(terminal.element && terminal.element.contains(element)) ||
		(terminal.textarea && terminal.textarea.contains(element)),
	);
};

const isTerminalConnected = (terminal: TerminalWithClipboardBridge): boolean =>
	Boolean(terminal.element?.isConnected || terminal.textarea?.isConnected);

const isTerminalActive = (terminal: TerminalWithClipboardBridge): boolean =>
	Boolean(
		terminal.element?.closest('[data-active="true"]') ||
		terminal.textarea?.closest('[data-active="true"]'),
	);

const copySelectionToNativeClipboard = (
	terminal: TerminalWithClipboardBridge,
	source: TerminalSelectionManagerLike | undefined,
	fallbackCopyToClipboard?: (text: string, html?: string) => void,
): boolean => {
	try {
		const text = source?.getSelection?.() ?? '';
		logClipboardDebug(terminal, 'selection_read', {
			source: source === terminal ? 'terminal' : 'selection_manager',
			hasText: text.length > 0,
			length: text.length,
		});
		if (!text) {
			return false;
		}
		void Clipboard.SetText(text)
			.then(() => {
				logClipboardDebug(terminal, 'set_text_ok', { length: text.length });
			})
			.catch((error: unknown) => {
				logClipboardDebug(terminal, 'set_text_failed', {
					length: text.length,
					error: error instanceof Error ? error.message : String(error),
					hasFallback: typeof fallbackCopyToClipboard === 'function',
				});
				fallbackCopyToClipboard?.(text);
			});
		return true;
	} catch (error) {
		logClipboardDebug(terminal, 'selection_read_failed', {
			error: error instanceof Error ? error.message : String(error),
		});
		return false;
	}
};

const getTerminalCopySource = (
	terminal: TerminalWithClipboardBridge,
): TerminalSelectionManagerLike | undefined => {
	if (typeof terminal.getSelection === 'function') {
		return terminal;
	}
	return terminal.selectionManager;
};

const findTerminalForCopyEvent = (
	doc: Document,
	target: EventTarget | null,
): TerminalWithClipboardBridge | null => {
	const terminals = terminalsByDocument.get(doc);
	if (!terminals) {
		return null;
	}

	const connectedTerminals = Array.from(terminals).filter(isTerminalConnected);
	if (connectedTerminals.length === 0) {
		return null;
	}

	const targetTerminal = connectedTerminals.find((terminal) =>
		isTargetInsideTerminal(terminal, target),
	);
	if (targetTerminal) {
		return targetTerminal;
	}

	if (isEditableTarget(target)) {
		return null;
	}

	return connectedTerminals.find(isTerminalActive) ?? null;
};

const copyFocusedEditableSelection = (doc: Document): boolean => {
	if (!isEditableTarget(doc.activeElement)) {
		return false;
	}
	try {
		return doc.execCommand('copy');
	} catch {
		return false;
	}
};

const installDocumentCopyEventBridge = (terminal: TerminalWithClipboardBridge): void => {
	const doc = getTerminalDocument(terminal);
	if (!doc) {
		return;
	}

	let terminals = terminalsByDocument.get(doc);
	if (!terminals) {
		terminals = new Set<TerminalWithClipboardBridge>();
		terminalsByDocument.set(doc, terminals);
	}
	terminals.add(terminal);

	if (documentsWithCopyBridge.has(doc)) {
		return;
	}
	documentsWithCopyBridge.add(doc);

	const handleCopyCommand = (target: EventTarget | null): boolean => {
		const targetTerminal = findTerminalForCopyEvent(doc, target);
		logClipboardDebug(targetTerminal ?? terminal, 'copy_command', {
			hasTargetTerminal: Boolean(targetTerminal),
			targetTag: getTargetElement(target)?.tagName ?? null,
			targetEditable: isEditableTarget(target),
			targetInsideTerminal: targetTerminal ? isTargetInsideTerminal(targetTerminal, target) : false,
			activeElementTag: doc.activeElement?.tagName ?? null,
		});
		if (!targetTerminal) {
			return false;
		}
		return copySelectionToNativeClipboard(targetTerminal, getTerminalCopySource(targetTerminal));
	};

	doc.addEventListener(
		'copy',
		(event) => {
			logClipboardDebug(terminal, 'dom_copy_event', {
				targetTag: getTargetElement(event.target)?.tagName ?? null,
				cancelable: event.cancelable,
			});
			if (!handleCopyCommand(event.target)) {
				return;
			}
			event.preventDefault();
			event.stopPropagation();
		},
		true,
	);

	doc.addEventListener(
		'keydown',
		(event) => {
			if (!isTerminalCopyShortcut(event)) {
				return;
			}
			logClipboardDebug(terminal, 'dom_keydown_copy', {
				code: event.code,
				key: event.key,
				metaKey: event.metaKey,
				ctrlKey: event.ctrlKey,
				shiftKey: event.shiftKey,
				altKey: event.altKey,
				targetTag: getTargetElement(event.target)?.tagName ?? null,
			});
			if (!handleCopyCommand(event.target)) {
				logClipboardDebug(terminal, 'dom_keydown_copy_unhandled');
				return;
			}
			event.preventDefault();
			event.stopPropagation();
		},
		true,
	);

	Events.On(NATIVE_COPY_COMMAND_EVENT, (event) => {
		logClipboardDebug(terminal, 'native_event', {
			sender: event.sender ?? null,
			data: event.data ?? null,
			activeElementTag: doc.activeElement?.tagName ?? null,
		});
		if (handleCopyCommand(doc.activeElement)) {
			return;
		}
		const copiedEditable = copyFocusedEditableSelection(doc);
		logClipboardDebug(terminal, 'native_event_fallback', {
			copiedEditable,
			activeElementTag: doc.activeElement?.tagName ?? null,
		});
	});
};

const installNativeClipboardEventBridge = (terminal: TerminalWithClipboardBridge): void => {
	if (typeof terminal.paste !== 'function') {
		// Copy still works without terminal.paste, so continue below.
	}

	const pasteFromNativeClipboard = async (fallbackText: string): Promise<void> => {
		if (typeof terminal.paste !== 'function') {
			return;
		}

		try {
			const text = await Clipboard.Text();
			if (text) {
				terminal.paste?.(text);
				return;
			}
		} catch {
			// Fall back to event clipboard data below when native clipboard reads fail.
		}

		if (fallbackText) {
			terminal.paste?.(fallbackText);
		}
	};

	const handlePaste = (event: Event): void => {
		const clipboardData =
			'clipboardData' in event
				? ((event as Event & { clipboardData?: DataTransfer | null }).clipboardData ?? null)
				: null;
		const fallbackText = clipboardData?.getData('text/plain') ?? '';
		event.preventDefault();
		event.stopPropagation();
		void pasteFromNativeClipboard(fallbackText);
	};

	const handleKeyDown = (event: KeyboardEvent): void => {
		if (isTerminalCopyShortcut(event)) {
			const copied = terminal.selectionManager?.copySelection?.() ?? false;
			if (copied) {
				logClipboardDebug(terminal, 'terminal_keydown_copy_selection_manager', {
					copied,
				});
				event.preventDefault();
				event.stopPropagation();
			}
			return;
		}

		if (!isTerminalPasteShortcut(event)) {
			return;
		}
		event.preventDefault();
		event.stopPropagation();
		void pasteFromNativeClipboard('');
	};

	terminal.element?.addEventListener('paste', handlePaste, true);
	terminal.textarea?.addEventListener('paste', handlePaste, true);
	terminal.element?.addEventListener('keydown', handleKeyDown, true);
	terminal.textarea?.addEventListener('keydown', handleKeyDown, true);
};

const installSelectionDebugBridge = (terminal: TerminalWithClipboardBridge): void => {
	if (terminalsWithSelectionDebug.has(terminal)) {
		return;
	}
	terminalsWithSelectionDebug.add(terminal);
	if (typeof terminal.onSelectionChange !== 'function') {
		logClipboardDebug(terminal, 'selection_debug_unavailable');
		return;
	}
	terminal.onSelectionChange(() => {
		let selectionDetails: { hasSelection: boolean; length: number };
		try {
			selectionDetails = {
				hasSelection: terminal.hasSelection?.() ?? false,
				length: terminal.getSelection?.().length ?? 0,
			};
		} catch (error) {
			logClipboardDebug(terminal, 'selection_change_read_failed', {
				error: error instanceof Error ? error.message : String(error),
			});
			return;
		}
		logClipboardDebug(terminal, 'selection_change', selectionDetails);
	});
};

const installMouseSelectionDebugBridge = (terminal: TerminalWithClipboardBridge): void => {
	if (terminalsWithMouseDebug.has(terminal)) {
		return;
	}
	terminalsWithMouseDebug.add(terminal);
	const element = terminal.element;
	if (!element) {
		logClipboardDebug(terminal, 'mouse_debug_unavailable');
		return;
	}
	let dragging = false;
	let loggedDragMove = false;
	element.addEventListener(
		'mousedown',
		(event) => {
			dragging = event.button === 0;
			loggedDragMove = false;
			logClipboardDebug(terminal, 'mouse_down', {
				button: event.button,
				buttons: event.buttons,
				targetTag: getTargetElement(event.target)?.tagName ?? null,
				defaultPrevented: event.defaultPrevented,
				clientX: event.clientX,
				clientY: event.clientY,
			});
		},
		true,
	);
	element.addEventListener(
		'mousemove',
		(event) => {
			if (!dragging || loggedDragMove || event.buttons === 0) {
				return;
			}
			loggedDragMove = true;
			logClipboardDebug(terminal, 'mouse_drag_move', {
				buttons: event.buttons,
				targetTag: getTargetElement(event.target)?.tagName ?? null,
				defaultPrevented: event.defaultPrevented,
				clientX: event.clientX,
				clientY: event.clientY,
			});
		},
		true,
	);
	element.ownerDocument.addEventListener(
		'mouseup',
		(event) => {
			if (!dragging) {
				return;
			}
			dragging = false;
			logClipboardDebug(terminal, 'mouse_up', {
				button: event.button,
				buttons: event.buttons,
				targetTag: getTargetElement(event.target)?.tagName ?? null,
				defaultPrevented: event.defaultPrevented,
				clientX: event.clientX,
				clientY: event.clientY,
			});
		},
		true,
	);
};

const hasWailsRuntimeScript = (): boolean => {
	if (typeof document === 'undefined') {
		return false;
	}

	return Array.from(document.scripts).some((script) => {
		const src = script.getAttribute('src') ?? '';
		return src.includes('/wails/');
	});
};

export const installTerminalClipboardBridge = (
	terminal: unknown,
	options: ClipboardBridgeOptions = {},
): void => {
	const candidate = terminal as TerminalWithClipboardBridge;
	if (options.logDebug) {
		loggersByTerminal.set(candidate, options.logDebug);
	}
	if (candidate.__worksetClipboardBridgeInstalled) {
		logClipboardDebug(candidate, 'install_skip_existing');
		return;
	}
	candidate.__worksetClipboardBridgeInstalled = true;

	if (!hasWailsRuntimeScript()) {
		logClipboardDebug(candidate, 'install_skip_no_wails_runtime');
		return;
	}

	const selectionManager = candidate.selectionManager;
	const hasSelectionSource =
		typeof candidate.getSelection === 'function' ||
		typeof selectionManager?.getSelection === 'function' ||
		typeof selectionManager?.copyToClipboard === 'function';

	if (hasSelectionSource) {
		logClipboardDebug(candidate, 'install', {
			hasTerminalGetSelection: typeof candidate.getSelection === 'function',
			hasTerminalHasSelection: typeof candidate.hasSelection === 'function',
			hasTerminalSelectionChange: typeof candidate.onSelectionChange === 'function',
			hasSelectionManagerGetSelection: typeof selectionManager?.getSelection === 'function',
			hasSelectionManagerCopyToClipboard: typeof selectionManager?.copyToClipboard === 'function',
			hasElement: Boolean(candidate.element),
			hasTextarea: Boolean(candidate.textarea),
		});
		installDocumentCopyEventBridge(candidate);
		installSelectionDebugBridge(candidate);
		installMouseSelectionDebugBridge(candidate);
	}

	if (!selectionManager || typeof selectionManager.copyToClipboard !== 'function') {
		return;
	}

	const originalCopyToClipboard = selectionManager.copyToClipboard.bind(selectionManager);
	selectionManager.copyToClipboard = (text: string, html?: string): void => {
		// In the desktop app, prefer the native clipboard bridge and only
		// fall back to browser copy behavior if the runtime call fails.
		void Clipboard.SetText(text).catch(() => {
			originalCopyToClipboard(text, html);
		});
	};

	selectionManager.copySelection = (): boolean => {
		return copySelectionToNativeClipboard(candidate, selectionManager, originalCopyToClipboard);
	};

	installNativeClipboardEventBridge(candidate);
};
