import { runRepoHooks, trustRepoHooks } from '../api/workspaces';
import { getGroup, listAliases, listGroups, openDirectoryDialog } from '../api/settings';
import type { Alias, GroupSummary, HookExecution, Workspace } from '../types';
import {
	loadWorkspaceActionContext,
	type WorkspaceActionDirectRepo,
} from './workspaceActionContextService';
import { formatWorkspaceActionError } from './workspaceActionErrors';
import {
	runPendingHookWithState,
	trustPendingHookWithState,
	type WorkspaceActionPendingHook,
} from './workspaceActionHooks';
import { workspaceActionMutations } from './workspaceActionService';

interface WorkspaceContextLoadParams {
	mode: 'create' | 'rename' | 'add-repo' | 'archive' | 'remove-workspace' | 'remove-repo' | null;
	workspaceId: string | null;
	repoName: string | null;
}

interface WorkspaceContextLoadDeps {
	loadWorkspaces: (force?: boolean) => Promise<void>;
	getWorkspaces: () => Workspace[];
}

export async function loadWorkspaceActionModalContext(
	params: WorkspaceContextLoadParams,
	deps: WorkspaceContextLoadDeps,
) {
	return loadWorkspaceActionContext(
		{
			mode: params.mode,
			workspaceId: params.workspaceId,
			repoName: params.repoName,
		},
		{
			loadWorkspaces: deps.loadWorkspaces,
			getWorkspaces: deps.getWorkspaces,
			listAliases,
			listGroups,
			getGroup,
		},
	);
}

interface RunPendingHookActionParams {
	pending: WorkspaceActionPendingHook;
	pendingHooks: WorkspaceActionPendingHook[];
	hookRuns: HookExecution[];
	workspaceReferences: Array<string | null | undefined>;
	activeHookOperation: string | null;
	getPendingHooks: () => WorkspaceActionPendingHook[];
	getHookRuns: () => HookExecution[];
	setPendingHooks: (next: WorkspaceActionPendingHook[]) => void;
	setHookRuns: (next: HookExecution[]) => void;
}

export async function runWorkspaceActionPendingHook(
	params: RunPendingHookActionParams,
): Promise<void> {
	await runPendingHookWithState(
		{
			pending: params.pending,
			pendingHooks: params.pendingHooks,
			hookRuns: params.hookRuns,
			workspaceReferences: params.workspaceReferences,
			activeHookOperation: params.activeHookOperation,
			getPendingHooks: params.getPendingHooks,
			getHookRuns: params.getHookRuns,
		},
		{
			runRepoHooks,
			formatError: formatWorkspaceActionError,
			setPendingHooks: params.setPendingHooks,
			setHookRuns: params.setHookRuns,
		},
	);
}

interface TrustPendingHookActionParams {
	pending: WorkspaceActionPendingHook;
	pendingHooks: WorkspaceActionPendingHook[];
	getPendingHooks: () => WorkspaceActionPendingHook[];
	setPendingHooks: (next: WorkspaceActionPendingHook[]) => void;
}

export async function trustWorkspaceActionPendingHook(
	params: TrustPendingHookActionParams,
): Promise<void> {
	await trustPendingHookWithState(
		{
			pending: params.pending,
			pendingHooks: params.pendingHooks,
			getPendingHooks: params.getPendingHooks,
		},
		{
			trustRepoHooks,
			formatError: formatWorkspaceActionError,
			setPendingHooks: params.setPendingHooks,
		},
	);
}

export async function browseWorkspaceActionDirectory(
	defaultDirectory: string,
): Promise<string | null> {
	return openDirectoryDialog('Select repo directory', defaultDirectory.trim());
}

interface RenameWorkspaceActionParams {
	workspaceId: string;
	workspaceName: string;
}

interface RenameWorkspaceActionDeps {
	loadWorkspaces: (force?: boolean) => Promise<void>;
	getActiveWorkspaceId: () => string | null;
	selectWorkspace: (workspaceId: string) => void;
}

export async function renameWorkspaceAction(
	params: RenameWorkspaceActionParams,
	deps: RenameWorkspaceActionDeps,
): Promise<string> {
	const result = await workspaceActionMutations.renameWorkspace(params);
	await deps.loadWorkspaces(true);
	if (deps.getActiveWorkspaceId() === params.workspaceId) {
		deps.selectWorkspace(result.workspaceName);
	}
	return result.workspaceName;
}

interface ArchiveWorkspaceActionParams {
	workspaceId: string;
	reason: string;
}

interface ArchiveWorkspaceActionDeps {
	loadWorkspaces: (force?: boolean) => Promise<void>;
	getActiveWorkspaceId: () => string | null;
	clearWorkspace: () => void;
}

export async function archiveWorkspaceAction(
	params: ArchiveWorkspaceActionParams,
	deps: ArchiveWorkspaceActionDeps,
): Promise<void> {
	const result = await workspaceActionMutations.archiveWorkspace(params);
	await deps.loadWorkspaces(true);
	if (deps.getActiveWorkspaceId() === result.workspaceId) {
		deps.clearWorkspace();
	}
}

export type WorkspaceActionContextLoadResult = Awaited<
	ReturnType<typeof loadWorkspaceActionModalContext>
>;

export type {
	WorkspaceActionDirectRepo,
	WorkspaceActionPendingHook,
	HookExecution,
	Alias,
	GroupSummary,
};
