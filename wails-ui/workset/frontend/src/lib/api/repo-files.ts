import type {
	RepoFileContent,
	RepoFileDefinitionResult,
	RepoFileHoverResult,
	RepoFileSearchResult,
	RepoImageContent,
	WorkspaceExtraRoot,
} from '../types';
import { Call } from '@wailsio/runtime';
import {
	CreateWorkspaceRepoFile,
	DeleteWorkspaceRepoFile,
	GetRepoBlame,
	ListRepoDirectory,
	ListWorkspaceExtraRoots,
	ReadWorkspaceRepoFile,
	ReadWorkspaceRepoFileAtRef,
	ReadWorkspaceRepoImageBase64,
	SearchWorkspaceRepoFiles,
	WriteWorkspaceRepoFile,
} from '../../../bindings/workset/app';
import { RepoDirectoryEntry } from '../../../bindings/workset/models';
import { LruTtlCache } from '../cache/lruTtlCache';
import { Fzf, type FzfResultItem } from 'fzf';

const REPO_FILE_INDEX_LIMIT = 5000;
const REPO_FILE_CACHE_TTL_MS = 10_000;

type RepoFileSearchCacheEntry = {
	items: RepoFileSearchResult[];
	fzf: Fzf<RepoFileSearchResult[]>;
	loadedAt: number;
};

const repoFileSearchCache = new Map<string, RepoFileSearchCacheEntry>();

const cacheKeyFor = (workspaceId: string, repoId?: string): string =>
	`${workspaceId.trim()}::${repoId?.trim() ?? '*'}`;

const buildFzfIndex = (items: RepoFileSearchResult[]): Fzf<RepoFileSearchResult[]> =>
	new Fzf(items, { selector: (item) => item.path });

const fzfSearch = (
	fzf: Fzf<RepoFileSearchResult[]>,
	items: RepoFileSearchResult[],
	query: string,
	limit: number,
): RepoFileSearchResult[] => {
	const trimmed = query.trim();
	if (trimmed.length === 0) return items.slice(0, Math.max(1, limit));
	const results: FzfResultItem<RepoFileSearchResult>[] = fzf.find(trimmed);
	return results.slice(0, Math.max(1, limit)).map((entry) => entry.item);
};

const isCacheFresh = (
	entry: RepoFileSearchCacheEntry | undefined,
): entry is RepoFileSearchCacheEntry =>
	Boolean(entry && Date.now() - entry.loadedAt < REPO_FILE_CACHE_TTL_MS);

export function clearRepoFileSearchCache(): void {
	repoFileSearchCache.clear();
}

const extraRootCache = new LruTtlCache<WorkspaceExtraRoot[]>({
	maxEntries: 20,
	maxBytes: 64 * 1024,
	ttlMs: 30_000,
	softTtlMs: 10_000,
	sizeOf: (roots) => JSON.stringify(roots).length,
});

export async function listWorkspaceExtraRoots(workspaceId: string): Promise<WorkspaceExtraRoot[]> {
	const key = workspaceId.trim();
	const cached = extraRootCache.get(key);
	if (cached && !cached.stale) return cached.value;

	const result = (await ListWorkspaceExtraRoots(workspaceId)) as WorkspaceExtraRoot[] | undefined;
	const roots = result ?? [];
	extraRootCache.set(key, roots);
	return roots;
}

export function invalidateWorkspaceExtraRoots(workspaceId: string): void {
	extraRootCache.remove(workspaceId.trim());
}

export function clearWorkspaceExtraRootsCache(): void {
	extraRootCache.clear();
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
		return fzfSearch(cached.fzf, cached.items, query, limit);
	}

	const payload = (await SearchWorkspaceRepoFiles({
		workspaceId,
		repoId,
		query: '',
		limit: REPO_FILE_INDEX_LIMIT,
	})) as RepoFileSearchResult[] | undefined;
	const items = payload ?? [];
	const fzf = buildFzfIndex(items);
	repoFileSearchCache.set(key, {
		items,
		fzf,
		loadedAt: Date.now(),
	});
	return fzfSearch(fzf, items, query, limit);
}

// ── File content caches ──────────────────────────────────

const fileContentCache = new LruTtlCache<RepoFileContent>({
	maxEntries: 50,
	maxBytes: 5 * 1024 * 1024,
	ttlMs: 60_000,
	softTtlMs: 10_000,
	sizeOf: (fc) => (fc.content?.length ?? 0) + 128,
});

const fileAtRefCache = new LruTtlCache<RepoFileAtRefResult>({
	maxEntries: 50,
	maxBytes: 5 * 1024 * 1024,
	ttlMs: 5 * 60_000,
	softTtlMs: 30_000,
	sizeOf: (r) => (r.content?.length ?? 0) + 64,
});

const fileContentKey = (wsId: string, repoId: string, path: string): string =>
	`${wsId}|${repoId}|${path}`;

const fileAtRefKey = (wsId: string, repoId: string, path: string, ref: string): string =>
	`${wsId}|${repoId}|${path}|${ref}`;

