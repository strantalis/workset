import { beforeEach, describe, expect, test, vi } from 'vitest';
import {
	clearRepoFileSearchCache,
	searchWorkspaceRepoFiles,
} from './repo-files';
import { SearchWorkspaceRepoFiles } from '../../../bindings/workset/app';

vi.mock('../../../bindings/workset/app', () => ({
	SearchWorkspaceRepoFiles: vi.fn(),
	ReadWorkspaceRepoFile: vi.fn(),
}));

const mockedSearchWorkspaceRepoFiles = vi.mocked(SearchWorkspaceRepoFiles);

describe('searchWorkspaceRepoFiles cache', () => {
	beforeEach(() => {
		clearRepoFileSearchCache();
		vi.clearAllMocks();
	});

	test('reuses a cached file index across multiple queries', async () => {
		mockedSearchWorkspaceRepoFiles.mockResolvedValue([
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'docs/README.md',
				isMarkdown: true,
				sizeBytes: 24,
				score: 0,
			},
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'internal/config/config.go',
				isMarkdown: false,
				sizeBytes: 128,
				score: 0,
			},
		]);

		const first = await searchWorkspaceRepoFiles('thread-alpha', 'readme', 20);
		const second = await searchWorkspaceRepoFiles('thread-alpha', 'config', 20);

		expect(first).toHaveLength(1);
		expect(second).toHaveLength(1);
		expect(mockedSearchWorkspaceRepoFiles).toHaveBeenCalledTimes(1);
		expect(mockedSearchWorkspaceRepoFiles).toHaveBeenCalledWith({
			workspaceId: 'thread-alpha',
			repoId: undefined,
			query: '',
			limit: 5000,
		});
	});

	test('supports repo-scoped cache keys', async () => {
		mockedSearchWorkspaceRepoFiles.mockResolvedValue([
			{
				workspaceId: 'thread-alpha',
				repoId: 'thread-alpha::api',
				repoName: 'api',
				path: 'docs/README.md',
				isMarkdown: true,
				sizeBytes: 24,
				score: 0,
			},
		]);

		await searchWorkspaceRepoFiles('thread-alpha', '', 20, 'thread-alpha::api');
		await searchWorkspaceRepoFiles('thread-alpha', 'readme', 20, 'thread-alpha::api');

		expect(mockedSearchWorkspaceRepoFiles).toHaveBeenCalledTimes(1);
		expect(mockedSearchWorkspaceRepoFiles).toHaveBeenCalledWith({
			workspaceId: 'thread-alpha',
			repoId: 'thread-alpha::api',
			query: '',
			limit: 5000,
		});
	});
});
