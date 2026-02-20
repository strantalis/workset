import type {
	PullRequestCheck,
	PullRequestCreated,
	PullRequestReviewComment,
	PullRequestStatusResult,
	RepoDiffFileSummary,
	Workspace,
} from '../../types';
import type { GitHubOperationStage } from '../../api/github';
import { buildFileLocalCacheKey, buildFilePrCacheKey } from '../../cache/repoDiffCache';

export type RepoDiffPrStatusEvent = {
	workspaceId: string;
	repoId: string;
	status: {
		pullRequest: {
			repo: string;
			number: number;
			url: string;
			title: string;
			state: string;
			draft: boolean;
			base_repo: string;
			base_branch: string;
			head_repo: string;
			head_branch: string;
			mergeable?: string;
		};
		checks: Array<{
			name: string;
			status: string;
			conclusion?: string;
			details_url?: string;
			started_at?: string;
			completed_at?: string;
			check_run_id?: number;
		}>;
	};
};

export type PullRequestReviewThread = {
	id: string;
	comments: PullRequestReviewComment[];
	path: string;
	line?: number;
	resolved: boolean;
	outdated: boolean;
};

type ViewMode = 'active' | 'ready';
type PrListItemRef = { id: string };
type PrBranches = { base: string; head: string };
type TrackedPrMap = Map<string, PullRequestCreated>;

const SIDEBAR_COLLAPSED_KEY = 'workset:pr-orchestration:sidebarCollapsed';

export const buildTrackedPrMap = (workspace: Workspace | null): Map<string, PullRequestCreated> => {
	const nextMap = new Map<string, PullRequestCreated>();
	if (!workspace) return nextMap;
	for (const repo of workspace.repos) {
		if (repo.trackedPullRequest) {
			nextMap.set(repo.id, repo.trackedPullRequest);
		}
	}
	return nextMap;
};

const isOpenPullRequest = (pr: PullRequestCreated | undefined): pr is PullRequestCreated =>
	Boolean(pr && pr.state.trim().toLowerCase() === 'open');

export const mergeTrackedPrMap = (
	workspace: Workspace | null,
	currentMap: Map<string, PullRequestCreated>,
	suppressedRepoIds: ReadonlySet<string> = new Set<string>(),
): Map<string, PullRequestCreated> => {
	if (!workspace) {
		return new Map<string, PullRequestCreated>();
	}

	const nextMap = new Map<string, PullRequestCreated>();
	for (const repo of workspace.repos) {
		if (suppressedRepoIds.has(repo.id)) {
			continue;
		}
		if (repo.trackedPullRequest) {
			if (isOpenPullRequest(repo.trackedPullRequest)) {
				nextMap.set(repo.id, repo.trackedPullRequest);
			}
			continue;
		}

		const cached = currentMap.get(repo.id);
		if (isOpenPullRequest(cached)) {
			nextMap.set(repo.id, cached);
		}
	}
	return nextMap;
};

export const trackedPrMapsEqual = (left: TrackedPrMap, right: TrackedPrMap): boolean => {
	if (left.size !== right.size) return false;
	for (const [repoId, leftPr] of left) {
		const rightPr = right.get(repoId);
		if (!rightPr) return false;
		if (
			leftPr.number !== rightPr.number ||
			leftPr.url !== rightPr.url ||
			leftPr.state !== rightPr.state ||
			leftPr.baseRepo !== rightPr.baseRepo ||
			leftPr.baseBranch !== rightPr.baseBranch ||
			leftPr.headRepo !== rightPr.headRepo ||
			leftPr.headBranch !== rightPr.headBranch
		) {
			return false;
		}
	}
	return true;
};

export type TrackedPrMapCoordinator = {
	applyWorkspace: (workspace: Workspace | null, currentMap: TrackedPrMap) => TrackedPrMap;
	markResolved: (
		repoId: string,
		pr: PullRequestCreated | null,
		previousPr?: PullRequestCreated | null,
	) => void;
};

const trackedPrIdentityKey = (pr: PullRequestCreated): string =>
	`${pr.number}:${pr.url}:${pr.baseRepo}:${pr.baseBranch}:${pr.headRepo}:${pr.headBranch}`;

