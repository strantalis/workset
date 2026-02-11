import type { StateCreator } from 'zustand';
import type { TerminalLayout, LayoutNode, LayoutTab, PaneNode, SplitNode } from '@/types/layout';
import { layoutGet, layoutSave } from '@/api/pty';
import { findPane, findParentSplit, replaceNodeInTree } from '@/commands/layoutUtils';

export type LayoutSlice = {
  layout: TerminalLayout | null;
  focusedPaneId: string | null;
  loadLayout: (workspaceName: string) => Promise<void>;
  saveLayout: (workspaceName: string) => Promise<void>;
  setLayout: (layout: TerminalLayout) => void;
  addTab: (paneId: string, tab: LayoutTab) => void;
  removeTab: (paneId: string, tabId: string) => void;
  setActiveTab: (paneId: string, tabId: string) => void;
  setFocusedPane: (paneId: string) => void;
  updateTabTitle: (terminalId: string, title: string) => void;
  closeTabByTerminalId: (terminalId: string) => void;
  splitPane: (paneId: string, direction: 'row' | 'column') => string;
  closePane: (paneId: string) => void;
  setSplitRatio: (splitId: string, ratio: number) => void;
  moveTab: (fromPaneId: string, toPaneId: string, tabId: string) => void;
  reorderTab: (paneId: string, tabId: string, newIndex: number) => void;
  renameTab: (paneId: string, tabId: string, title: string) => void;
};

function findAndUpdatePane(
  node: LayoutNode,
  paneId: string,
  updater: (pane: PaneNode) => PaneNode,
): LayoutNode {
  if (node.kind === 'pane') {
    if (node.id === paneId) {
      return updater(node);
    }
    return node;
  }
  return {
    ...node,
    first: findAndUpdatePane(node.first, paneId, updater),
    second: findAndUpdatePane(node.second, paneId, updater),
  };
}

function updateTabInNode(
  node: LayoutNode,
  terminalId: string,
  updater: (tab: LayoutTab) => LayoutTab,
): LayoutNode {
  if (node.kind === 'pane') {
    const updated = node.tabs.map((t) =>
      t.terminal_id === terminalId ? updater(t) : t,
    );
    if (updated === node.tabs) return node;
    return { ...node, tabs: updated };
  }
  return {
    ...node,
    first: updateTabInNode(node.first, terminalId, updater),
    second: updateTabInNode(node.second, terminalId, updater),
  };
}

function findTabLocation(
  node: LayoutNode,
  terminalId: string,
): { paneId: string; tabId: string } | null {
  if (node.kind === 'pane') {
    const tab = node.tabs.find((t) => t.terminal_id === terminalId);
    return tab ? { paneId: node.id, tabId: tab.id } : null;
  }
  return findTabLocation(node.first, terminalId) ?? findTabLocation(node.second, terminalId);
}

function defaultLayout(): TerminalLayout {
  return {
    version: 1,
    root: {
      kind: 'pane',
      id: 'main',
      tabs: [],
      active_tab_id: undefined,
    },
    focused_pane_id: 'main',
  };
}

