<script lang="ts">
	import {
		ChevronDown,
		ChevronRight,
		FilePen,
		FilePlus,
		FileX,
		FolderGit2,
		GitBranch,
		GitPullRequest,
		Maximize2,
		Minimize2,
		X,
	} from '@lucide/svelte';
	import type { DiffFile, Repo, Workspace } from '../../types';

	interface Props {
		workspace: Workspace | null;
		width?: number;
		onWidthChange?: (width: number) => void;
		onClose?: () => void;
		onInspectRepo?: (repoId: string) => void;
	}

	const {
		workspace,
		width = 340,
		onWidthChange = () => {},
		onClose = () => {},
		onInspectRepo = () => {},
	}: Props = $props();

	const MIN_WIDTH = 280;
	const COMPACT_WIDTH = 320;
	const EXPANDED_WIDTH = 560;
	const MAX_WIDTH = 800;

	const isRepoDirty = (repo: Repo): boolean =>
		repo.dirty ||
		(repo.diff?.added ?? 0) > 0 ||
		(repo.diff?.removed ?? 0) > 0 ||
		repo.files.length > 0;

	const dirtyRepos = $derived.by(() => {
		if (!workspace) return [] as Repo[];
		return workspace.repos.filter(isRepoDirty);
	});

	const totalFiles = $derived.by(() =>
		dirtyRepos.reduce((sum, repo) => sum + repo.files.length, 0),
	);
	const totalAdded = $derived.by(() =>
		dirtyRepos.reduce((sum, repo) => sum + (repo.diff?.added ?? 0), 0),
	);
	const totalRemoved = $derived.by(() =>
		dirtyRepos.reduce((sum, repo) => sum + (repo.diff?.removed ?? 0), 0),
	);

	const openTrackedPr = (repo: Repo): number | null => {
		const tracked = repo.trackedPullRequest;
		if (!tracked) return null;
		const state = tracked.state.toLowerCase();
		const merged = tracked.merged === true || state === 'merged';
		return state === 'open' && !merged ? tracked.number : null;
	};

	const fileKind = (file: DiffFile): 'added' | 'deleted' | 'modified' => {
		if (file.added > 0 && file.removed === 0) return 'added';
		if (file.removed > 0 && file.added === 0) return 'deleted';
		return 'modified';
	};

	let expandedRepos = $state<Set<string>>(new Set());
	let lastDirtyRepoKey = $state('');
	let resizing = $state(false);

	const isExpanded = $derived(width > (COMPACT_WIDTH + EXPANDED_WIDTH) / 2);

	const toggleRepo = (repoId: string): void => {
		const next = new Set(expandedRepos);
		if (next.has(repoId)) next.delete(repoId);
		else next.add(repoId);
		expandedRepos = next;
	};

	$effect(() => {
		const dirtyRepoKey = dirtyRepos.map((repo) => repo.id).join('|');
		if (dirtyRepoKey === lastDirtyRepoKey) return;
		lastDirtyRepoKey = dirtyRepoKey;
		expandedRepos = new Set(dirtyRepos.map((repo) => repo.id));
	});

	const toggleExpanded = (): void => {
		onWidthChange(isExpanded ? COMPACT_WIDTH : EXPANDED_WIDTH);
	};

	const handleResizeStart = (event: PointerEvent): void => {
		event.preventDefault();
		const target = event.currentTarget;
		if (!(target instanceof HTMLElement)) return;

		const startX = event.clientX;
		const startWidth = width;
		resizing = true;
		target.setPointerCapture(event.pointerId);

		const handlePointerMove = (moveEvent: PointerEvent): void => {
			const delta = startX - moveEvent.clientX;
			const nextWidth = Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, startWidth + delta));
			onWidthChange(nextWidth);
		};

		const stopResize = (): void => {
			resizing = false;
			target.removeEventListener('pointermove', handlePointerMove);
			target.removeEventListener('pointerup', stopResize);
			target.removeEventListener('pointercancel', stopResize);
		};

		target.addEventListener('pointermove', handlePointerMove);
		target.addEventListener('pointerup', stopResize);
		target.addEventListener('pointercancel', stopResize);
	};
