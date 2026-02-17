import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import TerminalCockpitView from './TerminalCockpitView.svelte';
import type { Workspace } from '../../types';

const COLLAPSED_KEY = 'workset:terminal-cockpit:sidebarCollapsed';

const buildWorkspace = (overrides: Partial<Workspace> = {}): Workspace => ({
	id: 'ws-1',
	name: 'workspace-one',
	path: '/tmp/workspace-one',
	archived: false,
	repos: [],
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: new Date().toISOString(),
	...overrides,
});

const createStorageMock = () => {
	const store: Record<string, string> = {};
	return {
		getItem: vi.fn((key: string) => store[key] ?? null),
		setItem: vi.fn((key: string, value: string) => {
			store[key] = value;
		}),
		removeItem: vi.fn((key: string) => {
			delete store[key];
		}),
		clear: vi.fn(() => {
			Object.keys(store).forEach((key) => delete store[key]);
		}),
	};
};

describe('TerminalCockpitView', () => {
	let storageMock: ReturnType<typeof createStorageMock>;

	beforeEach(() => {
		storageMock = createStorageMock();
		vi.stubGlobal('localStorage', storageMock);
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	test('routes file-system add button to onAddRepo', async () => {
		const onAddRepo = vi.fn<(workspaceId: string) => void>();
		const { container } = render(TerminalCockpitView, {
			props: {
				workspace: buildWorkspace(),
				onAddRepo,
			},
		});

		const addButton = container.querySelector('button.section-action');
		expect(addButton).toBeInTheDocument();
		await fireEvent.click(addButton!);

		expect(onAddRepo).toHaveBeenCalledWith('ws-1');
	});

	test('collapses sidebar when collapse button is clicked', async () => {
		const { container } = render(TerminalCockpitView, {
			props: {
				workspace: buildWorkspace(),
			},
		});

		expect(container.querySelector('.sidebar')).toBeInTheDocument();
		expect(container.querySelector('.sidebar-collapsed')).not.toBeInTheDocument();

		const collapseBtn = container.querySelector('.sidebar-header .collapse-btn');
		expect(collapseBtn).toBeInTheDocument();
		await fireEvent.click(collapseBtn!);

		expect(container.querySelector('.sidebar')).not.toBeInTheDocument();
		expect(container.querySelector('.sidebar-collapsed')).toBeInTheDocument();
		expect(storageMock.setItem).toHaveBeenCalledWith(COLLAPSED_KEY, 'true');
	});

	test('expands sidebar when expand button is clicked', async () => {
		storageMock.getItem.mockReturnValue('true');
		const { container } = render(TerminalCockpitView, {
			props: {
				workspace: buildWorkspace(),
			},
		});

		expect(container.querySelector('.sidebar-collapsed')).toBeInTheDocument();
		expect(container.querySelector('.sidebar')).not.toBeInTheDocument();

		const expandBtn = container.querySelector('.sidebar-collapsed .collapse-btn');
		expect(expandBtn).toBeInTheDocument();
		await fireEvent.click(expandBtn!);

		expect(container.querySelector('.sidebar-collapsed')).not.toBeInTheDocument();
		expect(container.querySelector('.sidebar')).toBeInTheDocument();
		expect(storageMock.setItem).toHaveBeenCalledWith(COLLAPSED_KEY, 'false');
	});

	test('restores collapsed state from localStorage', async () => {
		storageMock.getItem.mockReturnValue('true');
		const { container } = render(TerminalCockpitView, {
			props: {
				workspace: buildWorkspace(),
			},
		});

		expect(container.querySelector('.sidebar-collapsed')).toBeInTheDocument();
		expect(container.querySelector('.sidebar')).not.toBeInTheDocument();
	});
});
