import { describe, expect, it, vi } from 'vitest';
import {
	createSidebarResizeController,
	REPO_DIFF_DEFAULT_SIDEBAR_WIDTH,
	REPO_DIFF_MIN_SIDEBAR_WIDTH,
	REPO_DIFF_SIDEBAR_WIDTH_KEY,
} from './sidebarResizeController';

const createStorage = (initial: Record<string, string> = {}) => {
	const map = new Map<string, string>(Object.entries(initial));
	return {
		getItem: vi.fn((key: string) => map.get(key) ?? null),
		setItem: vi.fn((key: string, value: string) => {
			map.set(key, value);
		}),
	};
};

const createSetup = (storage = createStorage()) => {
	const state = {
		sidebarWidth: REPO_DIFF_DEFAULT_SIDEBAR_WIDTH,
		isResizing: false,
	};

	const controller = createSidebarResizeController({
		document,
		window,
		storage,
		storageKey: REPO_DIFF_SIDEBAR_WIDTH_KEY,
		minWidth: REPO_DIFF_MIN_SIDEBAR_WIDTH,
		getSidebarWidth: () => state.sidebarWidth,
		setSidebarWidth: (value) => {
			state.sidebarWidth = value;
		},
		setIsResizing: (value) => {
			state.isResizing = value;
		},
	});

	return { state, storage, controller };
};

describe('sidebarResizeController', () => {
	it('loads a persisted width when valid and ignores invalid values', () => {
		const setup = createSetup(createStorage({ [REPO_DIFF_SIDEBAR_WIDTH_KEY]: '360' }));
		setup.controller.loadPersistedWidth();
		expect(setup.state.sidebarWidth).toBe(360);

		const invalid = createSetup(createStorage({ [REPO_DIFF_SIDEBAR_WIDTH_KEY]: '100' }));
		invalid.controller.loadPersistedWidth();
		expect(invalid.state.sidebarWidth).toBe(REPO_DIFF_DEFAULT_SIDEBAR_WIDTH);
	});

	it('resizes with mousemove, clamps to min width, and persists on mouseup', () => {
		const setup = createSetup();

		setup.controller.startResize(new MouseEvent('mousedown', { clientX: 200 }));
		expect(setup.state.isResizing).toBe(true);

		document.dispatchEvent(new MouseEvent('mousemove', { clientX: 280 }));
		expect(setup.state.sidebarWidth).toBe(360);

		document.dispatchEvent(new MouseEvent('mousemove', { clientX: -100 }));
		expect(setup.state.sidebarWidth).toBe(REPO_DIFF_MIN_SIDEBAR_WIDTH);

		document.dispatchEvent(new MouseEvent('mouseup'));
		expect(setup.state.isResizing).toBe(false);
		expect(setup.storage.setItem).toHaveBeenCalledWith(
			REPO_DIFF_SIDEBAR_WIDTH_KEY,
			String(REPO_DIFF_MIN_SIDEBAR_WIDTH),
		);

		document.dispatchEvent(new MouseEvent('mousemove', { clientX: 600 }));
		expect(setup.state.sidebarWidth).toBe(REPO_DIFF_MIN_SIDEBAR_WIDTH);
	});

	it('stops resizing on blur without persisting width', () => {
		const setup = createSetup();
		setup.controller.startResize(new MouseEvent('mousedown', { clientX: 200 }));

		document.dispatchEvent(new MouseEvent('mousemove', { clientX: 300 }));
		expect(setup.state.sidebarWidth).toBe(380);

		window.dispatchEvent(new Event('blur'));
		expect(setup.state.isResizing).toBe(false);
		expect(setup.storage.setItem).not.toHaveBeenCalled();

		document.dispatchEvent(new MouseEvent('mousemove', { clientX: 500 }));
		expect(setup.state.sidebarWidth).toBe(380);
	});

	it('cleans listeners when destroyed during an active resize', () => {
		const setup = createSetup();
		setup.controller.startResize(new MouseEvent('mousedown', { clientX: 200 }));

		document.dispatchEvent(new MouseEvent('mousemove', { clientX: 240 }));
		expect(setup.state.sidebarWidth).toBe(320);

		setup.controller.destroy();
		document.dispatchEvent(new MouseEvent('mousemove', { clientX: 400 }));
		expect(setup.state.sidebarWidth).toBe(320);
		expect(setup.storage.setItem).not.toHaveBeenCalled();
	});
});
