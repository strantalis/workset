<script lang="ts">
	import type { PullRequestCreated, RemoteInfo } from '../../types';

	type PrCreateStageCopy = {
		button: string;
		detail: string;
	} | null;

	interface Props {
		effectiveMode: 'create' | 'status';
		prPanelExpanded?: boolean;
		remotes: RemoteInfo[];
		remotesLoading: boolean;
		prBaseRemote?: string;
		prBase?: string;
		prDraft?: boolean;
		prCreating: boolean;
		prCreateStageCopy: PrCreateStageCopy;
		prCreateError: string | null;
		prTracked: PullRequestCreated | null;
		prCreateSuccess: PullRequestCreated | null;
		prStatusError: string | null;
		prReviewsSent: boolean;
		hasUncommittedChanges: boolean;
		commitPushLoading: boolean;
		commitPushStageCopy: string;
		commitPushError: string | null;
		commitPushSuccess: boolean;
		onCreatePr: () => void | Promise<void>;
		onViewStatus: () => void;
		onCommitAndPush: () => void | Promise<void>;
	}

	/* eslint-disable prefer-const */
	// Svelte 5 requires `let` in the `$props()` declaration when a component uses bindable props.
	let {
		effectiveMode,
		prPanelExpanded = $bindable(false),
		remotes,
		remotesLoading,
		prBaseRemote = $bindable(''),
		prBase = $bindable(''),
		prDraft = $bindable(false),
		prCreating,
		prCreateStageCopy,
		prCreateError,
		prTracked,
		prCreateSuccess,
		prStatusError,
		prReviewsSent,
		hasUncommittedChanges,
		commitPushLoading,
		commitPushStageCopy,
		commitPushError,
		commitPushSuccess,
		onCreatePr,
		onViewStatus,
		onCommitAndPush,
	}: Props = $props();
	/* eslint-enable prefer-const */
</script>

