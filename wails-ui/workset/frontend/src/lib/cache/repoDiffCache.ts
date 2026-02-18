import type { RepoDiffSummary, RepoFileDiff } from '../types';

type CacheHit<T> = {
	value: T;
	stale: boolean;
};

type CacheEntry<T> = {
	value: T;
	size: number;
	createdAt: number;
	expiresAt: number;
};

type CacheOptions<T> = {
	maxEntries: number;
	maxBytes: number;
	ttlMs: number;
	softTtlMs: number;
	sizeOf: (value: T) => number;
};

class LruTtlCache<T> {
	private readonly entries = new Map<string, CacheEntry<T>>();
	private totalBytes = 0;

	constructor(private readonly options: CacheOptions<T>) {}

	get(key: string): CacheHit<T> | null {
		const entry = this.entries.get(key);
		if (!entry) return null;
		const now = Date.now();
		if (entry.expiresAt <= now) {
			this.delete(key);
			return null;
		}
		this.entries.delete(key);
		this.entries.set(key, entry);
		return {
			value: entry.value,
			stale: now - entry.createdAt >= this.options.softTtlMs,
		};
	}

	set(key: string, value: T): void {
		const now = Date.now();
		const size = this.options.sizeOf(value);
		if (size <= 0 || size > this.options.maxBytes) return;

		const existing = this.entries.get(key);
		if (existing) {
			this.totalBytes -= existing.size;
			this.entries.delete(key);
		}

		this.entries.set(key, {
			value,
			size,
			createdAt: now,
			expiresAt: now + this.options.ttlMs,
		});
		this.totalBytes += size;
		this.evict();
	}

	clear(): void {
		this.entries.clear();
		this.totalBytes = 0;
	}

	private delete(key: string): void {
		const entry = this.entries.get(key);
		if (!entry) return;
		this.totalBytes -= entry.size;
		this.entries.delete(key);
	}

	private evict(): void {
		while (this.entries.size > this.options.maxEntries || this.totalBytes > this.options.maxBytes) {
			const oldest = this.entries.keys().next().value as string | undefined;
			if (!oldest) return;
			this.delete(oldest);
		}
	}
}

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
	clear: (): void => {
		summaryCache.clear();
		fileDiffCache.clear();
	},
};
