import {
	addRepo as addRepoMutation,
	addReposToWorkset as addReposToWorksetMutation,
	archiveWorkspace as archiveWorkspaceMutation,
	createWorkspace as createWorkspaceMutation,
	removeRepo as removeRepoMutation,
	removeWorkspace as removeWorkspaceMutation,
	renameWorkspace as renameWorkspaceMutation,
} from '../api/workspaces';
import { deriveRepoName, isRepoSource } from '../names';
import { registerRepo as registerRepoMutation } from '../api/settings';
import type {
	HookExecution,
	RepoAddResponse,
	WorksetRepoAddResponse,
	WorkspaceCreateResponse,
} from '../types';

export type WorkspaceActionPendingHook = {
	event: string;
	repo: string;
	hooks: string[];
	status?: string;
	reason?: string;
};

type CreateDirectRepo = {
	url: string;
	register: boolean;
};

export type CreateWorkspaceMutationInput = {
	finalName: string;
	primaryInput: string;
	directRepos: CreateDirectRepo[];
	selectedAliases: Iterable<string>;
	worksetName?: string;
	worksetOnly?: boolean;
};

type CreateWorkspaceMutationDeps = {
	registerRepo: (
		name: string,
		source: string,
		description: string,
		repoDir: string,
	) => Promise<void>;
	createWorkspace: (
		name: string,
		path: string,
		workset?: string,
		repos?: string[],
		options?: { worksetOnly?: boolean },
	) => Promise<WorkspaceCreateResponse>;
};

export type AddItemsMutationInput = {
	workspaceId: string;
	source: string;
	selectedAliases: Iterable<string>;
};

type AddItemsMutationDeps = {
	addRepo: (
		workspaceId: string,
		source: string,
		name: string,
		repoDir: string,
	) => Promise<RepoAddResponse>;
	removeRepo?: (
		workspaceId: string,
		repoName: string,
		deleteWorktree: boolean,
		forget: boolean,
	) => Promise<void>;
};

export type AddReposToWorksetMutationInput = {
	worksetName: string;
	source: string;
	selectedAliases: Iterable<string>;
};

type AddReposToWorksetMutationDeps = {
	addReposToWorkset: (workset: string, sources: string[]) => Promise<WorksetRepoAddResponse>;
};

export type RenameWorkspaceMutationInput = {
	workspaceId: string;
	workspaceName: string;
};

type RenameWorkspaceMutationDeps = {
	renameWorkspace: (workspaceId: string, nextName: string) => Promise<void>;
};

export type ArchiveWorkspaceMutationInput = {
	workspaceId: string;
	reason: string;
};

type ArchiveWorkspaceMutationDeps = {
	archiveWorkspace: (workspaceId: string, reason: string) => Promise<void>;
};

export type RemoveWorkspaceMutationInput = {
	workspaceId: string;
	deleteFiles: boolean;
	force: boolean;
};

type RemoveWorkspaceMutationDeps = {
	removeWorkspace: (
		workspaceId: string,
		options: { deleteFiles: boolean; force: boolean },
	) => Promise<void>;
};

export type RemoveRepoMutationInput = {
	workspaceId: string;
	repoName: string;
	deleteWorktree: boolean;
};

type RemoveRepoMutationDeps = {
	removeRepo: (
		workspaceId: string,
		repoName: string,
		deleteWorktree: boolean,
		forget: boolean,
	) => Promise<void>;
};

export type HookTransitionInput = {
	warnings: string[];
	pendingHooks: WorkspaceActionPendingHook[];
	hookRuns: HookExecution[];
};

export type HookTransitionResult = {
	hasHookActivity: boolean;
	shouldAutoClose: boolean;
};

export type CreateWorkspaceMutationResult = {
	workspaceName: string;
	warnings: string[];
	pendingHooks: WorkspaceActionPendingHook[];
	hookRuns: HookExecution[];
};

export type AddItemsMutationResult = {
	itemCount: number;
	warnings: string[];
	pendingHooks: WorkspaceActionPendingHook[];
	hookRuns: HookExecution[];
};

export type RenameWorkspaceMutationResult = {
	workspaceId: string;
	workspaceName: string;
};

