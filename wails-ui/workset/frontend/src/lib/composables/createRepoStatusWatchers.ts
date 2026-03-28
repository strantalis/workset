import {
	startRepoDiffWatch,
	startRepoStatusWatch,
	stopRepoDiffWatch,
	stopRepoStatusWatch,
	updateRepoDiffWatch,
} from '../api/repo-diff';
import type { Workspace } from '../types';

export type RepoStatusWatcherManager = {
	sync: (workspaces: Workspace[]) => void;
	stopAll: () => void;
};

type WatchMode = 'local' | 'full';

type WatchEntry = {
	workspaceId: string;
	repoId: string;
	mode: WatchMode;
	prNumber?: number;
	prBranch?: string;
};

const buildEntrySignature = (entries: Map<string, WatchEntry>): string =>
	Array.from(entries.entries())
		.sort(([left], [right]) => left.localeCompare(right))
		.map(([key, entry]) =>
			[
				key,
				entry.workspaceId,
				entry.repoId,
				entry.mode,
				entry.prNumber ?? 0,
				entry.prBranch ?? '',
			].join('|'),
		)
		.join('\n');

const getTrackedPrWatch = (
	repo: Workspace['repos'][number],
): { prNumber: number; prBranch: string } | null => {
	const tracked = repo.trackedPullRequest;
	if (!tracked) return null;
	const state = tracked.state.toLowerCase();
	const merged = tracked.merged === true || state === 'merged';
	if (state !== 'open' || merged) return null;
	return {
		prNumber: tracked.number,
		prBranch: tracked.headBranch,
	};
};

const startWatch = async (entry: WatchEntry): Promise<void> => {
	if (entry.mode === 'full') {
		await startRepoDiffWatch(entry.workspaceId, entry.repoId, entry.prNumber, entry.prBranch);
		return;
	}
	await startRepoStatusWatch(entry.workspaceId, entry.repoId);
};

const stopWatch = async (entry: WatchEntry): Promise<void> => {
	if (entry.mode === 'full') {
		await stopRepoDiffWatch(entry.workspaceId, entry.repoId);
		return;
	}
	await stopRepoStatusWatch(entry.workspaceId, entry.repoId);
};

export function createRepoStatusWatchers(): RepoStatusWatcherManager {
	const watchers = new Map<string, WatchEntry>();

	// Safety net: detect runaway sync cycles within a single microtask flush.
	// Runtime patches must not change which repos are watched — only metadata
	// (status, diff counts, PR state). If that invariant is violated, the
	// event→patch→derived→sync chain could loop. This counter catches it.
	// Limit is 20 because rapid workspace switching legitimately fires multiple
	// sync calls per tick (each switch: activeId → warmIds → watchedWs → sync).
	let syncBurstCount = 0;
	let syncBurstTimer: ReturnType<typeof setTimeout> | null = null;
	const MAX_SYNC_BURST = 20;

	const sync = (workspaces: Workspace[]): void => {
		syncBurstCount++;
		if (!syncBurstTimer) {
			syncBurstTimer = setTimeout(() => {
				syncBurstCount = 0;
				syncBurstTimer = null;
			}, 0);
		}
		if (syncBurstCount > MAX_SYNC_BURST) {
			// eslint-disable-next-line no-console -- intentional safety net for reactive loop detection
			console.warn(
				`[RepoStatusWatchers] sync() called ${syncBurstCount} times in one tick — possible reactive loop, skipping`,
			);
			return;
		}

		const nextEntries = new Map<string, WatchEntry>();
		for (const workspace of workspaces) {
			if (workspace.archived) continue;
			if (workspace.placeholder) continue;
			for (const repo of workspace.repos) {
				const key = `${workspace.id}:${repo.id}`;
				const trackedWatch = getTrackedPrWatch(repo);
				const nextEntry: WatchEntry = trackedWatch
					? {
							workspaceId: workspace.id,
							repoId: repo.id,
							mode: 'full',
							prNumber: trackedWatch.prNumber,
							prBranch: trackedWatch.prBranch,
						}
					: {
							workspaceId: workspace.id,
							repoId: repo.id,
							mode: 'local',
						};
				nextEntries.set(key, nextEntry);

				const existing = watchers.get(key);
				if (!existing) {
					watchers.set(key, nextEntry);
					void startWatch(nextEntry).catch(() => {
						watchers.delete(key);
					});
					continue;
				}

				if (existing.mode !== nextEntry.mode) {
					watchers.set(key, nextEntry);
					void stopWatch(existing)
						.then(() => startWatch(nextEntry))
						.catch(() => {
							watchers.delete(key);
						});
					continue;
				}

				if (
					nextEntry.mode === 'full' &&
					(existing.prNumber !== nextEntry.prNumber || existing.prBranch !== nextEntry.prBranch)
				) {
					watchers.set(key, nextEntry);
					void updateRepoDiffWatch(
						nextEntry.workspaceId,
						nextEntry.repoId,
						nextEntry.prNumber,
						nextEntry.prBranch,
					).catch(() => {
						watchers.delete(key);
					});
				}
			}
		}

		if (buildEntrySignature(watchers) === buildEntrySignature(nextEntries)) {
			return;
		}

		for (const [key, entry] of watchers.entries()) {
			if (nextEntries.has(key)) continue;
			watchers.delete(key);
			void stopWatch(entry).catch(() => {});
		}
	};

	const stopAll = (): void => {
		for (const watcher of watchers.values()) {
			void stopWatch(watcher).catch(() => {});
		}
		watchers.clear();
	};

	return { sync, stopAll };
}
