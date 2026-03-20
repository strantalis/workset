import type { Repo, Workspace } from '../types';
import { formatRelativeTime } from './relativeTime';

export type WorksetIdentity = { id: string; label: string };

export const deriveWorksetIdentity = (workspace: Workspace): WorksetIdentity => {
	const key = workspace.worksetKey?.trim();
	const label = workspace.worksetLabel?.trim();
	const workset = workspace.workset?.trim();
	const normalizedWorkset = workset?.toLowerCase().replace(/\s+/g, '-') ?? '';
	return {
		id:
			key && key.length > 0
				? key
				: normalizedWorkset.length > 0
					? `workset:${normalizedWorkset}`
					: `workspace:${workspace.id.toLowerCase()}`,
		label:
			label && label.length > 0 ? label : workset && workset.length > 0 ? workset : workspace.name,
	};
};

export type HealthState = 'clean' | 'modified' | 'ahead' | 'error';

export type WorksetSummary = {
	id: string;
	label: string;
	description: string;
	workset: string;
	repos: string[];
	branch: string;
	repoCount: number;
	dirtyCount: number;
	openPrs: number;
	mergedPrs: number;
	linesAdded: number;
	linesRemoved: number;
	lastActive: string;
	lastActiveTs: number;
	health: HealthState[];
	pinned: boolean;
	archived: boolean;
	color?: string;
};

const getHealthForRepo = (repo: Repo): HealthState => {
	if (repo.missing) return 'error';
	if (repo.dirty) return 'modified';
	if ((repo.ahead ?? 0) > 0) return 'ahead';
	return 'clean';
};

const getWorkspaceBranch = (workspace: Workspace): string => {
	for (const repo of workspace.repos) {
		if (repo.currentBranch) return repo.currentBranch;
	}
	for (const repo of workspace.repos) {
		if (repo.defaultBranch) return repo.defaultBranch;
	}
	return 'main';
};

const getWorkspaceDescription = (workspace: Workspace): string => {
	if (workspace.description) return workspace.description;
	return '';
};

const isMergedTrackedPullRequest = (repo: Repo): boolean => {
	const tracked = repo.trackedPullRequest;
	if (!tracked) return false;
	if (tracked.merged === true) return true;
	return tracked.state.toLowerCase() === 'merged';
};

const normalizeWorkset = (workspace: Workspace): string => {
	const workset = workspace.workset?.trim();
	return workset && workset.length > 0 ? workset : 'Unassigned';
};

export const mapWorkspaceToSummary = (workspace: Workspace): WorksetSummary => {
	const health = workspace.repos.map(getHealthForRepo);
	const dirtyCount = workspace.repos.filter((repo) => repo.dirty).length;
	const openPrs = workspace.repos.filter(
		(repo) =>
			repo.trackedPullRequest?.state.toLowerCase() === 'open' && !isMergedTrackedPullRequest(repo),
	).length;
	const mergedPrs = workspace.repos.filter((repo) => isMergedTrackedPullRequest(repo)).length;
	const linesAdded = workspace.repos.reduce((acc, repo) => acc + (repo.diff?.added ?? 0), 0);
	const linesRemoved = workspace.repos.reduce((acc, repo) => acc + (repo.diff?.removed ?? 0), 0);

	return {
		id: workspace.id,
		label: workspace.name,
		description: getWorkspaceDescription(workspace),
		workset: normalizeWorkset(workspace),
		repos: workspace.repos.map((repo) => repo.name),
		branch: getWorkspaceBranch(workspace),
		repoCount: workspace.repos.length,
		dirtyCount,
		openPrs,
		mergedPrs,
		linesAdded,
		linesRemoved,
		lastActive: formatRelativeTime(workspace.lastUsed),
		lastActiveTs: new Date(workspace.lastUsed).getTime() || 0,
		health,
		pinned: workspace.pinned,
		archived: workspace.archived,
		color: workspace.color,
	};
};

export const mapWorkspacesToSummaries = (workspaces: Workspace[]): WorksetSummary[] =>
	workspaces.filter((workspace) => workspace.placeholder !== true).map(mapWorkspaceToSummary);

export const buildShortcutMap = (workspaces: Workspace[], max = 5): Map<string, number> => {
	const sorted = [...workspaces]
		.filter((workspace) => !workspace.archived && workspace.placeholder !== true)
		.sort((a, b) => {
			if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
			const aTs = new Date(a.lastUsed).getTime() || 0;
			const bTs = new Date(b.lastUsed).getTime() || 0;
			return bTs - aTs;
		});
	const map = new Map<string, number>();
	for (const [index, workspace] of sorted.slice(0, max).entries()) {
		map.set(workspace.id, index + 1);
	}
	return map;
};

export type GroupMode = 'all' | 'pinned' | 'recent' | 'archived';

export const groupWorksets = (
	worksets: WorksetSummary[],
	mode: GroupMode,
): Array<{ label: string; items: WorksetSummary[] }> => {
	if (mode === 'all') {
		return [{ label: 'All Worksets', items: worksets.filter((item) => !item.archived) }];
	}
	if (mode === 'pinned') {
		return [{ label: 'Pinned', items: worksets.filter((item) => item.pinned && !item.archived) }];
	}
	if (mode === 'archived') {
		return [{ label: 'Archived', items: worksets.filter((item) => item.archived) }];
	}
	const day = 24 * 60 * 60 * 1000;
	const now = Date.now();
	const today: WorksetSummary[] = [];
	const week: WorksetSummary[] = [];
	const older: WorksetSummary[] = [];
	for (const item of worksets.filter((entry) => !entry.archived)) {
		const age = now - item.lastActiveTs;
		if (age < day) {
			today.push(item);
		} else if (age < 7 * day) {
			week.push(item);
		} else {
			older.push(item);
		}
	}
	return [
		{ label: 'Today', items: today },
		{ label: 'This Week', items: week },
		{ label: 'Older', items: older },
	].filter((group) => group.items.length > 0);
};
