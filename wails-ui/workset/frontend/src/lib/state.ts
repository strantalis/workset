import { derived, get, writable, type Writable } from 'svelte/store';
import type {
	PullRequestReviewComment,
	PullRequestSummary,
	Repo,
	RepoDiffSummary,
	Workspace,
} from './types';
import type { RepoLocalStatus } from './api/github';
import {
	fetchWorkspaces,
	pinWorkspace as apiPinWorkspace,
	setWorkspaceColor as apiSetWorkspaceColor,
	setWorkspaceExpanded as apiSetWorkspaceExpanded,
	reorderWorkspaces as apiReorderWorkspaces,
	updateWorkspaceLastUsed as apiUpdateWorkspaceLastUsed,
} from './api/workspaces';

type RepoPatch = {
	dirty?: boolean;
	statusKnown?: boolean;
	missing?: boolean;
	diff?: { added: number; removed: number };
	ahead?: number;
	behind?: number;
	currentBranch?: string;
	trackedPullRequest?: PullRequestSummary | null;
};

type RepoRuntimePatchMap = Map<string, Map<string, RepoPatch>>;

const structuralWorkspaces = writable<Workspace[]>([]);
const repoRuntimePatches = writable<RepoRuntimePatchMap>(new Map());
export const activeWorkspaceId = writable<string | null>(null);
export const activeRepoId = writable<string | null>(null);
export const loadingWorkspaces = writable(false);
export const workspaceError = writable<string | null>(null);
let loadSequence = 0;

const workspaceOverlayCache = new Map<
	string,
	{
		source: Workspace;
		runtime: Map<string, RepoPatch> | undefined;
		repos: Workspace['repos'];
		overlay: Workspace;
	}
>();

const repoOverlayCache = new Map<
	string,
	{
		source: Repo;
		patch: RepoPatch;
		overlay: Repo;
	}
>();

const normalizeTrackedPullRequest = (tracked: PullRequestSummary): PullRequestSummary => ({
	...tracked,
	merged: tracked.merged === true || tracked.state.toLowerCase() === 'merged',
	commentsCount: tracked.commentsCount ?? 0,
	reviewCommentsCount: tracked.reviewCommentsCount ?? 0,
});

const shouldRetainTrackedPullRequest = (tracked: PullRequestSummary): boolean => {
	const state = tracked.state.toLowerCase();
	return tracked.merged === true || state === 'open' || state === 'draft' || state === 'merged';
};

const trackedPullRequestsEqual = (
	left: PullRequestSummary | undefined,
	right: PullRequestSummary | undefined,
): boolean => {
	if (left === right) return true;
	if (!left || !right) return false;
	return (
		left.repo === right.repo &&
		left.number === right.number &&
		left.url === right.url &&
		left.title === right.title &&
		left.body === right.body &&
		left.state === right.state &&
		left.draft === right.draft &&
		left.merged === right.merged &&
		left.baseRepo === right.baseRepo &&
		left.baseBranch === right.baseBranch &&
		left.headRepo === right.headRepo &&
		left.headBranch === right.headBranch &&
		left.updatedAt === right.updatedAt &&
		left.mergeable === right.mergeable &&
		left.author === right.author &&
		left.commentsCount === right.commentsCount &&
		left.reviewCommentsCount === right.reviewCommentsCount
	);
};

const applyPatchToRepo = (repo: Repo, patch: RepoPatch): Repo => {
	let updated = repo;
	const diffPatch = patch.diff;
	if (
		diffPatch &&
		(repo.diff.added !== diffPatch.added || repo.diff.removed !== diffPatch.removed)
	) {
		updated = { ...updated, diff: { added: diffPatch.added, removed: diffPatch.removed } };
	}
	if (patch.dirty !== undefined && updated.dirty !== patch.dirty) {
		updated = { ...updated, dirty: patch.dirty };
	}
	if (patch.statusKnown !== undefined && updated.statusKnown !== patch.statusKnown) {
		updated = { ...updated, statusKnown: patch.statusKnown };
	}
	if (patch.missing !== undefined && updated.missing !== patch.missing) {
		updated = { ...updated, missing: patch.missing };
	}
	if (patch.ahead !== undefined && updated.ahead !== patch.ahead) {
		updated = { ...updated, ahead: patch.ahead };
	}
	if (patch.behind !== undefined && updated.behind !== patch.behind) {
		updated = { ...updated, behind: patch.behind };
	}
	if (patch.currentBranch !== undefined && updated.currentBranch !== patch.currentBranch) {
		updated = { ...updated, currentBranch: patch.currentBranch };
	}
	if (patch.trackedPullRequest !== undefined) {
		const nextTracked =
			patch.trackedPullRequest === null
				? undefined
				: normalizeTrackedPullRequest(patch.trackedPullRequest);
		if (!trackedPullRequestsEqual(updated.trackedPullRequest, nextTracked)) {
			updated = { ...updated, trackedPullRequest: nextTracked };
		}
	}
	return updated;
};

