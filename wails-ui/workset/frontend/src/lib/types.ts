export type DiffFile = {
	path: string;
	added: number;
	removed: number;
	hunks: string[];
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

export type Workspace = {
	id: string;
	name: string;
	path: string;
	template?: string;
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

export type TerminalLayoutTab = {
	id: string;
	terminalId: string;
	title: string;
};

export type TerminalLayoutNode = {
	id: string;
	kind: 'pane' | 'split';
	tabs?: TerminalLayoutTab[];
	activeTabId?: string;
	direction?: 'row' | 'column';
	ratio?: number;
	first?: TerminalLayoutNode;
	second?: TerminalLayoutNode;
};

export type TerminalLayout = {
	version: number;
	root: TerminalLayoutNode;
	focusedPaneId?: string;
};

export type TerminalLayoutPayload = {
	workspaceId: string;
	workspacePath: string;
	layout?: TerminalLayout;
};

export type WorkspaceCreateResponse = {
	workspace: {
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
};

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
	phase: 'started' | 'finished';
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

export type GroupSummary = {
	name: string;
	description?: string;
	repo_count: number;
};

export type GroupMember = {
	repo: string;
};

export type Group = {
	name: string;
	description?: string;
	members: GroupMember[];
};

export type SettingsDefaults = {
	remote: string;
	baseBranch: string;
	workspace: string;
	workspaceRoot: string;
	repoStoreRoot: string;
	sessionBackend: string;
	sessionNameFormat: string;
	sessionTheme: string;
	sessionTmuxStyle: string;
	sessionTmuxLeft: string;
	sessionTmuxRight: string;
	sessionScreenHard: string;
	agent: string;
	agentModel: string;
	terminalIdleTimeout: string;
	terminalProtocolLog: string;
	terminalDebugOverlay: string;
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
	baseRepo: string;
	baseBranch: string;
	headRepo: string;
	headBranch: string;
	updatedAt?: string;
	mergeable?: string;
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

export type RemoteInfo = {
	name: string;
	owner: string;
	repo: string;
};
