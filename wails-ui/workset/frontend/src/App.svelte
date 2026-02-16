<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import {
		ArrowLeft,
		GitPullRequest,
		LayoutDashboard,
		PlusCircle,
		Settings,
		Sparkles,
		Terminal,
	} from '@lucide/svelte';
	import {
		activeRepo,
		activeWorkspace,
		activeWorkspaceId,
		applyRepoLocalStatus,
		clearRepo,
		loadWorkspaces,
		loadingWorkspaces,
		refreshWorkspacesStatus,
		selectWorkspace,
		toggleWorkspacePin,
		workspaceError,
		workspaces,
	} from './lib/state';
	import {
		archiveWorkspace,
		closeWorkspacePopout,
		listWorkspacePopouts,
		openWorkspacePopout,
		previewRepoHooks,
		setWorkspaceDescription,
		unarchiveWorkspace,
	} from './lib/api/workspaces';
	import type { RepoLocalStatus } from './lib/api/github';
	import { fetchGitHubAuthInfo } from './lib/api/github';
	import {
		EVENT_REPO_DIFF_LOCAL_STATUS,
		EVENT_WORKSPACE_POPOUT_CLOSED,
		EVENT_WORKSPACE_POPOUT_OPENED,
	} from './lib/events';
	import { subscribeRepoDiffEvent } from './lib/repoDiffService';
	import { releaseWorkspaceTerminals } from './lib/terminal/terminalService';
	import { subscribeWailsEvent } from './lib/wailsEventRegistry';
	import { startRepoStatusWatch, stopRepoStatusWatch } from './lib/api/repo-diff';
	import EmptyState from './lib/components/EmptyState.svelte';
	import GitHubLoginModal from './lib/components/GitHubLoginModal.svelte';
	import SettingsPanel from './lib/components/SettingsPanel.svelte';
	import WorkspaceActionModal from './lib/components/WorkspaceActionModal.svelte';
	import CommandPalette, { type AppView } from './lib/components/chrome/CommandPalette.svelte';
	import ContextBar from './lib/components/chrome/ContextBar.svelte';
	import CommandCenterView from './lib/components/views/CommandCenterView.svelte';
	import OnboardingView, {
		type OnboardingDraft,
		type OnboardingStartResult,
	} from './lib/components/views/OnboardingView.svelte';
	import PROrchestrationView from './lib/components/views/PROrchestrationView.svelte';
	import SkillRegistryView from './lib/components/views/SkillRegistryView.svelte';
	import TerminalCockpitView from './lib/components/views/TerminalCockpitView.svelte';
	import WorksetHubView, {
		type WorksetGroupMode,
		type WorksetLayoutMode,
	} from './lib/components/views/WorksetHubView.svelte';
	import { workspaceActionMutations } from './lib/services/workspaceActionService';
	import {
		loadOnboardingCatalog,
		type RegisteredRepo,
		type WorksetTemplate,
	} from './lib/view-models/onboardingViewModel';
	import { buildShortcutMap, mapWorkspacesToSummaries } from './lib/view-models/worksetViewModel';
	import {
		readWorksetHubGroupMode,
		readWorksetHubLayoutMode,
		persistWorksetHubGroupMode,
		persistWorksetHubLayoutMode,
	} from './lib/worksetHubPreferences';

	type RepoDiffLocalStatusEvent = {
		workspaceId: string;
		repoId: string;
		status: RepoLocalStatus;
	};

	type WorkspacePopoutEvent = {
		workspaceId: string;
		windowName: string;
		open: boolean;
	};

	type WorkspaceActionMode =
		| 'create'
		| 'rename'
		| 'add-repo'
		| 'archive'
		| 'remove-workspace'
		| 'remove-repo'
		| null;

	type NavItem = {
		view: AppView;
		label: string;
		icon: typeof LayoutDashboard;
	};

	const railNavItems: NavItem[] = [
		{ view: 'command-center', label: 'Command Center', icon: LayoutDashboard },
		{ view: 'terminal-cockpit', label: 'Engineering Cockpit', icon: Terminal },
		{ view: 'pr-orchestration', label: 'PR Orchestration', icon: GitPullRequest },
		{ view: 'skill-registry', label: 'Skill Registry', icon: Sparkles },
	];
	const popoutNavItems: NavItem[] = [
		{ view: 'command-center', label: 'Command Center', icon: LayoutDashboard },
		{ view: 'terminal-cockpit', label: 'Engineering Cockpit', icon: Terminal },
		{ view: 'pr-orchestration', label: 'PR Orchestration', icon: GitPullRequest },
	];

	const contextViews: AppView[] = [
		'command-center',
		'terminal-cockpit',
		'pr-orchestration',
		'skill-registry',
	];
	const popoutViews = new Set<AppView>(popoutNavItems.map((item) => item.view));
	const appViews = new Set<AppView>([
		'workset-hub',
		'command-center',
		'terminal-cockpit',
		'pr-orchestration',
		'skill-registry',
		'onboarding',
	]);
	const searchParams =
		typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
	const popoutMode = searchParams?.get('popout') === '1';
	const requestedWorkspace = searchParams?.get('workspace')?.trim() ?? '';
	const requestedView = searchParams?.get('view')?.trim() ?? '';
	const requestedAppView = appViews.has(requestedView as AppView)
		? (requestedView as AppView)
		: null;
	const fixedWorkspaceId = popoutMode && requestedWorkspace !== '' ? requestedWorkspace : null;
	const initialView: AppView = popoutMode
		? requestedAppView && popoutViews.has(requestedAppView)
			? requestedAppView
			: 'command-center'
		: (requestedAppView ?? 'workset-hub');

	const repoStatusWatchers = new Map<string, { workspaceId: string; repoId: string }>();

	const hasWorkspace = $derived($activeWorkspace !== null);
	const hasRepo = $derived($activeRepo !== null);
	const hasWorkspaces = $derived($workspaces.length > 0);

	let currentView = $state<AppView>(initialView);
	let prFocusWorkspaceId = $state<string | null>(null);
	let prFocusRepoId = $state<string | null>(null);
	let prFocusToken = $state(0);
	let worksetHubGroupMode = $state<WorksetGroupMode>(readWorksetHubGroupMode());
	let worksetHubLayoutMode = $state<WorksetLayoutMode>(readWorksetHubLayoutMode());
	let workspaceActionMode = $state<WorkspaceActionMode>(null);
	let workspaceActionWorkspaceId = $state<string | null>(null);
	let workspaceActionRepoName = $state<string | null>(null);
	let settingsOpen = $state(false);
	let commandPaletteOpen = $state(false);
	let authModalOpen = $state(false);
	let authModalDismissed = $state(false);
	let popoutBusy = $state(false);
	let openPopoutWorkspaces = $state<Record<string, string>>({});
	let popoutSelectionApplied = $state(false);
	let onboardingLoading = $state(false);
	let onboardingBusy = $state(false);
	let onboardingError = $state<string | null>(null);
	let onboardingTemplates = $state<WorksetTemplate[]>([]);
	let onboardingRepoRegistry = $state<RegisteredRepo[]>([]);
	let onboardingLoaded = $state(false);

	const visibleWorkspaces = $derived.by(() => {
		if (!fixedWorkspaceId) return $workspaces;
		return $workspaces.filter((workspace) => workspace.id === fixedWorkspaceId);
	});
	const worksetSummaries = $derived.by(() => mapWorkspacesToSummaries(visibleWorkspaces));
	const shortcutMap = $derived.by(() => buildShortcutMap(visibleWorkspaces));
	const activeSummary = $derived.by(
		() => worksetSummaries.find((summary) => summary.id === $activeWorkspaceId) ?? null,
	);
	const activeShortcut = $derived.by(() =>
		$activeWorkspaceId ? shortcutMap.get($activeWorkspaceId) : undefined,
	);
	const showContextBar = $derived.by(
		() => !hasRepo && activeSummary !== null && contextViews.includes(currentView),
	);

	const updateRepoStatusWatchers = (): void => {
		if (popoutMode) return;
		const nextKeys = new Set<string>();
		for (const workspace of $workspaces) {
			if (workspace.archived) continue;
			for (const repo of workspace.repos) {
				const key = `${workspace.id}:${repo.id}`;
				nextKeys.add(key);
				if (repoStatusWatchers.has(key)) continue;
				const entry = { workspaceId: workspace.id, repoId: repo.id };
				repoStatusWatchers.set(key, entry);
				void startRepoStatusWatch(workspace.id, repo.id).catch(() => {
					repoStatusWatchers.delete(key);
				});
			}
		}

		for (const [key, entry] of repoStatusWatchers.entries()) {
			if (nextKeys.has(key)) continue;
			repoStatusWatchers.delete(key);
			void stopRepoStatusWatch(entry.workspaceId, entry.repoId).catch(() => {});
		}
	};

	const stopAllRepoStatusWatchers = (): void => {
		for (const watcher of repoStatusWatchers.values()) {
			void stopRepoStatusWatch(watcher.workspaceId, watcher.repoId).catch(() => {});
		}
		repoStatusWatchers.clear();
	};

	const isWorkspacePoppedOut = (workspaceId: string | null | undefined): boolean => {
		if (!workspaceId) return false;
		return openPopoutWorkspaces[workspaceId] !== undefined;
	};

	const updateWorkspacePopoutState = (
		workspaceId: string,
		windowName: string,
		open: boolean,
	): void => {
		const id = workspaceId.trim();
		if (!id) return;
		if (open) {
			openPopoutWorkspaces = { ...openPopoutWorkspaces, [id]: windowName };
			if (!popoutMode) {
				releaseWorkspaceTerminals(id);
				if ($activeWorkspaceId === id && currentView === 'terminal-cockpit') {
					currentView = 'command-center';
				}
			}
			return;
		}
		if (openPopoutWorkspaces[id] === undefined) return;
		const next = { ...openPopoutWorkspaces };
		delete next[id];
		openPopoutWorkspaces = next;
	};

	const loadPopoutState = async (): Promise<void> => {
		try {
			const states = await listWorkspacePopouts();
			const next: Record<string, string> = {};
			for (const state of states) {
				if (!state.open || !state.workspaceId) continue;
				next[state.workspaceId] = state.windowName;
			}
			openPopoutWorkspaces = next;
			if (!popoutMode) {
				for (const workspaceId of Object.keys(next)) {
					releaseWorkspaceTerminals(workspaceId);
				}
				if ($activeWorkspaceId && next[$activeWorkspaceId] && currentView === 'terminal-cockpit') {
					currentView = 'command-center';
				}
			}
		} catch {
			// ignore state probe failures
		}
	};

	const checkGitHubAuth = async (): Promise<void> => {
		if (authModalDismissed) return;
		try {
			const info = await fetchGitHubAuthInfo();
			if (!info.status.authenticated) {
				authModalOpen = true;
			}
		} catch {
			// ignore auth probe failures
		}
	};

	const handleAuthClose = (): void => {
		authModalOpen = false;
		authModalDismissed = true;
	};

	const handleAuthSuccess = (): void => {
		authModalOpen = false;
		authModalDismissed = true;
	};

	const setView = (view: AppView): void => {
		if (popoutMode && !popoutViews.has(view)) {
			return;
		}
		currentView = view;
		if (hasRepo) {
			clearRepo();
		}
		if (view === 'onboarding') {
			void ensureOnboardingCatalog();
		}
	};

	const ensureOnboardingCatalog = async (): Promise<void> => {
		if (onboardingLoading || onboardingLoaded) return;
		onboardingLoading = true;
		onboardingError = null;
		try {
			const catalog = await loadOnboardingCatalog();
			onboardingTemplates = catalog.templates;
			onboardingRepoRegistry = catalog.repoRegistry;
			onboardingLoaded = true;
		} catch (error) {
			onboardingError = error instanceof Error ? error.message : 'Failed to load onboarding data.';
		} finally {
			onboardingLoading = false;
		}
	};

	const handleSelectWorkspace = (workspaceId: string): void => {
		if (fixedWorkspaceId && workspaceId !== fixedWorkspaceId) {
			return;
		}
		selectWorkspace(workspaceId);
		if (currentView === 'workset-hub' || currentView === 'onboarding') {
			currentView = 'command-center';
		}
	};

	const handleSelectWorkspaceFromPalette = (workspaceId: string): void => {
		if (fixedWorkspaceId && workspaceId !== fixedWorkspaceId) {
			return;
		}
		selectWorkspace(workspaceId);
		currentView = 'command-center';
		clearRepo();
	};

	const handleSelectRepo = (workspaceId: string, repoId: string): void => {
		if (fixedWorkspaceId && workspaceId !== fixedWorkspaceId) {
			return;
		}
		if ($activeWorkspaceId !== workspaceId) {
			selectWorkspace(workspaceId);
		}

		const workspace = $workspaces.find((entry) => entry.id === workspaceId);
		const repo = workspace?.repos.find((entry) => entry.id === repoId);
		if (!repo) {
			return;
		}

		clearRepo();
		prFocusWorkspaceId = workspaceId;
		prFocusRepoId = repoId;
		prFocusToken += 1;
		currentView = 'pr-orchestration';
	};

	const handleCreateWorkspace = (): void => {
		if (popoutMode) {
			return;
		}
		onboardingError = null;
		setView('onboarding');
	};

	const openWorkspaceActionModal = (
		mode: Exclude<WorkspaceActionMode, null>,
		workspaceId: string | null = null,
		repoName: string | null = null,
	): void => {
		if (popoutMode) return;
		workspaceActionMode = mode;
		workspaceActionWorkspaceId = workspaceId;
		workspaceActionRepoName = repoName;
	};

	const closeWorkspaceActionModal = (): void => {
		workspaceActionMode = null;
		workspaceActionWorkspaceId = null;
		workspaceActionRepoName = null;
	};

	const handleAddRepo = (workspaceId: string): void => {
		if (fixedWorkspaceId && workspaceId !== fixedWorkspaceId) {
			return;
		}
		openWorkspaceActionModal('add-repo', workspaceId);
	};

	const handleOnboardingStart = async (
		draft: OnboardingDraft,
	): Promise<OnboardingStartResult | void> => {
		if (onboardingBusy) return;
		onboardingBusy = true;
		onboardingError = null;
		try {
			const result = await workspaceActionMutations.createWorkspace({
				finalName: draft.workspaceName,
				primaryInput: draft.primarySource,
				directRepos: draft.directRepos,
				selectedAliases: draft.selectedAliases,
				selectedGroups: draft.selectedGroups,
			});
			if (draft.description) {
				await setWorkspaceDescription(result.workspaceName, draft.description);
			}
			return {
				workspaceName: result.workspaceName,
				warnings: result.warnings,
				pendingHooks: result.pendingHooks,
				hookRuns: result.hookRuns,
			};
		} catch (error) {
			onboardingError =
				error instanceof Error ? error.message : 'Failed to create workspace from onboarding.';
			throw error;
		} finally {
			onboardingBusy = false;
		}
	};

	const handleOnboardingComplete = async (workspaceName: string): Promise<void> => {
		await loadWorkspaces(true);
		selectWorkspace(workspaceName);
		clearRepo();
		currentView = 'command-center';
	};

	const handleOnboardingPreviewHooks = async (source: string): Promise<string[]> => {
		return previewRepoHooks(source);
	};

	const handleToggleArchive = async (workspaceId: string, archived: boolean): Promise<void> => {
		try {
			if (archived) {
				await unarchiveWorkspace(workspaceId);
			} else {
				await archiveWorkspace(workspaceId, 'Archived from workspace UI');
			}
			await refreshWorkspacesStatus(true);
		} catch {
			// ignore archive errors; they are surfaced elsewhere
		}
	};

	const handleShortcutSwitch = (index: number): void => {
		if (popoutMode) return;
		for (const [workspaceId, number] of shortcutMap.entries()) {
			if (number !== index) continue;
			handleSelectWorkspace(workspaceId);
			return;
		}
	};

	const handleGlobalKeydown = (event: KeyboardEvent): void => {
		if (popoutMode) return;
		if (!(event.metaKey || event.ctrlKey)) return;
		const key = event.key.toLowerCase();
		if (key === 'k') {
			event.preventDefault();
			commandPaletteOpen = !commandPaletteOpen;
			return;
		}
		if (key >= '1' && key <= '5') {
			event.preventDefault();
			handleShortcutSwitch(Number(key));
		}
	};

	let repoStatusUnsubscribe: (() => void) | null = null;
	let popoutOpenedUnsubscribe: (() => void) | null = null;
	let popoutClosedUnsubscribe: (() => void) | null = null;

	const handleOpenPopout = async (workspaceId: string): Promise<void> => {
		if (!workspaceId || popoutBusy) return;
		popoutBusy = true;
		try {
			const state = await openWorkspacePopout(workspaceId);
			updateWorkspacePopoutState(state.workspaceId, state.windowName, state.open);
		} catch {
			// ignore popout launch errors in UI
		} finally {
			popoutBusy = false;
		}
	};

	const handleClosePopout = async (workspaceId: string): Promise<void> => {
		if (!workspaceId || popoutBusy) return;
		popoutBusy = true;
		try {
			await closeWorkspacePopout(workspaceId);
			updateWorkspacePopoutState(workspaceId, '', false);
		} catch {
			// ignore popout close errors in UI
		} finally {
			popoutBusy = false;
		}
	};

	onMount(() => {
		void loadWorkspaces(true);
		void loadPopoutState();
		if (!popoutMode) {
			void checkGitHubAuth();
		}
		repoStatusUnsubscribe = subscribeRepoDiffEvent<RepoDiffLocalStatusEvent>(
			EVENT_REPO_DIFF_LOCAL_STATUS,
			(payload) => {
				applyRepoLocalStatus(payload.workspaceId, payload.repoId, payload.status);
			},
		);
		popoutOpenedUnsubscribe = subscribeWailsEvent<WorkspacePopoutEvent>(
			EVENT_WORKSPACE_POPOUT_OPENED,
			(payload) => {
				updateWorkspacePopoutState(payload.workspaceId, payload.windowName, true);
			},
		);
		popoutClosedUnsubscribe = subscribeWailsEvent<WorkspacePopoutEvent>(
			EVENT_WORKSPACE_POPOUT_CLOSED,
			(payload) => {
				updateWorkspacePopoutState(payload.workspaceId, payload.windowName, false);
			},
		);
	});

	onDestroy(() => {
		stopAllRepoStatusWatchers();
		repoStatusUnsubscribe?.();
		repoStatusUnsubscribe = null;
		popoutOpenedUnsubscribe?.();
		popoutOpenedUnsubscribe = null;
		popoutClosedUnsubscribe?.();
		popoutClosedUnsubscribe = null;
	});

	$effect(() => {
		updateRepoStatusWatchers();
	});

	$effect(() => {
		if (typeof localStorage === 'undefined') return;
		persistWorksetHubLayoutMode(worksetHubLayoutMode);
	});

	$effect(() => {
		if (typeof localStorage === 'undefined') return;
		persistWorksetHubGroupMode(worksetHubGroupMode);
	});

	$effect(() => {
		if (!fixedWorkspaceId || popoutSelectionApplied || $loadingWorkspaces) return;
		if ($workspaces.length === 0) return;
		const target = $workspaces.find(
			(workspace) => workspace.id === fixedWorkspaceId && !workspace.archived,
		);
		if (!target) {
			popoutSelectionApplied = true;
			workspaceError.set(`Workspace "${fixedWorkspaceId}" is unavailable for popout mode.`);
			return;
		}
		selectWorkspace(target.id);
		clearRepo();
		if (!popoutViews.has(currentView)) {
			currentView = 'command-center';
		}
		popoutSelectionApplied = true;
	});
