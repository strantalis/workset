import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/svelte';
import ExplorerPanel from './ExplorerPanel.svelte';
import type { Workspace } from '../../types';

const buildWorkspace = (overrides: Partial<Workspace> = {}): Workspace => ({
	id: 'ws-1',
	name: 'Thread One',
	path: '/tmp/ws-1',
	workset: 'Alpha',
	worksetKey: 'workset:alpha',
	worksetLabel: 'Alpha',
	placeholder: false,
	archived: false,
	repos: [],
	pinned: false,
	pinOrder: 0,
	expanded: true,
	lastUsed: new Date().toISOString(),
	...overrides,
});

describe('ExplorerPanel', () => {
	beforeEach(() => {
		Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
			configurable: true,
			value: vi.fn(),
		});
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	test('lets users select an empty workset and create a thread', async () => {
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();
		const onCreateThread = vi.fn<(worksetId: string) => void>();

		const { getByRole } = render(ExplorerPanel, {
			props: {
				workspaces: [
					buildWorkspace({
						id: 'thread-alpha',
						name: 'Alpha Thread',
						workset: 'Alpha',
						worksetKey: 'workset:alpha',
						worksetLabel: 'Alpha',
					}),
					buildWorkspace({
						id: 'placeholder-beta',
						name: 'Beta Placeholder',
						workset: 'Beta',
						worksetKey: 'workset:beta',
						worksetLabel: 'Beta',
						placeholder: true,
					}),
				],
				activeWorkspaceId: 'thread-alpha',
				shortcutMap: new Map<string, number>(),
				onSelectWorkspace,
				onCreateThread,
			},
		});

		await fireEvent.click(getByRole('button', { name: 'Switch workset' }));
		await fireEvent.click(getByRole('button', { name: /Beta/i }));

		expect(onSelectWorkspace).not.toHaveBeenCalled();

		const createThreadButton = getByRole('button', { name: 'Create thread in Beta' });
		await fireEvent.click(createThreadButton);
		expect(onCreateThread).toHaveBeenCalledWith('workset:beta');
	});

	test('keeps selection on the same workset when active thread is removed', async () => {
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();

		const { rerender } = render(ExplorerPanel, {
			props: {
				workspaces: [
					buildWorkspace({
						id: 'thread-alpha',
						name: 'Alpha Thread',
						workset: 'Alpha',
						worksetKey: 'workset:alpha',
						worksetLabel: 'Alpha',
					}),
					buildWorkspace({
						id: 'thread-beta-1',
						name: 'Beta Thread One',
						workset: 'Beta',
						worksetKey: 'workset:beta',
						worksetLabel: 'Beta',
					}),
					buildWorkspace({
						id: 'thread-beta-2',
						name: 'Beta Thread Two',
						workset: 'Beta',
						worksetKey: 'workset:beta',
						worksetLabel: 'Beta',
					}),
				],
				activeWorkspaceId: 'thread-beta-2',
				shortcutMap: new Map<string, number>(),
				onSelectWorkspace,
			},
		});

		await rerender({
			workspaces: [
				buildWorkspace({
					id: 'thread-alpha',
					name: 'Alpha Thread',
					workset: 'Alpha',
					worksetKey: 'workset:alpha',
					worksetLabel: 'Alpha',
				}),
				buildWorkspace({
					id: 'thread-beta-1',
					name: 'Beta Thread One',
					workset: 'Beta',
					worksetKey: 'workset:beta',
					worksetLabel: 'Beta',
				}),
			],
			activeWorkspaceId: null,
			shortcutMap: new Map<string, number>(),
			onSelectWorkspace,
		});

		await waitFor(() => expect(onSelectWorkspace).toHaveBeenCalledWith('thread-beta-1'));
	});

	test('does not auto-switch to another workset when selected workset is empty', async () => {
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();

		const { rerender } = render(ExplorerPanel, {
			props: {
				workspaces: [
					buildWorkspace({
						id: 'thread-alpha',
						name: 'Alpha Thread',
						workset: 'Alpha',
						worksetKey: 'workset:alpha',
						worksetLabel: 'Alpha',
					}),
					buildWorkspace({
						id: 'thread-beta',
						name: 'Beta Thread',
						workset: 'Beta',
						worksetKey: 'workset:beta',
						worksetLabel: 'Beta',
					}),
				],
				activeWorkspaceId: 'thread-beta',
				shortcutMap: new Map<string, number>(),
				onSelectWorkspace,
			},
		});

		await rerender({
			workspaces: [
				buildWorkspace({
					id: 'thread-alpha',
					name: 'Alpha Thread',
					workset: 'Alpha',
					worksetKey: 'workset:alpha',
					worksetLabel: 'Alpha',
				}),
				buildWorkspace({
					id: 'placeholder-beta',
					name: 'Beta Placeholder',
					workset: 'Beta',
					worksetKey: 'workset:beta',
					worksetLabel: 'Beta',
					placeholder: true,
				}),
			],
			activeWorkspaceId: null,
			shortcutMap: new Map<string, number>(),
			onSelectWorkspace,
		});

		await new Promise((resolve) => setTimeout(resolve, 0));
		expect(onSelectWorkspace).not.toHaveBeenCalled();
	});

	test('does not switch worksets while selected workset is temporarily missing', async () => {
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();

		const { rerender } = render(ExplorerPanel, {
			props: {
				workspaces: [
					buildWorkspace({
						id: 'thread-alpha',
						name: 'Alpha Thread',
						workset: 'Alpha',
						worksetKey: 'workset:alpha',
						worksetLabel: 'Alpha',
					}),
					buildWorkspace({
						id: 'thread-beta',
						name: 'Beta Thread',
						workset: 'Beta',
						worksetKey: 'workset:beta',
						worksetLabel: 'Beta',
					}),
				],
				activeWorkspaceId: 'thread-beta',
				shortcutMap: new Map<string, number>(),
				onSelectWorkspace,
			},
		});

		await rerender({
			workspaces: [
				buildWorkspace({
					id: 'thread-alpha',
					name: 'Alpha Thread',
					workset: 'Alpha',
					worksetKey: 'workset:alpha',
					worksetLabel: 'Alpha',
				}),
			],
			activeWorkspaceId: null,
			shortcutMap: new Map<string, number>(),
			onSelectWorkspace,
		});

		await new Promise((resolve) => setTimeout(resolve, 0));
		expect(onSelectWorkspace).not.toHaveBeenCalled();
	});

	test('allows add repo for worksets without threads', async () => {
		const onSelectWorkspace = vi.fn<(workspaceId: string) => void>();
		const onAddRepo = vi.fn<(worksetId: string) => void>();

		const { getByRole } = render(ExplorerPanel, {
			props: {
				workspaces: [
					buildWorkspace({
						id: 'thread-alpha',
						name: 'Alpha Thread',
						workset: 'Alpha',
						worksetKey: 'workset:alpha',
						worksetLabel: 'Alpha',
					}),
					buildWorkspace({
						id: 'placeholder-beta',
						name: 'Beta Placeholder',
						workset: 'Beta',
						worksetKey: 'workset:beta',
						worksetLabel: 'Beta',
						placeholder: true,
					}),
				],
				activeWorkspaceId: 'thread-alpha',
				shortcutMap: new Map<string, number>(),
				onSelectWorkspace,
				onAddRepo,
			},
		});

		await fireEvent.click(getByRole('button', { name: 'Switch workset' }));
		await fireEvent.click(getByRole('button', { name: /Beta/i }));

		const addRepoButton = getByRole('button', { name: 'Add repo to Beta' });
		await fireEvent.click(addRepoButton);
		expect(onAddRepo).toHaveBeenCalledWith('workset:beta');
	});

	test('renders Live badge for threads with active terminal IO', () => {
		const alpha = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const beta = buildWorkspace({
			id: 'thread-beta',
			name: 'beta',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const { getByText } = render(ExplorerPanel, {
			props: {
				workspaces: [alpha, beta],
				activeWorkspaceId: alpha.id,
				shortcutMap: new Map<string, number>(),
				activeTerminalWorkspaceIds: [beta.id],
				onSelectWorkspace: vi.fn(),
			},
		});

		const liveBadge = getByText('Live');
		expect(liveBadge).toBeInTheDocument();
		expect(getByText('beta').parentElement).toContainElement(liveBadge);
	});

	test('does not render Live badge when no thread has active terminal IO', () => {
		const alpha = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const beta = buildWorkspace({
			id: 'thread-beta',
			name: 'beta',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});
		const { queryByText } = render(ExplorerPanel, {
			props: {
				workspaces: [alpha, beta],
				activeWorkspaceId: alpha.id,
				shortcutMap: new Map<string, number>(),
				activeTerminalWorkspaceIds: [],
				onSelectWorkspace: vi.fn(),
			},
		});

		expect(queryByText('Live')).toBeNull();
	});

	test('opens files from the footer nav and marks the icon active', async () => {
		const onOpenFiles = vi.fn();
		const alpha = buildWorkspace({
			id: 'thread-alpha',
			name: 'alpha',
			workset: 'Core',
			worksetKey: 'workset:core',
			worksetLabel: 'Core',
		});

		const { getByRole } = render(ExplorerPanel, {
			props: {
				workspaces: [alpha],
				activeWorkspaceId: alpha.id,
				shortcutMap: new Map<string, number>(),
				filesActive: true,
				onSelectWorkspace: vi.fn(),
				onOpenFiles,
			},
		});

		const filesButton = getByRole('button', { name: 'Open files' });
		expect(filesButton).toHaveClass('active');

		await fireEvent.click(filesButton);
		expect(onOpenFiles).toHaveBeenCalledTimes(1);
	});
});
