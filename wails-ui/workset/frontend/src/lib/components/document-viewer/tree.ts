import type { RepoFileSearchResult } from '../../types';
import type { RepoDirectoryEntry } from '../../api/repo-files';

export type DocumentViewerTreeNode =
	| { kind: 'repo'; key: string; label: string; repoId: string; depth: number }
	| { kind: 'dir'; key: string; label: string; depth: number }
	| {
			kind: 'file';
			key: string;
			label: string;
			depth: number;
			path: string;
			repoId: string;
			isMarkdown: boolean;
	  };

export const buildExpandedKeysForQuery = (results: RepoFileSearchResult[]): Set<string> => {
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
	nodes: DocumentViewerTreeNode[],
	repoId: string,
	dirKey: string,
	label: string,
	depth: number,
): void => {
	nodes.push({
		kind: 'dir',
		key: dirKey,
		label,
		depth,
	});
};

const pushFileNode = (nodes: DocumentViewerTreeNode[], result: RepoFileSearchResult): void => {
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
	nodes: DocumentViewerTreeNode[],
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

			pushDirNode(nodes, repoId, dirKey, parts[i - 1] ?? '', i);
		}

		const parentKey =
			parts.length > 1 ? `dir:${repoId}:${parts.slice(0, parts.length - 1).join('/')}` : repoKey;
		if (!expandedNodes.has(parentKey)) continue;

		pushFileNode(nodes, result);
	}
};

export const computeChildCounts = (results: RepoFileSearchResult[]): Map<string, number> => {
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

export const buildDocumentViewerTree = (
	repos: RepoRef[],
	results: RepoFileSearchResult[],
	expandedNodes: Set<string>,
): DocumentViewerTreeNode[] => {
	const grouped = new Map(groupResultsByRepo(results));
	const sortedRepos = [...repos].sort((a, b) => a.name.localeCompare(b.name));
	const nodes: DocumentViewerTreeNode[] = [];

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

/** Key for looking up directory entries: "repoId:" for root, "repoId:path" for nested. */
export const dirEntriesKey = (repoId: string, dirPath: string): string => `${repoId}:${dirPath}`;

const appendDirChildren = (
	nodes: DocumentViewerTreeNode[],
	repoId: string,
	dirPath: string,
	dirEntries: Map<string, RepoDirectoryEntry[]>,
	expandedNodes: Set<string>,
	depth: number,
): void => {
	const key = dirEntriesKey(repoId, dirPath);
	const entries = dirEntries.get(key);
	if (!entries) return;

	for (const entry of entries) {
		if (entry.isDir) {
			const dirKey = `dir:${repoId}:${entry.path}`;
			nodes.push({ kind: 'dir', key: dirKey, label: entry.name, depth });

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
 * `dirEntries` is keyed by `dirEntriesKey(repoId, dirPath)`.
 */
export const buildDocumentViewerTreeFromDirs = (
	repos: RepoRef[],
	dirEntries: Map<string, RepoDirectoryEntry[]>,
	expandedNodes: Set<string>,
): DocumentViewerTreeNode[] => {
	const sortedRepos = [...repos].sort((a, b) => a.name.localeCompare(b.name));
	const nodes: DocumentViewerTreeNode[] = [];

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
export const computeDirChildCounts = (
	dirEntries: Map<string, RepoDirectoryEntry[]>,
): Map<string, number> => {
	const counts = new Map<string, number>();
	for (const [key, entries] of dirEntries) {
		// Extract repoId from key format "repoId:dirPath"
		const colonIdx = key.indexOf(':');
		const repoId = key.slice(0, colonIdx);
		const dirPath = key.slice(colonIdx + 1);
		const nodeKey = dirPath === '' ? `repo:${repoId}` : `dir:${repoId}:${dirPath}`;
		counts.set(nodeKey, entries.length);
	}
	return counts;
};
