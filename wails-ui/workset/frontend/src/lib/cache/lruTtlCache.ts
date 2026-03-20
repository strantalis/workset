export type CacheHit<T> = {
	value: T;
	stale: boolean;
};

type CacheEntry<T> = {
	value: T;
	size: number;
	createdAt: number;
	expiresAt: number;
};

export type CacheOptions<T> = {
	maxEntries: number;
	maxBytes: number;
	ttlMs: number;
	softTtlMs: number;
	sizeOf: (value: T) => number;
};

export class LruTtlCache<T> {
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

	remove(key: string): void {
		this.delete(key);
	}

	/** Remove all entries whose key starts with the given prefix. */
	removeByPrefix(prefix: string): void {
		for (const key of [...this.entries.keys()]) {
			if (key.startsWith(prefix)) this.delete(key);
		}
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
