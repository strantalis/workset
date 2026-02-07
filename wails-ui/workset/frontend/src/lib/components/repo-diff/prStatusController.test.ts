import { describe, expect, it, vi } from 'vitest';
import {
	createPrStatusController,
	mapPullRequestReviews,
	mapPullRequestStatus,
} from './prStatusController';
import type { PullRequestReviewComment, PullRequestStatusResult } from '../../types';
import type { RepoLocalStatus } from '../../api/github';

describe('prStatusController', () => {
	it('maps pull request status payload to frontend status model', () => {
		const result = mapPullRequestStatus({
			pullRequest: {
				repo: 'acme/widget',
				number: 17,
				url: 'https://github.com/acme/widget/pull/17',
				title: 'Improve widget',
				state: 'OPEN',
				draft: false,
				base_repo: 'acme/widget',
				base_branch: 'main',
				head_repo: 'acme/widget',
				head_branch: 'feature/widget',
			},
			checks: [
				{
					name: 'ci',
					status: 'completed',
					conclusion: 'success',
					check_run_id: 99,
				},
			],
		});

		expect(result.pullRequest.number).toBe(17);
		expect(result.pullRequest.baseRepo).toBe('acme/widget');
		expect(result.checks[0].checkRunId).toBe(99);
	});

	it('maps review payload fields to review comment model', () => {
		const result = mapPullRequestReviews([
			{
				id: 1,
				node_id: 'NODE',
				thread_id: 'THREAD',
				body: 'Needs changes',
				path: 'main.go',
				outdated: false,
				review_id: 12,
				author: 'octocat',
				line: 44,
			},
		]);

		expect(result).toHaveLength(1);
		expect(result[0].nodeId).toBe('NODE');
		expect(result[0].threadId).toBe('THREAD');
		expect(result[0].reviewId).toBe(12);
		expect(result[0].line).toBe(44);
	});

	it('refreshes summary and PR data in status mode', async () => {
		const state: {
			prStatus?: PullRequestStatusResult;
			prReviews: PullRequestReviewComment[];
			localStatus?: RepoLocalStatus;
			currentUserId: number | null;
		} = {
			prReviews: [],
			currentUserId: null,
		};
		const loadSummary = vi.fn(async () => undefined);
		const loadLocalSummary = vi.fn(async () => undefined);
		const applyRepoLocalStatus = vi.fn();
		const fetchCurrentGitHubUser = vi.fn(async () => ({ id: 42 }));
		const controller = createPrStatusController({
			workspaceId: () => 'ws',
			repoId: () => 'repo',
			prNumberInput: () => '7',
			prBranchInput: () => '',
			effectiveMode: () => 'status',
			currentUserId: () => state.currentUserId,
			setCurrentUserId: (value) => {
				state.currentUserId = value;
			},
			setPrStatus: (value) => {
				state.prStatus = value ?? undefined;
			},
			setPrStatusLoading: () => undefined,
			setPrStatusError: () => undefined,
			setPrReviews: (value) => {
				state.prReviews = value;
			},
			setPrReviewsLoading: () => undefined,
			setPrReviewsSent: () => undefined,
			setLocalStatus: (value) => {
				state.localStatus = value ?? undefined;
			},
			parseNumber: (value) => Number.parseInt(value, 10),
			runGitHubAction: async (action) => {
				await action();
			},
			loadSummary,
			loadLocalSummary,
			fetchPullRequestStatus: async () => ({
				pullRequest: {
					repo: 'acme/widget',
					number: 7,
					url: 'https://github.com/acme/widget/pull/7',
					title: 'Title',
					state: 'OPEN',
					draft: false,
					baseRepo: 'acme/widget',
					baseBranch: 'main',
					headRepo: 'acme/widget',
					headBranch: 'feature',
				},
				checks: [],
			}),
			fetchPullRequestReviews: async () => [
				{
					id: 10,
					body: 'comment',
					path: 'a.go',
					outdated: false,
				},
			],
			fetchCurrentGitHubUser,
			fetchRepoLocalStatus: async () => ({
				hasUncommitted: true,
				currentBranch: 'feature',
				ahead: 1,
				behind: 0,
			}),
			applyRepoLocalStatus,
		});

		await controller.handleRefresh();
		await Promise.resolve();

		expect(loadSummary).toHaveBeenCalledTimes(1);
		expect(loadLocalSummary).toHaveBeenCalledTimes(1);
		expect(state.prStatus?.pullRequest.number).toBe(7);
		expect(state.prReviews).toHaveLength(1);
		expect(state.localStatus?.hasUncommitted).toBe(true);
		expect(applyRepoLocalStatus).toHaveBeenCalledTimes(1);
		expect(fetchCurrentGitHubUser).toHaveBeenCalledTimes(1);
		expect(state.currentUserId).toBe(42);
	});
});
