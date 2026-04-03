<script lang="ts">
	import { PanelTopClose, Terminal, X } from '@lucide/svelte';
	import { onDestroy, untrack } from 'svelte';
	import ResizablePanel from './ui/ResizablePanel.svelte';
	import { tooltip } from '../actions/tooltip';
	import TerminalPane from './TerminalPane.svelte';
	import {
		createWorkspaceTerminal,
		fetchWorkspaceTerminalLayout,
		persistWorkspaceTerminalLayout,
		stopWorkspaceTerminal,
	} from '../api/terminal-layout';
	import { fetchSettings, setDefaultSetting } from '../api/settings';
	import { generateTerminalName } from '../names';
	import {
		captureTerminalSnapshot,
		closeTerminal,
		decreaseFontSize,
		getCurrentFontSize,
		increaseFontSize,
		resetFontSize,
	} from '../terminal/terminalService';
	import {
		matchTerminalKeybinding,
		resolveTerminalKeybindings,
		type ResolvedTerminalKeybindings,
	} from '../terminal/terminalKeybindings';
	import {
		LAYOUT_VERSION,
		activeTab,
		buildLeaf,
		buildTab,
		canSplit,
		closePane as closePaneInTree,
		collectLayoutTerminalIdsFromUnknown,
		collectLeaves,
		collectTabTerminalIds,
		countLeaves,
		ensureActiveTab,
		findAdjacentPane,
		findTab,
		moveTab,
		normalizeLayout,
		splitPane,
		updateNodeRatio,
		updateTab,
		type TerminalLayout,
		type TerminalLayoutNode,
		type TerminalSplitDirection,
		type TerminalTab,
	} from '../terminal/terminalLayoutModel';

	interface Props {
		workspaceId: string;
		workspaceName: string;
		active?: boolean;
	}

	type TabDragState = {
		tabId: string;
		sourceIndex: number;
	} | null;

	const { workspaceId, workspaceName, active = true }: Props = $props();

	const SAVE_DEBOUNCE_MS = 300;
	const MIN_RATIO = 0.15;
	const MAX_RATIO = 0.85;

	let layout = $state<TerminalLayout | null>(null);
	let layoutWorkspaceId = $state('');
	let initError = $state('');
	let loading = $state(false);
	let saveTimer: number | null = null;
	let pendingLayout: TerminalLayout | null = null;
	let pendingWorkspaceId = '';
	let resolvedKeybindings: ResolvedTerminalKeybindings = resolveTerminalKeybindings();
	let tabDragState = $state<TabDragState>(null);
	let topBarDropIndex = $state<number | null>(null);
	const activeLayout = $derived(layoutWorkspaceId === workspaceId ? layout : null);
	const workspaceTabs = $derived(activeLayout?.tabs ?? []);
	const activeWorkspaceTabId = $derived(activeLayout?.activeTabId ?? '');
	const currentWorkspaceTab = $derived(activeLayout ? activeTab(activeLayout) : null);
	const currentRoot = $derived(currentWorkspaceTab?.root ?? null);
	const currentLeafCount = $derived(currentRoot ? countLeaves(currentRoot) : 0);
	const currentFocusedPaneId = $derived(currentWorkspaceTab?.focusedPaneId ?? '');
	const currentCanSplit = $derived(
		currentRoot && currentFocusedPaneId ? canSplit(currentRoot, currentFocusedPaneId) : false,
	);

	const withCapturedNodeSnapshots = (
		wsId: string,
		node: TerminalLayoutNode,
	): TerminalLayoutNode => {
		if (node.kind === 'pane') {
			return {
				...node,
				snapshot: captureTerminalSnapshot(wsId, node.terminalId) ?? node.snapshot,
			};
		}
		return {
			...node,
			first: withCapturedNodeSnapshots(wsId, node.first),
			second: withCapturedNodeSnapshots(wsId, node.second),
		};
	};

	const withCapturedSnapshots = (
		targetWorkspaceId: string,
		targetLayout: TerminalLayout | null,
	): TerminalLayout | null => {
		if (!targetWorkspaceId || !targetLayout) return targetLayout;
		return {
			...targetLayout,
			tabs: targetLayout.tabs.map((tab) => ({
				...tab,
				root: withCapturedNodeSnapshots(targetWorkspaceId, tab.root),
			})),
		};
	};

	const persistLayoutWithSnapshotsNow = async (
		targetWorkspaceId: string,
		targetLayout: TerminalLayout | null,
	): Promise<void> => {
		const nextLayout = withCapturedSnapshots(targetWorkspaceId, targetLayout);
		if (!targetWorkspaceId || !nextLayout) return;
		await persistWorkspaceTerminalLayout(targetWorkspaceId, nextLayout).catch(() => undefined);
	};

	const resetWorkspaceState = (): void => {
		void persistLayoutWithSnapshotsNow(layoutWorkspaceId, layout);
		if (saveTimer) {
			window.clearTimeout(saveTimer);
			saveTimer = null;
		}
		pendingLayout = null;
		pendingWorkspaceId = '';
		layout = null;
		layoutWorkspaceId = '';
		initError = '';
	};

	const scheduleSaveLayout = (next: TerminalLayout): void => {
		if (!workspaceId) return;
		pendingLayout = next;
		pendingWorkspaceId = workspaceId;
		if (saveTimer) {
			window.clearTimeout(saveTimer);
		}
		saveTimer = window.setTimeout(() => {
			saveTimer = null;
			const target = pendingWorkspaceId;
			const toSave = withCapturedSnapshots(pendingWorkspaceId, pendingLayout);
			pendingWorkspaceId = '';
			pendingLayout = null;
			if (!target || !toSave) return;
			void persistWorkspaceTerminalLayout(target, toSave).catch(() => undefined);
		}, SAVE_DEBOUNCE_MS);
	};

	const nextTitle = (nextLayout: TerminalLayout | null): string =>
		generateTerminalName(workspaceName, nextLayout?.tabs.length ?? 0);

	const buildFreshLayout = (terminalId: string, title: string): TerminalLayout => {
		const tab = buildTab(terminalId, title);
		return {
			version: LAYOUT_VERSION,
			tabs: [tab],
			activeTabId: tab.id,
		};
	};

	const setLayout = (next: TerminalLayout, targetWorkspaceId: string): void => {
		layout = ensureActiveTab(next);
		layoutWorkspaceId = targetWorkspaceId;
	};

	const updateLayout = (next: TerminalLayout): void => {
		const normalized = ensureActiveTab(next);
		layout = normalized;
		layoutWorkspaceId = workspaceId;
		scheduleSaveLayout(normalized);
	};

	const persistCurrentFontSize = (): void => {
		void setDefaultSetting('defaults.terminal_font_size', String(getCurrentFontSize())).catch(
			() => undefined,
		);
	};

	const createAndPersistFreshLayout = async (
		targetWorkspaceId: string,
		targetWorkspaceName: string,
	): Promise<TerminalLayout> => {
		const created = await createWorkspaceTerminal(targetWorkspaceId);
		const freshLayout = buildFreshLayout(
			created.terminalId,
			generateTerminalName(targetWorkspaceName, 0),
		);
		await persistWorkspaceTerminalLayout(targetWorkspaceId, freshLayout);
		return freshLayout;
	};

	const stopLayoutSessions = async (
		targetWorkspaceId: string,
		layoutLike: unknown,
	): Promise<void> => {
		const terminalIds = collectLayoutTerminalIdsFromUnknown(layoutLike);
		if (terminalIds.length === 0) return;
		await Promise.allSettled(
			terminalIds.map((terminalId) => stopWorkspaceTerminal(targetWorkspaceId, terminalId)),
		);
	};

	const closeLayoutTerminals = async (terminalIds: string[]): Promise<void> => {
		if (terminalIds.length === 0) return;
		await Promise.allSettled(
			terminalIds.map((terminalId) => closeTerminal(workspaceId, terminalId)),
		);
	};

	const updateCurrentTab = (updater: (tab: TerminalTab) => TerminalTab): void => {
		if (!layout || !currentWorkspaceTab) return;
		updateLayout(updateTab(layout, currentWorkspaceTab.id, updater));
	};

	const setFocusedPane = (paneId: string): void => {
		if (!currentWorkspaceTab || currentWorkspaceTab.focusedPaneId === paneId) return;
		updateCurrentTab((tab) => ({
			...tab,
			focusedPaneId: paneId,
		}));
	};

	const maybeFocusAdjacentPane = (
		tab: TerminalTab | null,
		direction: 'up' | 'down' | 'left' | 'right',
	): void => {
		if (!tab || !tab.focusedPaneId) return;
		const nextPaneId = findAdjacentPane(tab.root, tab.focusedPaneId, direction);
		if (nextPaneId) {
			setFocusedPane(nextPaneId);
		}
	};

	let initToken = 0;

	const initWorkspace = async (): Promise<void> => {
		if (!workspaceId) return;
		const token = (initToken += 1);
		const targetWorkspaceId = workspaceId;
		const targetWorkspaceName = workspaceName;
		loading = true;
		initError = '';
		layout = null;
		layoutWorkspaceId = '';
		try {
			const payload = await fetchWorkspaceTerminalLayout(targetWorkspaceId);
			if (token !== initToken || workspaceId !== targetWorkspaceId) return;
			const normalized = normalizeLayout(payload?.layout);
			if (normalized) {
				setLayout(normalized, targetWorkspaceId);
				return;
			}
			if (payload?.layout) {
				await stopLayoutSessions(targetWorkspaceId, payload.layout);
				if (token !== initToken || workspaceId !== targetWorkspaceId) return;
			}
			const freshLayout = await createAndPersistFreshLayout(targetWorkspaceId, targetWorkspaceName);
			if (token !== initToken || workspaceId !== targetWorkspaceId) return;
			setLayout(freshLayout, targetWorkspaceId);
		} catch (error) {
			if (token !== initToken || workspaceId !== targetWorkspaceId) return;
			initError = String(error);
		} finally {
			if (token === initToken && workspaceId === targetWorkspaceId) {
				loading = false;
			}
		}
	};

	const handleSelectWorkspaceTab = (tabId: string): void => {
		if (!layout || layout.activeTabId === tabId) return;
		updateLayout({ ...layout, activeTabId: tabId });
	};

	const handleAddWorkspaceTab = async (): Promise<void> => {
		if (!layout) return;
		try {
			const created = await createWorkspaceTerminal(workspaceId);
			const tab = buildTab(created.terminalId, nextTitle(layout));
			updateLayout({
				...layout,
				tabs: [...layout.tabs, tab],
				activeTabId: tab.id,
			});
		} catch (error) {
			initError = String(error);
		}
	};

	const handleCloseWorkspaceTab = async (tabId: string): Promise<void> => {
		if (!layout) return;
		const currentLayout = layout;
		const closingTab = findTab(currentLayout, tabId);
		if (!closingTab) return;
		await closeLayoutTerminals(collectTabTerminalIds(closingTab));

		if (currentLayout.tabs.length === 1) {
			try {
				const freshLayout = await createAndPersistFreshLayout(workspaceId, workspaceName);
				setLayout(freshLayout, workspaceId);
			} catch (error) {
				initError = String(error);
			}
			return;
		}

		const nextTabs = currentLayout.tabs.filter((tab) => tab.id !== tabId);
		const closingIndex = currentLayout.tabs.findIndex((tab) => tab.id === tabId);
		const nextActive = nextTabs[Math.max(0, closingIndex - 1)] ?? nextTabs[0];
		updateLayout({
			...currentLayout,
			tabs: nextTabs,
			activeTabId: nextActive.id,
		});
	};

	const handleClosePane = async (paneId: string): Promise<void> => {
		if (!currentWorkspaceTab) return;
		const leaves = collectLeaves(currentWorkspaceTab.root);
		const closingLeaf = leaves.find((leaf) => leaf.id === paneId);
		if (!closingLeaf) return;

		await closeTerminal(workspaceId, closingLeaf.terminalId);

		// If only one pane, close the whole tab
		if (leaves.length === 1) {
			await handleCloseWorkspaceTab(currentWorkspaceTab.id);
			return;
		}

		updateCurrentTab((tab) => {
			const newRoot = closePaneInTree(tab.root, paneId);
			if (!newRoot) return tab;
			const remaining = collectLeaves(newRoot);
			return {
				...tab,
				root: newRoot,
				focusedPaneId: remaining[0]?.id ?? tab.focusedPaneId,
			};
		});
	};

	const handleSplitDirection = async (direction: TerminalSplitDirection): Promise<void> => {
		if (!currentWorkspaceTab || !currentFocusedPaneId) return;
		if (!canSplit(currentWorkspaceTab.root, currentFocusedPaneId)) return;
		try {
			const created = await createWorkspaceTerminal(workspaceId);
			const newLeaf = buildLeaf(created.terminalId);
			updateCurrentTab((tab) => {
				const newRoot = splitPane(tab.root, tab.focusedPaneId ?? '', newLeaf, direction);
				if (!newRoot) return tab;
				return { ...tab, root: newRoot, focusedPaneId: newLeaf.id };
			});
		} catch (error) {
			initError = String(error);
		}
	};

	const handleNodeRatioChange = (nodeId: string, ratio: number): void => {
		if (!currentWorkspaceTab) return;
		updateCurrentTab((tab) => ({
			...tab,
			root: updateNodeRatio(tab.root, nodeId, ratio),
		}));
	};

	const handleTabDragStart = (tabId: string, index: number, event: DragEvent): void => {
		tabDragState = { tabId, sourceIndex: index };
		topBarDropIndex = index;
		event.dataTransfer?.setData('text/plain', tabId);
		if (event.dataTransfer) {
			event.dataTransfer.effectAllowed = 'move';
		}
	};

	const handleTabDragOver = (index: number, event: DragEvent): void => {
		if (!tabDragState) return;
		event.preventDefault();
		topBarDropIndex = index;
		if (event.dataTransfer) {
			event.dataTransfer.dropEffect = 'move';
		}
	};

	const handleTabDrop = (index: number, event: DragEvent): void => {
		if (!layout || !tabDragState) return;
		event.preventDefault();
		updateLayout(moveTab(layout, tabDragState.sourceIndex, index));
		tabDragState = null;
		topBarDropIndex = null;
	};

	const handleTabDragEnd = (): void => {
		tabDragState = null;
		topBarDropIndex = null;
	};

	const handleWorkspaceKeydown = (event: KeyboardEvent): void => {
		if (!layout) return;
		const currentLayout = layout;
		const currentTab = activeTab(currentLayout);
		const action = matchTerminalKeybinding(event, resolvedKeybindings);
		if (!action) return;

		switch (action) {
			case 'terminal.focus_pane_up':
				event.preventDefault();
				maybeFocusAdjacentPane(currentTab, 'up');
				return;
			case 'terminal.focus_pane_down':
				event.preventDefault();
				maybeFocusAdjacentPane(currentTab, 'down');
				return;
			case 'terminal.focus_pane_left':
				event.preventDefault();
				maybeFocusAdjacentPane(currentTab, 'left');
				return;
			case 'terminal.focus_pane_right':
				event.preventDefault();
				maybeFocusAdjacentPane(currentTab, 'right');
				return;
			case 'terminal.next_tab':
			case 'terminal.prev_tab': {
				if (currentLayout.tabs.length <= 1) return;
				event.preventDefault();
				const currentIndex = currentLayout.tabs.findIndex(
					(tab) => tab.id === currentLayout.activeTabId,
				);
				const delta = action === 'terminal.prev_tab' ? -1 : 1;
				const nextIndex =
					(currentIndex + delta + currentLayout.tabs.length) % currentLayout.tabs.length;
				handleSelectWorkspaceTab(currentLayout.tabs[nextIndex].id);
				return;
			}
			case 'terminal.close_tab': {
				if (!currentTab) return;
				event.preventDefault();
				void handleCloseWorkspaceTab(currentTab.id);
				return;
			}
			case 'terminal.new_tab': {
				event.preventDefault();
				void handleAddWorkspaceTab();
				return;
			}
			case 'terminal.split_vertical': {
				event.preventDefault();
				void handleSplitDirection('vertical');
				return;
			}
			case 'terminal.split_horizontal': {
				event.preventDefault();
				void handleSplitDirection('horizontal');
				return;
			}
			case 'terminal.font_increase': {
				event.preventDefault();
				increaseFontSize();
				persistCurrentFontSize();
				return;
			}
			case 'terminal.font_decrease': {
				event.preventDefault();
				decreaseFontSize();
				persistCurrentFontSize();
				return;
			}
			case 'terminal.font_reset': {
				event.preventDefault();
				resetFontSize();
				persistCurrentFontSize();
				return;
			}
			default: {
				if (!action.startsWith('terminal.focus_tab_')) return;
				const index = Number.parseInt(action.replace('terminal.focus_tab_', ''), 10);
				if (!Number.isFinite(index) || index < 1 || index > 9) return;
				const tabIndex = index - 1;
				if (tabIndex < currentLayout.tabs.length) {
					event.preventDefault();
					handleSelectWorkspaceTab(currentLayout.tabs[tabIndex].id);
				}
			}
		}
	};

	onDestroy(() => {
		void persistLayoutWithSnapshotsNow(layoutWorkspaceId, layout);
		if (saveTimer) {
			window.clearTimeout(saveTimer);
		}
	});

	$effect(() => {
		const targetWorkspaceId = workspaceId.trim();
		untrack(() => {
			resetWorkspaceState();
			initToken += 1;
		});
		if (!targetWorkspaceId) {
			loading = false;
			return;
		}
		loading = true;
		void initWorkspace();
	});

	$effect(() => {
		if (typeof window === 'undefined') return;
		let cancelled = false;
		const loadKeybindings = async (): Promise<void> => {
			try {
				const settings = await fetchSettings();
				if (cancelled) return;
				resolvedKeybindings = resolveTerminalKeybindings(settings?.defaults?.terminalKeybindings);
			} catch {
				if (cancelled) return;
				resolvedKeybindings = resolveTerminalKeybindings();
			}
		};
		void loadKeybindings();
		return () => {
			cancelled = true;
		};
	});

	$effect(() => {
		if (!workspaceId || typeof window === 'undefined') return;
		const handler = (event: Event): void => {
			const detail = (event as CustomEvent<{ workspaceId?: string }>).detail;
			if (!detail?.workspaceId || detail.workspaceId !== workspaceId) return;
			void initWorkspace();
		};
		window.addEventListener('workset:terminal-layout-reset', handler);
		return () => {
			window.removeEventListener('workset:terminal-layout-reset', handler);
		};
	});

	$effect(() => {
		if (typeof window === 'undefined') return;
		const handleBeforeUnload = (): void => {
			void persistLayoutWithSnapshotsNow(layoutWorkspaceId, layout);
		};
		window.addEventListener('beforeunload', handleBeforeUnload);
		return () => {
			window.removeEventListener('beforeunload', handleBeforeUnload);
		};
	});
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<section
	class="terminal-workspace"
	role="application"
	tabindex="-1"
	onkeydown={handleWorkspaceKeydown}
