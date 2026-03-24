import type { RepoFileSearchResult } from '../../types';
import type { RepoDirectoryEntry } from '../../api/repo-files';
import type { RepoTreeNode } from '../repo-files/tree';

export type TreeSelection =
	| { kind: 'repo'; key: string; repoId: string }
	| { kind: 'dir'; key: string; repoId: string; path: string }
	| { kind: 'file'; key: string; repoId: string; path: string }
	| null;

export type InlineCreateState = {
	repoId: string;
	parentDirPath: string;
	insertAfterKey: string;
	depth: number;
	draftName: string;
	creating: boolean;
};

export type InlineCreateResolution = {
	inlineCreate: InlineCreateState;
	nextExpandedNodes: Set<string>;
	dirPathsToLoad: string[];
	shouldSelectRepo: boolean;
};

export type InlineCreateTreeNode = {
	kind: 'inline-create';
	key: 'inline-create';
	repoId: string;
	parentDirPath: string;
	depth: number;
};

export type ExplorerTreeNode = RepoTreeNode | InlineCreateTreeNode;

export const buildRepoNodeKey = (repoId: string): string => `repo:${repoId}`;
export const buildFileNodeKey = (repoId: string, path: string): string => `file:${repoId}:${path}`;
export const buildDirNodeKey = (repoId: string, path: string): string => `dir:${repoId}:${path}`;

export const getParentDirPath = (path: string): string => {
	const lastSlash = path.lastIndexOf('/');
	return lastSlash >= 0 ? path.slice(0, lastSlash) : '';
};

export const getFileDepthForParent = (parentDirPath: string): number =>
	parentDirPath === '' ? 1 : parentDirPath.split('/').length + 1;

export const insertInlineCreateNode = (
	nodes: RepoTreeNode[],
	createState: InlineCreateState | null,
): ExplorerTreeNode[] => {
	if (!createState) return nodes;

	const nextNodes: ExplorerTreeNode[] = [...nodes];
	const inlineNode: InlineCreateTreeNode = {
		kind: 'inline-create',
		key: 'inline-create',
		repoId: createState.repoId,
		parentDirPath: createState.parentDirPath,
		depth: createState.depth,
	};
	const insertIndex = nextNodes.findIndex((node) => node.key === createState.insertAfterKey);
	if (insertIndex < 0) {
		nextNodes.push(inlineNode);
	} else {
		nextNodes.splice(insertIndex + 1, 0, inlineNode);
	}
	return nextNodes;
};

export const sortDirectoryEntries = (entries: RepoDirectoryEntry[]): RepoDirectoryEntry[] =>
	[...entries].sort((left, right) => {
		if (left.isDir !== right.isDir) return left.isDir ? -1 : 1;
		return left.name.localeCompare(right.name);
	});

export const upsertCreatedDirectoryEntries = (
	entries: Map<string, RepoDirectoryEntry[]>,
	dirKey: string,
	fullPath: string,
	isMarkdown: boolean,
): Map<string, RepoDirectoryEntry[]> => {
	const fileName = fullPath.split('/').pop() ?? fullPath;
	const existingEntries = entries.get(dirKey) ?? [];
	const nextEntries = new Map(entries);
	nextEntries.set(
		dirKey,
		sortDirectoryEntries([
			...existingEntries.filter((entry) => entry.path !== fullPath),
			{
				name: fileName,
				path: fullPath,
				isDir: false,
				sizeBytes: 0,
				isMarkdown,
				childCount: 0,
			},
		]),
	);
	return nextEntries;
};

export const upsertLoadedRepoFileState = (
	files: RepoFileSearchResult[],
	workspaceId: string,
	repoId: string,
	repoName: string,
	fullPath: string,
	isMarkdown: boolean,
): RepoFileSearchResult[] =>
	[
		...files.filter((file) => file.path !== fullPath),
		{
			workspaceId,
			repoId,
			repoName,
			path: fullPath,
			isMarkdown,
			sizeBytes: 0,
			score: 0,
		},
	].sort((left, right) => left.path.localeCompare(right.path));

export const removeDeletedDirectoryEntry = (
	entries: Map<string, RepoDirectoryEntry[]>,
	dirKey: string,
	fullPath: string,
): Map<string, RepoDirectoryEntry[]> => {
	const existingEntries = entries.get(dirKey);
	if (!existingEntries) return entries;
	const nextEntries = existingEntries.filter((entry) => entry.path !== fullPath);
	const next = new Map(entries);
	next.set(dirKey, nextEntries);
	return next;
};

export const removeLoadedRepoFileState = (
	files: RepoFileSearchResult[],
	fullPath: string,
): RepoFileSearchResult[] => files.filter((file) => file.path !== fullPath);

const expandForParent = (
	repoId: string,
	parentDirPath: string,
	expandedNodes: Set<string>,
): { nextExpandedNodes: Set<string>; dirPathsToLoad: string[]; insertAfterKey: string } => {
	const repoKey = buildRepoNodeKey(repoId);
	const nextExpandedNodes = new Set(expandedNodes);
	const dirPathsToLoad = [''];
	nextExpandedNodes.add(repoKey);
	if (parentDirPath !== '') {
		const parts = parentDirPath.split('/');
		for (let i = 1; i <= parts.length; i += 1) {
			const dirPath = parts.slice(0, i).join('/');
			nextExpandedNodes.add(buildDirNodeKey(repoId, dirPath));
			dirPathsToLoad.push(dirPath);
		}
	}
	return {
		nextExpandedNodes,
		dirPathsToLoad,
		insertAfterKey: parentDirPath === '' ? repoKey : buildDirNodeKey(repoId, parentDirPath),
	};
};

export const resolveInlineCreate = (
	selection: TreeSelection,
	fallbackRepoId: string,
	expandedNodes: Set<string>,
): InlineCreateResolution => {
	if (selection?.kind === 'file') {
		const parentDirPath = getParentDirPath(selection.path);
		return {
			inlineCreate: {
				repoId: selection.repoId,
				parentDirPath,
				insertAfterKey: selection.key,
				depth: getFileDepthForParent(parentDirPath),
				draftName: '',
				creating: false,
			},
			nextExpandedNodes: expandedNodes,
			dirPathsToLoad: [],
			shouldSelectRepo: false,
		};
	}

	const repoId = selection?.repoId ?? fallbackRepoId;
	const parentDirPath = selection?.kind === 'dir' ? selection.path : '';
	const { nextExpandedNodes, dirPathsToLoad, insertAfterKey } = expandForParent(
		repoId,
		parentDirPath,
		expandedNodes,
	);
	return {
		inlineCreate: {
			repoId,
			parentDirPath,
			insertAfterKey,
			depth: getFileDepthForParent(parentDirPath),
			draftName: '',
			creating: false,
		},
		nextExpandedNodes,
		dirPathsToLoad,
		shouldSelectRepo: selection === null,
	};
};

export const validateInlineCreateFileName = (fileName: string): string | null => {
	if (fileName.length === 0) return 'Enter a filename before creating the file.';
	if (fileName === '.' || fileName === '..' || fileName.includes('/') || fileName.includes('\\')) {
		return 'Enter a filename only. Paths are inferred from the current selection.';
	}
	return null;
};
