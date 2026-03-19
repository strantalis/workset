<script lang="ts">
	import { GitBranch, GitPullRequest, Plus, Terminal } from '@lucide/svelte';
	import type { DocumentSession, Workspace } from '../../types';
	import DocumentViewer from '../DocumentViewer.svelte';
	import TerminalWorkspace from '../TerminalWorkspace.svelte';
	import PROrchestrationView from './PROrchestrationView.svelte';
	import ResizablePanel from '../ui/ResizablePanel.svelte';
	import { resolveWorkbenchLayout, type SurfaceTab } from './spacesWorkbenchView.helpers';

	type WorksetGroup = {
		id: string;
		label: string;
		threads: Workspace[];
	};

	interface Props {
		workspaces: Workspace[];
		activeWorkspaceId: string | null;
		popoutMode?: boolean;
		useGlobalExplorer?: boolean;
		preferredSurface?: SurfaceTab;
		documentSession?: DocumentSession | null;
		onSelectWorkspace: (workspaceId: string) => void;
		onCreateWorkspace: () => void;
		onCreateThread?: (worksetId: string) => void;
		onSurfaceChange?: (surface: SurfaceTab) => void;
		onCloseDocument?: () => void;
	}

	const {
		workspaces,
		activeWorkspaceId,
		popoutMode = false,
		useGlobalExplorer = false,
		preferredSurface = 'terminal',
		documentSession = null,
		onSelectWorkspace,
		onCreateWorkspace,
		onCreateThread = () => {},
		onSurfaceChange = () => {},
		onCloseDocument = () => {},
	}: Props = $props();

	const deriveWorksetIdentity = (workspace: Workspace): { id: string; label: string } => {
		const id = workspace.worksetKey?.trim();
		const label = workspace.worksetLabel?.trim();
		return {
			id: id && id.length > 0 ? id : `workspace:${workspace.id.toLowerCase()}`,
			label: label && label.length > 0 ? label : workspace.name,
		};
	};

	const worksetGroups = $derived.by<WorksetGroup[]>(() => {
		const byId = new Map<string, WorksetGroup>();
		for (const workspace of workspaces) {
			const { id, label } = deriveWorksetIdentity(workspace);
			const existing = byId.get(id);
			if (existing) {
				if (!workspace.placeholder) {
					existing.threads.push(workspace);
				}
				continue;
			}
			byId.set(id, {
				id,
				label,
				threads: workspace.placeholder ? [] : [workspace],
			});
		}

		return [...byId.values()]
			.map((group) => ({
				...group,
				threads: [...group.threads],
			}))
			.sort((left, right) => left.label.localeCompare(right.label));
	});

	const activeThread = $derived(
		workspaces.find(
			(workspace) => workspace.id === activeWorkspaceId && workspace.placeholder !== true,
		) ?? null,
	);

	const activeWorksetId = $derived.by(() => {
		if (!activeThread) return null;
		for (const group of worksetGroups) {
			if (group.threads.some((thread) => thread.id === activeThread.id)) return group.id;
		}
		return null;
	});

	const selectedWorkset = $derived.by(() => {
		if (activeWorksetId) {
			const match = worksetGroups.find((group) => group.id === activeWorksetId);
			if (match) return match;
		}
		return worksetGroups[0] ?? null;
	});

	const visibleThreads = $derived(selectedWorkset?.threads ?? []);
	const showThreadPanel = $derived(!useGlobalExplorer);
	const showWorkbenchHeader = $derived(!useGlobalExplorer);
	const showSurfaceTabs = $derived(!useGlobalExplorer);

	const activeBranch = $derived.by(() => {
		if (!activeThread) return 'main';
		for (const repo of activeThread.repos) {
			if (repo.currentBranch && repo.currentBranch.trim().length > 0) {
				return repo.currentBranch.trim();
			}
		}
		return 'main';
	});

	const primaryRepoLabel = $derived.by(() => {
		if (!activeThread) return 'terminal';
		const firstRepo = activeThread.repos[0];
		return firstRepo?.name ?? activeThread.name;
	});

	const openPrCount = $derived.by(() => {
		if (!activeThread) return 0;
		return activeThread.repos.filter((repo) => {
			const tracked = repo.trackedPullRequest;
			if (!tracked) return false;
			const state = tracked.state.toLowerCase();
			const merged = tracked.merged === true || state === 'merged';
			return state === 'open' && !merged;
		}).length;
	});

	const totalDirtyRepos = $derived.by(() => {
		if (!activeThread) return 0;
		return activeThread.repos.filter((repo) => {
			if (repo.missing) return true;
			if (repo.dirty) return true;
			return (repo.diff?.added ?? 0) > 0 || (repo.diff?.removed ?? 0) > 0;
		}).length;
	});

	const activeSurface = $derived(preferredSurface);
	const layoutMode = $derived(resolveWorkbenchLayout(activeSurface, documentSession !== null));

	const setSurface = (surface: SurfaceTab): void => {
		onSurfaceChange(surface);
	};
