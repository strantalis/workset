<script lang="ts">
	import {
		Box,
		Check,
		ChevronDown,
		ChevronRight,
		Clock3,
		FolderTree,
		FolderGit2,
		GitPullRequest,
		MessageSquare,
		PanelLeftClose,
		Pin,
		Plus,
		Search,
		Sparkles,
		Settings,
		Trash2,
	} from '@lucide/svelte';
	import { type ExplorerWorksetSummary } from '../../view-models/worksetViewModel';
	import { hasContext } from 'svelte';
	import { WORKSPACE_ACTIONS_KEY, useWorkspaceActions } from '../../contexts/workspaceActions';

	interface Props {
		activeWorkspaceId: string | null;
		groupedWorksets: ExplorerWorksetSummary[];
		activeTerminalWorkspaceIds?: string[];
		lockWorksetSelection?: boolean;
		canManageRepos?: boolean;
		activeView?: 'workspaces' | 'skill-registry';
		activeSurface?: 'terminal' | 'pull-requests';
		filesActive?: boolean;
		selectedWorksetId?: string | null;
		onSelectWorkspace: (workspaceId: string) => void;
		onSelectWorkset?: (worksetId: string) => void;
		onCreateWorkspace?: () => void;
		onCreateThread?: (worksetId: string) => void;
		onAddRepo?: (worksetId: string) => void;
		onRemoveThread?: (threadId: string) => void;
		onOpenFiles?: () => void;
		onOpenSkills?: () => void;
		onOpenSettings?: () => void;
		onCollapse?: () => void;
	}

	const {
		activeWorkspaceId,
		groupedWorksets,
		activeTerminalWorkspaceIds = [],
		lockWorksetSelection = false,
		canManageRepos = true,
		activeView = 'workspaces',
		activeSurface = 'terminal',
		filesActive = false,
		selectedWorksetId: selectedWorksetIdProp = undefined,
		onSelectWorkspace,
		onSelectWorkset = () => {},
		onCreateWorkspace: onCreateWorkspaceProp,
		onCreateThread: onCreateThreadProp,
		onAddRepo: onAddRepoProp,
		onRemoveThread: onRemoveThreadProp,
		onOpenFiles = () => {},
		onOpenSkills = () => {},
		onOpenSettings = () => {},
		onCollapse = () => {},
	}: Props = $props();

	const noop = (): void => {};
	const ctxActions = hasContext(WORKSPACE_ACTIONS_KEY) ? useWorkspaceActions() : null;
	const onCreateWorkspace = $derived(onCreateWorkspaceProp ?? ctxActions?.createWorkspace ?? noop);
	const onCreateThread = $derived(onCreateThreadProp ?? ctxActions?.createThread ?? noop);
	const onAddRepo = $derived(onAddRepoProp ?? ctxActions?.addRepo ?? noop);
	const onRemoveThread = $derived(onRemoveThreadProp ?? ctxActions?.removeThread ?? noop);

	const activeTerminalWorkspaceIdSet = $derived.by(() => new Set(activeTerminalWorkspaceIds));

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

	const activeWorksetId = $derived.by(() => {
		if (!activeWorkspaceId) return null;
		const active = groupedWorksets.find((group) =>
			group.threads.some((thread) => thread.id === activeWorkspaceId),
		);
		return active?.id ?? null;
	});

	let localSelectedWorksetId = $state<string | null>(null);

	const selectedWorksetId = $derived.by(() => {
		const preferred = selectedWorksetIdProp ?? localSelectedWorksetId;
		if (preferred) return preferred;
		if (activeWorksetId) return activeWorksetId;
		return groupedWorksets[0]?.id ?? null;
	});

	const selectedWorkset = $derived.by<ExplorerWorksetSummary | null>(() => {
		if (selectedWorksetId) {
			const selected = groupedWorksets.find((item) => item.id === selectedWorksetId);
			if (selected) return selected;
			return null;
		}
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
	let hoveredThreadId = $state<string | null>(null);
	let focusedRemoveThreadId = $state<string | null>(null);

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
		hoveredThreadId = null;
		focusedRemoveThreadId = null;
		onSelectWorkspace(workspaceId);
		switcherOpen = false;
	};

	const isThreadTerminalActive = (workspaceId: string): boolean =>
		activeTerminalWorkspaceIdSet.has(workspaceId);

	const isThreadRemoveVisible = (workspaceId: string): boolean =>
		hoveredThreadId === workspaceId || focusedRemoveThreadId === workspaceId;

	const selectWorkset = (worksetId: string): void => {
		const workset = groupedWorksets.find((item) => item.id === worksetId);
		if (!workset) return;
		if (selectedWorksetIdProp === undefined) {
			localSelectedWorksetId = workset.id;
		}
		onSelectWorkset(workset.id);
		const thread = workset.threads[0];
		if (thread) {
			selectThread(thread.id);
			return;
		}
		switcherOpen = false;
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
			localSelectedWorksetId = null;
			switcherOpen = false;
			switcherQuery = '';
			switcherFocusIndex = 0;
			return;
		}
		if (activeWorkspaceId) return;
		if (selectedWorksetIdProp !== undefined) return;

		if (localSelectedWorksetId) {
			const selected = groupedWorksets.find((item) => item.id === localSelectedWorksetId);
			if (selected) {
				const firstSelectedThread = selected.threads[0];
				if (firstSelectedThread) onSelectWorkspace(firstSelectedThread.id);
			}
			return;
		}

		const firstAvailable = groupedWorksets.find((item) => item.threads.length > 0)?.threads[0];
		if (firstAvailable) onSelectWorkspace(firstAvailable.id);
	});

	$effect(() => {
		if (selectedWorksetIdProp === undefined && activeWorksetId) {
			localSelectedWorksetId = activeWorksetId;
		}
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
				class="ws-icon-action-btn"
				data-hover-label="New Workset"
				aria-label="Create workset"
				onclick={onCreateWorkspace}
			>
				<Plus size={13} />
			</button>
			<button
				type="button"
				class="ws-icon-action-btn"
				data-hover-label="Collapse (⌘B)"
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
					<div
						class="thread-row"
						role="presentation"
						onmouseenter={() => (hoveredThreadId = thread.id)}
						onmouseleave={() => {
							if (hoveredThreadId === thread.id) hoveredThreadId = null;
						}}
					>
						<button
							type="button"
							class="thread-item"
							class:active={thread.id === activeWorkspaceId}
							onclick={() => selectThread(thread.id)}
						>
							<span class="status-dot status-{thread.status}"></span>
							<span class="thread-name">{thread.name}</span>
							{#if isThreadTerminalActive(thread.id)}
								<span class="thread-live-indicator" title="Work in progress">
									<span class="thread-live-glyph" aria-hidden="true">
										<span class="thread-live-core"></span>
									</span>
									<span class="thread-live-label">Work in progress</span>
								</span>
							{/if}
							{#if thread.openPrs > 0}
								<span class="thread-pr-indicator">PR</span>
							{/if}
							{#if thread.reviewCommentsCount > 0}
								<span class="thread-feedback-indicator">
									<MessageSquare size={8} />
									{thread.reviewCommentsCount}
								</span>
							{/if}
						</button>
						{#if canManageRepos}
							<div class="thread-remove-slot">
								<button
									type="button"
									class="thread-remove-btn"
									class:visible={isThreadRemoveVisible(thread.id)}
									title={`Remove thread ${thread.name}`}
									aria-label={`Remove thread ${thread.name}`}
									onfocus={() => (focusedRemoveThreadId = thread.id)}
									onblur={() => {
										if (focusedRemoveThreadId === thread.id) focusedRemoveThreadId = null;
									}}
									onclick={(event) => {
										event.stopPropagation();
										onRemoveThread(thread.id);
									}}
								>
									<Trash2 size={10} />
								</button>
							</div>
						{/if}
					</div>
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
					class="footer-pane-btn"
					class:active={activeView !== 'skill-registry' &&
						(activeSurface === 'pull-requests' || filesActive)}
					data-hover-label="Code"
					aria-label="Toggle code pane"
					onclick={onOpenFiles}
				>
					<FolderTree size={13} />
				</button>
				<button
					type="button"
					class="footer-pane-btn"
					class:active={activeView === 'skill-registry'}
					data-hover-label="Skills"
					aria-label="Toggle skills view"
					onclick={onOpenSkills}
				>
					<Sparkles size={13} />
				</button>
			</div>
			<div class="footer-utils">
				<button
					type="button"
					class="ws-icon-action-btn tooltip-up"
					data-hover-label="Settings"
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

<style src="./ExplorerPanel.css"></style>
