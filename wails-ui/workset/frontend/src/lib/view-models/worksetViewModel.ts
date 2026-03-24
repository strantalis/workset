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
export type ThreadStatus = 'active' | 'in-review' | 'merged' | 'stale';

export type ThreadShellSummary = {
	id: string;
	name: string;
	description: string;
	worksetId: string;
	worksetLabel: string;
	repoNames: string[];
	repoCount: number;
	dirtyRepos: number;
	openPrs: number;
	mergedPrs: number;
	reviewCommentsCount: number;
	linesAdded: number;
	linesRemoved: number;
	branch: string;
	health: HealthState[];
	status: ThreadStatus;
	lastActiveTs: number;
	lastActive: string;
	pinned: boolean;
};

export type ExplorerWorksetSummary = {
	id: string;
	label: string;
	description: string;
	threads: ThreadShellSummary[];
	repos: string[];
	health: HealthState[];
	lastActiveTs: number;
	pinned: boolean;
	shortcutNumber?: number;
	activeThreads: number;
	openPrs: number;
	dirtyRepos: number;
	linesAdded: number;
	linesRemoved: number;
};

export type WorksetThreadGroup = {
	id: string;
	label: string;
	repos: string[];
	threads: Workspace[];
};

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

const threadShellSummaryCache = new WeakMap<Workspace, ThreadShellSummary>();

const getThreadStatus = (workspace: Workspace): ThreadStatus => {
	if (workspace.repos.some((repo) => isOpenTrackedPullRequest(repo))) return 'in-review';
	if (workspace.repos.some((repo) => repo.dirty)) return 'active';
	if (workspace.repos.some((repo) => isMergedTrackedPullRequest(repo))) return 'merged';
	const age = Date.now() - (new Date(workspace.lastUsed).getTime() || 0);
	return age > 14 * 24 * 60 * 60 * 1000 ? 'stale' : 'active';
};

const isOpenTrackedPullRequest = (repo: Repo): boolean => {
	const tracked = repo.trackedPullRequest;
	if (!tracked) return false;
	const state = tracked.state.toLowerCase();
	const merged = tracked.merged === true || state === 'merged';
	return state === 'open' && !merged;
};

const buildWorksetDescription = (threads: ThreadShellSummary[]): string => {
	const explicit = threads.find((thread) => thread.description.trim().length > 0);
	if (explicit) return explicit.description.trim();
	const threadNames = threads
		.map((thread) => thread.name.trim())
		.filter((name) => name.length > 0)
		.slice(0, 2);
	if (threadNames.length === 0) return 'No threads yet';
	if (threadNames.length === 1) return threadNames[0];
	return `${threadNames[0]} + ${threadNames[1]}`;
};

export const mapWorkspaceToThreadShellSummary = (workspace: Workspace): ThreadShellSummary => {
	const cached = threadShellSummaryCache.get(workspace);
	if (cached) return cached;
	const identity = deriveWorksetIdentity(workspace);
	const health = workspace.repos.map(getHealthForRepo);
	const dirtyRepos = workspace.repos.filter((repo) => repo.dirty).length;
	const openPrs = workspace.repos.filter((repo) => isOpenTrackedPullRequest(repo)).length;
	const mergedPrs = workspace.repos.filter((repo) => isMergedTrackedPullRequest(repo)).length;
	const reviewCommentsCount = workspace.repos.reduce(
		(total, repo) => total + (repo.trackedPullRequest?.reviewCommentsCount ?? 0),
		0,
	);
	const linesAdded = workspace.repos.reduce((acc, repo) => acc + (repo.diff?.added ?? 0), 0);
	const linesRemoved = workspace.repos.reduce((acc, repo) => acc + (repo.diff?.removed ?? 0), 0);
	const summary: ThreadShellSummary = {
		id: workspace.id,
		name: workspace.name,
		description: getWorkspaceDescription(workspace),
		worksetId: identity.id,
		worksetLabel: identity.label,
		repoNames: workspace.repos.map((repo) => repo.name),
		repoCount: workspace.repos.length,
		dirtyRepos,
		openPrs,
		mergedPrs,
		reviewCommentsCount,
		linesAdded,
		linesRemoved,
		branch: getWorkspaceBranch(workspace),
		health,
		status: getThreadStatus(workspace),
		lastActiveTs: new Date(workspace.lastUsed).getTime() || 0,
		lastActive: formatRelativeTime(workspace.lastUsed),
		pinned: workspace.pinned,
	};
	threadShellSummaryCache.set(workspace, summary);
	return summary;
};

export const mapWorkspacesToThreadShellSummaries = (
	workspaces: Workspace[],
): ThreadShellSummary[] =>
	workspaces
		.filter((workspace) => workspace.placeholder !== true)
		.map(mapWorkspaceToThreadShellSummary);

