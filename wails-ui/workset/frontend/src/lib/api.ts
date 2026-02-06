import type {
	Alias,
	CheckAnnotation,
	CheckAnnotationsResponse,
	Group,
	GroupSummary,
	GitHubAuthInfo,
	GitHubAuthStatus,
	PullRequestCheck,
	PullRequestCreated,
	PullRequestGenerated,
	PullRequestReviewComment,
	PullRequestStatusResult,
	RemoteInfo,
	RepoAddResponse,
	RepoDiffSummary,
	RepoFileDiff,
	AgentCLIStatus,
	EnvSnapshotResult,
	SettingsSnapshot,
	AppVersion,
	Workspace,
	WorkspaceCreateResponse,
	TerminalLayout,
	TerminalLayoutPayload,
} from './types';
import type { main, worksetapi } from '../../wailsjs/go/models';
import {
	AddGroupMember,
	AddRepo,
	ApplyGroup,
	ArchiveWorkspace,
	CreateWorkspace,
	CreateAlias,
	CreateGroup,
	CommitAndPush,
	DeleteAlias,
	DeleteGroup,
	DeleteReviewComment,
	EditReviewComment,
	GetCurrentGitHubUser,
	GetGroup,
	GetRepoDiff,
	GetRepoDiffSummary,
	GetRepoFileDiff,
	GetRepoLocalStatus,
	GetBranchDiffSummary,
	GetBranchFileDiff,
	GetPullRequestReviews,
	GetPullRequestStatus,
	GetTrackedPullRequest,
	GeneratePullRequestText,
	GetSettings,
	GetSessiondStatus,
	RestartSessiond,
	RestartSessiondWithReason,
	ListAliases,
	ListGroups,
	ListRemotes,
	ListWorkspaceSnapshots,
	OpenDirectoryDialog,
	OpenFileDialog,
	RemoveGroupMember,
	RemoveRepo,
	RemoveWorkspace,
	RenameWorkspace,
	ReplyToReviewComment,
	ResolveReviewThread,
	SendPullRequestReviewsToTerminal,
	UpdateAlias,
	UpdateGroup,
	UnarchiveWorkspace,
	SetDefaultSetting,
	CreatePullRequest,
	GetCheckAnnotations,
	GetTerminalBacklog,
	GetTerminalBootstrap,
	GetTerminalSnapshot,
	LogTerminalDebug,
	GetWorkspaceTerminalStatus,
	CreateWorkspaceTerminal,
	StopWorkspaceTerminal,
	GetWorkspaceTerminalLayout,
	SetWorkspaceTerminalLayout,
	GetAppVersion,
	GetGitHubAuthInfo,
	GetGitHubAuthStatus,
	DisconnectGitHub,
	SetGitHubCLIPath,
	SetGitHubAuthMode,
	SetGitHubToken,
	CheckAgentStatus,
	SetAgentCLIPath,
	ReloadLoginEnv,
	StartRepoDiffWatch,
	UpdateRepoDiffWatch,
	StopRepoDiffWatch,
	PinWorkspace,
	SetWorkspaceColor,
	SetWorkspaceExpanded,
	ReorderWorkspaces,
	UpdateWorkspaceLastUsed,
	ListSkills as WailsListSkills,
	GetSkill as WailsGetSkill,
	SaveSkill as WailsSaveSkill,
	DeleteSkill as WailsDeleteSkill,
	SyncSkill as WailsSyncSkill,
} from '../../wailsjs/go/main/App';

type WorkspaceSnapshot = {
	id: string;
	name: string;
	path: string;
	createdAt?: string;
	lastUsed?: string;
	archived: boolean;
	archivedAt?: string;
	archivedReason?: string;
	repos: RepoSnapshot[];
	pinned?: boolean;
	pinOrder?: number;
	color?: string;
	expanded?: boolean;
};

type RepoSnapshot = {
	id: string;
	name: string;
	path: string;
	remote?: string;
	defaultBranch?: string;
	dirty: boolean;
	missing: boolean;
	statusKnown: boolean;
};

