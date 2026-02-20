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
		onAddRepo?: (workspaceId: string) => void;
	}

	const {
		workspaces,
		activeWorkspaceId,
		onCreateWorkspace = () => {},
		onSelectRepo,
		onAddRepo = () => {},
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
		<section class="empty-card ws-empty-state">
			<h2>No workspace selected</h2>
			<p class="ws-empty-state-copy">
				Select a workspace from the Hub or create a new one to see repository status.
			</p>
			<button type="button" class="cta" onclick={onCreateWorkspace}>
				<Plus size={15} /> Create Workspace
			</button>
		</section>
	{:else if activeWorkspace.repos.length === 0}
		<section class="empty-card ws-empty-state">
			<h2>No repos linked</h2>
			<p class="ws-empty-state-copy">
				Add repositories to this workspace to track branch health and drift.
			</p>
			<button type="button" class="cta" onclick={() => onAddRepo(activeWorkspaceId ?? '')}>
				<Plus size={15} /> Add Repo
			</button>
		</section>
	{:else}
		<!-- Repo grid panel -->
		<div class="repo-panel">
			<div class="panel-toolbar">
				<span class="panel-title">{activeWorkspaceName}</span>
				<div class="panel-actions">
					<button
						type="button"
						class="add-repo-btn"
						onclick={() => onAddRepo(activeWorkspaceId ?? '')}
					>
						<Plus size={14} />
						Add Repo
					</button>
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
											<span class={`status-dot ws-dot ws-dot-md ws-dot-${statusClass}`}></span>
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
																<span class="file-add ws-diffstat-add">+{file.added}</span>
															{/if}
															{#if file.removed > 0}
																<span class="file-del ws-diffstat-del">-{file.removed}</span>
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
												<span class="file-add ws-diffstat-add">
													+{summaryFiles.reduce((s, f) => s + f.added, 0)}
												</span>
												{#if summaryFiles.reduce((s, f) => s + f.removed, 0) > 0}
													<span class="file-del ws-diffstat-del">
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

<style src="./CommandCenterView.css"></style>
