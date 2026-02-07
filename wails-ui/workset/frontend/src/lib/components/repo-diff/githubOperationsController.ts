import type {
	GitHubOperationStage,
	GitHubOperationStatus,
	PullRequestCreated,
	PullRequestStatusResult,
} from '../../api';
import type { PrCreateStage } from '../../prCreateProgress';

type GitHubAction = () => Promise<void>;
type PendingGitHubAction = (() => Promise<void>) | null;

type GitHubOperationsControllerOptions = {
	workspaceId: () => string;
	repoId: () => string;
	prBase: () => string;
	prBaseRemote: () => string;
	prDraft: () => boolean;
	prCreating: () => boolean;
	commitPushLoading: () => boolean;
	authModalOpen: () => boolean;
	getAuthPendingAction: () => PendingGitHubAction;
	setAuthModalOpen: (value: boolean) => void;
	setAuthModalMessage: (value: string | null) => void;
	setAuthPendingAction: (value: PendingGitHubAction) => void;
	setPrPanelExpanded: (value: boolean) => void;
	setPrCreating: (value: boolean) => void;
	setPrCreatingStage: (value: PrCreateStage | null) => void;
	setPrCreateError: (value: string | null) => void;
	setPrCreateSuccess: (value: PullRequestCreated | null) => void;
	setPrTracked: (value: PullRequestCreated | null) => void;
	setForceMode: (value: 'create' | 'status' | null) => void;
	setPrNumberInput: (value: string) => void;
	setPrStatus: (value: PullRequestStatusResult | null) => void;
	setCommitPushLoading: (value: boolean) => void;
	setCommitPushStage: (value: GitHubOperationStage | null) => void;
	setCommitPushError: (value: string | null) => void;
	setCommitPushSuccess: (value: boolean) => void;
	handledOperationCompletions: Set<string>;
	handleRefresh: () => Promise<void>;
	formatError: (error: unknown, fallback: string) => string;
	startCreatePullRequestAsync: (
		workspaceId: string,
		repoId: string,
		payload: { base?: string; baseRemote?: string; draft: boolean },
	) => Promise<GitHubOperationStatus>;
	startCommitAndPushAsync: (workspaceId: string, repoId: string) => Promise<GitHubOperationStatus>;
	fetchGitHubOperationStatus: (
		workspaceId: string,
		repoId: string,
		type: 'create_pr' | 'commit_push',
	) => Promise<GitHubOperationStatus | null>;
};

const authRequiredPrefix = 'AUTH_REQUIRED:';

const isAuthRequiredMessage = (message: string): boolean => message.startsWith(authRequiredPrefix);

const stripAuthPrefix = (message: string): string =>
	message.replace(/^AUTH_REQUIRED:\s*/, '') || 'GitHub authentication required.';

const toPrCreateStage = (stage: GitHubOperationStage): PrCreateStage | null => {
	if (stage === 'queued' || stage === 'generating') return 'generating';
	if (stage === 'creating') return 'creating';
	return null;
};

