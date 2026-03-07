import type { Workspace } from '../types';
import { formatRelativeTime } from './relativeTime';

export type PrListItem = {
	id: string;
	repoId: string;
	repoName: string;
	title: string;
	branch: string;
	status: 'open' | 'merged' | 'running' | 'blocked';
	draft: boolean;
	dirty: boolean;
	hasLocalDiff: boolean;
	dirtyFiles: number;
	ahead: number;
	behind: number;
	updatedAtLabel: string;
	author: string;
	commentsCount: number;
	reviewCommentsCount: number;
};

const getStatus = (
	missing: boolean,
	dirty: boolean,
	trackedState?: string,
	trackedMerged?: boolean,
): PrListItem['status'] => {
	if (missing) return 'blocked';
	if (trackedMerged || trackedState?.toLowerCase() === 'merged') return 'merged';
	if (trackedState?.toLowerCase() === 'open') return 'open';
	if (dirty) return 'running';
	return 'open';
};

export const mapWorkspaceToPrItems = (workspace: Workspace | null): PrListItem[] => {
	if (!workspace) return [];
	return workspace.repos.map((repo) => ({
		// Keep this predicate aligned with App.handleSelectRepo and PROrchestrationView partitioning.
		hasLocalDiff: repo.dirty || (repo.diff.added ?? 0) > 0 || (repo.diff.removed ?? 0) > 0,
		id: `${workspace.id}:${repo.id}`,
		repoId: repo.id,
		repoName: repo.name,
		title: repo.trackedPullRequest?.title?.trim() || repo.name,
		branch:
			repo.trackedPullRequest?.headBranch ?? repo.currentBranch ?? repo.defaultBranch ?? 'main',
		status: getStatus(
			repo.missing,
			repo.dirty,
			repo.trackedPullRequest?.state,
			repo.trackedPullRequest?.merged,
		),
		draft: repo.trackedPullRequest?.draft ?? false,
		dirty: repo.dirty,
		dirtyFiles: repo.files.length,
		ahead: repo.ahead ?? 0,
		behind: repo.behind ?? 0,
		updatedAtLabel: formatRelativeTime(repo.trackedPullRequest?.updatedAt ?? workspace.lastUsed),
		author: repo.trackedPullRequest?.author ?? '',
		commentsCount: repo.trackedPullRequest?.commentsCount ?? 0,
		reviewCommentsCount: repo.trackedPullRequest?.reviewCommentsCount ?? 0,
	}));
};

export const partitionPrItems = (
	items: PrListItem[],
): {
	active: PrListItem[];
	readyToPR: PrListItem[];
} => ({
	active: items.filter((item) => item.status !== 'blocked'),
	readyToPR: items.filter((item) => item.ahead > 0 || item.hasLocalDiff),
});
