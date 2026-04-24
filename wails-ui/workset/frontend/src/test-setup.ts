import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

vi.mock('@wailsio/runtime', () => ({
	Call: {
		ByID: vi.fn(async () => undefined),
	},
	CancellablePromise: Promise,
	Create: {
		Any: (value: unknown) => value,
		Array:
			<T>(createValue: (value: unknown) => T) =>
			(value: unknown): T[] => {
				if (!Array.isArray(value)) return [];
				return value.map((entry) => createValue(entry));
			},
		Nullable:
			<T>(createValue: (value: unknown) => T) =>
			(value: unknown): T | null => {
				if (value === null || value === undefined) return null;
				return createValue(value);
			},
		Map:
			<TKey, TValue>(createKey: (key: unknown) => TKey, createValue: (value: unknown) => TValue) =>
			(value: unknown): Record<string, TValue> => {
				if (value === null || typeof value !== 'object' || Array.isArray(value)) {
					return {};
				}
				const out: Record<string, TValue> = {};
				for (const [entryKey, entryValue] of Object.entries(value)) {
					const normalizedKey = createKey(entryKey);
					out[String(normalizedKey)] = createValue(entryValue);
				}
				return out;
			},
		Events: {},
	},
	Log: {
		Debug: vi.fn(),
		Info: vi.fn(),
		Warn: vi.fn(),
		Error: vi.fn(),
	},
	Browser: {
		OpenURL: vi.fn(async () => undefined),
	},
	Clipboard: {
		SetText: vi.fn(async () => undefined),
		Text: vi.fn(async () => ''),
	},
	Events: {
		On: vi.fn(() => () => undefined),
		Off: vi.fn(),
		Emit: vi.fn(async () => undefined),
	},
}));
// Auto-cleanup is handled by svelteTesting() vite plugin
