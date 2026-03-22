import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import UnifiedRepoView from './UnifiedRepoView.svelte';
import type {
	PullRequestCreated,
	RepoDiffSummary,
	RepoFileSearchResult,
	Workspace,
} from '../../types';

const notifications = vi.hoisted(() => ({
	info: vi.fn(),
	error: vi.fn(),
}));

const repoFilesMocks = vi.hoisted(() => ({
	readWorkspaceRepoFile: vi.fn(),
	readWorkspaceRepoFileAtRef: vi.fn(),
	searchWorkspaceRepoFiles: vi.fn(),
	writeWorkspaceRepoFile: vi.fn(),
	invalidateRepoFileContent: vi.fn(),
	clearFileContentCache: vi.fn(),
	listRepoDirectory: vi.fn(),
	invalidateRepoDirCache: vi.fn(),
	clearDirListCache: vi.fn(),
	getRepoBlame: vi.fn(),
	createWorkspaceRepoFile: vi.fn(),
	deleteWorkspaceRepoFile: vi.fn(),
}));

const repoDiffMocks = vi.hoisted(() => ({
	fetchRepoDiffSummary: vi.fn(),
	fetchRepoFileDiff: vi.fn(),
	fetchBranchDiffSummary: vi.fn(),
	fetchBranchFileDiff: vi.fn(),
}));

const pullRequestMocks = vi.hoisted(() => ({
	fetchPullRequestReviews: vi.fn(),
	fetchPullRequestStatus: vi.fn(),
	fetchCheckAnnotations: vi.fn(),
}));

vi.mock('../../api/repo-files', () => repoFilesMocks);
vi.mock('../../api/repo-diff', () => repoDiffMocks);
vi.mock('../../api/github/pull-request', () => pullRequestMocks);
vi.mock('../../repoDiffService', () => ({
	subscribeRepoDiffEvent: vi.fn(() => () => {}),
}));
vi.mock('../../state', () => ({
	applyTrackedPullRequest: vi.fn(),
	refreshWorkspacesStatus: vi.fn(async () => {}),
}));
vi.mock('../../markdownImages', () => ({
	resolveMarkdownImages: vi.fn(async (rendered) => rendered),
	clearImageCache: vi.fn(),
}));
vi.mock('../../documentRender', () => ({
	renderMarkdownDocument: vi.fn(async () => ({ html: '<p>rendered</p>', containsMermaid: false })),
}));
vi.mock('../../contexts/notifications', () => ({
	useNotifications: () => notifications,
}));

const emptySummary: RepoDiffSummary = {
	files: [],
	totalAdded: 0,
	totalRemoved: 0,
};

const trackedPr: PullRequestCreated = {
	repo: 'octo/repo-alpha',
	number: 42,
	url: 'https://github.com/octo/repo-alpha/pull/42',
	title: 'Add file badges',
	state: 'open',
	draft: false,
	baseRepo: 'octo/repo-alpha',
	baseBranch: 'main',
	headRepo: 'octo/repo-alpha',
	headBranch: 'feature/lazy-tree',
	commentsCount: 1,
	reviewCommentsCount: 1,
};

const buildWorkspace = (withTrackedPr = false): Workspace => ({
	id: 'ws-1',
	name: 'Workset One',
	path: '/tmp/ws-1',
	archived: false,
	repos: [
		{
			id: 'ws-1::repo-alpha',
			name: 'repo-alpha',
			path: '/tmp/ws-1/repo-alpha',
			defaultBranch: 'main',
			currentBranch: 'feature/lazy-tree',
			ahead: 0,
			behind: 0,
			dirty: false,
			missing: false,
			trackedPullRequest: withTrackedPr ? trackedPr : undefined,
			diff: { added: 0, removed: 0 },
			files: [],
		},
	],
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: '2026-03-20T00:00:00Z',
});

