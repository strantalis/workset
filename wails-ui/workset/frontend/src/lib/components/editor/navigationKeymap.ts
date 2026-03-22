import { keymap, type KeyBinding } from '@codemirror/view';
import type { Extension } from '@codemirror/state';

export interface NavigationCallbacks {
	onPrevFile?: () => void;
	onNextFile?: () => void;
}

/**
 * CodeMirror keymap extension for navigating between changed files
 * while the editor has focus. Prevents the editor from being a
 * keyboard "trap" by providing Alt+[ / Alt+] shortcuts.
 */
export function navigationKeymap(callbacks: NavigationCallbacks): Extension {
	const bindings: KeyBinding[] = [];

	if (callbacks.onPrevFile) {
		const cb = callbacks.onPrevFile;
		bindings.push({
			key: 'Alt-[',
			run: () => {
				cb();
				return true;
			},
		});
	}

	if (callbacks.onNextFile) {
		const cb = callbacks.onNextFile;
		bindings.push({
			key: 'Alt-]',
			run: () => {
				cb();
				return true;
			},
		});
	}

	return keymap.of(bindings);
}
