import type {
	HooksRunResponse,
	RepoAddResponse,
	Thread,
	ThreadCreateResponse,
	WorksetRepoAddResponse,
	Workspace,
	WorkspaceCreateResponse,
} from '../types';
import { flushWorkspaceTerminalSnapshots } from '../terminal/terminalService';
import type { WorkspaceRefJSON } from '../../../bindings/github.com/strantalis/workset/pkg/worksetapi/models';
import {
	AddRepo,
	AddReposToWorkset,
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

export async function fetchThreads(
	includeArchived = false,
	includeStatus = false,
): Promise<Thread[]> {
	const snapshots = await ListWorkspaceSnapshots({ includeArchived, includeStatus });

	const mapped: Thread[] = snapshots.map((workspace) => {
		const identity = workspace as typeof workspace & {
			workset?: string;
			worksetKey?: string;
			worksetLabel?: string;
			placeholder?: boolean;
		};
		const workset = identity.workset;
		return {
			id: workspace.id,
			name: workspace.name,
			path: workspace.path,
			workset,
			worksetKey: identity.worksetKey ?? workspace.id,
			worksetLabel: identity.worksetLabel ?? workspace.name,
			placeholder: identity.placeholder === true,
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
					? (() => {
							const tracked = repo.trackedPullRequest as typeof repo.trackedPullRequest & {
								merged?: boolean;
								commentsCount?: number;
								reviewCommentsCount?: number;
							};
							return {
								repo: tracked.repo,
								number: tracked.number,
								url: tracked.url,
								title: tracked.title,
								body: tracked.body,
								state: tracked.state,
								draft: tracked.draft,
								merged: tracked.merged ?? tracked.state?.toLowerCase() === 'merged',
								baseRepo: tracked.baseRepo,
								baseBranch: tracked.baseBranch,
								headRepo: tracked.headRepo,
								headBranch: tracked.headBranch,
								updatedAt: tracked.updatedAt,
								commentsCount: tracked.commentsCount ?? 0,
								reviewCommentsCount: tracked.reviewCommentsCount ?? 0,
							};
						})()
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
		};
	});

	if (!includeStatus) {
		return mapped;
	}
	return mapped;
}

export const fetchWorkspaces = fetchThreads;

export async function createThread(
	name: string,
	path: string,
	workset?: string,
	repos?: string[],
	options: { worksetOnly?: boolean } = {},
): Promise<ThreadCreateResponse> {
	const normalizedWorkset = workset?.trim() || undefined;
	const request: Parameters<typeof CreateWorkspace>[0] & { worksetOnly?: boolean } = {
		name,
		path,
		workset: normalizedWorkset,
		worksetOnly: options.worksetOnly === true,
		repos,
	};
	const response = (await CreateWorkspace(request)) as WorkspaceCreateResponse & {
		workspace: ThreadCreateResponse['thread'];
	};
	return {
		...response,
		thread: response.workspace,
		workspace: response.workspace,
	};
}

export const createWorkspace = createThread;

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

export type RemoveThreadOptions = RemoveWorkspaceOptions;

export type WorkspacePopoutState = {
	workspaceId: string;
	windowName: string;
	open: boolean;
};

export type ThreadPopoutState = {
	threadId: string;
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

export const removeThread = removeWorkspace;

export async function addRepo(
	workspaceId: string,
	source: string,
	name: string,
	repoDir: string,
): Promise<RepoAddResponse> {
	return AddRepo({ workspaceId, source, name, repoDir });
}

export async function addReposToWorkset(
	workset: string,
	sources: string[],
): Promise<WorksetRepoAddResponse> {
	return AddReposToWorkset({ workset, sources });
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

export const pinThread = pinWorkspace;

export async function setWorkspaceColor(workspaceId: string, color: string): Promise<Workspace> {
	const result = await SetWorkspaceColor(workspaceId, color);
	return mapWorkspaceRefToWorkspace(result);
}

export const setThreadColor = setWorkspaceColor;

export async function setWorkspaceDescription(
	workspaceId: string,
	description: string,
): Promise<Workspace> {
	const result = await SetWorkspaceDescription(workspaceId, description);
	return mapWorkspaceRefToWorkspace(result);
}

export const setThreadDescription = setWorkspaceDescription;

export async function setWorkspaceExpanded(
	workspaceId: string,
	expanded: boolean,
): Promise<Workspace> {
	const result = await SetWorkspaceExpanded(workspaceId, expanded);
	return mapWorkspaceRefToWorkspace(result);
}

export const setThreadExpanded = setWorkspaceExpanded;

export async function reorderWorkspaces(orders: Record<string, number>): Promise<Workspace[]> {
	const result = await ReorderWorkspaces({ orders });
	return result.map(mapWorkspaceRefToWorkspace);
}

export const reorderThreads = reorderWorkspaces;

export async function updateWorkspaceLastUsed(workspaceId: string): Promise<string> {
	const result = await UpdateWorkspaceLastUsed(workspaceId);
	return result.last_used ?? result.created_at ?? '';
}

export const updateThreadLastUsed = updateWorkspaceLastUsed;

export async function openWorkspacePopout(workspaceId: string): Promise<WorkspacePopoutState> {
	await flushWorkspaceTerminalSnapshots(workspaceId);
	return OpenWorkspacePopout(workspaceId);
}

export async function openThreadPopout(workspaceId: string): Promise<ThreadPopoutState> {
	const state = await openWorkspacePopout(workspaceId);
	return {
		threadId: state.workspaceId,
		windowName: state.windowName,
		open: state.open,
	};
}

export async function closeWorkspacePopout(workspaceId: string): Promise<void> {
	await flushWorkspaceTerminalSnapshots(workspaceId);
	await CloseWorkspacePopout(workspaceId);
}

export const closeThreadPopout = closeWorkspacePopout;

export async function listWorkspacePopouts(): Promise<WorkspacePopoutState[]> {
	return ListWorkspacePopouts();
}

export async function listThreadPopouts(): Promise<ThreadPopoutState[]> {
	const states = await listWorkspacePopouts();
	return states.map((state) => ({
		threadId: state.workspaceId,
		windowName: state.windowName,
		open: state.open,
	}));
}

function mapWorkspaceRefToWorkspace(ref: WorkspaceRefJSON): Workspace {
	return {
		id: ref.name,
		name: ref.name,
		path: ref.path,
		workset: ref.workset,
		worksetKey: ref.name,
		worksetLabel: ref.name,
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