type RepoDiffSnapshot = {
	patch: string;
};

export async function fetchAppVersion(): Promise<AppVersion> {
	return (await GetAppVersion()) as AppVersion;
}

export async function reloadLoginEnv(): Promise<EnvSnapshotResult> {
	return (await ReloadLoginEnv()) as EnvSnapshotResult;
}

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

export async function checkAgentStatus(agent: string): Promise<AgentCLIStatus> {
	return (await CheckAgentStatus({ agent })) as AgentCLIStatus;
}

export async function setAgentCLIPath(agent: string, path: string): Promise<AgentCLIStatus> {
	return (await SetAgentCLIPath({ agent, path })) as AgentCLIStatus;
}

export async function startRepoDiffWatch(
	workspaceId: string,
	repoId: string,
	prNumber?: number,
	prBranch?: string,
): Promise<boolean> {
	return (await StartRepoDiffWatch({
		workspaceId,
		repoId,
		prNumber,
		prBranch,
	})) as boolean;
}

export async function startRepoStatusWatch(workspaceId: string, repoId: string): Promise<boolean> {
	return (await StartRepoDiffWatch({
		workspaceId,
		repoId,
		localOnly: true,
	})) as boolean;
}

export async function updateRepoDiffWatch(
	workspaceId: string,
	repoId: string,
	prNumber?: number,
	prBranch?: string,
): Promise<boolean> {
	return (await UpdateRepoDiffWatch({
		workspaceId,
		repoId,
		prNumber,
		prBranch,
	})) as boolean;
}

export async function stopRepoDiffWatch(workspaceId: string, repoId: string): Promise<boolean> {
	return (await StopRepoDiffWatch({ workspaceId, repoId })) as boolean;
}

export async function stopRepoStatusWatch(workspaceId: string, repoId: string): Promise<boolean> {
	return (await StopRepoDiffWatch({ workspaceId, repoId, localOnly: true })) as boolean;
}

export async function openFileDialog(title: string, defaultDirectory: string): Promise<string> {
	return (await OpenFileDialog(title, defaultDirectory)) as string;
}

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

export type TerminalBacklogResponse = {
	workspaceId: string;
	terminalId: string;
	data: string;
	nextOffset: number;
	truncated: boolean;
	source?: string;
};

export type TerminalSnapshotResponse = {
	workspaceId: string;
	terminalId: string;
	data: string;
	source?: string;
	kitty?: {
		images?: Array<{
			id: string;
			format?: string;
			width?: number;
			height?: number;
			data?: string | number[];
		}>;
		placements?: Array<{
			id: number;
			imageId: string;
			row: number;
			col: number;
			rows: number;
			cols: number;
			x?: number;
			y?: number;
			z?: number;
		}>;
	};
};

export type TerminalBootstrapResponse = {
	workspaceId: string;
	terminalId: string;
	snapshot?: string;
	snapshotSource?: string;
	kitty?: {
		images?: Array<{
			id: string;
			format?: string;
			width?: number;
			height?: number;
			data?: string | number[];
		}>;
		placements?: Array<{
			id: number;
			imageId: string;
			row: number;
			col: number;
			rows: number;
			cols: number;
			x?: number;
			y?: number;
			z?: number;
		}>;
	};
	backlog?: string;
	backlogSource?: string;
	backlogTruncated?: boolean;
	nextOffset?: number;
	source?: string;
	altScreen?: boolean;
	mouse?: boolean;
	mouseSGR?: boolean;
	mouseEncoding?: string;
	safeToReplay?: boolean;
	initialCredit?: number;
};

export type SessiondStatusResponse = {
	available: boolean;
	error?: string;
	warning?: string;
};

export type WorkspaceTerminalStatusResponse = {
	workspaceId: string;
	terminalId?: string;
	active: boolean;
	error?: string;
};

