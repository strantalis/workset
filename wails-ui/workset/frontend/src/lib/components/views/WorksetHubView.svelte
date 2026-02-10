<script lang="ts">
	import {
		AlertCircle,
		ArrowLeft,
		ArrowUpRight,
		Archive,
		ArchiveRestore,
		Box,
		Clock,
		Command,
		Eye,
		EyeOff,
		FolderGit2,
		GitBranch,
		GitPullRequest,
		Layers,
		LayoutGrid,
		List,
		MoreHorizontal,
		Pin,
		PinOff,
		Plus,
		Search,
	} from '@lucide/svelte';
	import { clickOutside } from '../../actions/clickOutside';
	import type { WorksetSummary } from '../../view-models/worksetViewModel';

	type GroupMode = 'all' | 'template' | 'repo' | 'active';
	type LayoutMode = 'grid' | 'list';
	type WorksetGroup = {
		label: string;
		items: WorksetSummary[];
	};

	interface Props {
		worksets: WorksetSummary[];
		shortcutMap?: Map<string, number>;
		activeWorkspaceId: string | null;
		onSelectWorkspace: (workspaceId: string) => void;
		onCreateWorkspace: () => void;
		onTogglePin: (workspaceId: string, nextPinned: boolean) => void;
		onToggleArchived: (workspaceId: string, archived: boolean) => void;
		onOpenPopout: (workspaceId: string) => void;
		onClosePopout: (workspaceId: string) => void;
		isWorkspacePoppedOut: (workspaceId: string) => boolean;
	}

	const {
		worksets,
		shortcutMap,
		activeWorkspaceId,
		onSelectWorkspace,
		onCreateWorkspace,
		onTogglePin,
		onToggleArchived,
		onOpenPopout,
		onClosePopout,
		isWorkspacePoppedOut,
	}: Props = $props();

	const GROUP_MODES: Array<{ id: GroupMode; label: string; icon: typeof LayoutGrid }> = [
		{ id: 'all', label: 'All', icon: LayoutGrid },
		{ id: 'template', label: 'Template', icon: Layers },
		{ id: 'repo', label: 'Repo', icon: FolderGit2 },
		{ id: 'active', label: 'Active', icon: Clock },
	];

	let searchQuery = $state('');
	let groupMode = $state<GroupMode>('all');
	let layoutMode = $state<LayoutMode>('grid');
	let showArchived = $state(false);
	let actionMenuFor = $state<string | null>(null);

	const sortWorksets = (items: WorksetSummary[]): WorksetSummary[] =>
		[...items].sort((left, right) => {
			if (left.pinned !== right.pinned) return left.pinned ? -1 : 1;
			return right.lastActiveTs - left.lastActiveTs;
		});

	const groupByActivity = (items: WorksetSummary[]): WorksetGroup[] => {
		const dayMs = 24 * 60 * 60 * 1000;
		const now = Date.now();
		const today: WorksetSummary[] = [];
		const thisWeek: WorksetSummary[] = [];
		const older: WorksetSummary[] = [];

		for (const item of items) {
			const age = now - item.lastActiveTs;
			if (age < dayMs) {
				today.push(item);
				continue;
			}
			if (age < dayMs * 7) {
				thisWeek.push(item);
				continue;
			}
			older.push(item);
		}

		return [
			{ label: 'Today', items: sortWorksets(today) },
			{ label: 'This Week', items: sortWorksets(thisWeek) },
			{ label: 'Older', items: sortWorksets(older) },
		].filter((group) => group.items.length > 0);
	};

	const filtered = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		if (!query) {
			return worksets;
		}
		return worksets.filter((item) => {
			const haystack =
				`${item.label} ${item.description} ${item.template} ${item.branch} ${item.repos.join(
					' ',
				)}`.toLowerCase();
			return haystack.includes(query);
		});
	});

	const visible = $derived.by(() =>
		showArchived ? filtered : filtered.filter((item) => !item.archived),
	);

	const groups = $derived.by<WorksetGroup[]>(() => {
		if (groupMode === 'all') {
			return [{ label: '', items: sortWorksets(visible) }];
		}

		if (groupMode === 'template') {
			const templateMap = new Map<string, WorksetSummary[]>();
			for (const item of visible) {
				const bucket = templateMap.get(item.template) ?? [];
				bucket.push(item);
				templateMap.set(item.template, bucket);
			}
			return [...templateMap.entries()]
				.sort(([left], [right]) => left.localeCompare(right))
				.map(([label, items]) => ({ label, items: sortWorksets(items) }));
		}

		if (groupMode === 'repo') {
			const repoMap = new Map<string, WorksetSummary[]>();
			for (const item of visible) {
				for (const repoName of item.repos) {
					const bucket = repoMap.get(repoName) ?? [];
					if (!bucket.some((entry) => entry.id === item.id)) {
						bucket.push(item);
					}
					repoMap.set(repoName, bucket);
				}
			}
			return [...repoMap.entries()]
				.sort((left, right) => {
					const byCount = right[1].length - left[1].length;
					if (byCount !== 0) return byCount;
					return left[0].localeCompare(right[0]);
				})
				.map(([label, items]) => ({ label, items: sortWorksets(items) }));
		}

		return groupByActivity(visible);
	});

	const visibleCatalog = $derived.by(() =>
		showArchived ? worksets : worksets.filter((item) => !item.archived),
	);
	const totalWorksets = $derived(visibleCatalog.length);
	const totalRepos = $derived.by(
		() =>
			new Set(visibleCatalog.flatMap((item) => item.repos.map((repo) => repo.toLowerCase()))).size,
	);
	const totalPrs = $derived(visibleCatalog.reduce((acc, item) => acc + item.openPrs, 0));
	const totalDirty = $derived(visibleCatalog.reduce((acc, item) => acc + item.dirtyCount, 0));
	const totalPinned = $derived(visibleCatalog.filter((item) => item.pinned).length);
	const archivedCount = $derived(worksets.filter((item) => item.archived).length);

	const getShortcutNumber = (workspaceId: string): number | undefined =>
		shortcutMap?.get(workspaceId);

	let menuClosedAt = 0;

	const closeActionMenu = (): void => {
		actionMenuFor = null;
		menuClosedAt = Date.now();
	};

	const toggleActionMenu = (workspaceId: string, event: MouseEvent): void => {
		event.stopPropagation();
		// Guard against clickOutside (capture phase) closing + toggle reopening in the same event cycle
		if (Date.now() - menuClosedAt < 50) return;
		actionMenuFor = actionMenuFor === workspaceId ? null : workspaceId;
	};

	const openWorkspace = (workspaceId: string): void => {
		closeActionMenu();
		onSelectWorkspace(workspaceId);
	};

	const handleWorksetKeyboard = (event: KeyboardEvent, workspaceId: string): void => {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		openWorkspace(workspaceId);
	};

	const handleTogglePin = (item: WorksetSummary, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		onTogglePin(item.id, !item.pinned);
	};

	const handleToggleArchive = (item: WorksetSummary, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		onToggleArchived(item.id, item.archived);
	};

	const handleOpenPopout = (workspaceId: string, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		onOpenPopout(workspaceId);
	};

	const handleClosePopout = (workspaceId: string, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		onClosePopout(workspaceId);
	};
