import type {
	HooksRunResponse,
	RepoAddResponse,
	Workspace,
	WorkspaceCreateResponse,
} from '../types';
import type { worksetapi } from '../../../wailsjs/go/models';
import {
	AddRepo,
	ArchiveWorkspace,
	CreateWorkspace,
	ListWorkspaceSnapshots,
	PinWorkspace,
	ReorderWorkspaces,
	RemoveRepo,
	RemoveWorkspace,
	RenameWorkspace,
	RunHooks,
	SetWorkspaceColor,
	SetWorkspaceExpanded,
	TrustRepoHooks,
	UnarchiveWorkspace,
	UpdateWorkspaceLastUsed,
} from '../../../wailsjs/go/main/App';

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

function mapWorkspaceRefToWorkspace(ref: worksetapi.WorkspaceRefJSON): Workspace {
	return {
		id: ref.name,
		name: ref.name,
		path: ref.path,
		archived: ref.archived,
		archivedAt: ref.archived_at,
		archivedReason: ref.archived_reason,
		repos: [],
		pinned: ref.pinned,
		pinOrder: ref.pin_order,
		color: ref.color,
		expanded: ref.expanded,
		lastUsed: ref.last_used ?? ref.created_at ?? new Date().toISOString(),
	};
}