export const mapWorkspacesToExplorerWorksets = (
	workspaces: Workspace[],
	shortcutMap: Map<string, number>,
): ExplorerWorksetSummary[] => {
	const byWorkset = new Map<
		string,
		{
			label: string;
			threads: ThreadShellSummary[];
			repos: Set<string>;
			health: Set<HealthState>;
			lastActiveTs: number;
			pinned: boolean;
			openPrs: number;
			dirtyRepos: number;
			linesAdded: number;
			linesRemoved: number;
		}
	>();
	for (const workspace of workspaces) {
		if (workspace.archived) continue;
		const summary = mapWorkspaceToThreadShellSummary(workspace);
		const target = byWorkset.get(summary.worksetId) ?? {
			label: summary.worksetLabel,
			threads: [],
			repos: new Set<string>(),
			health: new Set<HealthState>(),
			lastActiveTs: 0,
			pinned: false,
			openPrs: 0,
			dirtyRepos: 0,
			linesAdded: 0,
			linesRemoved: 0,
		};
		if (workspace.placeholder !== true) {
			target.threads.push(summary);
		}
		target.lastActiveTs = Math.max(target.lastActiveTs, summary.lastActiveTs);
		target.pinned = target.pinned || summary.pinned;
		for (const repoName of summary.repoNames) {
			target.repos.add(repoName);
		}
		for (const healthState of summary.health) {
			target.health.add(healthState);
		}
		target.openPrs += summary.openPrs;
		target.dirtyRepos += summary.dirtyRepos;
		target.linesAdded += summary.linesAdded;
		target.linesRemoved += summary.linesRemoved;
		byWorkset.set(summary.worksetId, target);
	}
	return [...byWorkset.entries()]
		.map(([id, value]) => {
			let shortcutNumber: number | undefined;
			for (const thread of value.threads) {
				const shortcut = shortcutMap.get(thread.id);
				if (shortcut === undefined) continue;
				shortcutNumber =
					shortcutNumber === undefined ? shortcut : Math.min(shortcutNumber, shortcut);
			}
			return {
				id,
				label: value.label,
				description: buildWorksetDescription(value.threads),
				threads: [...value.threads],
				repos: [...value.repos].sort((left, right) => left.localeCompare(right)),
				health: [...value.health],
				lastActiveTs: value.lastActiveTs,
				pinned: value.pinned,
				shortcutNumber,
				activeThreads: value.threads.filter(
					(thread) => thread.status === 'active' || thread.status === 'in-review',
				).length,
				openPrs: value.openPrs,
				dirtyRepos: value.dirtyRepos,
				linesAdded: value.linesAdded,
				linesRemoved: value.linesRemoved,
			};
		})
		.sort((left, right) => {
			if (left.pinned !== right.pinned) return left.pinned ? -1 : 1;
			if (left.lastActiveTs !== right.lastActiveTs) return right.lastActiveTs - left.lastActiveTs;
			return left.label.localeCompare(right.label);
		});
};

export const mapWorkspacesToThreadGroups = (workspaces: Workspace[]): WorksetThreadGroup[] => {
	const byId = new Map<string, WorksetThreadGroup>();
	for (const workspace of workspaces) {
		const { id, label } = deriveWorksetIdentity(workspace);
		const repoNames = workspace.repos.map((repo) => repo.name);
		const existing = byId.get(id);
		if (existing) {
			for (const repoName of repoNames) {
				if (!existing.repos.includes(repoName)) {
					existing.repos.push(repoName);
				}
			}
			if (workspace.placeholder !== true) {
				existing.threads.push(workspace);
			}
			continue;
		}
		byId.set(id, {
			id,
			label,
			repos: [...new Set(repoNames)],
			threads: workspace.placeholder === true ? [] : [workspace],
		});
	}
	return [...byId.values()]
		.map((group) => ({
			...group,
			repos: [...group.repos],
			threads: [...group.threads],
		}))
		.sort((left, right) => left.label.localeCompare(right.label));
};

export const mapWorkspaceToSummary = (workspace: Workspace): WorksetSummary => {
	const threadSummary = mapWorkspaceToThreadShellSummary(workspace);

	return {
		id: workspace.id,
		label: workspace.name,
		description: getWorkspaceDescription(workspace),
		workset: normalizeWorkset(workspace),
		repos: threadSummary.repoNames,
		branch: threadSummary.branch,
		repoCount: threadSummary.repoCount,
		dirtyCount: threadSummary.dirtyRepos,
		openPrs: threadSummary.openPrs,
		mergedPrs: threadSummary.mergedPrs,
		linesAdded: threadSummary.linesAdded,
		linesRemoved: threadSummary.linesRemoved,
		lastActive: threadSummary.lastActive,
		lastActiveTs: threadSummary.lastActiveTs,
		health: threadSummary.health,
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
