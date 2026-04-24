export type DiffFile = {
	path: string;
	added: number;
	removed: number;
	hunks: string[];
};

export type RepoFileSearchResult = {
	workspaceId: string;
	repoId: string;
	repoName: string;
	path: string;
	isMarkdown: boolean;
	sizeBytes: number;
	score: number;
};

export type RepoFileContent = {
	workspaceId: string;
	repoId: string;
	repoName: string;
	path: string;
	content: string;
	isMarkdown: boolean;
	isBinary: boolean;
	isTruncated: boolean;
	sizeBytes: number;
};

export type RepoFileHoverRange = {
	startLine: number;
	startCharacter: number;
	endLine: number;
	endCharacter: number;
};

export type RepoFileHoverResult = {
	supported: boolean;
	available: boolean;
	found: boolean;
	language?: string;
	provider?: string;
	header?: string;
	documentation?: string;
	documentationKind?: string;
	source?: string;
	range?: RepoFileHoverRange | null;
	unavailableReason?: string;
	installHint?: string;
};

export type RepoFileDefinitionTarget = {
	repoId: string;
	path: string;
	line: number;
	character: number;
	endLine: number;
	endCharacter: number;
};

export type RepoFileDefinitionResult = {
	supported: boolean;
	available: boolean;
	found: boolean;
	language?: string;
	provider?: string;
	targets?: RepoFileDefinitionTarget[];
	unavailableReason?: string;
	installHint?: string;
};

export type RepoImageContent = {
	base64: string;
	mimeType: string;
	error?: string;
};

export type WorkspaceExtraRoot = {
	id: string;
	label: string;
	relativePath: string;
	gitDetected: boolean;
};

export type Repo = {
	id: string;
	name: string;
	path: string;
	remote?: string;
	defaultBranch?: string;
	currentBranch?: string;
	ahead?: number;
	behind?: number;
	dirty: boolean;
	missing: boolean;
	statusKnown?: boolean;
	trackedPullRequest?: PullRequestSummary;
	diff: {
		added: number;
		removed: number;
	};
	files: DiffFile[];
};

export type Thread = {
	id: string;
	name: string;
	path: string;
	workset?: string;
	worksetKey?: string;
	worksetLabel?: string;
	placeholder?: boolean;
	archived: boolean;
	archivedAt?: string;
	archivedReason?: string;
	repos: Repo[];
	pinned: boolean;
	pinOrder: number;
	color?: string;
	description?: string;
	expanded: boolean;
	lastUsed: string;
};

export type Workspace = Thread;

export type TerminalSplitDirection = 'horizontal' | 'vertical';

export type TerminalLayoutLeaf = {
	kind: 'pane';
	id: string;
	terminalId: string;
	snapshot?: TerminalSnapshotLike;
};

export type TerminalLayoutSplit = {
	kind: 'split';
	id: string;
	direction: TerminalSplitDirection;
	ratio: number;
	first: TerminalLayoutNode;
	second: TerminalLayoutNode;
};

export type TerminalLayoutNode = TerminalLayoutLeaf | TerminalLayoutSplit;

export type TerminalLayoutTab = {
	id: string;
	title: string;
	root: TerminalLayoutNode;
	focusedPaneId?: string;
};

export type TerminalLayout = {
	version: number;
	tabs: TerminalLayoutTab[];
	activeTabId: string;
};

export type TerminalLayoutPayload = {
	workspaceId: string;
	workspacePath: string;
	layout?: TerminalLayout;
};

export type ThreadCreateResponse = {
	thread: {
		name: string;
		path: string;
		workset: string;
		branch: string;
		next: string;
	};
	warnings?: string[];
	pendingHooks?: {
		event: string;
		repo: string;
		hooks: string[];
		status?: string;
		reason?: string;
	}[];
	hookRuns?: HookExecution[];
	workspace?: {
		name: string;
		path: string;
		workset: string;
		branch: string;
		next: string;
	};
};

export type WorkspaceCreateResponse = ThreadCreateResponse;

export type RepoAddResponse = {
	payload: {
		status: string;
		workspace: string;
		repo: string;
		local_path: string;
		managed: boolean;
		pending_hooks?: {
			event: string;
			repo: string;
			hooks: string[];
			status?: string;
			reason?: string;
		}[];
	};
	warnings?: string[];
	pendingHooks?: {
		event: string;
		repo: string;
		hooks: string[];
		status?: string;
		reason?: string;
	}[];
	hookRuns?: HookExecution[];
};

export type WorksetRepoAddResponse = {
	payload: {
		status: string;
		workset: string;
		added?: string[];
	};
	warnings?: string[];
};

export type HookExecution = {
	event: string;
	repo: string;
	id: string;
	status: string;
	log_path?: string;
};

export type HooksRunResponse = {
	event: string;
	repo: string;
	results: {
		id: string;
		status: string;
		log_path?: string;
	}[];
};

export type HookProgressEvent = {
	operation?: string;
	reason?: string;
	workspace?: string;
	repo: string;
	event: string;
	hookId: string;
	phase: 'started' | 'finished' | 'clone-started' | 'clone-finished';
	status?: string;
	logPath?: string;
	error?: string;
};

export type RegisteredRepo = {
	name: string;
	url?: string;
	path?: string;
	remote?: string;
	default_branch?: string;
};

