<script lang="ts">
	import type { Repo, Workspace } from '../../types';

	interface Props {
		selectedWorkspace: Workspace;
		selectedRepoName: string | null;
		confirmRepoRemove: { workspaceId: string; repoName: string } | null;
		removeRepoDeleteWorktree: boolean;
		removeRepoDeleteLocal: boolean;
		onSelectRepoName: (repoName: string) => void;
		onConfirmRepoRemove: (value: { workspaceId: string; repoName: string } | null) => void;
		onRemoveRepoDeleteWorktreeChange: (value: boolean) => void;
		onRemoveRepoDeleteLocalChange: (value: boolean) => void;
		onRemoveRepo: (workspace: Workspace, repo: Repo) => void;
	}

	const {
		selectedWorkspace,
		selectedRepoName,
		confirmRepoRemove,
		removeRepoDeleteWorktree,
		removeRepoDeleteLocal,
		onSelectRepoName,
		onConfirmRepoRemove,
		onRemoveRepoDeleteWorktreeChange,
		onRemoveRepoDeleteLocalChange,
		onRemoveRepo,
	}: Props = $props();

	const resetRemovalOptions = (): void => {
		onRemoveRepoDeleteWorktreeChange(false);
		onRemoveRepoDeleteLocalChange(false);
	};
</script>

<div class="repo-list">
	<div class="section-title">Repos</div>
	{#if selectedWorkspace.repos.length === 0}
		<div class="empty">No repos configured yet.</div>
	{/if}
	{#each selectedWorkspace.repos as repo (repo.name)}
		<div class:active={repo.name === selectedRepoName} class="repo-row">
			<button class="repo-select" type="button" onclick={() => onSelectRepoName(repo.name)}>
				<div class="repo-name">{repo.name}</div>
				<div class="repo-path">{repo.path}</div>
			</button>
			<div class="card-actions">
				{#if confirmRepoRemove?.repoName === repo.name}
					<div class="remove-options">
						<label class="option">
							<input
								type="checkbox"
								checked={removeRepoDeleteWorktree}
								onchange={(event) =>
									onRemoveRepoDeleteWorktreeChange(
										(event.currentTarget as HTMLInputElement).checked,
									)}
							/>
							<span>Also delete worktrees for this repo</span>
						</label>
						<label class="option">
							<input
								type="checkbox"
								checked={removeRepoDeleteLocal}
								onchange={(event) =>
									onRemoveRepoDeleteLocalChange((event.currentTarget as HTMLInputElement).checked)}
							/>
							<span>Also delete local cache for this repo</span>
						</label>
						{#if removeRepoDeleteWorktree || removeRepoDeleteLocal}
							<div class="hint">Destructive deletes are permanent and cannot be undone.</div>
						{/if}
						{#if repo.statusKnown === false && (removeRepoDeleteWorktree || removeRepoDeleteLocal)}
							<div class="note warning">
								Repo status is still loading. Destructive deletes may be blocked if the repo is
								dirty.
							</div>
						{/if}
						{#if repo.dirty && (removeRepoDeleteWorktree || removeRepoDeleteLocal)}
							<div class="note warning">
								Uncommitted changes detected. Destructive deletes will be blocked until the repo is
								clean.
							</div>
						{/if}
					</div>
					<button
						class="danger"
						type="button"
						onclick={() => onRemoveRepo(selectedWorkspace, repo)}
					>
						Confirm remove
					</button>
					<button
						class="ghost"
						type="button"
						onclick={() => {
							onConfirmRepoRemove(null);
							resetRemovalOptions();
						}}
					>
						Cancel
					</button>
				{:else}
					<button
						class="ghost"
						type="button"
						onclick={() => {
							onConfirmRepoRemove({
								workspaceId: selectedWorkspace.id,
								repoName: repo.name,
							});
							resetRemovalOptions();
						}}
					>
						Remove
					</button>
				{/if}
			</div>
		</div>
	{/each}
</div>

<style>
	.section-title {
		font-size: 13px;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
		font-weight: 600;
	}

	.repo-list {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.repo-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 10px 12px;
		background: rgba(6, 12, 18, 0.4);
	}

	.repo-row.active {
		border-color: var(--accent);
		box-shadow: inset 0 0 0 1px rgba(45, 140, 255, 0.35);
	}

	.repo-select {
		flex: 1;
		background: none;
		border: none;
		color: inherit;
		text-align: left;
		cursor: pointer;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.repo-name {
		font-size: 14px;
		font-weight: 600;
	}

	.repo-path {
		font-size: 12px;
		color: var(--muted);
	}

	.card-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}

	.option {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: var(--text);
	}

	.option input {
		accent-color: var(--accent);
	}

	.remove-options {
		display: flex;
		flex-direction: column;
		gap: 6px;
		margin-bottom: 6px;
	}

	.hint {
		font-size: 12px;
		color: var(--muted);
	}

	.note {
		font-size: 13px;
	}

	.note.warning {
		color: var(--warning);
	}

	.empty {
		font-size: 13px;
		color: var(--muted);
		padding: 8px 0;
	}
</style>
