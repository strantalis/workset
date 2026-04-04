import { cleanup, fireEvent, render, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import UnifiedRepoView from './UnifiedRepoView.svelte';
import unifiedRepoViewSource from './UnifiedRepoView.svelte?raw';
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
	clearRepoFileSearchCache: vi.fn(),
	readWorkspaceRepoFile: vi.fn(),
	readWorkspaceRepoFileAtRef: vi.fn(),
	searchWorkspaceRepoFiles: vi.fn(),
	writeWorkspaceRepoFile: vi.fn(),
	invalidateRepoFileContent: vi.fn(),
	clearFileContentCache: vi.fn(),
	listRepoDirectory: vi.fn(),
	listWorkspaceExtraRoots: vi.fn(),
	invalidateRepoDirCache: vi.fn(),
	clearDirListCache: vi.fn(),
	invalidateWorkspaceExtraRoots: vi.fn(),
	clearWorkspaceExtraRootsCache: vi.fn(),
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
	listRemotes: vi.fn(),
	fetchPullRequestReviews: vi.fn(),
	fetchPullRequestStatus: vi.fn(),
	fetchCheckAnnotations: vi.fn(),
}));

const terminalLayoutMocks = vi.hoisted(() => ({
	logTerminalDebug: vi.fn().mockResolvedValue(undefined),
}));

const repoDiffEventBus = vi.hoisted(() => {
	const listeners = new Map<string, Set<(payload: unknown) => void>>();
	return {
		listeners,
		subscribe: vi.fn((event: string, handler: (payload: unknown) => void) => {
			const handlers = listeners.get(event) ?? new Set<(payload: unknown) => void>();
			handlers.add(handler);
			listeners.set(event, handlers);
			return () => {
				const nextHandlers = listeners.get(event);
				if (!nextHandlers) return;
				nextHandlers.delete(handler);
				if (nextHandlers.size === 0) listeners.delete(event);
			};
		}),
		emit: (event: string, payload: unknown) => {
			for (const handler of listeners.get(event) ?? []) {
				handler(payload);
			}
		},
		reset: () => listeners.clear(),
	};
});

