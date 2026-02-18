import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import PROrchestrationView from './PROrchestrationView.svelte';
import { repoDiffCache } from '../../cache/repoDiffCache';
import type { PullRequestCreated, RepoFileDiff, RepoDiffSummary, Workspace } from '../../types';
import * as githubApi from '../../api/github';
import * as repoDiffApi from '../../api/repo-diff';
import * as githubReviewApi from '../../api/github/review';
import * as githubUserApi from '../../api/github/user';

vi.mock('../../api/github', () => ({
	createPullRequest: vi.fn(),
	fetchPullRequestReviews: vi.fn(),
	fetchPullRequestStatus: vi.fn(),
	fetchRepoLocalStatus: vi.fn(),
	fetchTrackedPullRequest: vi.fn(),
	generatePullRequestText: vi.fn(),
	listRemotes: vi.fn(),
	replyToReviewComment: vi.fn(),
	resolveReviewThread: vi.fn(),
	startCommitAndPushAsync: vi.fn(),
}));

vi.mock('../../api/repo-diff', () => ({
	fetchBranchDiffSummary: vi.fn(),
	fetchBranchFileDiff: vi.fn(),
	fetchRepoDiffSummary: vi.fn(),
	fetchRepoFileDiff: vi.fn(),
	startRepoStatusWatch: vi.fn(),
	stopRepoStatusWatch: vi.fn(),
}));

vi.mock('../../api/github/review', () => ({
	deleteReviewComment: vi.fn(),
	editReviewComment: vi.fn(),
}));

vi.mock('../../api/github/user', () => ({
	fetchCurrentGitHubUser: vi.fn(),
}));

vi.mock('../../githubOperationService', () => ({
	subscribeGitHubOperationEvent: vi.fn(() => () => {}),
}));

vi.mock('@wailsio/runtime', () => ({
	Browser: {
		OpenURL: vi.fn(),
	},
}));

const trackedPr: PullRequestCreated = {
	repo: 'repo-one',
	number: 42,
	url: 'https://github.com/octo/repo-one/pull/42',
	title: 'Test PR title',
	state: 'open',
	draft: false,
	baseRepo: 'octo/repo-one',
	baseBranch: 'main',
	headRepo: 'octo/repo-one',
	headBranch: 'feature/sidebar-collapse',
	updatedAt: '2026-02-17T00:00:00Z',
};

const emptySummary: RepoDiffSummary = {
	files: [],
	totalAdded: 0,
	totalRemoved: 0,
};

const emptyDiff: RepoFileDiff = {
	patch: '',
	truncated: false,
	totalBytes: 0,
	totalLines: 0,
};

const SIDEBAR_COLLAPSED_KEY = 'workset:pr-orchestration:sidebarCollapsed';

const resetLocalStorage = (): void => {
	try {
		localStorage.removeItem(SIDEBAR_COLLAPSED_KEY);
		localStorage.removeItem('workset:pr-orchestration:sidebarRatio');
	} catch {
		// ignore storage failures in test environment
	}
};

const buildWorkspace = (): Workspace => ({
	id: 'ws-1',
	name: 'Workset One',
	path: '/tmp/ws-1',
	archived: false,
	repos: [
		{
			id: 'repo-1',
			name: 'repo-one',
			path: '/tmp/ws-1/repo-one',
			defaultBranch: 'main',
			currentBranch: 'feature/sidebar-collapse',
			ahead: 1,
			behind: 0,
			dirty: false,
			missing: false,
			trackedPullRequest: trackedPr,
			diff: { added: 0, removed: 0 },
			files: [],
		},
	],
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: '2026-02-17T00:00:00Z',
});

