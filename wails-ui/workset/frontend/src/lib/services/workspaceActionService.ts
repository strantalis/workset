import { deriveRepoName, isRepoSource } from '../names';
import type { HookExecution, RepoAddResponse, WorkspaceCreateResponse } from '../types';

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

type CreateWorkspaceMutationInput = {
	finalName: string;
	primaryInput: string;
	directRepos: CreateDirectRepo[];
	selectedAliases: Iterable<string>;
	selectedGroups: Iterable<string>;
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
		repos?: string[],
		groups?: string[],
	) => Promise<WorkspaceCreateResponse>;
};

type AddItemsMutationInput = {
	workspaceId: string;
	source: string;
	selectedAliases: Iterable<string>;
	selectedGroups: Iterable<string>;
};

type AddItemsMutationDeps = {
	addRepo: (
		workspaceId: string,
		source: string,
		name: string,
		repoDir: string,
	) => Promise<RepoAddResponse>;
	applyGroup: (workspaceId: string, group: string) => Promise<void>;
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

export const runCreateWorkspaceMutation = async (
	input: CreateWorkspaceMutationInput,
	deps: CreateWorkspaceMutationDeps,
): Promise<CreateWorkspaceMutationResult> => {
	const reposToProcess = [...input.directRepos];
	const pendingSource = input.primaryInput.trim();
	if (
		pendingSource &&
		isRepoSource(pendingSource) &&
		!reposToProcess.some((repo) => repo.url === pendingSource)
	) {
		reposToProcess.push({ url: pendingSource, register: true });
	}

	const repos: string[] = [];
	for (const repo of reposToProcess) {
		const repoName = deriveRepoName(repo.url) || repo.url;
		if (repo.register) {
			await deps.registerRepo(repoName, repo.url, '', '');
		}
		repos.push(repo.register ? repoName : repo.url);
	}

	for (const alias of input.selectedAliases) {
		repos.push(alias);
	}

	const groups = Array.from(input.selectedGroups);
	const result = await deps.createWorkspace(
		input.finalName,
		'',
		repos.length > 0 ? repos : undefined,
		groups.length > 0 ? groups : undefined,
	);

	return {
		workspaceName: result.workspace.name,
		warnings: dedupeWarnings(result.warnings ?? []),
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
	const groups = Array.from(input.selectedGroups);

	const collectedWarnings: string[] = [];
	const collectedPendingHooks: WorkspaceActionPendingHook[] = [];
	const collectedHookRuns: HookExecution[] = [];

	if (source.length > 0) {
		const result = await deps.addRepo(input.workspaceId, source, '', '');
		collectRepoAddResult(collectedWarnings, collectedPendingHooks, collectedHookRuns, result);
	}

	for (const alias of aliases) {
		const result = await deps.addRepo(input.workspaceId, alias, '', '');
		collectRepoAddResult(collectedWarnings, collectedPendingHooks, collectedHookRuns, result);
	}

	for (const group of groups) {
		await deps.applyGroup(input.workspaceId, group);
	}

	const pendingByKey = new Map<string, WorkspaceActionPendingHook>();
	for (const pending of collectedPendingHooks) {
		pendingByKey.set(`${pending.repo}:${pending.event}`, { ...pending });
	}

	return {
		itemCount: (source.length > 0 ? 1 : 0) + aliases.length + groups.length,
		warnings: dedupeWarnings(collectedWarnings),
		pendingHooks: Array.from(pendingByKey.values()),
		hookRuns: collectedHookRuns,
	};
};