>
	<div class="workspace-container">
		{#if initError}
			<div class="terminal-error">
				<div class="error-icon">
					<svg
						width="24"
						height="24"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					>
						<circle cx="12" cy="12" r="10" />
						<line x1="12" y1="8" x2="12" y2="12" />
						<line x1="12" y1="16" x2="12.01" y2="16" />
					</svg>
				</div>
				<div class="error-title">Failed to start terminal</div>
				<div class="error-detail">{initError}</div>
				<button type="button" class="retry-action" onclick={() => initWorkspace()}>
					Restart
				</button>
			</div>
		{:else if loading || !activeLayout}
			<div class="terminal-loading">
				<div class="loading-spinner"></div>
				<span>Starting thread terminals…</span>
			</div>
		{:else}
			<div class="workspace-shell">
				<div class="workspace-tabs" role="tablist" aria-label="Terminal tabs">
					<div
						class="workspace-tabs-scroll"
						onwheel={(e) => {
							if (e.deltaY === 0) return;
							e.preventDefault();
							e.currentTarget.scrollLeft += e.deltaY;
						}}
					>
						{#each workspaceTabs as tab, index (tab.id)}
							<div
								class="workspace-tab"
								class:active={tab.id === activeWorkspaceTabId}
								class:dragging={tabDragState?.tabId === tab.id}
								class:drop-target={topBarDropIndex === index && tabDragState?.tabId !== tab.id}
								role="tab"
								tabindex="0"
								aria-selected={tab.id === activeWorkspaceTabId}
								draggable="true"
								onclick={() => handleSelectWorkspaceTab(tab.id)}
								onkeydown={(event) => {
									if (event.key !== 'Enter' && event.key !== ' ') return;
									event.preventDefault();
									handleSelectWorkspaceTab(tab.id);
								}}
								ondragstart={(event) => handleTabDragStart(tab.id, index, event)}
								ondragover={(event) => handleTabDragOver(index, event)}
								ondrop={(event) => handleTabDrop(index, event)}
								ondragend={handleTabDragEnd}
							>
								<span class="workspace-tab-prompt"><Terminal size={12} /></span>
								<span class="workspace-tab-title">{tab.title}</span>
								<button
									type="button"
									class="workspace-tab-close"
									aria-label={`Close ${tab.title}`}
									onclick={(event) => {
										event.stopPropagation();
										void handleCloseWorkspaceTab(tab.id);
									}}
								>
									<X size={14} />
								</button>
							</div>
						{/each}
						<button
							type="button"
							class="workspace-tab-add"
							aria-label="New terminal tab"
							use:tooltip={'New tab (⌘T)'}
							onclick={() => void handleAddWorkspaceTab()}
						>
							+
						</button>
					</div>
					{#if currentWorkspaceTab}
						<div class="workspace-tab-actions">
							<button
								type="button"
								class="ws-icon-action-btn"
								class:disabled={!currentCanSplit}
								use:tooltip={'Split vertical (⌘\\)'}
								aria-label="Split vertical"
								aria-disabled={!currentCanSplit}
								onclick={() => currentCanSplit && void handleSplitDirection('vertical')}
							>
								<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
									<rect
										x="1"
										y="2"
										width="12"
										height="10"
										rx="1.5"
										stroke="currentColor"
										stroke-width="1.2"
									/>
									<path d="M7 2v10" stroke="currentColor" stroke-width="1.2" />
								</svg>
							</button>
							<button
								type="button"
								class="ws-icon-action-btn"
								class:disabled={!currentCanSplit}
								use:tooltip={'Split horizontal (⌘⇧\\)'}
								aria-label="Split horizontal"
								aria-disabled={!currentCanSplit}
								onclick={() => currentCanSplit && void handleSplitDirection('horizontal')}
							>
								<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
									<rect
										x="1"
										y="2"
										width="12"
										height="10"
										rx="1.5"
										stroke="currentColor"
										stroke-width="1.2"
									/>
									<path d="M1 7h12" stroke="currentColor" stroke-width="1.2" />
								</svg>
							</button>
							{#if currentLeafCount > 1 && currentFocusedPaneId}
								<button
									type="button"
									class="ws-icon-action-btn"
									use:tooltip={'Close pane'}
									aria-label="Close pane"
									onclick={() => void handleClosePane(currentFocusedPaneId)}
								>
									<PanelTopClose size={14} />
								</button>
							{/if}
						</div>
					{/if}
				</div>
				{#snippet renderNode(node: TerminalLayoutNode | null)}
					{#if !node}
						<!-- null guard during reactive transitions -->
					{:else if node.kind === 'pane'}
						<div
							class="workspace-pane-surface"
							class:focused={currentFocusedPaneId === node.id}
							role="button"
							tabindex="0"
							onclick={() => setFocusedPane(node.id)}
							onkeydown={(event) => {
								if (event.key !== 'Enter' && event.key !== ' ') return;
								event.preventDefault();
								setFocusedPane(node.id);
							}}
						>
							<TerminalPane
								{workspaceId}
								{workspaceName}
								terminalId={node.terminalId}
								initialSnapshot={node.snapshot}
								active={currentFocusedPaneId === node.id && active}
								compact={true}
								onTerminalClosed={() => void handleClosePane(node.id)}
							/>
						</div>
					{:else}
						<ResizablePanel
							direction={node.direction === 'horizontal' ? 'vertical' : 'horizontal'}
							ratio={node.ratio}
							minRatio={MIN_RATIO}
							maxRatio={MAX_RATIO}
							onRatioChange={(r) => handleNodeRatioChange(node.id, r)}
						>
							{@render renderNode(node.first)}
							{#snippet second()}
								{@render renderNode(node.second)}
							{/snippet}
						</ResizablePanel>
					{/if}
				{/snippet}

				<div class="workspace-layout">
					{#if currentRoot}
						{@render renderNode(currentRoot)}
					{/if}
				</div>
			</div>
		{/if}
	</div>
</section>

<style>
	.terminal-workspace {
		display: flex;
		flex-direction: column;
		gap: 0;
		height: 100%;
		position: relative;
	}

	.workspace-container {
		flex: 1;
		min-height: 0;
		display: flex;
		border: none;
		border-radius: 0;
		background: var(--panel);
		overflow: hidden;
	}

	.workspace-shell {
		display: flex;
		flex: 1;
		min-height: 0;
		flex-direction: column;
	}

	.workspace-tabs {
		display: flex;
		align-items: center;
		gap: 0;
		padding: 0 4px;
		border-bottom: 1px solid var(--border);
		background: var(--panel-strong);
	}

	.workspace-tabs-scroll {
		display: flex;
		align-items: center;
		gap: 2px;
		flex: 1 1 0;
		min-width: 0;
		width: 0;
		overflow-x: auto;
		scrollbar-width: none;
	}

	.workspace-tabs-scroll::-webkit-scrollbar {
		display: none;
	}

	.workspace-tab {
		display: flex;
		align-items: center;
		gap: 5px;
		flex-shrink: 0;
		max-width: 280px;
		padding: 6px 12px;
		background: transparent;
		color: var(--muted);
		cursor: grab;
		border: none;
		border-radius: 0;
		box-shadow: none;
		transition:
			color var(--transition-fast),
			background var(--transition-fast),
			box-shadow var(--transition-fast);
		position: relative;
	}

	.workspace-tab:hover {
		color: var(--text);
		background: var(--hover-bg);
	}

	.workspace-tab:active {
		cursor: grabbing;
	}

	.workspace-tab.active {
		color: var(--text);
		background: var(--panel);
		box-shadow: inset 0 2px 0 var(--accent);
	}

	.workspace-tab.dragging {
		opacity: 0.4;
	}

	.workspace-tab.drop-target {
		box-shadow:
			inset 2px 0 0 var(--accent),
			inset 0 2px 0 color-mix(in srgb, var(--accent) 35%, transparent);
	}

	.workspace-tab-title {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: var(--text-sm);
	}

	.workspace-tab-prompt {
		display: inline-flex;
		align-items: center;
		color: var(--accent);
		font-weight: 500;
		flex-shrink: 0;
	}

	.workspace-tab-close,
	.workspace-tab-add {
		border: none;
		background: none;
		color: inherit;
		cursor: pointer;
	}

	.workspace-tab-close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		margin-left: 6px;
		border-radius: 3px;
		font-size: 14px;
		line-height: 1;
		opacity: 0;
		transition:
			opacity var(--transition-fast),
			background var(--transition-fast),
			color var(--transition-fast);
	}

	.workspace-tab:hover .workspace-tab-close,
	.workspace-tab.active .workspace-tab-close {
		opacity: 0.7;
	}

	.workspace-tab-close:hover {
		opacity: 1;
		background: color-mix(in srgb, var(--warning) 20%, transparent);
		color: var(--warning);
	}

	.workspace-tab-add {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		margin-left: 4px;
		border-radius: 4px;
		font-size: 16px;
		color: var(--muted);
		flex-shrink: 0;
	}

	.workspace-tab-actions {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		margin-left: 6px;
		flex-shrink: 0;
	}

	.workspace-tab-add:hover {
		color: var(--text);
		background: var(--hover-bg);
	}

	.workspace-layout {
		flex: 1;
		min-height: 0;
		display: flex;
	}

	.workspace-pane-surface {
		flex: 1;
		min-width: 0;
		min-height: 0;
		display: flex;
		flex-direction: column;
		outline: none;
		position: relative;
	}

	.workspace-pane-surface::before {
		content: '';
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: 2px;
		background: transparent;
		transition: background var(--transition-fast);
		z-index: 1;
	}

	.workspace-pane-surface.focused::before {
		background: var(--accent);
	}

	.workspace-pane-surface :global(.terminal) {
		flex: 1;
		min-height: 0;
	}

	.ws-icon-action-btn.disabled {
		opacity: 0.3;
		cursor: default;
		pointer-events: none;
	}

	.terminal-loading {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 12px;
		padding: 16px;
		color: var(--muted);
		font-size: var(--text-sm);
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.loading-spinner {
		width: 20px;
		height: 20px;
		border: 2px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	.terminal-error {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 24px;
		color: var(--text);
		font-size: var(--text-sm);
		text-align: center;
	}

	.error-icon {
		color: var(--danger);
		margin-bottom: 4px;
	}

	.error-title {
		font-weight: 600;
	}

	.error-detail {
		max-width: 420px;
		color: var(--muted);
	}

	.retry-action {
		margin-top: 8px;
		border: 1px solid color-mix(in srgb, var(--accent) 45%, transparent);
		background: color-mix(in srgb, var(--accent) 16%, transparent);
		color: var(--text);
		border-radius: 8px;
		padding: 8px 12px;
		cursor: pointer;
	}
</style>