</script>

<svelte:window onkeydown={handleGlobalKeydown} />

<div class="app-shell" class:popout={popoutMode}>
	{#if !popoutMode}
		<aside class="rail">
			<button
				type="button"
				class="hub-btn"
				class:active={currentView === 'workset-hub'}
				onclick={() => setView('workset-hub')}
				data-tooltip="Workset Hub"
				aria-label="Workset Hub"
			>
				<img src="images/logo.png" alt="Workset" class="hub-icon" />
			</button>

			<nav class="rail-nav" aria-label="Main">
				{#each railNavItems as item (item.view)}
					<button
						type="button"
						class="rail-item"
						class:active={currentView === item.view}
						onclick={() => setView(item.view)}
						data-tooltip={item.label}
						aria-label={item.label}
					>
						<item.icon size={18} />
					</button>
				{/each}
			</nav>

			<div class="rail-divider"></div>

			<button
				type="button"
				class="rail-item"
				class:active={currentView === 'onboarding'}
				onclick={() => setView('onboarding')}
				data-tooltip="New Workset"
				aria-label="New Workset"
			>
				<PlusCircle size={18} />
			</button>

			<button
				type="button"
				class="rail-item settings"
				onclick={() => (settingsOpen = true)}
				data-tooltip="Settings"
				aria-label="Settings"
			>
				<Settings size={18} />
			</button>
		</aside>
	{:else}
		<aside class="rail popout-rail" aria-label="Workspace popout navigation">
			<nav class="rail-nav" aria-label="Popout views">
				{#each popoutNavItems as item (item.view)}
					<button
						type="button"
						class="rail-item"
						class:active={currentView === item.view}
						onclick={() => setView(item.view)}
						data-tooltip={item.label}
						aria-label={item.label}
					>
						<item.icon size={18} />
					</button>
				{/each}
			</nav>

			<button
				type="button"
				class="rail-item popout-return-rail"
				data-tooltip="Return to main window"
				aria-label="Return to main window"
				onclick={() => void handleClosePopout($activeWorkspaceId ?? fixedWorkspaceId ?? '')}
			>
				<ArrowLeft size={18} />
			</button>
		</aside>
	{/if}

	<section class="shell-main">
		{#if showContextBar}
			<ContextBar
				workset={activeSummary}
				shortcutNumber={popoutMode ? undefined : activeShortcut}
				showShortcut={!popoutMode}
				showPaletteHint={!popoutMode}
				onOpenHub={() => setView(popoutMode ? 'command-center' : 'workset-hub')}
				onOpenPalette={() => (commandPaletteOpen = true)}
			/>
		{/if}

		<div class="view-shell">
			{#key currentView}
				<div class="view-transition" in:fly={{ y: 10, duration: 200 }}>
					{#if $loadingWorkspaces}
						<EmptyState
							title="Loading workspaces"
							body="Fetching workspace snapshots and local status."
						/>
					{:else if $workspaceError}
						<section class="error">
							<div class="title">Failed to load workspaces</div>
							<div class="body">{$workspaceError}</div>
							<button class="retry" type="button" onclick={() => loadWorkspaces(true)}>Retry</button
							>
						</section>
					{:else if popoutMode && !hasWorkspace}
						<EmptyState
							title="Workspace unavailable"
							body="The requested workspace for this popout window could not be loaded."
							variant="centered"
						/>
					{:else if !hasWorkspace && !hasWorkspaces && currentView !== 'onboarding'}
						<EmptyState
							title="Create your first workspace"
							body="Workspaces are collections of repositories that move together across branches and PR flow."
							actionLabel="Create workspace"
							onAction={handleCreateWorkspace}
							variant="centered"
						/>
					{:else if currentView === 'workset-hub'}
						<WorksetHubView
							worksets={worksetSummaries}
							{shortcutMap}
							groupMode={worksetHubGroupMode}
							layoutMode={worksetHubLayoutMode}
							activeWorkspaceId={$activeWorkspaceId}
							onSelectWorkspace={handleSelectWorkspace}
							onCreateWorkspace={handleCreateWorkspace}
							onGroupModeChange={(value) => (worksetHubGroupMode = value)}
							onLayoutModeChange={(value) => (worksetHubLayoutMode = value)}
							onAddRepo={handleAddRepo}
							onTogglePin={(workspaceId, nextPinned) =>
								void toggleWorkspacePin(workspaceId, nextPinned)}
							onToggleArchived={(workspaceId, archived) =>
								void handleToggleArchive(workspaceId, archived)}
							onOpenPopout={handleOpenPopout}
							onClosePopout={handleClosePopout}
							{isWorkspacePoppedOut}
						/>
					{:else if currentView === 'command-center'}
						<CommandCenterView
							workspaces={visibleWorkspaces}
							activeWorkspaceId={$activeWorkspaceId}
							onCreateWorkspace={handleCreateWorkspace}
							onSelectRepo={handleSelectRepo}
						/>
					{:else if currentView === 'terminal-cockpit'}
						{#if !popoutMode && isWorkspacePoppedOut($activeWorkspaceId)}
							<EmptyState
								title="Workspace terminal is popped out"
								body="This workspace terminal is currently controlled by another window. Close the popout to reattach it here."
								actionLabel="Return To Main Window"
								onAction={() => void handleClosePopout($activeWorkspaceId ?? '')}
								variant="centered"
							/>
						{:else}
							<TerminalCockpitView
								workspace={$activeWorkspace}
								onOpenWorkspaceTerminal={handleSelectWorkspace}
								onAddRepo={handleAddRepo}
							/>
						{/if}
					{:else if currentView === 'pr-orchestration'}
						<PROrchestrationView
							workspace={$activeWorkspace}
							focusRepoId={prFocusWorkspaceId === $activeWorkspaceId ? prFocusRepoId : null}
							focusToken={prFocusWorkspaceId === $activeWorkspaceId ? prFocusToken : 0}
						/>
					{:else if currentView === 'skill-registry'}
						<SkillRegistryView />
					{:else}
						<OnboardingView
							busy={onboardingBusy}
							catalogLoading={onboardingLoading}
							errorMessage={onboardingError}
							templates={onboardingTemplates}
							repoRegistry={onboardingRepoRegistry}
							defaultWorkspaceName=""
							onStart={handleOnboardingStart}
							onPreviewHooks={handleOnboardingPreviewHooks}
							onComplete={handleOnboardingComplete}
							onCancel={() => setView('workset-hub')}
						/>
					{/if}
				</div>
			{/key}
		</div>
	</section>

	{#if !popoutMode}
		<CommandPalette
			open={commandPaletteOpen}
			worksets={worksetSummaries}
			{shortcutMap}
			onClose={() => (commandPaletteOpen = false)}
			onSelectView={setView}
			onSelectWorkset={handleSelectWorkspaceFromPalette}
		/>
	{/if}

	{#if !popoutMode && settingsOpen}
		<div
			class="overlay"
			role="button"
			tabindex="0"
			onclick={() => (settingsOpen = false)}
			onkeydown={(event) => {
				if (event.key === 'Escape') settingsOpen = false;
			}}
		>
			<div
				class="overlay-panel"
				role="presentation"
				onclick={(event) => event.stopPropagation()}
				onkeydown={(event) => event.stopPropagation()}
			>
				<SettingsPanel onClose={() => (settingsOpen = false)} />
			</div>
		</div>
	{/if}

	{#if !popoutMode && workspaceActionMode}
		<div
			class="overlay"
			role="button"
			tabindex="0"
			onclick={closeWorkspaceActionModal}
			onkeydown={(event) => {
				if (event.key === 'Escape') closeWorkspaceActionModal();
			}}
		>
			<div
				class="overlay-panel"
				role="presentation"
				onclick={(event) => event.stopPropagation()}
				onkeydown={(event) => event.stopPropagation()}
			>
				<WorkspaceActionModal
					onClose={closeWorkspaceActionModal}
					mode={workspaceActionMode}
					workspaceId={workspaceActionWorkspaceId}
					repoName={workspaceActionRepoName}
				/>
			</div>
		</div>
	{/if}

	{#if !popoutMode && authModalOpen}
		<div
			class="overlay"
			role="button"
			tabindex="0"
			onclick={handleAuthClose}
			onkeydown={(event) => {
				if (event.key === 'Escape') handleAuthClose();
			}}
		>
			<div
				class="overlay-panel"
				role="presentation"
				onclick={(event) => event.stopPropagation()}
				onkeydown={(event) => event.stopPropagation()}
			>
				<GitHubLoginModal
					cancelLabel="Not now"
					onClose={handleAuthClose}
					onSuccess={handleAuthSuccess}
				/>
			</div>
		</div>
	{/if}
</div>

<style>
	.app-shell {
		height: 100vh;
		display: grid;
		grid-template-columns: 96px 1fr;
		overflow: hidden;
		background: transparent;
		position: relative;
	}

	.app-shell.popout {
		grid-template-columns: 96px 1fr;
	}

	.rail {
		--rail-top-clearance: 52px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 14px;
		padding: 22px 10px;
		padding-top: var(--rail-top-clearance);
		padding-left: 14px;
		background: color-mix(in srgb, var(--panel) 72%, transparent);
		backdrop-filter: blur(14px);
		position: relative;
		z-index: 10;
		border-right: 1px solid color-mix(in srgb, var(--border) 78%, white);
	}

	.rail button,
	.rail nav {
		-webkit-app-region: no-drag;
	}

	.hub-btn,
	.rail-item {
		position: relative;
		width: 42px;
		height: 42px;
		border-radius: 12px;
		border: 1px solid transparent;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		display: grid;
		place-items: center;
		transition:
			color var(--transition-fast),
			background var(--transition-fast),
			border-color var(--transition-fast),
			transform var(--transition-fast);
	}

	.hub-btn {
		color: white;
		background: color-mix(in srgb, var(--accent) 55%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--accent) 52%, var(--border));
		box-shadow: 0 0 20px color-mix(in srgb, var(--accent) 30%, transparent);
		padding: 0;
		overflow: visible;
	}

	.hub-btn.active {
		transform: translateY(-1px);
	}

	.hub-btn img {
		width: 42px;
		height: 42px;
		object-fit: cover;
		border-radius: inherit;
	}

	.rail [data-tooltip]::after {
		content: attr(data-tooltip);
		position: absolute;
		left: calc(100% + 12px);
		top: 50%;
		transform: translateY(-50%) translateX(-4px);
		opacity: 0;
		pointer-events: none;
		white-space: nowrap;
		padding: 6px 9px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--border) 85%, white);
		background: color-mix(in srgb, var(--panel-strong) 90%, black);
		color: var(--text);
		font-size: var(--text-xs);
		font-weight: 600;
		letter-spacing: 0.01em;
		box-shadow: 0 10px 20px rgba(0, 0, 0, 0.3);
		transition:
			opacity var(--transition-fast),
			transform var(--transition-fast);
		z-index: 10001;
	}

	.rail [data-tooltip]:hover::after,
	.rail [data-tooltip]:focus-visible::after {
		opacity: 1;
		transform: translateY(-50%) translateX(0);
	}

	.rail-nav {
		display: flex;
		flex-direction: column;
		gap: 10px;
		margin-top: 8px;
	}

	.rail-divider {
		width: 32px;
		height: 1px;
		background: #2a3a4e;
		margin: 2px 0;
	}

	.rail-item:hover {
		color: var(--text);
		background: color-mix(in srgb, var(--panel-strong) 72%, transparent);
		border-color: color-mix(in srgb, var(--accent) 34%, var(--border));
	}

	.rail-item.active {
		color: var(--text);
		background: color-mix(in srgb, var(--accent) 16%, var(--panel-strong));
		border-color: color-mix(in srgb, var(--accent) 56%, var(--border));
	}

	.rail-item.active::before {
		content: '';
		position: absolute;
		left: -11px;
		top: 50%;
		transform: translateY(-50%);
		width: 4px;
		height: 20px;
		border-radius: 0 4px 4px 0;
		background: var(--accent);
	}

	.settings {
		margin-top: auto;
	}

	.popout-rail .rail-nav {
		margin-top: 0;
	}

	.popout-return-rail {
		margin-top: auto;
	}

	.shell-main {
		display: flex;
		flex-direction: column;
		min-height: 0;
		background: transparent;
	}

	.view-shell {
		flex: 1;
		min-height: 0;
		overflow: hidden;
		background: transparent;
	}

	.view-transition {
		height: 100%;
	}

	.error {
		margin: 24px;
		background: color-mix(in srgb, var(--panel) 88%, transparent);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 24px;
		display: grid;
		gap: 12px;
	}

	.error .title {
		font-size: var(--text-xl);
		font-weight: 600;
	}

	.error .body {
		color: var(--muted);
		font-size: var(--text-md);
	}

	.retry {
		justify-self: start;
		background: var(--accent);
		border: none;
		color: #081018;
		padding: 8px 12px;
		border-radius: var(--radius-md);
		font-weight: 600;
		cursor: pointer;
	}

	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(6, 9, 14, 0.85);
		backdrop-filter: blur(2px);
		display: grid;
		place-items: center;
		z-index: 200;
		padding: 24px;
		animation: overlayFadeIn var(--transition-normal) ease-out;
	}

	.overlay-panel {
		width: 100%;
		display: flex;
		justify-content: center;
		animation: modalSlideIn 200ms ease-out;
	}

	@keyframes overlayFadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	@keyframes modalSlideIn {
		from {
			opacity: 0;
			transform: translateY(-8px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

	@media (max-width: 860px) {
		.app-shell {
			grid-template-columns: 80px 1fr;
		}

		.rail {
			--rail-top-clearance: 48px;
			padding-left: 8px;
			padding-right: 8px;
		}

		.rail [data-tooltip]::after {
			display: none;
		}
	}
</style>
