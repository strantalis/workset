<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { slide } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';
	import {
		AlertCircle,
		ArrowDownLeft,
		ArrowUpRight,
		ChevronDown,
		CircleDot,
		FilePen,
		FilePlus,
		FileX,
		GitBranch,
		GitCommit,
		GitPullRequest,
		Plus,
		RefreshCw,
		Search,
	} from '@lucide/svelte';
	import type { FileDiffMetadata } from '@pierre/diffs';
	import type { Repo, RepoDiffFileSummary, RepoDiffSummary, Workspace } from '../../types';
	import { fetchRepoDiffSummary, fetchRepoFileDiff } from '../../api/repo-diff';

	interface Props {
		workspaces: Workspace[];
		activeWorkspaceId: string | null;
		onCreateWorkspace?: () => void;
		onSelectRepo?: (workspaceId: string, repoId: string) => void;
	}

	const {
		workspaces,
		activeWorkspaceId,
		onCreateWorkspace = () => {},
		onSelectRepo,
	}: Props = $props();

	type DiffsModule = typeof import('@pierre/diffs');

	let repoSearch = $state('');
	let expandedRepoId = $state<string | null>(null);
	let expandedFiles = $state<Set<string>>(new Set());
	let repoSummaries = $state<Record<string, RepoDiffSummary>>({});
	let summaryLoading = $state<Record<string, boolean>>({});
	let fileDiffs = $state<Record<string, FileDiffMetadata>>({});
	let fileDiffLoading = $state<Record<string, boolean>>({});
	let diffModule = $state<DiffsModule | null>(null);
	const diffRenderers: Record<string, InstanceType<DiffsModule['FileDiff']>> = {};

	const ensureDiffModule = async (): Promise<DiffsModule> => {
		if (diffModule) return diffModule;
		const mod = (await import('@pierre/diffs')) as DiffsModule;
		diffModule = mod;
		return mod;
	};

	const activeWorkspace = $derived(workspaces.find((w) => w.id === activeWorkspaceId) ?? null);
	const activeWorkspaceName = $derived(activeWorkspace?.name ?? 'No workspace selected');

	// Stats derived from active workspace
	const linkedRepos = $derived(activeWorkspace?.repos.length ?? 0);
	const dirtyFiles = $derived(
		activeWorkspace?.repos.reduce((acc, repo) => {
			if (!repo.dirty) return acc;
			// Prefer fetched summary count, fall back to repo.files
			const summary = repoSummaries[repo.id];
			return acc + (summary ? summary.files.length : repo.files.length);
		}, 0) ?? 0,
	);
	const openPrs = $derived(
		activeWorkspace?.repos.filter((repo) => repo.trackedPullRequest?.state.toLowerCase() === 'open')
			.length ?? 0,
	);

	/** Returns the file count for a dirty repo (summary or inline). */
	const getDirtyFileCount = (repo: Repo): number => {
		const summary = repoSummaries[repo.id];
		if (summary) return summary.files.length;
		return repo.files.length;
	};

	const filteredRepos = $derived.by(() => {
		if (!activeWorkspace) return [];
		const query = repoSearch.trim().toLowerCase();
		if (!query) return activeWorkspace.repos;
		return activeWorkspace.repos.filter(
			(repo) =>
				repo.name.toLowerCase().includes(query) ||
				(repo.currentBranch ?? '').toLowerCase().includes(query),
		);
	});

	const getStatusClass = (repo: Repo): string => {
		if (repo.missing) return 'error';
		if (repo.dirty) return 'modified';
		if ((repo.ahead ?? 0) > 0) return 'ahead';
		return 'clean';
	};

	const openRepoDetails = (repo: Repo): void => {
		if (!activeWorkspaceId || !onSelectRepo) return;
		onSelectRepo(activeWorkspaceId, repo.id);
	};

	const handleRepoHeaderClick = (repo: Repo): void => {
		if (repo.dirty) {
			void toggleExpand(repo);
			return;
		}
		openRepoDetails(repo);
	};

	const toggleExpand = async (repo: Repo): Promise<void> => {
		if (expandedRepoId === repo.id) {
			expandedRepoId = null;
			return;
		}
		expandedRepoId = repo.id;
		expandedFiles = new Set();

		// Load file-level diff summary if not cached and repo is dirty
		if (!repoSummaries[repo.id] && repo.dirty && activeWorkspaceId) {
			summaryLoading = { ...summaryLoading, [repo.id]: true };
			try {
				const summary = await fetchRepoDiffSummary(activeWorkspaceId, repo.id);
				repoSummaries = { ...repoSummaries, [repo.id]: summary };
			} catch {
				// Silently fail - we'll show the basic file list from Repo.files
			} finally {
				summaryLoading = { ...summaryLoading, [repo.id]: false };
			}
		}
	};

	const toggleFile = async (repoId: string, file: RepoDiffFileSummary): Promise<void> => {
		const key = `${repoId}:${file.path}`;
		const next = new Set(expandedFiles);
		if (next.has(key)) {
			next.delete(key);
			expandedFiles = next;
			return;
		}
		next.add(key);
		expandedFiles = next;

		// Fetch and parse patch if not cached
		if (!fileDiffs[key] && activeWorkspaceId) {
			fileDiffLoading = { ...fileDiffLoading, [key]: true };
			try {
				const [mod, result] = await Promise.all([
					ensureDiffModule(),
					fetchRepoFileDiff(activeWorkspaceId, repoId, file.path, file.prevPath ?? '', file.status),
				]);
				if (result.patch) {
					const parsed = mod.parsePatchFiles(result.patch);
					const firstFile = parsed[0]?.files?.[0];
					if (firstFile) {
						fileDiffs = { ...fileDiffs, [key]: firstFile };
					}
				}
			} catch {
				// Silently fail and keep fallback state.
			} finally {
				fileDiffLoading = { ...fileDiffLoading, [key]: false };
			}
		}

		// Wait for DOM to update so bind:this populates the container ref,
		// then imperatively render the diff into it.
		await tick();
		renderDiffInto(key, diffContainers[key]);
	};

	const diffContainers = $state<Record<string, HTMLElement | null>>({});

	/** Render diff into a container whenever both the data and element are ready. */
	const renderDiffInto = (key: string, container: HTMLElement | null) => {
		const diffData = fileDiffs[key];
		if (!diffData || !diffModule || !container) return;

		let renderer = diffRenderers[key];
		if (!renderer) {
			renderer = new diffModule.FileDiff({
				theme: 'pierre-dark',
				themeType: 'dark',
				diffStyle: 'unified',
				diffIndicators: 'bars',
				hunkSeparators: 'line-info',
				lineDiffType: 'word',
				overflow: 'scroll',
				disableFileHeader: true,
			});
			diffRenderers[key] = renderer;
		}

		renderer.render({
			fileDiff: diffData,
			fileContainer: container,
			forceRender: true,
		});
	};

	const getFileKindIcon = (status: string): typeof FilePen => {
		if (status === 'A' || status === 'added') return FilePlus;
		if (status === 'D' || status === 'deleted') return FileX;
		return FilePen;
	};

	const getFileKindColor = (status: string): string => {
		if (status === 'A' || status === 'added') return '#86C442';
		if (status === 'D' || status === 'deleted') return '#EF4444';
		return '#F59E0B';
	};

	/** Get RepoDiffFileSummary entries for a repo — prefers fetched summary, falls back to repo.files. */
	const getSummaryFiles = (repo: Repo): RepoDiffFileSummary[] => {
		const summary = repoSummaries[repo.id];
		if (summary) return summary.files;
		// Fallback: map DiffFile → RepoDiffFileSummary so file rows still render
		return repo.files.map((f) => ({
			path: f.path,
			added: f.added,
			removed: f.removed,
			status: f.added > 0 && f.removed === 0 ? 'A' : f.removed > 0 && f.added === 0 ? 'D' : 'M',
		}));
	};

	/** Preload diff summaries for all dirty repos so stats + cards are populated. */
	const preloadDirtySummaries = async (): Promise<void> => {
		if (!activeWorkspace || !activeWorkspaceId) return;
		const dirtyRepos = activeWorkspace.repos.filter(
			(r) => r.dirty && !repoSummaries[r.id] && !summaryLoading[r.id],
		);
		await Promise.allSettled(
			dirtyRepos.map(async (repo) => {
				summaryLoading = { ...summaryLoading, [repo.id]: true };
				try {
					const summary = await fetchRepoDiffSummary(activeWorkspaceId!, repo.id);
					repoSummaries = { ...repoSummaries, [repo.id]: summary };
				} catch {
					// Silently fail — card will show repo.files fallback
				} finally {
					summaryLoading = { ...summaryLoading, [repo.id]: false };
				}
			}),
		);
	};

	$effect(() => {
		const ws = activeWorkspace;
		const wsId = activeWorkspaceId;
		if (!ws || !wsId) return;
		// Read dirty flags so the effect re-runs when the background
		// status refresh marks repos as dirty after initial load.
		const dirtyRepoIds = ws.repos.filter((r) => r.dirty).map((r) => r.id);
		if (dirtyRepoIds.length === 0) return;
		untrack(() => {
			const hasPending = dirtyRepoIds.some((id) => !repoSummaries[id] && !summaryLoading[id]);
			if (hasPending) {
				void preloadDirtySummaries();
			}
		});
	});
