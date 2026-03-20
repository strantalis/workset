export type WorkspaceActionMode =
	| 'create-thread'
	| 'add-repo'
	| 'remove-thread'
	| 'remove-repo'
	| null;

export type WorkspaceActionModalOptions = {
	worksetName?: string | null;
	worksetRepos?: string[];
	workspaceIds?: string[];
};

export type WorkspaceActionModalState = {
	readonly mode: WorkspaceActionMode;
	readonly workspaceId: string | null;
	readonly workspaceIds: string[];
	readonly repoName: string | null;
	readonly worksetName: string | null;
	readonly worksetRepos: string[];
	open: (
		mode: Exclude<WorkspaceActionMode, null>,
		workspaceId?: string | null,
		repoName?: string | null,
		options?: WorkspaceActionModalOptions,
	) => void;
	close: () => void;
};

export function createWorkspaceActionModal(): WorkspaceActionModalState {
	let mode = $state<WorkspaceActionMode>(null);
	let workspaceId = $state<string | null>(null);
	let workspaceIds = $state<string[]>([]);
	let repoName = $state<string | null>(null);
	let worksetName = $state<string | null>(null);
	let worksetRepos = $state<string[]>([]);

	const open = (
		nextMode: Exclude<WorkspaceActionMode, null>,
		nextWorkspaceId: string | null = null,
		nextRepoName: string | null = null,
		options: WorkspaceActionModalOptions = {},
	): void => {
		mode = nextMode;
		workspaceId = nextWorkspaceId;
		workspaceIds = options.workspaceIds ?? [];
		repoName = nextRepoName;
		worksetName = options.worksetName ?? null;
		worksetRepos = options.worksetRepos ?? [];
	};

	const close = (): void => {
		mode = null;
		workspaceId = null;
		workspaceIds = [];
		repoName = null;
		worksetName = null;
		worksetRepos = [];
	};

	return {
		get mode() {
			return mode;
		},
		get workspaceId() {
			return workspaceId;
		},
		get workspaceIds() {
			return workspaceIds;
		},
		get repoName() {
			return repoName;
		},
		get worksetName() {
			return worksetName;
		},
		get worksetRepos() {
			return worksetRepos;
		},
		open,
		close,
	};
}