export type ArchiveWorkspaceMutationResult = {
	workspaceId: string;
};

export type RemoveWorkspaceMutationResult = {
	workspaceId: string;
};

export type RemoveRepoMutationResult = {
	workspaceId: string;
	repoName: string;
};

export type WorkspaceActionMutationGateway = {
	registerRepo: CreateWorkspaceMutationDeps['registerRepo'];
	createWorkspace: CreateWorkspaceMutationDeps['createWorkspace'];
	addRepo: AddItemsMutationDeps['addRepo'];
	addReposToWorkset: AddReposToWorksetMutationDeps['addReposToWorkset'];
	renameWorkspace: RenameWorkspaceMutationDeps['renameWorkspace'];
	archiveWorkspace: ArchiveWorkspaceMutationDeps['archiveWorkspace'];
	removeWorkspace: RemoveWorkspaceMutationDeps['removeWorkspace'];
	removeRepo: RemoveRepoMutationDeps['removeRepo'];
};

export type WorkspaceActionMutationService = {
	createWorkspace: (input: CreateWorkspaceMutationInput) => Promise<CreateWorkspaceMutationResult>;
	addItems: (input: AddItemsMutationInput) => Promise<AddItemsMutationResult>;
	addReposToWorkset: (input: AddReposToWorksetMutationInput) => Promise<AddItemsMutationResult>;
	renameWorkspace: (input: RenameWorkspaceMutationInput) => Promise<RenameWorkspaceMutationResult>;
	archiveWorkspace: (
		input: ArchiveWorkspaceMutationInput,
	) => Promise<ArchiveWorkspaceMutationResult>;
	removeWorkspace: (input: RemoveWorkspaceMutationInput) => Promise<RemoveWorkspaceMutationResult>;
	removeRepo: (input: RemoveRepoMutationInput) => Promise<RemoveRepoMutationResult>;
};

const registerRepoForWorkspaceAction = async (
	name: string,
	source: string,
	_description: string,
	_repoDir: string,
): Promise<void> => {
	await registerRepoMutation(name, source, 'origin', 'main');
};

export const workspaceActionMutationGateway: WorkspaceActionMutationGateway = {
	registerRepo: registerRepoForWorkspaceAction,
	createWorkspace: createWorkspaceMutation,
	addRepo: addRepoMutation,
	addReposToWorkset: addReposToWorksetMutation,
	renameWorkspace: renameWorkspaceMutation,
	archiveWorkspace: archiveWorkspaceMutation,
	removeWorkspace: removeWorkspaceMutation,
	removeRepo: removeRepoMutation,
};

const dedupeWarnings = (warnings: string[]): string[] => Array.from(new Set(warnings));

const normalizePendingHooks = (
	pendingHooks: WorkspaceActionPendingHook[] | undefined,
): WorkspaceActionPendingHook[] => (pendingHooks ?? []).map((pending) => ({ ...pending }));

export const evaluateHookTransition = ({
	warnings,
	pendingHooks,
	hookRuns,
}: HookTransitionInput): HookTransitionResult => {
	const hasHookActivity = warnings.length > 0 || pendingHooks.length > 0 || hookRuns.length > 0;
	const allRunsOk = hookRuns.every((run) => run.status === 'ok' || run.status === 'skipped');
	return {
		hasHookActivity,
		shouldAutoClose:
			hasHookActivity && pendingHooks.length === 0 && warnings.length === 0 && allRunsOk,
	};
};

export const createWorkspaceActionMutationService = (
	gateway: WorkspaceActionMutationGateway = workspaceActionMutationGateway,
): WorkspaceActionMutationService => ({
	createWorkspace: (input) =>
		runCreateWorkspaceMutation(input, {
			registerRepo: gateway.registerRepo,
			createWorkspace: gateway.createWorkspace,
		}),
	addItems: (input) =>
		runAddItemsMutation(input, {
			addRepo: gateway.addRepo,
			removeRepo: gateway.removeRepo,
		}),
	addReposToWorkset: (input) =>
		runAddReposToWorksetMutation(input, {
			addReposToWorkset: gateway.addReposToWorkset,
		}),
	renameWorkspace: (input) =>
		runRenameWorkspaceMutation(input, {
			renameWorkspace: gateway.renameWorkspace,
		}),
	archiveWorkspace: (input) =>
		runArchiveWorkspaceMutation(input, {
			archiveWorkspace: gateway.archiveWorkspace,
		}),
	removeWorkspace: (input) =>
		runRemoveWorkspaceMutation(input, {
			removeWorkspace: gateway.removeWorkspace,
		}),
	removeRepo: (input) =>
		runRemoveRepoMutation(input, {
			removeRepo: gateway.removeRepo,
		}),
});

