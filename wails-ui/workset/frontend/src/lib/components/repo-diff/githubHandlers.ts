import {
	deleteReviewComment,
	fetchTrackedPullRequest,
	listRemotes,
	resolveReviewThread,
} from '../../api/github';
import type { PullRequestCreated, RemoteInfo } from '../../types';
import { applyTrackedPullRequestContext } from './prOrchestrationSurface';

interface CreateRepoDiffGitHubHandlersParams {
	workspaceId: () => string;
	repoId: () => string;
	runGitHubAction: (
		action: () => Promise<void>,
		onError: (message: string) => void,
		fallback: string,
	) => Promise<void>;
	loadPrReviews: () => Promise<void>;
	loadPrStatus: () => Promise<void>;
	loadLocalStatus: () => Promise<void>;
	loadLocalSummary: () => Promise<void>;
	setRemotesLoading: (value: boolean) => void;
	setRemotes: (value: RemoteInfo[]) => void;
	setPrTracked: (value: PullRequestCreated | null) => void;
	getPrNumberInput: () => string;
	setPrNumberInput: (value: string) => void;
	getPrBranchInput: () => string;
	setPrBranchInput: (value: string) => void;
	alertUser: (message: string) => void;
}

export interface RepoDiffGitHubHandlers {
	loadRemotes: () => Promise<void>;
	loadTrackedPR: () => Promise<void>;
	handleDeleteComment: (commentId: number) => Promise<void>;
	handleResolveThread: (threadId: string, resolve: boolean) => Promise<void>;
}

export function createRepoDiffGitHubHandlers(
	params: CreateRepoDiffGitHubHandlersParams,
): RepoDiffGitHubHandlers {
	const {
		workspaceId,
		repoId,
		runGitHubAction,
		loadPrReviews,
		loadPrStatus,
		loadLocalStatus,
		loadLocalSummary,
		setRemotesLoading,
		setRemotes,
		setPrTracked,
		getPrNumberInput,
		setPrNumberInput,
		getPrBranchInput,
		setPrBranchInput,
		alertUser,
	} = params;

	let resolvingThread = false;

	const loadRemotes = async (): Promise<void> => {
		const currentRepoId = repoId();
		if (!currentRepoId) return;
		setRemotesLoading(true);
		try {
			setRemotes(await listRemotes(workspaceId(), currentRepoId));
		} catch {
			// Non-fatal: remotes loading is optional.
			setRemotes([]);
		} finally {
			setRemotesLoading(false);
		}
	};

	const loadTrackedPR = async (): Promise<void> => {
		const currentRepoId = repoId();
		if (!currentRepoId) return;
		try {
			const tracked = await fetchTrackedPullRequest(workspaceId(), currentRepoId);
			if (!tracked) return;
			applyTrackedPullRequestContext(tracked, {
				setPrTracked,
				prNumberInput: getPrNumberInput,
				setPrNumberInput,
				prBranchInput: getPrBranchInput,
				setPrBranchInput,
			});
			void loadPrStatus();
			void loadPrReviews();
			void loadLocalStatus().then(() => loadLocalSummary());
		} catch {
			// Ignore tracking failures.
		}
	};

	const handleDeleteComment = async (commentId: number): Promise<void> => {
		const currentRepoId = repoId();
		if (!currentRepoId) return;
		await runGitHubAction(
			async () => {
				await deleteReviewComment(workspaceId(), currentRepoId, commentId);
				await loadPrReviews();
			},
			(message) => {
				alertUser(message);
			},
			'Failed to delete comment.',
		);
	};

	const handleResolveThread = async (threadId: string, resolve: boolean): Promise<void> => {
		const currentRepoId = repoId();
		if (!currentRepoId) return;
		if (!threadId) {
			alertUser('No thread ID found for this comment');
			return;
		}
		if (resolvingThread) return;
		resolvingThread = true;
		await runGitHubAction(
			async () => {
				await resolveReviewThread(workspaceId(), currentRepoId, threadId, resolve);
				await loadPrReviews();
			},
			(message) => {
				alertUser(message);
			},
			resolve ? 'Failed to resolve thread.' : 'Failed to unresolve thread.',
		);
		resolvingThread = false;
	};

	return {
		loadRemotes,
		loadTrackedPR,
		handleDeleteComment,
		handleResolveThread,
	};
}
