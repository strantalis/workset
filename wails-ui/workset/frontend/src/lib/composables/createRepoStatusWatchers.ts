import { startRepoStatusWatch, stopRepoStatusWatch } from '../api/repo-diff';
import type { Workspace } from '../types';

export type RepoStatusWatcherManager = {
	sync: (workspaces: Workspace[]) => void;
	stopAll: () => void;
};

export function createRepoStatusWatchers(): RepoStatusWatcherManager {
	const watchers = new Map<string, { workspaceId: string; repoId: string }>();

	const sync = (workspaces: Workspace[]): void => {
		const nextKeys = new Set<string>();
		for (const workspace of workspaces) {
			if (workspace.archived) continue;
			if (workspace.placeholder) continue;
			for (const repo of workspace.repos) {
				const key = `${workspace.id}:${repo.id}`;
				nextKeys.add(key);
				if (watchers.has(key)) continue;
				const entry = { workspaceId: workspace.id, repoId: repo.id };
				watchers.set(key, entry);
				void startRepoStatusWatch(workspace.id, repo.id).catch(() => {
					watchers.delete(key);
				});
			}
		}

		for (const [key, entry] of watchers.entries()) {
			if (nextKeys.has(key)) continue;
			watchers.delete(key);
			void stopRepoStatusWatch(entry.workspaceId, entry.repoId).catch(() => {});
		}
	};

	const stopAll = (): void => {
		for (const watcher of watchers.values()) {
			void stopRepoStatusWatch(watcher.workspaceId, watcher.repoId).catch(() => {});
		}
		watchers.clear();
	};

	return { sync, stopAll };
}