const repoMatchesPatch = (repo: Repo, patch: RepoPatch): boolean => {
	if (
		patch.diff &&
		(repo.diff.added !== patch.diff.added || repo.diff.removed !== patch.diff.removed)
	) {
		return false;
	}
	if (patch.dirty !== undefined && repo.dirty !== patch.dirty) return false;
	if (patch.statusKnown !== undefined && repo.statusKnown !== patch.statusKnown) return false;
	if (patch.missing !== undefined && repo.missing !== patch.missing) return false;
	if (patch.ahead !== undefined && repo.ahead !== patch.ahead) return false;
	if (patch.behind !== undefined && repo.behind !== patch.behind) return false;
	if (patch.currentBranch !== undefined && repo.currentBranch !== patch.currentBranch) return false;
	if (patch.trackedPullRequest !== undefined) {
		const nextTracked =
			patch.trackedPullRequest === null
				? undefined
				: normalizeTrackedPullRequest(patch.trackedPullRequest);
		if (!trackedPullRequestsEqual(repo.trackedPullRequest, nextTracked)) return false;
	}
	return true;
};

const repoPatchesEqual = (left: RepoPatch | undefined, right: RepoPatch): boolean => {
	if (!left) return false;
	if (left.dirty !== right.dirty) return false;
	if (left.statusKnown !== right.statusKnown) return false;
	if (left.missing !== right.missing) return false;
	if (left.ahead !== right.ahead) return false;
	if (left.behind !== right.behind) return false;
	if (left.currentBranch !== right.currentBranch) return false;
	const leftDiff = left.diff;
	const rightDiff = right.diff;
	if (!!leftDiff !== !!rightDiff) return false;
	if (
		leftDiff &&
		rightDiff &&
		(leftDiff.added !== rightDiff.added || leftDiff.removed !== rightDiff.removed)
	) {
		return false;
	}
	const leftTracked =
		left.trackedPullRequest === undefined
			? undefined
			: left.trackedPullRequest === null
				? undefined
				: normalizeTrackedPullRequest(left.trackedPullRequest);
	const rightTracked =
		right.trackedPullRequest === undefined
			? undefined
			: right.trackedPullRequest === null
				? undefined
				: normalizeTrackedPullRequest(right.trackedPullRequest);
	return trackedPullRequestsEqual(leftTracked, rightTracked);
};

const pruneRepoRuntimePatches = (
	workspaces: Workspace[],
	current: RepoRuntimePatchMap,
): RepoRuntimePatchMap => {
	let changed = false;
	const workspaceMap = new Map(workspaces.map((workspace) => [workspace.id, workspace]));
	const next = new Map<string, Map<string, RepoPatch>>();
	for (const [workspaceId, repoPatches] of current) {
		const workspace = workspaceMap.get(workspaceId);
		if (!workspace) {
			changed = true;
			continue;
		}
		const repoMap = new Map(workspace.repos.map((repo) => [repo.id, repo]));
		let nextRepoPatches: Map<string, RepoPatch> | null = null;
		for (const [repoId, patch] of repoPatches) {
			const repo = repoMap.get(repoId);
			if (!repo || repoMatchesPatch(repo, patch)) {
				changed = true;
				continue;
			}
			if (!nextRepoPatches) nextRepoPatches = new Map();
			nextRepoPatches.set(repoId, patch);
		}
		if (nextRepoPatches && nextRepoPatches.size > 0) {
			next.set(workspaceId, nextRepoPatches);
			if (nextRepoPatches.size !== repoPatches.size) changed = true;
		} else if (repoPatches.size > 0) {
			changed = true;
		}
	}
	return changed ? next : current;
};