const buildMultiRepoWorkspace = (): Workspace => ({
	...buildWorkspace(),
	repos: [
		{
			id: 'ws-1::repo-alpha',
			name: 'repo-alpha',
			path: '/tmp/ws-1/repo-alpha',
			defaultBranch: 'main',
			currentBranch: 'feature/alpha',
			ahead: 0,
			behind: 0,
			dirty: false,
			missing: false,
			diff: { added: 0, removed: 0 },
			files: [],
		},
		{
			id: 'ws-1::repo-beta',
			name: 'repo-beta',
			path: '/tmp/ws-1/repo-beta',
			defaultBranch: 'main',
			currentBranch: 'feature/beta',
			ahead: 0,
			behind: 0,
			dirty: false,
			missing: false,
			diff: { added: 0, removed: 0 },
			files: [],
		},
	],
});

const createDeferredResults = () => {
	let resolve!: (value: RepoFileSearchResult[]) => void;
	const promise = new Promise<RepoFileSearchResult[]>((resolver) => {
		resolve = resolver;
	});
	return { promise, resolve };
};

describe('UnifiedRepoView lazy directory tree', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		repoFilesMocks.listRepoDirectory.mockReset();
		repoDiffMocks.fetchRepoDiffSummary.mockResolvedValue(emptySummary);
		repoDiffMocks.fetchRepoFileDiff.mockResolvedValue({
			patch: '',
			binary: false,
			truncated: false,
			totalBytes: 0,
			totalLines: 0,
		});
		repoDiffMocks.fetchBranchDiffSummary.mockResolvedValue(emptySummary);
		repoDiffMocks.fetchBranchFileDiff.mockResolvedValue({
			patch: '',
			binary: false,
			truncated: false,
			totalBytes: 0,
			totalLines: 0,
		});
		pullRequestMocks.fetchPullRequestReviews.mockResolvedValue([]);
		pullRequestMocks.fetchPullRequestStatus.mockResolvedValue({
			pullRequest: null,
			checks: [],
		});
		pullRequestMocks.fetchCheckAnnotations.mockResolvedValue([]);
	});

	afterEach(() => {
		cleanup();
	});

	test('expands directories with repo ids that contain double colons', async () => {
		repoFilesMocks.listRepoDirectory
			.mockResolvedValueOnce([{ name: 'src', path: 'src', isDir: true, childCount: 1 }])
			.mockResolvedValueOnce([
				{
					name: 'main.ts',
					path: 'src/main.ts',
					isDir: false,
					sizeBytes: 18,
					isMarkdown: false,
				},
			]);

		const { getByRole } = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspace(),
			},
		});

		await waitFor(() =>
			expect(repoFilesMocks.listRepoDirectory).toHaveBeenCalledWith('ws-1', 'ws-1::repo-alpha', ''),
		);

		await fireEvent.click(getByRole('button', { name: /^src/ }));

		await waitFor(() =>
			expect(repoFilesMocks.listRepoDirectory).toHaveBeenCalledWith(
				'ws-1',
				'ws-1::repo-alpha',
				'src',
			),
		);
		await waitFor(() => expect(getByRole('button', { name: /main\.ts/i })).toBeInTheDocument());
	});

	test('shows an inline directory error when expansion loading fails', async () => {
		repoFilesMocks.listRepoDirectory
			.mockResolvedValueOnce([{ name: 'src', path: 'src', isDir: true, childCount: 1 }])
			.mockRejectedValueOnce(new Error('Failed to load src listing.'));

		const { getByRole, getByText } = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspace(),
			},
		});

		await waitFor(() =>
			expect(repoFilesMocks.listRepoDirectory).toHaveBeenCalledWith('ws-1', 'ws-1::repo-alpha', ''),
		);

		await fireEvent.click(getByRole('button', { name: /^src/ }));

		await waitFor(() => expect(getByText('Failed to load src listing.')).toBeInTheDocument());
	});

	test('refreshes unresolved review-thread badges for directories and files when PR drawer opens', async () => {
		repoFilesMocks.listRepoDirectory
			.mockResolvedValueOnce([{ name: 'src', path: 'src', isDir: true, childCount: 1 }])
			.mockResolvedValueOnce([
				{
					name: 'main.ts',
					path: 'src/main.ts',
					isDir: false,
					sizeBytes: 18,
					isMarkdown: false,
				},
			]);
		repoDiffMocks.fetchBranchDiffSummary.mockResolvedValue({
			files: [{ path: 'src/main.ts', added: 2, removed: 0, status: 'modified', binary: false }],
			totalAdded: 2,
			totalRemoved: 0,
		});
		pullRequestMocks.fetchPullRequestReviews.mockResolvedValueOnce([]).mockResolvedValueOnce([
			{
				id: 1,
				threadId: 'thread-1',
				body: 'needs work',
				path: 'src/main.ts',
				line: 5,
				outdated: false,
				resolved: false,
			},
			{
				id: 2,
				threadId: 'thread-1',
				body: 'reply',
				path: 'src/main.ts',
				line: 5,
				outdated: false,
				resolved: false,
				inReplyTo: 1,
				reply: true,
			},
		]);
		pullRequestMocks.fetchPullRequestStatus.mockResolvedValue({
			pullRequest: trackedPr,
			checks: [],
		});

		const { container, getByRole } = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspace(true),
			},
		});

		await waitFor(() =>
			expect(repoFilesMocks.listRepoDirectory).toHaveBeenCalledWith('ws-1', 'ws-1::repo-alpha', ''),
		);
		expect(container.querySelector('.urv-tree-comment-badge')).toBeNull();

		const prIndicator = container.querySelector('.urv-pr-indicator');
		expect(prIndicator).not.toBeNull();
		await fireEvent.click(prIndicator!);

		await waitFor(() => {
			const dirRow = getByRole('button', { name: /src/i });
			expect(dirRow.querySelector('.urv-tree-comment-badge')?.textContent).toContain('1');
		});

		await fireEvent.click(getByRole('button', { name: /src/i }));

		await waitFor(() => {
			const fileRow = getByRole('button', { name: /main\.ts/i });
			expect(fileRow.querySelector('.urv-tree-file-comments')?.textContent).toContain('1');
		});
	});

	test('indexes the selected repo first and loads one repo at a time during tree search', async () => {
		repoFilesMocks.listRepoDirectory.mockResolvedValue([]);
		const alphaDeferred = createDeferredResults();
		const betaDeferred = createDeferredResults();
		repoFilesMocks.searchWorkspaceRepoFiles.mockImplementation(
			(_workspaceId: string, _query: string, _limit: number, repoId?: string) => {
				if (repoId === 'ws-1::repo-beta') return betaDeferred.promise;
				return alphaDeferred.promise;
			},
		);

		const { getByPlaceholderText, getByRole } = render(UnifiedRepoView, {
			props: {
				workspace: buildMultiRepoWorkspace(),
			},
		});

		await waitFor(() =>
			expect(repoFilesMocks.listRepoDirectory).toHaveBeenCalledWith('ws-1', 'ws-1::repo-alpha', ''),
		);

		await fireEvent.click(getByRole('button', { name: /^repo-beta/ }));

		await waitFor(() =>
			expect(repoFilesMocks.listRepoDirectory).toHaveBeenCalledWith('ws-1', 'ws-1::repo-beta', ''),
		);

		await fireEvent.input(getByPlaceholderText('Filter files...'), {
			target: { value: 'main' },
		});

		await waitFor(() => expect(repoFilesMocks.searchWorkspaceRepoFiles).toHaveBeenCalledTimes(1));
		expect(repoFilesMocks.searchWorkspaceRepoFiles).toHaveBeenNthCalledWith(
			1,
			'ws-1',
			'',
			5000,
			'ws-1::repo-beta',
		);

		betaDeferred.resolve([]);

		await waitFor(() => expect(repoFilesMocks.searchWorkspaceRepoFiles).toHaveBeenCalledTimes(2));
		expect(repoFilesMocks.searchWorkspaceRepoFiles).toHaveBeenNthCalledWith(
			2,
			'ws-1',
			'',
			5000,
			'ws-1::repo-alpha',
		);

		alphaDeferred.resolve([]);
	});
});
