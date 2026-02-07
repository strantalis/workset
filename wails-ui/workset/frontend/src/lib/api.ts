import type {
	Alias,
	Group,
	GroupSummary,
	RepoAddResponse,
	AgentCLIStatus,
	EnvSnapshotResult,
	HooksRunResponse,
	SettingsSnapshot,
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
	CreateAlias,
	CreateGroup,
	CreateWorkspace,
	DeleteAlias,
	DeleteGroup,
	GetGroup,
	GetSettings,
	GetSessiondStatus,
	GetTerminalBacklog,
	GetTerminalBootstrap,
	GetTerminalSnapshot,
	GetWorkspaceTerminalLayout,
	GetWorkspaceTerminalStatus,
	ListAliases,
	ListGroups,
	ListWorkspaceSnapshots,
	LogTerminalDebug,
	OpenDirectoryDialog,
	OpenFileDialog,
	PinWorkspace,
	ReorderWorkspaces,
	ReloadLoginEnv,
	RemoveGroupMember,
	RemoveRepo,
	RemoveWorkspace,
	RenameWorkspace,
	RestartSessiond,
	RestartSessiondWithReason,
	RunHooks,
	SetAgentCLIPath,
	SetDefaultSetting,
	SetWorkspaceColor,
	SetWorkspaceExpanded,
	SetWorkspaceTerminalLayout,
	StopWorkspaceTerminal,
	TrustRepoHooks,
	UnarchiveWorkspace,
	UpdateAlias,
	UpdateGroup,
	UpdateWorkspaceLastUsed,
	CheckAgentStatus,
	CreateWorkspaceTerminal,
	DeleteSkill as WailsDeleteSkill,
	GetSkill as WailsGetSkill,
	ListSkills as WailsListSkills,
	SaveSkill as WailsSaveSkill,
	SyncSkill as WailsSyncSkill,
} from '../../wailsjs/go/main/App';

export {
	checkForUpdates,
	fetchAppVersion,
	fetchUpdatePreferences,
	fetchUpdateState,
	setUpdatePreferences,
	startAppUpdate,
} from './api/updates';

export * from './api/github';
export * from './api/repo-diff';

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

export async function reloadLoginEnv(): Promise<EnvSnapshotResult> {
	return (await ReloadLoginEnv()) as EnvSnapshotResult;
}

export async function checkAgentStatus(agent: string): Promise<AgentCLIStatus> {
	return (await CheckAgentStatus({ agent })) as AgentCLIStatus;
}

export async function setAgentCLIPath(agent: string, path: string): Promise<AgentCLIStatus> {
	return (await SetAgentCLIPath({ agent, path })) as AgentCLIStatus;
}

export async function openFileDialog(title: string, defaultDirectory: string): Promise<string> {
	return (await OpenFileDialog(title, defaultDirectory)) as string;
}

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

export async function runRepoHooks(
	workspaceId: string,
	repo: string,
	event: string,
	reason = '',
): Promise<HooksRunResponse> {
	return RunHooks({ workspaceId, repo, event, reason });
}

export async function trustRepoHooks(repo: string): Promise<void> {
	await TrustRepoHooks(repo);
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
