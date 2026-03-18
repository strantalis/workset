import type { RepoFileSearchResult } from '../../types';

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

export const buildDocumentViewerTree = (
	results: RepoFileSearchResult[],
	expandedNodes: Set<string>,
): DocumentViewerTreeNode[] => {
	const nodes: DocumentViewerTreeNode[] = [];

	for (const [repoId, files] of groupResultsByRepo(results)) {
		const repoName = files[0]?.repoName ?? repoId;
		const repoKey = `repo:${repoId}`;
		nodes.push({
			kind: 'repo',
			key: repoKey,
			label: repoName,
			repoId,
			depth: 0,
		});

		if (!expandedNodes.has(repoKey)) continue;
		appendRepoChildren(nodes, repoId, repoKey, files, expandedNodes);
	}

	return nodes;
};
