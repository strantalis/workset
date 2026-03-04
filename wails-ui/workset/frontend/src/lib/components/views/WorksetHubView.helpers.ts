import type { Repo, Workspace } from '../../types';
import type { HealthState, WorksetSummary } from '../../view-models/worksetViewModel';

export type WorksetGroupMode = 'all' | 'repo' | 'active';
export type WorksetLayoutMode = 'grid' | 'list';
export type ThreadStatus = 'active' | 'in-review' | 'merged' | 'stale';

export type WorksetAggregate = {
	id: string;
	label: string;
	description: string;
	threads: WorksetSummary[];
	repos: string[];
	health: HealthState[];
	branch: string;
	dirtyCount: number;
	openPrs: number;
	mergedPrs: number;
	linesAdded: number;
	linesRemoved: number;
	lastActive: string;
	lastActiveTs: number;
	pinned: boolean;
	archived: boolean;
};

export type WorksetGroup = {
	label: string;
	items: WorksetAggregate[];
};

export type ActiveRepoRow = {
	id: string;
	name: string;
	branch: string;
	ahead: number;
	behind: number;
	dirtyFiles: number;
	prNumber: number | null;
	status: HealthState;
	added: number;
	removed: number;
};

const HEALTH_ORDER: HealthState[] = ['clean', 'modified', 'ahead', 'error'];
const HEALTH_RANK: Record<HealthState, number> = {
	clean: 0,
	ahead: 1,
	modified: 2,
	error: 3,
};

const DAY_MS = 24 * 60 * 60 * 1000;

const parseWorksetKey = (value: string): string => {
	const normalized = value.trim().toLowerCase();
	return normalized.length > 0 ? normalized : 'unassigned';
};

const summarizeThreads = (threads: WorksetSummary[]): WorksetAggregate | null => {
	const orderedThreads = [...threads];
	const latestThread =
		orderedThreads.reduce(
			(latest, thread) => (thread.lastActiveTs > latest.lastActiveTs ? thread : latest),
			orderedThreads[0],
		) ?? null;
	if (!latestThread) return null;

	const repos = new Set<string>();
	const health = new Set<HealthState>();
	let dirtyCount = 0;
	let openPrs = 0;
	let mergedPrs = 0;
	let linesAdded = 0;
	let linesRemoved = 0;
	for (const thread of orderedThreads) {
		for (const repoName of thread.repos) repos.add(repoName);
		for (const state of thread.health) health.add(state);
		dirtyCount += thread.dirtyCount;
		openPrs += thread.openPrs;
		mergedPrs += thread.mergedPrs;
		linesAdded += thread.linesAdded;
		linesRemoved += thread.linesRemoved;
	}

	return {
		id: buildWorksetId(latestThread.workset),
		label: latestThread.workset.trim().length > 0 ? latestThread.workset : 'Unassigned',
		description:
			orderedThreads.find((entry) => entry.description.trim().length > 0)?.description ?? '',
		threads: orderedThreads,
		repos: [...repos].sort((left, right) => left.localeCompare(right)),
		health: HEALTH_ORDER.filter((state) => health.has(state)),
		branch: latestThread.branch,
		dirtyCount,
		openPrs,
		mergedPrs,
		linesAdded,
		linesRemoved,
		lastActive: latestThread.lastActive,
		lastActiveTs: latestThread.lastActiveTs,
		pinned: orderedThreads.some((thread) => thread.pinned),
		archived: orderedThreads.every((thread) => thread.archived),
	};
};

