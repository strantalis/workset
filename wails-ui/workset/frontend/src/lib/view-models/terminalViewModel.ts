import type { Workspace } from '../types';

export type TerminalRepoItem = {
	id: string;
	name: string;
	warning: boolean;
};

export type TerminalWorkspaceModel = {
	id: string;
	name: string;
	repos: TerminalRepoItem[];
};

export const mapWorkspaceToTerminalModel = (
	workspace: Workspace | null,
): TerminalWorkspaceModel | null => {
	if (!workspace) return null;
	return {
		id: workspace.id,
		name: workspace.name,
		repos: workspace.repos.map((repo) => ({
			id: repo.id,
			name: repo.name,
			warning: repo.missing || repo.dirty,
		})),
	};
};
