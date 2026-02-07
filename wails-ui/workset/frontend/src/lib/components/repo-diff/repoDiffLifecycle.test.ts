import { describe, expect, it, vi } from 'vitest';
import {
	EVENT_REPO_DIFF_LOCAL_STATUS,
	EVENT_REPO_DIFF_LOCAL_SUMMARY,
	EVENT_REPO_DIFF_PR_REVIEWS,
	EVENT_REPO_DIFF_PR_STATUS,
	EVENT_REPO_DIFF_SUMMARY,
} from '../../events';
import type { GitHubOperationStatus } from '../../api';
import { createRepoDiffLifecycle, type RepoDiffSummaryEvent } from './repoDiffLifecycle';
import type { RepoDiffPrReviewsEvent, RepoDiffPrStatusEvent } from './prStatusController';
import type { RepoDiffWatchParams } from './watcherLifecycle';

type Setup = ReturnType<typeof createSetup>;

const createSetup = () => {
	const subscriptions = new Map<string, (payload: unknown) => void>();
	const unsubscribeFns: Array<ReturnType<typeof vi.fn>> = [];
	let gitHubHandler: ((payload: GitHubOperationStatus) => void) | null = null;

	const subscribeRepoDiffEvent = vi.fn(
		<T>(eventName: string, handler: (payload: T) => void): (() => void) => {
			subscriptions.set(eventName, handler as (payload: unknown) => void);
			const unsubscribe = vi.fn();
			unsubscribeFns.push(unsubscribe);
			return unsubscribe;
		},
	);

	const subscribeGitHubOperationEvent = vi.fn(
		(handler: (payload: GitHubOperationStatus) => void): (() => void) => {
			gitHubHandler = handler;
			const unsubscribe = vi.fn();
			unsubscribeFns.push(unsubscribe);
			return unsubscribe;
		},
	);

	const onGitHubOperationEvent = vi.fn();
	const onSummaryEvent = vi.fn();
	const onLocalSummaryEvent = vi.fn();
	const onLocalStatusEvent = vi.fn();
	const onPrStatusEvent = vi.fn();
	const onPrReviewsEvent = vi.fn();
	const loadSummary = vi.fn(async (): Promise<void> => undefined);
	const loadTrackedPR = vi.fn(async (): Promise<void> => undefined);
	const loadRemotes = vi.fn(async (): Promise<void> => undefined);
	const loadLocalStatus = vi.fn(async (): Promise<void> => undefined);
	const loadLocalSummary = vi.fn(async (): Promise<void> => undefined);
	const loadGitHubOperationStatuses = vi.fn(async (): Promise<void> => undefined);
	const cleanupDiff = vi.fn();
	const watcherLifecycle = {
		syncLifecycle: vi.fn(),
		syncUpdate: vi.fn(),
		dispose: vi.fn(),
	};

	const dependencies = {
		subscribeRepoDiffEvent: subscribeRepoDiffEvent as NonNullable<
			Parameters<typeof createRepoDiffLifecycle>[1]
		>['subscribeRepoDiffEvent'],
		subscribeGitHubOperationEvent: subscribeGitHubOperationEvent as NonNullable<
			Parameters<typeof createRepoDiffLifecycle>[1]
		>['subscribeGitHubOperationEvent'],
	};

	const lifecycle = createRepoDiffLifecycle(
		{
			workspaceId: () => 'ws-1',
			repoId: () => 'repo-1',
			onGitHubOperationEvent,
			onSummaryEvent,
			onLocalSummaryEvent,
			onLocalStatusEvent,
			onPrStatusEvent,
			onPrReviewsEvent,
			loadSummary,
			loadTrackedPR,
			loadRemotes,
			loadLocalStatus,
			loadLocalSummary,
			loadGitHubOperationStatuses,
			cleanupDiff,
			watcherLifecycle,
		},
		dependencies,
	);

	return {
		lifecycle,
		subscriptions,
		unsubscribeFns,
		getGitHubHandler: () => gitHubHandler,
		onGitHubOperationEvent,
		onSummaryEvent,
		onLocalSummaryEvent,
		onLocalStatusEvent,
		onPrStatusEvent,
		onPrReviewsEvent,
		loadSummary,
		loadTrackedPR,
		loadRemotes,
		loadLocalStatus,
		loadLocalSummary,
		loadGitHubOperationStatuses,
		cleanupDiff,
		watcherLifecycle,
	};
};

const emitRepoDiffEvent = (setup: Setup, eventName: string, payload: unknown): void => {
	const handler = setup.subscriptions.get(eventName);
	expect(handler).toBeTypeOf('function');
	handler?.(payload);
};

