import {
	CommitAndPush,
	GetGitHubOperationStatus,
	GetRepoLocalStatus,
	StartCommitAndPushAsync,
} from '../../../../bindings/workset/app';
import { Call } from '@wailsio/runtime';
import { isOperationStatusNotFound, mapGitHubOperationStatus } from './mappers';
import type {
	CommitAndPushResult,
	GitHubOperationStatus,
	GitHubOperationStatusResponse,
	GitHubOperationType,
	LocalMergeResult,
	PushBranchResult,
	RepoLocalStatus,
} from './types';

export async function startCreatePullRequestAsync(
	workspaceId: string,
	repoId: string,
	payload: {
		title?: string;
		body?: string;
		base?: string;
		head?: string;
		baseRemote?: string;
		draft: boolean;
	},
): Promise<GitHubOperationStatus> {
	const result = (await Call.ByName('main.App.StartCreatePullRequestAsync', {
		workspaceId,
		repoId,
		title: payload.title ?? '',
		body: payload.body ?? '',
		base: payload.base ?? '',
		head: payload.head ?? '',
		baseRemote: payload.baseRemote ?? '',
		draft: payload.draft,
	})) as GitHubOperationStatusResponse;

	return mapGitHubOperationStatus(result);
}

export async function startCommitAndPushAsync(
	workspaceId: string,
	repoId: string,
	message?: string,
): Promise<GitHubOperationStatus> {
	const result = (await StartCommitAndPushAsync({
		workspaceId,
		repoId,
		message: message ?? '',
	})) as GitHubOperationStatusResponse;

	return mapGitHubOperationStatus(result);
}

export async function startLocalMergeAsync(
	workspaceId: string,
	repoId: string,
	payload: {
		base?: string;
		message?: string;
	},
): Promise<GitHubOperationStatus> {
	const result = (await Call.ByName('main.App.StartLocalMergeAsync', {
		workspaceId,
		repoId,
		base: payload.base ?? '',
		message: payload.message ?? '',
	})) as GitHubOperationStatusResponse;

	return mapGitHubOperationStatus(result);
}

export async function fetchGitHubOperationStatus(
	workspaceId: string,
	repoId: string,
	type: GitHubOperationType,
): Promise<GitHubOperationStatus | null> {
	try {
		const result = (await GetGitHubOperationStatus({
			workspaceId,
			repoId,
			type,
		})) as GitHubOperationStatusResponse;

		if (!result?.operationId || !result?.type || !result?.state || !result?.stage) {
			return null;
		}

		return mapGitHubOperationStatus(result);
	} catch (err) {
		if (isOperationStatusNotFound(err)) {
			return null;
		}

		throw err;
	}
}

export async function fetchRepoLocalStatus(
	workspaceId: string,
	repoId: string,
): Promise<RepoLocalStatus> {
	const result = (await GetRepoLocalStatus({
		workspaceId,
		repoId,
	})) as RepoLocalStatus;

	return result;
}

export async function commitAndPush(
	workspaceId: string,
	repoId: string,
	message?: string,
): Promise<CommitAndPushResult> {
	const result = (await CommitAndPush({
		workspaceId,
		repoId,
		message: message ?? '',
	})) as CommitAndPushResult;

	return result;
}

export async function localMerge(
	workspaceId: string,
	repoId: string,
	payload: {
		base?: string;
		message?: string;
	},
): Promise<LocalMergeResult> {
	const result = (await Call.ByName('main.App.LocalMerge', {
		workspaceId,
		repoId,
		base: payload.base ?? '',
		message: payload.message ?? '',
	})) as LocalMergeResult;

	return result;
}

export async function pushBranch(
	workspaceId: string,
	repoId: string,
	branch: string,
): Promise<PushBranchResult> {
	const result = (await Call.ByName('main.App.PushBranch', {
		workspaceId,
		repoId,
		branch,
	})) as PushBranchResult;

	return result;
}
