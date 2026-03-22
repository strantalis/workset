import type { GitHubOperationStage } from './api/github';
import type {
	PullRequestCheck,
	PullRequestReviewComment,
	PullRequestStatusResult,
	RepoDiffFileSummary,
} from './types';

export type PullRequestReviewThread = {
	id: string;
	comments: PullRequestReviewComment[];
	path: string;
	line: number | null | undefined;
	resolved: boolean;
	outdated: boolean;
};

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

export const buildReviewThreadCountsByFile = (
	reviews: PullRequestReviewComment[],
	files: RepoDiffFileSummary[],
): Map<string, number> => {
	const counts = new Map<string, number>();
	if (reviews.length === 0 || files.length === 0) return counts;

	const fileByPath = new Map<string, RepoDiffFileSummary>();
	const fileByPrevPath = new Map<string, RepoDiffFileSummary>();
	for (const file of files) {
		fileByPath.set(file.path, file);
		if (file.prevPath) {
			fileByPrevPath.set(file.prevPath, file);
		}
	}

	for (const thread of buildReviewThreads(reviews)) {
		if (thread.resolved) continue;
		const matchedFile = fileByPath.get(thread.path) ?? fileByPrevPath.get(thread.path);
		if (!matchedFile) continue;
		counts.set(matchedFile.path, (counts.get(matchedFile.path) ?? 0) + 1);
	}

	return counts;
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
