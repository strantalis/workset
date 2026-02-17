import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	WORKSET_HUB_LAYOUT_MODE_KEY,
	WORKSET_HUB_GROUP_MODE_KEY,
	parseWorksetHubGroupMode,
	parseWorksetHubLayoutMode,
	persistWorksetHubGroupMode,
	persistWorksetHubLayoutMode,
	readWorksetHubGroupMode,
	readWorksetHubLayoutMode,
} from './worksetHubPreferences';

const installLocalStorage = (): void => {
	const store = new Map<string, string>();
	Object.defineProperty(globalThis, 'localStorage', {
		value: {
			getItem: (key: string) => store.get(key) ?? null,
			setItem: (key: string, value: string) => {
				store.set(key, String(value));
			},
			removeItem: (key: string) => {
				store.delete(key);
			},
			clear: () => {
				store.clear();
			},
		},
		configurable: true,
	});
};

describe('worksetHubPreferences', () => {
	beforeEach(() => {
		vi.resetModules();
		installLocalStorage();
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	it('defaults invalid values to expected modes', () => {
		expect(parseWorksetHubLayoutMode('bad-value')).toBe('grid');
		expect(parseWorksetHubGroupMode('bad-value')).toBe('active');
	});

	it('reads persisted values from localStorage', () => {
		persistWorksetHubLayoutMode('list');
		persistWorksetHubGroupMode('repo');

		expect(readWorksetHubLayoutMode()).toBe('list');
		expect(readWorksetHubGroupMode()).toBe('repo');
	});

	it('falls back to defaults when no values are persisted', () => {
		expect(readWorksetHubLayoutMode()).toBe('grid');
		expect(readWorksetHubGroupMode()).toBe('active');
	});

	it('persists mode selections using the expected storage keys', () => {
		persistWorksetHubLayoutMode('list');
		persistWorksetHubGroupMode('template');

		expect(localStorage.getItem(WORKSET_HUB_LAYOUT_MODE_KEY)).toBe('list');
		expect(localStorage.getItem(WORKSET_HUB_GROUP_MODE_KEY)).toBe('template');
	});
});