/** @deprecated Use RegisteredRepo instead */
export type Alias = RegisteredRepo;

export type SettingsDefaults = {
	remote: string;
	baseBranch: string;
	thread: string;
	worksetRoot: string;
	repoStoreRoot: string;
	agent: string;
	agentModel: string;
	terminalIdleTimeout: string;
	terminalDebugLog: string;
	terminalProtocolLog: string;
	terminalDebugOverlay: string;
	terminalFontSize: string;
	terminalCursorBlink: string;
	terminalKeybindings?: Record<string, string[]>;
};

export type SettingsDefaultField = Exclude<keyof SettingsDefaults, 'terminalKeybindings'>;

export type SettingsSnapshot = {
	defaults: SettingsDefaults;
	configPath: string;
};

export type CheckAnnotationsResponse = {
	annotations: {
		path: string;
		start_line: number;
		end_line: number;
		level: string;
		message: string;
		title?: string;
	}[];
};

export type AgentCLIStatus = {
	installed: boolean;
	path?: string;
	configuredPath?: string;
	command: string;
	error?: string;
};

export type EnvSnapshotResult = {
	updated: boolean;
	appliedKeys?: string[];
};

export type AppVersion = {
	version: string;
	commit: string;
	dirty: boolean;
};

export type UpdateChannel = 'stable' | 'alpha';

export type UpdatePreferences = {
	channel: UpdateChannel;
	autoCheck: boolean;
	dismissedVersion: string;
};

export type UpdateReleaseAsset = {
	name: string;
	url: string;
	sha256: string;
};

export type UpdateReleaseSigning = {
	teamId: string;
};

export type UpdateRelease = {
	version: string;
	pubDate: string;
	notesUrl: string;
	minimumVersion: string;
	asset: UpdateReleaseAsset;
	signing: UpdateReleaseSigning;
};

export type UpdateCheckResult = {
	status: 'up_to_date' | 'update_available' | 'unavailable';
	channel: UpdateChannel;
	currentVersion: string;
	latestVersion: string;
	message: string;
	release?: UpdateRelease;
};

export type UpdateState = {
	phase: 'idle' | 'checking' | 'downloading' | 'validating' | 'applying' | 'failed';
	channel: string;
	currentVersion: string;
	latestVersion: string;
	message: string;
	error: string;
	checkedAt: string;
};

export type UpdateStartResult = {
	started: boolean;
	state: UpdateState;
};

export type RepoDiffFileSummary = {
	path: string;
	prevPath?: string;
	added: number;
	removed: number;
	status: string;
	binary?: boolean;
};

export type RepoDiffSummary = {
	files: RepoDiffFileSummary[];
	totalAdded: number;
	totalRemoved: number;
};

export type RepoFileDiff = {
	patch: string;
	truncated: boolean;
	totalBytes: number;
	totalLines: number;
	binary?: boolean;
};

export type PullRequestSummary = {
	repo: string;
	number: number;
	url: string;
	title: string;
	body?: string;
	state: string;
	draft: boolean;
	merged?: boolean;
	baseRepo: string;
	baseBranch: string;
	headRepo: string;
	headBranch: string;
	updatedAt?: string;
	mergeable?: string;
	author?: string;
	commentsCount?: number;
	reviewCommentsCount?: number;
};

export type CheckAnnotation = {
	path: string;
	startLine: number;
	endLine: number;
	level: 'notice' | 'warning' | 'failure';
	message: string;
	title?: string;
};

export type PullRequestCheck = {
	name: string;
	status: string;
	conclusion?: string;
	detailsUrl?: string;
	startedAt?: string;
	completedAt?: string;
	checkRunId?: number;
};

export type PullRequestStatusResult = {
	pullRequest: PullRequestSummary;
	checks: PullRequestCheck[];
};

export type PullRequestCreated = PullRequestSummary;

export type PullRequestReviewComment = {
	id: number;
	nodeId?: string;
	threadId?: string;
	reviewId?: number;
	author?: string;
	authorId?: number;
	body: string;
	path: string;
	line?: number;
	side?: string;
	commitId?: string;
	originalCommit?: string;
	originalLine?: number;
	originalStart?: number;
	outdated: boolean;
	url?: string;
	createdAt?: string;
	updatedAt?: string;
	inReplyTo?: number;
	reply?: boolean;
	resolved?: boolean;
};

export type PullRequestGenerated = {
	title: string;
	body: string;
};

export type GitHubAuthStatus = {
	authenticated: boolean;
	login?: string;
	name?: string;
	scopes?: string[];
	tokenSource?: string;
};

export type GitHubCLIStatus = {
	installed: boolean;
	version?: string;
	path?: string;
	configuredPath?: string;
	error?: string;
};

export type GitHubAuthInfo = {
	mode: string;
	status: GitHubAuthStatus;
	cli: GitHubCLIStatus;
};

export type GitHubRepoSearchItem = {
	name: string;
	fullName: string;
	owner: string;
	defaultBranch: string;
	cloneUrl: string;
	sshUrl: string;
	private: boolean;
	archived: boolean;
	host: string;
};

export type RemoteInfo = {
	name: string;
	owner: string;
	repo: string;
};
import type { TerminalSnapshotLike } from './terminal/terminalEmulatorContracts';
