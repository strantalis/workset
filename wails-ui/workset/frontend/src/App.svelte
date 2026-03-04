<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { PanelLeft } from '@lucide/svelte';
	import {
		activeRepo,
		activeWorkspace,
		activeWorkspaceId,
		applyRepoLocalStatus,
		clearRepo,
		loadWorkspaces,
		loadingWorkspaces,
		selectWorkspace,
		workspaceError,
		workspaces,
	} from './lib/state';
	import {
		closeWorkspacePopout,
		listWorkspacePopouts,
		openWorkspacePopout,
		previewRepoHooks,
		setWorkspaceDescription,
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
	import ExplorerPanel from './lib/components/chrome/ExplorerPanel.svelte';
	import OnboardingView from './lib/components/views/OnboardingView.svelte';
	import type {
		OnboardingDraft,
		OnboardingStartResult,
	} from './lib/components/views/OnboardingView.utils';
	import type { Workspace } from './lib/types';
	import SkillRegistryView from './lib/components/views/SkillRegistryView.svelte';
	import SpacesWorkbenchView from './lib/components/views/SpacesWorkbenchView.svelte';
	import { workspaceActionMutations } from './lib/services/workspaceActionService';
	import {
		loadOnboardingCatalog,
		type RegisteredRepo,
	} from './lib/view-models/onboardingViewModel';
	import { buildShortcutMap, mapWorkspacesToSummaries } from './lib/view-models/worksetViewModel';

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
		| 'create-thread'
		| 'rename'
		| 'add-repo'
		| 'archive'
		| 'remove-workspace'
		| 'remove-repo'
		| null;

	type CockpitSurface = 'terminal' | 'pull-requests';

	const EXPLORER_OPEN_STORAGE_KEY = 'workset:app:explorerOpen';
	const AUTO_GITHUB_AUTH_CHECK = false;

	const readExplorerOpenPreference = (): boolean => {
		if (typeof localStorage === 'undefined') return true;
		const stored = localStorage.getItem(EXPLORER_OPEN_STORAGE_KEY);
		if (stored === null) return true;
		return stored === 'true';
	};

	const contextViews: AppView[] = ['terminal-cockpit', 'skill-registry'];
	const popoutViews = new Set<AppView>(['terminal-cockpit']);
	const appViews = new Set<AppView>(['terminal-cockpit', 'skill-registry', 'onboarding']);
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
			: 'terminal-cockpit'
		: (requestedAppView ?? 'terminal-cockpit');

	const repoStatusWatchers = new Map<string, { workspaceId: string; repoId: string }>();

	const hasWorkspace = $derived($activeWorkspace !== null);
	const hasRepo = $derived($activeRepo !== null);
	const hasWorkspaces = $derived($workspaces.length > 0);

	let currentView = $state<AppView>(initialView);
	let workspaceActionMode = $state<WorkspaceActionMode>(null);
	let workspaceActionWorkspaceId = $state<string | null>(null);
	let workspaceActionWorkspaceIds = $state<string[]>([]);
	let workspaceActionRepoName = $state<string | null>(null);
	let workspaceActionWorksetName = $state<string | null>(null);
	let workspaceActionWorksetRepos = $state<string[]>([]);
	let settingsOpen = $state(false);
	let commandPaletteOpen = $state(false);
	let authModalOpen = $state(false);
	let authModalDismissed = $state(false);
	let popoutBusy = $state(false);
	let explorerOpen = $state(readExplorerOpenPreference());
	let openPopoutWorkspaces = $state<Record<string, string>>({});
	let popoutSelectionApplied = $state(false);
	let onboardingLoading = $state(false);
	let onboardingBusy = $state(false);
	let onboardingError = $state<string | null>(null);
	let onboardingRepoRegistry = $state<RegisteredRepo[]>([]);
	let onboardingLoaded = $state(false);
	let cockpitSurface = $state<CockpitSurface>('terminal');

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

	const getWorksetThreads = (workspaceId: string): Workspace[] => {
		const target = $workspaces.find(
			(workspace) => workspace.id === workspaceId && !workspace.archived,
		);
		if (!target) return [];
		const identity = deriveWorksetIdentity(target);
		return $workspaces
			.filter(
				(workspace) => !workspace.archived && deriveWorksetIdentity(workspace).id === identity.id,
			)
			.sort((left, right) => left.id.localeCompare(right.id));
	};

	const resolvePopoutWorkspaceId = (workspaceId: string): string => {
		const id = workspaceId.trim();
		if (!id) return '';
		const threads = getWorksetThreads(id);
		if (threads.length === 0) return id;
		for (const thread of threads) {
			if (openPopoutWorkspaces[thread.id] !== undefined) {
				return thread.id;
			}
		}
		return threads[0]?.id ?? id;
	};

	const releaseWorksetTerminals = (workspaceId: string): void => {
		const threads = getWorksetThreads(workspaceId);
		if (threads.length === 0) {
			releaseWorkspaceTerminals(workspaceId);
			return;
		}
		for (const thread of threads) {
			releaseWorkspaceTerminals(thread.id);
		}
	};

	const visibleWorkspaces = $derived.by(() => {
		if (!fixedWorkspaceId) return $workspaces;
		const threads = getWorksetThreads(fixedWorkspaceId);
		if (threads.length > 0) return threads;
		return $workspaces.filter((workspace) => workspace.id === fixedWorkspaceId);
	});
	const worksetSummaries = $derived.by(() => mapWorkspacesToSummaries(visibleWorkspaces));
	const existingWorksetNames = $derived.by(() => {
		const names = new Set<string>();
		for (const workspace of $workspaces) {
			const label =
				workspace.worksetLabel?.trim() ||
				workspace.workset?.trim() ||
				workspace.template?.trim() ||
				workspace.name.trim();
			if (label.length > 0) {
				names.add(label);
			}
		}
		return Array.from(names);
	});
	const shortcutMap = $derived.by(() => buildShortcutMap(visibleWorkspaces));
	const activeSummary = $derived.by(
		() => worksetSummaries.find((summary) => summary.id === $activeWorkspaceId) ?? null,
	);
	const activeShortcut = $derived.by(() =>
		$activeWorkspaceId ? shortcutMap.get($activeWorkspaceId) : undefined,
	);
	const showContextBar = $derived.by(
		() => !hasRepo && hasWorkspaces && contextViews.includes(currentView),
	);
	const explorerViews = new Set<AppView>(['terminal-cockpit', 'skill-registry']);
	const showExplorer = $derived.by(() => hasWorkspaces && explorerViews.has(currentView));

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
		const popoutWorkspaceId = resolvePopoutWorkspaceId(workspaceId);
		if (!popoutWorkspaceId) return false;
		return openPopoutWorkspaces[popoutWorkspaceId] !== undefined;
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
				releaseWorksetTerminals(id);
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
					releaseWorksetTerminals(workspaceId);
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
			onboardingRepoRegistry = catalog.repoRegistry;
			onboardingLoaded = true;
		} catch (error) {
			onboardingError = error instanceof Error ? error.message : 'Failed to load onboarding data.';
		} finally {
			onboardingLoading = false;
		}
	};

	const handleSelectWorkspace = (workspaceId: string): void => {
		if (fixedWorkspaceId && !visibleWorkspaces.some((workspace) => workspace.id === workspaceId)) {
			return;
		}
		selectWorkspace(workspaceId);
		if (currentView === 'onboarding') {
			currentView = 'terminal-cockpit';
		}
	};

	const handleSelectWorkspaceFromPalette = (workspaceId: string): void => {
		if (fixedWorkspaceId && !visibleWorkspaces.some((workspace) => workspace.id === workspaceId)) {
			return;
		}
		selectWorkspace(workspaceId);
		currentView = 'terminal-cockpit';
		clearRepo();
	};

	const handleCreateWorkspace = (): void => {
		if (popoutMode) {
			return;
		}
		setView('onboarding');
		clearRepo();
	};

	const openWorkspaceActionModal = (
		mode: Exclude<WorkspaceActionMode, null>,
		workspaceId: string | null = null,
		repoName: string | null = null,
		options: {
			worksetName?: string | null;
			worksetRepos?: string[];
			workspaceIds?: string[];
		} = {},
	): void => {
		if (popoutMode) return;
		workspaceActionMode = mode;
		workspaceActionWorkspaceId = workspaceId;
		workspaceActionWorkspaceIds = options.workspaceIds ?? [];
		workspaceActionRepoName = repoName;
		workspaceActionWorksetName = options.worksetName ?? null;
		workspaceActionWorksetRepos = options.worksetRepos ?? [];
	};

	const closeWorkspaceActionModal = (): void => {
		workspaceActionMode = null;
		workspaceActionWorkspaceId = null;
		workspaceActionWorkspaceIds = [];
		workspaceActionRepoName = null;
		workspaceActionWorksetName = null;
		workspaceActionWorksetRepos = [];
	};

	const handleCreateThread = (worksetId: string): void => {
		if (popoutMode) {
			return;
		}

		const threads = visibleWorkspaces.filter((workspace) => {
			const identity = deriveWorksetIdentity(workspace);
			return identity.id === worksetId;
		});
		if (threads.length === 0) {
			return;
		}

		const first = threads[0];
		const label = deriveWorksetIdentity(first).label;
		const repos = Array.from(
			new Set(threads.flatMap((workspace) => workspace.repos.map((repo) => repo.name))),
		).sort((left, right) => left.localeCompare(right));

		openWorkspaceActionModal('create-thread', null, null, {
			worksetName: label,
			worksetRepos: repos,
		});
	};

	const handleAddRepoToWorkset = (worksetId: string): void => {
		if (popoutMode) {
			return;
		}
		const threads = visibleWorkspaces.filter((workspace) => {
			const identity = deriveWorksetIdentity(workspace);
			return identity.id === worksetId;
		});
		if (threads.length === 0) {
			return;
		}
		const first = threads[0];
		const label = deriveWorksetIdentity(first).label;
		openWorkspaceActionModal('add-repo', first.id, null, {
			worksetName: label,
			workspaceIds: threads.map((thread) => thread.id),
		});
	};

	const handleRemoveThread = (threadId: string): void => {
		if (popoutMode) return;
		if (!threadId) return;
		if (fixedWorkspaceId && threadId !== fixedWorkspaceId) return;
		openWorkspaceActionModal('remove-workspace', threadId);
	};

	const handleOnboardingStart = async (
		draft: OnboardingDraft,
	): Promise<OnboardingStartResult | void> => {
		if (onboardingBusy) return;
		onboardingBusy = true;
		onboardingError = null;
		try {
			const result = await workspaceActionMutations.createWorkspace({
				finalName: draft.threadName,
				primaryInput: draft.primarySource,
				directRepos: draft.directRepos,
				selectedAliases: draft.selectedAliases,
				worksetName: draft.worksetName,
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
		currentView = 'terminal-cockpit';
	};

	const handleOnboardingPreviewHooks = async (source: string): Promise<string[]> => {
		return previewRepoHooks(source);
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
			return;
		}
		if (key === 'b' && showExplorer) {
			event.preventDefault();
			explorerOpen = !explorerOpen;
		}
	};

	let repoStatusUnsubscribe: (() => void) | null = null;
	let popoutOpenedUnsubscribe: (() => void) | null = null;
	let popoutClosedUnsubscribe: (() => void) | null = null;

	const handleOpenPopout = async (workspaceId: string): Promise<void> => {
		if (!workspaceId || popoutBusy) return;
		const popoutWorkspaceId = resolvePopoutWorkspaceId(workspaceId);
		if (!popoutWorkspaceId) return;
		popoutBusy = true;
		try {
			const state = await openWorkspacePopout(popoutWorkspaceId);
			updateWorkspacePopoutState(state.workspaceId, state.windowName, state.open);
		} catch {
			// ignore popout launch errors in UI
		} finally {
			popoutBusy = false;
		}
	};

	const handleClosePopout = async (workspaceId: string): Promise<void> => {
		if (!workspaceId || popoutBusy) return;
		const popoutWorkspaceId = resolvePopoutWorkspaceId(workspaceId);
		if (!popoutWorkspaceId) return;
		popoutBusy = true;
		try {
			await closeWorkspacePopout(popoutWorkspaceId);
			updateWorkspacePopoutState(popoutWorkspaceId, '', false);
		} catch {
			// ignore popout close errors in UI
		} finally {
			popoutBusy = false;
		}
	};

	onMount(() => {
		void loadWorkspaces(true);
		void loadPopoutState();
		if (!popoutMode && AUTO_GITHUB_AUTH_CHECK) {
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
		localStorage.setItem(EXPLORER_OPEN_STORAGE_KEY, String(explorerOpen));
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
			currentView = 'terminal-cockpit';
		}
		popoutSelectionApplied = true;
	});
</script>

<svelte:window onkeydown={handleGlobalKeydown} />

<div class="app-shell" class:popout={popoutMode}>
	<section class="shell-main">
		{#if showContextBar}
			<ContextBar
				workset={activeSummary}
				shortcutNumber={popoutMode ? undefined : activeShortcut}
				showShortcut={!popoutMode}
				showPaletteHint={!popoutMode}
				showPopoutToggle={!!$activeWorkspaceId}
				workspacePoppedOut={isWorkspacePoppedOut($activeWorkspaceId)}
				onTogglePopout={() => {
					const workspaceId = $activeWorkspaceId;
					if (!workspaceId) return;
					if (isWorkspacePoppedOut(workspaceId)) {
						void handleClosePopout(workspaceId);
						return;
					}
					void handleOpenPopout(workspaceId);
				}}
				onOpenPalette={() => (commandPaletteOpen = true)}
			/>
		{/if}

		<div class="shell-content">
			{#if showExplorer && explorerOpen}
				<aside class="explorer-shell" in:fly={{ x: -10, duration: 120 }}>
					<ExplorerPanel
						workspaces={visibleWorkspaces}
						activeWorkspaceId={$activeWorkspaceId}
						{shortcutMap}
						lockWorksetSelection={popoutMode}
						canManageRepos={!popoutMode}
						activeView={currentView === 'skill-registry' ? 'skill-registry' : 'terminal-cockpit'}
						activeSurface={cockpitSurface}
						onSelectWorkspace={handleSelectWorkspace}
						onCreateWorkspace={handleCreateWorkspace}
						onCreateThread={handleCreateThread}
						onAddRepo={handleAddRepoToWorkset}
						onRemoveThread={handleRemoveThread}
						onOpenCockpit={() => {
							cockpitSurface = 'terminal';
							setView('terminal-cockpit');
						}}
						onOpenPullRequests={() => {
							cockpitSurface = 'pull-requests';
							setView('terminal-cockpit');
						}}
						onOpenSkills={() => setView('skill-registry')}
						onOpenSettings={() => (settingsOpen = true)}
						onCollapse={() => (explorerOpen = false)}
					/>
				</aside>
			{/if}
			{#if showExplorer && !explorerOpen}
				<button
					type="button"
					class="explorer-reopen-btn"
					aria-label="Open Explorer (⌘B)"
					title="Open Explorer (⌘B)"
					onclick={() => (explorerOpen = true)}
				>
					<PanelLeft size={13} />
				</button>
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
								<button class="retry" type="button" onclick={() => loadWorkspaces(true)}
									>Retry</button
								>
							</section>
						{:else if popoutMode && !hasWorkspace}
							<EmptyState
								title="Workset unavailable"
								body="The requested workset for this popout window could not be loaded."
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
						{:else if currentView === 'terminal-cockpit'}
							{#if !popoutMode && isWorkspacePoppedOut($activeWorkspaceId)}
								<EmptyState
									title="This workset is open in a popout"
									body="Use the popout window to continue. Return it here anytime."
									actionLabel="Focus Popout"
									onAction={() => void handleOpenPopout($activeWorkspaceId ?? '')}
									secondaryActionLabel="Return To Main Window"
									onSecondaryAction={() => void handleClosePopout($activeWorkspaceId ?? '')}
									variant="centered"
								/>
							{:else}
								<SpacesWorkbenchView
									workspaces={visibleWorkspaces}
									activeWorkspaceId={$activeWorkspaceId}
									{popoutMode}
									useGlobalExplorer={showExplorer}
									preferredSurface={cockpitSurface}
									onSurfaceChange={(surface) => (cockpitSurface = surface)}
									onSelectWorkspace={handleSelectWorkspace}
									onCreateWorkspace={handleCreateWorkspace}
									onCreateThread={handleCreateThread}
								/>
							{/if}
						{:else if currentView === 'skill-registry'}
							<SkillRegistryView />
						{:else}
							<OnboardingView
								busy={onboardingBusy}
								catalogLoading={onboardingLoading}
								errorMessage={onboardingError}
								repoRegistry={onboardingRepoRegistry}
								defaultWorkspaceName=""
								existingWorkspaceNames={existingWorksetNames}
								onStart={handleOnboardingStart}
								onPreviewHooks={handleOnboardingPreviewHooks}
								onComplete={handleOnboardingComplete}
								onCancel={() => setView('terminal-cockpit')}
							/>
						{/if}
					</div>
				{/key}
			</div>
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
					workspaceIds={workspaceActionWorkspaceIds}
					repoName={workspaceActionRepoName}
					worksetName={workspaceActionWorksetName}
					worksetRepos={workspaceActionWorksetRepos}
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

<style src="./App.css"></style>
