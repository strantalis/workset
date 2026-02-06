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
	startRepoDiffWatch: vi.fn(),
	updateRepoDiffWatch: vi.fn(),
	stopRepoDiffWatch: vi.fn(),
}));

vi.mock('../../../wailsjs/runtime/runtime', () => ({
	BrowserOpenURL: vi.fn(),
}));

vi.mock('../repoDiffService', () => ({
	subscribeRepoDiffEvent: vi.fn(() => () => {}),
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

const createDeferred = <T>() => {
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
	vi.mocked(api.fetchPullRequestStatus).mockResolvedValue({
		pullRequest: {
			repo: 'origin',
			number: 0,
			url: '',
			title: 'Draft',
			state: 'open',
			draft: false,
			baseRepo: 'origin',
			baseBranch: 'main',
			headRepo: 'origin',
			headBranch: 'feature',
		},
		checks: [],
	});
	vi.mocked(api.fetchRepoLocalStatus).mockResolvedValue({
		hasUncommitted: false,
		ahead: 0,
		behind: 0,
		currentBranch: 'main',
	});
	vi.mocked(api.startRepoDiffWatch).mockResolvedValue(true);
	vi.mocked(api.updateRepoDiffWatch).mockResolvedValue(true);
	vi.mocked(api.stopRepoDiffWatch).mockResolvedValue(true);
}, 30000);

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

describe('RepoDiff watcher lifecycle', () => {
	it('restarts watcher when repo changes', async () => {
		const onClose = vi.fn();
		const repoA: Repo = {
			id: 'repo-1',
			name: 'alpha',
			path: '/repo/a',
			dirty: false,
			missing: false,
			diff: { added: 0, removed: 0 },
			files: [],
		};
		const repoB: Repo = {
			id: 'repo-2',
			name: 'beta',
			path: '/repo/b',
			dirty: false,
			missing: false,
			diff: { added: 0, removed: 0 },
			files: [],
		};

		const { rerender } = render(RepoDiff, {
			props: {
				repo: repoA,
				workspaceId: 'ws-1',
				onClose,
			},
		});

		await waitFor(() => {
			expect(api.startRepoDiffWatch).toHaveBeenCalledWith('ws-1', 'repo-1', undefined, undefined);
		});

		await rerender({
			repo: repoB,
			workspaceId: 'ws-1',
			onClose,
		});

		await waitFor(() => {
			expect(api.stopRepoDiffWatch).toHaveBeenCalledWith('ws-1', 'repo-1');
			expect(api.startRepoDiffWatch).toHaveBeenCalledWith('ws-1', 'repo-2', undefined, undefined);
		});
	});
});

describe('RepoDiff local pending section', () => {
	it('shows local pending files separately when PR exists', async () => {
		vi.mocked(api.fetchTrackedPullRequest).mockResolvedValue({
			repo: 'acme/workset',
			number: 42,
			url: 'https://github.com/acme/workset/pull/42',
			title: 'Improve repo diff',
			state: 'open',
			draft: false,
			baseRepo: 'acme/workset',
			baseBranch: 'main',
			headRepo: 'acme/workset',
			headBranch: 'feature/local-diff',
		});
		vi.mocked(api.fetchPullRequestStatus).mockResolvedValue({
			pullRequest: {
				repo: 'acme/workset',
				number: 42,
				url: 'https://github.com/acme/workset/pull/42',
				title: 'Improve repo diff',
				state: 'open',
				draft: false,
				baseRepo: 'acme/workset',
				baseBranch: 'main',
				headRepo: 'acme/workset',
				headBranch: 'feature/local-diff',
			},
			checks: [],
		});
		vi.mocked(api.fetchRepoLocalStatus).mockResolvedValue({
			hasUncommitted: true,
			ahead: 0,
			behind: 0,
			currentBranch: 'feature/local-diff',
		});
		vi.mocked(api.fetchBranchDiffSummary).mockResolvedValue({
			files: [
				{
					path: 'pkg/worksetapi/workspaces.go',
					added: 7,
					removed: 0,
					status: 'modified',
				},
			],
			totalAdded: 7,
			totalRemoved: 0,
		});
		vi.mocked(api.fetchRepoDiffSummary).mockResolvedValue({
			files: [
				{
					path: 'pkg/worksetapi/service_workspaces_test.go',
					added: 33,
					removed: 0,
					status: 'modified',
				},
			],
			totalAdded: 33,
			totalRemoved: 0,
		});
		vi.mocked(api.fetchBranchFileDiff).mockResolvedValue({
			patch: `diff --git a/pkg/worksetapi/workspaces.go b/pkg/worksetapi/workspaces.go
index 1111111..2222222 100644
--- a/pkg/worksetapi/workspaces.go
+++ b/pkg/worksetapi/workspaces.go
@@ -1 +1 @@
-old
+new
`,
			truncated: false,
			totalBytes: 80,
			totalLines: 1,
		});
		vi.mocked(api.fetchRepoFileDiff).mockResolvedValue({
			patch: `diff --git a/pkg/worksetapi/service_workspaces_test.go b/pkg/worksetapi/service_workspaces_test.go
index 1111111..2222222 100644
--- a/pkg/worksetapi/service_workspaces_test.go
+++ b/pkg/worksetapi/service_workspaces_test.go
@@ -1 +1 @@
-old
+new
`,
			truncated: false,
			totalBytes: 80,
			totalLines: 1,
		});

		const { findByText } = render(RepoDiff, {
			props: {
				repo,
				workspaceId: 'ws-1',
				onClose: vi.fn(),
			},
		});

		expect(await findByText('Local pending changes')).toBeInTheDocument();
		expect(await findByText('pkg/worksetapi/service_workspaces_test.go')).toBeInTheDocument();
	});
});
