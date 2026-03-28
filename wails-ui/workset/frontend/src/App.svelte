<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import {
		activeRepo,
		activeWorkspace,
		activeWorkspaceId,
		applyRepoDiffSummary,
		applyRepoLocalStatus,
		applyTrackedPullRequest,
		applyTrackedPullRequestReviewComments,
		clearRepo,
		clearWorkspace,
		loadWorkspaces,
		loadingWorkspaces,
		selectWorkspace,
		workspaceError,
		workspaces,
	} from './lib/state';
	import type { RepoLocalStatus } from './lib/api/github';
	import { fetchGitHubAuthInfo } from './lib/api/github';
	import {
		EVENT_REPO_DIFF_LOCAL_SUMMARY,
		EVENT_REPO_DIFF_LOCAL_STATUS,
		EVENT_REPO_DIFF_PR_REVIEWS,
		EVENT_REPO_DIFF_PR_STATUS,
		EVENT_REPO_DIFF_SUMMARY,
		EVENT_WORKSPACE_POPOUT_CLOSED,
		EVENT_WORKSPACE_POPOUT_OPENED,
	} from './lib/events';
	import { subscribeRepoDiffEvent } from './lib/repoDiffService';
	import { releaseWorkspaceTerminals } from './lib/terminal/terminalService';
	import { shouldClearPreviousWorkspaceTerminalActivity } from './lib/terminal/terminalActivity';
	import { subscribeTerminalActivity } from './lib/terminal/terminalActivityBus';
	import { subscribeWailsEvent } from './lib/wailsEventRegistry';
	import EmptyState from './lib/components/EmptyState.svelte';
	import GitHubLoginModal from './lib/components/GitHubLoginModal.svelte';
	import SettingsPanel from './lib/components/SettingsPanel.svelte';
	import KeyboardShortcutsPanel from './lib/components/KeyboardShortcutsPanel.svelte';
	import UpdateNotificationCard from './lib/components/UpdateNotificationCard.svelte';
	import WorkspaceActionModal from './lib/components/WorkspaceActionModal.svelte';
	import CommandPalette, { type AppView } from './lib/components/chrome/CommandPalette.svelte';
	import FileSearchPalette from './lib/components/chrome/FileSearchPalette.svelte';
	import ContextBar from './lib/components/chrome/ContextBar.svelte';
	import ExplorerPanel from './lib/components/chrome/ExplorerPanel.svelte';
	import {
		mapPullRequestReviews,
		mapPullRequestStatus,
		type RepoDiffPrReviewsEvent,
		type RepoDiffPrStatusEvent,
	} from './lib/components/repo-diff/prStatusController';
	import type { RepoDiffSummary, UpdatePreferences, Workspace } from './lib/types';
	import SkillRegistryView from './lib/components/views/SkillRegistryView.svelte';
	import SpacesWorkbenchView from './lib/components/views/SpacesWorkbenchView.svelte';
	import {
		buildShortcutMap,
		deriveWorksetIdentity,
		mapWorkspacesToExplorerWorksets,
		mapWorkspacesToSummaries,
		mapWorkspacesToThreadGroups,
		mapWorkspacesToThreadShellSummaries,
	} from './lib/view-models/worksetViewModel';
	import {
		deriveHotWorksetIds,
		deriveWatchedWorkspaces,
		rememberWorksetId,
		resolveWorksetIdForWorkspace,
	} from './lib/view-models/repoWatchScope';
	import { createTerminalActivityTracker } from './lib/composables/createTerminalActivityTracker.svelte';
	import { createRepoStatusWatchers } from './lib/composables/createRepoStatusWatchers';
	import { createWorkspaceActionModal } from './lib/composables/createWorkspaceActionModal.svelte';
	import { createPopoutManager } from './lib/composables/createPopoutManager.svelte';
	import { createNotifications } from './lib/composables/createNotifications.svelte';
	import { createUpdateNotificationController } from './lib/composables/createUpdateNotificationController.svelte';
	import { provideNotifications } from './lib/contexts/notifications';
	import { provideWorkspaceActions } from './lib/contexts/workspaceActions';
	import {
		UPDATE_PREFERENCES_CHANGED_EVENT,
		type UpdatePreferencesChangedDetail,
	} from './lib/updatePreferences';

	type RepoDiffLocalStatusEvent = {
		workspaceId: string;
		repoId: string;
		status: RepoLocalStatus;
	};

	type RepoDiffSummaryEvent = {
		workspaceId: string;
		repoId: string;
		summary: RepoDiffSummary;
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
	const appViews = new Set<AppView>(['workspaces', 'skill-registry', 'settings']);
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
	const updateNotification = createUpdateNotificationController();
	provideNotifications(notifications);

	let commandPaletteOpen = $state(false);
	let fileSearchOpen = $state(false);
	let pendingFileSelection = $state<{ repoId: string; path: string } | null>(null);
	let authModalOpen = $state(false);
	let shortcutsOpen = $state(false);
	let authModalDismissed = $state(false);
	let explorerOpen = $state(readExplorerOpenPreference());
	let popoutSelectionApplied = $state(false);
	let workbenchSurface = $state<'terminal' | 'pull-requests'>('terminal');
	let warmWorksetIds = $state<string[]>([]);
	let selectedWorksetId = $state<string | null>(null);

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
	const watchedWorkspaces = $derived.by(() =>
		deriveWatchedWorkspaces({
			workspaces: threadVisibleWorkspaces,
			activeWorkspaceId: $activeWorkspaceId,
			fixedWorkspaceId,
			warmWorksetIds,
		}),
	);
	const hotWorksetIds = $derived.by(() =>
		deriveHotWorksetIds({
			workspaces: threadVisibleWorkspaces,
			activeWorkspaceId: $activeWorkspaceId,
			fixedWorkspaceId,
			warmWorksetIds,
		}),
	);
	const isWorkspaceHot = (workspaceId: string): boolean => {
		const workspace = threadVisibleWorkspaces.find((entry) => entry.id === workspaceId);
		if (!workspace) return false;
		return hotWorksetIds.has(deriveWorksetIdentity(workspace).id);
	};
	const worksetSummaries = $derived.by(() => mapWorkspacesToSummaries(threadVisibleWorkspaces));
	const shortcutMap = $derived.by(() => buildShortcutMap(threadVisibleWorkspaces));
	const threadShellSummaries = $derived.by(() =>
		mapWorkspacesToThreadShellSummaries(threadVisibleWorkspaces),
	);
	const threadSummaryMap = $derived.by(
		() => new Map(threadShellSummaries.map((summary) => [summary.id, summary])),
	);
	const explorerWorksets = $derived.by(() =>
		mapWorkspacesToExplorerWorksets(visibleWorkspaces, shortcutMap),
	);
	const worksetThreadGroups = $derived.by(() => mapWorkspacesToThreadGroups(visibleWorkspaces));
	const visibleActiveWorkspaceId = $derived.by(() => {
		const workspaceId = $activeWorkspaceId;
		if (!workspaceId) return null;
		return threadVisibleWorkspaces.some((workspace) => workspace.id === workspaceId)
			? workspaceId
			: null;
	});
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
		if (view === 'onboarding') {
			workspaceAction.open('create');
			return;
		}
		if (view === 'keyboard-shortcuts') {
			shortcutsOpen = true;
			return;
		}
		currentView = view;
		if (hasRepo) clearRepo();
	};

	const handleSelectWorkspace = (workspaceId: string): void => {
		const workspace = visibleWorkspaces.find((entry) => entry.id === workspaceId);
		if (!workspace) return;
		if (workspace?.placeholder) return;
		if (fixedWorkspaceId && !visibleWorkspaces.some((workspace) => workspace.id === workspaceId)) {
			return;
		}
		const previousWorkspaceId = $activeWorkspaceId;
		selectedWorksetId = deriveWorksetIdentity(workspace).id;
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
	};

	const handleSelectWorkspaceFromPalette = (workspaceId: string): void => {
		const workspace = threadVisibleWorkspaces.find((entry) => entry.id === workspaceId);
		if (!workspace) return;
		if (fixedWorkspaceId && !visibleWorkspaces.some((workspace) => workspace.id === workspaceId)) {
			return;
		}
		const previousWorkspaceId = $activeWorkspaceId;
		selectedWorksetId = deriveWorksetIdentity(workspace).id;
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

	const handleSelectWorkset = (worksetId: string): void => {
		const selected = worksetThreadGroups.find((group) => group.id === worksetId);
		if (!selected) return;
		selectedWorksetId = selected.id;
		currentView = 'workspaces';
		clearRepo();
		const firstThread = selected.threads[0];
		if (firstThread) {
			handleSelectWorkspace(firstThread.id);
			return;
		}
		clearWorkspace();
	};

	const handleCreateWorkspace = (): void => {
		if (popoutMode) return;
		workspaceAction.open('create');
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
			if (event.shiftKey) {
				// Cmd+Shift+P: command palette (VS Code convention)
				commandPaletteOpen = !commandPaletteOpen;
			} else {
				// Cmd+P: file search
				commandPaletteOpen = false;
				fileSearchOpen = !fileSearchOpen;
			}
			return;
		}
		if (popoutMode) return;
		if (key === 'k' && $activeWorkspaceId) {
			event.preventDefault();
			// Cmd+K: toggle workbench surface
			commandPaletteOpen = false;
			workbenchSurface = workbenchSurface === 'pull-requests' ? 'terminal' : 'pull-requests';
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
			return;
		}
		if (key === '?' || (key === '/' && event.shiftKey)) {
			event.preventDefault();
			shortcutsOpen = !shortcutsOpen;
		}
	};

	const handleOpenFiles = (): void => {
		if (!$activeWorkspaceId) return;
		if (currentView !== 'workspaces') {
			setView('workspaces');
		}
		commandPaletteOpen = false;
		fileSearchOpen = false;
		workbenchSurface = workbenchSurface === 'pull-requests' ? 'terminal' : 'pull-requests';
	};

	const handleFileSearchSelect = (repoId: string, path: string): void => {
		// Ensure code surface is visible
		if (currentView !== 'workspaces') setView('workspaces');
		workbenchSurface = 'pull-requests';
		pendingFileSelection = { repoId, path };
	};

	let repoSummaryUnsubscribe: (() => void) | null = null,
		repoLocalSummaryUnsubscribe: (() => void) | null = null,
		repoStatusUnsubscribe: (() => void) | null = null,
		repoPrStatusUnsubscribe: (() => void) | null = null,
		repoPrReviewsUnsubscribe: (() => void) | null = null,
		popoutOpenedUnsubscribe: (() => void) | null = null,
		popoutClosedUnsubscribe: (() => void) | null = null,
		terminalActivityUnsubscribe: (() => void) | null = null;
	let updatePreferencesListener: ((event: Event) => void) | null = null;

	onMount(() => {
		void loadWorkspaces(true);
		void popoutManager.loadState();
		if (!popoutMode) {
			void updateNotification.init();
			updatePreferencesListener = (event: Event) => {
				const detail = (event as CustomEvent<UpdatePreferencesChangedDetail>).detail;
				const preferences = detail?.preferences as UpdatePreferences | undefined;
				if (!preferences) {
					return;
				}
				void updateNotification.applyPreferences(preferences);
			};
			window.addEventListener(UPDATE_PREFERENCES_CHANGED_EVENT, updatePreferencesListener);
		}
		if (!popoutMode && AUTO_GITHUB_AUTH_CHECK) {
			void checkGitHubAuth();
		}
		repoStatusUnsubscribe = subscribeRepoDiffEvent<RepoDiffLocalStatusEvent>(
			EVENT_REPO_DIFF_LOCAL_STATUS,
			(payload) => {
				if (!isWorkspaceHot(payload.workspaceId)) return;
				applyRepoLocalStatus(payload.workspaceId, payload.repoId, payload.status);
			},
		);
		repoSummaryUnsubscribe = subscribeRepoDiffEvent<RepoDiffSummaryEvent>(
			EVENT_REPO_DIFF_SUMMARY,
			(payload) => {
				if (!isWorkspaceHot(payload.workspaceId)) return;
				applyRepoDiffSummary(payload.workspaceId, payload.repoId, payload.summary);
			},
		);
		repoLocalSummaryUnsubscribe = subscribeRepoDiffEvent<RepoDiffSummaryEvent>(
			EVENT_REPO_DIFF_LOCAL_SUMMARY,
			(payload) => {
				if (!isWorkspaceHot(payload.workspaceId)) return;
				applyRepoDiffSummary(payload.workspaceId, payload.repoId, payload.summary);
			},
		);
		repoPrStatusUnsubscribe = subscribeRepoDiffEvent<RepoDiffPrStatusEvent>(
			EVENT_REPO_DIFF_PR_STATUS,
			(payload) => {
				if (!isWorkspaceHot(payload.workspaceId)) return;
				applyTrackedPullRequest(
					payload.workspaceId,
					payload.repoId,
					mapPullRequestStatus(payload.status).pullRequest,
				);
			},
		);
		repoPrReviewsUnsubscribe = subscribeRepoDiffEvent<RepoDiffPrReviewsEvent>(
			EVENT_REPO_DIFF_PR_REVIEWS,
			(payload) => {
				if (!isWorkspaceHot(payload.workspaceId)) return;
				applyTrackedPullRequestReviewComments(
					payload.workspaceId,
					payload.repoId,
					mapPullRequestReviews(payload.comments),
				);
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
		repoPrStatusUnsubscribe?.();
		repoPrStatusUnsubscribe = null;
		repoPrReviewsUnsubscribe?.();
		repoPrReviewsUnsubscribe = null;
		popoutOpenedUnsubscribe?.();
		popoutOpenedUnsubscribe = null;
		popoutClosedUnsubscribe?.();
		popoutClosedUnsubscribe = null;
		terminalActivityUnsubscribe?.();
		terminalActivityUnsubscribe = null;
		if (updatePreferencesListener) {
			window.removeEventListener(UPDATE_PREFERENCES_CHANGED_EVENT, updatePreferencesListener);
			updatePreferencesListener = null;
		}
		terminalActivity.destroy();
		updateNotification.destroy();
		notifications.destroy();
	});

	$effect(() => {
		if (!popoutMode) repoStatusWatchers.sync(watchedWorkspaces);
	});

	$effect(() => {
		if (selectedWorksetId && !worksetThreadGroups.some((group) => group.id === selectedWorksetId)) {
			selectedWorksetId = null;
		}
	});

	$effect(() => {
		const activeWorksetId = resolveWorksetIdForWorkspace(
			threadVisibleWorkspaces,
			$activeWorkspaceId,
		);
		warmWorksetIds = rememberWorksetId(warmWorksetIds, activeWorksetId);
	});

	$effect(() => {
		if (!$activeWorkspaceId) return;
		const active = threadVisibleWorkspaces.find((workspace) => workspace.id === $activeWorkspaceId);
		if (!active) return;
		selectedWorksetId = deriveWorksetIdentity(active).id;
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
		selectedWorksetId = deriveWorksetIdentity(target).id;
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
						activeWorkspaceId={visibleActiveWorkspaceId}
						groupedWorksets={explorerWorksets}
						{selectedWorksetId}
						lockWorksetSelection={popoutMode}
						canManageRepos={!popoutMode}
						activeView={currentView === 'skill-registry'
							? 'skill-registry'
							: currentView === 'settings'
								? 'settings'
								: 'workspaces'}
						activeSurface={workbenchSurface}
						filesActive={workbenchSurface === 'pull-requests'}
						activeTerminalWorkspaceIds={terminalActivity.activeIds}
						onSelectWorkspace={handleSelectWorkspace}
						onSelectWorkset={handleSelectWorkset}
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
						{:else if !hasWorkspace && !hasWorkspaces}
							<EmptyState
								title="Create your first thread"
								body="Threads are collections of repositories that move together across branches and PR flow."
								actionLabel="Create thread"
								onAction={handleCreateWorkspace}
								variant="centered"
							/>
						{:else if currentView === 'workspaces'}
							{#if !popoutMode && popoutManager.isWorkspacePoppedOut(visibleActiveWorkspaceId)}
								<EmptyState
									title="This workset is open in a popout"
									body="Use the popout window to continue. Return it here anytime."
									actionLabel="Focus Popout"
									onAction={() =>
										void popoutManager.handlePopout(visibleActiveWorkspaceId ?? '', true)}
									secondaryActionLabel="Return To Main Window"
									onSecondaryAction={() =>
										void popoutManager.handlePopout(visibleActiveWorkspaceId ?? '', false)}
									variant="centered"
								/>
							{:else}
								<SpacesWorkbenchView
									activeWorkspaceId={visibleActiveWorkspaceId}
									worksetGroups={worksetThreadGroups}
									{selectedWorksetId}
									{threadSummaryMap}
									{popoutMode}
									useGlobalExplorer={showExplorer}
									preferredSurface={workbenchSurface}
									{pendingFileSelection}
									onSurfaceChange={(surface) => (workbenchSurface = surface)}
									onSelectWorkspace={handleSelectWorkspace}
									onSelectWorkset={handleSelectWorkset}
									onCreateWorkspace={handleCreateWorkspace}
									onCreateThread={handleCreateThread}
									onAddRepo={handleAddRepoToWorkset}
									onFileSelectionHandled={() => (pendingFileSelection = null)}
								/>
							{/if}
						{:else if currentView === 'skill-registry'}
							<SkillRegistryView
								workspaceId={$activeWorkspaceId}
								onClose={() => setView('workspaces')}
							/>
						{:else if currentView === 'settings'}
							<SettingsPanel onClose={() => setView('workspaces')} />
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

	{#if $activeWorkspaceId}
		<FileSearchPalette
			open={fileSearchOpen}
			workspaceId={$activeWorkspaceId}
			onClose={() => (fileSearchOpen = false)}
			onSelectFile={handleFileSearchSelect}
		/>
	{/if}

	{#if shortcutsOpen}
		<KeyboardShortcutsPanel onClose={() => (shortcutsOpen = false)} />
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
				selectWorkset={handleSelectWorkset}
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

	{#if updateNotification.card || notifications.notifications.length > 0}
		<div class="notification-stack">
			{#if updateNotification.card}
				<UpdateNotificationCard
					notification={updateNotification.card}
					busy={updateNotification.busy}
					onDismiss={() => void updateNotification.dismiss()}
					onUpdate={() => void updateNotification.startUpdate()}
				/>
			{/if}
			{#each notifications.notifications as notif (notif.id)}
				<div
					class="notification-toast notification-toast--{notif.level}"
					role="button"
					tabindex="0"
					onclick={() => notifications.dismiss(notif.id)}
					onkeydown={(event) => {
						if (event.key === 'Enter' || event.key === ' ') {
							event.preventDefault();
							notifications.dismiss(notif.id);
						}
					}}
				>
					<span>{notif.message}</span>
					{#if notif.actionLabel && notif.onAction}
						<button
							type="button"
							class="notification-toast__action"
							onclick={(event) => {
								event.stopPropagation();
								void notif.onAction?.();
								notifications.dismiss(notif.id);
							}}
						>
							{notif.actionLabel}
						</button>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<style src="./App.css"></style>