export async function fetchWorkspaces(
	includeArchived = false,
	includeStatus = false,
): Promise<Workspace[]> {
	const snapshots = await ListWorkspaceSnapshots({ includeArchived, includeStatus });
	return snapshots.map((workspace: WorkspaceSnapshot) => ({
		id: workspace.id,
		name: workspace.name,
		path: workspace.path,
		archived: workspace.archived,
		archivedAt: workspace.archivedAt,
		archivedReason: workspace.archivedReason,
		repos: workspace.repos.map((repo: RepoSnapshot) => ({
			id: repo.id,
			name: repo.name,
			path: repo.path,
			remote: repo.remote,
			defaultBranch: repo.defaultBranch,
			ahead: 0,
			behind: 0,
			dirty: repo.dirty,
			missing: repo.missing,
			statusKnown: repo.statusKnown,
			diff: { added: 0, removed: 0 },
			files: [],
		})),
		pinned: workspace.pinned ?? false,
		pinOrder: workspace.pinOrder ?? 0,
		color: workspace.color,
		expanded: workspace.expanded ?? false,
		lastUsed: workspace.lastUsed ?? workspace.createdAt ?? new Date().toISOString(),
	}));
}

export async function createWorkspace(
	name: string,
	path: string,
	aliases?: string[],
	groups?: string[],
): Promise<WorkspaceCreateResponse> {
	return CreateWorkspace({
		name,
		path,
		repos: aliases,
		groups,
	});
}

export async function openDirectoryDialog(
	title: string,
	defaultDirectory: string,
): Promise<string> {
	return OpenDirectoryDialog(title, defaultDirectory);
}

export async function fetchWorkspaceTerminalStatus(
	workspaceId: string,
	terminalId: string,
): Promise<WorkspaceTerminalStatusResponse> {
	return GetWorkspaceTerminalStatus(workspaceId, terminalId);
}

export async function fetchTerminalSnapshot(
	workspaceId: string,
	terminalId: string,
): Promise<TerminalSnapshotResponse> {
	return GetTerminalSnapshot(workspaceId, terminalId);
}

export async function fetchTerminalBootstrap(
	workspaceId: string,
	terminalId: string,
): Promise<TerminalBootstrapResponse> {
	return GetTerminalBootstrap(workspaceId, terminalId);
}

export async function logTerminalDebug(
	workspaceId: string,
	terminalId: string,
	event: string,
	details = '',
): Promise<void> {
	await LogTerminalDebug({ workspaceId, terminalId, event, details });
}

export async function renameWorkspace(workspaceId: string, newName: string): Promise<void> {
	await RenameWorkspace(workspaceId, newName);
}

export async function archiveWorkspace(workspaceId: string, reason: string): Promise<void> {
	await ArchiveWorkspace(workspaceId, reason);
}

export async function unarchiveWorkspace(workspaceId: string): Promise<void> {
	await UnarchiveWorkspace(workspaceId);
}

export type RemoveWorkspaceOptions = {
	deleteFiles?: boolean;
	force?: boolean;
	fetchRemotes?: boolean;
};

export async function removeWorkspace(
	workspaceId: string,
	options: RemoveWorkspaceOptions = {},
): Promise<void> {
	const { deleteFiles = false, force = false, fetchRemotes = deleteFiles } = options;
	await RemoveWorkspace({ workspaceId, deleteFiles, force, fetchRemotes });
}

export async function addRepo(
	workspaceId: string,
	source: string,
	name: string,
	repoDir: string,
): Promise<RepoAddResponse> {
	return AddRepo({ workspaceId, source, name, repoDir });
}

export async function removeRepo(
	workspaceId: string,
	repoName: string,
	deleteWorktree: boolean,
	deleteLocal: boolean,
): Promise<void> {
	await RemoveRepo({ workspaceId, repoName, deleteWorktree, deleteLocal });
}

export async function fetchTerminalBacklog(
	workspaceId: string,
	terminalId: string,
	since: number,
): Promise<TerminalBacklogResponse> {
	return GetTerminalBacklog(workspaceId, terminalId, since);
}