const buildOverlayWorkspaces = (
	structural: Workspace[],
	runtimePatches: RepoRuntimePatchMap,
): Workspace[] => {
	const nextIds = new Set(structural.map((workspace) => workspace.id));
	for (const cachedId of workspaceOverlayCache.keys()) {
		if (!nextIds.has(cachedId)) workspaceOverlayCache.delete(cachedId);
	}
	const overlaid: Workspace[] = [];
	for (const workspace of structural) {
		const runtime = runtimePatches.get(workspace.id);
		if (!runtime || runtime.size === 0) {
			workspaceOverlayCache.delete(workspace.id);
			overlaid.push(workspace);
			continue;
		}
		const cachedWorkspace = workspaceOverlayCache.get(workspace.id);
		if (
			cachedWorkspace &&
			cachedWorkspace.source === workspace &&
			cachedWorkspace.runtime === runtime
		) {
			overlaid.push(cachedWorkspace.overlay);
			continue;
		}
		let reposChanged = false;
		const repos = workspace.repos.map((repo) => {
			const patch = runtime.get(repo.id);
			if (!patch) return repo;
			const cacheKey = `${workspace.id}\u0000${repo.id}`;
			const cachedRepo = repoOverlayCache.get(cacheKey);
			if (cachedRepo && cachedRepo.source === repo && cachedRepo.patch === patch) {
				reposChanged = reposChanged || cachedRepo.overlay !== repo;
				return cachedRepo.overlay;
			}
			const overlay = applyPatchToRepo(repo, patch);
			repoOverlayCache.set(cacheKey, { source: repo, patch, overlay });
			reposChanged = reposChanged || overlay !== repo;
			return overlay;
		});
		const overlay = reposChanged ? { ...workspace, repos } : workspace;
		workspaceOverlayCache.set(workspace.id, {
			source: workspace,
			runtime,
			repos,
			overlay,
		});
		overlaid.push(overlay);
	}
	return overlaid;
};

const overlaidWorkspaces = derived(
	[structuralWorkspaces, repoRuntimePatches],
	([$structuralWorkspaces, $repoRuntimePatches]) =>
		buildOverlayWorkspaces($structuralWorkspaces, $repoRuntimePatches),
);

const writeStructuralWorkspaces = (next: Workspace[]): void => {
	structuralWorkspaces.set(next);
	repoRuntimePatches.update((current) => pruneRepoRuntimePatches(next, current));
};

export const workspaces: Writable<Workspace[]> = {
	subscribe: overlaidWorkspaces.subscribe,
	set(value) {
		writeStructuralWorkspaces(value);
	},
	update(updater) {
		writeStructuralWorkspaces(updater(get(overlaidWorkspaces)));
	},
};

export const activeWorkspace = derived(
	[workspaces, activeWorkspaceId],
	([$workspaces, $activeWorkspaceId]) =>
		$workspaces.find(
			(workspace) => workspace.id === $activeWorkspaceId && workspace.placeholder !== true,
		) ?? null,
);

export const activeRepo = derived(
	[activeWorkspace, activeRepoId],
	([$activeWorkspace, $activeRepoId]) =>
		$activeWorkspace?.repos.find((repo) => repo.id === $activeRepoId) ?? null,
);

// Derived stores for pinned and unpinned workspaces
export const pinnedWorkspaces = derived(workspaces, ($workspaces) =>
	$workspaces
		.filter((w) => w.pinned && !w.archived && w.placeholder !== true)
		.sort((a, b) => a.pinOrder - b.pinOrder),
);

export const unpinnedWorkspaces = derived(workspaces, ($workspaces) =>
	$workspaces
		.filter((w) => !w.pinned && !w.archived && w.placeholder !== true)
		.sort((a, b) => {
			const aTs = new Date(a.lastUsed).getTime() || 0;
			const bTs = new Date(b.lastUsed).getTime() || 0;
			return bTs - aTs;
		}),
);

export function selectWorkspace(workspaceId: string): void {
	const target = get(workspaces).find((workspace) => workspace.id === workspaceId);
	if (!target || target.placeholder === true) {
		return;
	}
	activeWorkspaceId.set(workspaceId);
	activeRepoId.set(null);
	// Update last used timestamp in background
	void apiUpdateWorkspaceLastUsed(workspaceId).then(() => {
		// Refresh workspace list to get updated lastUsed
		void refreshWorkspacesStatus();
	});
}

