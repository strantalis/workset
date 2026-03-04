import type { RepoDiffSummary, RepoFileDiff } from '../types';
import {
	GetBranchDiffSummary,
	GetBranchFileDiff,
	GetRepoDiff,
	GetRepoDiffSummary,
	GetRepoFileDiff,
	StartRepoDiffWatch,
	StopRepoDiffWatch,
	UpdateRepoDiffWatch,
} from '../../../bindings/workset/app';

export type RepoDiffSnapshot = {
	patch: string;
};

export async function startRepoDiffWatch(
	workspaceId: string,
	repoId: string,
	prNumber?: number,
	prBranch?: string,
): Promise<boolean> {
	return (await StartRepoDiffWatch({
		workspaceId,
		repoId,
		prNumber: prNumber ?? 0,
		prBranch: prBranch ?? '',
		localOnly: false,
	})) as boolean;
}

export async function startRepoStatusWatch(workspaceId: string, repoId: string): Promise<boolean> {
	return (await StartRepoDiffWatch({
		workspaceId,
		repoId,
		prNumber: 0,
		prBranch: '',
		localOnly: true,
	})) as boolean;
}

export async function updateRepoDiffWatch(
	workspaceId: string,
	repoId: string,
	prNumber?: number,
	prBranch?: string,
): Promise<boolean> {
	return (await UpdateRepoDiffWatch({
		workspaceId,
		repoId,
		prNumber: prNumber ?? 0,
		prBranch: prBranch ?? '',
		localOnly: false,
	})) as boolean;
}

export async function stopRepoDiffWatch(workspaceId: string, repoId: string): Promise<boolean> {
	return (await StopRepoDiffWatch({
		workspaceId,
		repoId,
		prNumber: 0,
		prBranch: '',
		localOnly: false,
	})) as boolean;
}

export async function stopRepoStatusWatch(workspaceId: string, repoId: string): Promise<boolean> {
	return (await StopRepoDiffWatch({
		workspaceId,
		repoId,
		prNumber: 0,
		prBranch: '',
		localOnly: true,
	})) as boolean;
}

export async function fetchRepoDiff(
	workspaceId: string,
	repoId: string,
): Promise<RepoDiffSnapshot> {
	return GetRepoDiff(workspaceId, repoId);
}

export async function fetchRepoDiffSummary(
	workspaceId: string,
	repoId: string,
): Promise<RepoDiffSummary> {
	return GetRepoDiffSummary(workspaceId, repoId);
}

export async function fetchRepoFileDiff(
	workspaceId: string,
	repoId: string,
	path: string,
	prevPath: string,
	status: string,
): Promise<RepoFileDiff> {
	return GetRepoFileDiff(workspaceId, repoId, path, prevPath, status);
}

export async function fetchBranchDiffSummary(
	workspaceId: string,
	repoId: string,
	base: string,
	head: string,
): Promise<RepoDiffSummary> {
	return GetBranchDiffSummary(workspaceId, repoId, base, head);
}

export async function fetchBranchFileDiff(
	workspaceId: string,
	repoId: string,
	base: string,
	head: string,
	path: string,
	prevPath: string,
): Promise<RepoFileDiff> {
	return GetBranchFileDiff(workspaceId, repoId, base, head, path, prevPath);
}
