import { describe, expect, test, vi } from 'vitest';
import { fireEvent, render } from '@testing-library/svelte';
import SpacesWorkbenchView from './SpacesWorkbenchView.svelte';
import type { WorksetThreadGroup } from '../../view-models/worksetViewModel';

vi.mock('../TerminalWorkspace.svelte', () => ({
	default: {},
}));

vi.mock('./UnifiedRepoView.svelte', () => ({
	default: {},
}));

vi.mock('../ui/ResizablePanel.svelte', () => ({
	default: {},
}));

describe('SpacesWorkbenchView', () => {
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
