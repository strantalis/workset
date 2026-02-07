import type { RepoLocalStatus } from '../../api';
import type { PullRequestReviewComment, PullRequestStatusResult } from '../../types';

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

export type RepoDiffPrReviewsEvent = {
	workspaceId: string;
	repoId: string;
	comments: Array<{
		id: number;
		node_id?: string;
		thread_id?: string;
		review_id?: number;
		author?: string;
		author_id?: number;
		body: string;
		path: string;
		line?: number;
		side?: string;
		commit_id?: string;
		original_commit_id?: string;
		original_line?: number;
		original_start_line?: number;
		outdated: boolean;
		url?: string;
		created_at?: string;
		updated_at?: string;
		in_reply_to?: number;
		reply?: boolean;
		resolved?: boolean;
	}>;
};

type PrStatusControllerOptions = {
	workspaceId: () => string;
	repoId: () => string;
	prNumberInput: () => string;
	prBranchInput: () => string;
	effectiveMode: () => 'create' | 'status';
	currentUserId: () => number | null;
	setCurrentUserId: (value: number | null) => void;
	setPrStatus: (value: PullRequestStatusResult | null) => void;
	setPrStatusLoading: (value: boolean) => void;
	setPrStatusError: (value: string | null) => void;
	setPrReviews: (value: PullRequestReviewComment[]) => void;
	setPrReviewsLoading: (value: boolean) => void;
	setPrReviewsSent: (value: boolean) => void;
	setLocalStatus: (value: RepoLocalStatus | null) => void;
	parseNumber: (value: string) => number | undefined;
	runGitHubAction: (
		action: () => Promise<void>,
		onError: (message: string) => void,
		fallback: string,
	) => Promise<void>;
	loadSummary: () => Promise<void>;
	loadLocalSummary: () => Promise<void>;
	fetchPullRequestStatus: (
		workspaceId: string,
		repoId: string,
		prNumber?: number,
		prBranch?: string,
	) => Promise<PullRequestStatusResult>;
	fetchPullRequestReviews: (
		workspaceId: string,
		repoId: string,
		prNumber?: number,
		prBranch?: string,
	) => Promise<PullRequestReviewComment[]>;
	fetchCurrentGitHubUser: (workspaceId: string, repoId: string) => Promise<{ id: number }>;
	fetchRepoLocalStatus: (workspaceId: string, repoId: string) => Promise<RepoLocalStatus | null>;
	applyRepoLocalStatus: (workspaceId: string, repoId: string, status: RepoLocalStatus) => void;
};

export const mapPullRequestStatus = (
	status: RepoDiffPrStatusEvent['status'],
): PullRequestStatusResult => {
	const checks = (status.checks ?? []).map((check) => ({
		name: check.name,
		status: check.status,
		conclusion: check.conclusion,
		detailsUrl: check.details_url,
		startedAt: check.started_at,
		completedAt: check.completed_at,
		checkRunId: check.check_run_id,
	}));
	return {
		pullRequest: {
			repo: status.pullRequest.repo,
			number: status.pullRequest.number,
			url: status.pullRequest.url,
			title: status.pullRequest.title,
			state: status.pullRequest.state,
			draft: status.pullRequest.draft,
			baseRepo: status.pullRequest.base_repo,
			baseBranch: status.pullRequest.base_branch,
			headRepo: status.pullRequest.head_repo,
			headBranch: status.pullRequest.head_branch,
			mergeable: status.pullRequest.mergeable,
		},
		checks,
	};
};

export const mapPullRequestReviews = (
	comments: RepoDiffPrReviewsEvent['comments'],
): PullRequestReviewComment[] =>
	comments.map((comment) => ({
		id: comment.id,
		nodeId: comment.node_id,
		threadId: comment.thread_id,
		reviewId: comment.review_id,
		author: comment.author,
		authorId: comment.author_id,
		body: comment.body,
		path: comment.path,
		line: comment.line,
		side: comment.side,
		commitId: comment.commit_id,
		originalCommit: comment.original_commit_id,
		originalLine: comment.original_line,
		originalStart: comment.original_start_line,
		outdated: comment.outdated,
		url: comment.url,
		createdAt: comment.created_at,
		updatedAt: comment.updated_at,
		inReplyTo: comment.in_reply_to,
		reply: comment.reply,
		resolved: comment.resolved,
	}));

export const createPrStatusController = (options: PrStatusControllerOptions) => {
	const loadCurrentUser = async (): Promise<void> => {
		const currentRepoId = options.repoId();
		if (!currentRepoId) return;
		try {
			const user = await options.fetchCurrentGitHubUser(options.workspaceId(), currentRepoId);
			options.setCurrentUserId(user.id);
		} catch {
			options.setCurrentUserId(null);
		}
	};

	const loadPrStatus = async (): Promise<void> => {
		const currentRepoId = options.repoId();
		if (!currentRepoId) return;
		options.setPrStatusLoading(true);
		options.setPrStatusError(null);
		try {
			await options.runGitHubAction(
				async () => {
					options.setPrStatus(
						await options.fetchPullRequestStatus(
							options.workspaceId(),
							currentRepoId,
							options.parseNumber(options.prNumberInput()),
							options.prBranchInput().trim() || undefined,
						),
					);
				},
				(message) => {
					options.setPrStatusError(message);
					options.setPrStatus(null);
				},
				'Failed to load pull request status.',
			);
		} finally {
			options.setPrStatusLoading(false);
		}
	};

	const loadPrReviews = async (): Promise<void> => {
		const currentRepoId = options.repoId();
		if (!currentRepoId) return;
		options.setPrReviewsLoading(true);
		options.setPrReviewsSent(false);
		try {
			await options.runGitHubAction(
				async () => {
					options.setPrReviews(
						await options.fetchPullRequestReviews(
							options.workspaceId(),
							currentRepoId,
							options.parseNumber(options.prNumberInput()),
							options.prBranchInput().trim() || undefined,
						),
					);
					if (options.currentUserId() === null) {
						void loadCurrentUser();
					}
				},
				() => {
					options.setPrReviews([]);
				},
				'Failed to load review comments.',
			);
		} finally {
			options.setPrReviewsLoading(false);
		}
	};

	const loadLocalStatus = async (): Promise<void> => {
		const currentRepoId = options.repoId();
		if (!currentRepoId) return;
		try {
			const status = await options.fetchRepoLocalStatus(options.workspaceId(), currentRepoId);
			options.setLocalStatus(status);
			if (status) {
				options.applyRepoLocalStatus(options.workspaceId(), currentRepoId, status);
			}
		} catch {
			options.setLocalStatus(null);
		}
	};

	const handleRefresh = async (): Promise<void> => {
		await options.loadSummary();
		if (options.effectiveMode() === 'status') {
			await loadPrStatus();
			await loadPrReviews();
			await loadLocalStatus();
			await options.loadLocalSummary();
		}
	};

	return {
		loadCurrentUser,
		loadPrStatus,
		loadPrReviews,
		loadLocalStatus,
		handleRefresh,
	};
};
