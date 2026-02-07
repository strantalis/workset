import { describe, expect, it, vi } from 'vitest';
import type { GitHubOperationStage, GitHubOperationStatus } from '../../api/github';
import type { PullRequestCreated, PullRequestStatusResult } from '../../types';
import type { PrCreateStage } from '../../prCreateProgress';
import { createGitHubOperationsController } from './githubOperationsController';

type State = {
	prBase: string;
	prBaseRemote: string;
	prDraft: boolean;
	prCreating: boolean;
	commitPushLoading: boolean;
	authModalOpen: boolean;
	authModalMessage: string | null;
	authPendingAction: (() => Promise<void>) | null;
	prPanelExpanded: boolean;
	prCreatingStage: PrCreateStage | null;
	prCreateError: string | null;
	prCreateSuccess: PullRequestCreated | null;
	prTracked: PullRequestCreated | null;
	forceMode: 'create' | 'status' | null;
	prNumberInput: string;
	prStatus: PullRequestStatusResult | null;
	commitPushStage: GitHubOperationStage | null;
	commitPushError: string | null;
	commitPushSuccess: boolean;
};

const createSetup = () => {
	const state: State = {
		prBase: '',
		prBaseRemote: '',
		prDraft: false,
		prCreating: false,
		commitPushLoading: false,
		authModalOpen: false,
		authModalMessage: null,
		authPendingAction: null,
		prPanelExpanded: false,
		prCreatingStage: null,
		prCreateError: null,
		prCreateSuccess: null,
		prTracked: null,
		forceMode: null,
		prNumberInput: '',
		prStatus: null,
		commitPushStage: null,
		commitPushError: null,
		commitPushSuccess: false,
	};

	const handleRefresh = vi.fn(async () => undefined);
	const startCreatePullRequestAsync = vi.fn(
		async (): Promise<GitHubOperationStatus> => ({
			operationId: 'create-op',
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			type: 'create_pr',
			stage: 'queued',
			state: 'running',
			startedAt: new Date().toISOString(),
		}),
	);
	const startCommitAndPushAsync = vi.fn(
		async (): Promise<GitHubOperationStatus> => ({
			operationId: 'commit-op',
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			type: 'commit_push',
			stage: 'queued',
			state: 'running',
			startedAt: new Date().toISOString(),
		}),
	);
	const fetchGitHubOperationStatus = vi.fn(async (): Promise<GitHubOperationStatus | null> => null);

	const controller = createGitHubOperationsController({
		workspaceId: () => 'ws-1',
		repoId: () => 'repo-1',
		prBase: () => state.prBase,
		prBaseRemote: () => state.prBaseRemote,
		prDraft: () => state.prDraft,
		prCreating: () => state.prCreating,
		commitPushLoading: () => state.commitPushLoading,
		authModalOpen: () => state.authModalOpen,
		getAuthPendingAction: () => state.authPendingAction,
		setAuthModalOpen: (value) => {
			state.authModalOpen = value;
		},
		setAuthModalMessage: (value) => {
			state.authModalMessage = value;
		},
		setAuthPendingAction: (value) => {
			state.authPendingAction = value;
		},
		setPrPanelExpanded: (value) => {
			state.prPanelExpanded = value;
		},
		setPrCreating: (value) => {
			state.prCreating = value;
		},
		setPrCreatingStage: (value) => {
			state.prCreatingStage = value;
		},
		setPrCreateError: (value) => {
			state.prCreateError = value;
		},
		setPrCreateSuccess: (value) => {
			state.prCreateSuccess = value;
		},
		setPrTracked: (value) => {
			state.prTracked = value;
		},
		setForceMode: (value) => {
			state.forceMode = value;
		},
		setPrNumberInput: (value) => {
			state.prNumberInput = value;
		},
		setPrStatus: (value) => {
			state.prStatus = value;
		},
		setCommitPushLoading: (value) => {
			state.commitPushLoading = value;
		},
		setCommitPushStage: (value) => {
			state.commitPushStage = value;
		},
		setCommitPushError: (value) => {
			state.commitPushError = value;
		},
		setCommitPushSuccess: (value) => {
			state.commitPushSuccess = value;
		},
		handledOperationCompletions: new Set<string>(),
		handleRefresh,
		formatError: (error, fallback) => (error instanceof Error ? error.message : fallback),
		startCreatePullRequestAsync,
		startCommitAndPushAsync,
		fetchGitHubOperationStatus,
	});

	return {
		state,
		handleRefresh,
		startCreatePullRequestAsync,
		startCommitAndPushAsync,
		fetchGitHubOperationStatus,
		controller,
	};
};

