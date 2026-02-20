import type { PullRequestCreated } from '../../types';

export type PullRequestStatusResponse = {
	pullRequest: {
		repo: string;
		number: number;
		url: string;
		title: string;
		state: string;
		draft: boolean;
		merged?: boolean;
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

export type PullRequestCreateResponse = {
	repo: string;
	number: number;
	url: string;
	title: string;
	body?: string;
	draft: boolean;
	state: string;
	merged?: boolean;
	base_repo: string;
	base_branch: string;
	head_repo: string;
	head_branch: string;
};

export type TrackedPullRequestResponse = {
	found: boolean;
	pull_request?: PullRequestCreateResponse;
};

export type PullRequestReviewCommentResponse = {
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

export type PullRequestReviewsResponse = {
	comments?: PullRequestReviewCommentResponse[];
};

export type GitHubOperationStatusResponse = {
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

export type RemoteInfoResponse = {
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