describe('PROrchestrationView sidebar collapse', () => {
	beforeEach(() => {
		resetLocalStorage();
		repoDiffCache.clear();
		vi.clearAllMocks();

		vi.mocked(githubApi.fetchTrackedPullRequest).mockResolvedValue(trackedPr);
		vi.mocked(githubApi.fetchRepoLocalStatus).mockResolvedValue({
			hasUncommitted: false,
			ahead: 0,
			behind: 0,
			currentBranch: 'feature/sidebar-collapse',
		});
		vi.mocked(githubApi.fetchPullRequestStatus).mockResolvedValue({
			pullRequest: trackedPr,
			checks: [],
		});
		vi.mocked(githubApi.fetchPullRequestReviews).mockResolvedValue([]);
		vi.mocked(githubApi.generatePullRequestText).mockResolvedValue({
			title: '',
			body: '',
		});
		vi.mocked(githubApi.listRemotes).mockResolvedValue([
			{ name: 'origin', owner: 'octo', repo: 'repo-one' },
		]);
		vi.mocked(githubApi.createPullRequest).mockResolvedValue(trackedPr);
		vi.mocked(githubApi.replyToReviewComment).mockResolvedValue({
			id: 1,
			body: 'reply',
			path: 'README.md',
			outdated: false,
		});
		vi.mocked(githubApi.resolveReviewThread).mockResolvedValue(true);
		vi.mocked(githubApi.startCommitAndPushAsync).mockResolvedValue({
			operationId: 'op-1',
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			type: 'commit_push',
			stage: 'queued',
			state: 'running',
			startedAt: '2026-02-17T00:00:00Z',
		});

		vi.mocked(repoDiffApi.fetchBranchDiffSummary).mockResolvedValue(emptySummary);
		vi.mocked(repoDiffApi.fetchRepoDiffSummary).mockResolvedValue(emptySummary);
		vi.mocked(repoDiffApi.fetchBranchFileDiff).mockResolvedValue(emptyDiff);
		vi.mocked(repoDiffApi.fetchRepoFileDiff).mockResolvedValue(emptyDiff);
		vi.mocked(repoDiffApi.startRepoStatusWatch).mockResolvedValue(true);
		vi.mocked(repoDiffApi.stopRepoStatusWatch).mockResolvedValue(true);

		vi.mocked(githubReviewApi.deleteReviewComment).mockResolvedValue(undefined);
		vi.mocked(githubReviewApi.editReviewComment).mockResolvedValue({
			id: 1,
			body: 'edited',
			path: 'README.md',
			outdated: false,
		});

		vi.mocked(githubUserApi.fetchCurrentGitHubUser).mockResolvedValue({
			id: 7,
			login: 'octocat',
		});
	});

	afterEach(() => {
		cleanup();
		resetLocalStorage();
		repoDiffCache.clear();
	});

	test('prevents collapsing when no PR item is selected', async () => {
		const workspace = buildWorkspace();
		const { getByRole, container, queryByRole } = render(PROrchestrationView, {
			props: { workspace },
		});

		const collapseButton = getByRole('button', { name: 'Collapse sidebar' });
		expect(collapseButton).toBeDisabled();

		await fireEvent.click(collapseButton);

		expect(queryByRole('button', { name: 'Expand sidebar' })).not.toBeInTheDocument();
		expect(container.querySelector('.sidebar-collapsed-layout')).not.toBeInTheDocument();
		expect(container.querySelector('.sidebar')).toBeInTheDocument();
	});

	test('collapses and expands the whole side panel after selecting an item', async () => {
		const workspace = buildWorkspace();
		const { container, getByRole, getByText, queryByText } = render(PROrchestrationView, {
			props: { workspace },
		});

		const listRow = getByText('repo-one').closest('button');
		expect(listRow).toBeTruthy();
		await fireEvent.click(listRow!);

		await waitFor(() => {
			expect(getByRole('button', { name: 'Collapse sidebar' })).toBeEnabled();
		});

		await fireEvent.click(getByRole('button', { name: 'Collapse sidebar' }));

		expect(getByRole('button', { name: 'Expand sidebar' })).toBeInTheDocument();
		expect(queryByText('Active PRs')).not.toBeInTheDocument();
		expect(container.querySelector('.sidebar-collapsed-layout')).toBeInTheDocument();
		expect(container.querySelector('.sidebar')).not.toBeInTheDocument();

		await fireEvent.click(getByRole('button', { name: 'Expand sidebar' }));

		expect(getByRole('button', { name: 'Collapse sidebar' })).toBeEnabled();
		expect(getByText('Active PRs')).toBeInTheDocument();
		expect(container.querySelector('.sidebar')).toBeInTheDocument();
	});

	test('uses remote-qualified refs when loading tracked PR summary', async () => {
		const workspace = buildWorkspace();
		const { getByText } = render(PROrchestrationView, {
			props: { workspace },
		});

		const listRow = getByText('repo-one').closest('button');
		expect(listRow).toBeTruthy();
		await fireEvent.click(listRow!);

		await waitFor(() => {
			expect(repoDiffApi.fetchBranchDiffSummary).toHaveBeenCalledWith(
				'ws-1',
				'repo-1',
				'origin/main',
				'origin/feature/sidebar-collapse',
			);
		});
	});
});
