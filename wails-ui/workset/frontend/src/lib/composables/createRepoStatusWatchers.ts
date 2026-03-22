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

	const sync = (workspaces: Workspace[]): void => {
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
