<script lang="ts">
	import { onDestroy } from 'svelte';
	import TerminalLayoutNode from './TerminalLayoutNode.svelte';
	import {
		createWorkspaceTerminal,
		fetchWorkspaceTerminalStatus,
		fetchWorkspaceTerminalLayout,
		persistWorkspaceTerminalLayout,
	} from '../api/terminal-layout';
	import { fetchSettings } from '../api/settings';
	import { generateTerminalName } from '../names';
	import {
		closeTerminal,
		decreaseFontSize,
		increaseFontSize,
		resetFontSize,
	} from '../terminal/terminalService';
	import {
		matchTerminalKeybinding,
		resolveTerminalKeybindings,
		type ResolvedTerminalKeybindings,
	} from '../terminal/terminalKeybindings';
	import {
		clearLegacyTerminalLayout,
		loadLegacyTerminalLayout,
		markLayoutMigrationComplete,
		shouldRunLayoutMigration,
	} from '../terminal/terminalLayoutStorage';
	import {
		LAYOUT_VERSION,
		applyTabFixes,
		buildPane,
		buildPanePositions,
		buildTab,
		collectPaneIds,
		collectTabs,
		ensureFocusedPane,
		findAdjacentPane,
		findPane,
		moveTab,
		newId,
		normalizeLayout,
		removePane,
		splitPane,
		updatePane,
		updateSplitRatio,
		type LayoutNode,
		type PaneNode,
		type TerminalLayout,
	} from '../terminal/terminalLayoutTree';

	interface Props {
		workspaceId: string;
		workspaceName: string;
		active?: boolean;
	}

	const { workspaceId, workspaceName, active = true }: Props = $props();

	const SAVE_DEBOUNCE_MS = 300;

	let layout = $state<TerminalLayout | null>(null);
	let initError = $state('');
	let loading = $state(false);
	let saveTimer: number | null = null;
	let pendingLayout: TerminalLayout | null = null;
	let pendingWorkspaceId = '';
	let resolvedKeybindings: ResolvedTerminalKeybindings = resolveTerminalKeybindings();

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
			const toSave = pendingLayout;
			pendingWorkspaceId = '';
			pendingLayout = null;
			if (!target || !toSave) return;
			void persistWorkspaceTerminalLayout(target, toSave).catch(() => {});
		}, SAVE_DEBOUNCE_MS);
	};

	const loadLayout = async (id: string): Promise<TerminalLayout | null> => {
		if (!id) return null;
		try {
			const payload = await fetchWorkspaceTerminalLayout(id);
			return normalizeLayout(payload?.layout);
		} catch {
			return null;
		}
	};

	const nextTitle = (node: LayoutNode): string => {
		const count = collectTabs(node).length;
		return generateTerminalName(workspaceName, count);
	};

	const migrateLayoutOnce = async (
		id: string,
		layoutToMigrate: TerminalLayout,
	): Promise<{ layout: TerminalLayout; changed: boolean }> => {
		if (!shouldRunLayoutMigration(id)) {
			return { layout: layoutToMigrate, changed: false };
		}
		markLayoutMigrationComplete(id);
		const tabs = collectTabs(layoutToMigrate.root);
		if (tabs.length === 0) {
			return { layout: layoutToMigrate, changed: false };
		}
		let changed = false;
		const fixes = new Map<string, { terminalId?: string; drop?: boolean }>();
		for (const tab of tabs) {
			if (!tab.terminalId) {
				fixes.set(tab.id, { drop: true });
				changed = true;
				continue;
			}
			let shouldReplace = false;
			try {
				const status = await fetchWorkspaceTerminalStatus(id, tab.terminalId);
				if (!status) {
					continue;
				}
				if (status.active) {
					continue;
				}
				if (status.error) {
					continue;
				}
				shouldReplace = true;
			} catch {
				continue;
			}
			if (!shouldReplace) continue;
			try {
				const created = await createWorkspaceTerminal(id);
				if (created?.terminalId) {
					fixes.set(tab.id, { terminalId: created.terminalId });
				} else {
					fixes.set(tab.id, { drop: true });
				}
				changed = true;
			} catch {
				fixes.set(tab.id, { drop: true });
				changed = true;
			}
		}
		if (!changed) {
			return { layout: layoutToMigrate, changed: false };
		}
		const nextRoot = applyTabFixes(layoutToMigrate.root, fixes);
		if (!nextRoot) {
			const created = await createWorkspaceTerminal(id);
			const tab = buildTab(created.terminalId, generateTerminalName(workspaceName, 0));
			const pane = buildPane(tab);
			const fresh = ensureFocusedPane({
				version: LAYOUT_VERSION,
				root: pane,
				focusedPaneId: pane.id,
			});
			return { layout: fresh, changed: true };
		}
		const updated = ensureFocusedPane({ ...layoutToMigrate, root: nextRoot });
		return { layout: updated, changed: true };
	};

	const setLayout = (next: TerminalLayout): void => {
		layout = ensureFocusedPane(next);
	};

	const updateLayout = (next: TerminalLayout): void => {
		const normalized = ensureFocusedPane(next);
		layout = normalized;
		scheduleSaveLayout(normalized);
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
		try {
			const stored = await loadLayout(targetWorkspaceId);
			if (token !== initToken || workspaceId !== targetWorkspaceId) return;
			if (stored) {
				const migrated = await migrateLayoutOnce(targetWorkspaceId, stored);
				if (token !== initToken || workspaceId !== targetWorkspaceId) return;
				if (migrated.changed) {
					setLayout(migrated.layout);
					void persistWorkspaceTerminalLayout(targetWorkspaceId, migrated.layout).catch(() => {});
				} else {
					setLayout(migrated.layout);
				}
				return;
			}
			const legacy = loadLegacyTerminalLayout(targetWorkspaceId);
			if (token !== initToken || workspaceId !== targetWorkspaceId) return;
			if (legacy) {
				const migrated = await migrateLayoutOnce(targetWorkspaceId, legacy);
				if (token !== initToken || workspaceId !== targetWorkspaceId) return;
				setLayout(migrated.layout);
				void persistWorkspaceTerminalLayout(targetWorkspaceId, migrated.layout)
					.then(() => {
						clearLegacyTerminalLayout(targetWorkspaceId);
					})
					.catch(() => {});
				return;
			}
			const created = await createWorkspaceTerminal(targetWorkspaceId);
			if (token !== initToken || workspaceId !== targetWorkspaceId) return;
			const tab = buildTab(created.terminalId, generateTerminalName(targetWorkspaceName, 0));
			const pane = buildPane(tab);
			updateLayout({ version: LAYOUT_VERSION, root: pane, focusedPaneId: pane.id });
		} catch (error) {
			if (token !== initToken || workspaceId !== targetWorkspaceId) return;
			initError = String(error);
		} finally {
			if (token === initToken && workspaceId === targetWorkspaceId) {
				loading = false;
			}
		}
	};

	const handleFocusPane = (paneId: string): void => {
		if (!layout) return;
		if (layout.focusedPaneId === paneId) return;
		updateLayout({ ...layout, focusedPaneId: paneId });
	};

	const handleSelectTab = (paneId: string, tabId: string): void => {
		if (!layout) return;
		const nextRoot = updatePane(layout.root, paneId, (pane) => ({
			...pane,
			activeTabId: tabId,
		}));
		updateLayout({ ...layout, root: nextRoot, focusedPaneId: paneId });
	};

	const handleAddTab = async (paneId: string): Promise<void> => {
		if (!layout) return;
		try {
			const created = await createWorkspaceTerminal(workspaceId);
			const title = nextTitle(layout.root);
			const tab = buildTab(created.terminalId, title);
			const nextRoot = updatePane(layout.root, paneId, (pane) => ({
				...pane,
				tabs: [...pane.tabs, tab],
				activeTabId: tab.id,
			}));
			updateLayout({ ...layout, root: nextRoot, focusedPaneId: paneId });
		} catch (error) {
			initError = String(error);
		}
	};

	const handleSplitPane = async (paneId: string, direction: 'row' | 'column'): Promise<void> => {
		if (!layout) return;
		try {
			const created = await createWorkspaceTerminal(workspaceId);
			const title = nextTitle(layout.root);
			const tab = buildTab(created.terminalId, title);
			const pane = buildPane(tab);
			const nextRoot = splitPane(layout.root, paneId, direction, pane);
			updateLayout({ ...layout, root: nextRoot, focusedPaneId: paneId });
		} catch (error) {
			initError = String(error);
		}
	};

	const handleCloseTab = (paneId: string, tabId: string): void => {
		if (!layout) return;
		const pane = findPane(layout.root, paneId);
		if (!pane) return;
		const closing = pane.tabs.find((tab) => tab.id === tabId);
		if (closing) {
			void closeTerminal(workspaceId, closing.terminalId);
		}
		const remaining = pane.tabs.filter((tab) => tab.id !== tabId);
		if (remaining.length === 0) {
			handleClosePane(paneId);
			return;
		}
		const nextActive = pane.activeTabId === tabId ? remaining[0].id : pane.activeTabId;
		const nextRoot = updatePane(layout.root, paneId, (existing) => ({
			...existing,
			tabs: remaining,
			activeTabId: nextActive,
		}));
		updateLayout({ ...layout, root: nextRoot, focusedPaneId: paneId });
	};

	const handleClosePane = (paneId: string): void => {
		if (!layout) return;
		const pane = findPane(layout.root, paneId);
		if (pane) {
			for (const tab of pane.tabs) {
				void closeTerminal(workspaceId, tab.terminalId);
			}
		}
		const nextRoot = removePane(layout.root, paneId);
		if (!nextRoot) {
			void (async () => {
				try {
					const created = await createWorkspaceTerminal(workspaceId);
					const tab = buildTab(created.terminalId, generateTerminalName(workspaceName, 0));
					const pane = buildPane(tab);
					updateLayout({ version: LAYOUT_VERSION, root: pane, focusedPaneId: pane.id });
				} catch (error) {
					initError = String(error);
				}
			})();
			return;
		}
		updateLayout({ ...layout, root: nextRoot });
	};

	const MIN_RATIO = 0.15;
	const MAX_RATIO = 0.85;

	const handleResizeSplit = (splitId: string, ratio: number): void => {
		if (!layout) return;
		const clampedRatio = Math.max(MIN_RATIO, Math.min(MAX_RATIO, ratio));
		const nextRoot = updateSplitRatio(layout.root, splitId, clampedRatio);
		updateLayout({ ...layout, root: nextRoot });
	};

	// Drag state for tab reordering
	type DragState = {
		tabId: string;
		sourcePaneId: string;
		sourceIndex: number;
	} | null;

	let dragState = $state<DragState>(null);

	const handleTabDragStart = (paneId: string, tabId: string, index: number): void => {
		dragState = { tabId, sourcePaneId: paneId, sourceIndex: index };
	};

	const handleTabDragEnd = (): void => {
		dragState = null;
	};

	const handleTabDrop = (targetPaneId: string, targetIndex: number): void => {
		if (!layout || !dragState) return;
		const { tabId, sourcePaneId } = dragState;

		// Move the tab
		let nextRoot = moveTab(layout.root, sourcePaneId, targetPaneId, tabId, targetIndex);

		// Check if source pane is now empty and needs to be removed
		const sourcePane = findPane(nextRoot, sourcePaneId);
		if (sourcePane && sourcePane.tabs.length === 0) {
			const removed = removePane(nextRoot, sourcePaneId);
			if (removed) {
				nextRoot = removed;
			}
		}

		updateLayout({ ...layout, root: nextRoot, focusedPaneId: targetPaneId });
		dragState = null;
	};

	const handleTabSplitDrop = (
		targetPaneId: string,
		direction: 'row' | 'column',
		position: 'before' | 'after',
	): void => {
		if (!layout || !dragState) return;
		const { tabId, sourcePaneId } = dragState;

		// Find the tab in source pane
		const sourcePane = findPane(layout.root, sourcePaneId);
		if (!sourcePane) return;
		const tab = sourcePane.tabs.find((t) => t.id === tabId);
		if (!tab) return;

		// Create a new pane with the tab
		const newPane: PaneNode = {
			id: newId(),
			kind: 'pane',
			tabs: [tab],
			activeTabId: tab.id,
		};

		// Remove tab from source pane
		let nextRoot = updatePane(layout.root, sourcePaneId, (pane) => ({
			...pane,
			tabs: pane.tabs.filter((t) => t.id !== tabId),
			activeTabId:
				pane.activeTabId === tabId
					? (pane.tabs.find((t) => t.id !== tabId)?.id ?? pane.activeTabId)
					: pane.activeTabId,
		}));

		// Check if source pane is now empty and needs to be removed
		const updatedSourcePane = findPane(nextRoot, sourcePaneId);
		if (updatedSourcePane && updatedSourcePane.tabs.length === 0) {
			const removed = removePane(nextRoot, sourcePaneId);
			if (removed) {
				nextRoot = removed;
			}
		}

		// Split the target pane with the new pane
		// position 'before' means new pane goes first, 'after' means new pane goes second
		const splitWithNewPane = (node: LayoutNode): LayoutNode => {
			if (node.kind === 'pane') {
				if (node.id !== targetPaneId) return node;
				return {
					id: newId(),
					kind: 'split',
					direction,
					ratio: 0.5,
					first: position === 'before' ? newPane : node,
					second: position === 'before' ? node : newPane,
				};
			}
			const first = splitWithNewPane(node.first);
			const second = splitWithNewPane(node.second);
			if (first === node.first && second === node.second) return node;
			return { ...node, first, second };
		};

		nextRoot = splitWithNewPane(nextRoot);
		updateLayout({ ...layout, root: nextRoot, focusedPaneId: newPane.id });
		dragState = null;
	};

	const handleWorkspaceKeydown = (event: KeyboardEvent): void => {
		if (!layout) return;
		const action = matchTerminalKeybinding(event, resolvedKeybindings);
		if (!action) return;

		switch (action) {
			case 'terminal.focus_pane_up':
			case 'terminal.focus_pane_down':
			case 'terminal.focus_pane_left':
			case 'terminal.focus_pane_right': {
				if (!layout.focusedPaneId) return;
				const direction = action.replace('terminal.focus_pane_', '') as
					| 'up'
					| 'down'
					| 'left'
					| 'right';
				event.preventDefault();
				const positions = buildPanePositions(layout.root);
				const nextPaneId = findAdjacentPane(layout.focusedPaneId, direction, positions);
				if (nextPaneId) {
					handleFocusPane(nextPaneId);
				}
				return;
			}
			case 'terminal.next_tab':
			case 'terminal.prev_tab': {
				if (!layout.focusedPaneId) return;
				const pane = findPane(layout.root, layout.focusedPaneId);
				if (!pane || pane.tabs.length <= 1) return;
				event.preventDefault();
				const currentIndex = pane.tabs.findIndex((t) => t.id === pane.activeTabId);
				const delta = action === 'terminal.prev_tab' ? -1 : 1;
				const nextIndex = (currentIndex + delta + pane.tabs.length) % pane.tabs.length;
				handleSelectTab(layout.focusedPaneId, pane.tabs[nextIndex].id);
				return;
			}
			case 'terminal.close_tab': {
				if (!layout.focusedPaneId) return;
				const pane = findPane(layout.root, layout.focusedPaneId);
				if (pane && pane.activeTabId) {
					event.preventDefault();
					handleCloseTab(layout.focusedPaneId, pane.activeTabId);
				}
				return;
			}
			case 'terminal.new_tab': {
				if (!layout.focusedPaneId) return;
				event.preventDefault();
				void handleAddTab(layout.focusedPaneId);
				return;
			}
			case 'terminal.split_vertical': {
				if (!layout.focusedPaneId) return;
				event.preventDefault();
				void handleSplitPane(layout.focusedPaneId, 'row');
				return;
			}
			case 'terminal.split_horizontal': {
				if (!layout.focusedPaneId) return;
				event.preventDefault();
				void handleSplitPane(layout.focusedPaneId, 'column');
				return;
			}
			case 'terminal.font_increase': {
				event.preventDefault();
				increaseFontSize();
				return;
			}
			case 'terminal.font_decrease': {
				event.preventDefault();
				decreaseFontSize();
				return;
			}
			case 'terminal.font_reset': {
				event.preventDefault();
				resetFontSize();
				return;
			}
			default: {
				if (action.startsWith('terminal.focus_tab_')) {
					if (!layout.focusedPaneId) return;
					const index = Number.parseInt(action.replace('terminal.focus_tab_', ''), 10);
					if (!Number.isFinite(index) || index < 1 || index > 9) return;
					const pane = findPane(layout.root, layout.focusedPaneId);
					if (!pane) return;
					const tabIndex = index - 1;
					if (tabIndex < pane.tabs.length) {
						event.preventDefault();
						handleSelectTab(layout.focusedPaneId, pane.tabs[tabIndex].id);
					}
				}
				return;
			}
		}
	};

	onDestroy(() => {
		if (saveTimer) {
			window.clearTimeout(saveTimer);
		}
	});

	$effect(() => {
		if (!workspaceId) return;
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
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<!-- role="application" is a complex widget that manages its own keyboard handling -->
<section
	class="terminal-workspace"
	role="application"
	tabindex="-1"
	onkeydown={handleWorkspaceKeydown}
>
	<div class="workspace-container">
		{#if initError}
			<div class="terminal-error">
				<div class="status-text">Terminal startup error.</div>
				<div class="status-message">{initError}</div>
				<button type="button" class="retry-action" onclick={() => initWorkspace()}>Retry</button>
			</div>
		{:else if loading || !layout}
			<div class="terminal-loading">Preparing terminals…</div>
		{:else}
			{@const rootNode = layout?.root ?? null}
			{@const totalPaneCount = rootNode ? collectPaneIds(rootNode).length : 0}
			<TerminalLayoutNode
				node={rootNode}
				{workspaceId}
				{workspaceName}
				{active}
				focusedPaneId={layout?.focusedPaneId}
				{totalPaneCount}
				{dragState}
				onFocusPane={handleFocusPane}
				onSelectTab={handleSelectTab}
				onAddTab={handleAddTab}
				onSplitPane={handleSplitPane}
				onCloseTab={handleCloseTab}
				onClosePane={handleClosePane}
				onResizeSplit={handleResizeSplit}
				onTabDragStart={handleTabDragStart}
				onTabDragEnd={handleTabDragEnd}
				onTabDrop={handleTabDrop}
				onTabSplitDrop={handleTabSplitDrop}
			/>
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

	.terminal-loading,
	.terminal-error {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 16px;
		color: var(--muted);
		font-size: var(--text-sm);
	}

	.terminal-error {
		color: var(--text);
	}

	.terminal-error .status-message {
		color: var(--muted);
	}

	.retry-action {
		margin-top: 8px;
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 6px 12px;
		font-size: var(--text-sm);
		background: transparent;
		color: var(--text);
		cursor: pointer;
	}

	.retry-action:hover {
		border-color: var(--accent);
	}
</style>
