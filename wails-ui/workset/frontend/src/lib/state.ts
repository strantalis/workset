import { derived, get, writable } from 'svelte/store';
import type { RepoDiffSummary, Workspace } from './types';
import type { RepoLocalStatus } from './api/github';
import {
	fetchWorkspaces,
	pinWorkspace as apiPinWorkspace,
	setWorkspaceColor as apiSetWorkspaceColor,
	setWorkspaceExpanded as apiSetWorkspaceExpanded,
	reorderWorkspaces as apiReorderWorkspaces,
	updateWorkspaceLastUsed as apiUpdateWorkspaceLastUsed,
} from './api/workspaces';

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

// Derived stores for pinned and unpinned workspaces
export const pinnedWorkspaces = derived(workspaces, ($workspaces) =>
	$workspaces.filter((w) => w.pinned && !w.archived).sort((a, b) => a.pinOrder - b.pinOrder),
);

export const unpinnedWorkspaces = derived(workspaces, ($workspaces) =>
	$workspaces
		.filter((w) => !w.pinned && !w.archived)
		.sort((a, b) => new Date(b.lastUsed).getTime() - new Date(a.lastUsed).getTime()),
);

export function selectWorkspace(workspaceId: string): void {
	activeWorkspaceId.set(workspaceId);
	activeRepoId.set(null);
	// Update last used timestamp in background
	void apiUpdateWorkspaceLastUsed(workspaceId).then(() => {
		// Refresh workspace list to get updated lastUsed
		void refreshWorkspacesStatus();
	});
}

// Update workspace pin status
export async function toggleWorkspacePin(workspaceId: string, pin: boolean): Promise<void> {
	// Optimistically update local state first
	workspaces.update((list) => {
		const workspace = list.find((w) => w.id === workspaceId);
		if (!workspace) return list;

		// Calculate new pin order if pinning
		let newPinOrder = workspace.pinOrder;
		if (pin && !workspace.pinned) {
			const maxOrder = Math.max(...list.filter((w) => w.pinned).map((w) => w.pinOrder), -1);
			newPinOrder = maxOrder + 1;
		} else if (!pin) {
			newPinOrder = 0;
		}

		return list.map((w) =>
			w.id === workspaceId ? { ...w, pinned: pin, pinOrder: newPinOrder } : w,
		);
	});

	// Then try to sync with backend
	try {
		await apiPinWorkspace(workspaceId, pin);
	} catch (error) {
		// eslint-disable-next-line no-console
		console.error('Failed to sync pin status:', error);
		// Could revert here if needed, but optimistic UI is fine for now
	}
}

// Update workspace color
export async function setWorkspaceColor(workspaceId: string, color: string): Promise<void> {
	// Optimistically update local state first
	workspaces.update((list) => list.map((w) => (w.id === workspaceId ? { ...w, color } : w)));

	// Then try to sync with backend
	try {
		await apiSetWorkspaceColor(workspaceId, color);
	} catch (error) {
		// eslint-disable-next-line no-console
		console.error('Failed to sync workspace color:', error);
	}
}

// Update workspace expanded state
export async function setWorkspaceExpanded(workspaceId: string, expanded: boolean): Promise<void> {
	// Optimistically update local state first
	workspaces.update((list) => list.map((w) => (w.id === workspaceId ? { ...w, expanded } : w)));

	// Then try to sync with backend
	try {
		await apiSetWorkspaceExpanded(workspaceId, expanded);
	} catch (error) {
		// eslint-disable-next-line no-console
		console.error('Failed to sync workspace expanded state:', error);
	}
}

// Reorder workspaces after drag and drop
export async function reorderWorkspaces(orders: Record<string, number>): Promise<void> {
	// Optimistically update local state first
	workspaces.update((list) =>
		list.map((w) => (orders[w.id] !== undefined ? { ...w, pinOrder: orders[w.id] } : w)),
	);

	// Then try to sync with backend
	try {
		await apiReorderWorkspaces(orders);
	} catch (error) {
		// eslint-disable-next-line no-console
		console.error('Failed to sync workspace reorder:', error);
	}
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
	currentBranch?: string;
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
				if (patch.currentBranch !== undefined && updated.currentBranch !== patch.currentBranch) {
					updated = { ...updated, currentBranch: patch.currentBranch };
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
		currentBranch: status.currentBranch,
	};
	if (!status.hasUncommitted) {
		patch.diff = { added: 0, removed: 0 };
	}
	applyRepoPatch(workspaceId, repoId, patch);
};