const groupByRepo = (items: WorksetAggregate[]): WorksetGroup[] => {
	const noReposLabel = 'No Repos';
	const repoMap = new Map<string, WorksetAggregate[]>();
	for (const item of items) {
		if (item.repos.length === 0) {
			const bucket = repoMap.get(noReposLabel) ?? [];
			bucket.push(item);
			repoMap.set(noReposLabel, bucket);
			continue;
		}
		for (const repoName of item.repos) {
			const bucket = repoMap.get(repoName) ?? [];
			if (!bucket.some((entry) => entry.id === item.id)) bucket.push(item);
			repoMap.set(repoName, bucket);
		}
	}

	return [...repoMap.entries()]
		.sort((left, right) => {
			if (left[0] === noReposLabel && right[0] !== noReposLabel) return 1;
			if (right[0] === noReposLabel && left[0] !== noReposLabel) return -1;
			const byCount = right[1].length - left[1].length;
			if (byCount !== 0) return byCount;
			return left[0].localeCompare(right[0]);
		})
		.map(([label, groupItems]) => ({ label, items: sortByLabel(groupItems) }));
};

const groupByRecency = (items: WorksetAggregate[]): WorksetGroup[] => {
	const now = Date.now();
	const today: WorksetAggregate[] = [];
	const thisWeek: WorksetAggregate[] = [];
	const older: WorksetAggregate[] = [];

	for (const item of items) {
		const age = now - item.lastActiveTs;
		if (age < DAY_MS) {
			today.push(item);
			continue;
		}
		if (age < DAY_MS * 7) {
			thisWeek.push(item);
			continue;
		}
		older.push(item);
	}

	return [
		{ label: 'Today', items: sortByActivity(today) },
		{ label: 'This Week', items: sortByActivity(thisWeek) },
		{ label: 'Older', items: sortByActivity(older) },
	].filter((group) => group.items.length > 0);
};

const upsertRepoRow = (
	byRepo: Map<string, ActiveRepoRow>,
	repo: Repo,
	status: HealthState,
	openPr: number | null,
): void => {
	const key = repo.name.toLowerCase();
	const current = byRepo.get(key);
	if (!current) {
		byRepo.set(key, {
			id: repo.id,
			name: repo.name,
			branch: repo.currentBranch || repo.defaultBranch || 'main',
			ahead: repo.ahead ?? 0,
			behind: repo.behind ?? 0,
			dirtyFiles: repo.dirty ? repo.files.length : 0,
			prNumber: openPr,
			status,
			added: repo.diff?.added ?? 0,
			removed: repo.diff?.removed ?? 0,
		});
		return;
	}

	current.ahead = Math.max(current.ahead, repo.ahead ?? 0);
	current.behind = Math.max(current.behind, repo.behind ?? 0);
	current.dirtyFiles = Math.max(current.dirtyFiles, repo.dirty ? repo.files.length : 0);
	current.prNumber = current.prNumber ?? openPr;
	current.added += repo.diff?.added ?? 0;
	current.removed += repo.diff?.removed ?? 0;
	if (HEALTH_RANK[status] > HEALTH_RANK[current.status]) {
		current.status = status;
		current.branch = repo.currentBranch || repo.defaultBranch || current.branch;
	}
};

export const getRepoHealth = (repo: Repo): HealthState => {
	if (repo.missing) return 'error';
	if (repo.dirty) return 'modified';
	if ((repo.ahead ?? 0) > 0) return 'ahead';
	return 'clean';
};

export const getHealthStatusLabel = (status: HealthState): string => {
	if (status === 'error') return 'Missing';
	if (status === 'modified') return 'Modified';
	if (status === 'ahead') return 'Ahead';
	return 'Clean';
};

export const getWorkspaceWorksetLabel = (workspace: Workspace): string => {
	const value = workspace.worksetLabel?.trim() || workspace.workset?.trim() || workspace.template?.trim();
	return value && value.length > 0 ? value : workspace.name;
};

export const buildWorksetId = (label: string): string => {
	const slug = label
		.trim()
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/(^-|-$)/g, '');
	return slug.length > 0 ? `workset:${slug}` : 'workset:unassigned';
};

export const getThreadStatus = (thread: WorksetSummary): ThreadStatus => {
	if (thread.openPrs > 0) return 'in-review';
	if (thread.dirtyCount > 0) return 'active';
	if (thread.mergedPrs > 0) return 'merged';
	const age = Date.now() - thread.lastActiveTs;
	return age > 14 * DAY_MS ? 'stale' : 'active';
};

