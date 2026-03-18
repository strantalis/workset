<script lang="ts">
	import { fade } from 'svelte/transition';
	import {
		AlignLeft,
		AlertTriangle,
		ArrowRight,
		CheckCircle2,
		ChevronLeft,
		CircleX,
		GitBranch,
		Loader2,
		SkipForward,
		Zap,
	} from '@lucide/svelte';
	import type { HookExecution } from '../../../types';
	import type { WorkspaceActionPendingHook } from '../../../services/workspaceActionHooks';
	import type { ReviewRepoEntry } from '../OnboardingView.utils';

	interface Props {
		threadName: string;
		featureBranch: string;
		reviewDetailsExpanded: boolean;
		reviewRepoEntries: ReviewRepoEntry[];
		formName: string;
		formDescription: string;
		hookPreviewEnabled: boolean;
		hookPreviewLoading: boolean;
		hookPreviewError: string | null;
		hasPreviewedHooks: boolean;
		initializeStarted: boolean;
		isInitializing: boolean;
		hookWarnings: string[];
		hookRuns: HookExecution[];
		pendingHooks: WorkspaceActionPendingHook[];
		canOpenWorkset: boolean;
		busy: boolean;
		trimmedThreadName: string;
		runError: string | null;
		errorMessage: string | null;
		selectedRepoCount: number;
		onThreadNameInput: (value: string) => void;
		onFeatureBranchInput: (value: string) => void;
		onToggleReviewDetails: () => void;
		onRunPendingHook: (pending: WorkspaceActionPendingHook) => Promise<void>;
		onTrustPendingHook: (pending: WorkspaceActionPendingHook) => Promise<void>;
		onInitialize: () => Promise<void> | void;
		onPrevStep: () => void;
	}

	const {
		threadName,
		featureBranch,
		reviewDetailsExpanded,
		reviewRepoEntries,
		formName,
		formDescription,
		hookPreviewEnabled,
		hookPreviewLoading,
		hookPreviewError,
		hasPreviewedHooks,
		initializeStarted,
		isInitializing,
		hookWarnings,
		hookRuns,
		pendingHooks,
		canOpenWorkset,
		busy,
		trimmedThreadName,
		runError,
		errorMessage,
		selectedRepoCount,
		onThreadNameInput,
		onFeatureBranchInput,
		onToggleReviewDetails,
		onRunPendingHook,
		onTrustPendingHook,
		onInitialize,
		onPrevStep,
	}: Props = $props();

	const totalHookCount = $derived(
		reviewRepoEntries.reduce((sum, entry) => sum + entry.hooks.length, 0),
	);
</script>

<div class="first-thread-note">Every workset needs at least one thread to initialize.</div>
<label class="field">
	<span class="field-label-sm">Thread Name</span>
	<input
		type="text"
		value={threadName}
		oninput={(event) => onThreadNameInput((event.currentTarget as HTMLInputElement).value)}
		placeholder="e.g., OAuth2 Migration"
		autocapitalize="off"
		autocorrect="off"
		spellcheck="false"
	/>
</label>
<label class="field">
	<span class="field-label-sm">Feature Branch (optional)</span>
	<input
		type="text"
		value={featureBranch}
		oninput={(event) => onFeatureBranchInput((event.currentTarget as HTMLInputElement).value)}
		placeholder="e.g., feature/oauth2-migration"
		class="mono"
		autocapitalize="off"
		autocorrect="off"
		spellcheck="false"
	/>
</label>
<button
	type="button"
	class="review-toggle"
	aria-expanded={reviewDetailsExpanded}
	onclick={onToggleReviewDetails}
>
	<span>{reviewDetailsExpanded ? 'Hide review details' : 'Show review details'}</span>
	<span class="review-toggle-meta"
		>{reviewRepoEntries.length} repo{reviewRepoEntries.length === 1 ? '' : 's'}</span
	>
</button>

