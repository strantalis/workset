<script lang="ts">
	import type { WorkspaceActionDirectRepo } from '../../services/workspaceActionContextService';
	import Button from '../ui/Button.svelte';

	interface Props {
		primaryInput: string;
		directRepos: WorkspaceActionDirectRepo[];
		deriveRepoName: (source: string) => string | null;
		isRepoSource: (source: string) => boolean;
		onPrimaryInput: (value: string) => void;
		onAddDirectRepo: () => void;
		onBrowsePrimary: () => void;
		onToggleDirectRepoRegister: (url: string) => void;
		onRemoveDirectRepo: (url: string) => void;
	}

	const {
		primaryInput,
		directRepos,
		deriveRepoName,
		isRepoSource,
		onPrimaryInput,
		onAddDirectRepo,
		onBrowsePrimary,
		onToggleDirectRepoRegister,
		onRemoveDirectRepo,
	}: Props = $props();
</script>

<label class="field">
	<span>Repo URL or local path</span>
	<div class="inline">
		<input
			value={primaryInput}
			placeholder="git@github.com:org/repo.git"
			autocapitalize="off"
			autocorrect="off"
			spellcheck="false"
			oninput={(event) => onPrimaryInput((event.currentTarget as HTMLInputElement).value)}
			onkeydown={(event) => {
				if (event.key === 'Enter') {
					event.preventDefault();
					onAddDirectRepo();
				}
			}}
		/>
		<Button variant="ghost" size="sm" onclick={onBrowsePrimary}>Browse</Button>
		<Button
			variant="primary"
			size="sm"
			onclick={onAddDirectRepo}
			disabled={!primaryInput.trim() || !isRepoSource(primaryInput)}
		>
			Add
		</Button>
	</div>
</label>

{#if directRepos.length > 0}
	<div class="direct-repos-list">
		{#each directRepos as repo (repo.url)}
			<div class="direct-repo-item">
				<div class="direct-repo-info">
					<span class="direct-repo-name">{deriveRepoName(repo.url) || repo.url}</span>
					<span class="direct-repo-url">{repo.url}</span>
				</div>
				<label class="direct-repo-register" title="Save to Repo Registry for future use">
					<input
						type="checkbox"
						checked={repo.register}
						onchange={() => onToggleDirectRepoRegister(repo.url)}
					/>
					<span>Register</span>
				</label>
				<button
					type="button"
					class="direct-repo-remove"
					onclick={() => onRemoveDirectRepo(repo.url)}
				>
					×
				</button>
			</div>
		{/each}
	</div>
{/if}

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.field input {
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		color: var(--text);
		padding: 8px 10px;
		font-size: var(--text-md);
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.field input:focus {
		background: rgba(255, 255, 255, 0.04);
	}

	.inline {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.inline input {
		flex: 1;
	}

	.direct-repos-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
		margin-top: 8px;
		max-height: 180px;
		overflow-y: auto;
	}

	.direct-repo-item {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		font-size: var(--text-base);
	}

	.direct-repo-info {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.direct-repo-name {
		font-weight: 500;
		color: var(--text);
	}

	.direct-repo-url {
		font-size: var(--text-xs);
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.direct-repo-register {
		display: flex;
		align-items: center;
		gap: 4px;
		font-size: var(--text-xs);
		color: var(--muted);
		cursor: pointer;
		flex-shrink: 0;
	}

	.direct-repo-register input {
		accent-color: var(--accent);
	}

	.direct-repo-register:hover {
		color: var(--text);
	}

	.direct-repo-remove {
		background: transparent;
		border: none;
		color: var(--muted);
		cursor: pointer;
		padding: 0 4px;
		font-size: var(--text-xl);
		line-height: 1;
		transition: color var(--transition-fast);
		flex-shrink: 0;
	}

	.direct-repo-remove:hover {
		color: var(--danger, #ef4444);
	}
</style>
