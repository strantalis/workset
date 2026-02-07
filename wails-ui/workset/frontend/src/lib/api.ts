import type {
	RepoAddResponse,
	HooksRunResponse,
	Workspace,
	WorkspaceCreateResponse,
	TerminalLayout,
	TerminalLayoutPayload,
} from './types';
import type { main, worksetapi } from '../../wailsjs/go/models';
import {
	AddRepo,
	ArchiveWorkspace,
	CreateWorkspace,
	GetTerminalBacklog,
	GetTerminalBootstrap,
	GetTerminalSnapshot,
	GetWorkspaceTerminalLayout,
	GetWorkspaceTerminalStatus,
	ListWorkspaceSnapshots,
	LogTerminalDebug,
	PinWorkspace,
	ReorderWorkspaces,
	RemoveRepo,
	RemoveWorkspace,
	RenameWorkspace,
	RunHooks,
	SetWorkspaceColor,
	SetWorkspaceExpanded,
	SetWorkspaceTerminalLayout,
	StopWorkspaceTerminal,
	TrustRepoHooks,
	UnarchiveWorkspace,
	UpdateWorkspaceLastUsed,
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

export type { PullRequestCreated, PullRequestStatusResult } from './types';

export * from './api/github';
export * from './api/repo-diff';
export * from './api/settings';

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
