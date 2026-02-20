import { describe, expect, test, vi } from 'vitest';
import { SearchGitHubRepositories } from '../../../../bindings/workset/app';
import { searchGitHubRepositories } from './search';

vi.mock('../../../../bindings/workset/app', () => ({
	SearchGitHubRepositories: vi.fn(),
}));

describe('searchGitHubRepositories', () => {
	test('maps snake_case response to frontend GitHub repo search items', async () => {
		vi.mocked(SearchGitHubRepositories).mockResolvedValue([
			{
				name: 'workset',
				full_name: 'strantalis/workset',
				owner: 'strantalis',
				default_branch: 'main',
				clone_url: 'https://github.com/strantalis/workset.git',
				ssh_url: 'git@github.com:strantalis/workset.git',
				private: false,
				archived: false,
				host: 'github.com',
			},
		]);

		const result = await searchGitHubRepositories('workset');

		expect(SearchGitHubRepositories).toHaveBeenCalledWith('workset', 8);
		expect(result).toEqual([
			{
				name: 'workset',
				fullName: 'strantalis/workset',
				owner: 'strantalis',
				defaultBranch: 'main',
				cloneUrl: 'https://github.com/strantalis/workset.git',
				sshUrl: 'git@github.com:strantalis/workset.git',
				private: false,
				archived: false,
				host: 'github.com',
			},
		]);
	});
});
