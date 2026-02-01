import { derived, get, writable } from 'svelte/store';
import type { Workspace } from './types';
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
