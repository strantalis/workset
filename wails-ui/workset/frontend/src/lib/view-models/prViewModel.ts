import type { Workspace } from '../types';
import { formatRelativeTime } from './relativeTime';

export type PrListItem = {
	id: string;
	repoId: string;
	repoName: string;
	title: string;
	branch: string;
	status: 'open' | 'running' | 'blocked';
	dirty: boolean;
	dirtyFiles: number;
	ahead: number;
	behind: number;
	updatedAtLabel: string;
};

const getStatus = (
	missing: boolean,
	dirty: boolean,
	trackedState?: string,
): PrListItem['status'] => {
	if (missing) return 'blocked';
	if (trackedState?.toLowerCase() === 'open') return 'open';
	if (dirty) return 'running';
	return 'open';
};

export const mapWorkspaceToPrItems = (workspace: Workspace | null): PrListItem[] => {
	if (!workspace) return [];
	return workspace.repos.map((repo) => ({
		id: `${workspace.id}:${repo.id}`,
		repoId: repo.id,
		repoName: repo.name,
		title: repo.trackedPullRequest?.title?.trim() || repo.name,
		branch:
			repo.trackedPullRequest?.headBranch ?? repo.currentBranch ?? repo.defaultBranch ?? 'main',
		status: getStatus(repo.missing, repo.dirty, repo.trackedPullRequest?.state),
		dirty: repo.dirty,
		dirtyFiles: repo.files.length,
		ahead: repo.ahead ?? 0,
		behind: repo.behind ?? 0,
		updatedAtLabel: formatRelativeTime(repo.trackedPullRequest?.updatedAt ?? workspace.lastUsed),
	}));
};

export const partitionPrItems = (
	items: PrListItem[],
): {
	active: PrListItem[];
	readyToPR: PrListItem[];
} => ({
	active: items.filter((item) => item.status !== 'blocked'),
	readyToPR: items.filter((item) => item.ahead > 0 || item.dirtyFiles > 0),
});