// Update workspace pin status
export async function toggleWorkspacePin(workspaceId: string, pin: boolean): Promise<void> {
	// Optimistically update local state first
	workspaces.update((list) => {
		const workspace = list.find((w) => w.id === workspaceId);
		if (!workspace) return list;

		// Calculate new pin order if pinning
		let newPinOrder = workspace.pinOrder;
		if (pin && !workspace.pinned) {
			const maxOrder = Math.max(...list.filter((w) => w.pinned).map((w) => w.pinOrder), -1);
			newPinOrder = maxOrder + 1;
		} else if (!pin) {
			newPinOrder = 0;
		}

		return list.map((w) =>
			w.id === workspaceId ? { ...w, pinned: pin, pinOrder: newPinOrder } : w,
		);
	});

	// Then try to sync with backend
	try {
		await apiPinWorkspace(workspaceId, pin);
	} catch (error) {
		// eslint-disable-next-line no-console
		console.error('Failed to sync pin status:', error);
		// Could revert here if needed, but optimistic UI is fine for now
	}
}

// Update workspace color
export async function setWorkspaceColor(workspaceId: string, color: string): Promise<void> {
	// Optimistically update local state first
	workspaces.update((list) => list.map((w) => (w.id === workspaceId ? { ...w, color } : w)));

	// Then try to sync with backend
	try {
		await apiSetWorkspaceColor(workspaceId, color);
	} catch (error) {
		// eslint-disable-next-line no-console
		console.error('Failed to sync workspace color:', error);
	}
}

// Update workspace expanded state
export async function setWorkspaceExpanded(workspaceId: string, expanded: boolean): Promise<void> {
	// Optimistically update local state first
	workspaces.update((list) => list.map((w) => (w.id === workspaceId ? { ...w, expanded } : w)));

	// Then try to sync with backend
	try {
		await apiSetWorkspaceExpanded(workspaceId, expanded);
	} catch (error) {
		// eslint-disable-next-line no-console
		console.error('Failed to sync workspace expanded state:', error);
	}
}

// Reorder workspaces after drag and drop
export async function reorderWorkspaces(orders: Record<string, number>): Promise<void> {
	// Optimistically update local state first
	workspaces.update((list) =>
		list.map((w) => (orders[w.id] !== undefined ? { ...w, pinOrder: orders[w.id] } : w)),
	);

	// Then try to sync with backend
	try {
		await apiReorderWorkspaces(orders);
	} catch (error) {
		// eslint-disable-next-line no-console
		console.error('Failed to sync workspace reorder:', error);
	}
}

export function selectRepo(repoId: string): void {
	activeRepoId.set(repoId);
}

export function clearRepo(): void {
	activeRepoId.set(null);
}

export function clearWorkspace(): void {
	activeWorkspaceId.set(null);
	activeRepoId.set(null);
}

const syncSelection = (data: Workspace[]): void => {
	const currentWorkspaceId = get(activeWorkspaceId);
	const currentRepoId = get(activeRepoId);
	const activeWorkspace =
		currentWorkspaceId &&
		data.find(
			(workspace) =>
				workspace.id === currentWorkspaceId &&
				!workspace.archived &&
				workspace.placeholder !== true,
		);
	if (!activeWorkspace) {
		activeWorkspaceId.set(null);
		activeRepoId.set(null);
		return;
	}
	if (currentRepoId && !activeWorkspace.repos.some((repo) => repo.id === currentRepoId)) {
		activeRepoId.set(null);
	}
};

export async function loadWorkspaces(includeArchived = false): Promise<void> {
	const sequence = ++loadSequence;
	loadingWorkspaces.set(true);
	workspaceError.set(null);
	try {
		const data = await fetchWorkspaces(includeArchived, false);
		if (sequence !== loadSequence) {
			return;
		}
		workspaces.set(data);
		syncSelection(data);
	} catch (error) {
		const message = error instanceof Error ? error.message : 'Failed to load workspaces.';
		workspaceError.set(message);
	} finally {
		loadingWorkspaces.set(false);
	}

	void fetchWorkspaces(includeArchived, true)
		.then((data) => {
			if (sequence !== loadSequence) {
				return;
			}
			workspaces.set(data);
			syncSelection(data);
		})
		.catch(() => {});
}

