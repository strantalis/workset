<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import {
		activeRepo,
		activeWorkspace,
		activeWorkspaceId,
		applyRepoDiffSummary,
		applyRepoLocalStatus,
		clearRepo,
		loadWorkspaces,
		loadingWorkspaces,
		selectWorkspace,
		workspaceError,
		workspaces,
	} from './lib/state';
	import { previewRepoHooks, setWorkspaceDescription } from './lib/api/workspaces';
	import type { RepoLocalStatus } from './lib/api/github';
	import { fetchGitHubAuthInfo } from './lib/api/github';
	import { openDefaultDocumentSession, reconcileDocumentSession } from './lib/documentSessionState';
	import {
		EVENT_REPO_DIFF_LOCAL_SUMMARY,
		EVENT_REPO_DIFF_LOCAL_STATUS,
		EVENT_REPO_DIFF_SUMMARY,
		EVENT_WORKSPACE_POPOUT_CLOSED,
		EVENT_WORKSPACE_POPOUT_OPENED,
	} from './lib/events';
	import { subscribeRepoDiffEvent } from './lib/repoDiffService';
	import { resolveWorkbenchPaneState, type WorkbenchSurface } from './lib/appPaneState';
	import { releaseWorkspaceTerminals } from './lib/terminal/terminalService';
	import { shouldClearPreviousWorkspaceTerminalActivity } from './lib/terminal/terminalActivity';
	import { subscribeTerminalActivity } from './lib/terminal/terminalActivityBus';
	import { subscribeWailsEvent } from './lib/wailsEventRegistry';
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
	import type { DocumentSession, Workspace } from './lib/types';
	import SkillRegistryView from './lib/components/views/SkillRegistryView.svelte';
	import SpacesWorkbenchView from './lib/components/views/SpacesWorkbenchView.svelte';
	import { workspaceActionMutations } from './lib/services/workspaceActionService';
	import {
		loadOnboardingCatalog,
		type RegisteredRepo,
	} from './lib/view-models/onboardingViewModel';
	import {
		buildShortcutMap,
		deriveWorksetIdentity,
		mapWorkspacesToSummaries,
	} from './lib/view-models/worksetViewModel';
	import { createTerminalActivityTracker } from './lib/composables/createTerminalActivityTracker.svelte';
	import { createRepoStatusWatchers } from './lib/composables/createRepoStatusWatchers';
	import { createWorkspaceActionModal } from './lib/composables/createWorkspaceActionModal.svelte';
	import { createPopoutManager } from './lib/composables/createPopoutManager.svelte';
	import { createNotifications } from './lib/composables/createNotifications.svelte';
	import { provideNotifications } from './lib/contexts/notifications';
	import { provideWorkspaceActions } from './lib/contexts/workspaceActions';

	type RepoDiffLocalStatusEvent = {
		workspaceId: string;
		repoId: string;
		status: RepoLocalStatus;
	};

	type RepoDiffSummaryEvent = {
		workspaceId: string;
		repoId: string;
		summary: {
			files: Array<unknown>;
			totalAdded: number;
			totalRemoved: number;
		};
	};

	type WorkspacePopoutEvent = {
		workspaceId: string;
		windowName: string;
		open: boolean;
	};

	const EXPLORER_OPEN_STORAGE_KEY = 'workset:app:explorerOpen';
	const AUTO_GITHUB_AUTH_CHECK = false;
	const TERMINAL_ACTIVITY_TTL_MS = 20_000;

	const readExplorerOpenPreference = (): boolean => {
		if (typeof localStorage === 'undefined') return true;
		const stored = localStorage.getItem(EXPLORER_OPEN_STORAGE_KEY);
		if (stored === null) return true;
		return stored === 'true';
	};

	const contextViews: AppView[] = ['workspaces', 'skill-registry', 'settings'];
	const popoutViews = new Set<AppView>(['workspaces']);
	const appViews = new Set<AppView>(['workspaces', 'skill-registry', 'settings', 'onboarding']);
	const searchParams =
		typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
	const popoutMode = searchParams?.get('popout') === '1';
	const requestedWorkspace = searchParams?.get('workspace')?.trim() ?? '';
	const requestedView = searchParams?.get('view')?.trim() ?? '';
	const normalizeAppView = (value: string): AppView | null => {
		const trimmed = value.trim();
		if (trimmed === 'terminal-cockpit') return 'workspaces';
		return appViews.has(trimmed as AppView) ? (trimmed as AppView) : null;
	};
	const requestedAppView = normalizeAppView(requestedView);
	const fixedWorkspaceId = popoutMode && requestedWorkspace !== '' ? requestedWorkspace : null;
	const initialView: AppView = popoutMode
		? requestedAppView && popoutViews.has(requestedAppView)
			? requestedAppView
			: 'workspaces'
		: (requestedAppView ?? 'workspaces');

	const repoStatusWatchers = createRepoStatusWatchers();
	const terminalActivity = createTerminalActivityTracker(TERMINAL_ACTIVITY_TTL_MS);

	const hasWorkspace = $derived($activeWorkspace !== null);
	const hasRepo = $derived($activeRepo !== null);
	const hasWorkspaces = $derived($workspaces.length > 0);

	let currentView = $state<AppView>(initialView);
	const workspaceAction = createWorkspaceActionModal();
	const notifications = createNotifications();
	provideNotifications(notifications);

	let commandPaletteOpen = $state(false);
	let authModalOpen = $state(false);
	let authModalDismissed = $state(false);
	let explorerOpen = $state(readExplorerOpenPreference());
	let popoutSelectionApplied = $state(false);
	let onboardingLoading = $state(false);
	let onboardingBusy = $state(false);
	let onboardingError = $state<string | null>(null);
	let onboardingRepoRegistry = $state<RegisteredRepo[]>([]);
	let onboardingLoaded = $state(false);
	let workbenchSurface = $state<WorkbenchSurface>('terminal');
	// eslint-disable-next-line svelte/prefer-writable-derived
	let documentSession = $state<DocumentSession | null>(null);

	const getWorksetThreads = (workspaceId: string): Workspace[] => {
		const target = $workspaces.find(
			(workspace) =>
				workspace.id === workspaceId && !workspace.archived && workspace.placeholder !== true,
		);
		if (!target) return [];
		const identity = deriveWorksetIdentity(target);
		return $workspaces
			.filter(
				(workspace) =>
					!workspace.archived &&
					workspace.placeholder !== true &&
					deriveWorksetIdentity(workspace).id === identity.id,
			)
			.sort((left, right) => left.id.localeCompare(right.id));
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

	const popoutManager = createPopoutManager({
		popoutMode,
		getWorksetThreads,
		releaseWorksetTerminals,
	});

	const visibleWorkspaces = $derived.by(() => {
		if (!fixedWorkspaceId) return $workspaces;
		const threads = getWorksetThreads(fixedWorkspaceId);
		if (threads.length > 0) return threads;
		return $workspaces.filter((workspace) => workspace.id === fixedWorkspaceId);
	});
	const threadVisibleWorkspaces = $derived.by(() =>
		visibleWorkspaces.filter((workspace) => workspace.placeholder !== true),
	);
	const worksetSummaries = $derived.by(() => mapWorkspacesToSummaries(threadVisibleWorkspaces));
	const existingWorksetNames = $derived.by(() => {
		const names = new Set<string>();
		for (const workspace of $workspaces) {
			const label =
				workspace.worksetLabel?.trim() || workspace.workset?.trim() || workspace.name.trim();
			if (label.length > 0) {
				names.add(label);
			}
		}
		return Array.from(names);
	});
	const shortcutMap = $derived.by(() => buildShortcutMap(threadVisibleWorkspaces));
	const activeSummary = $derived.by(
		() => worksetSummaries.find((summary) => summary.id === $activeWorkspaceId) ?? null,
	);
	const activeShortcut = $derived.by(() =>
		$activeWorkspaceId ? shortcutMap.get($activeWorkspaceId) : undefined,
	);
	const showContextBar = $derived.by(
		() => !hasRepo && hasWorkspaces && contextViews.includes(currentView),
	);
	const explorerViews = new Set<AppView>(['workspaces', 'skill-registry', 'settings']);
	const showExplorer = $derived.by(() => hasWorkspaces && explorerViews.has(currentView));

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

	const handleAuthClose = (): void => void ((authModalOpen = false), (authModalDismissed = true));

	const handleAuthSuccess = (): void => void ((authModalOpen = false), (authModalDismissed = true));

	const setView = (view: AppView): void => {
		if (popoutMode && !popoutViews.has(view)) return;
		currentView = view;
		if (hasRepo) clearRepo();
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
		const workspace = visibleWorkspaces.find((entry) => entry.id === workspaceId);
		if (workspace?.placeholder) return;
		if (fixedWorkspaceId && !visibleWorkspaces.some((workspace) => workspace.id === workspaceId)) {
			return;
		}
		const previousWorkspaceId = $activeWorkspaceId;
		selectWorkspace(workspaceId);
		if (
			shouldClearPreviousWorkspaceTerminalActivity({
				previousWorkspaceId,
				nextWorkspaceId: workspaceId,
				previousWorkspacePoppedOut: popoutManager.isWorkspacePoppedOut(previousWorkspaceId),
			})
		) {
			terminalActivity.clear(previousWorkspaceId);
		}
		if (currentView === 'onboarding') {
			currentView = 'workspaces';
		}
	};

	const handleSelectWorkspaceFromPalette = (workspaceId: string): void => {
		const workspace = threadVisibleWorkspaces.find((entry) => entry.id === workspaceId);
		if (!workspace) return;
		if (fixedWorkspaceId && !visibleWorkspaces.some((workspace) => workspace.id === workspaceId)) {
			return;
		}
		const previousWorkspaceId = $activeWorkspaceId;
		selectWorkspace(workspaceId);
		if (
			shouldClearPreviousWorkspaceTerminalActivity({
				previousWorkspaceId,
				nextWorkspaceId: workspaceId,
				previousWorkspacePoppedOut: popoutManager.isWorkspacePoppedOut(previousWorkspaceId),
			})
		) {
			terminalActivity.clear(previousWorkspaceId);
		}
		currentView = 'workspaces';
		clearRepo();
	};

	const handleCreateWorkspace = (): void => {
		if (popoutMode) return;
		setView('onboarding');
		clearRepo();
	};

	const handleCreateThread = (worksetId: string): void => {
		if (popoutMode) {
			return;
		}

		const threads = visibleWorkspaces.filter((workspace) => {
			if (workspace.placeholder) return false;
			const identity = deriveWorksetIdentity(workspace);
			return identity.id === worksetId;
		});
		const placeholder = visibleWorkspaces.find((workspace) => {
			if (!workspace.placeholder) return false;
			const identity = deriveWorksetIdentity(workspace);
			return identity.id === worksetId;
		});
		if (threads.length === 0 && !placeholder) {
			return;
		}

		const first = threads[0] ?? placeholder ?? null;
		if (!first) return;
		const label = deriveWorksetIdentity(first).label;
		const repos = Array.from(
			new Set(
				threads.length > 0
					? threads.flatMap((workspace) => workspace.repos.map((repo) => repo.name))
					: (placeholder?.repos ?? []).map((repo) => repo.name),
			),
		).sort((left, right) => left.localeCompare(right));

		workspaceAction.open('create-thread', null, null, {
			worksetName: label,
			worksetRepos: repos,
		});
	};

	const handleAddRepoToWorkset = (worksetId: string): void => {
		if (popoutMode) return;
		const threads = visibleWorkspaces.filter((workspace) => {
			if (workspace.placeholder) return false;
			const identity = deriveWorksetIdentity(workspace);
			return identity.id === worksetId;
		});
		const placeholder = visibleWorkspaces.find((workspace) => {
			if (!workspace.placeholder) return false;
			const identity = deriveWorksetIdentity(workspace);
			return identity.id === worksetId;
		});
		const first = threads[0] ?? placeholder ?? null;
		if (!first) return;
		const label = deriveWorksetIdentity(first).label;
		workspaceAction.open('add-repo', first.id, null, {
			worksetName: label,
			workspaceIds: threads.map((thread) => thread.id),
		});
	};

	const handleRemoveThread = (threadId: string): void => {
		if (popoutMode) return;
		if (!threadId) return;
		if (fixedWorkspaceId && threadId !== fixedWorkspaceId) return;
		workspaceAction.open('remove-thread', threadId);
	};

	provideWorkspaceActions({
		createWorkspace: handleCreateWorkspace,
		createThread: handleCreateThread,
		addRepo: handleAddRepoToWorkset,
		removeThread: handleRemoveThread,
	});

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
				error instanceof Error ? error.message : 'Failed to create thread from onboarding.';
			throw error;
		} finally {
			onboardingBusy = false;
		}
	};

	const handleOnboardingComplete = async (workspaceName: string): Promise<void> => {
		await loadWorkspaces(true);
		selectWorkspace(workspaceName);
		clearRepo();
		currentView = 'workspaces';
	};

	const handleOnboardingPreviewHooks = async (source: string): Promise<string[]> =>
		previewRepoHooks(source);

	const handleShortcutSwitch = (index: number): void => {
		if (popoutMode) return;
		for (const [workspaceId, number] of shortcutMap.entries()) {
			if (number !== index) continue;
			handleSelectWorkspace(workspaceId);
			return;
		}
	};

	const handleGlobalKeydown = (event: KeyboardEvent): void => {
		if (!(event.metaKey || event.ctrlKey)) return;
		const key = event.key.toLowerCase();
		if (key === 'p' && $activeWorkspaceId) {
			event.preventDefault();
			commandPaletteOpen = false;
			const nextPane = resolveWorkbenchPaneState({
				surface: workbenchSurface,
				filesOpen: documentSession !== null,
				intent: 'files',
			});
			workbenchSurface = nextPane.surface;
			documentSession = nextPane.filesOpen
				? openDefaultDocumentSession($activeWorkspaceId, threadVisibleWorkspaces)
				: null;
			return;
		}
		if (popoutMode) return;
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

	const closeDocument = (): void => void (documentSession = null);

	const handleOpenFiles = (): void => {
		if (!$activeWorkspaceId) return;
		if (currentView !== 'workspaces') {
			setView('workspaces');
		}
		commandPaletteOpen = false;
		const nextPane = resolveWorkbenchPaneState({
			surface: workbenchSurface,
			filesOpen: documentSession !== null,
			intent: 'files',
		});
		workbenchSurface = nextPane.surface;
		documentSession = nextPane.filesOpen
			? openDefaultDocumentSession($activeWorkspaceId, threadVisibleWorkspaces)
			: null;
	};

	let repoSummaryUnsubscribe: (() => void) | null = null,
		repoLocalSummaryUnsubscribe: (() => void) | null = null,
		repoStatusUnsubscribe: (() => void) | null = null,
		popoutOpenedUnsubscribe: (() => void) | null = null,
		popoutClosedUnsubscribe: (() => void) | null = null,
		terminalActivityUnsubscribe: (() => void) | null = null;

	onMount(() => {
		void loadWorkspaces(true);
		void popoutManager.loadState();
		if (!popoutMode && AUTO_GITHUB_AUTH_CHECK) {
			void checkGitHubAuth();
		}
		repoStatusUnsubscribe = subscribeRepoDiffEvent<RepoDiffLocalStatusEvent>(
			EVENT_REPO_DIFF_LOCAL_STATUS,
			(payload) => {
				applyRepoLocalStatus(payload.workspaceId, payload.repoId, payload.status);
			},
		);
		repoSummaryUnsubscribe = subscribeRepoDiffEvent<RepoDiffSummaryEvent>(
			EVENT_REPO_DIFF_SUMMARY,
			(payload) => {
				applyRepoDiffSummary(payload.workspaceId, payload.repoId, payload.summary);
			},
		);
		repoLocalSummaryUnsubscribe = subscribeRepoDiffEvent<RepoDiffSummaryEvent>(
			EVENT_REPO_DIFF_LOCAL_SUMMARY,
			(payload) => {
				applyRepoDiffSummary(payload.workspaceId, payload.repoId, payload.summary);
			},
		);
		popoutOpenedUnsubscribe = subscribeWailsEvent<WorkspacePopoutEvent>(
			EVENT_WORKSPACE_POPOUT_OPENED,
			(payload) => {
				popoutManager.updateState(payload.workspaceId, payload.windowName, true);
			},
		);
		popoutClosedUnsubscribe = subscribeWailsEvent<WorkspacePopoutEvent>(
			EVENT_WORKSPACE_POPOUT_CLOSED,
			(payload) => {
				popoutManager.updateState(payload.workspaceId, payload.windowName, false);
			},
		);
		terminalActivityUnsubscribe = subscribeTerminalActivity((payload) => {
			terminalActivity.mark(payload.workspaceId);
		});
	});

	onDestroy(() => {
		repoStatusWatchers.stopAll();
		repoSummaryUnsubscribe?.();
		repoSummaryUnsubscribe = null;
		repoLocalSummaryUnsubscribe?.();
		repoLocalSummaryUnsubscribe = null;
		repoStatusUnsubscribe?.();
		repoStatusUnsubscribe = null;
		popoutOpenedUnsubscribe?.();
		popoutOpenedUnsubscribe = null;
		popoutClosedUnsubscribe?.();
		popoutClosedUnsubscribe = null;
		terminalActivityUnsubscribe?.();
		terminalActivityUnsubscribe = null;
		terminalActivity.destroy();
		notifications.destroy();
	});

	$effect(() => {
		if (!popoutMode) repoStatusWatchers.sync($workspaces);
	});

	$effect(() => {
		if (typeof localStorage === 'undefined') return;
		localStorage.setItem(EXPLORER_OPEN_STORAGE_KEY, String(explorerOpen));
	});

	$effect(() => {
		documentSession = reconcileDocumentSession(documentSession, $activeWorkspaceId, $workspaces);
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
			currentView = 'workspaces';
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
				workspacePoppedOut={popoutManager.isWorkspacePoppedOut($activeWorkspaceId)}
				onTogglePopout={() => {
					const workspaceId = $activeWorkspaceId;
					if (!workspaceId) return;
					if (popoutManager.isWorkspacePoppedOut(workspaceId)) {
						void popoutManager.handlePopout(workspaceId, false);
						return;
					}
					void popoutManager.handlePopout(workspaceId, true);
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
						activeView={currentView === 'skill-registry' ? 'skill-registry' : 'workspaces'}
						activeSurface={workbenchSurface}
						filesActive={documentSession !== null}
						activeTerminalWorkspaceIds={terminalActivity.activeIds}
						onSelectWorkspace={handleSelectWorkspace}
						onOpenPullRequests={() => {
							const nextPane = resolveWorkbenchPaneState({
								surface: workbenchSurface,
								filesOpen: documentSession !== null,
								intent: 'pull-requests',
							});
							workbenchSurface = nextPane.surface;
							documentSession = nextPane.filesOpen ? documentSession : null;
							setView('workspaces');
						}}
						onOpenFiles={handleOpenFiles}
						onOpenSkills={() =>
							setView(currentView === 'skill-registry' ? 'workspaces' : 'skill-registry')}
						onOpenSettings={() => setView(currentView === 'settings' ? 'workspaces' : 'settings')}
						onCollapse={() => (explorerOpen = false)}
					/>
				</aside>
			{/if}
			{#if showExplorer && !explorerOpen}
				<button
					type="button"
					class="ws-panel-edge-tab ws-panel-edge-tab--left ws-panel-edge-tab--absolute"
					aria-label="Open Explorer (⌘B)"
					title="Open Explorer (⌘B)"
					onclick={() => (explorerOpen = true)}
				>
				</button>
			{/if}

			<div class="view-shell">
				{#key currentView}
					<div class="view-transition" in:fly={{ y: 10, duration: 200 }}>
						{#if $loadingWorkspaces}
							<EmptyState
								title="Loading workspaces"
								body="Fetching thread snapshots and local status."
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
								title="Create your first thread"
								body="Threads are collections of repositories that move together across branches and PR flow."
								actionLabel="Create thread"
								onAction={handleCreateWorkspace}
								variant="centered"
							/>
						{:else if currentView === 'workspaces'}
							{#if !popoutMode && popoutManager.isWorkspacePoppedOut($activeWorkspaceId)}
								<EmptyState
									title="This workset is open in a popout"
									body="Use the popout window to continue. Return it here anytime."
									actionLabel="Focus Popout"
									onAction={() => void popoutManager.handlePopout($activeWorkspaceId ?? '', true)}
									secondaryActionLabel="Return To Main Window"
									onSecondaryAction={() =>
										void popoutManager.handlePopout($activeWorkspaceId ?? '', false)}
									variant="centered"
								/>
							{:else}
								<SpacesWorkbenchView
									workspaces={visibleWorkspaces}
									activeWorkspaceId={$activeWorkspaceId}
									{popoutMode}
									useGlobalExplorer={showExplorer}
									preferredSurface={workbenchSurface}
									{documentSession}
									onSurfaceChange={(surface) => (workbenchSurface = surface)}
									onSelectWorkspace={handleSelectWorkspace}
									onCreateWorkspace={handleCreateWorkspace}
									onCreateThread={handleCreateThread}
									onCloseDocument={closeDocument}
								/>
							{/if}
						{:else if currentView === 'skill-registry'}
							<SkillRegistryView
								workspaceId={$activeWorkspaceId}
								onClose={() => setView('workspaces')}
							/>
						{:else if currentView === 'settings'}
							<SettingsPanel onClose={() => setView('workspaces')} />
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
								onCancel={() => setView('workspaces')}
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

	{#if !popoutMode && workspaceAction.mode}
		<div
			class="action-panel-backdrop"
			role="button"
			tabindex="0"
			onclick={workspaceAction.close}
			onkeydown={(event) => {
				if (event.key === 'Escape') workspaceAction.close();
			}}
		></div>
		<aside
			class="action-panel"
			role="presentation"
			onclick={(event) => event.stopPropagation()}
			onkeydown={(event) => event.stopPropagation()}
		>
			<WorkspaceActionModal
				onClose={workspaceAction.close}
				mode={workspaceAction.mode}
				workspaceId={workspaceAction.workspaceId}
				workspaceIds={workspaceAction.workspaceIds}
				repoName={workspaceAction.repoName}
				worksetName={workspaceAction.worksetName}
				worksetRepos={workspaceAction.worksetRepos}
			/>
		</aside>
	{/if}

	{#if !popoutMode && authModalOpen}
		<div
			class="overlay"
			role="dialog"
			aria-modal="true"
			aria-label="GitHub authentication"
			tabindex="-1"
			onclick={handleAuthClose}
			onkeydown={(event) => {
				if (event.key === 'Escape') handleAuthClose();
				if (event.key === 'Tab') {
					// Trap focus inside the dialog
					const focusable = event.currentTarget.querySelectorAll<HTMLElement>(
						'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
					);
					if (focusable.length === 0) return;
					const first = focusable[0];
					const last = focusable[focusable.length - 1];
					if (event.shiftKey && document.activeElement === first) {
						event.preventDefault();
						last.focus();
					} else if (!event.shiftKey && document.activeElement === last) {
						event.preventDefault();
						first.focus();
					}
				}
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

	{#if notifications.notifications.length > 0}
		<div class="notification-stack">
			{#each notifications.notifications as notif (notif.id)}
				<button
					type="button"
					class="notification-toast notification-toast--{notif.level}"
					onclick={() => notifications.dismiss(notif.id)}
				>
					{notif.message}
				</button>
			{/each}
		</div>
	{/if}
</div>

<style src="./App.css"></style>
