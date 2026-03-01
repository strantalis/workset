<script lang="ts">
	import {
		Box,
		Check,
		ChevronDown,
		ChevronRight,
		Clock3,
		FolderGit2,
		GitPullRequest,
		MessageSquare,
		PanelLeftClose,
		Pin,
		Plus,
		Search,
		Sparkles,
		Settings,
		Terminal,
	} from '@lucide/svelte';
	import type { Workspace } from '../../types';

	type HealthState = 'clean' | 'modified' | 'ahead' | 'error';
	type ThreadStatus = 'active' | 'in-review' | 'merged' | 'stale';

	type WorksetNode = {
		id: string;
		label: string;
		description: string;
		threads: Workspace[];
		repos: string[];
		health: HealthState[];
		lastActiveTs: number;
		pinned: boolean;
		shortcutNumber?: number;
		activeThreads: number;
		openPrs: number;
		dirtyRepos: number;
		linesAdded: number;
		linesRemoved: number;
	};

	interface Props {
		workspaces: Workspace[];
		activeWorkspaceId: string | null;
		shortcutMap: Map<string, number>;
		lockWorksetSelection?: boolean;
		canManageRepos?: boolean;
		activeView?: 'terminal-cockpit' | 'skill-registry';
		activeSurface?: 'terminal' | 'pull-requests';
		onSelectWorkspace: (workspaceId: string) => void;
		onCreateWorkspace?: () => void;
		onCreateThread?: (worksetId: string) => void;
		onAddRepo?: (worksetId: string) => void;
		onOpenCockpit?: () => void;
		onOpenPullRequests?: () => void;
		onOpenSkills?: () => void;
		onOpenSettings?: () => void;
		onCollapse?: () => void;
	}

	const {
		workspaces,
		activeWorkspaceId,
		shortcutMap,
		lockWorksetSelection = false,
		canManageRepos = true,
		activeView = 'terminal-cockpit',
		activeSurface = 'terminal',
		onSelectWorkspace,
		onCreateWorkspace = () => {},
		onCreateThread = () => {},
		onAddRepo = () => {},
		onOpenCockpit = () => {},
		onOpenPullRequests = () => {},
		onOpenSkills = () => {},
		onOpenSettings = () => {},
		onCollapse = () => {},
	}: Props = $props();

	const parseLastUsed = (value: string): number => {
		const timestamp = Date.parse(value);
		return Number.isNaN(timestamp) ? 0 : timestamp;
	};

	const getRepoHealth = (repo: Workspace['repos'][number]): HealthState => {
		if (repo.missing) return 'error';
		if (repo.dirty) return 'modified';
		if ((repo.ahead ?? 0) > 0) return 'ahead';
		return 'clean';
	};

	const deriveWorksetIdentity = (workspace: Workspace): { id: string; label: string } => {
		const key = workspace.worksetKey?.trim();
		const label = workspace.worksetLabel?.trim();
		const legacy = workspace.workset?.trim() || workspace.template?.trim();
		const normalizedLegacy = legacy?.toLowerCase().replace(/\s+/g, '-') ?? '';
		return {
			id:
				key && key.length > 0
					? key
					: normalizedLegacy.length > 0
						? `workset:${normalizedLegacy}`
						: `workspace:${workspace.id.toLowerCase()}`,
			label:
				label && label.length > 0 ? label : legacy && legacy.length > 0 ? legacy : workspace.name,
		};
	};

	const isOpenTrackedPullRequest = (repo: Workspace['repos'][number]): boolean => {
		const tracked = repo.trackedPullRequest;
		if (!tracked) return false;
		const state = tracked.state.toLowerCase();
		const merged = tracked.merged === true || state === 'merged';
		return state === 'open' && !merged;
	};

	const isMergedTrackedPullRequest = (repo: Workspace['repos'][number]): boolean => {
		const tracked = repo.trackedPullRequest;
		if (!tracked) return false;
		return tracked.merged === true || tracked.state.toLowerCase() === 'merged';
	};

	const buildWorksetDescription = (threads: Workspace[]): string => {
		const explicit = threads.find((thread) => (thread.description ?? '').trim().length > 0);
		if (explicit?.description) return explicit.description.trim();
		const threadNames = threads
			.map((thread) => thread.name.trim())
			.filter((name) => name.length > 0)
			.slice(0, 2);
		if (threadNames.length === 0) return 'No threads yet';
		if (threadNames.length === 1) return threadNames[0];
		return `${threadNames[0]} + ${threadNames[1]}`;
	};

	const formatRelativeAge = (timestamp: number): string => {
		if (!Number.isFinite(timestamp) || timestamp <= 0) return 'just now';
		const delta = Math.max(0, Date.now() - timestamp);
		const minute = 60 * 1000;
		const hour = 60 * minute;
		const day = 24 * hour;
		if (delta < hour) return `${Math.max(1, Math.floor(delta / minute))}m ago`;
		if (delta < day) return `${Math.floor(delta / hour)}h ago`;
		return `${Math.floor(delta / day)}d ago`;
	};

	const getThreadStatus = (workspace: Workspace): ThreadStatus => {
		if (workspace.repos.some((repo) => isOpenTrackedPullRequest(repo))) return 'in-review';
		if (workspace.repos.some((repo) => repo.dirty)) return 'active';
		if (workspace.repos.some((repo) => isMergedTrackedPullRequest(repo))) return 'merged';
		const age = Date.now() - parseLastUsed(workspace.lastUsed);
		return age > 14 * 24 * 60 * 60 * 1000 ? 'stale' : 'active';
	};

	const groupedWorksets = $derived.by<WorksetNode[]>(() => {
		const byWorkset = new Map<
			string,
			{
				label: string;
				threads: Workspace[];
				repos: Set<string>;
				health: Set<HealthState>;
				lastActiveTs: number;
				pinned: boolean;
				openPrs: number;
				dirtyRepos: number;
				linesAdded: number;
				linesRemoved: number;
			}
		>();

		for (const workspace of workspaces.filter((entry) => !entry.archived)) {
			const identity = deriveWorksetIdentity(workspace);
			const lastUsed = parseLastUsed(workspace.lastUsed);
			const existing = byWorkset.get(identity.id);
			const target = existing ?? {
				label: identity.label,
				threads: [],
				repos: new Set<string>(),
				health: new Set<HealthState>(),
				lastActiveTs: 0,
				pinned: false,
				openPrs: 0,
				dirtyRepos: 0,
				linesAdded: 0,
				linesRemoved: 0,
			};

			target.threads.push(workspace);
			target.lastActiveTs = Math.max(target.lastActiveTs, lastUsed);
			target.pinned = target.pinned || workspace.pinned;
			for (const repo of workspace.repos) {
				target.repos.add(repo.name);
				target.health.add(getRepoHealth(repo));
				target.linesAdded += repo.diff?.added ?? 0;
				target.linesRemoved += repo.diff?.removed ?? 0;
				if (repo.dirty) target.dirtyRepos += 1;
				if (isOpenTrackedPullRequest(repo)) target.openPrs += 1;
			}

			byWorkset.set(identity.id, target);
		}

		return [...byWorkset.entries()]
			.map(([id, value]) => {
				const threads = [...value.threads];

				let shortcutNumber: number | undefined;
				for (const thread of threads) {
					const shortcut = shortcutMap.get(thread.id);
					if (shortcut === undefined) continue;
					shortcutNumber =
						shortcutNumber === undefined ? shortcut : Math.min(shortcutNumber, shortcut);
				}

				return {
					id,
					label: value.label,
					description: buildWorksetDescription(threads),
					threads,
					repos: [...value.repos].sort((left, right) => left.localeCompare(right)),
					health: [...value.health],
					lastActiveTs: value.lastActiveTs,
					pinned: value.pinned,
					shortcutNumber,
					activeThreads: threads.filter((thread) => {
						const status = getThreadStatus(thread);
						return status === 'active' || status === 'in-review';
					}).length,
					openPrs: value.openPrs,
					dirtyRepos: value.dirtyRepos,
					linesAdded: value.linesAdded,
					linesRemoved: value.linesRemoved,
				};
			})
			.sort((left, right) => {
				if (left.pinned !== right.pinned) return left.pinned ? -1 : 1;
				if (left.lastActiveTs !== right.lastActiveTs) return right.lastActiveTs - left.lastActiveTs;
				return left.label.localeCompare(right.label);
			});
	});

	const activeWorksetId = $derived.by(() => {
		if (!activeWorkspaceId) return null;
		const active = workspaces.find((workspace) => workspace.id === activeWorkspaceId);
		if (!active) return null;
		return deriveWorksetIdentity(active).id;
	});

	const selectedWorkset = $derived.by<WorksetNode | null>(() => {
		if (activeWorksetId) {
			const current = groupedWorksets.find((item) => item.id === activeWorksetId);
			if (current) return current;
		}
		return groupedWorksets[0] ?? null;
	});

	let switcherOpen = $state(false);
	let switcherQuery = $state('');
	let switcherFocusIndex = $state(0);
	let switcherSearchInputEl: HTMLInputElement | null = $state(null);
	let switcherListEl: HTMLDivElement | null = $state(null);
	let reposOpen = $state(false);

	const switcherAvailableWorksets = $derived.by(() => {
		if (!selectedWorkset) return groupedWorksets;
		return groupedWorksets.filter((workset) => workset.id !== selectedWorkset.id);
	});

	const switcherFilteredWorksets = $derived.by(() => {
		const query = switcherQuery.trim().toLowerCase();
		if (query.length === 0) return switcherAvailableWorksets;
		return switcherAvailableWorksets.filter((workset) => {
			if (workset.label.toLowerCase().includes(query)) return true;
			if (workset.description.toLowerCase().includes(query)) return true;
			return workset.repos.some((repo) => repo.toLowerCase().includes(query));
		});
	});

	const switcherPinnedWorksets = $derived.by(() =>
		switcherFilteredWorksets.filter((workset) => workset.pinned),
	);

	const switcherRecentWorksets = $derived.by(() =>
		switcherFilteredWorksets.filter((workset) => !workset.pinned),
	);

	const switcherFlatWorksets = $derived.by(() => [
		...switcherPinnedWorksets,
		...switcherRecentWorksets,
	]);
	const worksetSwitcherEnabled = $derived.by(
		() => !lockWorksetSelection && groupedWorksets.length > 1,
	);

	const selectThread = (workspaceId: string): void => {
		if (!workspaceId) return;
		onSelectWorkspace(workspaceId);
		switcherOpen = false;
	};

	const selectWorkset = (worksetId: string): void => {
		const workset = groupedWorksets.find((item) => item.id === worksetId);
		const thread = workset?.threads[0];
		if (!thread) return;
		selectThread(thread.id);
	};

	const handleSwitcherKeydown = (event: KeyboardEvent): void => {
		if (!switcherOpen) return;
		if (switcherFlatWorksets.length === 0) {
			if (event.key === 'Escape') {
				event.preventDefault();
				switcherOpen = false;
			}
			return;
		}
		if (event.key === 'ArrowDown') {
			event.preventDefault();
			switcherFocusIndex = Math.min(switcherFocusIndex + 1, switcherFlatWorksets.length - 1);
			return;
		}
		if (event.key === 'ArrowUp') {
			event.preventDefault();
			switcherFocusIndex = Math.max(switcherFocusIndex - 1, 0);
			return;
		}
		if (event.key === 'Enter') {
			event.preventDefault();
			const focused = switcherFlatWorksets[switcherFocusIndex];
			if (focused) selectWorkset(focused.id);
			return;
		}
		if (event.key === 'Escape') {
			event.preventDefault();
			switcherOpen = false;
		}
	};

	$effect(() => {
		if (groupedWorksets.length === 0) {
			switcherOpen = false;
			switcherQuery = '';
			switcherFocusIndex = 0;
			return;
		}
		if (activeWorkspaceId) return;
		const first = groupedWorksets[0]?.threads[0];
		if (first) onSelectWorkspace(first.id);
	});

	$effect(() => {
		if (!worksetSwitcherEnabled && switcherOpen) {
			switcherOpen = false;
		}
	});

	$effect(() => {
		if (!switcherOpen) return;
		switcherQuery = '';
		switcherFocusIndex = 0;
		const token = requestAnimationFrame(() => switcherSearchInputEl?.focus());
		return () => cancelAnimationFrame(token);
	});

	$effect(() => {
		if (!switcherOpen) return;
		const maxIndex = Math.max(0, switcherFlatWorksets.length - 1);
		switcherFocusIndex = Math.min(switcherFocusIndex, maxIndex);
	});

	$effect(() => {
		if (!switcherOpen || !switcherListEl) return;
		const activeItem = switcherListEl.querySelector<HTMLElement>(
			`[data-switcher-index="${switcherFocusIndex}"]`,
		);
		activeItem?.scrollIntoView({ block: 'nearest' });
	});
