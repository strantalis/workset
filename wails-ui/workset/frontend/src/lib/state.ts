import { derived, get, writable } from 'svelte/store';
import type { RepoDiffSummary, Workspace } from './types';
import type { RepoLocalStatus } from './api';
import { fetchWorkspaces } from './api';

export const workspaces = writable<Workspace[]>([]);
export const activeWorkspaceId = writable<string | null>(null);
export const activeRepoId = writable<string | null>(null);
export const loadingWorkspaces = writable(false);
export const workspaceError = writable<string | null>(null);
let loadSequence = 0;

export const activeWorkspace = derived(
	[workspaces, activeWorkspaceId],
	([$workspaces, $activeWorkspaceId]) =>
		$workspaces.find((workspace) => workspace.id === $activeWorkspaceId) ?? null,
);

export const activeRepo = derived(
	[activeWorkspace, activeRepoId],
	([$activeWorkspace, $activeRepoId]) =>
		$activeWorkspace?.repos.find((repo) => repo.id === $activeRepoId) ?? null,
);

export function selectWorkspace(workspaceId: string): void {
	activeWorkspaceId.set(workspaceId);
	activeRepoId.set(null);
}

export function selectRepo(repoId: string): void {
	activeRepoId.set(repoId);
}

export function clearRepo(): void {
	activeRepoId.set(null);
}

export function clearWorkspace(): void {
	activeWorkspaceId.set(null);
	activeRepoId.set(null);
}

const syncSelection = (data: Workspace[]): void => {
	const currentWorkspaceId = get(activeWorkspaceId);
	const currentRepoId = get(activeRepoId);
	const activeWorkspace =
		currentWorkspaceId &&
		data.find((workspace) => workspace.id === currentWorkspaceId && !workspace.archived);
	if (!activeWorkspace) {
		activeWorkspaceId.set(null);
		activeRepoId.set(null);
		return;
	}
	if (currentRepoId && !activeWorkspace.repos.some((repo) => repo.id === currentRepoId)) {
		activeRepoId.set(null);
	}
};

export async function loadWorkspaces(includeArchived = false): Promise<void> {
	const sequence = ++loadSequence;
	loadingWorkspaces.set(true);
	workspaceError.set(null);
	try {
		const data = await fetchWorkspaces(includeArchived, false);
		if (sequence !== loadSequence) {
			return;
		}
		workspaces.set(data);
		syncSelection(data);
	} catch (error) {
		const message = error instanceof Error ? error.message : 'Failed to load workspaces.';
		workspaceError.set(message);
	} finally {
		loadingWorkspaces.set(false);
	}

	void fetchWorkspaces(includeArchived, true)
		.then((data) => {
			if (sequence !== loadSequence) {
				return;
			}
			workspaces.set(data);
			syncSelection(data);
		})
		.catch(() => {});
}

export async function refreshWorkspacesStatus(includeArchived = false): Promise<void> {
	const sequence = ++loadSequence;
	try {
		const data = await fetchWorkspaces(includeArchived, true);
		if (sequence !== loadSequence) {
			return;
		}
		workspaces.set(data);
		syncSelection(data);
	} catch {
		// Ignore status refresh failures to avoid interrupting the UI.
	}
}

type RepoPatch = {
	dirty?: boolean;
	statusKnown?: boolean;
	missing?: boolean;
	diff?: { added: number; removed: number };
	ahead?: number;
	behind?: number;
};

const applyRepoPatch = (workspaceId: string, repoId: string, patch: RepoPatch): void => {
	workspaces.update((current) => {
		let changed = false;
		const next = current.map((workspace) => {
			if (workspace.id !== workspaceId) {
				return workspace;
			}
			let repoChanged = false;
			const repos = workspace.repos.map((repo) => {
				if (repo.id !== repoId) {
					return repo;
				}
				let updated = repo;
				const diffPatch = patch.diff;
				if (diffPatch) {
					if (repo.diff.added !== diffPatch.added || repo.diff.removed !== diffPatch.removed) {
						updated = { ...updated, diff: { added: diffPatch.added, removed: diffPatch.removed } };
						repoChanged = true;
					}
				}
				if (patch.dirty !== undefined && updated.dirty !== patch.dirty) {
					updated = { ...updated, dirty: patch.dirty };
					repoChanged = true;
				}
				if (patch.statusKnown !== undefined && updated.statusKnown !== patch.statusKnown) {
					updated = { ...updated, statusKnown: patch.statusKnown };
					repoChanged = true;
				}
				if (patch.missing !== undefined && updated.missing !== patch.missing) {
					updated = { ...updated, missing: patch.missing };
					repoChanged = true;
				}
				if (patch.ahead !== undefined && updated.ahead !== patch.ahead) {
					updated = { ...updated, ahead: patch.ahead };
					repoChanged = true;
				}
				if (patch.behind !== undefined && updated.behind !== patch.behind) {
					updated = { ...updated, behind: patch.behind };
					repoChanged = true;
				}
				return repoChanged ? updated : repo;
			});
			if (!repoChanged) {
				return workspace;
			}
			changed = true;
			return { ...workspace, repos };
		});
		return changed ? next : current;
	});
};

export const applyRepoDiffSummary = (
	workspaceId: string,
	repoId: string,
	summary: RepoDiffSummary,
): void => {
	applyRepoPatch(workspaceId, repoId, {
		diff: { added: summary.totalAdded, removed: summary.totalRemoved },
	});
};

export const applyRepoLocalStatus = (
	workspaceId: string,
	repoId: string,
	status: RepoLocalStatus,
): void => {
	const patch: RepoPatch = {
		dirty: status.hasUncommitted,
		statusKnown: true,
		ahead: status.ahead,
		behind: status.behind,
	};
	if (!status.hasUncommitted) {
		patch.diff = { added: 0, removed: 0 };
	}
	applyRepoPatch(workspaceId, repoId, patch);
};