export const workspaceActionMutations = createWorkspaceActionMutationService();

const deriveReposToProcess = (input: CreateWorkspaceMutationInput): CreateDirectRepo[] => {
	const reposToProcess = [...input.directRepos];
	const pendingSource = input.primaryInput.trim();
	const hasPendingSource = pendingSource.length > 0;
	if (!hasPendingSource || !isRepoSource(pendingSource)) {
		return reposToProcess;
	}
	if (reposToProcess.some((repo) => repo.url === pendingSource)) {
		return reposToProcess;
	}
	reposToProcess.push({ url: pendingSource, register: true });
	return reposToProcess;
};

const buildCreateWorkspaceRepoSources = (
	reposToProcess: CreateDirectRepo[],
	selectedAliases: Iterable<string>,
): string[] => {
	const repos: string[] = reposToProcess.map((repo) => repo.url);
	for (const alias of selectedAliases) {
		repos.push(alias);
	}
	return repos;
};

const registerReposInCatalog = async (
	reposToProcess: CreateDirectRepo[],
	deps: CreateWorkspaceMutationDeps,
	collectedWarnings: string[],
): Promise<void> => {
	for (const repo of reposToProcess) {
		if (!repo.register) continue;
		const repoName = deriveRepoName(repo.url) || repo.url;
		try {
			await deps.registerRepo(repoName, repo.url, '', '');
		} catch (error) {
			const message = error instanceof Error ? error.message : String(error);
			collectedWarnings.push(
				`Registered ${repoName} in workset but failed to save in Repo Catalog: ${message}`,
			);
		}
	}
};

export const runCreateWorkspaceMutation = async (
	input: CreateWorkspaceMutationInput,
	deps: CreateWorkspaceMutationDeps,
): Promise<CreateWorkspaceMutationResult> => {
	const reposToProcess = deriveReposToProcess(input);
	const repos = buildCreateWorkspaceRepoSources(reposToProcess, input.selectedAliases);

	const worksetName = input.worksetName?.trim() || undefined;
	const result = await deps.createWorkspace(
		input.finalName,
		'',
		worksetName,
		repos.length > 0 ? repos : undefined,
		{ worksetOnly: input.worksetOnly === true },
	);
	const collectedWarnings = [...(result.warnings ?? [])];
	if (!input.worksetOnly) {
		await registerReposInCatalog(reposToProcess, deps, collectedWarnings);
	}

	return {
		workspaceName: result.thread.name,
		warnings: dedupeWarnings(collectedWarnings),
		pendingHooks: normalizePendingHooks(result.pendingHooks),
		hookRuns: result.hookRuns ?? [],
	};
};

const collectRepoAddResult = (
	targetWarnings: string[],
	targetPendingHooks: WorkspaceActionPendingHook[],
	targetHookRuns: HookExecution[],
	result: RepoAddResponse,
): void => {
	if (result.warnings?.length) {
		targetWarnings.push(...result.warnings);
	}
	if (result.pendingHooks?.length) {
		targetPendingHooks.push(...result.pendingHooks);
	}
	if (result.hookRuns?.length) {
		targetHookRuns.push(...result.hookRuns);
	}
};