export async function createWorkspaceTerminal(
	workspaceId: string,
): Promise<{ workspaceId: string; terminalId: string }> {
	return CreateWorkspaceTerminal(workspaceId);
}

export async function stopWorkspaceTerminal(
	workspaceId: string,
	terminalId: string,
): Promise<void> {
	await StopWorkspaceTerminal(workspaceId, terminalId);
}

export async function fetchSessiondStatus(): Promise<SessiondStatusResponse> {
	return GetSessiondStatus();
}

export async function restartSessiond(reason?: string): Promise<SessiondStatusResponse> {
	const trimmed = reason?.trim();
	if (trimmed) {
		return RestartSessiondWithReason(trimmed);
	}
	return RestartSessiond();
}

export async function listRegisteredRepos(): Promise<Alias[]> {
	return ListAliases();
}

export async function registerRepo(
	name: string,
	source: string,
	remote: string,
	defaultBranch: string,
): Promise<void> {
	await CreateAlias({ name, source, remote, defaultBranch });
}

export async function updateRegisteredRepo(
	name: string,
	source: string,
	remote: string,
	defaultBranch: string,
): Promise<void> {
	await UpdateAlias({ name, source, remote, defaultBranch });
}

export async function unregisterRepo(name: string): Promise<void> {
	await DeleteAlias(name);
}

/** @deprecated Use listRegisteredRepos instead */
export const listAliases = listRegisteredRepos;

/** @deprecated Use registerRepo instead */
export const createAlias = registerRepo;

/** @deprecated Use updateRegisteredRepo instead */
export const updateAlias = updateRegisteredRepo;

/** @deprecated Use unregisterRepo instead */
export const deleteAlias = unregisterRepo;

export async function listGroups(): Promise<GroupSummary[]> {
	return ListGroups();
}

export async function getGroup(name: string): Promise<Group> {
	return GetGroup(name);
}

export async function createGroup(name: string, description: string): Promise<void> {
	await CreateGroup({ name, description });
}

export async function updateGroup(name: string, description: string): Promise<void> {
	await UpdateGroup({ name, description });
}

export async function deleteGroup(name: string): Promise<void> {
	await DeleteGroup(name);
}

export async function addGroupMember(groupName: string, repoName: string): Promise<void> {
	await AddGroupMember({
		groupName,
		repoName,
	});
}

export async function removeGroupMember(groupName: string, repoName: string): Promise<void> {
	await RemoveGroupMember({
		groupName,
		repoName,
	});
}

export async function applyGroup(workspaceId: string, groupName: string): Promise<void> {
	await ApplyGroup(workspaceId, groupName);
}

export async function fetchSettings(): Promise<SettingsSnapshot> {
	return (await GetSettings()) as unknown as SettingsSnapshot;
}

export async function fetchWorkspaceTerminalLayout(
	workspaceId: string,
): Promise<TerminalLayoutPayload> {
	return (await GetWorkspaceTerminalLayout(workspaceId)) as TerminalLayoutPayload;
}

export async function persistWorkspaceTerminalLayout(
	workspaceId: string,
	layout: TerminalLayout,
): Promise<void> {
	await SetWorkspaceTerminalLayout({
		workspaceId,
		layout: layout as unknown as main.TerminalLayout,
	} as unknown as main.TerminalLayoutRequest);
}

export async function setDefaultSetting(key: string, value: string): Promise<void> {
	await SetDefaultSetting(key, value);
}

export async function fetchRepoDiff(
	workspaceId: string,
	repoId: string,
): Promise<RepoDiffSnapshot> {
	return GetRepoDiff(workspaceId, repoId);
}

export async function fetchRepoDiffSummary(
	workspaceId: string,
	repoId: string,
): Promise<RepoDiffSummary> {
	return GetRepoDiffSummary(workspaceId, repoId);
}

export async function fetchRepoFileDiff(
	workspaceId: string,
	repoId: string,
	path: string,
	prevPath: string,
	status: string,
): Promise<RepoFileDiff> {
	return GetRepoFileDiff(workspaceId, repoId, path, prevPath, status);
}