</script>

<div class="hub-shell">
	<header class="hub-header">
		<div class="title-wrap">
			<h1>Worksets</h1>
			<p>Your units of intent — each one owns repos, branches, PRs, and agent activity.</p>
		</div>
		<button type="button" class="new-workset-btn" onclick={onCreateWorkspace}
			><Plus size={16} /> New Workset</button
		>

		<div class="stats-row">
			<div class="stat-pill">
				<div class="dot accent"></div>
				<span>Worksets</span>
				<strong>{totalWorksets}</strong>
			</div>
			<div class="stat-pill">
				<div class="dot violet"></div>
				<span>Repos</span>
				<strong>{totalRepos}</strong>
			</div>
			<div class="stat-pill">
				<div class="dot success"></div>
				<span>Open PRs</span>
				<strong>{totalPrs}</strong>
			</div>
			{#if totalDirty > 0}
				<div class="stat-pill">
					<div class="dot gold"></div>
					<span>Dirty</span>
					<strong>{totalDirty}</strong>
				</div>
			{/if}
			<div class="stat-pill">
				<div class="dot gold"></div>
				<span>Pinned</span>
				<strong>{totalPinned}</strong>
			</div>
		</div>

		<div class="toolbar">
			<label class="search-wrap">
				<Search size={15} />
				<input
					type="text"
					placeholder="Search worksets, repos, templates..."
					bind:value={searchQuery}
				/>
			</label>

			<div class="segmented-control" role="radiogroup" aria-label="Group mode">
				{#each GROUP_MODES as mode (mode.id)}
					<button
						type="button"
						class="segment"
						class:active={groupMode === mode.id}
						onclick={() => (groupMode = mode.id)}
					>
						<mode.icon size={13} />
						{mode.label}
					</button>
				{/each}
			</div>

			<div class="segmented-control icon-only" aria-label="Layout mode">
				<button
					type="button"
					class="segment icon"
					class:active={layoutMode === 'grid'}
					onclick={() => (layoutMode = 'grid')}
					aria-label="Grid layout"
				>
					<LayoutGrid size={14} />
				</button>
				<button
					type="button"
					class="segment icon"
					class:active={layoutMode === 'list'}
					onclick={() => (layoutMode = 'list')}
					aria-label="List layout"
				>
					<List size={14} />
				</button>
			</div>

			{#if archivedCount > 0}
				<button
					type="button"
					class="archived-toggle"
					class:active={showArchived}
					onclick={() => (showArchived = !showArchived)}
				>
					{#if showArchived}
						<EyeOff size={13} />
					{:else}
						<Eye size={13} />
					{/if}
					{archivedCount} archived
				</button>
			{/if}
		</div>
	</header>

	<section class="content">
		{#if groups.length === 0 || groups.every((group) => group.items.length === 0)}
			<div class="empty-state">
				<Search size={32} />
				<p>No worksets match "{searchQuery}"</p>
			</div>
		{:else}
			<div class="groups">
				{#each groups as group (group.label || 'all')}
					<div class="group">
						{#if group.label}
							<div class="group-header">
								<Layers size={14} />
								<h2>{group.label}</h2>
								<span>{group.items.length} workset{group.items.length !== 1 ? 's' : ''}</span>
							</div>
						{/if}

						{#if layoutMode === 'grid'}
							<div class="grid">
								{#each group.items as item (item.id)}
									<div
										class="workset-card"
										class:active={activeWorkspaceId === item.id}
										class:archived={item.archived}
										role="button"
										tabindex="0"
										onclick={() => openWorkspace(item.id)}
										onkeydown={(event) => handleWorksetKeyboard(event, item.id)}
									>
										{#if activeWorkspaceId === item.id}
											<div class="active-bar"></div>
										{/if}

										<div class="card-body">
											<div class="card-head">
												<div class="card-icon" class:active={activeWorkspaceId === item.id}>
													<Box size={18} />
												</div>

												<div class="card-title">
													<div class="card-title-row">
														<h3>{item.label}</h3>
														{#if item.pinned}
															<Pin size={10} class="pin-indicator" />
														{/if}
														{#if activeWorkspaceId === item.id}
															<span class="badge active">Active</span>
														{/if}
														{#if item.archived}
															<span class="badge archived">Archived</span>
														{/if}
													</div>
													<span>{item.template}</span>
												</div>

												<div class="item-actions">
													<button
														type="button"
														class="popout-trigger"
														aria-label={isWorkspacePoppedOut(item.id)
															? 'Return workspace to main window'
															: 'Open workspace popout'}
														title={isWorkspacePoppedOut(item.id)
															? 'Return to main window'
															: 'Open workspace popout'}
														onclick={(event) =>
															isWorkspacePoppedOut(item.id)
																? handleClosePopout(item.id, event)
																: handleOpenPopout(item.id, event)}
													>
														{#if isWorkspacePoppedOut(item.id)}
															<ArrowLeft size={13} />
														{:else}
															<ArrowUpRight size={13} />
														{/if}
													</button>
													<div class="menu-anchor">
														<button
															type="button"
															class="menu-trigger"
															aria-label="Workset actions"
															aria-expanded={actionMenuFor === item.id}
															onclick={(event) => toggleActionMenu(item.id, event)}
														>
															<MoreHorizontal size={14} />
														</button>
														{#if actionMenuFor === item.id}
															<div
																class="action-menu"
																role="menu"
																use:clickOutside={{ callback: closeActionMenu }}
															>
																<button
																	type="button"
																	class:item-pinned={item.pinned}
																	onclick={(event) => handleTogglePin(item, event)}
																>
																	{#if item.pinned}
																		<PinOff size={13} />
																		Unpin
																	{:else}
																		<Pin size={13} />
																		Pin to top
																	{/if}
																</button>
																<button
																	type="button"
																	class="item-archive"
																	class:item-archived={item.archived}
																	onclick={(event) => handleToggleArchive(item, event)}
																>
																	{#if item.archived}
																		<ArchiveRestore size={13} />
																		Unarchive
																	{:else}
																		<Archive size={13} />
																		Archive
																	{/if}
																</button>
															</div>
														{/if}
													</div>
												</div>
											</div>

											{#if item.description}
												<p class="card-description">{item.description}</p>
											{/if}

											<div class="branch-chip">
												<GitBranch size={11} />
												<span>{item.branch}</span>
											</div>

											<div class="repo-chips">
												{#each item.repos.slice(0, 4) as repoName (repoName)}
													<span>{repoName}</span>
												{/each}
												{#if item.repos.length > 4}
													<span class="muted">+{item.repos.length - 4} more</span>
												{/if}
											</div>

											<div class="health-row">
												{#each item.health as status, index (`${item.id}-health-${index}`)}
													<span class="dot {status}" title={status}></span>
												{/each}
												<span class="diff"
													><span class="plus">+{item.linesAdded}</span>
													<span class="minus">-{item.linesRemoved}</span></span
												>
											</div>
										</div>

										<div class="card-footer">
											<div class="footer-meta">
												<span class="prs"><GitPullRequest size={10} /> {item.openPrs}</span>
												<span class="dirty"><AlertCircle size={10} /> {item.dirtyCount} dirty</span>
												<span><Clock size={10} /> {item.lastActive}</span>
											</div>
											<div class="footer-actions">
												{#if getShortcutNumber(item.id)}
													<kbd><Command size={7} />{getShortcutNumber(item.id)}</kbd>
												{/if}
											</div>
										</div>
									</div>
								{/each}
							</div>
						{:else}
							<div class="list">
								{#each group.items as item (item.id)}
									<div
										class="list-row"
										class:active={activeWorkspaceId === item.id}
										class:archived={item.archived}
										role="button"
										tabindex="0"
										onclick={() => openWorkspace(item.id)}
										onkeydown={(event) => handleWorksetKeyboard(event, item.id)}
									>
										<div class="row-icon" class:active={activeWorkspaceId === item.id}>
											<Box size={16} />
										</div>

										<div class="row-title">
											<div class="row-title-line">
												<strong>{item.label}</strong>
												{#if item.pinned}
													<Pin size={10} class="pin-indicator" />
												{/if}
												{#if activeWorkspaceId === item.id}
													<span class="badge active">Active</span>
												{/if}
												{#if item.archived}
													<span class="badge archived">Archived</span>
												{/if}
												<span class="badge template">{item.template}</span>
											</div>
											{#if item.description}
												<p>{item.description}</p>
											{/if}
										</div>

										<div class="row-branch">
											<GitBranch size={10} />
											{item.branch}
										</div>

										<div class="row-repo-count">
											<FolderGit2 size={11} />
											{item.repoCount}
										</div>

										<div class="row-health">
											{#each item.health as status, index (`${item.id}-list-health-${index}`)}
												<span class="dot {status}"></span>
											{/each}
										</div>

										<div class="row-stats">
											<span class="diff"
												><span class="plus">+{item.linesAdded}</span>
												<span class="minus">-{item.linesRemoved}</span></span
											>
											<span class="prs"><GitPullRequest size={10} /> {item.openPrs}</span>
											<span class="dirty">{item.dirtyCount} dirty</span>
											<span>{item.lastActive}</span>
										</div>

										{#if getShortcutNumber(item.id)}
											<kbd><Command size={7} />{getShortcutNumber(item.id)}</kbd>
										{/if}

										<div class="item-actions">
											<button
												type="button"
												class="popout-trigger"
												aria-label={isWorkspacePoppedOut(item.id)
													? 'Return workspace to main window'
													: 'Open workspace popout'}
												title={isWorkspacePoppedOut(item.id)
													? 'Return to main window'
													: 'Open workspace popout'}
												onclick={(event) =>
													isWorkspacePoppedOut(item.id)
														? handleClosePopout(item.id, event)
														: handleOpenPopout(item.id, event)}
											>
												{#if isWorkspacePoppedOut(item.id)}
													<ArrowLeft size={13} />
												{:else}
													<ArrowUpRight size={13} />
												{/if}
											</button>
											<div class="menu-anchor">
												<button
													type="button"
													class="menu-trigger"
													aria-label="Workset actions"
													aria-expanded={actionMenuFor === item.id}
													onclick={(event) => toggleActionMenu(item.id, event)}
												>
													<MoreHorizontal size={14} />
												</button>
												{#if actionMenuFor === item.id}
													<div
														class="action-menu"
														role="menu"
														use:clickOutside={{ callback: closeActionMenu }}
													>
														<button
															type="button"
															class:item-pinned={item.pinned}
															onclick={(event) => handleTogglePin(item, event)}
														>
															{#if item.pinned}
																<PinOff size={13} />
																Unpin
															{:else}
																<Pin size={13} />
																Pin to top
															{/if}
														</button>
														<button
															type="button"
															class="item-archive"
															class:item-archived={item.archived}
															onclick={(event) => handleToggleArchive(item, event)}
														>
															{#if item.archived}
																<ArchiveRestore size={13} />
																Unarchive
															{:else}
																<Archive size={13} />
																Archive
															{/if}
														</button>
													</div>
												{/if}
											</div>
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	</section>
</div>

<style>
	.hub-shell {
		--hub-bg: color-mix(in srgb, var(--bg) 90%, black);
		--hub-surface: color-mix(in srgb, var(--panel) 78%, transparent);
		--hub-surface-strong: color-mix(in srgb, var(--panel-strong) 90%, transparent);
		--hub-border: color-mix(in srgb, var(--border) 84%, transparent);
		--hub-muted: color-mix(in srgb, var(--muted) 82%, white);
		--hub-violet: #8b8aed;
		--hub-gold: #f5a623;
		--hub-shadow: 0 14px 32px rgba(0, 0, 0, 0.24);
		height: 100%;
		display: flex;
		flex-direction: column;
		background: var(--hub-bg);
	}

	.hub-header {
		position: relative;
		padding: 28px 32px 0;
		flex-shrink: 0;
	}

	.title-wrap {
		display: grid;
		gap: 8px;
		margin-bottom: 24px;
	}

	h1 {
		margin: 0;
		font-size: var(--text-3xl);
		line-height: 1.15;
		letter-spacing: -0.015em;
		font-weight: 600;
		color: var(--text);
	}

	.title-wrap p {
		margin: 0;
		font-size: var(--text-base);
		line-height: 1.45;
		color: var(--hub-muted);
	}

	.new-workset-btn {
		position: absolute;
		top: 28px;
		right: 32px;
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 10px 14px;
		border-radius: 10px;
		border: none;
		background: var(--accent);
		color: #fff;
		font-size: var(--text-base);
		font-weight: 600;
		cursor: pointer;
		box-shadow: 0 12px 24px rgba(var(--accent-rgb), 0.24);
	}

	.new-workset-btn:hover {
		filter: brightness(1.06);
	}

	.stats-row {
		display: flex;
		flex-wrap: wrap;
		gap: 10px;
		margin-bottom: 20px;
	}

	.stat-pill {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 8px 11px;
		border-radius: 10px;
		border: 1px solid var(--hub-border);
		background: var(--hub-surface);
		font-size: var(--text-xs);
	}

	.stat-pill span {
		color: var(--hub-muted);
	}

	.stat-pill strong {
		font-size: var(--text-sm);
		color: var(--text);
	}

	.dot {
		width: 8px;
		height: 8px;
		border-radius: 999px;
	}

	.dot.accent,
	.dot.ahead {
		background: var(--accent);
	}

	.dot.violet {
		background: var(--hub-violet);
	}

	.dot.success,
	.dot.clean {
		background: var(--success);
	}

	.dot.warning,
	.dot.modified {
		background: var(--warning);
	}

	.dot.gold,
	.dot.dirty {
		background: var(--hub-gold);
	}

	.dot.error {
		background: var(--danger);
	}

	.toolbar {
		display: flex;
		align-items: center;
		gap: 10px;
		margin-bottom: 14px;
		flex-wrap: wrap;
	}

	.search-wrap {
		flex: 1;
		min-width: 240px;
		max-width: 460px;
		display: inline-flex;
		align-items: center;
		gap: 9px;
		padding: 9px 12px;
		border-radius: 10px;
		background: color-mix(in srgb, var(--panel) 94%, transparent);
		border: 1px solid var(--hub-border);
		color: var(--hub-muted);
	}

	.search-wrap input {
		flex: 1;
		min-width: 0;
		border: none;
		background: transparent;
		color: var(--text);
		font-size: var(--text-base);
	}

	.search-wrap input:focus {
		outline: none;
	}

	.segmented-control {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		padding: 3px;
		border-radius: 10px;
		border: 1px solid var(--hub-border);
		background: color-mix(in srgb, var(--panel) 94%, transparent);
	}

	.segment {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		border: 1px solid transparent;
		border-radius: 8px;
		background: transparent;
		color: color-mix(in srgb, var(--muted) 76%, white);
		font-size: var(--text-xs);
		font-weight: 600;
		cursor: pointer;
	}

	.segment.active {
		background: color-mix(in srgb, var(--panel-strong) 82%, transparent);
		border-color: color-mix(in srgb, var(--border) 64%, transparent);
		color: var(--text);
	}

	.icon-only .segment.icon {
		padding: 6px;
	}

	.archived-toggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		border-radius: 10px;
		border: 1px solid var(--hub-border);
		background: color-mix(in srgb, var(--panel) 94%, transparent);
		color: color-mix(in srgb, var(--muted) 74%, white);
		font-size: var(--text-xs);
		font-weight: 600;
		cursor: pointer;
	}

	.archived-toggle.active {
		background: color-mix(in srgb, var(--panel-strong) 84%, transparent);
		color: var(--text);
	}

	.content {
		flex: 1;
		overflow: auto;
		padding: 20px 32px 32px;
	}

	.groups {
		display: grid;
		gap: 28px;
	}

	.group-header {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 12px;
		color: var(--accent);
	}

	.group-header h2 {
		margin: 0;
		font-size: var(--text-md);
		font-weight: 700;
		color: var(--text);
	}

	.group-header span {
		font-size: var(--text-sm);
		color: var(--success);
	}

	.grid {
		display: grid;
		gap: 14px;
		grid-template-columns: repeat(3, minmax(0, 1fr));
	}

	.workset-card {
		position: relative;
		display: flex;
		flex-direction: column;
		min-height: 240px;
		border-radius: 14px;
		border: 1px solid var(--hub-border);
		background: var(--hub-surface);
		cursor: pointer;
		transition:
			border-color var(--transition-fast),
			background var(--transition-fast),
			transform var(--transition-fast);
		box-shadow: var(--inset-highlight), var(--shadow-sm);
	}

	.workset-card:hover {
		border-color: color-mix(in srgb, var(--accent) 28%, var(--hub-border));
		background: color-mix(in srgb, var(--panel-strong) 82%, transparent);
		transform: translateY(-1px);
	}

	.workset-card.active {
		border-color: color-mix(in srgb, var(--accent) 42%, var(--hub-border));
		background: color-mix(in srgb, var(--panel-strong) 88%, transparent);
		box-shadow: var(--inset-highlight), var(--hub-shadow);
	}

	.workset-card.archived {
		opacity: 0.58;
	}

	.active-bar {
		position: absolute;
		top: 0;
		left: 16px;
		right: 16px;
		height: 2px;
		background: var(--accent);
		border-radius: 0 0 999px 999px;
	}

	.card-body {
		padding: 16px 16px 14px;
		display: grid;
		gap: 8px;
		flex: 1;
	}

	.card-head {
		display: flex;
		align-items: flex-start;
		gap: 10px;
	}

	.card-icon {
		width: 40px;
		height: 40px;
		border-radius: 10px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: 1px solid color-mix(in srgb, var(--border) 75%, transparent);
		background: color-mix(in srgb, var(--bg) 85%, transparent);
		color: var(--hub-muted);
		flex-shrink: 0;
	}

	.card-icon.active {
		border-color: color-mix(in srgb, var(--accent) 34%, transparent);
		background: color-mix(in srgb, var(--accent) 12%, transparent);
		color: var(--accent);
	}

	.card-title {
		flex: 1;
		min-width: 0;
		display: grid;
		gap: 2px;
	}

	.card-title-row {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}

	.card-title h3 {
		margin: 0;
		font-size: var(--text-lg);
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.workset-card.active .card-title h3 {
		color: var(--accent);
	}

	.card-title span {
		font-size: var(--text-xs);
		color: color-mix(in srgb, var(--muted) 62%, white);
	}

	:global(.pin-indicator) {
		color: var(--hub-gold);
		flex-shrink: 0;
	}

	.badge {
		padding: 2px 6px;
		border-radius: 999px;
		font-size: var(--text-xs);
		font-weight: 600;
		letter-spacing: 0.02em;
		border: 1px solid transparent;
		flex-shrink: 0;
	}

	.badge.active {
		color: var(--accent);
		background: color-mix(in srgb, var(--accent) 14%, transparent);
		border-color: color-mix(in srgb, var(--accent) 24%, transparent);
	}

	.badge.archived {
		color: color-mix(in srgb, var(--muted) 68%, white);
		background: color-mix(in srgb, var(--border) 44%, transparent);
		border-color: color-mix(in srgb, var(--border) 64%, transparent);
	}

	.badge.template {
		color: color-mix(in srgb, var(--muted) 70%, white);
		background: color-mix(in srgb, var(--border) 38%, transparent);
		border-color: color-mix(in srgb, var(--border) 56%, transparent);
	}

	.card-description {
		margin: 0;
		font-size: var(--text-base);
		line-height: 1.45;
		color: var(--hub-muted);
		line-clamp: 2;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.branch-chip {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 5px 8px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--border) 55%, transparent);
		background: color-mix(in srgb, var(--panel) 50%, black);
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: color-mix(in srgb, var(--muted) 72%, white);
	}

	.branch-chip :global(svg) {
		color: var(--accent);
		flex-shrink: 0;
	}

	.branch-chip span {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.repo-chips {
		display: flex;
		flex-wrap: nowrap;
		gap: 6px;
		overflow: hidden;
	}

	.repo-chips span {
		display: inline-flex;
		align-items: center;
		padding: 4px 8px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--border) 68%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 48%, transparent);
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: color-mix(in srgb, var(--muted) 82%, white);
		flex-shrink: 0;
	}

	.repo-chips span.muted {
		color: color-mix(in srgb, var(--muted) 64%, white);
		background: color-mix(in srgb, var(--border) 28%, transparent);
	}

	.health-row {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.health-row .dot {
		width: 7px;
		height: 7px;
	}

	.diff {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		margin-left: 8px;
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
	}

	.plus {
		color: var(--success);
	}

	.minus {
		color: var(--danger);
	}

	.card-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 10px 16px;
		border-top: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
	}

	.footer-meta {
		display: inline-flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 10px;
		font-size: var(--text-xs);
		color: color-mix(in srgb, var(--muted) 68%, white);
	}

	.footer-meta span {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.prs {
		color: var(--hub-violet);
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.dirty {
		color: var(--hub-gold);
	}

	.footer-actions {
		display: inline-flex;
		align-items: center;
		gap: 8px;
	}

	kbd {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		padding: 1px 5px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--border) 70%, transparent);
		background: color-mix(in srgb, var(--bg) 84%, transparent);
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: color-mix(in srgb, var(--muted) 68%, white);
	}

	.list {
		display: grid;
		gap: 8px;
	}

	.list-row {
		display: grid;
		grid-template-columns:
			40px minmax(0, 1fr) auto auto auto auto
			auto auto;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
		border-radius: 12px;
		border: 1px solid var(--hub-border);
		background: var(--hub-surface);
		cursor: pointer;
		transition:
			border-color var(--transition-fast),
			background var(--transition-fast);
	}

	.list-row:hover {
		border-color: color-mix(in srgb, var(--accent) 26%, var(--hub-border));
		background: color-mix(in srgb, var(--panel-strong) 80%, transparent);
	}

	.list-row.active {
		border-color: color-mix(in srgb, var(--accent) 38%, var(--hub-border));
		background: color-mix(in srgb, var(--panel-strong) 88%, transparent);
	}

	.list-row.archived {
		opacity: 0.58;
	}

	.row-icon {
		width: 38px;
		height: 38px;
		border-radius: 10px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
		background: color-mix(in srgb, var(--bg) 84%, transparent);
		color: var(--hub-muted);
	}

	.row-icon.active {
		border-color: color-mix(in srgb, var(--accent) 34%, transparent);
		background: color-mix(in srgb, var(--accent) 12%, transparent);
		color: var(--accent);
	}

	.row-title {
		min-width: 0;
	}

	.row-title-line {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}

	.row-title strong {
		font-size: var(--text-base);
		font-weight: 600;
		color: var(--text);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.list-row.active .row-title strong {
		color: var(--accent);
	}

	.row-title p {
		margin: 2px 0 0;
		font-size: var(--text-xs);
		color: var(--hub-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.row-branch {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 8px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--border) 55%, transparent);
		background: color-mix(in srgb, var(--panel) 50%, black);
		font-size: var(--text-mono-xs);
		font-family: var(--font-mono);
		color: color-mix(in srgb, var(--muted) 72%, white);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 200px;
	}

	.row-branch :global(svg) {
		color: var(--accent);
		flex-shrink: 0;
	}

	.row-repo-count {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font-size: var(--text-xs);
		color: var(--hub-muted);
	}

	.row-health {
		display: inline-flex;
		align-items: center;
		gap: 3px;
	}

	.row-health .dot {
		width: 7px;
		height: 7px;
	}

	.row-stats {
		display: inline-flex;
		align-items: center;
		justify-content: flex-end;
		flex-wrap: nowrap;
		gap: 12px;
		font-size: var(--text-xs);
		color: color-mix(in srgb, var(--muted) 68%, white);
	}

	.row-stats .diff {
		margin-right: 4px;
	}

	.menu-anchor {
		position: relative;
		flex-shrink: 0;
	}

	.item-actions {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex-shrink: 0;
	}

	.popout-trigger,
	.menu-trigger {
		width: 28px;
		height: 28px;
		border-radius: 8px;
		border: 1px solid transparent;
		background: transparent;
		color: color-mix(in srgb, var(--muted) 70%, white);
		display: inline-flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		opacity: 0.62;
		transition:
			opacity var(--transition-fast),
			background var(--transition-fast),
			border-color var(--transition-fast),
			color var(--transition-fast);
	}

	.popout-trigger {
		opacity: 0.88;
	}

	.workset-card:hover .menu-trigger,
	.list-row:hover .menu-trigger,
	.menu-trigger:focus-visible,
	.menu-trigger[aria-expanded='true'] {
		opacity: 1;
	}

	.popout-trigger:hover,
	.popout-trigger:focus-visible,
	.menu-trigger:hover,
	.menu-trigger:focus-visible {
		background: color-mix(in srgb, var(--panel-strong) 78%, transparent);
		border-color: color-mix(in srgb, var(--border) 62%, transparent);
		color: var(--text);
	}

	.action-menu {
		position: absolute;
		right: 0;
		top: calc(100% + 4px);
		width: 152px;
		padding: 4px;
		border-radius: 10px;
		border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 96%, transparent);
		box-shadow: 0 12px 26px rgba(0, 0, 0, 0.36);
		z-index: 20;
	}

	.action-menu button {
		width: 100%;
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 8px 9px;
		border: none;
		border-radius: 7px;
		background: transparent;
		color: var(--hub-muted);
		font-size: var(--text-xs);
		text-align: left;
		cursor: pointer;
	}

	.action-menu button:hover {
		background: color-mix(in srgb, var(--panel) 72%, transparent);
		color: var(--text);
	}

	.action-menu button.item-pinned {
		color: var(--hub-gold);
	}

	.action-menu button.item-archive {
		color: var(--danger);
	}

	.action-menu button.item-archive:hover {
		color: var(--danger);
	}

	.action-menu button.item-archived {
		color: var(--success);
	}

	.empty-state {
		min-height: 280px;
		display: grid;
		place-content: center;
		justify-items: center;
		gap: 12px;
		color: color-mix(in srgb, var(--muted) 56%, white);
		font-size: var(--text-base);
	}

	@media (max-width: 1024px) {
		.grid {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}

		.row-health,
		.row-stats {
			display: none;
		}
	}

	@media (max-width: 920px) {
		.hub-header {
			padding-inline: 18px;
			padding-top: 18px;
		}

		.content {
			padding-inline: 18px;
		}

		.new-workset-btn {
			position: static;
			margin-bottom: 16px;
			width: fit-content;
		}

		.grid {
			grid-template-columns: 1fr;
		}

		.list-row {
			grid-template-columns: 36px minmax(0, 1fr) auto auto;
			gap: 10px;
		}

		.row-branch,
		.row-repo-count,
		.row-health,
		.row-stats {
			display: none;
		}
	}
</style>
