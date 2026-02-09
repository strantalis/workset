import type { StateCreator } from 'zustand';
import type { TerminalLayout, LayoutNode, LayoutTab, PaneNode } from '@/types/layout';
import { layoutGet, layoutSave } from '@/api/pty';

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
});