export const buildWorksetAggregates = (items: WorksetSummary[]): WorksetAggregate[] => {
	const byWorkset = new Map<string, WorksetSummary[]>();
	for (const thread of items) {
		const key = parseWorksetKey(thread.workset);
		const bucket = byWorkset.get(key) ?? [];
		bucket.push(thread);
		byWorkset.set(key, bucket);
	}

	const aggregates: WorksetAggregate[] = [];
	for (const [, threads] of byWorkset.entries()) {
		const summary = summarizeThreads(threads);
		if (summary) {
			aggregates.push(summary);
		}
	}
	return aggregates;
};

export const sortByActivity = (items: WorksetAggregate[]): WorksetAggregate[] =>
	[...items].sort((left, right) => {
		if (left.pinned !== right.pinned) return left.pinned ? -1 : 1;
		return right.lastActiveTs - left.lastActiveTs;
	});

export const sortByLabel = (items: WorksetAggregate[]): WorksetAggregate[] =>
	[...items].sort((left, right) => left.label.localeCompare(right.label));

export const buildWorksetGroups = (
	visible: WorksetAggregate[],
	groupMode: WorksetGroupMode,
): WorksetGroup[] => {
	if (groupMode === 'all') {
		return [{ label: '', items: sortByActivity(visible) }];
	}
	if (groupMode === 'repo') {
		return groupByRepo(visible);
	}
	return groupByRecency(visible);
};

export const resolveActiveWorkspaceEntry = (
	workspaceCatalog: Workspace[],
	activeWorkspaceId: string | null,
): Workspace | null => {
	if (workspaceCatalog.length === 0) return null;
	if (activeWorkspaceId) {
		const active = workspaceCatalog.find((workspace) => workspace.id === activeWorkspaceId);
		if (active) return active;
	}
	return workspaceCatalog.find((workspace) => !workspace.archived) ?? workspaceCatalog[0] ?? null;
};

export const resolveActiveWorksetCard = (
	allWorksets: WorksetAggregate[],
	visibleCatalog: WorksetAggregate[],
	activeWorkspaceEntry: Workspace | null,
	activeWorkspaceId: string | null,
): WorksetAggregate | null => {
	if (activeWorkspaceId) {
		const byThread = allWorksets.find((item) =>
			item.threads.some((thread) => thread.id === activeWorkspaceId),
		);
		if (byThread) return byThread;
	}
	if (activeWorkspaceEntry) {
		const worksetId = buildWorksetId(getWorkspaceWorksetLabel(activeWorkspaceEntry));
		const byId = allWorksets.find((item) => item.id === worksetId);
		if (byId) return byId;
	}
	return visibleCatalog[0] ?? null;
};

export const buildActiveWorksetRows = (
	active: WorksetAggregate | null,
	workspaceCatalog: Workspace[],
): ActiveRepoRow[] => {
	if (!active) return [];

	const threads = workspaceCatalog.filter(
		(workspace) => buildWorksetId(getWorkspaceWorksetLabel(workspace)) === active.id,
	);
	if (threads.length === 0) return [];

	const byRepo = new Map<string, ActiveRepoRow>();
	for (const thread of threads) {
		for (const repo of thread.repos) {
			const tracked = repo.trackedPullRequest;
			const openPr =
				tracked && tracked.state.toLowerCase() === 'open' && tracked.merged !== true
					? tracked.number
					: null;
			upsertRepoRow(byRepo, repo, getRepoHealth(repo), openPr);
		}
	}

	return [...byRepo.values()].sort((left, right) => {
		const statusDelta = HEALTH_RANK[right.status] - HEALTH_RANK[left.status];
		if (statusDelta !== 0) return statusDelta;
		return left.name.localeCompare(right.name);
	});
};
