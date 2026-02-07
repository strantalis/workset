export type RepoDiffWatchParams = {
	workspaceId: string;
	repoId: string;
	prNumber?: number;
	prBranch?: string;
};

export type RepoDiffWatcherLifecycleDependencies = {
	startWatch: (
		workspaceId: string,
		repoId: string,
		prNumber?: number,
		prBranch?: string,
	) => Promise<unknown>;
	updateWatch: (
		workspaceId: string,
		repoId: string,
		prNumber?: number,
		prBranch?: string,
	) => Promise<unknown>;
	stopWatch: (workspaceId: string, repoId: string) => Promise<unknown>;
};

export type RepoDiffWatcherLifecycle = {
	syncLifecycle: (params: RepoDiffWatchParams) => void;
	syncUpdate: (params: RepoDiffWatchParams) => void;
	dispose: () => void;
};

export const createRepoDiffWatcherLifecycle = (
	dependencies: RepoDiffWatcherLifecycleDependencies,
): RepoDiffWatcherLifecycle => {
	let watchActive = false;
	let watchStarting = false;
	let lastRepoId = '';
	let lastWorkspaceId = '';
	let latestParams: RepoDiffWatchParams | null = null;

	const applyUpdate = (): void => {
		if (!watchActive || !latestParams || !latestParams.repoId) return;
		void dependencies.updateWatch(
			latestParams.workspaceId,
			latestParams.repoId,
			latestParams.prNumber,
			latestParams.prBranch,
		);
	};

	const syncLifecycle = (params: RepoDiffWatchParams): void => {
		latestParams = params;
		const { workspaceId, repoId } = params;

		if (!repoId) {
			if (lastRepoId && lastWorkspaceId) {
				void dependencies.stopWatch(lastWorkspaceId, lastRepoId);
				watchActive = false;
				watchStarting = false;
				lastRepoId = '';
				lastWorkspaceId = '';
			}
			return;
		}

		const repoChanged = lastRepoId !== '' && lastRepoId !== repoId;
		const workspaceChanged = lastWorkspaceId !== '' && lastWorkspaceId !== workspaceId;
		if ((repoChanged || workspaceChanged) && lastRepoId && lastWorkspaceId) {
			void dependencies.stopWatch(lastWorkspaceId, lastRepoId);
			watchActive = false;
			watchStarting = false;
		}

		if ((!watchActive || repoChanged || workspaceChanged) && !watchStarting) {
			const startWorkspaceId = workspaceId;
			const startRepoId = repoId;
			const startPrNumber = params.prNumber;
			const startPrBranch = params.prBranch;

			watchStarting = true;
			void dependencies
				.startWatch(startWorkspaceId, startRepoId, startPrNumber, startPrBranch)
				.then(() => {
					if (lastRepoId === startRepoId && lastWorkspaceId === startWorkspaceId) {
						watchActive = true;
						applyUpdate();
					}
				})
				.catch(() => {
					if (lastRepoId === startRepoId && lastWorkspaceId === startWorkspaceId) {
						watchActive = false;
					}
				})
				.finally(() => {
					watchStarting = false;
				});
		}

		lastRepoId = repoId;
		lastWorkspaceId = workspaceId;
	};

	const syncUpdate = (params: RepoDiffWatchParams): void => {
		latestParams = params;
		applyUpdate();
	};

	const dispose = (): void => {
		if (lastRepoId && lastWorkspaceId) {
			void dependencies.stopWatch(lastWorkspaceId, lastRepoId);
		}
	};

	return {
		syncLifecycle,
		syncUpdate,
		dispose,
	};
};