{#if reviewDetailsExpanded}
	<div class="review-card">
		<div class="review-meta">Creating workset:</div>
		<div class="review-name">{formName}</div>
		<div class="review-thread-row">
			<GitBranch size={12} />
			<span class="review-thread-name">{trimmedThreadName || 'Name your first thread'}</span>
		</div>
		{#if featureBranch.trim().length > 0}
			<div class="review-thread-branch">{featureBranch.trim()}</div>
		{/if}
		{#if formDescription}
			<div class="review-desc-row">
				<AlignLeft size={11} />
				<p>{formDescription}</p>
			</div>
		{/if}
		<div class="review-mode-badge">
			<GitBranch size={11} /> Repository Setup
		</div>

		<div class="review-repos-label">Repository:</div>
		<ul class="review-repo-list">
			{#each reviewRepoEntries as entry (entry.key)}
				{@const repo = entry.repo}
				<li>
					<div class="review-repo-header">
						<GitBranch size={14} class="review-repo-icon" />
						<span>{repo.name}</span>
						{#if repo.remoteUrl}
							<span class="review-repo-url">({repo.remoteUrl})</span>
						{/if}
					</div>
				</li>
			{/each}
		</ul>
		{#if hookPreviewEnabled}
			{#if hookPreviewLoading}
				<div class="review-hooks-status">
					<span class="hook-spin"><Loader2 size={13} /></span>
					<span>Checking lifecycle hooks in repository config…</span>
				</div>
			{:else if hookPreviewError}
				<div class="review-hooks-warning">
					<AlertTriangle size={12} />
					<span>{hookPreviewError}</span>
				</div>
			{/if}

			{#if hasPreviewedHooks}
				<div class="review-hooks-label">Discovered lifecycle hooks</div>
				<ul class="review-hooks-list">
					{#each reviewRepoEntries as entry (entry.key)}
						{#if entry.hooks.length > 0}
							<li class="review-hooks-item">
								<span class="review-hooks-repo">{entry.repo.name}</span>
								<div class="review-hooks-chip-row">
									{#each entry.hooks as hook (`${entry.key}-${hook}`)}
										<span class="review-hooks-chip">
											<Zap size={10} />
											{hook}
										</span>
									{/each}
								</div>
							</li>
						{/if}
					{/each}
				</ul>
			{:else if !hookPreviewLoading}
				<div class="review-no-hooks">No lifecycle hooks found in repository config.</div>
			{/if}
		{:else}
			<div class="review-no-hooks">
				Lifecycle hooks are discovered from repository config when initialization starts.
			</div>
		{/if}
	</div>
{:else}
	<div class="review-collapsed-note">
		Review details are collapsed. Expand to inspect repositories and hook previews.
	</div>
{/if}

{#if initializeStarted}
	<div class="hook-runtime-card" in:fade={{ duration: 180 }}>
		{#if isInitializing}
			<div class="hook-runtime-status">
				<span class="hook-spin"><Loader2 size={13} /></span>
				<span>Cloning repositories and discovering lifecycle hooks…</span>
			</div>
		{/if}

		{#if hookWarnings.length > 0}
			<div class="hook-warning-list">
				{#each hookWarnings as warning (warning)}
					<div class="hook-warning-item">
						<AlertTriangle size={12} />
						<span>{warning}</span>
					</div>
				{/each}
			</div>
		{/if}

		{#if hookRuns.length > 0}
			<div class="hook-runs-list">
				{#each hookRuns as run, i (`${run.repo}:${run.event}:${run.id}`)}
					<div
						class="hook-run-row"
						class:ok={run.status === 'ok'}
						class:failed={run.status === 'failed'}
						class:running-status={run.status === 'running'}
						class:skipped={run.status === 'skipped'}
						style="animation-delay: {i * 60}ms"
					>
						<span class="hook-run-icon">
							{#if run.status === 'ok'}
								<CheckCircle2 size={14} />
							{:else if run.status === 'failed'}
								<CircleX size={14} />
							{:else if run.status === 'running'}
								<span class="hook-spin"><Loader2 size={14} /></span>
							{:else}
								<SkipForward size={14} />
							{/if}
						</span>
						<span class="hook-run-body">
							<span class="hook-run-repo">{run.repo}</span>
							<span class="hook-run-id">{run.id}</span>
						</span>
						<span
							class="hook-run-status"
							class:ok={run.status === 'ok'}
							class:failed={run.status === 'failed'}
							class:running-status={run.status === 'running'}
							class:skipped={run.status === 'skipped'}
						>
							{run.status}
						</span>
					</div>
				{/each}
			</div>
		{/if}

		{#if pendingHooks.length > 0}
			<div class="pending-hooks-list">
				{#each pendingHooks as pending (`${pending.repo}:${pending.event}`)}
					<div class="pending-hook-row">
						<div class="pending-hook-copy">
							<div class="pending-hook-title">
								<Zap size={12} />
								<span>{pending.repo}</span>
								{#if pending.trusted}
									<span class="pending-hook-trusted">Trusted</span>
								{/if}
							</div>
							<div class="pending-hook-body">{pending.hooks.join(', ')}</div>
						</div>
						<div class="ws-pending-hook-actions">
							<button
								type="button"
								class="pending-hook-btn"
								disabled={pending.running || pending.trusted}
								onclick={() => void onRunPendingHook(pending)}
							>
								{pending.running ? 'Running…' : 'Run now'}
							</button>
							<button
								type="button"
								class="pending-hook-btn ghost"
								disabled={pending.trusting || pending.trusted}
								onclick={() => void onTrustPendingHook(pending)}
							>
								{pending.trusting ? 'Trusting…' : pending.trusted ? 'Trusted' : 'Trust'}
							</button>
						</div>
						{#if pending.runError}
							<div class="pending-hook-error ws-pending-hook-error">{pending.runError}</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	</div>
{/if}

{#if !initializeStarted && trimmedThreadName.length > 0}
	<div class="init-confirm-line">
		This will clone {selectedRepoCount}
		{selectedRepoCount === 1 ? 'repo' : 'repos'}{#if totalHookCount > 0}
			and run {totalHookCount} lifecycle {totalHookCount === 1 ? 'hook' : 'hooks'}{/if}
	</div>
{/if}

<div class="step3-nav">
	{#if !isInitializing && !busy}
		<button type="button" class="back-btn" onclick={onPrevStep}>
			<ChevronLeft size={20} />
		</button>
	{/if}
	<button
		type="button"
		class="init-btn"
		class:finished={canOpenWorkset}
		class:running={isInitializing && !canOpenWorkset}
		onclick={() => void onInitialize()}
		disabled={busy ||
			isInitializing ||
			(initializeStarted && !canOpenWorkset) ||
			(!initializeStarted && trimmedThreadName.length === 0)}
	>
		{#if canOpenWorkset}
			Open Workset <ArrowRight size={16} />
		{:else if isInitializing || busy}
			Initializing Environment...
		{:else if initializeStarted}
			Resolve Hook Trust To Continue
		{:else if trimmedThreadName.length === 0}
			Name your first thread to continue
		{:else}
			Initialize Workset
		{/if}
	</button>
</div>

{#if runError || errorMessage}
	<div class="init-error">{runError ?? errorMessage}</div>
{/if}

<style src="./OnboardingReviewStep.css"></style>
