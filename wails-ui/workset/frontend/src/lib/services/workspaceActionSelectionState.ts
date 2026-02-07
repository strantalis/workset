import type { WorkspaceActionDirectRepo } from './workspaceActionContextService';

export const toggleSetItem = (current: Set<string>, value: string): Set<string> => {
	const next = new Set(current);
	if (next.has(value)) next.delete(value);
	else next.add(value);
	return next;
};

export const removeSetItem = (current: Set<string>, value: string): Set<string> => {
	const next = new Set(current);
	next.delete(value);
	return next;
};

export const addDirectRepoSource = (
	directRepos: WorkspaceActionDirectRepo[],
	source: string,
	isRepoSource: (source: string) => boolean,
): { directRepos: WorkspaceActionDirectRepo[]; source: string } => {
	const trimmed = source.trim();
	if (!trimmed || !isRepoSource(trimmed) || directRepos.some((entry) => entry.url === trimmed)) {
		return { directRepos, source };
	}
	return {
		directRepos: [...directRepos, { url: trimmed, register: true }],
		source: '',
	};
};

export const removeDirectRepoByURL = (
	directRepos: WorkspaceActionDirectRepo[],
	url: string,
): WorkspaceActionDirectRepo[] => directRepos.filter((entry) => entry.url !== url);

export const toggleDirectRepoRegisterByURL = (
	directRepos: WorkspaceActionDirectRepo[],
	url: string,
): WorkspaceActionDirectRepo[] =>
	directRepos.map((entry) => (entry.url === url ? { ...entry, register: !entry.register } : entry));