</script>

{#if workspaces.length === 0}
	<div class="spaces-empty">
		<h2>No worksets available</h2>
		<p>Create a workset to start organizing threads.</p>
		<button type="button" class="create-btn" onclick={onCreateWorkspace}>
			<Plus size={14} />
			Create Workset
		</button>
	</div>
{:else}
	<div
		class="spaces-workbench"
		class:popout={popoutMode}
		class:global-explorer={showThreadPanel === false}
	>
		{#if showThreadPanel}
			<aside class="threads-panel">
				<div class="panel-header">
					<span class="panel-title">Threads</span>
				</div>
				<div class="thread-list">
					{#each visibleThreads as thread (thread.id)}
						<button
							type="button"
							class="thread-item"
							class:active={thread.id === activeWorkspaceId}
							onclick={() => onSelectWorkspace(thread.id)}
						>
							<div class="thread-name">{thread.name}</div>
							<div class="thread-meta">
								<span>{thread.repos.length} repos</span>
								<span>{thread.repos.filter((repo) => repo.dirty).length} dirty</span>
							</div>
						</button>
					{/each}
					{#if !popoutMode}
						<button
							type="button"
							class="thread-create-row"
							onclick={() =>
								selectedWorkset ? onCreateThread(selectedWorkset.id) : onCreateWorkspace()}
							title={selectedWorkset
								? `Create thread in ${selectedWorkset.label}`
								: 'Create workset'}
							aria-label={selectedWorkset
								? `Create thread in ${selectedWorkset.label}`
								: 'Create workset'}
						>
							<Plus size={11} />
							<span>{selectedWorkset ? 'New Thread' : 'Create Workset'}</span>
						</button>
					{/if}
				</div>
			</aside>
		{/if}

		<section class="spaces-main">
			{#if showWorkbenchHeader}
				<header class="spaces-main-header">
					{#if activeThread}
						<div class="spaces-main-title">
							<h2>{activeThread.name}</h2>
							<div class="spaces-main-meta">
								<span class="branch-pill">
									<GitBranch size={12} />
									{activeBranch}
								</span>
								<span>{activeThread.repos.length} repos</span>
								<span>{visibleThreads.length} threads</span>
								<span>{totalDirtyRepos} repos with changes</span>
							</div>
						</div>
					{/if}
				</header>
			{/if}

			{#if activeThread}
				{#if showSurfaceTabs}
					<div class="surface-tabs" role="tablist" aria-label="Cockpit surfaces">
						<button
							type="button"
							class="surface-tab"
							class:active={activeSurface === 'terminal'}
							onclick={() => setSurface('terminal')}
							role="tab"
							aria-selected={activeSurface === 'terminal'}
						>
							<span class="surface-tab-icon"><Terminal size={12} /></span>
							<span>{primaryRepoLabel}</span>
						</button>
						<button
							type="button"
							class="surface-tab"
							class:active={activeSurface === 'pull-requests'}
							onclick={() => setSurface('pull-requests')}
							role="tab"
							aria-selected={activeSurface === 'pull-requests'}
						>
							<span class="surface-tab-icon"><GitPullRequest size={12} /></span>
							<span>Pull Requests</span>
							{#if openPrCount > 0}
								<span class="surface-tab-badge">{openPrCount}</span>
							{/if}
						</button>
					</div>
				{/if}
				<div class="spaces-main-body">
					{#if layoutMode === 'terminal-with-prs'}
						<ResizablePanel
							direction="horizontal"
							initialRatio={0.62}
							minRatio={0.35}
							maxRatio={0.78}
							storageKey="workset-pr-panel"
						>
							<div class="spaces-surface">
								<TerminalWorkspace
									workspaceId={activeThread.id}
									workspaceName={activeThread.name}
								/>
							</div>

							{#snippet second()}
								<aside class="spaces-side-pane spaces-pr-pane">
									<PROrchestrationView workspace={activeThread} />
								</aside>
							{/snippet}
						</ResizablePanel>
					{:else if layoutMode === 'terminal-with-document' && documentSession}
						<ResizablePanel
							direction="horizontal"
							initialRatio={0.58}
							minRatio={0.3}
							maxRatio={0.7}
							storageKey="workset-document-panel"
						>
							<div class="spaces-surface">
								<TerminalWorkspace
									workspaceId={activeThread.id}
									workspaceName={activeThread.name}
								/>
							</div>

							{#snippet second()}
								<aside class="spaces-side-pane spaces-document-pane">
									<DocumentViewer
										session={documentSession}
										repos={activeThread?.repos.map((r) => ({ id: r.id, name: r.name })) ?? []}
										onClose={onCloseDocument}
									/>
								</aside>
							{/snippet}
						</ResizablePanel>
					{:else}
						<div class="spaces-surface">
							<TerminalWorkspace workspaceId={activeThread.id} workspaceName={activeThread.name} />
						</div>
					{/if}
				</div>
			{:else}
				<div class="spaces-empty-state">
					<p>Select a thread to continue.</p>
				</div>
			{/if}
		</section>
	</div>
{/if}

<style>
	.spaces-empty {
		height: 100%;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 10px;
		color: var(--muted);
	}

	.spaces-empty h2 {
		margin: 0;
		font-size: var(--text-xl);
		color: var(--text);
	}

	.spaces-empty p {
		margin: 0;
		font-size: var(--text-sm);
	}

	.create-btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 8px 12px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--panel-strong);
		color: var(--text);
		font-size: var(--text-sm);
		cursor: pointer;
	}

	.create-btn:hover {
		border-color: color-mix(in srgb, var(--accent) 45%, var(--border));
	}

	.spaces-workbench {
		height: 100%;
		display: grid;
		grid-template-columns: 260px minmax(0, 1fr);
		min-height: 0;
	}

	.spaces-workbench.global-explorer {
		grid-template-columns: minmax(0, 1fr);
	}

	.threads-panel {
		display: flex;
		flex-direction: column;
		min-height: 0;
		background: color-mix(in srgb, var(--panel) 86%, transparent);
	}

	.threads-panel {
		border-right: 1px solid var(--border);
	}

	.panel-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 12px;
		border-bottom: 1px solid var(--border);
	}

	.panel-title {
		font-size: var(--text-xs);
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--muted);
	}

	.thread-list {
		flex: 1;
		overflow-y: auto;
		padding: 8px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.thread-item {
		display: flex;
		flex-direction: column;
		gap: 4px;
		width: 100%;
		padding: 10px;
		border: 1px solid transparent;
		border-radius: 8px;
		background: transparent;
		color: var(--muted);
		text-align: left;
	}

	.thread-item:hover {
		background: var(--hover-bg);
		border-color: var(--border);
		color: var(--text);
	}

	.thread-item.active {
		background: var(--active-accent-bg);
		border-color: var(--active-accent-border);
		color: var(--text);
	}

	.thread-create-row {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		padding: 8px 10px;
		border: 1px dashed color-mix(in srgb, var(--border) 80%, transparent);
		border-radius: 8px;
		background: transparent;
		color: var(--muted);
		font-size: var(--text-xs);
		font-weight: 600;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		cursor: pointer;
	}

	.thread-create-row:hover {
		color: var(--text);
		background: var(--hover-bg);
		border-color: var(--active-accent-border);
	}

	.thread-name {
		font-size: var(--text-sm);
		font-weight: 600;
		color: inherit;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.thread-meta {
		display: inline-flex;
		gap: 10px;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.spaces-main {
		display: flex;
		flex-direction: column;
		min-height: 0;
		min-width: 0;
	}

	.spaces-main-body {
		height: 100%;
		display: flex;
		min-height: 0;
		min-width: 0;
		flex: 1;
		padding-top: 4px;
	}

	.spaces-surface {
		flex: 1;
		min-width: 0;
		min-height: 0;
	}

	.spaces-side-pane {
		min-height: 0;
		height: 100%;
		overflow: hidden;
		border-left: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
		background: color-mix(in srgb, var(--panel) 76%, transparent);
	}

	.spaces-pr-pane {
		min-width: 420px;
	}

	.spaces-document-pane {
		min-width: 320px;
	}

	.spaces-main-header {
		padding: 10px 14px;
		border-bottom: 1px solid var(--border);
		background: color-mix(in srgb, var(--panel) 75%, transparent);
	}

	.spaces-main-title h2 {
		margin: 0;
		font-size: var(--text-lg);
		font-weight: 600;
		color: var(--text);
	}

	.spaces-main-meta {
		margin-top: 6px;
		display: inline-flex;
		align-items: center;
		gap: 10px;
		font-size: var(--text-xs);
		color: var(--muted);
	}

	.branch-pill {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		color: var(--accent);
		font-family: var(--font-mono);
	}

	.surface-tabs {
		display: flex;
		align-items: center;
		height: 48px;
		padding: 6px 10px 0;
		border-bottom: 1px solid color-mix(in srgb, var(--border) 58%, transparent);
		background: var(--panel-strong);
		gap: 4px;
		flex-shrink: 0;
	}

	.surface-tab {
		height: 100%;
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 10px 18px;
		font-size: var(--text-mono-sm);
		font-family: var(--font-mono);
		white-space: nowrap;
		border: none;
		border-bottom: 2px solid transparent;
		border-top-left-radius: 8px;
		border-top-right-radius: 8px;
		background: transparent;
		color: var(--muted);
		cursor: pointer;
		transition:
			color var(--transition-fast),
			background var(--transition-fast),
			border-color var(--transition-fast);
	}

	.surface-tab:hover {
		color: var(--text);
		background: var(--hover-bg);
	}

	.surface-tab.active {
		color: var(--accent);
		background: color-mix(in srgb, var(--panel) 82%, var(--panel-strong));
		box-shadow:
			inset 0 2px 0 color-mix(in srgb, var(--accent) 92%, transparent),
			inset 0 -1px 0 color-mix(in srgb, var(--panel) 94%, transparent);
	}

	.surface-tab-icon {
		color: var(--accent);
	}

	.surface-tab-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 16px;
		height: 15px;
		padding: 0 4px;
		border-radius: 999px;
		font-size: 10px;
		font-family: var(--font-mono);
		background: color-mix(in srgb, var(--accent) 18%, transparent);
		color: var(--accent);
	}

	.spaces-empty-state {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 24px;
		font-size: var(--text-sm);
		color: var(--muted);
	}

	@media (max-width: 1280px) {
		.spaces-workbench {
			grid-template-columns: 240px minmax(0, 1fr);
		}
	}

	@media (max-width: 1040px) {
		.spaces-workbench {
			grid-template-columns: 220px minmax(0, 1fr);
		}
	}
</style>