</script>

<div class="command-center">
	<!-- Header -->
	<header class="cc-header">
		<div>
			<h1>Command Center</h1>
			<p>Working tree for <span class="ws-name">{activeWorkspaceName}</span></p>
		</div>
		<div class="header-actions">
			{#if activeWorkspace}
				<div class="daemon-badge">
					<span class="daemon-ping">
						<span class="ping-ring"></span>
						<span class="ping-dot"></span>
					</span>
					Daemon Active
				</div>
			{/if}
			<button
				type="button"
				class="refresh-btn"
				onclick={() => {
					repoSummaries = {};
					void preloadDirtySummaries();
				}}
			>
				<RefreshCw size={18} />
			</button>
		</div>
	</header>

	<!-- Stat cards -->
	<section class="stats">
		<article class="stat-card">
			<div class="stat-text">
				<span class="stat-label">Active Workset</span>
				<strong>{activeWorkspace ? activeWorkspaceName : '—'}</strong>
			</div>
			<div class="stat-icon">
				<GitBranch size={18} class="icon-green" />
			</div>
		</article>
		<article class="stat-card">
			<div class="stat-text">
				<span class="stat-label">Linked Repos</span>
				<strong>{linkedRepos}</strong>
			</div>
			<div class="stat-icon">
				<GitCommit size={18} class="icon-blue" />
			</div>
		</article>
		<article class="stat-card" class:accent={dirtyFiles > 0}>
			<div class="stat-text">
				<span class="stat-label">Dirty Files</span>
				<strong>{dirtyFiles} file{dirtyFiles !== 1 ? 's' : ''}</strong>
			</div>
			<div class="stat-icon">
				<AlertCircle size={18} class="icon-yellow" />
			</div>
		</article>
		<article class="stat-card">
			<div class="stat-text">
				<span class="stat-label">Open PRs</span>
				<strong>{openPrs}</strong>
			</div>
			<div class="stat-icon">
				<GitPullRequest size={18} class="icon-purple" />
			</div>
		</article>
	</section>

	{#if !activeWorkspace}
		<section class="empty-card">
			<h2>No workspace selected</h2>
			<p>Select a workspace from the Hub or create a new one to see repository status.</p>
			<button type="button" class="cta" onclick={onCreateWorkspace}>
				<Plus size={15} /> Create Workspace
			</button>
		</section>
	{:else if activeWorkspace.repos.length === 0}
		<section class="empty-card">
			<h2>No repos linked</h2>
			<p>Add repositories to this workspace to track branch health and drift.</p>
		</section>
	{:else}
		<!-- Repo grid panel -->
		<div class="repo-panel">
			<div class="panel-toolbar">
				<span class="panel-title">{activeWorkspaceName}</span>
				<label class="panel-search">
					<Search size={14} />
					<input
						type="text"
						bind:value={repoSearch}
						placeholder="Filter repos..."
						autocapitalize="off"
						autocorrect="off"
						spellcheck="false"
					/>
				</label>
			</div>

			<div class="panel-content">
				{#if filteredRepos.length === 0}
					<div class="no-results">No repos matched your filter.</div>
				{:else}
					<div class="repo-grid">
						{#each filteredRepos as repo (repo.id)}
							{@const isExpanded = expandedRepoId === repo.id}
							{@const canExpand = repo.dirty}
							{@const fileCount = getDirtyFileCount(repo)}
							{@const isLoading = summaryLoading[repo.id] ?? false}
							{@const statusClass = getStatusClass(repo)}
							<div class="repo-card {statusClass}" class:expanded={isExpanded}>
								<!-- Card header -->
								<button
									type="button"
									class="repo-header"
									class:clickable={canExpand || !!onSelectRepo}
									onclick={() => handleRepoHeaderClick(repo)}
								>
									<div class="repo-title-row">
										<div class="repo-name-group">
											<h3>{repo.name}</h3>
											<span class="repo-branch"
												>{repo.currentBranch || repo.defaultBranch || 'main'}</span
											>
										</div>
										<div class="repo-badges">
											<span class="status-dot {statusClass}"></span>
											{#if canExpand}
												<span class="chevron" class:open={isExpanded}>
													<ChevronDown size={14} />
												</span>
											{/if}
										</div>
									</div>
									<div class="repo-meta-row">
										<span class="meta-pair" class:highlight-blue={(repo.ahead ?? 0) > 0}>
											<ArrowUpRight size={12} />
											{repo.ahead ?? 0}
										</span>
										<span class="meta-pair" class:highlight-yellow={(repo.behind ?? 0) > 0}>
											<ArrowDownLeft size={12} />
											{repo.behind ?? 0}
										</span>
										{#if canExpand}
											<span class="dirty-badge">
												<CircleDot size={10} />
												{fileCount || '...'} dirty
											</span>
										{/if}
									</div>
								</button>
								<!-- Expanded file list -->
								{#if isExpanded && canExpand}
									{@const summaryFiles = getSummaryFiles(repo)}
									<div class="expanded-body" transition:slide={{ duration: 300, easing: cubicOut }}>
										<div class="expanded-header">
											<CircleDot size={13} class="icon-yellow" />
											<span class="expanded-label">Local Changes</span>
											<span class="uncommitted-badge">uncommitted</span>
										</div>

										{#if isLoading}
											<div class="file-loading">Loading file diffs...</div>
										{:else}
											<div class="file-list">
												{#each summaryFiles as file (file.path)}
													{@const fileKey = `${repo.id}:${file.path}`}
													{@const isFileOpen = expandedFiles.has(fileKey)}
													{@const FileIcon = getFileKindIcon(file.status)}
													<div class="file-entry">
														<button
															type="button"
															class="file-row"
															class:file-open={isFileOpen}
															onclick={() => toggleFile(repo.id, file)}
														>
															<span class="file-chevron" class:open={isFileOpen}>
																<ChevronDown size={11} />
															</span>
															<span
																class="file-kind-icon"
																style="color: {getFileKindColor(file.status)}"
															>
																<FileIcon size={13} />
															</span>
															<span class="file-path">{file.path}</span>
															{#if file.added > 0}
																<span class="file-add">+{file.added}</span>
															{/if}
															{#if file.removed > 0}
																<span class="file-del">-{file.removed}</span>
															{/if}
														</button>

														<!-- Inline diff -->
														{#if isFileOpen}
															<div
																class="inline-diff"
																transition:slide={{ duration: 250, easing: cubicOut }}
															>
																{#if fileDiffLoading[fileKey]}
																	<div class="diff-loading">Loading diff...</div>
																{:else if fileDiffs[fileKey]}
																	<diffs-container
																		class="diff-container"
																		bind:this={diffContainers[fileKey]}
																	></diffs-container>
																{:else}
																	<div class="diff-loading">No diff available</div>
																{/if}
															</div>
														{/if}
													</div>
												{/each}
											</div>

											<div class="expanded-footer">
												<span class="file-add">
													+{summaryFiles.reduce((s, f) => s + f.added, 0)}
												</span>
												{#if summaryFiles.reduce((s, f) => s + f.removed, 0) > 0}
													<span class="file-del">
														-{summaryFiles.reduce((s, f) => s + f.removed, 0)}
													</span>
												{/if}
												<span
													>{summaryFiles.length} file{summaryFiles.length !== 1 ? 's' : ''} changed</span
												>
											</div>
										{/if}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	/* ─── Shell ─── */
	.command-center {
		display: flex;
		flex-direction: column;
		height: 100%;
		padding: 24px;
		gap: 0;
		overflow: hidden;
		background: color-mix(in srgb, var(--bg) 90%, transparent);
	}

	/* ─── Header ─── */
	.cc-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: 28px;
	}

	.cc-header h1 {
		margin: 0;
		font-size: var(--text-3xl);
		font-weight: 600;
		color: var(--text);
		letter-spacing: -0.01em;
	}

	.cc-header p {
		margin: 4px 0 0;
		font-size: var(--text-base);
		color: var(--muted);
	}

	.ws-name {
		font-family: var(--font-mono);
		color: var(--accent);
	}

	.header-actions {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.daemon-badge {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 5px 12px;
		border-radius: 999px;
		border: 1px solid color-mix(in srgb, var(--accent) 20%, transparent);
		background: color-mix(in srgb, var(--accent) 10%, transparent);
		color: var(--accent);
		font-size: var(--text-xs);
		font-weight: 500;
	}

	.daemon-ping {
		position: relative;
		display: flex;
		width: 8px;
		height: 8px;
	}

	.ping-ring {
		position: absolute;
		inset: 0;
		border-radius: 50%;
		background: var(--accent);
		opacity: 0.75;
		animation: ping 2s cubic-bezier(0, 0, 0.2, 1) infinite;
	}

	.ping-dot {
		position: relative;
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--accent);
	}

	@keyframes ping {
		75%,
		100% {
			transform: scale(2);
			opacity: 0;
		}
	}

	.refresh-btn {
		padding: 8px;
		border-radius: 8px;
		border: none;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition: all var(--transition-fast);
	}

	.refresh-btn:hover {
		color: var(--text);
		background: var(--panel-strong);
	}

	/* ─── Stats ─── */
	.stats {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 14px;
		margin-bottom: 28px;
	}

	.stat-card {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 14px 16px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--panel);
		transition: border-color 200ms;
	}

	.stat-card:hover {
		border-color: color-mix(in srgb, var(--accent) 30%, var(--border));
	}

	.stat-card.accent {
		border-color: color-mix(in srgb, var(--yellow) 20%, var(--border));
	}

	.stat-card.accent:hover {
		border-color: color-mix(in srgb, var(--yellow) 40%, var(--border));
	}

	.stat-text {
		display: grid;
		gap: 4px;
	}

	.stat-label {
		font-size: var(--text-xs);
		text-transform: uppercase;
		letter-spacing: 0.06em;
		font-weight: 500;
		color: var(--muted);
	}

	.stat-text strong {
		font-size: var(--text-2xl);
		font-weight: 600;
		color: var(--text);
	}

	.stat-icon {
		width: 40px;
		height: 40px;
		border-radius: 50%;
		background: var(--panel-strong);
		display: grid;
		place-items: center;
		opacity: 0.8;
		transition: opacity 200ms;
	}

	.stat-card:hover .stat-icon {
		opacity: 1;
	}

	.stat-icon :global(.icon-green) {
		color: var(--success);
	}
	.stat-icon :global(.icon-blue) {
		color: var(--accent);
	}
	.stat-icon :global(.icon-yellow) {
		color: var(--yellow);
	}
	.stat-icon :global(.icon-purple) {
		color: var(--purple);
	}

	/* ─── Repo Panel ─── */
	.repo-panel {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		overflow: hidden;
		box-shadow: 0 12px 40px rgba(0, 0, 0, 0.3);
	}

	.panel-toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 16px;
		border-bottom: 1px solid var(--border);
		background: var(--panel-soft);
	}

	.panel-title {
		font-size: var(--text-base);
		font-weight: 500;
		color: var(--text);
	}

	.panel-search {
		position: relative;
		display: inline-flex;
		align-items: center;
	}

	.panel-search :global(svg) {
		position: absolute;
		left: 10px;
		color: var(--muted);
		pointer-events: none;
	}

	.panel-search input {
		width: 240px;
		height: 32px;
		background: var(--panel-strong);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0 12px 0 32px;
		color: var(--text);
		font-size: var(--text-base);
		transition: border-color var(--transition-fast);
	}

	.panel-search input::placeholder {
		color: color-mix(in srgb, var(--muted) 50%, transparent);
	}

	.panel-search input:focus {
		outline: none;
		border-color: color-mix(in srgb, var(--accent) 50%, var(--border));
	}

	.panel-content {
		flex: 1;
		overflow-y: auto;
		padding: 16px;
	}

	/* ─── Repo Grid ─── */
	.repo-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 14px;
	}

	.repo-card {
		border-radius: 12px;
		border: 1px solid var(--border-strong);
		background: var(--panel-strong);
		overflow: hidden;
		transition: all 200ms;
	}

	.repo-card:hover {
		border-color: color-mix(in srgb, var(--accent) 30%, var(--border-strong));
	}

	.repo-card.modified {
		border-color: color-mix(in srgb, var(--yellow) 20%, var(--border-strong));
	}

	.repo-card.modified:hover {
		border-color: color-mix(in srgb, var(--yellow) 40%, var(--border-strong));
	}

	.repo-card.error {
		border-color: color-mix(in srgb, var(--status-error) 20%, var(--border-strong));
	}

	.repo-card.error:hover {
		border-color: color-mix(in srgb, var(--status-error) 40%, var(--border-strong));
	}

	.repo-card.expanded {
		grid-column: 1 / -1;
	}

	/* Card header */
	.repo-header {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 14px 16px;
		width: 100%;
		text-align: left;
		border: none;
		background: transparent;
		color: inherit;
		cursor: default;
	}

	.repo-header.clickable {
		cursor: pointer;
	}

	.repo-header.clickable:hover {
		background: color-mix(in srgb, var(--panel) 50%, transparent);
	}

	.repo-title-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}

	.repo-name-group {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.repo-name-group h3 {
		margin: 0;
		font-size: var(--text-base);
		font-weight: 500;
		color: var(--text);
		white-space: nowrap;
	}

	.repo-branch {
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		color: var(--subtle);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.repo-badges {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-shrink: 0;
	}

	.status-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
	}

	.status-dot.clean {
		background: var(--success);
		box-shadow: 0 0 8px rgba(134, 196, 66, 0.4);
	}

	.status-dot.modified {
		background: var(--yellow);
		box-shadow: 0 0 8px rgba(245, 158, 11, 0.4);
	}

	.status-dot.ahead {
		background: var(--accent);
		box-shadow: 0 0 8px rgba(45, 140, 255, 0.4);
	}

	.status-dot.error {
		background: var(--status-error);
		box-shadow: 0 0 8px rgba(239, 68, 68, 0.4);
	}

	.chevron {
		color: var(--subtle);
		transition: transform 200ms;
	}

	.chevron.open {
		transform: rotate(180deg);
	}

	/* Meta row */
	.repo-meta-row {
		display: flex;
		align-items: center;
		gap: 14px;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.meta-pair {
		display: inline-flex;
		align-items: center;
		gap: 3px;
	}

	.meta-pair.highlight-blue {
		color: var(--accent);
	}
	.meta-pair.highlight-yellow {
		color: var(--yellow);
	}

	.dirty-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-size: var(--text-xs);
		color: var(--yellow);
	}

	/* ─── Expanded body ─── */
	.expanded-body {
		border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
		background: color-mix(in srgb, var(--panel-soft) 60%, transparent);
	}

	.expanded-header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 16px 6px;
	}

	.expanded-header :global(.icon-yellow) {
		color: var(--yellow);
	}

	.expanded-label {
		font-size: var(--text-xs);
		font-weight: 500;
		color: var(--yellow);
	}

	.uncommitted-badge {
		font-size: var(--text-xs);
		color: var(--subtle);
		background: var(--panel-strong);
		padding: 2px 6px;
		border-radius: 4px;
		border: 1px solid var(--border);
	}

	.file-loading,
	.diff-loading {
		padding: 12px 16px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	.file-list {
		padding: 4px 8px;
		display: grid;
		gap: 2px;
		max-height: 50vh;
		overflow-y: auto;
	}

	.file-entry {
		border-radius: 8px;
	}

	.file-row {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		border-radius: 8px;
		border: none;
		background: none;
		width: 100%;
		text-align: left;
		cursor: pointer;
		color: inherit;
		font-family: inherit;
		transition: background 80ms;
	}

	.file-row:hover {
		background: var(--panel-strong);
	}

	.file-row.file-open {
		background: color-mix(in srgb, var(--panel-strong) 60%, transparent);
	}

	.file-chevron {
		display: flex;
		align-items: center;
		color: var(--subtle);
		transform: rotate(-90deg);
		transition: transform 200ms ease;
	}

	.file-chevron.open {
		transform: rotate(0deg);
	}

	.file-kind-icon {
		display: flex;
		flex-shrink: 0;
	}

	.file-path {
		flex: 1;
		min-width: 0;
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		color: var(--text);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.file-add {
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		color: var(--success);
		flex-shrink: 0;
	}

	.file-del {
		font-family: var(--font-mono);
		font-size: var(--text-mono-xs);
		color: var(--status-error);
		flex-shrink: 0;
	}

	/* ─── Inline diff ─── */
	.inline-diff {
		margin: 2px 8px 8px;
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
		background: var(--bg);
	}

	diffs-container {
		display: block;
	}

	.diff-container {
		font-size: var(--text-sm);
		max-height: 400px;
		overflow: auto;
	}

	.expanded-footer {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 8px 16px;
		border-top: 1px solid color-mix(in srgb, var(--border) 30%, transparent);
		font-size: var(--text-xs);
		color: var(--subtle);
	}

	/* ─── Empty / CTA ─── */
	.empty-card {
		flex: 1;
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 28px;
		display: grid;
		gap: 10px;
		justify-items: start;
		align-content: start;
	}

	.empty-card h2 {
		margin: 0;
		font-size: var(--text-2xl);
		color: var(--text);
	}

	.empty-card p {
		margin: 0;
		font-size: var(--text-base);
		color: var(--muted);
	}

	.cta {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 9px 14px;
		border-radius: 8px;
		border: none;
		background: var(--accent);
		color: white;
		font-size: var(--text-base);
		font-weight: 500;
		cursor: pointer;
		box-shadow: 0 4px 16px color-mix(in srgb, var(--accent) 20%, transparent);
	}

	.no-results {
		padding: 20px;
		font-size: var(--text-base);
		color: var(--muted);
		text-align: center;
	}

	/* ─── Responsive ─── */
	@media (max-width: 1200px) {
		.repo-grid {
			grid-template-columns: repeat(2, 1fr);
		}
	}

	@media (max-width: 980px) {
		.stats {
			grid-template-columns: repeat(2, 1fr);
		}
	}

	@media (max-width: 700px) {
		.command-center {
			padding: 16px;
		}

		.cc-header {
			flex-direction: column;
			gap: 12px;
		}

		.stats {
			grid-template-columns: 1fr;
		}

		.repo-grid {
			grid-template-columns: 1fr;
		}

		.panel-toolbar {
			flex-direction: column;
			gap: 8px;
			align-items: stretch;
		}

		.panel-search input {
			width: 100%;
		}
	}
</style>
