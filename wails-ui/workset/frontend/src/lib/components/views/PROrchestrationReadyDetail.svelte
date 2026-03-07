<script lang="ts">
	import {
		AlertCircle,
		CheckCircle2,
		FileCode,
		GitBranch,
		GitMerge,
		Loader2,
		Upload,
	} from '@lucide/svelte';
	import {
		createPullRequest,
		generatePullRequestText,
		pushBranch,
		startLocalMergeAsync,
		type LocalMergeResult,
		type GitHubOperationStatus,
	} from '../../api/github';
	import { subscribeGitHubOperationEvent } from '../../githubOperationService';
	import type { PullRequestCreated, RepoDiffFileSummary, RepoFileDiff } from '../../types';

	interface ReadyDetailItem {
		id: string;
		repoId: string;
		repoName: string;
		branch: string;
		dirtyFiles: number;
		ahead: number;
		hasLocalDiff: boolean;
	}

	interface Props {
		selectedItem: ReadyDetailItem;
		workspaceName: string;
		showCreatePanel: boolean;
		initialMode?: 'pull_request' | 'local_merge';
		workspaceId: string;
		baseBranch: string;
		filesForDetail: RepoDiffFileSummary[];
		totalAdd: number;
		totalDel: number;
		diffSummaryLoading: boolean;
		selectedFileIdx: number;
		fileDiffError: string | null;
		fileDiffContent: RepoFileDiff | null;
		fileDiffLoading: boolean;
		commitPushLoading: boolean;
		commitPushRepoId: string | null;
		onPushFromSidebar: (itemId: string) => Promise<void> | void;
		onPullRequestCreated: (created: PullRequestCreated) => Promise<void> | void;
		onRefreshReadyState: () => Promise<void> | void;
		diffContainer?: HTMLElement | null;
	}

	/* eslint-disable prefer-const */
	let {
		selectedItem,
		workspaceName,
		showCreatePanel,
		initialMode = 'pull_request',
		workspaceId,
		baseBranch,
		filesForDetail,
		totalAdd,
		totalDel,
		diffSummaryLoading,
		selectedFileIdx,
		fileDiffError,
		fileDiffContent,
		fileDiffLoading,
		commitPushLoading,
		commitPushRepoId,
		onPushFromSidebar,
		onPullRequestCreated,
		onRefreshReadyState,
		diffContainer = $bindable(null),
	}: Props = $props();
	/* eslint-enable prefer-const */

	let prTitle = $state('');
	let prBody = $state('');
	let prTextGenerating = $state(false);
	let prTextGenerationRequestId = 0;
	let isDraft = $state(false);
	let isCreating = $state(false);
	let prCreateError: string | null = $state(null);
	let detailMode = $state<'pull_request' | 'local_merge'>('pull_request');
	let localMergeLoading = $state(false);
	let localMergeStage = $state<string | null>(null);
	let localMergeError: string | null = $state(null);
	let localMergeResult: LocalMergeResult | null = $state(null);
	let pushingBase = $state(false);
	let pushBaseError: string | null = $state(null);
	let pushBaseSuccess = $state(false);
	let composerContextKey = '';
	let prSuggestionContextKey = '';

	const resetComposerState = (): void => {
		prTitle = '';
		prBody = '';
		isDraft = false;
		isCreating = false;
		prCreateError = null;
		prTextGenerating = false;
		prTextGenerationRequestId += 1;
		prSuggestionContextKey = '';
	};

	const localMergeStageLabel = (stage: string | null): string => {
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
				return 'Merging locally...';
		}
	};

	const loadSuggestedPrText = async (wsId: string, repoId: string): Promise<void> => {
		const requestId = ++prTextGenerationRequestId;
		prTextGenerating = true;
		try {
			const generated = await generatePullRequestText(wsId, repoId);
			if (requestId !== prTextGenerationRequestId) return;
			if (generated.title && !prTitle) prTitle = generated.title;
			if (generated.body && !prBody) prBody = generated.body;
		} catch {
			// non-fatal: user can still type manually
		} finally {
			if (requestId === prTextGenerationRequestId) {
				prTextGenerating = false;
			}
		}
	};

	const handleCreatePr = async (): Promise<void> => {
		if (!showCreatePanel || isCreating) return;
		const title = prTitle.trim();
		if (title === '') {
			prCreateError = 'PR title is required.';
			return;
		}
		isCreating = true;
		prCreateError = null;
		try {
			const created = await createPullRequest(workspaceId, selectedItem.repoId, {
				title,
				body: prBody.trim(),
				base: baseBranch,
				head: selectedItem.branch,
				draft: isDraft,
				autoCommit: true,
				autoPush: true,
			});
			await onPullRequestCreated(created);
		} catch (err) {
			prCreateError = err instanceof Error ? err.message : 'Failed to create pull request.';
		} finally {
			isCreating = false;
		}
	};

	const handleStartLocalMerge = async (): Promise<void> => {
		if (!showCreatePanel || localMergeLoading) return;
		localMergeLoading = true;
		localMergeStage = 'queued';
		localMergeError = null;
		pushBaseError = null;
		pushBaseSuccess = false;
		try {
			await startLocalMergeAsync(workspaceId, selectedItem.repoId, {
				base: baseBranch,
			});
		} catch (err) {
			localMergeLoading = false;
			localMergeStage = null;
			localMergeError = err instanceof Error ? err.message : 'Failed to start local merge.';
		}
	};

	const handlePushBaseBranch = async (): Promise<void> => {
		if (!localMergeResult?.baseBranch || pushingBase) return;
		pushingBase = true;
		pushBaseError = null;
		pushBaseSuccess = false;
		try {
			const result = await pushBranch(
				workspaceId,
				selectedItem.repoId,
				localMergeResult.baseBranch,
			);
			if (result.pushed) {
				localMergeResult = { ...localMergeResult, baseBranchPushed: true };
				pushBaseSuccess = true;
				await onRefreshReadyState();
			}
		} catch (err) {
			pushBaseError = err instanceof Error ? err.message : 'Failed to push base branch.';
		} finally {
			pushingBase = false;
		}
	};

	$effect(() => {
		if (!showCreatePanel) {
			composerContextKey = '';
			prTextGenerating = false;
			prTextGenerationRequestId += 1;
			return;
		}

		const nextKey = `${workspaceId}:${selectedItem.repoId}:${selectedItem.branch}`;
		if (composerContextKey === nextKey && detailMode === initialMode) return;
		composerContextKey = nextKey;
		resetComposerState();
		localMergeLoading = false;
		localMergeStage = null;
		localMergeError = null;
		localMergeResult = null;
		pushingBase = false;
		pushBaseError = null;
		pushBaseSuccess = false;
		detailMode = initialMode;
		if (initialMode === 'pull_request') {
			prSuggestionContextKey = nextKey;
			void loadSuggestedPrText(workspaceId, selectedItem.repoId);
		}
	});

	$effect(() => {
		if (!showCreatePanel || detailMode !== 'pull_request' || composerContextKey === '') {
			return;
		}
		if (prSuggestionContextKey === composerContextKey) {
			return;
		}
		prSuggestionContextKey = composerContextKey;
		void loadSuggestedPrText(workspaceId, selectedItem.repoId);
	});

	$effect(() => {
		const unsub = subscribeGitHubOperationEvent((status: GitHubOperationStatus) => {
			if (status.workspaceId !== workspaceId) return;
			if (status.repoId !== selectedItem.repoId) return;
			if (status.type !== 'local_merge') return;

			if (status.state === 'running') {
				localMergeLoading = true;
				localMergeStage = status.stage;
				localMergeError = null;
				pushBaseSuccess = false;
				return;
			}
			if (status.state === 'completed') {
				localMergeLoading = false;
				localMergeStage = null;
				localMergeError = null;
				localMergeResult = status.localMerge ?? null;
				void onRefreshReadyState();
				return;
			}
			localMergeLoading = false;
			localMergeStage = null;
			localMergeError = status.error || 'Local merge failed.';
		});
		return unsub;
	});