vi.mock('../../api/repo-files', () => repoFilesMocks);
vi.mock('../../api/repo-diff', () => repoDiffMocks);
vi.mock('../../api/github/pull-request', () => pullRequestMocks);
vi.mock('../../api/terminal-layout', () => terminalLayoutMocks);
vi.mock('../../repoDiffService', () => ({
	subscribeRepoDiffEvent: repoDiffEventBus.subscribe,
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

const buildWorkspaceWithId = (workspaceId: string, withTrackedPr = false): Workspace => ({
	...buildWorkspace(withTrackedPr),
	id: workspaceId,
	name: `Workset ${workspaceId.toUpperCase()}`,
	path: `/tmp/${workspaceId}`,
	repos: buildWorkspace(withTrackedPr).repos.map((repo, index) => ({
		...repo,
		id: `${workspaceId}::repo-${index === 0 ? 'alpha' : `repo-${index}`}`,
		path: `/tmp/${workspaceId}/${repo.name}`,
	})),
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

const buildWorkspaceWithoutRepos = (): Workspace => ({
	...buildWorkspace(),
	repos: [],
});

const buildWorkspaceWithoutReposWithId = (workspaceId: string): Workspace => ({
	...buildWorkspaceWithId(workspaceId),
	repos: [],
});

const createDeferredResults = () => {
	let resolve!: (value: RepoFileSearchResult[]) => void;
	const promise = new Promise<RepoFileSearchResult[]>((resolver) => {
		resolve = resolver;
	});
	return { promise, resolve };
};

const createDeferredValue = <T>() => {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((resolver) => {
		resolve = resolver;
	});
	return { promise, resolve };
};

const emitRepoDiffSummaryEvent = (payload: {
	workspaceId: string;
	repoId: string;
	summary: RepoDiffSummary;
}): void => {
	repoDiffEventBus.emit('repodiff:summary', payload);
};

describe('UnifiedRepoView lazy directory tree', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		repoDiffEventBus.reset();
		repoFilesMocks.listRepoDirectory.mockReset();
		repoFilesMocks.listWorkspaceExtraRoots.mockResolvedValue([]);
		repoFilesMocks.readWorkspaceRepoFile.mockImplementation(
			async (_workspaceId: string, repoId: string, path: string) => ({
				workspaceId: 'ws-1',
				repoId,
				repoName: 'repo-alpha',
				path,
				content: 'console.log("hello");\n',
				isMarkdown: false,
				isBinary: false,
				isTruncated: false,
				sizeBytes: 21,
			}),
		);
		repoFilesMocks.createWorkspaceRepoFile.mockResolvedValue({ written: true });
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
		pullRequestMocks.listRemotes.mockResolvedValue([]);
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

		const { getByRole, getByTitle } = render(UnifiedRepoView, {
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
		await waitFor(() => expect(getByTitle('src/main.ts')).toBeInTheDocument());
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

		const { container, getByRole, getByTitle } = render(UnifiedRepoView, {
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
			const fileRow = getByTitle('src/main.ts');
			expect(fileRow.querySelector('.urv-tree-file-comments')?.textContent).toContain('1');
		});
	});

	test('uses resolved remote refs for tracked PR summaries and file diffs', async () => {
		repoFilesMocks.listRepoDirectory.mockResolvedValueOnce([
			{
				name: 'README.md',
				path: 'README.md',
				isDir: false,
				sizeBytes: 18,
				isMarkdown: true,
			},
		]);
		pullRequestMocks.listRemotes.mockResolvedValue([
			{ name: 'origin', owner: 'octo', repo: 'repo-alpha' },
		]);
		repoDiffMocks.fetchBranchDiffSummary.mockResolvedValue({
			files: [{ path: 'README.md', added: 3, removed: 1, status: 'modified', binary: false }],
			totalAdded: 3,
			totalRemoved: 1,
		});

		const { getByTitle } = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspace(true),
			},
		});

		await waitFor(() =>
			expect(repoDiffMocks.fetchBranchDiffSummary).toHaveBeenCalledWith(
				'ws-1',
				'ws-1::repo-alpha',
				'origin/main',
				'origin/feature/lazy-tree',
			),
		);

		await fireEvent.click(getByTitle('README.md'));

		await waitFor(() =>
			expect(repoDiffMocks.fetchBranchFileDiff).toHaveBeenCalledWith(
				'ws-1',
				'ws-1::repo-alpha',
				'origin/main',
				'origin/feature/lazy-tree',
				'README.md',
				'',
			),
		);
	});

	test('refreshes PR drawer stats from repo diff summary events', async () => {
		repoFilesMocks.listRepoDirectory.mockResolvedValue([]);
		repoDiffMocks.fetchBranchDiffSummary.mockResolvedValue({
			files: [{ path: 'src/main.ts', added: 2, removed: 1, status: 'modified', binary: false }],
			totalAdded: 2,
			totalRemoved: 1,
		});
		pullRequestMocks.fetchPullRequestStatus.mockResolvedValue({
			pullRequest: trackedPr,
			checks: [],
		});

		const { container, getByText } = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspace(true),
			},
		});

		const prIndicator = container.querySelector('.urv-pr-indicator');
		expect(prIndicator).not.toBeNull();
		await fireEvent.click(prIndicator!);

		await waitFor(() => expect(getByText('1 file')).toBeInTheDocument());
		await waitFor(() => expect(getByText('+2')).toBeInTheDocument());
		await waitFor(() => expect(getByText('-1')).toBeInTheDocument());

		emitRepoDiffSummaryEvent({
			workspaceId: 'ws-1',
			repoId: 'ws-1::repo-alpha',
			summary: {
				files: [{ path: 'src/main.ts', added: 7, removed: 3, status: 'modified', binary: false }],
				totalAdded: 7,
				totalRemoved: 3,
			},
		});

		await waitFor(() => expect(getByText('+7')).toBeInTheDocument());
		await waitFor(() => expect(getByText('-3')).toBeInTheDocument());
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

	test('creates a root-level file inline without requiring a prior tree selection', async () => {
		repoFilesMocks.listRepoDirectory.mockResolvedValueOnce([]);

		const { getByPlaceholderText, getByTitle } = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspace(),
			},
		});

		await waitFor(() =>
			expect(repoFilesMocks.listRepoDirectory).toHaveBeenCalledWith('ws-1', 'ws-1::repo-alpha', ''),
		);

		await fireEvent.click(getByTitle('New file'));

		const input = getByPlaceholderText('new-file.ts') as HTMLInputElement;
		await fireEvent.input(input, { target: { value: 'n' } });
		await Promise.resolve();
		expect(input.selectionStart).toBe(1);
		expect(input.selectionEnd).toBe(1);
		await fireEvent.input(input, { target: { value: 'notes.ts' } });
		await fireEvent.keyDown(input, { key: 'Enter' });

		await waitFor(() =>
			expect(repoFilesMocks.createWorkspaceRepoFile).toHaveBeenCalledWith(
				'ws-1',
				'ws-1::repo-alpha',
				'notes.ts',
			),
		);
		await waitFor(() => expect(getByTitle('notes.ts')).toBeInTheDocument());
	});

	test('creates a child file inline when a directory is selected', async () => {
		repoFilesMocks.listRepoDirectory
			.mockResolvedValueOnce([{ name: 'src', path: 'src', isDir: true, childCount: 0 }])
			.mockResolvedValueOnce([]);

		const { getByPlaceholderText, getByRole, getByTitle } = render(UnifiedRepoView, {
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

		await fireEvent.click(getByTitle('New file'));

		const input = getByPlaceholderText('new-file.ts');
		await fireEvent.input(input, { target: { value: 'child.ts' } });
		await fireEvent.keyDown(input, { key: 'Enter' });

		await waitFor(() =>
			expect(repoFilesMocks.createWorkspaceRepoFile).toHaveBeenCalledWith(
				'ws-1',
				'ws-1::repo-alpha',
				'src/child.ts',
			),
		);
		await waitFor(() => expect(getByTitle('src/child.ts')).toBeInTheDocument());
	});

	test('creates a sibling file inline when a file is selected', async () => {
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

		const { getByPlaceholderText, getByRole, getByTitle } = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspace(),
			},
		});

		await waitFor(() =>
			expect(repoFilesMocks.listRepoDirectory).toHaveBeenCalledWith('ws-1', 'ws-1::repo-alpha', ''),
		);

		await fireEvent.click(getByRole('button', { name: /^src/ }));
		await waitFor(() => expect(getByTitle('src/main.ts')).toBeInTheDocument());
		await fireEvent.click(getByTitle('src/main.ts'));

		await fireEvent.click(getByTitle('New file'));

		const input = getByPlaceholderText('new-file.ts');
		await fireEvent.input(input, { target: { value: 'sibling.ts' } });
		await fireEvent.keyDown(input, { key: 'Enter' });

		await waitFor(() =>
			expect(repoFilesMocks.createWorkspaceRepoFile).toHaveBeenCalledWith(
				'ws-1',
				'ws-1::repo-alpha',
				'src/sibling.ts',
			),
		);
		await waitFor(() => expect(getByTitle('src/sibling.ts')).toBeInTheDocument());
	});

	test('shows inline delete confirmation, removes the file, and supports undo for text files', async () => {
		repoFilesMocks.listRepoDirectory.mockResolvedValueOnce([
			{
				name: 'notes.ts',
				path: 'notes.ts',
				isDir: false,
				sizeBytes: 18,
				isMarkdown: false,
			},
		]);
		repoFilesMocks.deleteWorkspaceRepoFile.mockResolvedValue({ deleted: true });

		const { getByRole, getByTitle, queryByTitle } = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspace(),
			},
		});

		await waitFor(() => expect(getByTitle('notes.ts')).toBeInTheDocument());

		await fireEvent.click(getByRole('button', { name: /delete notes\.ts/i }));
		expect(getByRole('button', { name: /confirm delete notes\.ts/i })).toBeInTheDocument();

		await fireEvent.click(getByRole('button', { name: /confirm delete notes\.ts/i }));

		await waitFor(() =>
			expect(repoFilesMocks.deleteWorkspaceRepoFile).toHaveBeenCalledWith(
				'ws-1',
				'ws-1::repo-alpha',
				'notes.ts',
			),
		);
		await waitFor(() => expect(queryByTitle('notes.ts')).not.toBeInTheDocument());

		const undoNotification = notifications.info.mock.calls.find(
			([message]) => message === 'Deleted notes.ts',
		)?.[1] as { actionLabel?: string; onAction?: () => Promise<void> } | undefined;
		expect(undoNotification?.actionLabel).toBe('Undo');

		await undoNotification?.onAction?.();
		await waitFor(() =>
			expect(repoFilesMocks.createWorkspaceRepoFile).toHaveBeenCalledWith(
				'ws-1',
				'ws-1::repo-alpha',
				'notes.ts',
				'console.log("hello");\n',
			),
		);
		await waitFor(() => expect(getByTitle('notes.ts')).toBeInTheDocument());
	});

	test('shows workspace-root extras when no configured repos exist', async () => {
		repoFilesMocks.listWorkspaceExtraRoots.mockResolvedValue([
			{
				id: 'ws-1::extra::scratch',
				label: 'scratch',
				relativePath: 'scratch',
				gitDetected: false,
			},
		]);
		repoFilesMocks.listRepoDirectory.mockResolvedValue([
			{
				name: 'notes.md',
				path: 'notes.md',
				isDir: false,
				sizeBytes: 12,
				isMarkdown: true,
			},
		]);

		const { getByRole, getByTitle } = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspaceWithoutRepos(),
			},
		});

		await waitFor(() =>
			expect(repoFilesMocks.listWorkspaceExtraRoots).toHaveBeenCalledWith('ws-1'),
		);
		await waitFor(() =>
			expect(repoFilesMocks.listRepoDirectory).toHaveBeenCalledWith(
				'ws-1',
				'ws-1::extra::scratch',
				'',
			),
		);
		expect(getByRole('button', { name: /^scratch/ })).toBeInTheDocument();
		expect(getByTitle('notes.md')).toBeInTheDocument();
	});

	test('does not index workspace-root extras during file search', async () => {
		repoFilesMocks.listRepoDirectory.mockResolvedValue([]);
		repoFilesMocks.listWorkspaceExtraRoots.mockResolvedValue([
			{
				id: 'ws-1::extra::scratch',
				label: 'scratch',
				relativePath: 'scratch',
				gitDetected: false,
			},
		]);
		repoFilesMocks.searchWorkspaceRepoFiles.mockResolvedValue([]);

		const { getByPlaceholderText, getByRole } = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspace(),
			},
		});

		await waitFor(() =>
			expect(repoFilesMocks.listRepoDirectory).toHaveBeenCalledWith('ws-1', 'ws-1::repo-alpha', ''),
		);

		await fireEvent.click(getByRole('button', { name: /^scratch/ }));
		await fireEvent.input(getByPlaceholderText('Filter files...'), {
			target: { value: 'main' },
		});

		await waitFor(() => expect(repoFilesMocks.searchWorkspaceRepoFiles).toHaveBeenCalledTimes(1));
		expect(repoFilesMocks.searchWorkspaceRepoFiles).toHaveBeenCalledWith(
			'ws-1',
			'',
			5000,
			'ws-1::repo-alpha',
		);
	});

	test('ignores stale workspace extra-root responses after switching workspaces', async () => {
		const ws1Deferred =
			createDeferredValue<
				{ id: string; label: string; relativePath: string; gitDetected: boolean }[]
			>();
		repoFilesMocks.listRepoDirectory.mockResolvedValue([]);
		repoFilesMocks.listWorkspaceExtraRoots.mockImplementation((workspaceId: string) => {
			if (workspaceId === 'ws-1') return ws1Deferred.promise;
			if (workspaceId === 'ws-2') {
				return Promise.resolve([
					{
						id: 'ws-2::extra::docs',
						label: 'docs',
						relativePath: 'docs',
						gitDetected: false,
					},
				]);
			}
			return Promise.resolve([]);
		});

		const view = render(UnifiedRepoView, {
			props: {
				workspace: buildWorkspaceWithoutReposWithId('ws-1'),
			},
		});

		await waitFor(() =>
			expect(repoFilesMocks.listWorkspaceExtraRoots).toHaveBeenCalledWith('ws-1'),
		);

		await view.rerender({
			workspace: buildWorkspaceWithoutReposWithId('ws-2'),
		});

		await waitFor(() =>
			expect(repoFilesMocks.listWorkspaceExtraRoots).toHaveBeenCalledWith('ws-2'),
		);
		await waitFor(() => expect(view.getByRole('button', { name: /^docs/ })).toBeInTheDocument());

		ws1Deferred.resolve([
			{
				id: 'ws-1::extra::scratch',
				label: 'scratch',
				relativePath: 'scratch',
				gitDetected: false,
			},
		]);
		await Promise.resolve();

		expect(view.queryByRole('button', { name: /^scratch/ })).not.toBeInTheDocument();
	});

	test('wires onViewReady for both edit and read-only code editors', () => {
		expect(unifiedRepoViewSource.match(/onViewReady=\{handleEditorReady\}/g)).toHaveLength(2);
	});

	test('tracks editor readiness with reactive version state for cross-file definition jumps', () => {
		expect(unifiedRepoViewSource).toContain('let editorViewVersion = $state(0);');
		expect(
			unifiedRepoViewSource.match(/\(editorViewVersion \+= 1\)/g)?.length ?? 0,
		).toBeGreaterThan(0);
		expect(unifiedRepoViewSource).toContain('editorViewVersion === viewVersion');
	});

	test('keeps semantic hover extensions independent from editor view readiness', () => {
		expect(unifiedRepoViewSource).toContain(
			'const handleDefinitionNavigate = createRepoDefinitionNavigateHandler(',
		);
		expect(unifiedRepoViewSource).toContain(
			'const semanticHoverExtensions = $derived.by(() => createRepoSemanticHoverExtensions(wsId, selectedRepoId, selectedFilePath, handleDefinitionNavigate));',
		);
	});

	test('uses request-keyed file refresh state instead of loading from mutable payload effects', () => {
		expect(unifiedRepoViewSource).toContain('let fileRefreshVersion = $state(0);');
		expect(unifiedRepoViewSource).toContain("let fileLoadRequestKey = '';");
		expect(unifiedRepoViewSource).toContain(
			'const selectedDiffFileSignature = $derived.by(() => {',
		);
		expect(unifiedRepoViewSource).toContain('if (requestKey === fileLoadRequestKey) return;');
		expect(unifiedRepoViewSource).toContain('fileRefreshVersion += 1;');
		expect(unifiedRepoViewSource).toContain(
			'const blockedByEdit = editMode && editedContent !== null;',
		);
		expect(unifiedRepoViewSource).toContain('if (blockedByEdit) return;');
	});
});
