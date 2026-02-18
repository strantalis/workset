import type { GitHubOperationStage, RepoLocalStatus } from '../../api/github';
import type { PrCreateStage } from '../../prCreateProgress';
import type {
	PullRequestCreated,
	PullRequestReviewComment,
	PullRequestStatusResult,
} from '../../types';
import {
	mapPullRequestReviews,
	mapPullRequestStatus,
	type RepoDiffPrReviewsEvent,
	type RepoDiffPrStatusEvent,
} from './prStatusController';

type PendingGitHubAction = (() => Promise<void>) | null;

export type RepoDiffPrMode = 'create' | 'status';
export type RepoDiffPrForceMode = RepoDiffPrMode | null;

export const resolveEffectivePrMode = (
	forceMode: RepoDiffPrForceMode,
	prTracked: PullRequestCreated | null,
): RepoDiffPrMode => forceMode ?? (prTracked ? 'status' : 'create');

type GitHubOperationsStateSurfaceOptions = {
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
	setForceMode: (value: RepoDiffPrForceMode) => void;
	setPrNumberInput: (value: string) => void;
	setPrStatus: (value: PullRequestStatusResult | null) => void;
	setCommitPushLoading: (value: boolean) => void;
	setCommitPushStage: (value: GitHubOperationStage | null) => void;
	setCommitPushError: (value: string | null) => void;
	setCommitPushSuccess: (value: boolean) => void;
};

export const buildGitHubOperationsStateSurface = (
	options: GitHubOperationsStateSurfaceOptions,
) => ({
	workspaceId: options.workspaceId,
	repoId: options.repoId,
	prBase: options.prBase,
	prBaseRemote: options.prBaseRemote,
	prDraft: options.prDraft,
	prCreating: options.prCreating,
	commitPushLoading: options.commitPushLoading,
	authModalOpen: options.authModalOpen,
	getAuthPendingAction: options.getAuthPendingAction,
	setAuthModalOpen: options.setAuthModalOpen,
	setAuthModalMessage: options.setAuthModalMessage,
	setAuthPendingAction: options.setAuthPendingAction,
	setPrPanelExpanded: options.setPrPanelExpanded,
	setPrCreating: options.setPrCreating,
	setPrCreatingStage: options.setPrCreatingStage,
	setPrCreateError: options.setPrCreateError,
	setPrCreateSuccess: options.setPrCreateSuccess,
	setPrTracked: options.setPrTracked,
	setForceMode: options.setForceMode,
	setPrNumberInput: options.setPrNumberInput,
	setPrStatus: options.setPrStatus,
	setCommitPushLoading: options.setCommitPushLoading,
	setCommitPushStage: options.setCommitPushStage,
	setCommitPushError: options.setCommitPushError,
	setCommitPushSuccess: options.setCommitPushSuccess,
});

type PrStatusStateSurfaceOptions = {
	workspaceId: () => string;
	repoId: () => string;
	prNumberInput: () => string;
	prBranchInput: () => string;
	effectiveMode: () => RepoDiffPrMode;
	currentUserId: () => number | null;
	setCurrentUserId: (value: number | null) => void;
	setPrStatus: (value: PullRequestStatusResult | null) => void;
	setPrStatusLoading: (value: boolean) => void;
	setPrStatusError: (value: string | null) => void;
	setPrReviews: (value: PullRequestReviewComment[]) => void;
	setPrReviewsLoading: (value: boolean) => void;
	setPrReviewsSent: (value: boolean) => void;
	setLocalStatus: (value: RepoLocalStatus | null) => void;
};

export const buildPrStatusStateSurface = (options: PrStatusStateSurfaceOptions) => ({
	workspaceId: options.workspaceId,
	repoId: options.repoId,
	prNumberInput: options.prNumberInput,
	prBranchInput: options.prBranchInput,
	effectiveMode: options.effectiveMode,
	currentUserId: options.currentUserId,
	setCurrentUserId: options.setCurrentUserId,
	setPrStatus: options.setPrStatus,
	setPrStatusLoading: options.setPrStatusLoading,
	setPrStatusError: options.setPrStatusError,
	setPrReviews: options.setPrReviews,
	setPrReviewsLoading: options.setPrReviewsLoading,
	setPrReviewsSent: options.setPrReviewsSent,
	setLocalStatus: options.setLocalStatus,
});

type TrackedPullRequestContextOptions = {
	setPrTracked: (value: PullRequestCreated) => void;
	prNumberInput: () => string;
	setPrNumberInput: (value: string) => void;
	prBranchInput: () => string;
	setPrBranchInput: (value: string) => void;
};

export const applyTrackedPullRequestContext = (
	tracked: PullRequestCreated,
	options: TrackedPullRequestContextOptions,
): void => {
	options.setPrTracked(tracked);
	if (!options.prNumberInput()) {
		options.setPrNumberInput(`${tracked.number}`);
	}
	if (!options.prBranchInput() && tracked.headBranch) {
		options.setPrBranchInput(tracked.headBranch);
	}
};

type PrStatusLifecycleStateOptions = {
	setPrStatus: (value: PullRequestStatusResult | null) => void;
	setPrStatusError: (value: string | null) => void;
	setPrStatusLoading: (value: boolean) => void;
};

export const applyPrStatusLifecycleEvent = (
	payload: RepoDiffPrStatusEvent,
	options: PrStatusLifecycleStateOptions,
): void => {
	options.setPrStatus(mapPullRequestStatus(payload.status));
	options.setPrStatusError(null);
	options.setPrStatusLoading(false);
};

type PrReviewsLifecycleStateOptions = {
	setPrReviews: (value: PullRequestReviewComment[]) => void;
	setPrReviewsLoading: (value: boolean) => void;
	setPrReviewsSent: (value: boolean) => void;
	currentUserId: () => number | null;
	loadCurrentUser: () => Promise<void>;
};

export const applyPrReviewsLifecycleEvent = (
	payload: RepoDiffPrReviewsEvent,
	options: PrReviewsLifecycleStateOptions,
): void => {
	options.setPrReviews(mapPullRequestReviews(payload.comments));
	options.setPrReviewsLoading(false);
	options.setPrReviewsSent(false);
	if (options.currentUserId() === null) {
		void options.loadCurrentUser();
	}
};
