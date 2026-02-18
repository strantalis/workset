import type { HookExecution, HookProgressEvent, HooksRunResponse } from '../types';
import type { WorkspaceActionPendingHook as WorkspaceActionBasePendingHook } from './workspaceActionService';

export type WorkspaceActionPendingHook = WorkspaceActionBasePendingHook & {
	running?: boolean;
	runError?: string;
	trusting?: boolean;
	trusted?: boolean;
};

export type HookTrackingState = {
	activeHookOperation: string | null;
	activeHookWorkspace: string | null;
	hookRuns: HookExecution[];
	pendingHooks: WorkspaceActionPendingHook[];
};

export type HookTrackingContext = Pick<
	HookTrackingState,
	'activeHookOperation' | 'activeHookWorkspace'
> & {
	loading: boolean;
};

export type HandleRunPendingHookCoreInput = {
	pending: WorkspaceActionPendingHook;
	pendingHooks: WorkspaceActionPendingHook[];
	hookRuns: HookExecution[];
	workspaceReferences: Array<string | null | undefined>;
	activeHookOperation: string | null;
	getPendingHooks?: () => WorkspaceActionPendingHook[];
	getHookRuns?: () => HookExecution[];
};

export type HandleTrustPendingHookCoreInput = {
	pending: WorkspaceActionPendingHook;
	pendingHooks: WorkspaceActionPendingHook[];
	getPendingHooks?: () => WorkspaceActionPendingHook[];
};

type RunPendingHookCoreResultState = {
	pendingHooks: WorkspaceActionPendingHook[];
	hookRuns: HookExecution[];
};

type RunPendingHookCoreDeps = {
	runRepoHooks: (
		workspace: string,
		repo: string,
		event: string,
		reason: string,
	) => Promise<HooksRunResponse>;
	formatError: (err: unknown, fallback: string) => string;
};

type TrustPendingHookCoreDeps = {
	trustRepoHooks: (repo: string) => Promise<void>;
	formatError: (err: unknown, fallback: string) => string;
};

type RunPendingHookStateDeps = RunPendingHookCoreDeps & {
	setPendingHooks: (pendingHooks: WorkspaceActionPendingHook[]) => void;
	setHookRuns: (hookRuns: HookExecution[]) => void;
};

type TrustPendingHookStateDeps = TrustPendingHookCoreDeps & {
	setPendingHooks: (pendingHooks: WorkspaceActionPendingHook[]) => void;
};

export const beginHookTracking = (
	operation: string,
	workspaceName: string | null,
): HookTrackingState => ({
	activeHookOperation: operation,
	activeHookWorkspace: workspaceName,
	hookRuns: [],
	pendingHooks: [],
});

export const clearHookTracking = (): Pick<
	HookTrackingState,
	'activeHookOperation' | 'activeHookWorkspace'
> => ({
	activeHookOperation: null,
	activeHookWorkspace: null,
});

export const appendHookRuns = (
	currentHookRuns: HookExecution[],
	runs: HookExecution[] | undefined,
): HookExecution[] => {
	if (!runs || runs.length === 0) {
		return currentHookRuns;
	}
	const byKey = new Map<string, HookExecution>();
	for (const run of currentHookRuns) {
		byKey.set(`${run.repo}:${run.event}:${run.id}`, run);
	}
	for (const run of runs) {
		byKey.set(`${run.repo}:${run.event}:${run.id}`, run);
	}
	return Array.from(byKey.values());
};

export const shouldTrackHookEvent = (
	payload: HookProgressEvent,
	context: HookTrackingContext,
): boolean => {
	if (!context.activeHookOperation || !context.loading) {
		return false;
	}
	if (payload.operation !== context.activeHookOperation) {
		return false;
	}
	if (
		context.activeHookWorkspace &&
		payload.workspace &&
		payload.workspace.trim() &&
		payload.workspace !== context.activeHookWorkspace
	) {
		return false;
	}
	return true;
};

export const applyHookProgress = (
	currentHookRuns: HookExecution[],
	payload: HookProgressEvent,
): HookExecution[] => {
	const existingIdx = currentHookRuns.findIndex(
		(entry) =>
			entry.repo === payload.repo && entry.event === payload.event && entry.id === payload.hookId,
	);
	const next: HookExecution = {
		repo: payload.repo,
		event: payload.event,
		id: payload.hookId,
		status:
			payload.phase === 'finished'
				? payload.status || (payload.error ? 'failed' : 'ok')
				: 'running',
		log_path: payload.logPath,
	};
	if (existingIdx >= 0) {
		return currentHookRuns.map((entry, index) => (index === existingIdx ? next : entry));
	}
	return [...currentHookRuns, next];
};

