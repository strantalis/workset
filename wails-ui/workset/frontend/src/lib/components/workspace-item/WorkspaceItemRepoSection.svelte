<script lang="ts">
	import { ellipsisMiddle } from '../../names';
	import type { Repo, Workspace } from '../../types';
	import DropdownMenu from '../ui/DropdownMenu.svelte';

	type LabelLimits = {
		repo: number;
		ref: number;
	};

	type RepoStatusDot = {
		className: string;
		label: string;
		title: string;
	};

	interface Props {
		workspace: Workspace;
		isSingleRepo: boolean;
		isMultiRepo: boolean;
		isExpanded: boolean;
		labelLimits: LabelLimits;
		onSelectRepo: (repoId: string) => void;
		onManageRepo: (repoId: string, action: 'remove') => void;
		formatLastUsed: (lastUsed: string) => string;
		formatRepoRef: (repo: Repo) => string;
		getRepoStatusDot: (repo: Repo) => RepoStatusDot;
	}

	const {
		workspace,
		isSingleRepo,
		isMultiRepo,
		isExpanded,
		labelLimits,
		onSelectRepo,
		onManageRepo,
		formatLastUsed,
		formatRepoRef,
		getRepoStatusDot,
	}: Props = $props();

	let repoMenu: string | null = $state(null);
	let repoTrigger: HTMLElement | null = $state(null);

	const closeRepoMenus = (): void => {
		repoMenu = null;
	};
</script>

