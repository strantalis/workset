import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import PrLifecycleDrawer from './PrLifecycleDrawer.svelte';
import type { GitHubOperationStatus } from '../../api/github';
import type { PullRequestCreated } from '../../types';

const githubMocks = vi.hoisted(() => ({
	fetchGitHubOperationStatus: vi.fn(),
	fetchPullRequestStatus: vi.fn(),
	fetchRepoLocalStatus: vi.fn(),
	generatePullRequestText: vi.fn(),
	startCommitAndPushAsync: vi.fn(),
	startCreatePullRequestAsync: vi.fn(),
}));

const githubEventState = vi.hoisted(() => ({
	handler: null as ((status: GitHubOperationStatus) => void) | null,
	unsubscribe: vi.fn(),
}));

const browserOpenURL = vi.hoisted(() => vi.fn());

vi.mock('../../api/github', () => githubMocks);
vi.mock('../../githubOperationService', () => ({
	subscribeGitHubOperationEvent: vi.fn((handler: (status: GitHubOperationStatus) => void) => {
		githubEventState.handler = handler;
		return githubEventState.unsubscribe;
	}),
}));
vi.mock('@wailsio/runtime', () => ({
	Browser: {
		OpenURL: browserOpenURL,
	},
}));

const createdPullRequest: PullRequestCreated = {
	repo: 'octo/repo-alpha',
	number: 42,
	url: 'https://github.com/octo/repo-alpha/pull/42',
	title: 'PR ready',
	body: 'Shipped.',
	state: 'open',
	draft: false,
	baseRepo: 'octo/repo-alpha',
	baseBranch: 'main',
	headRepo: 'octo/repo-alpha',
	headBranch: 'feature/pr-cleanup',
};

const buildCreateRunningStatus = (
	stage: GitHubOperationStatus['stage'] = 'creating',
): GitHubOperationStatus => ({
	operationId: 'create-op',
	workspaceId: 'ws-1',
	repoId: 'repo-1',
	type: 'create_pr',
	stage,
	state: 'running',
	startedAt: new Date().toISOString(),
});

const buildCreateCompletedStatus = (): GitHubOperationStatus => ({
	operationId: 'create-op',
	workspaceId: 'ws-1',
	repoId: 'repo-1',
	type: 'create_pr',
	stage: 'completed',
	state: 'completed',
	startedAt: new Date().toISOString(),
	finishedAt: new Date().toISOString(),
	pullRequest: createdPullRequest,
});

const baseProps = () => ({
	open: true,
	workspaceId: 'ws-1',
	repoId: 'repo-1',
	repoName: 'repo-alpha',
	branch: 'feature/pr-cleanup',
	baseBranch: 'main',
	trackedPr: null,
	diffStats: null,
	unresolvedThreads: 0,
	onClose: vi.fn(),
	onStatusChanged: vi.fn(),
	onTrackedPrChanged: vi.fn(),
});

describe('PrLifecycleDrawer', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		githubEventState.handler = null;
		githubMocks.fetchGitHubOperationStatus.mockResolvedValue(null);
		githubMocks.fetchPullRequestStatus.mockResolvedValue({
			pullRequest: createdPullRequest,
			checks: [],
		});
		githubMocks.fetchRepoLocalStatus.mockResolvedValue({
			hasUncommitted: false,
			ahead: 0,
			behind: 0,
			currentBranch: 'feature/pr-cleanup',
		});
		githubMocks.generatePullRequestText.mockResolvedValue({
			title: 'Suggested title',
			body: 'Suggested body',
		});
		githubMocks.startCommitAndPushAsync.mockResolvedValue(buildCreateRunningStatus('queued'));
		githubMocks.startCreatePullRequestAsync.mockResolvedValue(buildCreateRunningStatus('queued'));
	});

	afterEach(() => {
		cleanup();
	});

	it('generates suggestion only once for the same open context', async () => {
		const props = baseProps();
		const view = render(PrLifecycleDrawer, { props });

		await waitFor(() => expect(githubMocks.generatePullRequestText).toHaveBeenCalledTimes(1));
		expect(view.getByDisplayValue('Suggested title')).toBeInTheDocument();

		await view.rerender({ ...props, unresolvedThreads: 1 });

		await waitFor(() => expect(githubMocks.generatePullRequestText).toHaveBeenCalledTimes(1));
	});

	it('starts async create with edited form values and switches to lifecycle view on completion', async () => {
		const props = baseProps();
		const view = render(PrLifecycleDrawer, { props });

		await waitFor(() => view.getByDisplayValue('Suggested title'));

		const titleInput = view.getByDisplayValue('Suggested title');
		const bodyInput = view.getByDisplayValue('Suggested body');
		await fireEvent.input(titleInput, { target: { value: 'Edited title' } });
		await fireEvent.input(bodyInput, { target: { value: 'Edited body' } });

		await fireEvent.click(view.getByRole('button', { name: 'Create Pull Request' }));

		expect(githubMocks.startCreatePullRequestAsync).toHaveBeenCalledWith('ws-1', 'repo-1', {
			title: 'Edited title',
			body: 'Edited body',
			base: 'main',
			head: 'feature/pr-cleanup',
			draft: false,
		});

		githubEventState.handler?.(buildCreateCompletedStatus());

		await waitFor(() => expect(props.onTrackedPrChanged).toHaveBeenCalledWith(createdPullRequest));
		await waitFor(() => expect(view.getByText('PR ready')).toBeInTheDocument());
		expect(view.getByRole('button', { name: 'GitHub' })).toBeInTheDocument();
	});

	it('restores in-flight create progress after closing and reopening the drawer', async () => {
		const props = baseProps();
		const view = render(PrLifecycleDrawer, { props });

		await waitFor(() => expect(githubMocks.generatePullRequestText).toHaveBeenCalledTimes(1));

		await fireEvent.click(view.getByRole('button', { name: 'Create Pull Request' }));
		expect(githubMocks.startCreatePullRequestAsync).toHaveBeenCalledTimes(1);

		await view.rerender({ ...props, open: false });
		githubMocks.fetchGitHubOperationStatus.mockResolvedValueOnce(
			buildCreateRunningStatus('creating'),
		);
		await view.rerender(props);

		await waitFor(() => expect(githubMocks.fetchGitHubOperationStatus).toHaveBeenCalledTimes(2));
		await waitFor(() =>
			expect(view.getAllByText('Creating pull request...').length).toBeGreaterThan(0),
		);
		expect(githubMocks.generatePullRequestText).toHaveBeenCalledTimes(1);
	});

	it('restores the created PR after reopening once the async operation completed', async () => {
		const props = baseProps();
		const view = render(PrLifecycleDrawer, { props });

		await waitFor(() => expect(githubMocks.generatePullRequestText).toHaveBeenCalledTimes(1));

		await fireEvent.click(view.getByRole('button', { name: 'Create Pull Request' }));
		await view.rerender({ ...props, open: false });

		githubMocks.fetchGitHubOperationStatus.mockResolvedValueOnce(buildCreateCompletedStatus());
		await view.rerender(props);

		await waitFor(() => expect(props.onTrackedPrChanged).toHaveBeenCalledWith(createdPullRequest));
		await waitFor(() => expect(view.getByText('PR ready')).toBeInTheDocument());
		expect(githubMocks.generatePullRequestText).toHaveBeenCalledTimes(1);
	});
});
