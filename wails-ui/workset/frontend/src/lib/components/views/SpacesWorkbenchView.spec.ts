import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@testing-library/svelte';
import SpacesWorkbenchView from './SpacesWorkbenchView.svelte';
import type { Workspace } from '../../types';
import type { WorksetThreadGroup } from '../../view-models/worksetViewModel';
import {
	resetTerminalWorkspaceLifecycle,
	terminalWorkspaceLifecycle,
} from '../test-utils/mockTerminalWorkspaceTracker';

vi.mock('../TerminalWorkspace.svelte', async () => {
	const module = await import('../test-utils/MockTerminalWorkspace.svelte');
	return { default: module.default };
});

vi.mock('./UnifiedRepoView.svelte', () => ({
	default: () => null,
}));

vi.mock('../ui/ResizablePanel.svelte', async () => {
	const module = await import('../test-utils/MockResizablePanel.svelte');
	return { default: module.default };
});

describe('SpacesWorkbenchView', () => {
	beforeEach(() => {
		resetTerminalWorkspaceLifecycle();
	});

	afterEach(() => {
		cleanup();
		resetTerminalWorkspaceLifecycle();
	});

	test('stays stable when the active workspace disappears during a rerender sequence', async () => {
		const thread: Workspace = {
			id: 'thread-alpha',
			name: 'Alpha Thread',
			path: '/tmp/thread-alpha',
			archived: false,
			repos: [],
			pinned: false,
			pinOrder: 0,
			expanded: false,
			lastUsed: '2026-03-28T00:00:00Z',
		};
		const baseGroup: WorksetThreadGroup = {
			id: 'workset:alpha',
			label: 'Alpha',
			repos: [],
			threads: [thread],
		};

		const view = render(SpacesWorkbenchView, {
			props: {
				activeWorkspaceId: thread.id,
				worksetGroups: [baseGroup],
				selectedWorksetId: baseGroup.id,
				threadSummaryMap: new Map(),
				onSelectWorkspace: vi.fn(),
				onSelectWorkset: vi.fn(),
				onCreateWorkspace: vi.fn(),
				onCreateThread: vi.fn(),
				onAddRepo: vi.fn(),
			},
		});

		expect(view.getByRole('heading', { name: 'Alpha Thread' })).toBeInTheDocument();

		await expect(
			view.rerender({
				activeWorkspaceId: thread.id,
				worksetGroups: [{ ...baseGroup, threads: [] }],
				selectedWorksetId: baseGroup.id,
				threadSummaryMap: new Map(),
				onSelectWorkspace: vi.fn(),
				onSelectWorkset: vi.fn(),
				onCreateWorkspace: vi.fn(),
				onCreateThread: vi.fn(),
				onAddRepo: vi.fn(),
			}),
		).resolves.toBeUndefined();

		await expect(
			view.rerender({
				activeWorkspaceId: null,
				worksetGroups: [{ ...baseGroup, threads: [] }],
				selectedWorksetId: baseGroup.id,
				threadSummaryMap: new Map(),
				onSelectWorkspace: vi.fn(),
				onSelectWorkset: vi.fn(),
				onCreateWorkspace: vi.fn(),
				onCreateThread: vi.fn(),
				onAddRepo: vi.fn(),
			}),
		).resolves.toBeUndefined();
	});

	test('renders an empty-workset state with create and add actions', async () => {
		const onCreateThread = vi.fn<(worksetId: string) => void>();
		const onAddRepo = vi.fn<(worksetId: string) => void>();
		const worksetGroups: WorksetThreadGroup[] = [
			{
				id: 'workset:platform-core',
				label: 'Platform Core',
				repos: ['repo-one'],
				threads: [],
			},
		];

		const { getAllByText, getByRole, getByText } = render(SpacesWorkbenchView, {
			props: {
				activeWorkspaceId: null,
				worksetGroups,
				selectedWorksetId: 'workset:platform-core',
				threadSummaryMap: new Map(),
				onSelectWorkspace: vi.fn(),
				onSelectWorkset: vi.fn(),
				onCreateWorkspace: vi.fn(),
				onCreateThread,
				onAddRepo,
			},
		});

		expect(getAllByText('Platform Core')).toHaveLength(2);
		expect(
			getByText('No threads yet. Create the first thread or add repos to shape this workset.'),
		).toBeInTheDocument();

		await fireEvent.click(getByRole('button', { name: 'New Thread' }));
		expect(onCreateThread).toHaveBeenCalledWith('workset:platform-core');

		await fireEvent.click(getByRole('button', { name: 'Add Repo' }));
		expect(onAddRepo).toHaveBeenCalledWith('workset:platform-core');
	});

	test('remounts the terminal workspace when the active thread changes', async () => {
		const alpha: Workspace = {
			id: 'thread-alpha',
			name: 'Alpha Thread',
			path: '/tmp/thread-alpha',
			archived: false,
			repos: [],
			pinned: false,
			pinOrder: 0,
			expanded: false,
			lastUsed: '2026-03-28T00:00:00Z',
		};
		const beta: Workspace = {
			...alpha,
			id: 'thread-beta',
			name: 'Beta Thread',
			path: '/tmp/thread-beta',
		};
		const baseGroup: WorksetThreadGroup = {
			id: 'workset:alpha',
			label: 'Alpha',
			repos: [],
			threads: [alpha, beta],
		};

		const view = render(SpacesWorkbenchView, {
			props: {
				activeWorkspaceId: alpha.id,
				worksetGroups: [baseGroup],
				selectedWorksetId: baseGroup.id,
				threadSummaryMap: new Map(),
				onSelectWorkspace: vi.fn(),
				onSelectWorkset: vi.fn(),
				onCreateWorkspace: vi.fn(),
				onCreateThread: vi.fn(),
				onAddRepo: vi.fn(),
			},
		});

		expect(view.getByTestId('mock-terminal-workspace')).toHaveTextContent(
			'thread-alpha:Alpha Thread',
		);
		expect(terminalWorkspaceLifecycle.mounts).toEqual([
			{ workspaceId: 'thread-alpha', workspaceName: 'Alpha Thread' },
		]);

		await view.rerender({
			activeWorkspaceId: beta.id,
			worksetGroups: [baseGroup],
			selectedWorksetId: baseGroup.id,
			threadSummaryMap: new Map(),
			onSelectWorkspace: vi.fn(),
			onSelectWorkset: vi.fn(),
			onCreateWorkspace: vi.fn(),
			onCreateThread: vi.fn(),
			onAddRepo: vi.fn(),
		});

		expect(view.getByTestId('mock-terminal-workspace')).toHaveTextContent(
			'thread-beta:Beta Thread',
		);
		expect(terminalWorkspaceLifecycle.mounts).toEqual([
			{ workspaceId: 'thread-alpha', workspaceName: 'Alpha Thread' },
			{ workspaceId: 'thread-beta', workspaceName: 'Beta Thread' },
		]);
		expect(terminalWorkspaceLifecycle.destroys).toEqual([
			{ workspaceId: 'thread-alpha', workspaceName: 'Alpha Thread' },
		]);
	});

	test('keeps the terminal workspace mounted when toggling code view', async () => {
		const thread: Workspace = {
			id: 'thread-alpha',
			name: 'Alpha Thread',
			path: '/tmp/thread-alpha',
			archived: false,
			repos: [],
			pinned: false,
			pinOrder: 0,
			expanded: false,
			lastUsed: '2026-03-28T00:00:00Z',
		};
		const baseGroup: WorksetThreadGroup = {
			id: 'workset:alpha',
			label: 'Alpha',
			repos: [],
			threads: [thread],
		};

		const view = render(SpacesWorkbenchView, {
			props: {
				activeWorkspaceId: thread.id,
				worksetGroups: [baseGroup],
				selectedWorksetId: baseGroup.id,
				threadSummaryMap: new Map(),
				preferredSurface: 'terminal',
				onSelectWorkspace: vi.fn(),
				onSelectWorkset: vi.fn(),
				onCreateWorkspace: vi.fn(),
				onCreateThread: vi.fn(),
				onAddRepo: vi.fn(),
			},
		});

		expect(view.getByTestId('mock-terminal-workspace')).toHaveTextContent(
			'thread-alpha:Alpha Thread',
		);
		expect(view.queryByTestId('mock-resizable-panel-second')).not.toBeInTheDocument();
		expect(terminalWorkspaceLifecycle.mounts).toEqual([
			{ workspaceId: 'thread-alpha', workspaceName: 'Alpha Thread' },
		]);

		await view.rerender({
			activeWorkspaceId: thread.id,
			worksetGroups: [baseGroup],
			selectedWorksetId: baseGroup.id,
			threadSummaryMap: new Map(),
			preferredSurface: 'pull-requests',
			onSelectWorkspace: vi.fn(),
			onSelectWorkset: vi.fn(),
			onCreateWorkspace: vi.fn(),
			onCreateThread: vi.fn(),
			onAddRepo: vi.fn(),
		});

		expect(view.getByTestId('mock-terminal-workspace')).toHaveTextContent(
			'thread-alpha:Alpha Thread',
		);
		expect(view.getByTestId('mock-resizable-panel-second')).toBeInTheDocument();
		expect(terminalWorkspaceLifecycle.mounts).toEqual([
			{ workspaceId: 'thread-alpha', workspaceName: 'Alpha Thread' },
		]);
		expect(terminalWorkspaceLifecycle.destroys).toEqual([]);
	});
});