export const createTrackedPrMapCoordinator = (): TrackedPrMapCoordinator => {
	const suppressedRepoPrKeys = new Map<string, string | null>();

	const applyWorkspace = (workspace: Workspace | null, currentMap: TrackedPrMap): TrackedPrMap => {
		if (!workspace) {
			suppressedRepoPrKeys.clear();
			return new Map<string, PullRequestCreated>();
		}

		for (const [repoId, suppressedKey] of Array.from(suppressedRepoPrKeys.entries())) {
			const repo = workspace.repos.find((candidate) => candidate.id === repoId);
			if (!repo?.trackedPullRequest) {
				suppressedRepoPrKeys.delete(repoId);
				continue;
			}
			if (isOpenPullRequest(repo.trackedPullRequest)) {
				const reportedKey = trackedPrIdentityKey(repo.trackedPullRequest);
				if (suppressedKey === null || suppressedKey !== reportedKey) {
					suppressedRepoPrKeys.delete(repoId);
				}
			}
		}

		const nextMap = mergeTrackedPrMap(workspace, currentMap, new Set(suppressedRepoPrKeys.keys()));
		return trackedPrMapsEqual(currentMap, nextMap) ? currentMap : nextMap;
	};

	const markResolved = (
		repoId: string,
		pr: PullRequestCreated | null,
		previousPr: PullRequestCreated | null = null,
	): void => {
		if (pr) {
			suppressedRepoPrKeys.delete(repoId);
			return;
		}
		const previousKey =
			previousPr && isOpenPullRequest(previousPr) ? trackedPrIdentityKey(previousPr) : null;
		if (previousKey !== null) {
			suppressedRepoPrKeys.set(repoId, previousKey);
			return;
		}
		if (!suppressedRepoPrKeys.has(repoId)) {
			suppressedRepoPrKeys.set(repoId, null);
		}
	};

	return { applyWorkspace, markResolved };
};

export const withTrackedPr = (
	currentMap: TrackedPrMap,
	repoId: string,
	pr: PullRequestCreated | null,
): TrackedPrMap => {
	const nextMap = new Map(currentMap);
	if (pr) {
		nextMap.set(repoId, pr);
	} else {
		nextMap.delete(repoId);
	}
	return nextMap;
};

export const readSidebarCollapsed = (): boolean => {
	try {
		return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === 'true';
	} catch {
		return false;
	}
};

export const persistSidebarCollapsed = (collapsed: boolean): void => {
	try {
		localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(collapsed));
	} catch {
		// ignore storage failures
	}
};

export const buildCheckStats = (
	checks: PullRequestCheck[],
): { passed: number; failed: number; pending: number; total: number } => {
	let passed = 0;
	let failed = 0;
	let pending = 0;
	for (const check of checks) {
		if (check.conclusion === 'success') passed++;
		else if (check.conclusion === 'failure') failed++;
		else pending++;
	}
	return { passed, failed, pending, total: checks.length };
};

export const buildReviewThreads = (
	reviews: PullRequestReviewComment[],
): PullRequestReviewThread[] => {
	const threadMap = new Map<string, PullRequestReviewComment[]>();
	for (const comment of reviews) {
		const key = comment.threadId ?? `single-${comment.id}`;
		const comments = threadMap.get(key) ?? [];
		comments.push(comment);
		threadMap.set(key, comments);
	}
	return Array.from(threadMap.entries())
		.map(([id, comments]) => ({
			id,
			comments: comments.sort((a, b) => (a.createdAt ?? '').localeCompare(b.createdAt ?? '')),
			path: comments[0]?.path ?? '',
			line: comments[0]?.line,
			resolved: comments[0]?.resolved ?? false,
			outdated: comments[0]?.outdated ?? false,
		}))
		.sort((a, b) => {
			if (a.resolved !== b.resolved) return a.resolved ? 1 : -1;
			return a.path.localeCompare(b.path);
		});
};

export const commitPushStageLabel = (stage: GitHubOperationStage | null): string | null => {
	const labels: Record<string, string> = {
		queued: 'Queuing...',
		generating_message: 'Generating commit message...',
		staging: 'Staging files...',
		committing: 'Committing...',
		pushing: 'Pushing...',
	};
	return stage ? (labels[stage] ?? 'Processing...') : null;
};

export const mapPrStatusEventToTrackedPr = (
	payload: RepoDiffPrStatusEvent,
): PullRequestCreated => ({
	repo: payload.status.pullRequest.repo,
	number: payload.status.pullRequest.number,
	url: payload.status.pullRequest.url,
	title: payload.status.pullRequest.title,
	state: payload.status.pullRequest.state,
	draft: payload.status.pullRequest.draft,
	baseRepo: payload.status.pullRequest.base_repo,
	baseBranch: payload.status.pullRequest.base_branch,
	headRepo: payload.status.pullRequest.head_repo,
	headBranch: payload.status.pullRequest.head_branch,
});

export const mapPrStatusEventToStatus = (
	payload: RepoDiffPrStatusEvent,
): PullRequestStatusResult => ({
	pullRequest: {
		repo: payload.status.pullRequest.repo,
		number: payload.status.pullRequest.number,
		url: payload.status.pullRequest.url,
		title: payload.status.pullRequest.title,
		state: payload.status.pullRequest.state,
		draft: payload.status.pullRequest.draft,
		baseRepo: payload.status.pullRequest.base_repo,
		baseBranch: payload.status.pullRequest.base_branch,
		headRepo: payload.status.pullRequest.head_repo,
		headBranch: payload.status.pullRequest.head_branch,
		mergeable: payload.status.pullRequest.mergeable,
	},
	checks: (payload.status.checks ?? []).map((check) => ({
		name: check.name,
		status: check.status,
		conclusion: check.conclusion,
		detailsUrl: check.details_url,
		startedAt: check.started_at,
		completedAt: check.completed_at,
		checkRunId: check.check_run_id,
	})),
});

