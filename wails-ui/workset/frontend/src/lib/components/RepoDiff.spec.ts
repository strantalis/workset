import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, cleanup, waitFor } from '@testing-library/svelte';
import type { Repo } from '../types';

vi.mock('../api', () => ({
	commitAndPush: vi.fn(),
	createPullRequest: vi.fn(),
	deleteReviewComment: vi.fn(),
	editReviewComment: vi.fn(),
	fetchCurrentGitHubUser: vi.fn(),
	fetchTrackedPullRequest: vi.fn(),
	fetchPullRequestReviews: vi.fn(),
	fetchPullRequestStatus: vi.fn(),
	fetchRepoLocalStatus: vi.fn(),
	fetchRepoDiffSummary: vi.fn(),
	fetchRepoFileDiff: vi.fn(),
	fetchBranchDiffSummary: vi.fn(),
	fetchBranchFileDiff: vi.fn(),
	generatePullRequestText: vi.fn(),
	listRemotes: vi.fn(),
	replyToReviewComment: vi.fn(),
	resolveReviewThread: vi.fn(),
}));

vi.mock('../../../wailsjs/runtime/runtime', () => ({
	BrowserOpenURL: vi.fn(),
}));

const repo: Repo = {
	id: 'repo-1',
	name: 'workset',
	path: '/repo/path',
	dirty: false,
	missing: false,
	diff: { added: 0, removed: 0 },
	files: [],
};

const mockSummary = { files: [], totalAdded: 0, totalRemoved: 0 };

const createDeferred = <T,>() => {
	let resolve: (value: T) => void;
	let reject: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve: resolve!, reject: reject! };
};

let api: typeof import('../api');
let RepoDiff: typeof import('./RepoDiff.svelte').default;

beforeEach(async () => {
	api = await import('../api');
	RepoDiff = (await import('./RepoDiff.svelte')).default;
	vi.mocked(api.fetchRepoDiffSummary).mockResolvedValue(mockSummary);
	vi.mocked(api.fetchBranchDiffSummary).mockResolvedValue(mockSummary);
	vi.mocked(api.fetchTrackedPullRequest).mockResolvedValue(null);
	vi.mocked(api.listRemotes).mockResolvedValue([]);
	vi.mocked(api.fetchPullRequestReviews).mockResolvedValue([]);
	vi.mocked(api.fetchPullRequestStatus).mockResolvedValue(null);
	vi.mocked(api.fetchRepoLocalStatus).mockResolvedValue({
		hasUncommitted: false,
		ahead: 0,
		behind: 0,
		currentBranch: 'main',
	});
});

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
});

describe('RepoDiff create PR feedback', () => {
	it('shows progress stages and clears on error', async () => {
		const generateDeferred = createDeferred<{ title: string; body: string }>();
		const createDeferredResult = createDeferred<{
			repo: string;
			number: number;
			url: string;
			title: string;
			state: string;
			draft: boolean;
			baseRepo: string;
			baseBranch: string;
			headRepo: string;
			headBranch: string;
		}>();

		vi.mocked(api.generatePullRequestText).mockReturnValueOnce(generateDeferred.promise);
		vi.mocked(api.createPullRequest).mockReturnValueOnce(createDeferredResult.promise);

		const { getByRole, queryByText, container, findByText } = render(RepoDiff, {
			props: {
				repo,
				workspaceId: 'ws-1',
				onClose: vi.fn(),
			},
		});

		const createButton = getByRole('button', { name: 'Create PR' });
		await fireEvent.click(createButton);

		await waitFor(() => expect(createButton).toHaveTextContent('Generating title...'));
		expect(createButton).toBeDisabled();
		expect(queryByText('Step 1/2: Generating title...')).toBeInTheDocument();
		expect(container.querySelector('.pr-panel-content')).toHaveClass('expanded');

		generateDeferred.resolve({ title: 'Title', body: 'Body' });
		await waitFor(() => expect(createButton).toHaveTextContent('Creating PR...'));
		expect(queryByText('Step 2/2: Creating PR...')).toBeInTheDocument();

		createDeferredResult.reject(new Error('Failed to create pull request.'));
		await waitFor(() => expect(createButton).toHaveTextContent('Create PR'));
		expect(queryByText('Step 1/2: Generating title...')).not.toBeInTheDocument();
		expect(queryByText('Step 2/2: Creating PR...')).not.toBeInTheDocument();
		expect(await findByText('Failed to create pull request.')).toBeInTheDocument();
	});
});
