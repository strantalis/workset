import type { Workspace } from '../types';
import { deriveWorksetIdentity } from './worksetViewModel';

type RepoWatchScopeOptions = {
	workspaces: Workspace[];
	activeWorkspaceId: string | null;
	fixedWorkspaceId?: string | null;
	warmWorksetIds?: string[];
};

const isRenderableWorkspace = (workspace: Workspace): boolean =>
	!workspace.archived && workspace.placeholder !== true;

export const resolveWorksetIdForWorkspace = (
	workspaces: Workspace[],
	workspaceId: string | null,
): string | null => {
	if (!workspaceId) return null;
	const target = workspaces.find(
		(workspace) => workspace.id === workspaceId && isRenderableWorkspace(workspace),
	);
	if (!target) return null;
	return deriveWorksetIdentity(target).id;
};

export const rememberWorksetId = (
	currentIds: string[],
	nextId: string | null,
	limit = 3,
): string[] => {
	if (!nextId) return currentIds;
	const next = [nextId, ...currentIds.filter((value) => value !== nextId)];
	const capped = next.slice(0, Math.max(1, limit));
	if (
		capped.length === currentIds.length &&
		capped.every((value, index) => value === currentIds[index])
	) {
		return currentIds;
	}
	return capped;
};

export const deriveHotWorksetIds = ({
	workspaces,
	activeWorkspaceId,
	fixedWorkspaceId = null,
	warmWorksetIds = [],
}: RepoWatchScopeOptions): Set<string> => {
	const hotIds = new Set<string>();
	const activeWorksetId = resolveWorksetIdForWorkspace(workspaces, activeWorkspaceId);
	if (activeWorksetId) hotIds.add(activeWorksetId);
	const fixedWorksetId = resolveWorksetIdForWorkspace(workspaces, fixedWorkspaceId);
	if (fixedWorksetId) hotIds.add(fixedWorksetId);
	for (const warmId of warmWorksetIds) {
		if (warmId.trim().length > 0) {
			hotIds.add(warmId);
		}
	}
	return hotIds;
};

export const deriveWatchedWorkspaces = (options: RepoWatchScopeOptions): Workspace[] => {
	const hotWorksetIds = deriveHotWorksetIds(options);
	if (hotWorksetIds.size === 0) return [];
	return options.workspaces.filter((workspace) => {
		if (!isRenderableWorkspace(workspace)) return false;
		return hotWorksetIds.has(deriveWorksetIdentity(workspace).id);
	});
};
