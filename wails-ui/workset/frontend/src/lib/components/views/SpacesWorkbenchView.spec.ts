import { describe, expect, test, vi } from 'vitest';
import { fireEvent, render } from '@testing-library/svelte';
import SpacesWorkbenchView from './SpacesWorkbenchView.svelte';
import type { Workspace } from '../../types';
import type { WorksetThreadGroup } from '../../view-models/worksetViewModel';

vi.mock('../TerminalWorkspace.svelte', () => ({
	default: () => null,
}));

vi.mock('./UnifiedRepoView.svelte', () => ({
	default: () => null,
}));

vi.mock('../ui/ResizablePanel.svelte', () => ({
	default: () => null,
}));

describe('SpacesWorkbenchView', () => {
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
});