</script>

<div class="explorer-panel">
	<div class="panel-header">
		<span class="panel-title">Explorer</span>
		<div class="panel-header-actions">
			<button
				type="button"
				class="icon-btn"
				title="New Workset"
				aria-label="Create workset"
				onclick={onCreateWorkspace}
			>
				<Plus size={13} />
			</button>
			<button
				type="button"
				class="icon-btn"
				title="Collapse Explorer (⌘B)"
				aria-label="Collapse explorer"
				onclick={onCollapse}
			>
				<PanelLeftClose size={13} />
			</button>
		</div>
	</div>

	<div class="workset-switcher-shell">
		{#if selectedWorkset}
			<button
				type="button"
				class="workset-switcher"
				class:locked={!worksetSwitcherEnabled}
				aria-label={worksetSwitcherEnabled ? 'Switch workset' : 'Current workset'}
				aria-expanded={switcherOpen}
				disabled={!worksetSwitcherEnabled}
				onclick={() => {
					if (!worksetSwitcherEnabled) return;
					switcherOpen = !switcherOpen;
				}}
			>
				<span class="switcher-icon"><Box size={11} /></span>
				<span class="switcher-label">{selectedWorkset.label}</span>
				<div class="switcher-health">
					{#each selectedWorkset.health.slice(0, 4) as status, index (`selected-${index}`)}
						<span class="ws-dot ws-dot-xs ws-dot-{status}"></span>
					{/each}
				</div>
				{#if worksetSwitcherEnabled}
					<ChevronDown size={11} class={`switcher-chevron${switcherOpen ? ' open' : ''}`} />
				{/if}
			</button>

			{#if switcherOpen}
				<button
					type="button"
					class="switcher-backdrop"
					aria-label="Close workset switcher"
					onclick={() => (switcherOpen = false)}
				></button>
				<div
					class="switcher-dropdown"
					role="dialog"
					aria-label="Workset switcher"
					tabindex="-1"
					onkeydown={handleSwitcherKeydown}
				>
					<div class="switcher-search-row">
						<Search size={11} class="switcher-search-icon" />
						<input
							bind:this={switcherSearchInputEl}
							bind:value={switcherQuery}
							class="switcher-search-input"
							type="text"
							placeholder="Search worksets..."
							aria-label="Search worksets"
						/>
						{#if switcherQuery.trim().length > 0}
							<span class="switcher-search-count"
								>{switcherFlatWorksets.length}/{switcherAvailableWorksets.length}</span
							>
						{/if}
					</div>

					<div class="switcher-list" bind:this={switcherListEl}>
						{#if switcherFlatWorksets.length === 0}
							<div class="switcher-empty">No worksets match "{switcherQuery.trim()}"</div>
						{:else}
							{#if switcherPinnedWorksets.length > 0}
								<div class="switcher-section-label">
									<Pin size={8} />
									<span>Pinned</span>
								</div>
								{#each switcherPinnedWorksets as workset, index (workset.id)}
									<button
										type="button"
										class="switcher-item switcher-item-rich"
										class:focused={switcherFocusIndex === index}
										onmouseenter={() => (switcherFocusIndex = index)}
										onclick={() => selectWorkset(workset.id)}
										data-switcher-index={index}
									>
										<span class="switcher-item-icon">
											<Box size={9} />
										</span>
										<span class="switcher-item-main">
											<span class="switcher-item-label">
												{workset.label}
											</span>
											<span class="switcher-item-description">
												{workset.description}
											</span>
											<span class="switcher-item-meta">
												{#if workset.activeThreads > 0}
													<span class="switcher-meta-chip">
														<MessageSquare size={8} />
														{workset.activeThreads}
													</span>
												{/if}
												{#if workset.openPrs > 0}
													<span class="switcher-meta-chip switcher-meta-pr">
														<GitPullRequest size={8} />
														{workset.openPrs}
													</span>
												{/if}
												{#if workset.dirtyRepos > 0}
													<span class="switcher-meta-chip switcher-meta-dirty">
														<FolderGit2 size={8} />
														{workset.dirtyRepos}
													</span>
												{/if}
												<span class="switcher-meta-time">
													{formatRelativeAge(workset.lastActiveTs)}
												</span>
											</span>
										</span>
										<span class="switcher-item-side">
											{#if workset.shortcutNumber}
												<kbd class="ui-kbd switcher-item-kbd">⌘{workset.shortcutNumber}</kbd>
											{/if}
											<span class="switcher-item-health">
												{#each workset.health.slice(0, 4) as status, dotIndex (`${workset.id}-${dotIndex}`)}
													<span class="ws-dot ws-dot-xs ws-dot-{status}"></span>
												{/each}
											</span>
											{#if switcherFocusIndex === index}
												<Check size={10} class="switcher-item-check" />
											{/if}
										</span>
									</button>
								{/each}
							{/if}

							{#if switcherRecentWorksets.length > 0}
								{#if switcherPinnedWorksets.length > 0}
									<div class="switcher-section-divider"></div>
								{/if}
								<div class="switcher-section-label">
									<Clock3 size={8} />
									<span>Recent</span>
								</div>
								{#each switcherRecentWorksets as workset, recentIndex (workset.id)}
									<button
										type="button"
										class="switcher-item switcher-item-rich"
										class:focused={switcherFocusIndex ===
											switcherPinnedWorksets.length + recentIndex}
										onmouseenter={() =>
											(switcherFocusIndex = switcherPinnedWorksets.length + recentIndex)}
										onclick={() => selectWorkset(workset.id)}
										data-switcher-index={switcherPinnedWorksets.length + recentIndex}
									>
										<span class="switcher-item-icon">
											<Box size={9} />
										</span>
										<span class="switcher-item-main">
											<span class="switcher-item-label">
												{workset.label}
											</span>
											<span class="switcher-item-description">
												{workset.description}
											</span>
											<span class="switcher-item-meta">
												{#if workset.activeThreads > 0}
													<span class="switcher-meta-chip">
														<MessageSquare size={8} />
														{workset.activeThreads}
													</span>
												{/if}
												{#if workset.openPrs > 0}
													<span class="switcher-meta-chip switcher-meta-pr">
														<GitPullRequest size={8} />
														{workset.openPrs}
													</span>
												{/if}
												{#if workset.dirtyRepos > 0}
													<span class="switcher-meta-chip switcher-meta-dirty">
														<FolderGit2 size={8} />
														{workset.dirtyRepos}
													</span>
												{/if}
												<span class="switcher-meta-time">
													{formatRelativeAge(workset.lastActiveTs)}
												</span>
											</span>
										</span>
										<span class="switcher-item-side">
											{#if workset.shortcutNumber}
												<kbd class="ui-kbd switcher-item-kbd">⌘{workset.shortcutNumber}</kbd>
											{/if}
											<span class="switcher-item-health">
												{#each workset.health.slice(0, 4) as status, dotIndex (`${workset.id}-${dotIndex}`)}
													<span class="ws-dot ws-dot-xs ws-dot-{status}"></span>
												{/each}
											</span>
											{#if switcherFocusIndex === switcherPinnedWorksets.length + recentIndex}
												<Check size={10} class="switcher-item-check" />
											{/if}
										</span>
									</button>
								{/each}
							{/if}
						{/if}
					</div>

					<div class="switcher-footer-hints">
						<span><kbd class="ui-kbd">↑↓</kbd> navigate</span>
						<span><kbd class="ui-kbd">↵</kbd> select</span>
						<span><kbd class="ui-kbd">esc</kbd> close</span>
					</div>
				</div>
			{/if}
		{/if}
	</div>

	<div class="panel-body">
		{#if selectedWorkset}
			<div class="section-heading">
				<MessageSquare size={9} />
				<span>Threads</span>
				<span class="count">{selectedWorkset.threads.length}</span>
			</div>
			<div class="thread-list">
				{#each selectedWorkset.threads as thread (thread.id)}
					<button
						type="button"
						class="thread-item"
						class:active={thread.id === activeWorkspaceId}
						onclick={() => selectThread(thread.id)}
					>
						<span class="status-dot status-{getThreadStatus(thread)}"></span>
						<span class="thread-name">{thread.name}</span>
						{#if thread.repos.some((repo) => isOpenTrackedPullRequest(repo))}
							<span class="thread-pr-indicator">PR</span>
						{/if}
					</button>
				{/each}
				<button
					type="button"
					class="thread-create-row"
					onclick={() => onCreateThread(selectedWorkset.id)}
					title={`Create thread in ${selectedWorkset.label}`}
					aria-label={`Create thread in ${selectedWorkset.label}`}
				>
					<Plus size={10} />
					<span>New Thread</span>
				</button>
			</div>

			<div class="repos-heading">
				<button
					type="button"
					class="section-heading repos-toggle"
					onclick={() => (reposOpen = !reposOpen)}
				>
					{#if reposOpen}
						<ChevronDown size={9} />
					{:else}
						<ChevronRight size={9} />
					{/if}
					<FolderGit2 size={9} />
					<span>Repos</span>
					<span class="count">{selectedWorkset.repos.length}</span>
				</button>
				{#if canManageRepos}
					<button
						type="button"
						class="repo-add-btn"
						title={`Add repo to ${selectedWorkset.label}`}
						aria-label={`Add repo to ${selectedWorkset.label}`}
						onclick={() => onAddRepo(selectedWorkset.id)}
					>
						<Plus size={10} />
						<span>Add Repo</span>
					</button>
				{/if}
			</div>
			{#if reposOpen}
				<div class="repo-list">
					{#each selectedWorkset.repos as repo (`${selectedWorkset.id}-${repo}`)}
						<div class="repo-item" title={repo}>
							<FolderGit2 size={10} />
							<span>{repo}</span>
						</div>
					{/each}
				</div>
			{/if}

			<div class="summary-card">
				<div class="summary-row">
					<span>Open PRs</span>
					<strong>{selectedWorkset.openPrs}</strong>
				</div>
				<div class="summary-row">
					<span>Dirty repos</span>
					<strong>{selectedWorkset.dirtyRepos}</strong>
				</div>
				<div class="summary-row">
					<span>Line diff</span>
					<strong>
						<span class="plus">+{selectedWorkset.linesAdded}</span>
						<span class="minus">-{selectedWorkset.linesRemoved}</span>
					</strong>
				</div>
			</div>
		{/if}
	</div>

	<div class="panel-footer">
		<div class="footer-controls">
			<div class="footer-nav">
				<button
					type="button"
					class="icon-btn"
					class:active={activeView !== 'skill-registry' && activeSurface === 'terminal'}
					title="Cockpit"
					aria-label="Open cockpit"
					onclick={onOpenCockpit}
				>
					<Terminal size={13} />
				</button>
				<button
					type="button"
					class="icon-btn"
					class:active={activeView !== 'skill-registry' && activeSurface === 'pull-requests'}
					title="Pull Requests"
					aria-label="Open pull requests"
					onclick={onOpenPullRequests}
				>
					<GitPullRequest size={13} />
				</button>
				<button
					type="button"
					class="icon-btn"
					class:active={activeView === 'skill-registry'}
					title="Skills"
					aria-label="Open skills"
					onclick={onOpenSkills}
				>
					<Sparkles size={13} />
				</button>
			</div>
			<div class="footer-utils">
				<button
					type="button"
					class="icon-btn"
					title="Settings"
					aria-label="Open settings"
					onclick={onOpenSettings}
				>
					<Settings size={13} />
				</button>
				<div class="footer-kbd">
					<kbd class="ui-kbd">⌘B</kbd>
				</div>
			</div>
		</div>
	</div>
</div>

<style>
	.explorer-panel {
		height: 100%;
		display: flex;
		flex-direction: column;
		background: transparent;
	}

	.panel-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 12px 12px 8px;
	}

	.panel-title {
		font-size: var(--text-xs);
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--muted);
	}

	.panel-header-actions {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}

	.icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 22px;
		height: 22px;
		border-radius: 6px;
		border: 1px solid transparent;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition:
			color var(--transition-fast),
			background var(--transition-fast),
			border-color var(--transition-fast);
	}

	.icon-btn:hover {
		color: var(--text);
		background: color-mix(in srgb, var(--panel-strong) 65%, transparent);
		border-color: var(--border);
	}

	.icon-btn.active {
		color: var(--text);
		background: color-mix(in srgb, var(--accent) 14%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--accent) 46%, var(--border));
	}

	.workset-switcher-shell {
		position: relative;
		padding: 0 8px 4px;
		z-index: 2;
	}

	.workset-switcher {
		width: 100%;
		display: flex;
		align-items: center;
		gap: 7px;
		padding: 7px 8px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--border) 80%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 72%, transparent);
		color: var(--text);
		cursor: pointer;
	}

	.workset-switcher:disabled {
		cursor: default;
	}

	.workset-switcher.locked {
		border-color: color-mix(in srgb, var(--border) 68%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 62%, transparent);
	}

	.switcher-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		color: var(--accent);
		flex-shrink: 0;
	}

	.switcher-label {
		flex: 1;
		min-width: 0;
		text-align: left;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		font-size: var(--text-sm);
		font-weight: 600;
	}

	.switcher-health {
		display: inline-flex;
		align-items: center;
		gap: 3px;
	}

	.switcher-chevron {
		color: var(--muted);
		transition: transform var(--transition-fast);
	}

	.switcher-chevron.open {
		transform: rotate(180deg);
	}

	.switcher-backdrop {
		position: fixed;
		inset: 0;
		border: none;
		background: transparent;
		padding: 0;
	}

	.switcher-dropdown {
		position: absolute;
		left: 8px;
		right: 8px;
		top: calc(100% + 4px);
		border-radius: 10px;
		border: 1px solid var(--glass-border);
		background: var(--glass-bg-strong);
		backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
		-webkit-backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
		box-shadow: var(--glass-shadow), var(--inset-highlight);
		display: flex;
		flex-direction: column;
		overflow: hidden;
		max-height: min(72vh, 420px);
		z-index: 4;
	}

	.switcher-search-row {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 8px 10px;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 52%, transparent);
	}

	.switcher-search-icon {
		color: var(--muted);
		flex-shrink: 0;
	}

	.switcher-search-input {
		flex: 1;
		min-width: 0;
		border: none;
		outline: none;
		background: transparent;
		color: var(--text);
		font-size: 11px;
	}

	.switcher-search-input::placeholder {
		color: color-mix(in srgb, var(--muted) 82%, transparent);
	}

	.switcher-search-count {
		font-size: 10px;
		font-family: var(--font-mono);
		color: var(--muted);
	}

	.switcher-list {
		overflow-y: auto;
		padding: 4px 0;
		max-height: min(56vh, 340px);
	}

	.switcher-empty {
		padding: 16px 12px;
		text-align: center;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.switcher-section-label {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 4px 10px;
		font-size: 9px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: color-mix(in srgb, var(--muted) 84%, transparent);
	}

	.switcher-section-divider {
		margin: 6px 8px;
		height: 1px;
		background: color-mix(in srgb, var(--border) 38%, transparent);
	}

	.switcher-item {
		display: flex;
		width: 100%;
		padding: 7px 10px;
		border: none;
		background: transparent;
		color: var(--text);
		border-radius: 0;
		cursor: pointer;
		text-align: left;
	}

	.switcher-item:hover,
	.switcher-item.focused {
		background: color-mix(in srgb, var(--accent) 10%, var(--panel-strong));
	}

	.switcher-item-rich {
		align-items: flex-start;
		gap: 8px;
	}

	.switcher-item-icon {
		width: 18px;
		height: 18px;
		margin-top: 1px;
		border-radius: 5px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		color: var(--accent);
		background: color-mix(in srgb, var(--panel-strong) 74%, transparent);
	}

	.switcher-item-main {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.switcher-item-label {
		font-size: 11px;
		font-weight: 600;
		color: var(--text);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.switcher-item-description {
		font-size: 10px;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.switcher-item-meta {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		margin-top: 2px;
	}

	.switcher-meta-chip {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		font-size: 9px;
		color: color-mix(in srgb, var(--muted) 86%, transparent);
	}

	.switcher-meta-pr {
		color: #8b8aed;
	}

	.switcher-meta-dirty {
		color: var(--warning);
	}

	.switcher-meta-time {
		font-size: 9px;
		color: color-mix(in srgb, var(--muted) 90%, transparent);
	}

	.switcher-item-side {
		display: inline-flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 4px;
		flex-shrink: 0;
		min-width: 44px;
	}

	.switcher-item-health {
		display: inline-flex;
		align-items: center;
		gap: 3px;
	}

	.switcher-item-kbd {
		font-size: 9px;
	}

	.switcher-item-check {
		color: var(--accent);
	}

	.switcher-footer-hints {
		display: inline-flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 6px 10px;
		border-top: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
		font-size: 9px;
		color: var(--muted);
	}

	.switcher-footer-hints span {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.switcher-footer-hints .ui-kbd {
		font-size: 8px;
	}

	.panel-body {
		flex: 1;
		overflow-y: auto;
		padding: 4px 8px 10px;
	}

	.section-heading {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		width: 100%;
		padding: 8px 6px 4px;
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: color-mix(in srgb, var(--muted) 82%, transparent);
	}

	.repos-toggle {
		border: none;
		background: transparent;
		cursor: pointer;
	}

	.repos-heading {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.repo-add-btn {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		height: 22px;
		padding: 0 8px;
		border-radius: 7px;
		border: 1px solid color-mix(in srgb, var(--border) 80%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 56%, transparent);
		color: var(--muted);
		font-size: 10px;
		font-weight: 600;
		cursor: pointer;
		white-space: nowrap;
		transition:
			color var(--transition-fast),
			background var(--transition-fast),
			border-color var(--transition-fast);
	}

	.repo-add-btn:hover {
		color: var(--text);
		background: color-mix(in srgb, var(--accent) 14%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--accent) 44%, var(--border));
	}

	.count {
		margin-left: auto;
		font-size: 10px;
		font-family: var(--font-mono);
		color: var(--muted);
	}

	.thread-list,
	.repo-list {
		display: flex;
		flex-direction: column;
		gap: 3px;
	}

	.thread-item {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		border: none;
		background: transparent;
		border-radius: 8px;
		padding: 5px 8px;
		color: var(--text);
		cursor: pointer;
		text-align: left;
	}

	.thread-item:hover {
		background: color-mix(in srgb, var(--panel-strong) 65%, transparent);
	}

	.thread-item.active {
		background: color-mix(in srgb, var(--accent) 12%, var(--panel-strong));
	}

	.thread-name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: var(--text-xs);
		font-weight: 600;
	}

	.thread-pr-indicator {
		font-size: 10px;
		color: #8b8aed;
		font-family: var(--font-mono);
	}

	.thread-create-row {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		padding: 6px 8px;
		border: none;
		background: transparent;
		border-radius: 8px;
		color: var(--muted);
		font-size: var(--text-xs);
		cursor: pointer;
		text-align: left;
	}

	.thread-create-row:hover {
		color: var(--text);
		background: color-mix(in srgb, var(--panel-strong) 65%, transparent);
	}

	.repo-item {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 3px 8px;
		color: var(--muted);
		font-size: var(--text-xs);
	}

	.repo-item span {
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}

	.summary-card {
		margin-top: 10px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--border) 70%, transparent);
		background: color-mix(in srgb, var(--panel-strong) 40%, transparent);
		padding: 8px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.summary-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.summary-row strong {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: var(--text-xs);
		font-family: var(--font-mono);
	}

	.plus {
		color: var(--success);
	}

	.minus {
		color: var(--danger);
	}

	.status-dot {
		width: 6px;
		height: 6px;
		border-radius: 999px;
		flex-shrink: 0;
	}

	.status-dot.status-active {
		background: var(--success);
	}

	.status-dot.status-in-review {
		background: #8b8aed;
	}

	.status-dot.status-merged {
		background: var(--accent);
	}

	.status-dot.status-stale {
		background: color-mix(in srgb, var(--muted) 68%, white);
	}

	.panel-footer {
		border-top: 1px solid var(--border);
		padding: 8px;
		display: flex;
		align-items: center;
	}

	.footer-controls {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
	}

	.footer-nav {
		display: inline-flex;
		align-items: center;
		gap: 8px;
	}

	.footer-utils {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		margin-left: auto;
	}

	.footer-kbd {
		display: inline-flex;
		align-items: center;
		color: var(--muted);
		font-size: 11px;
	}
</style>
