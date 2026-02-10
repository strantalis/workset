import { GetCurrentGitHubUser } from '../../../../bindings/workset/app';
import type { GitHubUser } from './types';

export async function fetchCurrentGitHubUser(
	workspaceId: string,
	repoId: string,
): Promise<GitHubUser> {
	const result = (await GetCurrentGitHubUser({
		workspaceId,
		repoId,
	})) as GitHubUser;

	return result;
}