export const createGitHubOperationsController = (options: GitHubOperationsControllerOptions) => {
	const runGitHubAction = async (
		action: GitHubAction,
		onError: (message: string) => void,
		fallback: string,
	): Promise<void> => {
		if (options.authModalOpen()) {
			options.setAuthPendingAction(() => runGitHubAction(action, onError, fallback));
			return;
		}

		try {
			await action();
		} catch (error) {
			const message = options.formatError(error, fallback);
			if (isAuthRequiredMessage(message)) {
				options.setAuthModalMessage(stripAuthPrefix(message));
				options.setAuthPendingAction(() => runGitHubAction(action, onError, fallback));
				options.setAuthModalOpen(true);
				return;
			}
			onError(message);
		}
	};

	const handleAuthSuccess = async (): Promise<void> => {
		options.setAuthModalOpen(false);
		options.setAuthModalMessage(null);
		const pendingAction = options.getAuthPendingAction();
		options.setAuthPendingAction(null);
		if (pendingAction) {
			await pendingAction();
		}
	};

	const handleAuthClose = (): void => {
		options.setAuthModalOpen(false);
		options.setAuthPendingAction(null);
	};

	const applyGitHubOperationStatus = (status: GitHubOperationStatus): void => {
		if (status.workspaceId !== options.workspaceId() || status.repoId !== options.repoId()) return;

		if (status.type === 'create_pr') {
			if (status.state === 'running') {
				options.setPrPanelExpanded(true);
				options.setPrCreating(true);
				options.setPrCreatingStage(toPrCreateStage(status.stage));
				options.setPrCreateError(null);
				return;
			}

			options.setPrCreating(false);
			options.setPrCreatingStage(null);
			if (status.state === 'completed') {
				options.setPrCreateError(null);
				if (status.pullRequest) {
					options.setPrCreateSuccess(status.pullRequest);
					options.setPrTracked(status.pullRequest);
					options.setForceMode(null);
					options.setPrNumberInput(`${status.pullRequest.number}`);
					options.setPrStatus({
						pullRequest: status.pullRequest,
						checks: [],
					});
				}
				if (!options.handledOperationCompletions.has(status.operationId)) {
					options.handledOperationCompletions.add(status.operationId);
					void options.handleRefresh();
				}
				return;
			}

			if (status.state === 'failed') {
				options.setPrCreateError(status.error || 'Failed to create pull request.');
			}
			return;
		}

		if (status.type !== 'commit_push') return;

		if (status.state === 'running') {
			options.setCommitPushLoading(true);
			options.setCommitPushStage(status.stage);
			options.setCommitPushError(null);
			options.setCommitPushSuccess(false);
			return;
		}

		options.setCommitPushLoading(false);
		options.setCommitPushStage(null);
		if (status.state === 'completed') {
			options.setCommitPushError(null);
			options.setCommitPushSuccess(true);
			if (!options.handledOperationCompletions.has(status.operationId)) {
				options.handledOperationCompletions.add(status.operationId);
				void options.handleRefresh();
			}
			return;
		}

		if (status.state === 'failed') {
			options.setCommitPushSuccess(false);
			options.setCommitPushError(status.error || 'Failed to commit and push.');
		}
	};

	const loadGitHubOperationStatuses = async (): Promise<void> => {
		const currentRepoId = options.repoId();
		if (!currentRepoId) return;
		try {
			const [createStatus, commitStatus] = await Promise.all([
				options.fetchGitHubOperationStatus(options.workspaceId(), currentRepoId, 'create_pr'),
				options.fetchGitHubOperationStatus(options.workspaceId(), currentRepoId, 'commit_push'),
			]);
			if (createStatus) {
				applyGitHubOperationStatus(createStatus);
			}
			if (commitStatus) {
				applyGitHubOperationStatus(commitStatus);
			}
		} catch {
			// best effort recovery, ignore status load errors
		}
	};

	const handleCommitAndPush = async (): Promise<void> => {
		const currentRepoId = options.repoId();
		if (!currentRepoId || options.commitPushLoading()) return;

		options.setCommitPushLoading(true);
		options.setCommitPushStage('queued');
		options.setCommitPushError(null);
		options.setCommitPushSuccess(false);

		await runGitHubAction(
			async () => {
				const status = await options.startCommitAndPushAsync(options.workspaceId(), currentRepoId);
				applyGitHubOperationStatus(status);
			},
			(message) => {
				options.setCommitPushLoading(false);
				options.setCommitPushStage(null);
				options.setCommitPushSuccess(false);
				options.setCommitPushError(message);
			},
			'Failed to commit and push.',
		);
	};

	const handleCreatePR = async (): Promise<void> => {
		const currentRepoId = options.repoId();
		if (!currentRepoId || options.prCreating()) return;

		options.setPrPanelExpanded(true);
		options.setPrCreating(true);
		options.setPrCreatingStage('generating');
		options.setPrCreateError(null);
		options.setPrCreateSuccess(null);

		await runGitHubAction(
			async () => {
				const status = await options.startCreatePullRequestAsync(
					options.workspaceId(),
					currentRepoId,
					{
						base: options.prBase().trim() || undefined,
						baseRemote: options.prBaseRemote() || undefined,
						draft: options.prDraft(),
					},
				);
				applyGitHubOperationStatus(status);
			},
			(message) => {
				options.setPrCreating(false);
				options.setPrCreatingStage(null);
				options.setPrCreateError(message);
			},
			'Failed to create pull request.',
		);
	};

	return {
		runGitHubAction,
		handleAuthSuccess,
		handleAuthClose,
		applyGitHubOperationStatus,
		loadGitHubOperationStatuses,
		handleCommitAndPush,
		handleCreatePR,
	};
};