export const createLayoutSlice: StateCreator<LayoutSlice, [], [], LayoutSlice> = (set, get) => ({
  layout: null,
  focusedPaneId: null,

  loadLayout: async (workspaceName) => {
    try {
      const saved = await layoutGet(workspaceName);
      const layout = saved ?? defaultLayout();
      set({ layout, focusedPaneId: layout.focused_pane_id ?? 'main' });
    } catch {
      set({ layout: defaultLayout(), focusedPaneId: 'main' });
    }
  },

  saveLayout: async (workspaceName) => {
    const { layout } = get();
    if (layout) {
      await layoutSave(workspaceName, layout).catch(() => {});
    }
  },

  setLayout: (layout) => set({ layout }),

  addTab: (paneId, tab) => {
    const { layout } = get();
    if (!layout) return;
    const newRoot = findAndUpdatePane(layout.root, paneId, (pane) => ({
      ...pane,
      tabs: [...pane.tabs, tab],
      active_tab_id: tab.id,
    }));
    set({ layout: { ...layout, root: newRoot } });
  },

  removeTab: (paneId, tabId) => {
    const { layout } = get();
    if (!layout) return;
    const newRoot = findAndUpdatePane(layout.root, paneId, (pane) => {
      const tabs = pane.tabs.filter((t) => t.id !== tabId);
      const activeId =
        pane.active_tab_id === tabId
          ? tabs[tabs.length - 1]?.id
          : pane.active_tab_id;
      return { ...pane, tabs, active_tab_id: activeId };
    });
    set({ layout: { ...layout, root: newRoot } });

    // Auto-collapse empty panes (unless it's the root)
    const updatedPane = findPane(newRoot, paneId);
    if (updatedPane && updatedPane.tabs.length === 0 && newRoot.id !== paneId) {
      get().closePane(paneId);
    }
  },

  setActiveTab: (paneId, tabId) => {
    const { layout } = get();
    if (!layout) return;
    const newRoot = findAndUpdatePane(layout.root, paneId, (pane) => ({
      ...pane,
      active_tab_id: tabId,
    }));
    set({ layout: { ...layout, root: newRoot } });
  },

  setFocusedPane: (paneId) => {
    const { layout } = get();
    if (!layout) return;
    set({
      layout: { ...layout, focused_pane_id: paneId },
      focusedPaneId: paneId,
    });
  },

  updateTabTitle: (terminalId, title) => {
    const { layout } = get();
    if (!layout) return;
    const newRoot = updateTabInNode(layout.root, terminalId, (tab) => ({
      ...tab,
      title,
    }));
    set({ layout: { ...layout, root: newRoot } });
  },

  closeTabByTerminalId: (terminalId) => {
    const { layout } = get();
    if (!layout) return;
    const loc = findTabLocation(layout.root, terminalId);
    if (!loc) return;
    get().removeTab(loc.paneId, loc.tabId);
  },

  splitPane: (paneId, direction) => {
    const { layout } = get();
    if (!layout) return paneId;
    const pane = findPane(layout.root, paneId);
    if (!pane) return paneId;
    const newPaneId = `pane-${crypto.randomUUID().slice(0, 8)}`;
    const newPane: PaneNode = { kind: 'pane', id: newPaneId, tabs: [], active_tab_id: undefined };
    const split: SplitNode = {
      kind: 'split',
      id: `split-${crypto.randomUUID().slice(0, 8)}`,
      direction,
      ratio: 0.5,
      first: pane,
      second: newPane,
    };
    const newRoot = replaceNodeInTree(layout.root, paneId, split);
    set({ layout: { ...layout, root: newRoot }, focusedPaneId: newPaneId });
    return newPaneId;
  },

  closePane: (paneId) => {
    const { layout, focusedPaneId } = get();
    if (!layout) return;
    // Can't close root pane
    if (layout.root.id === paneId) return;
    const result = findParentSplit(layout.root, paneId);
    if (!result) return;
    const sibling = result.side === 'first' ? result.parent.second : result.parent.first;
    const newRoot = replaceNodeInTree(layout.root, result.parent.id, sibling);
    // If focused pane was the closed one, focus the first pane in the sibling
    const newFocused = focusedPaneId === paneId
      ? (sibling.kind === 'pane' ? sibling.id : findFirstPaneId(sibling))
      : focusedPaneId;
    set({
      layout: { ...layout, root: newRoot, focused_pane_id: newFocused ?? undefined },
      focusedPaneId: newFocused,
    });
  },

  setSplitRatio: (splitId, ratio) => {
    const { layout } = get();
    if (!layout) return;
    const clamped = Math.min(0.85, Math.max(0.15, ratio));
    const newRoot = updateSplitRatio(layout.root, splitId, clamped);
    set({ layout: { ...layout, root: newRoot } });
  },

  moveTab: (fromPaneId, toPaneId, tabId) => {
    const { layout } = get();
    if (!layout) return;
    if (fromPaneId === toPaneId) return;
    const srcPane = findPane(layout.root, fromPaneId);
    if (!srcPane) return;
    const tab = srcPane.tabs.find((t) => t.id === tabId);
    if (!tab) return;
    let newRoot = findAndUpdatePane(layout.root, fromPaneId, (pane) => {
      const tabs = pane.tabs.filter((t) => t.id !== tabId);
      const activeId = pane.active_tab_id === tabId ? tabs[tabs.length - 1]?.id : pane.active_tab_id;
      return { ...pane, tabs, active_tab_id: activeId };
    });
    newRoot = findAndUpdatePane(newRoot, toPaneId, (pane) => ({
      ...pane,
      tabs: [...pane.tabs, tab],
      active_tab_id: tab.id,
    }));
    set({ layout: { ...layout, root: newRoot }, focusedPaneId: toPaneId });
  },

  reorderTab: (paneId, tabId, newIndex) => {
    const { layout } = get();
    if (!layout) return;
    const newRoot = findAndUpdatePane(layout.root, paneId, (pane) => {
      const tabs = [...pane.tabs];
      const oldIndex = tabs.findIndex((t) => t.id === tabId);
      if (oldIndex === -1 || oldIndex === newIndex) return pane;
      const [tab] = tabs.splice(oldIndex, 1);
      tabs.splice(newIndex, 0, tab);
      return { ...pane, tabs };
    });
    set({ layout: { ...layout, root: newRoot } });
  },

  renameTab: (paneId, tabId, title) => {
    const { layout } = get();
    if (!layout) return;
    const newRoot = findAndUpdatePane(layout.root, paneId, (pane) => ({
      ...pane,
      tabs: pane.tabs.map((t) => (t.id === tabId ? { ...t, title } : t)),
    }));
    set({ layout: { ...layout, root: newRoot } });
  },

});

function findFirstPaneId(node: LayoutNode): string {
  if (node.kind === 'pane') return node.id;
  return findFirstPaneId(node.first);
}

function updateSplitRatio(node: LayoutNode, splitId: string, ratio: number): LayoutNode {
  if (node.kind === 'pane') return node;
  if (node.id === splitId) return { ...node, ratio };
  return {
    ...node,
    first: updateSplitRatio(node.first, splitId, ratio),
    second: updateSplitRatio(node.second, splitId, ratio),
  };
}
