<script lang="ts">
	import {
		AlertCircle,
		ArrowLeft,
		ArrowUpRight,
		Archive,
		ArchiveRestore,
		Box,
		CheckCircle2,
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
		Trash2,
	} from '@lucide/svelte';
	import { clickOutside } from '../../actions/clickOutside';
	import type { WorksetSummary } from '../../view-models/worksetViewModel';

	export type WorksetGroupMode = 'all' | 'template' | 'repo' | 'active';
	export type WorksetLayoutMode = 'grid' | 'list';
	type WorksetGroup = {
		label: string;
		items: WorksetSummary[];
	};

	interface Props {
		worksets: WorksetSummary[];
		shortcutMap?: Map<string, number>;
		activeWorkspaceId: string | null;
		groupMode?: WorksetGroupMode;
		onSelectWorkspace: (workspaceId: string) => void;
		onCreateWorkspace: () => void;
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
		shortcutMap,
		activeWorkspaceId,
		onSelectWorkspace,
		onCreateWorkspace,
		onAddRepo,
		onRemoveWorkspace,
		onTogglePin,
		onToggleArchived,
		onOpenPopout,
		onClosePopout,
		isWorkspacePoppedOut,
		onGroupModeChange = () => {},
		groupMode: groupModeProp = 'active',
		layoutMode: layoutModeProp = 'grid',
		onLayoutModeChange = () => {},
	}: Props = $props();

	const GROUP_MODES: Array<{ id: WorksetGroupMode; label: string; icon: typeof LayoutGrid }> = [
		{ id: 'all', label: 'All', icon: LayoutGrid },
		{ id: 'template', label: 'Template', icon: Layers },
		{ id: 'repo', label: 'Repo', icon: FolderGit2 },
		{ id: 'active', label: 'Active', icon: Clock },
	];

	let searchQuery = $state('');
	let groupMode = $state<WorksetGroupMode>('active');
	let layoutMode = $state<WorksetLayoutMode>('grid');
	let showArchived = $state(false);
	let actionMenuFor = $state<string | null>(null);
	let groupModeInitialized = false;
	let layoutModeInitialized = false;

	$effect(() => {
		if (groupModeInitialized) return;
		groupMode = groupModeProp;
		groupModeInitialized = true;
	});

	$effect(() => {
		if (layoutModeInitialized) return;
		layoutMode = layoutModeProp;
		layoutModeInitialized = true;
	});

	const sortWorksetsByName = (items: WorksetSummary[]): WorksetSummary[] =>
		[...items].sort((left, right) => left.label.localeCompare(right.label));

	const sortWorksetsByActivity = (items: WorksetSummary[]): WorksetSummary[] =>
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
			{ label: 'Today', items: sortWorksetsByActivity(today) },
			{ label: 'This Week', items: sortWorksetsByActivity(thisWeek) },
			{ label: 'Older', items: sortWorksetsByActivity(older) },
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
			const pinnedItems = visible.filter((item) => item.pinned);
			if (pinnedItems.length === 0) {
				return [{ label: '', items: sortWorksetsByName(visible) }];
			}
			const unpinnedItems = visible.filter((item) => !item.pinned);
			return [
				{ label: 'Pinned', items: sortWorksetsByName(pinnedItems) },
				{ label: 'Unpinned', items: sortWorksetsByName(unpinnedItems) },
			];
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
				.map(([label, items]) => ({ label, items: sortWorksetsByName(items) }));
		}

		if (groupMode === 'repo') {
			const repoMap = new Map<string, WorksetSummary[]>();
			const noReposLabel = 'No Repos';
			for (const item of visible) {
				if (item.repos.length === 0) {
					const bucket = repoMap.get(noReposLabel) ?? [];
					bucket.push(item);
					repoMap.set(noReposLabel, bucket);
					continue;
				}
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
					if (left[0] === noReposLabel && right[0] !== noReposLabel) return 1;
					if (right[0] === noReposLabel && left[0] !== noReposLabel) return -1;
					const byCount = right[1].length - left[1].length;
					if (byCount !== 0) return byCount;
					return left[0].localeCompare(right[0]);
				})
				.map(([label, items]) => ({ label, items: sortWorksetsByName(items) }));
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
	const totalMergedPrs = $derived(visibleCatalog.reduce((acc, item) => acc + item.mergedPrs, 0));
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

	const updateGroupMode = (next: WorksetGroupMode): void => {
		groupMode = next;
		onGroupModeChange(next);
	};

	const updateLayoutMode = (next: WorksetLayoutMode): void => {
		layoutMode = next;
		onLayoutModeChange(next);
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

	const handleAddRepo = (workspaceId: string, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		onAddRepo(workspaceId);
	};

	const handleRemoveWorkspace = (workspaceId: string, event: MouseEvent): void => {
		event.stopPropagation();
		closeActionMenu();
		onRemoveWorkspace(workspaceId);
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
				<span>Open PRs</span>
				<strong>{totalPrs}</strong>
			</div>
			{#if totalMergedPrs > 0}
				<div class="stat-pill">
					<div class="ws-dot ws-dot-md ws-dot-violet"></div>
					<span>Merged PRs</span>
					<strong>{totalMergedPrs}</strong>
				</div>
			{/if}
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
																	onclick={(event) => handleAddRepo(item.id, event)}
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
																	onclick={(event) => handleRemoveWorkspace(item.id, event)}
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
												<span class="prs"><GitPullRequest size={10} /> {item.openPrs}</span>
												{#if item.mergedPrs > 0}
													<span class="merged"
														><CheckCircle2 size={10} /> {item.mergedPrs} merged</span
													>
												{/if}
												<span class="dirty"><AlertCircle size={10} /> {item.dirtyCount} dirty</span>
												<span><Clock size={10} /> {item.lastActive}</span>
											</div>
											<div class="footer-actions">
												{#if getShortcutNumber(item.id)}
													<kbd class="ui-kbd"><Command size={7} />{getShortcutNumber(item.id)}</kbd>
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
												<span class={`ws-dot ws-dot-sm ws-dot-${status}`}></span>
											{/each}
										</div>

										<div class="row-stats">
											<span class="diff ws-diffstat"
												><span class="ws-diffstat-add">+{item.linesAdded}</span>
												<span class="ws-diffstat-del">-{item.linesRemoved}</span></span
											>
											<span class="prs"><GitPullRequest size={10} /> {item.openPrs}</span>
											{#if item.mergedPrs > 0}
												<span class="merged"
													><CheckCircle2 size={10} /> {item.mergedPrs} merged</span
												>
											{/if}
											<span class="dirty">{item.dirtyCount} dirty</span>
											<span>{item.lastActive}</span>
										</div>

										{#if getShortcutNumber(item.id)}
											<kbd class="ui-kbd"><Command size={7} />{getShortcutNumber(item.id)}</kbd>
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
															onclick={(event) => handleAddRepo(item.id, event)}
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
															onclick={(event) => handleRemoveWorkspace(item.id, event)}
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