export const runAddItemsMutation = async (
	input: AddItemsMutationInput,
	deps: AddItemsMutationDeps,
): Promise<AddItemsMutationResult> => {
	const source = input.source.trim();
	const aliases = Array.from(input.selectedAliases);

	const collectedWarnings: string[] = [];
	const collectedPendingHooks: WorkspaceActionPendingHook[] = [];
	const collectedHookRuns: HookExecution[] = [];
	const addedRepoNames: string[] = [];

	const addAndCollect = async (itemSource: string): Promise<void> => {
		const result = await deps.addRepo(input.workspaceId, itemSource, '', '');
		collectRepoAddResult(collectedWarnings, collectedPendingHooks, collectedHookRuns, result);
		const addedRepoName = result.payload?.repo?.trim();
		if (addedRepoName) {
			addedRepoNames.push(addedRepoName);
		}
	};

	const rollbackAddedRepos = async (): Promise<string[]> => {
		if (!deps.removeRepo || addedRepoNames.length === 0) {
			return [];
		}
		const rollbackWarnings: string[] = [];
		for (const repoName of addedRepoNames.reverse()) {
			try {
				await deps.removeRepo(input.workspaceId, repoName, false, false);
			} catch (error) {
				const message = error instanceof Error ? error.message : String(error);
				rollbackWarnings.push(`Rollback failed for ${repoName}: ${message}`);
			}
		}
		return rollbackWarnings;
	};

	try {
		if (source.length > 0) {
			await addAndCollect(source);
		}
		for (const alias of aliases) {
			await addAndCollect(alias);
		}
	} catch (error) {
		const rollbackWarnings = await rollbackAddedRepos();
		const baseMessage = error instanceof Error ? error.message : String(error);
		const fullMessage =
			rollbackWarnings.length > 0
				? `${baseMessage} (rollback warnings: ${rollbackWarnings.join('; ')})`
				: baseMessage;
		throw new Error(fullMessage, { cause: error });
	}

	const pendingByKey = new Map<string, WorkspaceActionPendingHook>();
	for (const pending of collectedPendingHooks) {
		pendingByKey.set(`${pending.repo}:${pending.event}`, { ...pending });
	}

	return {
		itemCount: (source.length > 0 ? 1 : 0) + aliases.length,
		warnings: dedupeWarnings(collectedWarnings),
		pendingHooks: Array.from(pendingByKey.values()),
		hookRuns: collectedHookRuns,
	};
};

export const runAddReposToWorksetMutation = async (
	input: AddReposToWorksetMutationInput,
	deps: AddReposToWorksetMutationDeps,
): Promise<AddItemsMutationResult> => {
	const source = input.source.trim();
	const sources = [
		...(source.length > 0 ? [source] : []),
		...Array.from(input.selectedAliases)
			.map((alias) => alias.trim())
			.filter((alias) => alias.length > 0),
	];
	const result = await deps.addReposToWorkset(input.worksetName, sources);
	return {
		itemCount: result.payload.added?.length ?? 0,
		warnings: dedupeWarnings(result.warnings ?? []),
		pendingHooks: [],
		hookRuns: [],
	};
};

export const runRenameWorkspaceMutation = async (
	input: RenameWorkspaceMutationInput,
	deps: RenameWorkspaceMutationDeps,
): Promise<RenameWorkspaceMutationResult> => {
	await deps.renameWorkspace(input.workspaceId, input.workspaceName);
	return {
		workspaceId: input.workspaceId,
		workspaceName: input.workspaceName,
	};
};

export const runArchiveWorkspaceMutation = async (
	input: ArchiveWorkspaceMutationInput,
	deps: ArchiveWorkspaceMutationDeps,
): Promise<ArchiveWorkspaceMutationResult> => {
	await deps.archiveWorkspace(input.workspaceId, input.reason);
	return { workspaceId: input.workspaceId };
};

export const runRemoveWorkspaceMutation = async (
	input: RemoveWorkspaceMutationInput,
	deps: RemoveWorkspaceMutationDeps,
): Promise<RemoveWorkspaceMutationResult> => {
	await deps.removeWorkspace(input.workspaceId, {
		deleteFiles: input.deleteFiles,
		force: input.force,
	});
	return { workspaceId: input.workspaceId };
};

export const runRemoveRepoMutation = async (
	input: RemoveRepoMutationInput,
	deps: RemoveRepoMutationDeps,
): Promise<RemoveRepoMutationResult> => {
	await deps.removeRepo(input.workspaceId, input.repoName, input.deleteWorktree, false);
	return {
		workspaceId: input.workspaceId,
		repoName: input.repoName,
	};
};