</script>

<div class="cd-header">
	<div class="cd-left">
		<GitBranch size={14} class="cd-icon" />
		<div class="cd-info">
			<div class="cd-title-row">
				<span class="cd-repo">{selectedItem.repoName}</span>
				<span class="cd-dot">·</span>
				<span class="cd-thread">{workspaceName}</span>
				<span class="cd-arrow">→</span>
				<span class="cd-branch">{selectedItem.branch}</span>
			</div>
			<div class="cd-meta">
				<span
					>{filesForDetail.length || selectedItem.dirtyFiles}
					{(filesForDetail.length || selectedItem.dirtyFiles) === 1 ? 'file' : 'files'} changed</span
				>
				<span class="cd-dot">·</span>
				{#if totalAdd > 0}<span class="cd-add">+{totalAdd}</span>{/if}
				{#if totalDel > 0}<span class="cd-del">-{totalDel}</span>{/if}
				{#if selectedItem.ahead > 0}
					<span class="cd-dot">·</span>
					<span>{selectedItem.ahead} commit{selectedItem.ahead !== 1 ? 's' : ''} ahead</span>
				{/if}
			</div>
		</div>
	</div>
	<div class="cd-actions">
		{#if selectedItem.ahead > 0 || selectedItem.hasLocalDiff}
			<button
				type="button"
				class="cd-push-btn"
				disabled={commitPushLoading}
				onclick={() => void onPushFromSidebar(selectedItem.id)}
			>
				{#if commitPushLoading && commitPushRepoId === selectedItem.repoId}
					<Loader2 size={12} class="spin" />
					Pushing...
				{:else}
					<Upload size={12} />
					Push {selectedItem.ahead > 0 ? `${selectedItem.ahead}↑` : ''}
				{/if}
			</button>
		{/if}
	</div>
</div>

{#if showCreatePanel}
	<div class="cd-create-panel">
		{#if detailMode === 'pull_request' && prTextGenerating}
			<div class="cd-generating" role="status" aria-live="polite">
				<Loader2 size={12} class="spin" />
				AI is drafting title and description...
			</div>
		{/if}
		{#if detailMode === 'pull_request'}
			<div class="cd-create-fields">
				<label class="cd-create-field">
					<span class="cd-create-label">PR Title</span>
					<input
						type="text"
						class="cd-create-input"
						class:cd-input-generating={prTextGenerating && !prTitle}
						value={prTitle}
						oninput={(event) => {
							prTitle = (event.currentTarget as HTMLInputElement).value;
							if (prCreateError) prCreateError = null;
						}}
						placeholder={prTextGenerating && !prTitle ? 'Generating title...' : 'Enter PR title...'}
					/>
				</label>
				<label class="cd-create-field">
					<span class="cd-create-label">Description</span>
					<textarea
						class="cd-create-textarea"
						class:cd-input-generating={prTextGenerating && !prBody}
						rows={3}
						value={prBody}
						oninput={(event) => {
							prBody = (event.currentTarget as HTMLTextAreaElement).value;
						}}
						placeholder={prTextGenerating && !prBody
							? 'Generating description...'
							: 'Describe the changes in this PR...'}
					></textarea>
				</label>
			</div>
			<div class="cd-create-actions">
				<label class="cd-draft-toggle">
					<input
						type="checkbox"
						checked={isDraft}
						onchange={(event) => {
							isDraft = (event.currentTarget as HTMLInputElement).checked;
						}}
					/>
					<span>Create as draft</span>
				</label>
				<button
					type="button"
					class="cd-create-btn"
					disabled={isCreating || !prTitle.trim()}
					onclick={() => void handleCreatePr()}
				>
					{#if isCreating}
						<Loader2 size={12} class="spin" />
						Creating...
					{:else}
						Create PR
					{/if}
				</button>
			</div>
			{#if prCreateError}
				<div class="cd-create-error">{prCreateError}</div>
			{/if}
		{:else}
			<div class="cd-create-fields">
				<div class="cd-create-field">
					<span class="cd-create-label">Local Merge</span>
					<p class="field-hint">
						Squash merge <strong>{selectedItem.branch}</strong> into
						<strong>{baseBranch || 'main'}</strong> locally.
					</p>
				</div>
				<div class="cd-create-field">
					<span class="cd-create-label">Behavior</span>
					<p class="field-hint">
						Uncommitted workspace changes will be committed first if needed. Pushing {baseBranch ||
							'main'} remains a separate step.
					</p>
				</div>
			</div>
			<div class="cd-create-actions cd-create-actions-right">
				{#if localMergeResult?.pushable}
					<button
						type="button"
						class="cd-create-btn cd-create-btn-secondary"
						disabled={pushingBase || localMergeLoading}
						onclick={() => void handlePushBaseBranch()}
					>
						{#if pushingBase}
							<Loader2 size={12} class="spin" />
							Pushing...
						{:else}
							<Upload size={12} />
							Push {localMergeResult.baseBranch}
						{/if}
					</button>
				{/if}
				<button
					type="button"
					class="cd-create-btn"
					disabled={localMergeLoading || localMergeResult !== null}
					onclick={() => void handleStartLocalMerge()}
				>
					{#if localMergeLoading}
						<Loader2 size={12} class="spin" />
						{localMergeStageLabel(localMergeStage)}
					{:else if localMergeResult}
						<CheckCircle2 size={12} />
						Merged into {baseBranch || 'main'}
					{:else}
						<GitMerge size={12} />
						Merge into {baseBranch || 'main'}
					{/if}
				</button>
			</div>
			{#if localMergeResult}
				<div class="cd-generating" role="status" aria-live="polite">
					<CheckCircle2 size={12} />
					Local merge complete on {localMergeResult.baseBranch} at {localMergeResult.baseSHA?.slice(
						0,
						7,
					)}
				</div>
			{/if}
			{#if pushBaseSuccess}
				<div class="cd-generating" role="status" aria-live="polite">
					<CheckCircle2 size={12} />
					Pushed {localMergeResult?.baseBranch} to {localMergeResult?.pushRemote}
				</div>
			{/if}
			{#if localMergeError}
				<div class="cd-create-error">{localMergeError}</div>
			{/if}
			{#if pushBaseError}
				<div class="cd-create-error">{pushBaseError}</div>
			{/if}
		{/if}
	</div>
{/if}

<div class="cd-body">
	<div class="fp-diff">
		{#if diffSummaryLoading}
			<div class="diff-placeholder full">
				<Loader2 size={20} class="spin" />
				<p>Loading files...</p>
			</div>
		{:else if filesForDetail[selectedFileIdx]}
			{@const activeFile = filesForDetail[selectedFileIdx]}
			<div class="diff-card">
				<div class="diff-header">
					<span>{activeFile.path}</span>
					<span>
						{#if activeFile.added > 0}<span class="text-green">+{activeFile.added}</span>{/if}
						{#if activeFile.removed > 0}<span class="text-red">-{activeFile.removed}</span>{/if}
					</span>
				</div>
				<div class="diff-body">
					{#if fileDiffError}
						<div class="diff-placeholder">
							<AlertCircle size={20} />
							<p>{fileDiffError}</p>
						</div>
					{:else if fileDiffContent?.binary}
						<div class="diff-placeholder">
							<FileCode size={24} />
							<p>Binary file</p>
						</div>
					{:else if fileDiffContent?.patch}
						<div class="diff-renderer-wrap">
							<div class="diff-renderer">
								<diffs-container bind:this={diffContainer}></diffs-container>
							</div>
							{#if fileDiffLoading}
								<div class="diff-loading-overlay">
									<Loader2 size={18} class="spin" />
									<p>Refreshing diff...</p>
								</div>
							{/if}
						</div>
						{#if fileDiffContent.truncated}
							<div class="diff-truncated">
								Diff truncated ({fileDiffContent.totalLines} total lines)
							</div>
						{/if}
					{:else if fileDiffLoading}
						<div class="diff-placeholder">
							<Loader2 size={20} class="spin" />
							<p>Loading diff...</p>
						</div>
					{:else}
						<div class="diff-placeholder">
							<FileCode size={24} />
							<p>No diff content</p>
						</div>
					{/if}
				</div>
			</div>
		{:else}
			<div class="diff-placeholder full">
				<FileCode size={24} />
				<p>No changed files detected</p>
			</div>
		{/if}
	</div>
</div>

<style src="./PROrchestrationReadyDetail.css"></style>
