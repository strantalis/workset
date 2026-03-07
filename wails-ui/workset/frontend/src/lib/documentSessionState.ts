import type { DocumentSession, Workspace } from './types';

export function createDocumentSession(input: {
	workspaceId: string;
	workspaceName: string;
	repoId: string;
	repoName: string;
	path: string;
	preferredMode?: DocumentSession['preferredMode'];
}): DocumentSession {
	return {
		workspaceId: input.workspaceId,
		workspaceName: input.workspaceName,
		repoId: input.repoId,
		repoName: input.repoName,
		path: input.path,
		openedAt: Date.now(),
		preferredMode: input.preferredMode,
	};
}

export function openDefaultDocumentSession(
	activeWorkspaceId: string | null | undefined,
	workspaces: Workspace[],
): DocumentSession | null {
	const workspaceId = activeWorkspaceId?.trim() ?? '';
	if (workspaceId === '') return null;
	const workspace = workspaces.find((entry) => entry.id === workspaceId);
	const repo = workspace?.repos[0];
	if (!workspace || !repo) return null;
	return createDocumentSession({
		workspaceId: workspace.id,
		workspaceName: workspace.name,
		repoId: repo.id,
		repoName: repo.name,
		path: '',
	});
}

export function openWorkspaceDocumentSession(
	activeWorkspaceId: string | null | undefined,
	workspaces: Workspace[],
	repoId: string,
	path: string,
): DocumentSession | null {
	const workspaceId = activeWorkspaceId?.trim() ?? '';
	if (workspaceId === '' || repoId.trim() === '') return null;
	const workspace = workspaces.find((entry) => entry.id === workspaceId);
	if (!workspace) return null;
	const repo = workspace.repos.find((entry) => entry.id === repoId);
	if (!repo) return null;
	return createDocumentSession({
		workspaceId: workspace.id,
		workspaceName: workspace.name,
		repoId: repo.id,
		repoName: repo.name,
		path,
	});
}

export function reconcileDocumentSession(
	session: DocumentSession | null,
	activeWorkspaceId: string | null | undefined,
	workspaces: Workspace[],
): DocumentSession | null {
	if (!session) return null;
	const workspaceId = activeWorkspaceId?.trim() ?? '';
	if (workspaceId === '' || session.workspaceId !== workspaceId) {
		return null;
	}
	const workspace = workspaces.find((entry) => entry.id === workspaceId);
	if (!workspace) return null;
	const repoExists = workspace.repos.some((repo) => repo.id === session.repoId);
	return repoExists ? session : null;
}
