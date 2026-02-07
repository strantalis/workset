<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import {
		activeRepo,
		activeRepoId,
		activeWorkspace,
		activeWorkspaceId,
		applyRepoLocalStatus,
		clearRepo,
		loadWorkspaces,
		loadingWorkspaces,
		workspaceError,
		workspaces,
	} from './lib/state';
	import EmptyState from './lib/components/EmptyState.svelte';
	import GitHubLoginModal from './lib/components/GitHubLoginModal.svelte';
	import RepoDiff from './lib/components/RepoDiff.svelte';
	import SettingsPanel from './lib/components/SettingsPanel.svelte';
	import TerminalWorkspace from './lib/components/TerminalWorkspace.svelte';
	import WorkspaceActionModal from './lib/components/WorkspaceActionModal.svelte';
	import WorkspaceTree from './lib/components/WorkspaceTree.svelte';
	import type { Workspace } from './lib/types';
	import type { RepoLocalStatus } from './lib/api';
	import { fetchGitHubAuthInfo, startRepoStatusWatch, stopRepoStatusWatch } from './lib/api';
	import { subscribeRepoDiffEvent } from './lib/repoDiffService';
	import { EVENT_REPO_DIFF_LOCAL_STATUS } from './lib/events';

	// Sidebar resize constraints
	const MIN_SIDEBAR_WIDTH = 200;
	const MAX_SIDEBAR_WIDTH = 480;
	const DEFAULT_SIDEBAR_WIDTH = 280;

	const hasWorkspace = $derived($activeWorkspace !== null);
	const hasRepo = $derived($activeRepo !== null);
	const hasWorkspaces = $derived($workspaces.length > 0);
	let settingsOpen = $state(false);
	let sidebarCollapsed = $state(false);
	let sidebarWidth = $state(DEFAULT_SIDEBAR_WIDTH);
	let isResizingSidebar = $state(false);
	let actionOpen = $state(false);
	let authModalOpen = $state(false);
	let authModalDismissed = $state(false);
	const repoStatusWatchers = new Map<string, { workspaceId: string; repoId: string }>();

	type RepoDiffLocalStatusEvent = {
		workspaceId: string;
		repoId: string;
		status: RepoLocalStatus;
	};

	let actionContext: {
		mode: 'create' | 'rename' | 'add-repo' | 'archive' | 'remove-workspace' | 'remove-repo' | null;
		workspaceId: string | null;
		repoName: string | null;
	} = $state({
		mode: null,
		workspaceId: null,
		repoName: null,
	});

	const openAction = (
		mode: 'create' | 'rename' | 'add-repo' | 'archive' | 'remove-workspace' | 'remove-repo',
		workspaceId: string | null,
		repoName: string | null,
	): void => {
		actionContext = { mode, workspaceId, repoName };
		actionOpen = true;
	};

	const checkGitHubAuth = async (): Promise<void> => {
		if (authModalDismissed) return;
		try {
			const info = await fetchGitHubAuthInfo();
			if (!info.status.authenticated) {
				authModalOpen = true;
			}
		} catch (error) {
			// eslint-disable-next-line no-console
			console.warn('Unable to check GitHub auth status', error);
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

	// Sidebar resize handlers
	const handleSidebarResizeStart = (event: PointerEvent): void => {
		if (sidebarCollapsed) return;
		event.preventDefault();
		isResizingSidebar = true;
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
	};

	const handleSidebarResizeMove = (event: PointerEvent): void => {
		if (!isResizingSidebar) return;
		const newWidth = Math.min(MAX_SIDEBAR_WIDTH, Math.max(MIN_SIDEBAR_WIDTH, event.clientX));
		sidebarWidth = newWidth;
	};

	const handleSidebarResizeEnd = (event: PointerEvent): void => {
		if (!isResizingSidebar) return;
		isResizingSidebar = false;
		(event.currentTarget as HTMLElement).releasePointerCapture(event.pointerId);
	};

	const updateRepoStatusWatchers = (data: Workspace[]): void => {
		const nextKeys = new Set<string>();
		for (const workspace of data) {
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

		for (const [key, entry] of repoStatusWatchers) {
			if (nextKeys.has(key)) continue;
			repoStatusWatchers.delete(key);
			void stopRepoStatusWatch(entry.workspaceId, entry.repoId).catch(() => {});
		}
	};

	const stopAllRepoStatusWatchers = (): void => {
		for (const entry of repoStatusWatchers.values()) {
			void stopRepoStatusWatch(entry.workspaceId, entry.repoId).catch(() => {});
		}
		repoStatusWatchers.clear();
	};

	const onUnmount = (): void => {
		stopAllRepoStatusWatchers();
		repoStatusUnsubscribe?.();
		repoStatusUnsubscribe = null;
	};

	let repoStatusUnsubscribe: (() => void) | null = null;

	onMount(() => {
		void loadWorkspaces();
		void checkGitHubAuth();
		repoStatusUnsubscribe = subscribeRepoDiffEvent<RepoDiffLocalStatusEvent>(
			EVENT_REPO_DIFF_LOCAL_STATUS,
			(payload) => {
				applyRepoLocalStatus(payload.workspaceId, payload.repoId, payload.status);
			},
		);
	});

	onDestroy(() => {
		onUnmount();
	});

	$effect(() => {
		updateRepoStatusWatchers($workspaces);
	});
</script>

<div
	class:collapsed={sidebarCollapsed}
	class:resizing={isResizingSidebar}
	class="app"
	style={sidebarCollapsed ? '' : `--sidebar-width: ${sidebarWidth}px`}
>
	<aside class:collapsed={sidebarCollapsed} class:repo-view={hasRepo} class="sidebar">
		{#if !sidebarCollapsed}
			<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
			<div
				class="sidebar-resize-handle"
				class:active={isResizingSidebar}
				role="separator"
				tabindex="0"
				aria-orientation="vertical"
				aria-valuenow={sidebarWidth}
				aria-valuemin={MIN_SIDEBAR_WIDTH}
				aria-valuemax={MAX_SIDEBAR_WIDTH}
				onpointerdown={handleSidebarResizeStart}
				onpointermove={handleSidebarResizeMove}
				onpointerup={handleSidebarResizeEnd}
				onpointercancel={handleSidebarResizeEnd}
			></div>
		{/if}
		<WorkspaceTree
			{sidebarCollapsed}
			onToggleSidebar={() => (sidebarCollapsed = !sidebarCollapsed)}
			onCreateWorkspace={() => openAction('create', null, null)}
			onAddRepo={(workspaceId) => openAction('add-repo', workspaceId, null)}
			onManageWorkspace={(workspaceId, action) => {
				if (action === 'rename') {
					openAction('rename', workspaceId, null);
				} else if (action === 'archive') {
					openAction('archive', workspaceId, null);
				} else {
					openAction('remove-workspace', workspaceId, null);
				}
			}}
			onManageRepo={(workspaceId, repoName, _action) => {
				openAction('remove-repo', workspaceId, repoName);
			}}
		/>
	</aside>

	<div class="main-area">
		<header class:repo-view={hasRepo} class:no-workspace={!hasWorkspace} class="topbar">
			<button
				class="icon-button settings-btn"
				type="button"
				onclick={() => (settingsOpen = true)}
				aria-label="Settings"
			>
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<circle cx="12" cy="12" r="3" />
					<path
						d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"
					/>
				</svg>
			</button>
		</header>

		<main class="main">
			{#if $loadingWorkspaces}
				<EmptyState title="Loading workspaces" body="Fetching your workspace list." />
			{:else if $workspaceError}
				<section class="error">
					<div class="title">Failed to load workspaces</div>
					<div class="body">{$workspaceError}</div>
					<button class="retry" onclick={() => loadWorkspaces()} type="button">Retry</button>
				</section>
			{:else if !hasWorkspace}
				<EmptyState
					title={hasWorkspaces ? 'Select a workspace' : 'Create your first workspace'}
					body={hasWorkspaces
						? 'Choose a workspace from the sidebar, or create a new one with the repos you need.'
						: 'Workspaces are collections of Git repositories you work on together. Add repos by URL or local path, use registered repos, or apply entire team bundles (groups).'}
					actionLabel="Create workspace with repos"
					onAction={() => openAction('create', null, null)}
					hint={hasWorkspaces ? undefined : 'Add by URL · Use registered repos · Apply groups'}
					variant="centered"
				/>
			{:else}
				<div class="view-stack">
					<div class="view-pane" class:active={!hasRepo} aria-hidden={hasRepo}>
						{#key $activeWorkspaceId}
							<TerminalWorkspace
								workspaceId={$activeWorkspace?.id ?? ''}
								workspaceName={$activeWorkspace?.name ?? 'Workspace'}
								active={!hasRepo}
							/>
						{/key}
					</div>
					{#if hasRepo}
						<div class="view-pane active" aria-hidden={!hasRepo}>
							{#key $activeRepoId}
								<RepoDiff
									repo={$activeRepo!}
									workspaceId={$activeWorkspaceId ?? ''}
									onClose={clearRepo}
								/>
							{/key}
						</div>
					{/if}
				</div>
			{/if}
		</main>
	</div>

	{#if settingsOpen}
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

	{#if actionOpen}
		<div
			class="overlay"
			role="button"
			tabindex="0"
			onclick={() => (actionOpen = false)}
			onkeydown={(event) => {
				if (event.key === 'Escape') actionOpen = false;
			}}
		>
			<div
				class="overlay-panel"
				role="presentation"
				onclick={(event) => event.stopPropagation()}
				onkeydown={(event) => event.stopPropagation()}
			>
				<WorkspaceActionModal
					onClose={() => (actionOpen = false)}
					mode={actionContext.mode}
					workspaceId={actionContext.workspaceId}
					repoName={actionContext.repoName}
				/>
			</div>
		</div>
	{/if}

	{#if authModalOpen}
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
	.app {
		height: 100vh;
		display: grid;
		grid-template-columns: var(--sidebar-width, 280px) 1fr;
		transition: grid-template-columns 160ms ease;
		overflow: hidden;
	}

	.app.collapsed {
		grid-template-columns: 72px 1fr;
	}

	.app.resizing {
		transition: none;
		user-select: none;
	}

	.topbar {
		display: flex;
		justify-content: flex-end;
		align-items: center;
		padding: 4px 12px;
		background: color-mix(in srgb, var(--panel-strong) 80%, var(--panel));
		--wails-draggable: drag;
	}

	.topbar.no-workspace,
	.topbar.repo-view {
		background: var(--bg);
	}

	.icon-button {
		width: 28px;
		height: 28px;
		border-radius: var(--radius-sm);
		border: none;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		display: grid;
		place-items: center;
		transition:
			color var(--transition-fast),
			background var(--transition-fast);
		--wails-draggable: no-drag;
	}

	.icon-button:hover {
		color: var(--text);
		background: rgba(255, 255, 255, 0.06);
	}

	.icon-button svg {
		width: 16px;
		height: 16px;
		stroke: currentColor;
		stroke-width: 1.6;
		fill: none;
	}

	.sidebar {
		border-right: 1px solid rgba(255, 255, 255, 0.06);
		padding: 20px 12px;
		padding-top: 36px; /* Space for traffic lights */
		background: var(--panel);
		transition: padding 160ms ease;
		overflow-y: auto;
		overflow-x: hidden;
		min-height: 0;
		height: 100%;
		display: flex;
		flex-direction: column;
		position: relative;
		z-index: 100;
	}

	.app.collapsed .sidebar {
		padding: 20px 8px;
		padding-top: 36px;
	}

	.sidebar.repo-view {
		background: var(--panel-soft);
	}

	.sidebar-resize-handle {
		position: absolute;
		right: -2px;
		top: 0;
		bottom: 0;
		width: 4px;
		cursor: col-resize;
		background: transparent;
		z-index: 101;
		transition:
			background 0.15s ease,
			width 0.15s ease;
		touch-action: none;
	}

	/* Expanded hit area (12px total) for easier grabbing */
	.sidebar-resize-handle::after {
		content: '';
		position: absolute;
		top: 0;
		bottom: 0;
		width: 12px;
		left: 50%;
		transform: translateX(-50%);
	}

	.sidebar-resize-handle:hover,
	.sidebar-resize-handle:focus,
	.sidebar-resize-handle.active {
		background: var(--accent);
		width: 4px;
	}

	.sidebar-resize-handle:focus {
		outline: none;
	}

	.main-area {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		position: relative;
		z-index: 1;
	}

	.main {
		padding: 0;
		overflow-x: visible;
		overflow-y: hidden;
		background: transparent; /* Let vibrancy show through */
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
	}

	.view-stack {
		position: relative;
		height: 100%;
		min-height: 0;
	}

	.view-pane {
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		min-height: 0;
		opacity: 0;
		pointer-events: none;
		transition: opacity var(--transition-fast);
	}

	.view-pane.active {
		opacity: 1;
		pointer-events: auto;
	}

	.error {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 24px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.error .title {
		font-size: 18px;
		font-weight: 600;
	}

	.error .body {
		color: var(--muted);
		font-size: 14px;
	}

	.retry {
		align-self: flex-start;
		background: var(--accent);
		border: none;
		color: #081018;
		padding: 8px 12px;
		border-radius: var(--radius-md);
		font-weight: 600;
		cursor: pointer;
		transition:
			background var(--transition-fast),
			transform var(--transition-fast);
	}

	.retry:hover:not(:disabled) {
		background: color-mix(in srgb, var(--accent) 85%, white);
	}

	.retry:active:not(:disabled) {
		transform: scale(0.98);
	}

	.retry:disabled {
		opacity: 0.5;
		cursor: not-allowed;
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

	@media (max-width: 720px) {
		.overlay {
			padding: 0;
		}
	}
</style>
