<script lang="ts">
	import { CheckCircle2, GitMerge, Loader2, Upload } from '@lucide/svelte';
	import {
		startLocalMergeAsync,
		pushBranch,
		type LocalMergeResult,
		type GitHubOperationStatus,
	} from '../../api/github';
	import { subscribeGitHubOperationEvent } from '../../githubOperationService';
	import SlideDrawer from '../ui/SlideDrawer.svelte';

	interface Props {
		open: boolean;
		workspaceId: string;
		repoId: string;
		repoName: string;
		branch: string;
		baseBranch: string;
		onClose: () => void;
		onMerged: () => void;
	}

	const { open, workspaceId, repoId, repoName, branch, baseBranch, onClose, onMerged }: Props =
		$props();

	let mergeLoading = $state(false);
	let mergeStage: string | null = $state(null);
	let mergeError: string | null = $state(null);
	let mergeResult: LocalMergeResult | null = $state(null);
	let pushingBase = $state(false);
	let pushError: string | null = $state(null);
	let pushSuccess = $state(false);

	const stageLabel = (stage: string | null): string => {
		switch (stage) {
			case 'generating_message':
				return 'Drafting commit...';
			case 'committing_worktree':
				return 'Committing branch...';
			case 'preparing_base':
				return 'Preparing base...';
			case 'merging':
				return 'Merging locally...';
			case 'committing_base':
				return 'Committing base...';
			default:
				return 'Merging...';
		}
	};

	const resetState = (): void => {
		mergeLoading = false;
		mergeStage = null;
		mergeError = null;
		mergeResult = null;
		pushingBase = false;
		pushError = null;
		pushSuccess = false;
	};

	const handleMerge = async (): Promise<void> => {
		if (mergeLoading) return;
		mergeLoading = true;
		mergeStage = 'queued';
		mergeError = null;
		try {
			await startLocalMergeAsync(workspaceId, repoId, { base: baseBranch });
		} catch (err) {
			mergeLoading = false;
			mergeStage = null;
			mergeError = err instanceof Error ? err.message : 'Failed to start merge.';
		}
	};

	const handlePush = async (): Promise<void> => {
		if (!mergeResult?.baseBranch || pushingBase) return;
		pushingBase = true;
		pushError = null;
		try {
			const result = await pushBranch(workspaceId, repoId, mergeResult.baseBranch);
			if (result.pushed) {
				mergeResult = { ...mergeResult, baseBranchPushed: true };
				pushSuccess = true;
				onMerged();
			}
		} catch (err) {
			pushError = err instanceof Error ? err.message : 'Failed to push.';
		} finally {
			pushingBase = false;
		}
	};

	$effect(() => {
		if (!open) {
			resetState();
			return;
		}
	});

	$effect(() => {
		if (!open) return;
		const unsub = subscribeGitHubOperationEvent((status: GitHubOperationStatus) => {
			if (status.workspaceId !== workspaceId) return;
			if (status.repoId !== repoId) return;
			if (status.type !== 'local_merge') return;

			if (status.state === 'running') {
				mergeLoading = true;
				mergeStage = status.stage;
				mergeError = null;
				return;
			}
			if (status.state === 'completed') {
				mergeLoading = false;
				mergeStage = null;
				mergeResult = status.localMerge ?? null;
				onMerged();
				return;
			}
			mergeLoading = false;
			mergeStage = null;
			mergeError = status.error || 'Merge failed.';
		});
		return unsub;
	});
</script>

<SlideDrawer {open} title="Local Merge" {onClose}>
	<div class="lmd-form">
		<div class="lmd-context">
			<span class="lmd-repo">{repoName}</span>
			<span class="lmd-arrow">{branch} → {baseBranch}</span>
		</div>

		<p class="lmd-desc">
			Squash merge <strong>{branch}</strong> into <strong>{baseBranch}</strong> locally. Uncommitted changes
			will be committed first.
		</p>

		{#if mergeResult}
			<div class="lmd-success">
				<CheckCircle2 size={14} />
				<span>
					Merged into {mergeResult.baseBranch} at
					<code>{mergeResult.baseSHA?.slice(0, 7)}</code>
				</span>
			</div>

			{#if mergeResult.pushable && !mergeResult.baseBranchPushed}
				<button
					type="button"
					class="lmd-btn lmd-btn-secondary"
					disabled={pushingBase}
					onclick={() => void handlePush()}
				>
					{#if pushingBase}
						<Loader2 size={12} class="spin" />
						Pushing...
					{:else}
						<Upload size={12} />
						Push {mergeResult.baseBranch}
					{/if}
				</button>
			{/if}

			{#if pushSuccess}
				<div class="lmd-success">
					<CheckCircle2 size={14} />
					Pushed {mergeResult.baseBranch} to {mergeResult.pushRemote}
				</div>
			{/if}
		{:else}
			<button
				type="button"
				class="lmd-btn"
				disabled={mergeLoading}
				onclick={() => void handleMerge()}
			>
				{#if mergeLoading}
					<Loader2 size={12} class="spin" />
					{stageLabel(mergeStage)}
				{:else}
					<GitMerge size={12} />
					Merge into {baseBranch}
				{/if}
			</button>
		{/if}

		{#if mergeError}
			<div class="lmd-error">{mergeError}</div>
		{/if}
		{#if pushError}
			<div class="lmd-error">{pushError}</div>
		{/if}
	</div>
</SlideDrawer>

<style>
	.lmd-form {
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.lmd-context {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-xs);
		color: var(--muted);
	}
	.lmd-repo {
		font-weight: 500;
		color: var(--text);
	}
	.lmd-arrow {
		font-family: var(--font-mono);
		font-size: var(--text-mono-2xs);
		color: var(--accent);
	}
	.lmd-desc {
		font-size: var(--text-xs);
		color: var(--muted);
		line-height: 1.5;
		margin: 0;
	}
	.lmd-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		padding: 8px 16px;
		border: 1px solid color-mix(in srgb, var(--accent) 45%, var(--border));
		border-radius: 6px;
		background: color-mix(in srgb, var(--accent) 16%, transparent);
		color: var(--text);
		font-size: var(--text-xs);
		font-weight: 600;
		cursor: pointer;
	}
	.lmd-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.lmd-btn-secondary {
		border-color: var(--border);
		background: transparent;
		color: var(--muted);
	}
	.lmd-btn-secondary:not(:disabled):hover {
		color: var(--text);
		border-color: color-mix(in srgb, var(--border) 140%, transparent);
	}
	.lmd-success {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		color: var(--success);
	}
	.lmd-success code {
		font-family: var(--font-mono);
		font-size: var(--text-mono-2xs);
	}
	.lmd-error {
		font-size: var(--text-xs);
		color: var(--danger);
	}
</style>
