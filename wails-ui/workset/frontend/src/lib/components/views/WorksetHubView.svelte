<script lang="ts">
	import {
		AlertCircle,
		ArrowDownLeft,
		ArrowLeft,
		ArrowRight,
		ArrowUpRight,
		Archive,
		ArchiveRestore,
		Box,
		CheckCircle2,
		CircleDot,
		Clock,
		Command,
		Eye,
		EyeOff,
		FolderGit2,
		GitBranch,
		GitPullRequest,
		LayoutGrid,
		List,
		MessageSquare,
		MoreHorizontal,
		Pin,
		PinOff,
		Plus,
		Search,
		Terminal,
		Trash2,
	} from '@lucide/svelte';
	import { clickOutside } from '../../actions/clickOutside';
	import type { Workspace } from '../../types';
	import type { WorksetSummary } from '../../view-models/worksetViewModel';
	import {
		buildActiveWorksetRows,
		buildWorksetAggregates,
		buildWorksetGroups,
		getHealthStatusLabel,
		getThreadStatus,
		resolveActiveWorksetCard,
		resolveActiveWorkspaceEntry,
		type ActiveRepoRow,
		type WorksetAggregate,
		type WorksetGroup,
		type WorksetGroupMode,
		type WorksetLayoutMode,
	} from './WorksetHubView.helpers';

	interface Props {
		worksets: WorksetSummary[];
		workspaceCatalog?: Workspace[];
		shortcutMap?: Map<string, number>;
		activeWorkspaceId: string | null;
		groupMode?: WorksetGroupMode;
		onSelectWorkspace: (workspaceId: string) => void;
		onCreateWorkspace: () => void;
		onOpenCockpit?: () => void;
		onAddRepo: (workspaceId: string) => void;
		onRemoveWorkspace: (workspaceId: string) => void;
		onTogglePin: (workspaceId: string, nextPinned: boolean) => void;
		onToggleArchived: (workspaceId: string, archived: boolean) => void;
		onOpenPopout: (workspaceId: string) => void;
		onClosePopout: (workspaceId: string) => void;
		isWorkspacePoppedOut: (workspaceId: string) => boolean;
		onGroupModeChange?: (groupMode: WorksetGroupMode) => void;
		layoutMode?: WorksetLayoutMode;
		onLayoutModeChange?: (layoutMode: WorksetLayoutMode) => void;
	}

	const {
		worksets,
		workspaceCatalog = [],
		shortcutMap,
		activeWorkspaceId,
		onSelectWorkspace,
		onCreateWorkspace,
		onOpenCockpit = () => {},
		onAddRepo,
		onRemoveWorkspace,
		onTogglePin,
		onToggleArchived,
		onOpenPopout,
		onClosePopout,
		isWorkspacePoppedOut,
		onGroupModeChange = () => {},
		groupMode: groupModeProp,
		layoutMode: layoutModeProp,
		onLayoutModeChange = () => {},
	}: Props = $props();

	const GROUP_MODES: Array<{ id: WorksetGroupMode; label: string; icon: typeof LayoutGrid }> = [
		{ id: 'all', label: 'All', icon: LayoutGrid },
		{ id: 'repo', label: 'Repo', icon: FolderGit2 },
		{ id: 'active', label: 'Active', icon: Clock },
	];
	let searchQuery = $state('');
	let groupMode = $state<WorksetGroupMode>('all');
	let layoutMode = $state<WorksetLayoutMode>('grid');
	let showArchived = $state(false);
	let actionMenuFor = $state<string | null>(null);
	let menuClosedAt = $state(0);

	$effect(() => {
		if (groupModeProp === undefined || groupMode === groupModeProp) return;
		groupMode = groupModeProp;
	});

	$effect(() => {
		if (layoutModeProp === undefined || layoutMode === layoutModeProp) return;
		layoutMode = layoutModeProp;
	});

	const allWorksets = $derived.by(() => buildWorksetAggregates(worksets));

	const filtered = $derived.by(() => {
		const query = searchQuery.trim().toLowerCase();
		if (!query) return allWorksets;
		return allWorksets.filter((item) => {
			const threadNames = item.threads.map((thread) => thread.label).join(' ');
			const branches = item.threads.map((thread) => thread.branch).join(' ');
			const descriptions = item.threads.map((thread) => thread.description).join(' ');
			const haystack =
				`${item.label} ${item.description} ${threadNames} ${branches} ${descriptions} ${item.repos.join(
					' ',
				)}`.toLowerCase();
			return haystack.includes(query);
		});
	});

	const visible = $derived.by(() =>
		showArchived ? filtered : filtered.filter((item) => !item.archived),
	);

	const groups = $derived.by<WorksetGroup[]>(() => buildWorksetGroups(visible, groupMode));

	const visibleCatalog = $derived.by(() =>
		showArchived ? allWorksets : allWorksets.filter((item) => !item.archived),
	);
	const totalWorksets = $derived(visibleCatalog.length);
	const totalRepos = $derived.by(
		() =>
			new Set(visibleCatalog.flatMap((item) => item.repos.map((repo) => repo.toLowerCase()))).size,
	);
	const totalThreads = $derived(visibleCatalog.reduce((acc, item) => acc + item.threads.length, 0));
	const totalPrs = $derived(visibleCatalog.reduce((acc, item) => acc + item.openPrs, 0));
	const totalDirty = $derived(visibleCatalog.reduce((acc, item) => acc + item.dirtyCount, 0));
	const totalPinned = $derived(visibleCatalog.filter((item) => item.pinned).length);
	const archivedCount = $derived(allWorksets.filter((item) => item.archived).length);
	const activeWorkspaceEntry = $derived.by(() =>
		resolveActiveWorkspaceEntry(workspaceCatalog, activeWorkspaceId),
	);
	const activeWorksetCard = $derived.by(() =>
		resolveActiveWorksetCard(allWorksets, visibleCatalog, activeWorkspaceEntry, activeWorkspaceId),
	);
	const activeWorksetRows = $derived.by<ActiveRepoRow[]>(() =>
		buildActiveWorksetRows(activeWorksetCard, workspaceCatalog),
	);

	const getPrimaryThread = (item: WorksetAggregate): WorksetSummary | null => {
		if (activeWorkspaceId) {
			const activeThread = item.threads.find((thread) => thread.id === activeWorkspaceId);
			if (activeThread) return activeThread;
		}
		return item.threads[0] ?? null;
	};

	const getShortcutNumber = (item: WorksetAggregate): number | undefined => {
		let value: number | undefined;
		for (const thread of item.threads) {
			const next = shortcutMap?.get(thread.id);
			if (next === undefined) continue;
			value = value === undefined ? next : Math.min(value, next);
		}
		return value;
	};

	const isItemActive = (item: WorksetAggregate): boolean =>
		!!activeWorkspaceId && item.threads.some((thread) => thread.id === activeWorkspaceId);

	const closeActionMenu = (): void => {
		actionMenuFor = null;
		menuClosedAt = Date.now();
	};

	const updateGroupMode = (next: WorksetGroupMode): void => {
		if (groupMode === next) return;
		groupMode = next;
		onGroupModeChange(next);
	};

	const updateLayoutMode = (next: WorksetLayoutMode): void => {
		if (layoutMode === next) return;
		layoutMode = next;
		onLayoutModeChange(next);
	};

	const toggleActionMenu = (itemId: string, event: MouseEvent): void => {
		event.stopPropagation();
		// Avoid immediate reopen after clickOutside closes within the same event cycle.
		if (Date.now() - menuClosedAt < 50) return;
		actionMenuFor = actionMenuFor === itemId ? null : itemId;
	};

	const openWorkset = (item: WorksetAggregate): void => {
		closeActionMenu();
		const primaryThread = getPrimaryThread(item);
		if (!primaryThread) return;
		onSelectWorkspace(primaryThread.id);
	};

	const handleWorksetKeyboard = (event: KeyboardEvent, item: WorksetAggregate): void => {
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		openWorkset(item);
	};

	const handleTogglePin = (item: WorksetAggregate, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		const primaryThread = getPrimaryThread(item);
		if (!primaryThread) return;
		onTogglePin(primaryThread.id, !item.pinned);
	};

	const handleToggleArchive = (item: WorksetAggregate, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		const primaryThread = getPrimaryThread(item);
		if (!primaryThread) return;
		onToggleArchived(primaryThread.id, item.archived);
	};

	const handleAddRepo = (item: WorksetAggregate, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		const primaryThread = getPrimaryThread(item);
		if (!primaryThread) return;
		onAddRepo(primaryThread.id);
	};

	const handleRemoveWorkspace = (item: WorksetAggregate, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		const primaryThread = getPrimaryThread(item);
		if (!primaryThread) return;
		onRemoveWorkspace(primaryThread.id);
	};

	const handleOpenPopout = (item: WorksetAggregate, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		const primaryThread = getPrimaryThread(item);
		if (!primaryThread) return;
		onOpenPopout(primaryThread.id);
	};

	const handleClosePopout = (item: WorksetAggregate, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		const primaryThread = getPrimaryThread(item);
		if (!primaryThread) return;
		onClosePopout(primaryThread.id);
	};

	const itemHasPopout = (item: WorksetAggregate): boolean => {
		const primaryThread = getPrimaryThread(item);
		if (!primaryThread) return false;
		return isWorkspacePoppedOut(primaryThread.id);
	};