export async function refreshWorkspacesStatus(includeArchived = false): Promise<void> {
	const sequence = ++loadSequence;
	try {
		const data = await fetchWorkspaces(includeArchived, true);
		if (sequence !== loadSequence) {
			return;
		}
		workspaces.set(data);
		syncSelection(data);
	} catch {
		// Ignore status refresh failures to avoid interrupting the UI.
	}
}

const applyRepoPatch = (workspaceId: string, repoId: string, patch: RepoPatch): void => {
	repoRuntimePatches.update((current) => {
		const next = new Map(current);
		const currentWorkspacePatches = current.get(workspaceId);
		const workspacePatches = currentWorkspacePatches ? new Map(currentWorkspacePatches) : new Map();
		const mergedPatch = { ...(workspacePatches.get(repoId) ?? {}), ...patch };
		const structuralWorkspace = get(structuralWorkspaces).find(
			(workspace) => workspace.id === workspaceId,
		);
		const structuralRepo = structuralWorkspace?.repos.find((repo) => repo.id === repoId);
		if (structuralRepo && repoMatchesPatch(structuralRepo, mergedPatch)) {
			if (!workspacePatches.has(repoId)) {
				return current;
			}
			workspacePatches.delete(repoId);
			if (workspacePatches.size === 0) {
				next.delete(workspaceId);
			} else {
				next.set(workspaceId, workspacePatches);
			}
			return next;
		}
		if (repoPatchesEqual(workspacePatches.get(repoId), mergedPatch)) {
			return current;
		}
		workspacePatches.set(repoId, mergedPatch);
		next.set(workspaceId, workspacePatches);
		return next;
	});
};

export const applyRepoDiffSummary = (
	workspaceId: string,
	repoId: string,
	summary: RepoDiffSummary,
): void => {
	applyRepoPatch(workspaceId, repoId, {
		diff: { added: summary.totalAdded, removed: summary.totalRemoved },
	});
};

export const applyRepoLocalStatus = (
	workspaceId: string,
	repoId: string,
	status: RepoLocalStatus,
): void => {
	const patch: RepoPatch = {
		dirty: status.hasUncommitted,
		statusKnown: true,
		ahead: status.ahead,
		behind: status.behind,
		currentBranch: status.currentBranch,
	};
	if (!status.hasUncommitted) {
		patch.diff = { added: 0, removed: 0 };
	}
	applyRepoPatch(workspaceId, repoId, patch);
};

export const applyTrackedPullRequest = (
	workspaceId: string,
	repoId: string,
	trackedPullRequest: PullRequestSummary | null,
): void => {
	if (!trackedPullRequest) {
		applyRepoPatch(workspaceId, repoId, { trackedPullRequest: null });
		return;
	}

	const normalized = normalizeTrackedPullRequest(trackedPullRequest);
	applyRepoPatch(workspaceId, repoId, {
		trackedPullRequest: shouldRetainTrackedPullRequest(normalized) ? normalized : null,
	});
};

export const applyTrackedPullRequestReviewComments = (
	workspaceId: string,
	repoId: string,
	comments: PullRequestReviewComment[],
): void => {
	const repo = get(workspaces)
		.find((workspace) => workspace.id === workspaceId)
		?.repos.find((candidate) => candidate.id === repoId);
	if (!repo?.trackedPullRequest) return;
	const currentReviewCount = repo.trackedPullRequest.reviewCommentsCount ?? 0;
	const currentTotalCount = repo.trackedPullRequest.commentsCount ?? currentReviewCount;
	const baseCommentCount = Math.max(0, currentTotalCount - currentReviewCount);
	const nextReviewCount = comments.length;
	const nextTotalCount = baseCommentCount + nextReviewCount;
	if (currentReviewCount === nextReviewCount && currentTotalCount === nextTotalCount) return;
	applyRepoPatch(workspaceId, repoId, {
		trackedPullRequest: {
			...repo.trackedPullRequest,
			commentsCount: nextTotalCount,
			reviewCommentsCount: nextReviewCount,
		},
	});
};
