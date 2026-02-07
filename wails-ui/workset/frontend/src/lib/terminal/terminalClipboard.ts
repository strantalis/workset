type ClipboardSelection = string;

const MAX_CLIPBOARD_BYTES = 1024 * 1024;
const textEncoder = typeof TextEncoder !== 'undefined' ? new TextEncoder() : null;
const textDecoder = typeof TextDecoder !== 'undefined' ? new TextDecoder() : null;

const getClipboardPayloadBytes = (value: string): number => {
	if (!value) return 0;
	if (textEncoder) {
		return textEncoder.encode(value).length;
	}
	return value.length;
};

const encodeClipboardText = (value: string): string => {
	if (!value) return '';
	if (typeof btoa === 'function') {
		if (textEncoder) {
			const bytes = textEncoder.encode(value);
			let binary = '';
			for (const byte of bytes) {
				binary += String.fromCharCode(byte);
			}
			return btoa(binary);
		}
		try {
			return btoa(value);
		} catch {
			return '';
		}
	}
	return '';
};

const decodeClipboardText = (value: string): string => {
	if (!value) return '';
	const sanitized = value.replace(/\s+/g, '').replace(/-/g, '+').replace(/_/g, '/');
	const padding = sanitized.length % 4;
	const normalized = padding ? sanitized.padEnd(sanitized.length + (4 - padding), '=') : sanitized;
	try {
		if (typeof atob === 'function') {
			const binary = atob(normalized);
			if (textDecoder) {
				const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
				return textDecoder.decode(bytes);
			}
			return binary;
		}
	} catch {
		// Ignore invalid base64.
	}
	return '';
};

const getRuntimeClipboard = (): ((text: string) => Promise<boolean>) | null => {
	if (typeof window === 'undefined') return null;
	const runtime = (
		window as Window & {
			runtime?: { ClipboardSetText?: (text: string) => Promise<boolean> };
		}
	).runtime;
	if (!runtime?.ClipboardSetText) return null;
	return runtime.ClipboardSetText.bind(runtime);
};

export const createTerminalClipboardBase64 = (): {
	encodeText: (data: string) => string;
	decodeText: (data: string) => string;
} => ({
	encodeText: encodeClipboardText,
	decodeText: decodeClipboardText,
});

export const createTerminalClipboardProvider = (): {
	readText: (selection: ClipboardSelection) => Promise<string>;
	writeText: (selection: ClipboardSelection, text: string) => Promise<void>;
} => ({
	readText: async (_selection) => '',
	writeText: async (_selection, text) => {
		if (!text) return;
		if (getClipboardPayloadBytes(text) > MAX_CLIPBOARD_BYTES) return;
		const runtimeClipboard = getRuntimeClipboard();
		if (runtimeClipboard) {
			try {
				const ok = await runtimeClipboard(text);
				if (ok) return;
			} catch {
				// Fall back to browser clipboard.
			}
		}
		if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) return;
		try {
			await navigator.clipboard.writeText(text);
		} catch {
			// Ignore clipboard failures (permissions or missing API).
		}
	},
});
