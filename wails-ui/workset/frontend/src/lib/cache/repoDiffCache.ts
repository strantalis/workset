import type { RepoDiffSummary, RepoFileDiff } from '../types';
import { LruTtlCache, type CacheHit } from './lruTtlCache';

const DEFAULT_TTL_MS = 5 * 60 * 1000;
const DEFAULT_SOFT_TTL_MS = 45 * 1000;

const summaryCache = new LruTtlCache<RepoDiffSummary>({
	maxEntries: 50,
	maxBytes: 2 * 1024 * 1024,
	ttlMs: DEFAULT_TTL_MS,
	softTtlMs: DEFAULT_SOFT_TTL_MS,
	sizeOf: (summary) => JSON.stringify(summary).length,
});

const fileDiffCache = new LruTtlCache<RepoFileDiff>({
	maxEntries: 250,
	maxBytes: 10 * 1024 * 1024,
	ttlMs: DEFAULT_TTL_MS,
	softTtlMs: DEFAULT_SOFT_TTL_MS,
	sizeOf: (diff) => (diff.patch?.length ?? 0) + 128,
});

export const buildSummaryLocalCacheKey = (wsId: string, repoId: string): string =>
	`${wsId}|${repoId}|local`;

export const buildSummaryPrCacheKey = (
	wsId: string,
	repoId: string,
	base: string,
	head: string,
): string => `${wsId}|${repoId}|base|${base}|head|${head}`;

export const buildFileLocalCacheKey = (
	wsId: string,
	repoId: string,
	status: string,
	path: string,
	prevPath: string,
): string => `${wsId}|${repoId}|local|${status}|${path}|${prevPath}`;

export const buildFilePrCacheKey = (
	wsId: string,
	repoId: string,
	base: string,
	head: string,
	path: string,
	prevPath: string,
): string => `${wsId}|${repoId}|pr|${base}|${head}|${path}|${prevPath}`;

export const repoDiffCache = {
	getSummary: (key: string): CacheHit<RepoDiffSummary> | null => summaryCache.get(key),
	setSummary: (key: string, value: RepoDiffSummary): void => summaryCache.set(key, value),
	getFileDiff: (key: string): CacheHit<RepoFileDiff> | null => fileDiffCache.get(key),
	setFileDiff: (key: string, value: RepoFileDiff): void => fileDiffCache.set(key, value),
	deleteFileDiff: (key: string): void => fileDiffCache.remove(key),
	clear: (): void => {
		summaryCache.clear();
		fileDiffCache.clear();
	},
};
