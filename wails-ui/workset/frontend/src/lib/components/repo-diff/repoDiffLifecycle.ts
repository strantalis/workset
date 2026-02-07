import type { GitHubOperationStatus, RepoLocalStatus } from '../../api';
import {
	EVENT_REPO_DIFF_LOCAL_STATUS,
	EVENT_REPO_DIFF_LOCAL_SUMMARY,
	EVENT_REPO_DIFF_PR_REVIEWS,
	EVENT_REPO_DIFF_PR_STATUS,
	EVENT_REPO_DIFF_SUMMARY,
} from '../../events';
import { subscribeGitHubOperationEvent } from '../../githubOperationService';
import { subscribeRepoDiffEvent } from '../../repoDiffService';
import type { RepoDiffSummary } from '../../types';
import type { RepoDiffPrReviewsEvent, RepoDiffPrStatusEvent } from './prStatusController';
import type { RepoDiffWatchParams, RepoDiffWatcherLifecycle } from './watcherLifecycle';

export type RepoDiffSummaryEvent = {
	workspaceId: string;
	repoId: string;
	summary: RepoDiffSummary;
};

export type RepoDiffLocalStatusEvent = {
	workspaceId: string;
	repoId: string;
	status: RepoLocalStatus;
};

type SubscribeRepoDiffEvent = <T>(event: string, handler: (payload: T) => void) => () => void;
type SubscribeGitHubOperationEvent = (
	handler: (payload: GitHubOperationStatus) => void,
) => () => void;

type RepoDiffLifecycleDependencies = {
	subscribeRepoDiffEvent: SubscribeRepoDiffEvent;
	subscribeGitHubOperationEvent: SubscribeGitHubOperationEvent;
};

type RepoDiffLifecycleOptions = {
	workspaceId: () => string;
	repoId: () => string;
	onGitHubOperationEvent: (payload: GitHubOperationStatus) => void;
	onSummaryEvent: (payload: RepoDiffSummaryEvent) => void;
	onLocalSummaryEvent: (payload: RepoDiffSummaryEvent) => void;
	onLocalStatusEvent: (payload: RepoDiffLocalStatusEvent) => void;
	onPrStatusEvent: (payload: RepoDiffPrStatusEvent) => void;
	onPrReviewsEvent: (payload: RepoDiffPrReviewsEvent) => void;
	loadSummary: () => Promise<void>;
	loadTrackedPR: () => Promise<void>;
	loadRemotes: () => Promise<void>;
	loadLocalStatus: () => Promise<void>;
	loadLocalSummary: () => Promise<void>;
	loadGitHubOperationStatuses: () => Promise<void>;
	cleanupDiff: () => void;
	watcherLifecycle: RepoDiffWatcherLifecycle;
};

export type RepoDiffLifecycle = {
	mount: () => void;
	destroy: () => void;
	syncWatchLifecycle: (params: RepoDiffWatchParams) => void;
	syncWatchUpdate: (params: RepoDiffWatchParams) => void;
};

const defaultDependencies: RepoDiffLifecycleDependencies = {
	subscribeRepoDiffEvent,
	subscribeGitHubOperationEvent,
};

export const createRepoDiffLifecycle = (
	options: RepoDiffLifecycleOptions,
	dependencies: RepoDiffLifecycleDependencies = defaultDependencies,
): RepoDiffLifecycle => {
	let unsubscribers: Array<() => void> = [];

	const subscribeScopedRepoDiffEvent = <T extends { workspaceId: string; repoId: string }>(
		eventName: string,
		handler: (payload: T) => void,
	): (() => void) =>
		dependencies.subscribeRepoDiffEvent<T>(eventName, (payload) => {
			if (payload.workspaceId !== options.workspaceId() || payload.repoId !== options.repoId())
				return;
			handler(payload);
		});

	const mount = (): void => {
		unsubscribers = [
			dependencies.subscribeGitHubOperationEvent(options.onGitHubOperationEvent),
			subscribeScopedRepoDiffEvent<RepoDiffSummaryEvent>(
				EVENT_REPO_DIFF_SUMMARY,
				options.onSummaryEvent,
			),
			subscribeScopedRepoDiffEvent<RepoDiffSummaryEvent>(
				EVENT_REPO_DIFF_LOCAL_SUMMARY,
				options.onLocalSummaryEvent,
			),
			subscribeScopedRepoDiffEvent<RepoDiffLocalStatusEvent>(
				EVENT_REPO_DIFF_LOCAL_STATUS,
				options.onLocalStatusEvent,
			),
			subscribeScopedRepoDiffEvent<RepoDiffPrStatusEvent>(
				EVENT_REPO_DIFF_PR_STATUS,
				options.onPrStatusEvent,
			),
			subscribeScopedRepoDiffEvent<RepoDiffPrReviewsEvent>(
				EVENT_REPO_DIFF_PR_REVIEWS,
				options.onPrReviewsEvent,
			),
		];

		void options.loadSummary();
		void options.loadTrackedPR();
		void options.loadRemotes();
		void options.loadLocalStatus().then(() => options.loadLocalSummary());
		void options.loadGitHubOperationStatuses();
	};

	const destroy = (): void => {
		options.cleanupDiff();
		unsubscribers.forEach((unsubscribe) => unsubscribe());
		unsubscribers = [];
		options.watcherLifecycle.dispose();
	};

	return {
		mount,
		destroy,
		syncWatchLifecycle: (params) => {
			options.watcherLifecycle.syncLifecycle(params);
		},
		syncWatchUpdate: (params) => {
			options.watcherLifecycle.syncUpdate(params);
		},
	};
};
