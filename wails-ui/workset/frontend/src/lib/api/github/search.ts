import type { GitHubRepoSearchItem } from '../../types';
import type { GitHubRepoSearchItemResponse } from './types';
import { SearchGitHubRepositories } from '../../../../bindings/workset/app';

export async function searchGitHubRepositories(
	query: string,
	limit = 8,
): Promise<GitHubRepoSearchItem[]> {
	const response = (await SearchGitHubRepositories(query, limit)) as GitHubRepoSearchItemResponse[];
	return response.map((item) => ({
		name: item.name,
		fullName: item.full_name,
		owner: item.owner,
		defaultBranch: item.default_branch,
		cloneUrl: item.clone_url,
		sshUrl: item.ssh_url,
		private: item.private,
		archived: item.archived,
		host: item.host,
	}));
}
