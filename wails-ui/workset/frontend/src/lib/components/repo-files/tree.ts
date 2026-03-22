import type { RepoFileSearchResult } from '../../types';
import type { RepoDirectoryEntry } from '../../api/repo-files';

const DIR_ENTRIES_SEPARATOR = '\u0000';

export type RepoTreeNode =
	| { kind: 'repo'; key: string; label: string; repoId: string; depth: number }
	| { kind: 'dir'; key: string; label: string; depth: number; repoId: string; path: string }
	| {
			kind: 'file';
			key: string;
			label: string;
			depth: number;
			path: string;
			repoId: string;
			isMarkdown: boolean;
	  };

export const buildExpandedRepoTreeKeysForQuery = (results: RepoFileSearchResult[]): Set<string> => {
	const keys = new Set<string>();
	for (const result of results) {
		const parts = result.path.split('/');
		for (let i = 1; i < parts.length; i += 1) {
			keys.add(`dir:${result.repoId}:${parts.slice(0, i).join('/')}`);
		}
		keys.add(`repo:${result.repoId}`);
	}
	return keys;
};

export const shouldReplaceExpandedNodeSet = (current: Set<string>, next: Set<string>): boolean =>
	next.size !== current.size || Array.from(next).some((key) => !current.has(key));

const groupResultsByRepo = (
	results: RepoFileSearchResult[],
): Array<[string, RepoFileSearchResult[]]> => {
	const byRepo = new Map<string, RepoFileSearchResult[]>();
	for (const result of results) {
		const existing = byRepo.get(result.repoId);
		if (existing) existing.push(result);
		else byRepo.set(result.repoId, [result]);
	}
	return [...byRepo.entries()].sort((left, right) =>
		(left[1][0]?.repoName ?? '').localeCompare(right[1][0]?.repoName ?? ''),
	);
};

const pushDirNode = (
	nodes: RepoTreeNode[],
	repoId: string,
	dirKey: string,
	label: string,
	depth: number,
	path: string,
): void => {
	nodes.push({
		kind: 'dir',
		key: dirKey,
		label,
		depth,
		repoId,
		path,
	});
};

const pushFileNode = (nodes: RepoTreeNode[], result: RepoFileSearchResult): void => {
	const parts = result.path.split('/');
	nodes.push({
		kind: 'file',
		key: `file:${result.repoId}:${result.path}`,
		label: parts[parts.length - 1] ?? result.path,
		depth: Math.max(1, parts.length),
		path: result.path,
		repoId: result.repoId,
		isMarkdown: result.isMarkdown,
	});
};

const appendRepoChildren = (
	nodes: RepoTreeNode[],
	repoId: string,
	repoKey: string,
	files: RepoFileSearchResult[],
	expandedNodes: Set<string>,
): void => {
	const seenDirs = new Set<string>();
	const sortedFiles = [...files].sort((left, right) => left.path.localeCompare(right.path));

	for (const result of sortedFiles) {
		const parts = result.path.split('/');
		for (let i = 1; i < parts.length; i += 1) {
			const dirPath = parts.slice(0, i).join('/');
			const dirKey = `dir:${repoId}:${dirPath}`;
			if (seenDirs.has(dirKey)) continue;
			seenDirs.add(dirKey);

			const parentKey = i === 1 ? repoKey : `dir:${repoId}:${parts.slice(0, i - 1).join('/')}`;
			if (!expandedNodes.has(parentKey)) continue;

			pushDirNode(nodes, repoId, dirKey, parts[i - 1] ?? '', i, dirPath);
		}

		const parentKey =
			parts.length > 1 ? `dir:${repoId}:${parts.slice(0, parts.length - 1).join('/')}` : repoKey;
		if (!expandedNodes.has(parentKey)) continue;

		pushFileNode(nodes, result);
	}
};

export const computeRepoTreeChildCounts = (
	results: RepoFileSearchResult[],
): Map<string, number> => {
	const childSets = new Map<string, Set<string>>();

	for (const result of results) {
		const parts = result.path.split('/');
		const repoKey = `repo:${result.repoId}`;

		if (!childSets.has(repoKey)) childSets.set(repoKey, new Set());
		childSets.get(repoKey)!.add(parts[0]);

		for (let i = 1; i < parts.length; i += 1) {
			const dirPath = parts.slice(0, i).join('/');
			const dirKey = `dir:${result.repoId}:${dirPath}`;
			if (!childSets.has(dirKey)) childSets.set(dirKey, new Set());
			childSets.get(dirKey)!.add(parts[i]);
		}
	}

	const counts = new Map<string, number>();
	for (const [key, children] of childSets) {
		counts.set(key, children.size);
	}
	return counts;
};