{#if isSingleRepo && workspace.repos[0]}
	{@const repo = workspace.repos[0]}
	{@const status = getRepoStatusDot(repo)}
	{@const repoRef = formatRepoRef(repo)}
	<div class="repo-item">
		<button
			class="repo-info-single"
			onclick={() => onSelectRepo(repo.id)}
			type="button"
			title={repo.name}
		>
			<span class="repo-name">{ellipsisMiddle(repo.name, labelLimits.repo)}</span>
			{#if repoRef}
				<span class="branch" title={repoRef}>
					{ellipsisMiddle(repoRef, labelLimits.ref)}
				</span>
			{/if}
			<svg
				class="status-dot {status.className}"
				viewBox="0 0 6 6"
				role="img"
				aria-label={status.label}
			>
				<title>{status.title}</title>
				<circle cx="3" cy="3" r="3" />
			</svg>
			<span class="last-used-inline">{formatLastUsed(workspace.lastUsed)}</span>
		</button>
		<div class="repo-actions">
			<button
				class="menu-trigger-small"
				type="button"
				aria-label="Repo actions"
				onclick={(event) => {
					repoTrigger = event.currentTarget;
					repoMenu = repo.id;
				}}
			>
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<circle cx="5" cy="12" r="1.5" fill="currentColor" />
					<circle cx="12" cy="12" r="1.5" fill="currentColor" />
					<circle cx="19" cy="12" r="1.5" fill="currentColor" />
				</svg>
			</button>
			<DropdownMenu
				open={repoMenu === repo.id}
				onClose={closeRepoMenus}
				position="left"
				trigger={repoTrigger}
			>
				<button
					class="danger"
					type="button"
					onclick={() => {
						closeRepoMenus();
						onManageRepo(repo.name, 'remove');
					}}
				>
					<svg viewBox="0 0 24 24" aria-hidden="true">
						<path
							d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
						/>
					</svg>
					Remove
				</button>
			</DropdownMenu>
		</div>
	</div>
{:else if isMultiRepo && isExpanded}
	<div class="repo-list">
		{#each workspace.repos as repo (repo.id)}
			{@const status = getRepoStatusDot(repo)}
			{@const repoRef = formatRepoRef(repo)}
			<div class="repo-item">
				<button
					class="repo-button"
					onclick={() => onSelectRepo(repo.id)}
					type="button"
					title={repo.name}
				>
					<span class="repo-name">{ellipsisMiddle(repo.name, labelLimits.repo)}</span>
					<span class="repo-meta">
						{#if repoRef}
							<span class="branch" title={repoRef}>
								{ellipsisMiddle(repoRef, labelLimits.ref)}
							</span>
						{/if}
						<svg
							class="status-dot {status.className}"
							viewBox="0 0 6 6"
							role="img"
							aria-label={status.label}
						>
							<title>{status.title}</title>
							<circle cx="3" cy="3" r="3" />
						</svg>
					</span>
				</button>
				<div class="repo-actions">
					<button
						class="menu-trigger-small"
						type="button"
						aria-label="Repo actions"
						onclick={(event) => {
							repoTrigger = event.currentTarget;
							repoMenu = repo.id;
						}}
					>
						<svg viewBox="0 0 24 24" aria-hidden="true">
							<circle cx="5" cy="12" r="1.5" fill="currentColor" />
							<circle cx="12" cy="12" r="1.5" fill="currentColor" />
							<circle cx="19" cy="12" r="1.5" fill="currentColor" />
						</svg>
					</button>
					<DropdownMenu
						open={repoMenu === repo.id}
						onClose={closeRepoMenus}
						position="left"
						trigger={repoTrigger}
					>
						<button
							class="danger"
							type="button"
							onclick={() => {
								closeRepoMenus();
								onManageRepo(repo.name, 'remove');
							}}
						>
							<svg viewBox="0 0 24 24" aria-hidden="true">
								<path
									d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"
								/>
							</svg>
							Remove
						</button>
					</DropdownMenu>
				</div>
			</div>
		{/each}
	</div>
{/if}

<style>
	.repo-info-single {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: 0 var(--space-3) var(--space-1) 44px;
		font-size: var(--text-sm);
		background: none;
		border: none;
		color: inherit;
		cursor: pointer;
		width: 100%;
		text-align: left;
		transition: background 0.15s ease;
		flex-wrap: nowrap;
		min-width: 0;
	}

	.repo-info-single:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.repo-name {
		color: var(--text);
		font-weight: 500;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}

	.branch {
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		color: var(--muted);
		opacity: 0.7;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}

	.repo-info-single .repo-name {
		flex: 1;
	}

	.repo-info-single .branch {
		max-width: 40%;
	}

	.repo-list {
		display: flex;
		flex-direction: column;
		padding-left: 44px;
		padding-bottom: var(--space-2);
		gap: 2px;
	}

	.repo-item {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: var(--space-1);
		border-radius: var(--radius-sm);
		transition: background 0.15s ease;
	}

	.repo-item:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.repo-button {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
		background: none;
		border: none;
		color: var(--text);
		padding: 5px var(--space-3) 5px var(--space-2);
		border-radius: var(--radius-sm);
		cursor: pointer;
		text-align: left;
		transition: all 0.15s ease;
		min-width: 0;
		width: 100%;
	}

	.repo-button:hover {
		background: rgba(255, 255, 255, 0.04);
	}

	.repo-button .repo-name {
		font-size: var(--text-sm);
		flex: 1;
	}

	.repo-meta {
		display: flex;
		align-items: center;
		gap: var(--space-1);
		flex-shrink: 0;
		min-width: 0;
		max-width: 45%;
	}

	.repo-actions {
		opacity: 0;
		transition: opacity 0.15s ease;
		padding-right: var(--space-2);
	}

	.repo-item:hover .repo-actions {
		opacity: 1;
	}

	.menu-trigger-small {
		width: 24px;
		height: 24px;
		border-radius: var(--radius-sm);
		border: 1px solid var(--border);
		background: rgba(255, 255, 255, 0.02);
		color: var(--text);
		cursor: pointer;
		display: grid;
		place-items: center;
		padding: 0;
		transition: all 0.15s ease;
	}

	.menu-trigger-small:hover {
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.04);
	}

	.menu-trigger-small :global(svg) {
		width: 14px;
		height: 14px;
		stroke: currentColor;
		stroke-width: 1.6;
		fill: none;
	}

	.status-dot {
		width: 4px;
		height: 4px;
		flex-shrink: 0;
		opacity: 0.8;
	}

	.status-dot.missing {
		fill: var(--danger);
	}

	.status-dot.unknown {
		fill: var(--muted);
	}

	.status-dot.modified {
		fill: var(--warning);
	}

	.status-dot.changes {
		fill: var(--accent);
	}

	.status-dot.clean {
		fill: var(--success);
	}

	.last-used-inline {
		margin-left: auto;
		font-size: var(--text-xs);
		color: var(--muted);
		opacity: 0.7;
		white-space: nowrap;
	}

	:global(.workspace-item:hover .last-used-inline),
	:global(.workspace-item.active .last-used-inline),
	:global(.workspace-item:focus-within .last-used-inline) {
		opacity: 0;
	}
</style>