</script>

<section class="changes-panel" aria-label="Changes panel">
	<div
		class="resize-handle"
		class:dragging={resizing}
		role="separator"
		aria-orientation="vertical"
		aria-label="Resize changes panel"
		title="Drag to resize. Double-click to toggle width."
		onpointerdown={handleResizeStart}
		ondblclick={toggleExpanded}
	>
		<div class="resize-line"></div>
	</div>

	<header class="changes-header">
		<h2>Changes</h2>
		<div class="header-actions">
			<button
				type="button"
				class="icon-btn"
				aria-label={isExpanded ? 'Collapse changes panel' : 'Expand changes panel'}
				title={isExpanded ? 'Collapse panel' : 'Expand panel'}
				onclick={toggleExpanded}
			>
				{#if isExpanded}
					<Minimize2 size={13} />
				{:else}
					<Maximize2 size={13} />
				{/if}
			</button>
			<button type="button" class="icon-btn" aria-label="Close changes panel" onclick={onClose}>
				<X size={14} />
			</button>
		</div>
	</header>

	<div class="changes-summary">
		<span>{totalFiles} files across {dirtyRepos.length} repos</span>
		<div class="totals">
			{#if totalAdded > 0}
				<span class="plus">+{totalAdded}</span>
			{/if}
			{#if totalRemoved > 0}
				<span class="minus">-{totalRemoved}</span>
			{/if}
		</div>
	</div>

	<div class="changes-body">
		{#if !workspace}
			<div class="empty">Select a workset to view local changes.</div>
		{:else if dirtyRepos.length === 0}
			<div class="empty">No local changes detected.</div>
		{:else}
			{#each dirtyRepos as repo (repo.id)}
				<div class="repo-node">
					<button
						type="button"
						class="repo-row"
						onclick={() => toggleRepo(repo.id)}
						aria-label={`Toggle ${repo.name} files`}
					>
						{#if expandedRepos.has(repo.id)}
							<ChevronDown size={11} />
						{:else}
							<ChevronRight size={11} />
						{/if}
						<span class="repo-icon"><FolderGit2 size={11} /></span>
						<span class="repo-name">{repo.name}</span>
						<span class="repo-diff plus">+{repo.diff?.added ?? 0}</span>
						<span class="repo-diff minus">-{repo.diff?.removed ?? 0}</span>
						<span class="repo-files">{repo.files.length}</span>
					</button>

					<div class="repo-meta">
						<span class="branch">
							<GitBranch size={10} />
							{repo.currentBranch || repo.defaultBranch || 'main'}
						</span>
						{#if openTrackedPr(repo) !== null}
							<span class="pr">
								<GitPullRequest size={10} />
								PR #{openTrackedPr(repo)}
							</span>
						{/if}
					</div>

					{#if expandedRepos.has(repo.id)}
						<div class="file-list">
							{#each repo.files as file (`${repo.id}:${file.path}`)}
								<button
									type="button"
									class="file-row"
									onclick={() => onInspectRepo(repo.id)}
									aria-label={`Open ${repo.name} details`}
								>
									<span
										class="file-kind"
										class:added={fileKind(file) === 'added'}
										class:deleted={fileKind(file) === 'deleted'}
									>
										{#if fileKind(file) === 'added'}
											<FilePlus size={10} />
										{:else if fileKind(file) === 'deleted'}
											<FileX size={10} />
										{:else}
											<FilePen size={10} />
										{/if}
									</span>
									<span class="file-path">{file.path}</span>
									<span class="file-diff plus">+{file.added}</span>
									<span class="file-diff minus">-{file.removed}</span>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			{/each}
		{/if}
	</div>
</section>

<style>
	.changes-panel {
		position: relative;
		height: 100%;
		display: grid;
		grid-template-rows: auto auto minmax(0, 1fr);
		background: color-mix(in srgb, var(--panel) 78%, transparent);
		border-left: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
		backdrop-filter: blur(12px);
	}

	.resize-handle {
		position: absolute;
		left: 0;
		top: 0;
		bottom: 0;
		width: 6px;
		cursor: col-resize;
		z-index: 3;
	}

	.resize-line {
		position: absolute;
		left: 0;
		top: 0;
		bottom: 0;
		width: 1px;
		background: color-mix(in srgb, var(--border) 65%, transparent);
	}

	.resize-handle:hover .resize-line,
	.resize-handle.dragging .resize-line {
		background: color-mix(in srgb, var(--accent) 64%, var(--border));
	}

	.changes-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 14px 10px 18px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 45%, transparent);
	}

	.changes-header h2 {
		margin: 0;
		font-size: var(--text-xs);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		font-weight: 700;
		color: color-mix(in srgb, var(--muted) 82%, white);
	}

	.icon-btn {
		width: 24px;
		height: 24px;
		border: 1px solid transparent;
		border-radius: 8px;
		background: transparent;
		color: var(--muted);
		display: inline-grid;
		place-items: center;
		cursor: pointer;
	}

	.icon-btn:hover {
		border-color: var(--border);
		color: var(--text);
		background: color-mix(in srgb, var(--panel-strong) 70%, transparent);
	}

	.header-actions {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.changes-summary {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 10px 14px 10px 18px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 35%, transparent);
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.totals {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
	}

	.plus {
		color: var(--success);
	}

	.minus {
		color: var(--danger);
	}

	.changes-body {
		min-height: 0;
		overflow: auto;
		padding: 6px 6px 10px 12px;
		display: grid;
		gap: 6px;
	}

	.empty {
		padding: 20px 12px;
		color: var(--muted);
		font-size: var(--text-sm);
		text-align: center;
	}

	.repo-node {
		border: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
		border-radius: 10px;
		background: color-mix(in srgb, var(--panel-strong) 66%, transparent);
		overflow: hidden;
	}

	.repo-row {
		width: 100%;
		display: grid;
		grid-template-columns: 14px 14px minmax(0, 1fr) auto auto auto;
		align-items: center;
		gap: 6px;
		padding: 7px 8px;
		border: none;
		background: transparent;
		color: inherit;
		cursor: pointer;
	}

	.repo-row:hover {
		background: color-mix(in srgb, var(--panel) 60%, transparent);
	}

	.repo-icon {
		color: var(--warning);
	}

	.repo-name {
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--text);
		min-width: 0;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		text-align: left;
	}

	.repo-diff,
	.file-diff {
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
	}

	.repo-files {
		font-size: var(--text-mono-xs);
		color: var(--muted);
		border: 1px solid color-mix(in srgb, var(--border) 70%, transparent);
		border-radius: 999px;
		padding: 0 5px;
	}

	.repo-meta {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 0 8px 7px 36px;
		font-size: var(--text-mono-xs);
		color: var(--muted);
	}

	.branch,
	.pr {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		min-width: 0;
	}

	.pr {
		color: #8b8aed;
	}

	.file-list {
		display: grid;
		gap: 1px;
		padding: 0 4px 4px;
	}

	.file-row {
		width: 100%;
		display: grid;
		grid-template-columns: 16px minmax(0, 1fr) auto auto;
		align-items: center;
		gap: 6px;
		padding: 5px 8px 5px 30px;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: inherit;
		cursor: pointer;
	}

	.file-row:hover {
		background: color-mix(in srgb, var(--accent) 10%, transparent);
	}

	.file-kind {
		color: var(--warning);
		display: inline-grid;
		place-items: center;
	}

	.file-kind.added {
		color: var(--success);
	}

	.file-kind.deleted {
		color: var(--danger);
	}

	.file-path {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		text-align: left;
		font-size: var(--text-xs);
		color: var(--text);
	}
</style>
