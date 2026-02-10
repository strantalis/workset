import type {
	HooksRunResponse,
	RepoAddResponse,
	Workspace,
	WorkspaceCreateResponse,
} from '../types';
import type { WorkspaceRefJSON } from '../../../bindings/github.com/strantalis/workset/pkg/worksetapi/models';
import {
	AddRepo,
	ArchiveWorkspace,
	CloseWorkspacePopout,
	CreateWorkspace,
	ListWorkspaceSnapshots,
	ListWorkspacePopouts,
	OpenWorkspacePopout,
	PinWorkspace,
	PreviewRepoHooks,
	ReorderWorkspaces,
	RemoveRepo,
	RemoveWorkspace,
	RenameWorkspace,
	RunHooks,
	SetWorkspaceColor,
	SetWorkspaceDescription,
	SetWorkspaceExpanded,
	TrustRepoHooks,
	UnarchiveWorkspace,
	UpdateWorkspaceLastUsed,
} from '../../../bindings/workset/app';

export async function fetchWorkspaces(
	includeArchived = false,
	includeStatus = false,
): Promise<Workspace[]> {
	const snapshots = await ListWorkspaceSnapshots({ includeArchived, includeStatus });

	const mapped: Workspace[] = snapshots.map((workspace) => ({
		id: workspace.id,
		name: workspace.name,
		path: workspace.path,
		archived: workspace.archived,
		archivedAt: workspace.archivedAt,
		archivedReason: workspace.archivedReason,
		repos: workspace.repos.map((repo) => ({
			id: repo.id,
			name: repo.name,
			path: repo.path,
			remote: repo.remote,
			defaultBranch: repo.defaultBranch,
			dirty: repo.dirty,
			missing: repo.missing,
			statusKnown: repo.statusKnown,
			currentBranch: repo.currentBranch,
			ahead: repo.ahead ?? 0,
			behind: repo.behind ?? 0,
			trackedPullRequest: repo.trackedPullRequest
				? {
						repo: repo.trackedPullRequest.repo,
						number: repo.trackedPullRequest.number,
						url: repo.trackedPullRequest.url,
						title: repo.trackedPullRequest.title,
						body: repo.trackedPullRequest.body,
						state: repo.trackedPullRequest.state,
						draft: repo.trackedPullRequest.draft,
						baseRepo: repo.trackedPullRequest.baseRepo,
						baseBranch: repo.trackedPullRequest.baseBranch,
						headRepo: repo.trackedPullRequest.headRepo,
						headBranch: repo.trackedPullRequest.headBranch,
						updatedAt: repo.trackedPullRequest.updatedAt,
					}
				: undefined,
			diff: {
				added: repo.diff?.added ?? 0,
				removed: repo.diff?.removed ?? 0,
			},
			files: (repo.files ?? []).map((file) => ({
				path: file.path,
				added: file.added,
				removed: file.removed,
				hunks: [],
			})),
		})),
		pinned: workspace.pinned ?? false,
		pinOrder: workspace.pinOrder ?? 0,
		color: workspace.color,
		description: workspace.description,
		expanded: workspace.expanded ?? false,
		lastUsed: workspace.lastUsed ?? workspace.createdAt ?? '',
	}));

	if (!includeStatus) {
		return mapped;
	}
	return mapped;
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

export type WorkspacePopoutState = {
	workspaceId: string;
	windowName: string;
	open: boolean;
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

export async function previewRepoHooks(source: string, ref = ''): Promise<string[]> {
	const payload = await PreviewRepoHooks({
		source,
		ref: ref.trim() || undefined,
	});
	if (!payload.exists || !payload.hooks || payload.hooks.length === 0) {
		return [];
	}

	const unique = new Set<string>();
	for (const hook of payload.hooks) {
		const id = hook.id?.trim();
		if (id) {
			unique.add(id);
			continue;
		}
		const run = hook.run?.map((entry) => entry.trim()).filter((entry) => entry.length > 0) ?? [];
		if (run.length > 0) {
			unique.add(run.join(' && '));
		}
	}
	return Array.from(unique);
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

export async function setWorkspaceDescription(
	workspaceId: string,
	description: string,
): Promise<Workspace> {
	const result = await SetWorkspaceDescription(workspaceId, description);
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

export async function openWorkspacePopout(workspaceId: string): Promise<WorkspacePopoutState> {
	return OpenWorkspacePopout(workspaceId);
}

export async function closeWorkspacePopout(workspaceId: string): Promise<void> {
	await CloseWorkspacePopout(workspaceId);
}

export async function listWorkspacePopouts(): Promise<WorkspacePopoutState[]> {
	return ListWorkspacePopouts();
}

function mapWorkspaceRefToWorkspace(ref: WorkspaceRefJSON): Workspace {
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
		description: ref.description,
		expanded: ref.expanded,
		lastUsed: ref.last_used ?? ref.created_at ?? '',
	};
}