const updatePendingHookByRepo = (
	pendingHooks: WorkspaceActionPendingHook[],
	repo: string,
	patch: Partial<WorkspaceActionPendingHook>,
): WorkspaceActionPendingHook[] =>
	pendingHooks.map((entry) => (entry.repo === repo ? { ...entry, ...patch } : entry));

const resolveWorkspaceReference = (
	workspaceReferences: Array<string | null | undefined>,
): string => {
	for (const candidate of workspaceReferences) {
		if (candidate) {
			return candidate;
		}
	}
	return '';
};

export const handleRunPendingHookCore = (
	input: HandleRunPendingHookCoreInput,
	deps: RunPendingHookCoreDeps,
): RunPendingHookCoreResultState & { completion: Promise<RunPendingHookCoreResultState> } => {
	const targetWorkspace = resolveWorkspaceReference(input.workspaceReferences);
	if (!targetWorkspace) {
		const nextPendingHooks = updatePendingHookByRepo(input.pendingHooks, input.pending.repo, {
			running: false,
			runError: 'Workspace reference unavailable for hook run.',
		});
		const state = {
			pendingHooks: nextPendingHooks,
			hookRuns: input.hookRuns,
		};
		return {
			...state,
			completion: Promise.resolve(state),
		};
	}

	const runningPendingHooks = updatePendingHookByRepo(input.pendingHooks, input.pending.repo, {
		running: true,
		runError: undefined,
	});

	const completion = (async (): Promise<RunPendingHookCoreResultState> => {
		try {
			const result = await deps.runRepoHooks(
				targetWorkspace,
				input.pending.repo,
				input.pending.event,
				`${input.activeHookOperation ?? 'hooks.run'}.ui`,
			);
			const latestHookRuns = input.getHookRuns?.() ?? input.hookRuns;
			const nextHookRuns = appendHookRuns(
				latestHookRuns,
				result.results.map((run) => ({
					event: result.event,
					repo: result.repo,
					id: run.id,
					status: run.status,
					log_path: run.log_path,
				})),
			);
			const latestPendingHooks = input.getPendingHooks?.() ?? runningPendingHooks;
			return {
				pendingHooks: latestPendingHooks.filter((entry) => entry.repo !== input.pending.repo),
				hookRuns: nextHookRuns,
			};
		} catch (err) {
			const message = deps.formatError(err, `Failed to run hooks for ${input.pending.repo}.`);
			const latestPendingHooks = input.getPendingHooks?.() ?? runningPendingHooks;
			return {
				pendingHooks: updatePendingHookByRepo(latestPendingHooks, input.pending.repo, {
					running: false,
					runError: message,
				}),
				hookRuns: input.getHookRuns?.() ?? input.hookRuns,
			};
		}
	})();

	return {
		pendingHooks: runningPendingHooks,
		hookRuns: input.hookRuns,
		completion,
	};
};

export const handleTrustPendingHookCore = (
	input: HandleTrustPendingHookCoreInput,
	deps: TrustPendingHookCoreDeps,
): {
	pendingHooks: WorkspaceActionPendingHook[];
	completion: Promise<WorkspaceActionPendingHook[]>;
} => {
	const trustingPendingHooks = updatePendingHookByRepo(input.pendingHooks, input.pending.repo, {
		trusting: true,
		runError: undefined,
	});

	const completion = (async (): Promise<WorkspaceActionPendingHook[]> => {
		try {
			await deps.trustRepoHooks(input.pending.repo);
			const latestPendingHooks = input.getPendingHooks?.() ?? trustingPendingHooks;
			return updatePendingHookByRepo(latestPendingHooks, input.pending.repo, {
				trusting: false,
				trusted: true,
			});
		} catch (err) {
			const message = deps.formatError(err, `Failed to trust hooks for ${input.pending.repo}.`);
			const latestPendingHooks = input.getPendingHooks?.() ?? trustingPendingHooks;
			return updatePendingHookByRepo(latestPendingHooks, input.pending.repo, {
				trusting: false,
				runError: message,
			});
		}
	})();

	return {
		pendingHooks: trustingPendingHooks,
		completion,
	};
};

export const runPendingHookWithState = async (
	input: HandleRunPendingHookCoreInput,
	deps: RunPendingHookStateDeps,
): Promise<void> => {
	const state = handleRunPendingHookCore(input, deps);
	deps.setPendingHooks(state.pendingHooks);
	deps.setHookRuns(state.hookRuns);
	const completed = await state.completion;
	deps.setPendingHooks(completed.pendingHooks);
	deps.setHookRuns(completed.hookRuns);
};

export const trustPendingHookWithState = async (
	input: HandleTrustPendingHookCoreInput,
	deps: TrustPendingHookStateDeps,
): Promise<void> => {
	const state = handleTrustPendingHookCore(input, deps);
	deps.setPendingHooks(state.pendingHooks);
	const completed = await state.completion;
	deps.setPendingHooks(completed);
};
