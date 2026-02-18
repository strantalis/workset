import type {
	CheckAnnotation,
	CheckAnnotationsResponse,
	PullRequestCheck,
	PullRequestCreated,
	PullRequestReviewComment,
	PullRequestStatusResult,
	RemoteInfo,
} from '../../types';
import type {
	GitHubOperationStatus,
	GitHubOperationStatusResponse,
	PullRequestCreateResponse,
	PullRequestReviewCommentResponse,
	PullRequestStatusResponse,
	RemoteInfoResponse,
} from './types';

export function mapPullRequest(result: PullRequestCreateResponse): PullRequestCreated {
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

export function mapGitHubOperationStatus(
	result: GitHubOperationStatusResponse,
): GitHubOperationStatus {
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

export function mapRemoteInfo(remote: RemoteInfoResponse): RemoteInfo {
	return {
		name: remote.name,
		owner: remote.owner,
		repo: remote.repo,
	};
}

export function mapPullRequestStatus(result: PullRequestStatusResponse): PullRequestStatusResult {
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

export function mapCheckAnnotations(result: CheckAnnotationsResponse): CheckAnnotation[] {
	return (result.annotations ?? []).map((annotation) => ({
		path: annotation.path,
		startLine: annotation.start_line,
		endLine: annotation.end_line,
		level: annotation.level as 'notice' | 'warning' | 'failure',
		message: annotation.message,
		title: annotation.title,
	}));
}

export function mapCommentResponse(
	comment: PullRequestReviewCommentResponse,
): PullRequestReviewComment {
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

export function mapPullRequestReviewComments(
	comments: PullRequestReviewCommentResponse[],
): PullRequestReviewComment[] {
	return comments.map((comment) => mapCommentResponse(comment));
}

export function isOperationStatusNotFound(err: unknown): boolean {
	if (err instanceof Error) {
		return err.message.includes('operation status not found');
	}
	if (typeof err === 'string') {
		return err.includes('operation status not found');
	}
	return false;
}
