import type { RepoFileContent, RepoFileSearchResult, RepoImageContent } from '../types';
import {
	ReadWorkspaceRepoFile,
	ReadWorkspaceRepoFileAtRef,
	ReadWorkspaceRepoImageBase64,
	SearchWorkspaceRepoFiles,
	WriteWorkspaceRepoFile,
} from '../../../bindings/workset/app';

const REPO_FILE_INDEX_LIMIT = 5000;
const REPO_FILE_CACHE_TTL_MS = 10_000;

type RepoFileSearchCacheEntry = {
	items: RepoFileSearchResult[];
	loadedAt: number;
};

const repoFileSearchCache = new Map<string, RepoFileSearchCacheEntry>();

const cacheKeyFor = (workspaceId: string, repoId?: string): string =>
	`${workspaceId.trim()}::${repoId?.trim() ?? '*'}`;

const scoreResult = (result: RepoFileSearchResult, query: string): number => {
	const queryFolded = query.trim().toLowerCase();
	if (queryFolded.length === 0) return result.score;
	const path = result.path.toLowerCase();
	const repoPath = `${result.repoName}/${result.path}`.toLowerCase();
	if (path.startsWith(queryFolded)) return 3;
	if (path.includes(queryFolded)) return 2;
	if (repoPath.includes(queryFolded)) return 1;
	return 0;
};

const filterAndRankResults = (
	items: RepoFileSearchResult[],
	query: string,
	limit: number,
): RepoFileSearchResult[] => {
	const queryFolded = query.trim().toLowerCase();
	const filtered =
		queryFolded.length === 0
			? [...items]
			: items.filter((item) => scoreResult(item, queryFolded) > 0);

	filtered.sort((left, right) => {
		const leftScore = scoreResult(left, queryFolded);
		const rightScore = scoreResult(right, queryFolded);
		if (leftScore !== rightScore) return rightScore - leftScore;
		if (left.repoName !== right.repoName) return left.repoName.localeCompare(right.repoName);
		return left.path.localeCompare(right.path);
	});

	return filtered.slice(0, Math.max(1, limit));
};

const isCacheFresh = (
	entry: RepoFileSearchCacheEntry | undefined,
): entry is RepoFileSearchCacheEntry =>
	Boolean(entry && Date.now() - entry.loadedAt < REPO_FILE_CACHE_TTL_MS);

export function clearRepoFileSearchCache(): void {
	repoFileSearchCache.clear();
}

export async function searchWorkspaceRepoFiles(
	workspaceId: string,
	query: string,
	limit = 250,
	repoId?: string,
): Promise<RepoFileSearchResult[]> {
	const key = cacheKeyFor(workspaceId, repoId);
	const cached = repoFileSearchCache.get(key);
	if (isCacheFresh(cached)) {
		return filterAndRankResults(cached.items, query, limit);
	}

	const payload = (await SearchWorkspaceRepoFiles({
		workspaceId,
		repoId,
		query: '',
		limit: REPO_FILE_INDEX_LIMIT,
	})) as RepoFileSearchResult[] | undefined;
	const items = payload ?? [];
	repoFileSearchCache.set(key, {
		items,
		loadedAt: Date.now(),
	});
	return filterAndRankResults(items, query, limit);
}

export async function readWorkspaceRepoFile(
	workspaceId: string,
	repoId: string,
	path: string,
): Promise<RepoFileContent> {
	return (await ReadWorkspaceRepoFile({
		workspaceId,
		repoId,
		path,
	})) as RepoFileContent;
}

export async function readWorkspaceRepoImageBase64(
	workspaceId: string,
	repoId: string,
	path: string,
): Promise<RepoImageContent> {
	return (await ReadWorkspaceRepoImageBase64({
		workspaceId,
		repoId,
		path,
	})) as RepoImageContent;
}

export type RepoFileWriteResult = {
	written: boolean;
	error?: string;
};

export async function writeWorkspaceRepoFile(
	workspaceId: string,
	repoId: string,
	path: string,
	content: string,
): Promise<RepoFileWriteResult> {
	return (await WriteWorkspaceRepoFile({
		workspaceId,
		repoId,
		path,
		content,
	})) as RepoFileWriteResult;
}

export type RepoFileAtRefResult = {
	content: string;
	found: boolean;
	binary: boolean;
};

export async function readWorkspaceRepoFileAtRef(
	workspaceId: string,
	repoId: string,
	path: string,
	ref = 'HEAD',
): Promise<RepoFileAtRefResult> {
	return (await ReadWorkspaceRepoFileAtRef({
		workspaceId,
		repoId,
		path,
		ref,
	})) as RepoFileAtRefResult;
}
