import type { GitHubOperationStage, GitHubOperationStatus } from '../../api/github';
import { commitPushStageLabel as formatStageLabel } from '../views/prOrchestrationView.helpers';

export type CommitPushState = {
	readonly loading: boolean;
	readonly repoId: string | null;
	readonly stage: GitHubOperationStage | null;
	readonly error: string | null;
	readonly success: boolean;
	readonly stageLabel: string | null;
	start: (repoId: string) => void;
	handleEvent: (status: GitHubOperationStatus, onCompleted: (repoId: string) => void) => void;
	reset: () => void;
	destroy: () => void;
};

export function createCommitPushController(): CommitPushState {
	let loading = $state(false);
	let repoId = $state<string | null>(null);
	let stage = $state<GitHubOperationStage | null>(null);
	let error = $state<string | null>(null);
	let success = $state(false);
	let successTimer: ReturnType<typeof setTimeout> | null = null;

	const stageLabel = $derived(formatStageLabel(stage));

	const clearSuccessTimer = (): void => {
		if (successTimer != null) {
			clearTimeout(successTimer);
			successTimer = null;
		}
	};

	const start = (nextRepoId: string): void => {
		loading = true;
		repoId = nextRepoId;
		stage = 'queued';
		error = null;
		success = false;
		clearSuccessTimer();
	};

	const handleEvent = (
		status: GitHubOperationStatus,
		onCompleted: (completedRepoId: string) => void,
	): void => {
		if (status.type !== 'commit_push') return;
		if (repoId && status.repoId !== repoId) return;

		const targetRepoId = status.repoId;

		if (status.state === 'running') {
			loading = true;
			repoId = targetRepoId;
			stage = status.stage;
			error = null;
			success = false;
		} else if (status.state === 'completed') {
			loading = false;
			repoId = null;
			stage = null;
			error = null;
			success = true;
			onCompleted(targetRepoId);
			clearSuccessTimer();
			successTimer = setTimeout(() => {
				success = false;
				successTimer = null;
			}, 3000);
		} else if (status.state === 'failed') {
			loading = false;
			repoId = null;
			stage = null;
			success = false;
			error = status.error || 'Failed to commit and push.';
		}
	};

	const reset = (): void => {
		loading = false;
		repoId = null;
		stage = null;
		error = null;
		success = false;
		clearSuccessTimer();
	};

	const destroy = (): void => {
		clearSuccessTimer();
	};

	return {
		get loading() {
			return loading;
		},
		get repoId() {
			return repoId;
		},
		get stage() {
			return stage;
		},
		get error() {
			return error;
		},
		get success() {
			return success;
		},
		get stageLabel() {
			return stageLabel;
		},
		start,
		handleEvent,
		reset,
		destroy,
	};
}