</script>

<div class="hub-shell">
	<header class="hub-header">
		<div class="title-wrap">
			<h1>Worksets</h1>
			<p>Each workset groups repos and feature threads into a single unit of intent.</p>
		</div>
		<button type="button" class="new-workset-btn" onclick={onCreateWorkspace}
			><Plus size={16} /> New Workset</button
		>

		<div class="stats-row">
			<div class="stat-pill">
				<div class="ws-dot ws-dot-md ws-dot-ahead"></div>
				<span>Worksets</span>
				<strong>{totalWorksets}</strong>
			</div>
			<div class="stat-pill">
				<div class="ws-dot ws-dot-md ws-dot-violet"></div>
				<span>Repos</span>
				<strong>{totalRepos}</strong>
			</div>
			<div class="stat-pill">
				<div class="ws-dot ws-dot-md ws-dot-clean"></div>
				<span>Threads</span>
				<strong>{totalThreads}</strong>
			</div>
			<div class="stat-pill">
				<div class="ws-dot ws-dot-md ws-dot-clean"></div>
				<span>Open PRs</span>
				<strong>{totalPrs}</strong>
			</div>
			{#if totalDirty > 0}
				<div class="stat-pill">
					<div class="ws-dot ws-dot-md ws-dot-gold"></div>
					<span>Dirty</span>
					<strong>{totalDirty}</strong>
				</div>
			{/if}
			<div class="stat-pill">
				<div class="ws-dot ws-dot-md ws-dot-gold"></div>
				<span>Pinned</span>
				<strong>{totalPinned}</strong>
			</div>
		</div>

		<div class="toolbar">
			<label class="search-wrap">
				<Search size={15} />
				<input
					type="text"
					placeholder="Search worksets, repos, threads..."
					bind:value={searchQuery}
				/>
			</label>

			<div class="segmented-control" role="radiogroup" aria-label="Group mode">
				{#each GROUP_MODES as mode (mode.id)}
					<button
						type="button"
						class="segment"
						class:active={groupMode === mode.id}
						onclick={() => updateGroupMode(mode.id)}
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
					onclick={() => updateLayoutMode('grid')}
					aria-label="Grid layout"
				>
					<LayoutGrid size={14} />
				</button>
				<button
					type="button"
					class="segment icon"
					class:active={layoutMode === 'list'}
					onclick={() => updateLayoutMode('list')}
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
		{#if activeWorksetCard}
			<div class="active-workset-card">
				<div class="active-workset-header">
					<div class="active-workset-title">
						<div class="active-workset-icon">
							<Box size={14} />
						</div>
						<div>
							<h2>{activeWorksetCard.label}</h2>
							<p>{activeWorksetCard.description}</p>
						</div>
					</div>
					<div class="active-workset-actions">
						<div class="daemon-pill">
							<span class="daemon-dot"></span>
							Daemon Active
						</div>
						<button type="button" class="cockpit-btn" onclick={onOpenCockpit}>
							<Terminal size={12} />
							Open Cockpit
						</button>
					</div>
				</div>

				<div class="active-workset-stats">
					<span><FolderGit2 size={11} /> {activeWorksetCard.repos.length} repos</span>
					<span><MessageSquare size={11} /> {activeWorksetCard.threads.length} threads</span>
					{#if activeWorksetCard.openPrs > 0}
						<span class="pr"><GitPullRequest size={11} /> {activeWorksetCard.openPrs} open PRs</span
						>
					{/if}
					{#if activeWorksetCard.dirtyCount > 0}
						<span class="dirty"><CircleDot size={11} /> {activeWorksetCard.dirtyCount} dirty</span>
					{/if}
					{#if activeWorksetCard.linesAdded > 0 || activeWorksetCard.linesRemoved > 0}
						<span class="ws-diffstat">
							<span class="ws-diffstat-add">+{activeWorksetCard.linesAdded}</span>
							<span class="ws-diffstat-del">-{activeWorksetCard.linesRemoved}</span>
						</span>
					{/if}
				</div>

				<div class="active-repo-table">
					{#each activeWorksetRows as repo (`active-${repo.name}`)}
						<div class="active-repo-row">
							<div class={`ws-dot ws-dot-sm ws-dot-${repo.status}`}></div>
							<span class="repo-name">{repo.name}</span>
							<span class="repo-branch">
								<GitBranch size={10} />
								{repo.branch}
							</span>
							<div class="repo-movement">
								{#if repo.ahead > 0}
									<span><ArrowUpRight size={10} /> {repo.ahead}</span>
								{/if}
								{#if repo.behind > 0}
									<span><ArrowDownLeft size={10} /> {repo.behind}</span>
								{/if}
							</div>
							{#if repo.prNumber}
								<span class="repo-pr"><GitPullRequest size={10} /> #{repo.prNumber}</span>
							{/if}
							{#if repo.dirtyFiles > 0}
								<span class="repo-dirty">{repo.dirtyFiles} dirty</span>
							{/if}
							<span class="repo-status">{getHealthStatusLabel(repo.status)}</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		<div class="all-worksets-heading">
			<h2>All Worksets</h2>
			<div class="line"></div>
		</div>

		{#if groups.length === 0 || groups.every((group) => group.items.length === 0)}
			<div class="empty-state ws-empty-state">
				<Search size={32} />
				<p class="ws-empty-state-copy">No worksets match "{searchQuery}"</p>
			</div>
		{:else}
			<div class="groups">
				{#each groups as group (group.label || 'all')}
					<div class="group">
						{#if group.label}
							<div class="group-header">
								{#if groupMode === 'repo'}
									<FolderGit2 size={14} />
								{:else if groupMode === 'active'}
									<Clock size={14} />
								{:else}
									<Box size={14} />
								{/if}
								<h2>{group.label}</h2>
								<span>{group.items.length} workset{group.items.length !== 1 ? 's' : ''}</span>
							</div>
						{/if}

						{#if layoutMode === 'grid'}
							<div class="grid">
								{#each group.items as item (item.id)}
									<div
										class="workset-card"
										class:active={isItemActive(item)}
										class:archived={item.archived}
										role="button"
										tabindex="0"
										onclick={() => openWorkset(item)}
										onkeydown={(event) => handleWorksetKeyboard(event, item)}
									>
										{#if isItemActive(item)}
											<div class="active-bar"></div>
										{/if}

										<div class="card-body">
											<div class="card-head">
												<div class="card-icon" class:active={isItemActive(item)}>
													<Box size={18} />
												</div>

												<div class="card-title">
													<div class="card-title-row">
														<h3>{item.label}</h3>
														{#if item.pinned}
															<Pin size={10} class="pin-indicator" />
														{/if}
														{#if isItemActive(item)}
															<span class="badge active">Active</span>
														{/if}
														{#if item.archived}
															<span class="badge archived">Archived</span>
														{/if}
													</div>
													<span
														>{item.threads.length} thread{item.threads.length !== 1
															? 's'
															: ''}</span
													>
												</div>

												<div class="item-actions">
													<button
														type="button"
														class="popout-trigger"
														aria-label={itemHasPopout(item)
															? 'Return workspace to main window'
															: 'Open workspace popout'}
														title={itemHasPopout(item)
															? 'Return to main window'
															: 'Open workspace popout'}
														onclick={(event) =>
															itemHasPopout(item)
																? handleClosePopout(item, event)
																: handleOpenPopout(item, event)}
													>
														{#if itemHasPopout(item)}
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
																	onclick={(event) => handleAddRepo(item, event)}
																>
																	<Plus size={13} />
																	Add repo
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
																<button
																	type="button"
																	class="item-delete"
																	onclick={(event) => handleRemoveWorkspace(item, event)}
																>
																	<Trash2 size={13} />
																	Delete workset
																</button>
															</div>
														{/if}
													</div>
												</div>
											</div>

											{#if item.description}
												<p class="card-description">{item.description}</p>
											{/if}

											<div class="thread-list">
												{#each item.threads.slice(0, 3) as thread (thread.id)}
													<div class="thread-line">
														<span class="status-dot status-{getThreadStatus(thread)}"></span>
														<span class="thread-name">{thread.label}</span>
														<span class="thread-branch">
															<GitBranch size={10} />
															{thread.branch}
														</span>
													</div>
												{/each}
												{#if item.threads.length > 3}
													<span class="thread-more">+{item.threads.length - 3} more</span>
												{/if}
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
													<span class={`ws-dot ws-dot-sm ws-dot-${status}`} title={status}></span>
												{/each}
												<span class="diff ws-diffstat"
													><span class="ws-diffstat-add">+{item.linesAdded}</span>
													<span class="ws-diffstat-del">-{item.linesRemoved}</span></span
												>
											</div>
										</div>

										<div class="card-footer">
											<div class="footer-meta">
												<span class="prs"
													><MessageSquare size={10} />
													{item.threads.length} thread{item.threads.length !== 1 ? 's' : ''}</span
												>
												{#if item.openPrs > 0}
													<span class="prs"><GitPullRequest size={10} /> {item.openPrs}</span>
												{/if}
												{#if item.mergedPrs > 0}
													<span class="merged"
														><CheckCircle2 size={10} /> {item.mergedPrs} merged</span
													>
												{/if}
												<span class="dirty"><AlertCircle size={10} /> {item.dirtyCount} dirty</span>
												<span><Clock size={10} /> {item.lastActive}</span>
											</div>
											<div class="footer-actions">
												{#if getShortcutNumber(item)}
													<kbd class="ui-kbd"><Command size={7} />{getShortcutNumber(item)}</kbd>
												{/if}
												<div class="open-indicator">
													Open
													<ArrowRight size={10} />
												</div>
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
										class:active={isItemActive(item)}
										class:archived={item.archived}
										role="button"
										tabindex="0"
										onclick={() => openWorkset(item)}
										onkeydown={(event) => handleWorksetKeyboard(event, item)}
									>
										<div class="row-icon" class:active={isItemActive(item)}>
											<Box size={16} />
										</div>

										<div class="row-title">
											<div class="row-title-line">
												<strong>{item.label}</strong>
												{#if item.pinned}
													<Pin size={10} class="pin-indicator" />
												{/if}
												{#if isItemActive(item)}
													<span class="badge active">Active</span>
												{/if}
												{#if item.archived}
													<span class="badge archived">Archived</span>
												{/if}
											</div>
											<p>
												{item.threads
													.slice(0, 2)
													.map((thread) => thread.label)
													.join(' · ')}
											</p>
										</div>

										<div class="row-branch">
											<GitBranch size={10} />
											{item.branch}
										</div>

										<div class="row-repo-count">
											<FolderGit2 size={11} />
											{item.repos.length}
										</div>

										<div class="row-health">
											{#each item.health as status, index (`${item.id}-list-health-${index}`)}
												<span class={`ws-dot ws-dot-sm ws-dot-${status}`}></span>
											{/each}
										</div>

										<div class="row-stats">
											<span class="diff ws-diffstat"
												><span class="ws-diffstat-add">+{item.linesAdded}</span>
												<span class="ws-diffstat-del">-{item.linesRemoved}</span></span
											>
											<span class="prs"
												><MessageSquare size={10} />
												{item.threads.length} thread{item.threads.length !== 1 ? 's' : ''}</span
											>
											{#if item.openPrs > 0}
												<span class="prs"><GitPullRequest size={10} /> {item.openPrs}</span>
											{/if}
											{#if item.mergedPrs > 0}
												<span class="merged"
													><CheckCircle2 size={10} /> {item.mergedPrs} merged</span
												>
											{/if}
											<span class="dirty">{item.dirtyCount} dirty</span>
											<span>{item.lastActive}</span>
										</div>

										{#if getShortcutNumber(item)}
											<kbd class="ui-kbd"><Command size={7} />{getShortcutNumber(item)}</kbd>
										{/if}

										<div class="item-actions">
											<button
												type="button"
												class="popout-trigger"
												aria-label={itemHasPopout(item)
													? 'Return workspace to main window'
													: 'Open workspace popout'}
												title={itemHasPopout(item)
													? 'Return to main window'
													: 'Open workspace popout'}
												onclick={(event) =>
													itemHasPopout(item)
														? handleClosePopout(item, event)
														: handleOpenPopout(item, event)}
											>
												{#if itemHasPopout(item)}
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
														<button type="button" onclick={(event) => handleAddRepo(item, event)}>
															<Plus size={13} />
															Add repo
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
														<button
															type="button"
															class="item-delete"
															onclick={(event) => handleRemoveWorkspace(item, event)}
														>
															<Trash2 size={13} />
															Delete workset
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

<style src="./WorksetHubView.css"></style>
