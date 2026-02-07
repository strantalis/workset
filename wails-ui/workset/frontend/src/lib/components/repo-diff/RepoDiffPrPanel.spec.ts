import { fireEvent, render } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import RepoDiffPrPanel from './RepoDiffPrPanel.svelte';

const baseProps = {
	effectiveMode: 'create' as const,
	prPanelExpanded: false,
	remotes: [{ name: 'origin', owner: 'acme', repo: 'workset' }],
	remotesLoading: false,
	prBaseRemote: '',
	prBase: 'main',
	prDraft: false,
	prCreating: false,
	prCreateStageCopy: null,
	prCreateError: null,
	prTracked: null,
	prCreateSuccess: null,
	prStatusError: null,
	prReviewsSent: false,
	hasUncommittedChanges: false,
	commitPushLoading: false,
	commitPushStageCopy: 'Committing...',
	commitPushError: null,
	commitPushSuccess: false,
	onCreatePr: vi.fn(),
	onViewStatus: vi.fn(),
	onCommitAndPush: vi.fn(),
};

describe('RepoDiffPrPanel', () => {
	it('expands and triggers create callback in create mode', async () => {
		const onCreatePr = vi.fn();
		const { container, getByRole } = render(RepoDiffPrPanel, {
			props: { ...baseProps, onCreatePr },
		});

		const panel = container.querySelector('.pr-panel-content');
		expect(panel).not.toBeNull();
		expect(panel).not.toHaveClass('expanded');

		await fireEvent.click(getByRole('button', { name: /Create Pull Request/ }));
		expect(panel).toHaveClass('expanded');

		await fireEvent.click(getByRole('button', { name: 'Create PR' }));
		expect(onCreatePr).toHaveBeenCalledTimes(1);
	});

	it('routes view-status and commit-push actions in status mode', async () => {
		const onViewStatus = vi.fn();
		const onCommitAndPush = vi.fn();
		const { getByRole, getByText } = render(RepoDiffPrPanel, {
			props: {
				...baseProps,
				effectiveMode: 'create',
				prTracked: {
					repo: 'acme/workset',
					number: 42,
					url: 'https://github.com/acme/workset/pull/42',
					title: 'Add repo diff panel split',
					state: 'open',
					draft: false,
					baseRepo: 'acme/workset',
					baseBranch: 'main',
					headRepo: 'acme/workset',
					headBranch: 'feature/pr-panel',
				},
				onViewStatus,
			},
		});

		await fireEvent.click(getByRole('button', { name: 'View status →' }));
		expect(onViewStatus).toHaveBeenCalledTimes(1);

		const statusRender = render(RepoDiffPrPanel, {
			props: {
				...baseProps,
				effectiveMode: 'status',
				prStatusError: 'Failed to fetch status',
				prReviewsSent: true,
				hasUncommittedChanges: true,
				commitPushLoading: false,
				onCommitAndPush,
			},
		});

		expect(statusRender.getByText('Failed to fetch status')).toBeInTheDocument();
		expect(statusRender.getByText('Sent to terminal')).toBeInTheDocument();
		expect(statusRender.getByText('You have uncommitted local changes')).toBeInTheDocument();
		const commitButton = statusRender.getByRole('button', { name: 'Commit & Push' });
		expect(commitButton).toBeEnabled();

		await fireEvent.click(commitButton);
		expect(onCommitAndPush).toHaveBeenCalledTimes(1);
		expect(getByText('Existing PR #42 found.')).toBeInTheDocument();
	});
});