export async function fetchBranchDiffSummary(
	workspaceId: string,
	repoId: string,
	base: string,
	head: string,
): Promise<RepoDiffSummary> {
	return GetBranchDiffSummary(workspaceId, repoId, base, head);
}

export async function fetchBranchFileDiff(
	workspaceId: string,
	repoId: string,
	base: string,
	head: string,
	path: string,
	prevPath: string,
): Promise<RepoFileDiff> {
	return GetBranchFileDiff(workspaceId, repoId, base, head, path, prevPath);
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

type RemoteInfoResponse = {
	name: string;
	owner: string;
	repo: string;
};

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

export type GitHubUser = {
	id: number;
	login: string;
	name?: string;
	email?: string;
};

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

// Workspace UI management functions
export async function pinWorkspace(workspaceId: string, pin: boolean): Promise<Workspace> {
	const result = await PinWorkspace(workspaceId, pin);
	return mapWorkspaceRefToWorkspace(result);
}

export async function setWorkspaceColor(workspaceId: string, color: string): Promise<Workspace> {
	const result = await SetWorkspaceColor(workspaceId, color);
	return mapWorkspaceRefToWorkspace(result);
}

export async function setWorkspaceExpanded(
	workspaceId: string,
	expanded: boolean,
): Promise<Workspace> {
	const result = await SetWorkspaceExpanded(workspaceId, expanded);
	return mapWorkspaceRefToWorkspace(result);
}

export async function reorderWorkspaces(orders: Record<string, number>): Promise<Workspace[]> {
	const result = await ReorderWorkspaces({ orders });
	return result.map(mapWorkspaceRefToWorkspace);
}

export async function updateWorkspaceLastUsed(workspaceId: string): Promise<void> {
	await UpdateWorkspaceLastUsed(workspaceId);
}

// --- Skills API ---

export type SkillInfo = {
	name: string;
	description: string;
	dirName: string;
	scope: string;
	tools: string[];
	path: string;
};

export type SkillContent = SkillInfo & {
	content: string;
};

export async function listSkills(workspaceId?: string): Promise<SkillInfo[]> {
	return (await WailsListSkills({ workspaceId: workspaceId ?? '' })) as SkillInfo[];
}

export async function getSkill(
	scope: string,
	dirName: string,
	tool: string,
	workspaceId?: string,
): Promise<SkillContent> {
	return (await WailsGetSkill({
		scope,
		dirName,
		tool,
		workspaceId: workspaceId ?? '',
	})) as SkillContent;
}

export async function saveSkill(
	scope: string,
	dirName: string,
	tool: string,
	content: string,
	workspaceId?: string,
): Promise<void> {
	await WailsSaveSkill({ scope, dirName, tool, content, workspaceId: workspaceId ?? '' });
}

export async function deleteSkill(
	scope: string,
	dirName: string,
	tool: string,
	workspaceId?: string,
): Promise<void> {
	await WailsDeleteSkill({ scope, dirName, tool, workspaceId: workspaceId ?? '' });
}

export async function syncSkill(
	scope: string,
	dirName: string,
	fromTool: string,
	toTools: string[],
	workspaceId?: string,
): Promise<void> {
	await WailsSyncSkill({ scope, dirName, fromTool, toTools, workspaceId: workspaceId ?? '' });
}

// Helper to map WorkspaceRefJSON to Workspace type
function mapWorkspaceRefToWorkspace(ref: worksetapi.WorkspaceRefJSON): Workspace {
	return {
		id: ref.name,
		name: ref.name,
		path: ref.path,
		archived: ref.archived,
		archivedAt: ref.archived_at,
		archivedReason: ref.archived_reason,
		repos: [], // Will be populated by fetchWorkspaces
		pinned: ref.pinned,
		pinOrder: ref.pin_order,
		color: ref.color,
		expanded: ref.expanded,
		lastUsed: ref.last_used ?? ref.created_at ?? new Date().toISOString(),
	};
}
