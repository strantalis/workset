import type {
	CheckAnnotation,
	CheckAnnotationsResponse,
	GitHubAuthInfo,
	GitHubAuthStatus,
	PullRequestCheck,
	PullRequestCreated,
	PullRequestGenerated,
	PullRequestReviewComment,
	PullRequestStatusResult,
	RemoteInfo,
} from '../types';
import {
	CommitAndPush,
	CreatePullRequest,
	DeleteReviewComment,
	DisconnectGitHub,
	EditReviewComment,
	GeneratePullRequestText,
	GetCheckAnnotations,
	GetCurrentGitHubUser,
	GetGitHubAuthInfo,
	GetGitHubAuthStatus,
	GetGitHubOperationStatus,
	GetPullRequestReviews,
	GetPullRequestStatus,
	GetRepoLocalStatus,
	GetTrackedPullRequest,
	ListRemotes,
	ReplyToReviewComment,
	ResolveReviewThread,
	SendPullRequestReviewsToTerminal,
	SetGitHubAuthMode,
	SetGitHubCLIPath,
	SetGitHubToken,
	StartCommitAndPushAsync,
	StartCreatePullRequestAsync,
} from '../../../wailsjs/go/main/App';

type PullRequestStatusResponse = {
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

type PullRequestCreateResponse = {
	repo: string;
	number: number;
	url: string;
	title: string;
	body?: string;
	draft: boolean;
	state: string;
	base_repo: string;
	base_branch: string;
	head_repo: string;
	head_branch: string;
};

type PullRequestReviewCommentResponse = {
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
};

type GitHubOperationStatusResponse = {
	operationId: string;
	workspaceId: string;
	repoId: string;
	type: GitHubOperationType;
	stage: GitHubOperationStage;
	state: GitHubOperationState;
	startedAt: string;
	finishedAt?: string;
	error?: string;
	pullRequest?: PullRequestCreateResponse;
	commitPush?: CommitAndPushResult;
};

type RemoteInfoResponse = {
	name: string;
	owner: string;
	repo: string;
};

export type RepoLocalStatus = {
	hasUncommitted: boolean;
	ahead: number;
	behind: number;
	currentBranch: string;
};

export type CommitAndPushResult = {
	committed: boolean;
	pushed: boolean;
	message: string;
	sha?: string;
};

export type GitHubOperationType = 'create_pr' | 'commit_push';

export type GitHubOperationStage =
	| 'queued'
	| 'generating'
	| 'creating'
	| 'generating_message'
	| 'staging'
	| 'committing'
	| 'pushing'
	| 'completed'
	| 'failed';

export type GitHubOperationState = 'running' | 'completed' | 'failed';

export type GitHubOperationStatus = {
	operationId: string;
	workspaceId: string;
	repoId: string;
	type: GitHubOperationType;
	stage: GitHubOperationStage;
	state: GitHubOperationState;
	startedAt: string;
	finishedAt?: string;
	error?: string;
	pullRequest?: PullRequestCreated;
	commitPush?: CommitAndPushResult;
};

export type GitHubUser = {
	id: number;
	login: string;
	name?: string;
	email?: string;
};

export async function fetchGitHubAuthStatus(): Promise<GitHubAuthStatus> {
	return (await GetGitHubAuthStatus()) as GitHubAuthStatus;
}

export async function fetchGitHubAuthInfo(): Promise<GitHubAuthInfo> {
	return (await GetGitHubAuthInfo()) as GitHubAuthInfo;
}

export async function setGitHubToken(token: string, source = 'pat'): Promise<GitHubAuthStatus> {
	return (await SetGitHubToken({ token, source })) as GitHubAuthStatus;
}

export async function setGitHubAuthMode(mode: string): Promise<GitHubAuthInfo> {
	return (await SetGitHubAuthMode({ mode })) as GitHubAuthInfo;
}

export async function disconnectGitHub(): Promise<void> {
	await DisconnectGitHub();
}

export async function setGitHubCLIPath(path: string): Promise<GitHubAuthInfo> {
	return (await SetGitHubCLIPath({ path })) as GitHubAuthInfo;
}

export async function createPullRequest(
	workspaceId: string,
	repoId: string,
	payload: {
		title: string;
		body: string;
		base?: string;
		head?: string;
		baseRemote?: string;
		draft: boolean;
		autoCommit?: boolean;
		autoPush?: boolean;
	},
): Promise<PullRequestCreated> {
	const result = (await CreatePullRequest({
		workspaceId,
		repoId,
		title: payload.title,
		body: payload.body,
		base: payload.base ?? '',
		head: payload.head ?? '',
		baseRemote: payload.baseRemote ?? '',
		draft: payload.draft,
		autoCommit: payload.autoCommit ?? false,
		autoPush: payload.autoPush ?? false,
	})) as PullRequestCreateResponse;
	return mapPullRequest(result);
}

export async function startCreatePullRequestAsync(
	workspaceId: string,
	repoId: string,
	payload: {
		base?: string;
		head?: string;
		baseRemote?: string;
		draft: boolean;
	},
): Promise<GitHubOperationStatus> {
	const result = (await StartCreatePullRequestAsync({
		workspaceId,
		repoId,
		base: payload.base ?? '',
		head: payload.head ?? '',
		baseRemote: payload.baseRemote ?? '',
		draft: payload.draft,
	})) as GitHubOperationStatusResponse;
	return mapGitHubOperationStatus(result);
}

export async function startCommitAndPushAsync(
	workspaceId: string,
	repoId: string,
	message?: string,
): Promise<GitHubOperationStatus> {
	const result = (await StartCommitAndPushAsync({
		workspaceId,
		repoId,
		message: message ?? '',
	})) as GitHubOperationStatusResponse;
	return mapGitHubOperationStatus(result);
}

export async function fetchGitHubOperationStatus(
	workspaceId: string,
	repoId: string,
	type: GitHubOperationType,
): Promise<GitHubOperationStatus | null> {
	try {
		const result = (await GetGitHubOperationStatus({
			workspaceId,
			repoId,
			type,
		})) as GitHubOperationStatusResponse;
		return mapGitHubOperationStatus(result);
	} catch (err) {
		if (isOperationStatusNotFound(err)) {
			return null;
		}
		throw err;
	}
}

export async function listRemotes(workspaceId: string, repoId: string): Promise<RemoteInfo[]> {
	const result = (await ListRemotes({
		workspaceId,
		repoId,
	})) as RemoteInfoResponse[];
	return result.map((r) => ({
		name: r.name,
		owner: r.owner,
		repo: r.repo,
	}));
}

export async function fetchTrackedPullRequest(
	workspaceId: string,
	repoId: string,
): Promise<PullRequestCreated | null> {
	const result = (await GetTrackedPullRequest({
		workspaceId,
		repoId,
	})) as unknown as { found: boolean; pull_request?: PullRequestCreateResponse };
	if (!result.found || !result.pull_request) {
		return null;
	}
	return mapPullRequest(result.pull_request);
}

export async function fetchPullRequestStatus(
	workspaceId: string,
	repoId: string,
	number?: number,
	branch?: string,
): Promise<PullRequestStatusResult> {
	const result = (await GetPullRequestStatus({
		workspaceId,
		repoId,
		number: number ?? 0,
		branch: branch ?? '',
	})) as unknown as PullRequestStatusResponse;
	const checks: PullRequestCheck[] = (result.checks ?? []).map((check) => ({
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
			repo: result.pullRequest.repo,
			number: result.pullRequest.number,
			url: result.pullRequest.url,
			title: result.pullRequest.title,
			state: result.pullRequest.state,
			draft: result.pullRequest.draft,
			baseRepo: result.pullRequest.base_repo,
			baseBranch: result.pullRequest.base_branch,
			headRepo: result.pullRequest.head_repo,
			headBranch: result.pullRequest.head_branch,
			mergeable: result.pullRequest.mergeable,
		},
		checks,
	};
}

export async function fetchCheckAnnotations(
	owner: string,
	repo: string,
	checkRunId: number,
): Promise<CheckAnnotation[]> {
	const result = (await GetCheckAnnotations({
		owner,
		repo,
		checkRunId,
	})) as unknown as CheckAnnotationsResponse;
	return (result.annotations ?? []).map((ann) => ({
		path: ann.path,
		startLine: ann.start_line,
		endLine: ann.end_line,
		level: ann.level as 'notice' | 'warning' | 'failure',
		message: ann.message,
		title: ann.title,
	}));
}

export async function fetchPullRequestReviews(
	workspaceId: string,
	repoId: string,
	number?: number,
	branch?: string,
): Promise<PullRequestReviewComment[]> {
	const result = (await GetPullRequestReviews({
		workspaceId,
		repoId,
		number: number ?? 0,
		branch: branch ?? '',
	})) as unknown as { comments: PullRequestReviewCommentResponse[] };
	return (result.comments ?? []).map((comment) => ({
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
}

export async function generatePullRequestText(
	workspaceId: string,
	repoId: string,
): Promise<PullRequestGenerated> {
	const result = (await GeneratePullRequestText({
		workspaceId,
		repoId,
	})) as PullRequestGenerated;
	return result;
}

export async function sendPullRequestReviewsToTerminal(
	workspaceId: string,
	repoId: string,
	number?: number,
	branch?: string,
	terminalId?: string,
): Promise<void> {
	await SendPullRequestReviewsToTerminal({
		workspaceId,
		repoId,
		number: number ?? 0,
		branch: branch ?? '',
		terminalId: terminalId ?? '',
	});
}

export async function fetchRepoLocalStatus(
	workspaceId: string,
	repoId: string,
): Promise<RepoLocalStatus> {
	const result = (await GetRepoLocalStatus({
		workspaceId,
		repoId,
	})) as RepoLocalStatus;
	return result;
}

export async function commitAndPush(
	workspaceId: string,
	repoId: string,
	message?: string,
): Promise<CommitAndPushResult> {
	const result = (await CommitAndPush({
		workspaceId,
		repoId,
		message: message ?? '',
	})) as CommitAndPushResult;
	return result;
}

export async function replyToReviewComment(
	workspaceId: string,
	repoId: string,
	commentId: number,
	body: string,
	number?: number,
	branch?: string,
): Promise<PullRequestReviewComment> {
	const result = (await ReplyToReviewComment({
		workspaceId,
		repoId,
		commentId,
		body,
		number: number ?? 0,
		branch: branch ?? '',
	})) as PullRequestReviewCommentResponse;
	return mapCommentResponse(result);
}

export async function editReviewComment(
	workspaceId: string,
	repoId: string,
	commentId: number,
	body: string,
): Promise<PullRequestReviewComment> {
	const result = (await EditReviewComment({
		workspaceId,
		repoId,
		commentId,
		body,
	})) as PullRequestReviewCommentResponse;
	return mapCommentResponse(result);
}

export async function deleteReviewComment(
	workspaceId: string,
	repoId: string,
	commentId: number,
): Promise<void> {
	await DeleteReviewComment({
		workspaceId,
		repoId,
		commentId,
	});
}

export async function resolveReviewThread(
	workspaceId: string,
	repoId: string,
	threadId: string,
	resolve: boolean,
): Promise<boolean> {
	return (await ResolveReviewThread({
		workspaceId,
		repoId,
		threadId,
		resolve,
	})) as boolean;
}

export async function fetchCurrentGitHubUser(
	workspaceId: string,
	repoId: string,
): Promise<GitHubUser> {
	const result = (await GetCurrentGitHubUser({
		workspaceId,
		repoId,
	})) as GitHubUser;
	return result;
}

function mapPullRequest(result: PullRequestCreateResponse): PullRequestCreated {
	return {
		repo: result.repo,
		number: result.number,
		url: result.url,
		title: result.title,
		body: result.body,
		draft: result.draft,
		state: result.state,
		baseRepo: result.base_repo,
		baseBranch: result.base_branch,
		headRepo: result.head_repo,
		headBranch: result.head_branch,
	};
}

function mapGitHubOperationStatus(result: GitHubOperationStatusResponse): GitHubOperationStatus {
	return {
		operationId: result.operationId,
		workspaceId: result.workspaceId,
		repoId: result.repoId,
		type: result.type,
		stage: result.stage,
		state: result.state,
		startedAt: result.startedAt,
		finishedAt: result.finishedAt,
		error: result.error,
		pullRequest: result.pullRequest ? mapPullRequest(result.pullRequest) : undefined,
		commitPush: result.commitPush,
	};
}

function isOperationStatusNotFound(err: unknown): boolean {
	if (err instanceof Error) {
		return err.message.includes('operation status not found');
	}
	if (typeof err === 'string') {
		return err.includes('operation status not found');
	}
	return false;
}

function mapCommentResponse(comment: PullRequestReviewCommentResponse): PullRequestReviewComment {
	return {
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
	};
}