export const applyPrStatusEvent = (
	payload: RepoDiffPrStatusEvent,
	repoId: string,
	trackedPrMap: Map<string, PullRequestCreated>,
): {
	prStatus: PullRequestStatusResult;
	trackedPr: PullRequestCreated | null;
	trackedPrMap: Map<string, PullRequestCreated>;
	shouldReconcileTrackedPr: boolean;
} => {
	const prStatus = mapPrStatusEventToStatus(payload);
	const nextState = payload.status.pullRequest.state.trim().toLowerCase();
	if (nextState === 'open') {
		const trackedPr = mapPrStatusEventToTrackedPr(payload);
		const nextMap = new Map(trackedPrMap);
		nextMap.set(repoId, trackedPr);
		return { prStatus, trackedPr, trackedPrMap: nextMap, shouldReconcileTrackedPr: false };
	}

	const nextMap = new Map(trackedPrMap);
	const shouldReconcileTrackedPr = nextMap.delete(repoId);
	return {
		prStatus,
		trackedPr: null,
		trackedPrMap: nextMap,
		shouldReconcileTrackedPr,
	};
};

export const shouldClearSelectedItem = (
	selectedItemId: string | null,
	viewMode: ViewMode,
	allItems: PrListItemRef[],
	activeItems: PrListItemRef[],
	readyItems: PrListItemRef[],
): boolean => {
	if (!selectedItemId) return false;
	if (!allItems.some((item) => item.id === selectedItemId)) return true;
	const visibleItems = viewMode === 'active' ? activeItems : readyItems;
	return !visibleItems.some((item) => item.id === selectedItemId);
};

export const buildFileDiffCacheKeyForSource = (
	wsId: string,
	repoId: string,
	file: RepoDiffFileSummary,
	source: 'pr' | 'local',
	activePrBranches: PrBranches | null,
): string => {
	if (source === 'local') {
		return buildFileLocalCacheKey(wsId, repoId, file.status ?? '', file.path, file.prevPath ?? '');
	}
	if (activePrBranches) {
		return buildFilePrCacheKey(
			wsId,
			repoId,
			activePrBranches.base,
			activePrBranches.head,
			file.path,
			file.prevPath ?? '',
		);
	}
	return buildFileLocalCacheKey(wsId, repoId, file.status ?? '', file.path, file.prevPath ?? '');
};

export const createTrackedPrStateReconciler = (deps: {
	loadTrackedPr: (wsId: string, repoId: string) => Promise<void>;
	refreshWorkspacesStatus: () => Promise<void>;
	getSelectedRepoId: () => string;
	loadRepoLocalStatus: (wsId: string, repoId: string) => Promise<void>;
	loadDiffSummary: (wsId: string, repoId: string, pr?: PullRequestCreated) => Promise<void>;
	getTrackedPr: (repoId: string) => PullRequestCreated | undefined;
	getActiveWatchKey: () => { wsId: string; repoId: string } | null;
	clearActivePrBranches: () => void;
	stopActiveWatch: () => Promise<void>;
}): ((wsId: string, repoId: string) => Promise<void>) => {
	let inFlight = false;
	return async (wsId: string, repoId: string): Promise<void> => {
		if (inFlight) return;
		inFlight = true;
		try {
			await deps.loadTrackedPr(wsId, repoId);
			await deps.refreshWorkspacesStatus();
			if (deps.getSelectedRepoId() === repoId) {
				await deps.loadRepoLocalStatus(wsId, repoId);
				await deps.loadDiffSummary(wsId, repoId, deps.getTrackedPr(repoId));
				return;
			}
			const activeWatchKey = deps.getActiveWatchKey();
			if (activeWatchKey?.wsId === wsId && activeWatchKey.repoId === repoId) {
				deps.clearActivePrBranches();
				await deps.stopActiveWatch();
			}
		} finally {
			inFlight = false;
		}
	};
};

export const getCheckIcon = (check: PullRequestCheck): string => {
	if (check.conclusion === 'success') return 'success';
	if (check.conclusion === 'failure') return 'failure';
	if (check.status === 'in_progress' || check.status === 'queued') return 'running';
	return 'pending';
};

export const formatCheckDuration = (check: PullRequestCheck): string => {
	if (!check.startedAt || !check.completedAt) {
		if (check.status === 'in_progress' || check.status === 'queued') return 'Running...';
		return 'Pending';
	}
	const ms = new Date(check.completedAt).getTime() - new Date(check.startedAt).getTime();
	if (ms < 1000) return `${ms}ms`;
	if (ms < 60000) return `${Math.round(ms / 1000)}s`;
	return `${Math.round(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`;
};
