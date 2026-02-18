import type {
	PullRequestCheck,
	PullRequestCreated,
	PullRequestReviewComment,
	PullRequestStatusResult,
	Workspace,
} from '../../types';

export type ReviewThread = {
	id: string;
	comments: PullRequestReviewComment[];
	path: string;
	line: number | null | undefined;
	resolved: boolean;
	outdated: boolean;
};

export const buildTrackedPrMap = (workspace: Workspace): Map<string, PullRequestCreated> => {
	const nextMap = new Map<string, PullRequestCreated>();
	for (const repo of workspace.repos) {
		if (repo.trackedPullRequest) {
			nextMap.set(repo.id, repo.trackedPullRequest);
		}
	}
	return nextMap;
};

export const buildDiffTargetKey = (
	wsId: string,
	repoId: string,
	pr?: PullRequestCreated,
): string =>
	pr
		? `${wsId}:${repoId}:pr:${pr.number}:${pr.baseRepo}:${pr.baseBranch}:${pr.headRepo}:${pr.headBranch}`
		: `${wsId}:${repoId}:local`;

export const buildCheckStats = (prStatus: PullRequestStatusResult | null) => {
	const checks = prStatus?.checks ?? [];
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

export const buildReviewThreads = (prReviews: PullRequestReviewComment[]): ReviewThread[] => {
	const threadMap = new Map<string, PullRequestReviewComment[]>();
	for (const comment of prReviews) {
		const key = comment.threadId ?? `single-${comment.id}`;
		const arr = threadMap.get(key) ?? [];
		arr.push(comment);
		threadMap.set(key, arr);
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
