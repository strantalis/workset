import { describe, expect, it, vi } from 'vitest';
import type { PullRequestCreated } from '../../types';
import {
	applyPrReviewsLifecycleEvent,
	applyPrStatusLifecycleEvent,
	applyTrackedPullRequestContext,
	buildGitHubOperationsStateSurface,
	buildPrStatusStateSurface,
	resolveEffectivePrMode,
} from './prOrchestrationSurface';

const trackedPr: PullRequestCreated = {
	repo: 'acme/workset',
	number: 42,
	url: 'https://github.com/acme/workset/pull/42',
	title: 'Improve repo diff',
	state: 'open',
	draft: false,
	baseRepo: 'acme/workset',
	baseBranch: 'main',
	headRepo: 'acme/workset',
	headBranch: 'feature/repo-diff',
};

describe('prOrchestrationSurface', () => {
	it('resolves effective mode from force mode and tracked PR', () => {
		expect(resolveEffectivePrMode(null, null)).toBe('create');
		expect(resolveEffectivePrMode(null, trackedPr)).toBe('status');
		expect(resolveEffectivePrMode('create', trackedPr)).toBe('create');
		expect(resolveEffectivePrMode('status', null)).toBe('status');
	});

	it('hydrates tracked PR context without overriding existing inputs', () => {
		let prNumberInput = '';
		let prBranchInput = '';
		let prTracked: PullRequestCreated | null = null;

		applyTrackedPullRequestContext(trackedPr, {
			setPrTracked: (value) => {
				prTracked = value;
			},
			prNumberInput: () => prNumberInput,
			setPrNumberInput: (value) => {
				prNumberInput = value;
			},
			prBranchInput: () => prBranchInput,
			setPrBranchInput: (value) => {
				prBranchInput = value;
			},
		});

		expect(prTracked).toEqual(trackedPr);
		expect(prNumberInput).toBe('42');
		expect(prBranchInput).toBe('feature/repo-diff');

		prNumberInput = '99';
		prBranchInput = 'already-set';
		applyTrackedPullRequestContext(trackedPr, {
			setPrTracked: (value) => {
				prTracked = value;
			},
			prNumberInput: () => prNumberInput,
			setPrNumberInput: (value) => {
				prNumberInput = value;
			},
			prBranchInput: () => prBranchInput,
			setPrBranchInput: (value) => {
				prBranchInput = value;
			},
		});

		expect(prNumberInput).toBe('99');
		expect(prBranchInput).toBe('already-set');
	});

	it('applies PR status lifecycle events with mapped state and loading reset', () => {
		let prStatus: unknown = null;
		let prStatusError: string | null = 'error';
		let prStatusLoading = true;

		applyPrStatusLifecycleEvent(
			{
				workspaceId: 'ws-1',
				repoId: 'repo-1',
				status: {
					pullRequest: {
						repo: 'acme/workset',
						number: 7,
						url: 'https://github.com/acme/workset/pull/7',
						title: 'Improve widget',
						state: 'OPEN',
						draft: false,
						base_repo: 'acme/workset',
						base_branch: 'main',
						head_repo: 'acme/workset',
						head_branch: 'feature/widget',
					},
					checks: [
						{
							name: 'ci',
							status: 'completed',
							conclusion: 'success',
							details_url: 'https://github.com/acme/workset/actions/1',
							check_run_id: 11,
						},
					],
				},
			},
			{
				setPrStatus: (value) => {
					prStatus = value;
				},
				setPrStatusError: (value) => {
					prStatusError = value;
				},
				setPrStatusLoading: (value) => {
					prStatusLoading = value;
				},
			},
		);

		expect(prStatus).toMatchObject({
			pullRequest: {
				number: 7,
				baseRepo: 'acme/workset',
				headBranch: 'feature/widget',
			},
			checks: [
				{
					checkRunId: 11,
					detailsUrl: 'https://github.com/acme/workset/actions/1',
				},
			],
		});
		expect(prStatusError).toBeNull();
		expect(prStatusLoading).toBe(false);
	});

	it('applies PR reviews lifecycle events and loads current user only when missing', async () => {
		let prReviews: unknown[] = [];
		let prReviewsLoading = true;
		let prReviewsSent = true;
		const loadCurrentUser = vi.fn(async () => undefined);

		applyPrReviewsLifecycleEvent(
			{
				workspaceId: 'ws-1',
				repoId: 'repo-1',
				comments: [
					{
						id: 1,
						body: 'Looks good',
						path: 'main.go',
						outdated: false,
						node_id: 'NODE-1',
					},
				],
			},
			{
				setPrReviews: (value) => {
					prReviews = value;
				},
				setPrReviewsLoading: (value) => {
					prReviewsLoading = value;
				},
				setPrReviewsSent: (value) => {
					prReviewsSent = value;
				},
				currentUserId: () => null,
				loadCurrentUser,
			},
		);

		expect(prReviews).toMatchObject([{ id: 1, nodeId: 'NODE-1' }]);
		expect(prReviewsLoading).toBe(false);
		expect(prReviewsSent).toBe(false);
		expect(loadCurrentUser).toHaveBeenCalledTimes(1);

		applyPrReviewsLifecycleEvent(
			{
				workspaceId: 'ws-1',
				repoId: 'repo-1',
				comments: [],
			},
			{
				setPrReviews: (value) => {
					prReviews = value;
				},
				setPrReviewsLoading: (value) => {
					prReviewsLoading = value;
				},
				setPrReviewsSent: (value) => {
					prReviewsSent = value;
				},
				currentUserId: () => 42,
				loadCurrentUser,
			},
		);

		expect(loadCurrentUser).toHaveBeenCalledTimes(1);
	});

	it('builds pass-through state surfaces for controller option wiring', () => {
		let authModalMessage: string | null = null;
		let prStatus: unknown = null;
		let localStatus: unknown = null;

		const githubStateSurface = buildGitHubOperationsStateSurface({
			workspaceId: () => 'ws-1',
			repoId: () => 'repo-1',
			prBase: () => 'main',
			prBaseRemote: () => 'upstream',
			prDraft: () => true,
			prCreating: () => false,
			commitPushLoading: () => false,
			authModalOpen: () => false,
			getAuthPendingAction: () => null,
			setAuthModalOpen: () => undefined,
			setAuthModalMessage: (value) => {
				authModalMessage = value;
			},
			setAuthPendingAction: () => undefined,
			setPrPanelExpanded: () => undefined,
			setPrCreating: () => undefined,
			setPrCreatingStage: () => undefined,
			setPrCreateError: () => undefined,
			setPrCreateSuccess: () => undefined,
			setPrTracked: () => undefined,
			setForceMode: () => undefined,
			setPrNumberInput: () => undefined,
			setPrStatus: (value) => {
				prStatus = value;
			},
			setCommitPushLoading: () => undefined,
			setCommitPushStage: () => undefined,
			setCommitPushError: () => undefined,
			setCommitPushSuccess: () => undefined,
		});
		githubStateSurface.setAuthModalMessage('Sign in');
		githubStateSurface.setPrStatus(null);

		const prStatusStateSurface = buildPrStatusStateSurface({
			workspaceId: () => 'ws-1',
			repoId: () => 'repo-1',
			prNumberInput: () => '7',
			prBranchInput: () => 'feature',
			effectiveMode: () => 'status',
			currentUserId: () => null,
			setCurrentUserId: () => undefined,
			setPrStatus: (value) => {
				prStatus = value;
			},
			setPrStatusLoading: () => undefined,
			setPrStatusError: () => undefined,
			setPrReviews: () => undefined,
			setPrReviewsLoading: () => undefined,
			setPrReviewsSent: () => undefined,
			setLocalStatus: (value) => {
				localStatus = value;
			},
		});
		prStatusStateSurface.setLocalStatus(null);

		expect(githubStateSurface.workspaceId()).toBe('ws-1');
		expect(githubStateSurface.prDraft()).toBe(true);
		expect(authModalMessage).toBe('Sign in');
		expect(prStatus).toBeNull();
		expect(prStatusStateSurface.effectiveMode()).toBe('status');
		expect(localStatus).toBeNull();
	});
});