{#if effectiveMode === 'create'}
	<section class="pr-panel">
		<button
			class="pr-panel-toggle"
			type="button"
			onclick={() => (prPanelExpanded = !prPanelExpanded)}
		>
			<span class="pr-panel-toggle-icon">{prPanelExpanded ? '▾' : '▸'}</span>
			<span class="pr-title">Create Pull Request</span>
		</button>

		<div class="pr-panel-content" class:expanded={prPanelExpanded}>
			<div class="pr-panel-inner">
				<div class="pr-form-row">
					<label class="field-inline">
						<span>Target</span>
						<select
							bind:value={prBaseRemote}
							disabled={remotesLoading}
							title="Base remote (defaults to upstream if available)"
						>
							<option value="">Auto</option>
							{#each remotes as remote (remote.name)}
								<option value={remote.name}>{remote.name}</option>
							{/each}
						</select>
					</label>
					<span class="field-separator">/</span>
					<label class="field-inline">
						<input
							class="branch-input"
							type="text"
							bind:value={prBase}
							placeholder="main"
							autocapitalize="off"
							autocorrect="off"
							spellcheck="false"
						/>
					</label>
					<label class="checkbox-inline">
						<input type="checkbox" bind:checked={prDraft} />
						Draft
					</label>
					<button
						class="pr-create-btn"
						class:loading={prCreating}
						type="button"
						onclick={onCreatePr}
						disabled={prCreating}
					>
						{#if prCreating}
							<span class="pr-create-spinner" aria-hidden="true">
								<svg
									class="pr-create-spinner-icon"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
								>
									<circle cx="12" cy="12" r="9" opacity="0.25" />
									<path d="M21 12a9 9 0 0 0-9-9" stroke-linecap="round" />
								</svg>
							</span>
						{/if}
						<span class="pr-create-label"
							>{prCreating ? (prCreateStageCopy?.button ?? 'Creating PR...') : 'Create PR'}</span
						>
					</button>
				</div>

				{#if prCreating && prCreateStageCopy}
					<div class="pr-create-progress" role="status" aria-live="polite">
						{prCreateStageCopy.detail}
					</div>
				{/if}

				{#if prCreateError}
					<div class="error">{prCreateError}</div>
				{/if}

				{#if prTracked && !prCreateSuccess}
					<div class="info-banner">
						Existing PR #{prTracked.number} found.
						<button class="mode-link" type="button" onclick={onViewStatus}>View status →</button>
					</div>
				{/if}
			</div>
		</div>
	</section>
{/if}

{#if effectiveMode === 'status'}
	{#if prStatusError}
		<div class="error-banner compact">{prStatusError}</div>
	{/if}
	{#if prReviewsSent}
		<div class="success-banner compact">Sent to terminal</div>
	{/if}
{/if}

{#if effectiveMode === 'status' && hasUncommittedChanges}
	<section class="local-changes-banner">
		<span class="local-changes-text">You have uncommitted local changes</span>
		<button
			class="commit-push-btn"
			type="button"
			onclick={onCommitAndPush}
			disabled={commitPushLoading}
		>
			{commitPushLoading ? commitPushStageCopy : 'Commit & Push'}
		</button>
	</section>
	{#if commitPushError}
		<div class="error-banner compact">{commitPushError}</div>
	{/if}
	{#if commitPushSuccess}
		<div class="success-banner compact">Changes committed and pushed</div>
	{/if}
{/if}

<style>
	/* Local changes warning banner */
	.local-changes-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 12px 16px;
		border-radius: 10px;
		background: rgba(210, 153, 34, 0.12);
		border: 1px solid rgba(210, 153, 34, 0.35);
	}

	.local-changes-text {
		font-size: 13px;
		font-weight: 500;
		color: #d29922;
	}

	.commit-push-btn {
		padding: 8px 16px;
		border-radius: 8px;
		border: none;
		background: linear-gradient(135deg, #d29922 0%, #b8860b 100%);
		color: #1a1a1a;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		transition:
			transform 0.15s ease,
			box-shadow 0.15s ease,
			opacity 0.15s ease;
	}

	.commit-push-btn:hover:not(:disabled) {
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(210, 153, 34, 0.3);
	}

	.commit-push-btn:active:not(:disabled) {
		transform: translateY(0);
	}

	.commit-push-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.pr-panel {
		border-radius: 14px;
		background: var(--panel);
		border: 1px solid var(--border);
		overflow: hidden;
	}

	.pr-panel-toggle {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 14px 16px;
		background: transparent;
		border: none;
		cursor: pointer;
		text-align: left;
		transition: background 0.15s ease;
	}

	.pr-panel-toggle:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.pr-panel-toggle-icon {
		font-size: 12px;
		color: var(--muted);
		width: 12px;
	}

	.pr-title {
		font-weight: 600;
		font-size: 14px;
		color: var(--text);
	}

	.pr-panel-content {
		display: grid;
		grid-template-rows: 0fr;
		transition: grid-template-rows 0.2s ease;
	}

	.pr-panel-content.expanded {
		grid-template-rows: 1fr;
	}

	.pr-panel-inner {
		overflow: hidden;
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 0 16px 14px;
	}

	.pr-form-row {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.field-inline {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--muted);
	}

	.field-inline span {
		white-space: nowrap;
	}

	.field-inline input,
	.field-inline select {
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 6px 10px;
		color: var(--text);
		font-size: 13px;
		font-family: inherit;
	}

	.field-inline select {
		cursor: pointer;
		appearance: none;
		background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%238b949e' d='M3 4.5L6 7.5L9 4.5'/%3E%3C/svg%3E");
		background-repeat: no-repeat;
		background-position: right 8px center;
		padding-right: 26px;
		min-width: 80px;
	}

	.field-inline .branch-input {
		width: 120px;
	}

	.field-separator {
		color: var(--muted);
		font-size: 14px;
	}

	.checkbox-inline {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--muted);
		white-space: nowrap;
	}

	.pr-create-btn {
		padding: 6px 14px;
		border-radius: 8px;
		border: none;
		background: var(--accent);
		color: var(--text);
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
		white-space: nowrap;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		transition: opacity 0.15s ease;
	}

	.pr-create-btn.loading {
		animation: pr-create-pulse 1.6s ease-in-out infinite;
	}

	.pr-create-btn:hover:not(:disabled) {
		opacity: 0.9;
	}

	.pr-create-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.pr-create-spinner {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		animation: pr-create-glow 1.6s ease-in-out infinite;
	}

	.pr-create-spinner-icon {
		width: 12px;
		height: 12px;
		animation: pr-create-spin 0.8s linear infinite;
	}

	.pr-create-progress {
		font-size: 12px;
		color: var(--text);
		opacity: 0.75;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		background: var(--panel-soft);
		border-radius: 8px;
		border: 1px solid var(--border);
		border-left: 3px solid var(--accent);
	}

	@keyframes pr-create-spin {
		to {
			transform: rotate(360deg);
		}
	}

	@keyframes pr-create-glow {
		0%,
		100% {
			opacity: 0.6;
		}
		50% {
			opacity: 1;
		}
	}

	@keyframes pr-create-pulse {
		0%,
		100% {
			box-shadow: 0 0 0 rgba(0, 0, 0, 0);
		}
		50% {
			box-shadow: 0 0 0 4px rgba(255, 255, 255, 0.08);
		}
	}

	.info-banner {
		font-size: 12px;
		color: var(--muted);
		padding: 8px 10px;
		background: var(--panel-soft);
		border-radius: 8px;
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.mode-link {
		font-size: 12px;
		color: var(--muted);
		cursor: pointer;
		background: none;
		border: none;
		padding: 0;
	}

	.mode-link:hover {
		color: var(--text);
	}

	@keyframes fadeIn {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	/* Error & Success Banners */
	.error-banner {
		padding: 10px 12px;
		border-radius: 8px;
		background: rgba(248, 81, 73, 0.1);
		border: 1px solid rgba(248, 81, 73, 0.3);
		color: #f85149;
		font-size: 12px;
	}

	.error-banner.compact,
	.success-banner.compact {
		padding: 6px 10px;
		font-size: 11px;
	}

	.success-banner {
		padding: 10px 12px;
		border-radius: 8px;
		background: rgba(46, 160, 67, 0.1);
		border: 1px solid rgba(46, 160, 67, 0.3);
		color: #3fb950;
		font-size: 12px;
		animation: fadeIn 0.2s ease;
	}
</style>