describe('githubOperationsController', () => {
	it('updates tracked PR state and deduplicates completion refresh', () => {
		const setup = createSetup();
		setup.state.prCreating = true;
		setup.state.prCreatingStage = 'creating';

		const pullRequest: PullRequestCreated = {
			repo: 'acme/workset',
			number: 42,
			url: 'https://github.com/acme/workset/pull/42',
			title: 'Improve RepoDiff',
			state: 'open',
			draft: false,
			baseRepo: 'acme/workset',
			baseBranch: 'main',
			headRepo: 'acme/workset',
			headBranch: 'feature/repo-diff',
		};

		const status: GitHubOperationStatus = {
			operationId: 'create-op',
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			type: 'create_pr',
			stage: 'completed',
			state: 'completed',
			startedAt: new Date().toISOString(),
			finishedAt: new Date().toISOString(),
			pullRequest,
		};

		setup.controller.applyGitHubOperationStatus(status);
		setup.controller.applyGitHubOperationStatus(status);

		expect(setup.state.prCreating).toBe(false);
		expect(setup.state.prCreatingStage).toBeNull();
		expect(setup.state.prCreateError).toBeNull();
		expect(setup.state.prTracked?.number).toBe(42);
		expect(setup.state.prCreateSuccess?.number).toBe(42);
		expect(setup.state.prNumberInput).toBe('42');
		expect(setup.state.prStatus?.pullRequest.number).toBe(42);
		expect(setup.handleRefresh).toHaveBeenCalledTimes(1);
	});

	it('loads create/commit operation statuses and applies them', async () => {
		const setup = createSetup();
		setup.fetchGitHubOperationStatus
			.mockResolvedValueOnce({
				operationId: 'create-running',
				workspaceId: 'ws-1',
				repoId: 'repo-1',
				type: 'create_pr',
				stage: 'generating',
				state: 'running',
				startedAt: new Date().toISOString(),
			})
			.mockResolvedValueOnce({
				operationId: 'commit-running',
				workspaceId: 'ws-1',
				repoId: 'repo-1',
				type: 'commit_push',
				stage: 'staging',
				state: 'running',
				startedAt: new Date().toISOString(),
			});

		await setup.controller.loadGitHubOperationStatuses();

		expect(setup.fetchGitHubOperationStatus).toHaveBeenNthCalledWith(
			1,
			'ws-1',
			'repo-1',
			'create_pr',
		);
		expect(setup.fetchGitHubOperationStatus).toHaveBeenNthCalledWith(
			2,
			'ws-1',
			'repo-1',
			'commit_push',
		);
		expect(setup.state.prPanelExpanded).toBe(true);
		expect(setup.state.prCreating).toBe(true);
		expect(setup.state.prCreatingStage).toBe('generating');
		expect(setup.state.commitPushLoading).toBe(true);
		expect(setup.state.commitPushStage).toBe('staging');
	});

	it('starts create PR with trimmed inputs and updates state from running status', async () => {
		const setup = createSetup();
		setup.state.prBase = '  main  ';
		setup.state.prBaseRemote = 'upstream';
		setup.state.prDraft = true;

		await setup.controller.handleCreatePR();

		expect(setup.startCreatePullRequestAsync).toHaveBeenCalledWith('ws-1', 'repo-1', {
			base: 'main',
			baseRemote: 'upstream',
			draft: true,
		});
		expect(setup.state.prPanelExpanded).toBe(true);
		expect(setup.state.prCreating).toBe(true);
		expect(setup.state.prCreatingStage).toBe('generating');
		expect(setup.state.prCreateError).toBeNull();
	});

	it('opens auth modal on auth-required errors and retries pending action after auth success', async () => {
		const setup = createSetup();
		const action = vi
			.fn<() => Promise<void>>()
			.mockRejectedValueOnce(new Error('AUTH_REQUIRED: Sign in with GitHub'))
			.mockResolvedValueOnce(undefined);
		const onError = vi.fn();

		await setup.controller.runGitHubAction(action, onError, 'Fallback message');

		expect(action).toHaveBeenCalledTimes(1);
		expect(onError).not.toHaveBeenCalled();
		expect(setup.state.authModalOpen).toBe(true);
		expect(setup.state.authModalMessage).toBe('Sign in with GitHub');
		expect(setup.state.authPendingAction).toBeTypeOf('function');

		await setup.controller.handleAuthSuccess();

		expect(action).toHaveBeenCalledTimes(2);
		expect(onError).not.toHaveBeenCalled();
		expect(setup.state.authModalOpen).toBe(false);
		expect(setup.state.authModalMessage).toBeNull();
		expect(setup.state.authPendingAction).toBeNull();
	});
});