describe('repoDiffLifecycle', () => {
	it('mounts subscriptions, loads initial data, and scopes repo diff events', async () => {
		const setup = createSetup();
		let releaseLocalStatus = (): void => undefined;
		setup.loadLocalStatus.mockImplementation(
			() =>
				new Promise<void>((resolve) => {
					releaseLocalStatus = resolve;
				}),
		);

		setup.lifecycle.mount();

		expect(setup.loadSummary).toHaveBeenCalledTimes(1);
		expect(setup.loadTrackedPR).toHaveBeenCalledTimes(1);
		expect(setup.loadRemotes).toHaveBeenCalledTimes(1);
		expect(setup.loadLocalStatus).toHaveBeenCalledTimes(1);
		expect(setup.loadGitHubOperationStatuses).toHaveBeenCalledTimes(1);
		expect(setup.loadLocalSummary).not.toHaveBeenCalled();

		releaseLocalStatus();
		await Promise.resolve();
		expect(setup.loadLocalSummary).toHaveBeenCalledTimes(1);

		emitRepoDiffEvent(setup, EVENT_REPO_DIFF_SUMMARY, {
			workspaceId: 'ws-2',
			repoId: 'repo-1',
			summary: { files: [], totalAdded: 0, totalRemoved: 0 },
		} satisfies RepoDiffSummaryEvent);
		expect(setup.onSummaryEvent).not.toHaveBeenCalled();

		const matchingSummary = {
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			summary: { files: [], totalAdded: 0, totalRemoved: 0 },
		} satisfies RepoDiffSummaryEvent;
		emitRepoDiffEvent(setup, EVENT_REPO_DIFF_SUMMARY, matchingSummary);
		expect(setup.onSummaryEvent).toHaveBeenCalledWith(matchingSummary);

		const matchingLocalSummary = {
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			summary: { files: [], totalAdded: 1, totalRemoved: 1 },
		} satisfies RepoDiffSummaryEvent;
		emitRepoDiffEvent(setup, EVENT_REPO_DIFF_LOCAL_SUMMARY, matchingLocalSummary);
		expect(setup.onLocalSummaryEvent).toHaveBeenCalledWith(matchingLocalSummary);

		emitRepoDiffEvent(setup, EVENT_REPO_DIFF_LOCAL_STATUS, {
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			status: { hasUncommitted: false, ahead: 0, behind: 0, currentBranch: 'main' },
		});
		expect(setup.onLocalStatusEvent).toHaveBeenCalledTimes(1);

		const statusEvent = {
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			status: {
				pullRequest: {
					repo: 'acme/workset',
					number: 1,
					url: '',
					title: '',
					state: 'open',
					draft: false,
					base_repo: 'acme/workset',
					base_branch: 'main',
					head_repo: 'acme/workset',
					head_branch: 'feature',
				},
				checks: [],
			},
		} satisfies RepoDiffPrStatusEvent;
		emitRepoDiffEvent(setup, EVENT_REPO_DIFF_PR_STATUS, statusEvent);
		expect(setup.onPrStatusEvent).toHaveBeenCalledWith(statusEvent);

		const reviewsEvent = {
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			comments: [],
		} satisfies RepoDiffPrReviewsEvent;
		emitRepoDiffEvent(setup, EVENT_REPO_DIFF_PR_REVIEWS, reviewsEvent);
		expect(setup.onPrReviewsEvent).toHaveBeenCalledWith(reviewsEvent);

		const githubHandler = setup.getGitHubHandler();
		expect(githubHandler).toBeTypeOf('function');
		const operationEvent: GitHubOperationStatus = {
			operationId: 'op-1',
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			type: 'create_pr',
			stage: 'queued',
			state: 'running',
			startedAt: new Date().toISOString(),
		};
		githubHandler?.(operationEvent);
		expect(setup.onGitHubOperationEvent).toHaveBeenCalledWith(operationEvent);
	});

	it('destroys subscriptions and watcher lifecycle', () => {
		const setup = createSetup();
		setup.lifecycle.mount();

		setup.lifecycle.destroy();

		expect(setup.cleanupDiff).toHaveBeenCalledTimes(1);
		for (const unsubscribe of setup.unsubscribeFns) {
			expect(unsubscribe).toHaveBeenCalledTimes(1);
		}
		expect(setup.watcherLifecycle.dispose).toHaveBeenCalledTimes(1);
	});

	it('forwards watcher sync calls', () => {
		const setup = createSetup();
		const params: RepoDiffWatchParams = {
			workspaceId: 'ws-1',
			repoId: 'repo-1',
			prNumber: 42,
			prBranch: 'feature/repo-diff',
		};

		setup.lifecycle.syncWatchLifecycle(params);
		setup.lifecycle.syncWatchUpdate(params);

		expect(setup.watcherLifecycle.syncLifecycle).toHaveBeenCalledWith(params);
		expect(setup.watcherLifecycle.syncUpdate).toHaveBeenCalledWith(params);
	});
});