export type RepoRef = { id: string; name: string };

export const buildRepoTree = (
	repos: RepoRef[],
	results: RepoFileSearchResult[],
	expandedNodes: Set<string>,
): RepoTreeNode[] => {
	const grouped = new Map(groupResultsByRepo(results));
	const sortedRepos = [...repos].sort((a, b) => a.name.localeCompare(b.name));
	const nodes: RepoTreeNode[] = [];

	for (const repo of sortedRepos) {
		const repoKey = `repo:${repo.id}`;
		nodes.push({
			kind: 'repo',
			key: repoKey,
			label: repo.name,
			repoId: repo.id,
			depth: 0,
		});

		if (!expandedNodes.has(repoKey)) continue;
		const files = grouped.get(repo.id);
		if (files) appendRepoChildren(nodes, repo.id, repoKey, files, expandedNodes);
	}

	return nodes;
};

// ── Directory-based tree builder (lazy loading) ──────────

export const createRepoDirEntriesKey = (repoId: string, dirPath: string): string =>
	`${repoId}${DIR_ENTRIES_SEPARATOR}${dirPath}`;

const readRepoDirEntriesKey = (key: string): { repoId: string; dirPath: string } => {
	const separatorIndex = key.indexOf(DIR_ENTRIES_SEPARATOR);
	if (separatorIndex < 0) {
		return { repoId: key, dirPath: '' };
	}

	return {
		repoId: key.slice(0, separatorIndex),
		dirPath: key.slice(separatorIndex + DIR_ENTRIES_SEPARATOR.length),
	};
};

const appendDirChildren = (
	nodes: RepoTreeNode[],
	repoId: string,
	dirPath: string,
	dirEntries: Map<string, RepoDirectoryEntry[]>,
	expandedNodes: Set<string>,
	depth: number,
): void => {
	const key = createRepoDirEntriesKey(repoId, dirPath);
	const entries = dirEntries.get(key);
	if (!entries) return;

	for (const entry of entries) {
		if (entry.isDir) {
			const dirKey = `dir:${repoId}:${entry.path}`;
			nodes.push({
				kind: 'dir',
				key: dirKey,
				label: entry.name,
				depth,
				repoId,
				path: entry.path,
			});

			if (expandedNodes.has(dirKey)) {
				appendDirChildren(nodes, repoId, entry.path, dirEntries, expandedNodes, depth + 1);
			}
		} else {
			nodes.push({
				kind: 'file',
				key: `file:${repoId}:${entry.path}`,
				label: entry.name,
				depth,
				path: entry.path,
				repoId,
				isMarkdown: entry.isMarkdown ?? false,
			});
		}
	}
};

/**
 * Build a tree from lazily-loaded directory entries.
 * `dirEntries` is keyed by `createRepoDirEntriesKey(repoId, dirPath)`.
 */
export const buildRepoTreeFromDirectories = (
	repos: RepoRef[],
	dirEntries: Map<string, RepoDirectoryEntry[]>,
	expandedNodes: Set<string>,
): RepoTreeNode[] => {
	const sortedRepos = [...repos].sort((a, b) => a.name.localeCompare(b.name));
	const nodes: RepoTreeNode[] = [];

	for (const repo of sortedRepos) {
		const repoKey = `repo:${repo.id}`;
		nodes.push({
			kind: 'repo',
			key: repoKey,
			label: repo.name,
			repoId: repo.id,
			depth: 0,
		});

		if (!expandedNodes.has(repoKey)) continue;
		appendDirChildren(nodes, repo.id, '', dirEntries, expandedNodes, 1);
	}

	return nodes;
};

/** Compute child counts from directory entries (for badges in the tree). */
export const computeRepoTreeDirectoryCounts = (
	dirEntries: Map<string, RepoDirectoryEntry[]>,
): Map<string, number> => {
	const counts = new Map<string, number>();
	for (const [key, entries] of dirEntries) {
		const { repoId, dirPath } = readRepoDirEntriesKey(key);
		const nodeKey = dirPath === '' ? `repo:${repoId}` : `dir:${repoId}:${dirPath}`;
		counts.set(nodeKey, entries.length);
	}
	return counts;
};
