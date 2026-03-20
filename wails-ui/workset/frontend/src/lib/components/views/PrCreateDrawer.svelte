<script lang="ts">
	import { Loader2 } from '@lucide/svelte';
	import { createPullRequest, generatePullRequestText } from '../../api/github';
	import type { PullRequestCreated } from '../../types';
	import SlideDrawer from '../ui/SlideDrawer.svelte';

	interface Props {
		open: boolean;
		workspaceId: string;
		repoId: string;
		repoName: string;
		branch: string;
		baseBranch: string;
		onClose: () => void;
		onCreated: (pr: PullRequestCreated) => void;
	}

	const { open, workspaceId, repoId, repoName, branch, baseBranch, onClose, onCreated }: Props =
		$props();

	let prTitle = $state('');
	let prBody = $state('');
	let isDraft = $state(false);
	let isCreating = $state(false);
	let createError: string | null = $state(null);
	let generating = $state(false);
	let generationRequestId = 0;
	let lastContextKey = '';

	const resetForm = (): void => {
		prTitle = '';
		prBody = '';
		isDraft = false;
		isCreating = false;
		createError = null;
		generating = false;
		generationRequestId += 1;
		lastContextKey = '';
	};

	const loadSuggestion = async (): Promise<void> => {
		const requestId = ++generationRequestId;
		generating = true;
		try {
			const generated = await generatePullRequestText(workspaceId, repoId);
			if (requestId !== generationRequestId) return;
			if (generated.title && !prTitle) prTitle = generated.title;
			if (generated.body && !prBody) prBody = generated.body;
		} catch {
			// non-fatal
		} finally {
			if (requestId === generationRequestId) generating = false;
		}
	};

	const handleCreate = async (): Promise<void> => {
		if (isCreating) return;
		const title = prTitle.trim();
		if (!title) {
			createError = 'Title is required.';
			return;
		}
		isCreating = true;
		createError = null;
		try {
			const created = await createPullRequest(workspaceId, repoId, {
				title,
				body: prBody.trim(),
				base: baseBranch,
				head: branch,
				draft: isDraft,
				autoCommit: true,
				autoPush: true,
			});
			onCreated(created);
			onClose();
		} catch (err) {
			createError = err instanceof Error ? err.message : 'Failed to create PR.';
		} finally {
			isCreating = false;
		}
	};

	// Load suggestion when drawer opens with new context
	$effect(() => {
		if (!open) {
			resetForm();
			return;
		}
		const contextKey = `${workspaceId}:${repoId}:${branch}`;
		if (contextKey === lastContextKey) return;
		lastContextKey = contextKey;
		resetForm();
		void loadSuggestion();
	});
</script>

<SlideDrawer {open} title="Create Pull Request" {onClose}>
	<div class="pcd-form">
		<div class="pcd-context">
			<span class="pcd-repo">{repoName}</span>
			<span class="pcd-arrow">{branch} → {baseBranch}</span>
		</div>

		{#if generating}
			<div class="pcd-generating">
				<Loader2 size={12} class="spin" />
				AI is drafting title and description...
			</div>
		{/if}

		<label class="pcd-field">
			<span class="pcd-label">Title</span>
			<input
				type="text"
				class="pcd-input"
				class:pcd-shimmer={generating && !prTitle}
				value={prTitle}
				oninput={(e) => {
					prTitle = (e.currentTarget as HTMLInputElement).value;
					if (createError) createError = null;
				}}
				placeholder={generating && !prTitle ? 'Generating...' : 'PR title'}
			/>
		</label>

		<label class="pcd-field">
			<span class="pcd-label">Description</span>
			<textarea
				class="pcd-textarea"
				class:pcd-shimmer={generating && !prBody}
				rows={5}
				value={prBody}
				oninput={(e) => {
					prBody = (e.currentTarget as HTMLTextAreaElement).value;
				}}
				placeholder={generating && !prBody ? 'Generating...' : 'Describe the changes...'}
			></textarea>
		</label>

		<label class="pcd-draft">
			<input
				type="checkbox"
				checked={isDraft}
				onchange={(e) => {
					isDraft = (e.currentTarget as HTMLInputElement).checked;
				}}
			/>
			<span>Create as draft</span>
		</label>

		{#if createError}
			<div class="pcd-error">{createError}</div>
		{/if}

		<button
			type="button"
			class="pcd-submit"
			disabled={isCreating || !prTitle.trim()}
			onclick={() => void handleCreate()}
		>
			{#if isCreating}
				<Loader2 size={12} class="spin" />
				Creating...
			{:else}
				Create Pull Request
			{/if}
		</button>
	</div>
</SlideDrawer>

<style>
	.pcd-form {
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.pcd-context {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: var(--text-xs);
		color: var(--muted);
	}
	.pcd-repo {
		font-weight: 500;
		color: var(--text);
	}
	.pcd-arrow {
		font-family: var(--font-mono);
		font-size: var(--text-mono-2xs);
		color: var(--accent);
	}
	.pcd-generating {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		color: color-mix(in srgb, var(--accent) 65%, var(--text));
	}
	.pcd-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.pcd-label {
		font-size: var(--text-2xs);
		color: var(--muted);
	}
	.pcd-input,
	.pcd-textarea {
		width: 100%;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: color-mix(in srgb, var(--panel-strong) 70%, transparent);
		color: var(--text);
		font-family: inherit;
		font-size: var(--text-xs);
		padding: 8px 10px;
	}
	.pcd-textarea {
		resize: vertical;
		min-height: 80px;
	}
	.pcd-input:focus,
	.pcd-textarea:focus {
		outline: 1px solid color-mix(in srgb, var(--accent) 60%, var(--border));
		outline-offset: 0;
	}
	.pcd-shimmer {
		background: linear-gradient(
				110deg,
				color-mix(in srgb, var(--panel-strong) 78%, transparent) 8%,
				color-mix(in srgb, var(--accent) 14%, transparent) 18%,
				color-mix(in srgb, var(--panel-strong) 78%, transparent) 33%
			)
			0 0 / 220% 100%;
		animation: pcd-shimmer 1.1s linear infinite;
	}
	@keyframes pcd-shimmer {
		to {
			background-position: -220% 0;
		}
	}
	.pcd-draft {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		color: var(--muted);
	}
	.pcd-error {
		font-size: var(--text-xs);
		color: var(--danger);
	}
	.pcd-submit {
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
	.pcd-submit:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