export async function readWorkspaceRepoFile(
	workspaceId: string,
	repoId: string,
	path: string,
): Promise<RepoFileContent> {
	const key = fileContentKey(workspaceId, repoId, path);
	const cached = fileContentCache.get(key);
	if (cached && !cached.stale) return cached.value;

	const result = (await ReadWorkspaceRepoFile({
		workspaceId,
		repoId,
		path,
	})) as RepoFileContent;
	fileContentCache.set(key, result);
	return result;
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

export async function getRepoFileHover(input: {
	workspaceId: string;
	repoId: string;
	path: string;
	content: string;
	line: number;
	character: number;
}): Promise<RepoFileHoverResult> {
	return (await Call.ByName('main.App.GetRepoFileHover', input)) as RepoFileHoverResult;
}

export async function getRepoFileDefinition(input: {
	workspaceId: string;
	repoId: string;
	path: string;
	content: string;
	line: number;
	character: number;
}): Promise<RepoFileDefinitionResult> {
	return (await Call.ByName('main.App.GetRepoFileDefinition', input)) as RepoFileDefinitionResult;
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
	const result = (await WriteWorkspaceRepoFile({
		workspaceId,
		repoId,
		path,
		content,
	})) as RepoFileWriteResult;
	// Invalidate cached content for this file after a successful write
	if (result.written) {
		invalidateFileContent(workspaceId, repoId, path);
	}
	return result;
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
	const key = fileAtRefKey(workspaceId, repoId, path, ref);
	const cached = fileAtRefCache.get(key);
	if (cached && !cached.stale) return cached.value;

	const result = (await ReadWorkspaceRepoFileAtRef({
		workspaceId,
		repoId,
		path,
		ref,
	})) as RepoFileAtRefResult;
	fileAtRefCache.set(key, result);
	return result;
}

/** Invalidate cached content for a specific file (both working copy and all refs). */
export function invalidateFileContent(wsId: string, repoId: string, path: string): void {
	fileContentCache.remove(fileContentKey(wsId, repoId, path));
	fileAtRefCache.removeByPrefix(`${wsId}|${repoId}|${path}|`);
}

/** Invalidate all cached content for a repo (e.g. on diff summary change). */
export function invalidateRepoFileContent(wsId: string, repoId: string): void {
	fileContentCache.removeByPrefix(`${wsId}|${repoId}|`);
	fileAtRefCache.removeByPrefix(`${wsId}|${repoId}|`);
}

/** Clear all file content caches (e.g. on workspace switch). */
export function clearFileContentCache(): void {
	fileContentCache.clear();
	fileAtRefCache.clear();
}

// ── Lazy directory listing ───────────────────────────────

const dirListCache = new LruTtlCache<RepoDirectoryEntry[]>({
	maxEntries: 200,
	maxBytes: 1 * 1024 * 1024,
	ttlMs: 30_000,
	softTtlMs: 10_000,
	sizeOf: (entries) => JSON.stringify(entries).length,
});

const dirListKey = (wsId: string, repoId: string, dirPath: string): string =>
	`${wsId}|${repoId}|dir:${dirPath}`;

export async function listRepoDirectory(
	workspaceId: string,
	repoId: string,
	dirPath: string,
): Promise<RepoDirectoryEntry[]> {
	const key = dirListKey(workspaceId, repoId, dirPath);
	const cached = dirListCache.get(key);
	if (cached && !cached.stale) return cached.value;

	const result = (await ListRepoDirectory({
		workspaceId,
		repoId,
		dirPath,
	})) as RepoDirectoryEntry[];
	dirListCache.set(key, result ?? []);
	return result ?? [];
}

/** Invalidate directory listing cache for a repo (e.g. on diff event). */
export function invalidateRepoDirCache(wsId: string, repoId: string): void {
	dirListCache.removeByPrefix(`${wsId}|${repoId}|`);
}

/** Clear all directory listing caches. */
export function clearDirListCache(): void {
	dirListCache.clear();
}

export { RepoDirectoryEntry };

// ── Git Blame ────────────────────────────────────────────

export type BlameEntry = {
	startLine: number;
	endLine: number;
	commitHash: string;
	author: string;
	authorDate: string;
	summary: string;
};

export async function getRepoBlame(
	workspaceId: string,
	repoId: string,
	path: string,
	ref = 'HEAD',
): Promise<BlameEntry[]> {
	const result = await GetRepoBlame({ workspaceId, repoId, path, ref });
	return (result as unknown as BlameEntry[]) ?? [];
}

// ── File Creation / Deletion ─────────────────────────────

export type RepoFileDeleteResult = { deleted: boolean; error?: string };

export async function createWorkspaceRepoFile(
	workspaceId: string,
	repoId: string,
	path: string,
	content = '',
): Promise<RepoFileWriteResult> {
	const result = await CreateWorkspaceRepoFile({ workspaceId, repoId, path, content });
	return result as unknown as RepoFileWriteResult;
}

export async function deleteWorkspaceRepoFile(
	workspaceId: string,
	repoId: string,
	path: string,
): Promise<RepoFileDeleteResult> {
	const result = await DeleteWorkspaceRepoFile({ workspaceId, repoId, path });
	return result as unknown as RepoFileDeleteResult;
}
