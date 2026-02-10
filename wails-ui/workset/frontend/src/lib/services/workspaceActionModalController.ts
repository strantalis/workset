import type { HookExecution } from '../types';
import type { WorkspaceActionMode } from './workspaceActionContextService';
import { evaluateHookTransition, type WorkspaceActionPendingHook } from './workspaceActionService';

export type WorkspaceActionModalPhase = 'form' | 'hook-results';

export type WorkspaceActionHookResultContext = {
	action: 'created' | 'added';
	name: string;
	itemCount?: number;
};

type BaseHookTransitionInput = {
	warnings: string[];
	pendingHooks: WorkspaceActionPendingHook[];
	hookRuns: HookExecution[];
};

type CreateHookTransitionInput = BaseHookTransitionInput & {
	action: 'created';
	workspaceName: string;
};

type AddItemsHookTransitionInput = BaseHookTransitionInput & {
	action: 'added';
	workspaceName: string;
	itemCount: number;
};

type HookTransitionInput = CreateHookTransitionInput | AddItemsHookTransitionInput;

export type HookTransitionResolution = {
	phase: WorkspaceActionModalPhase;
	hookResultContext: WorkspaceActionHookResultContext | null;
	success: string | null;
	shouldClose: boolean;
	shouldAutoClose: boolean;
};

type ModalSubtitleInput = {
	phase: WorkspaceActionModalPhase;
	mode: WorkspaceActionMode;
	workspaceName: string | null;
	hookResultContext: WorkspaceActionHookResultContext | null;
};

type RemovalStateInput = {
	removeDeleteFiles: boolean;
	removeForceDelete: boolean;
	removeConfirmText: string;
	removeRepoConfirmRequired: boolean;
	removeRepoConfirmText: string;
	removeRepoStatusRequested: boolean;
};

export type RemovalStateResolution = {
	removeForceDelete: boolean;
	removeConfirmText: string;
	removeRepoConfirmText: string;
	removeRepoStatusRequested: boolean;
};

export const resetWorkspaceActionFlow = (): Pick<
	HookTransitionResolution,
	'phase' | 'hookResultContext'
> => ({
	phase: 'form',
	hookResultContext: null,
});

export const resolveMutationHookTransition = (
	input: HookTransitionInput,
): HookTransitionResolution => {
	const transition = evaluateHookTransition({
		warnings: input.warnings,
		pendingHooks: input.pendingHooks,
		hookRuns: input.hookRuns,
	});

	if (!transition.hasHookActivity) {
		return {
			phase: 'form',
			hookResultContext: null,
			success: null,
			shouldClose: true,
			shouldAutoClose: false,
		};
	}

	const success =
		input.action === 'created'
			? `Created ${input.workspaceName}.`
			: `Added ${input.itemCount} item${input.itemCount !== 1 ? 's' : ''}.`;

	return {
		phase: 'hook-results',
		hookResultContext:
			input.action === 'created'
				? { action: 'created', name: input.workspaceName }
				: { action: 'added', name: input.workspaceName, itemCount: input.itemCount },
		success,
		shouldClose: false,
		shouldAutoClose: transition.shouldAutoClose,
	};
};

export const deriveWorkspaceActionModalTitle = (
	mode: WorkspaceActionMode,
	phase: WorkspaceActionModalPhase,
): string => {
	if (phase === 'hook-results') return 'Hook results';
	if (mode === 'create') return 'Create workset';
	if (mode === 'rename') return 'Rename workset';
	if (mode === 'add-repo') return 'Add to workset';
	if (mode === 'archive') return 'Archive workset';
	if (mode === 'remove-workspace') return 'Remove workset';
	if (mode === 'remove-repo') return 'Remove repo';
	return 'Workset action';
};

export const deriveWorkspaceActionModalSubtitle = (input: ModalSubtitleInput): string => {
	if (input.phase === 'hook-results') return input.hookResultContext?.name ?? '';
	if (input.mode === 'create') return '';
	return input.workspaceName ?? '';
};

export const deriveWorkspaceActionModalSize = (
	mode: WorkspaceActionMode,
	phase: WorkspaceActionModalPhase,
): 'md' | 'wide' => {
	if (phase === 'hook-results') return 'md';
	if (mode === 'create' || mode === 'add-repo') return 'wide';
	return 'md';
};

export const resolveRemovalState = (input: RemovalStateInput): RemovalStateResolution => ({
	removeForceDelete: input.removeDeleteFiles ? input.removeForceDelete : false,
	removeConfirmText: input.removeDeleteFiles ? input.removeConfirmText : '',
	removeRepoConfirmText: input.removeRepoConfirmRequired ? input.removeRepoConfirmText : '',
	removeRepoStatusRequested: input.removeRepoConfirmRequired
		? input.removeRepoStatusRequested
		: false,
});

export const shouldRefreshRemoveRepoStatus = (
	removeRepoConfirmRequired: boolean,
	removeRepoStatusRequested: boolean,
): boolean => removeRepoConfirmRequired && !removeRepoStatusRequested;
