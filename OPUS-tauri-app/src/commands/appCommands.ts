import {
  LayoutGrid,
  Layers,
  Settings,
  Plus,
  Terminal,
  PanelRightClose,
  Command,
  ArrowUp,
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  X,
} from 'lucide-react';
import { registerCommands } from './registry';
import { findPane } from './layoutUtils';
import { useAppStore } from '@/state/store';

const store = () => useAppStore.getState();

registerCommands([
  // ── Navigation ──────────────────────────────────────────────
  {
    id: 'nav.command-center',
    label: 'Go to Command Center',
    category: 'navigation',
    icon: LayoutGrid,
    shortcut: { modifiers: ['meta'], key: '1' },
    execute: () => store().setActivePage('command-center'),
  },
  {
    id: 'nav.spaces',
    label: 'Go to Spaces',
    category: 'navigation',
    icon: Layers,
    shortcut: { modifiers: ['meta'], key: '2' },
    execute: () => store().setActivePage('spaces'),
  },
  {
    id: 'nav.settings',
    label: 'Go to Settings',
    category: 'navigation',
    icon: Settings,
    shortcut: { modifiers: ['meta'], key: '3' },
    execute: () => store().setActivePage('settings'),
  },

  // ── Workset ─────────────────────────────────────────────────
  {
    id: 'workset.create',
    label: 'Create New Workset',
    category: 'workset',
    icon: Plus,
    execute: () => store().openModal('create-workset'),
  },

  // ── Workspace ───────────────────────────────────────────────
  {
    id: 'workspace.create',
    label: 'Create New Workspace',
    category: 'workspace',
    icon: Plus,
    when: () => store().activeWorksetId !== null,
    execute: () => store().openModal('create-workspace'),
  },
  {
    id: 'workspace.prev',
    label: 'Previous Workspace',
    category: 'workspace',
    icon: ArrowUp,
    shortcut: { modifiers: ['meta'], key: 'arrowup' },
    when: () =>
      store().activePage === 'spaces' &&
      store().activeWorkspaceName !== null &&
      store().workspaces.length > 1,
    execute: () => {
      const s = store();
      const idx = s.workspaces.findIndex((ws) => ws.name === s.activeWorkspaceName);
      const next = (idx - 1 + s.workspaces.length) % s.workspaces.length;
      s.setActiveWorkspace(s.workspaces[next].name);
    },
  },
  {
    id: 'workspace.next',
    label: 'Next Workspace',
    category: 'workspace',
    icon: ArrowDown,
    shortcut: { modifiers: ['meta'], key: 'arrowdown' },
    when: () =>
      store().activePage === 'spaces' &&
      store().activeWorkspaceName !== null &&
      store().workspaces.length > 1,
    execute: () => {
      const s = store();
      const idx = s.workspaces.findIndex((ws) => ws.name === s.activeWorkspaceName);
      const next = (idx + 1) % s.workspaces.length;
      s.setActiveWorkspace(s.workspaces[next].name);
    },
  },

  // ── Terminal ────────────────────────────────────────────────
  {
    id: 'terminal.new',
    label: 'New Terminal Tab',
    category: 'terminal',
    icon: Terminal,
    shortcut: { modifiers: ['meta'], key: 't' },
    when: () =>
      store().activePage === 'spaces' && store().activeWorkspaceName !== null,
    execute: async () => {
      const s = store();
      if (!s.activeWorkspaceName) return;
      const paneId = s.focusedPaneId ?? 'main';
      const terminalId = await s.allocatePtySession(s.activeWorkspaceName, 'terminal');
      s.addTab(paneId, {
        id: `tab-${Date.now()}`,
        terminal_id: terminalId,
        title: 'Terminal',
        kind: 'terminal',
      });
    },
  },
  {
    id: 'terminal.close-tab',
    label: 'Close Active Tab',
    category: 'terminal',
    icon: X,
    shortcut: { modifiers: ['meta'], key: 'w' },
    when: () =>
      store().activePage === 'spaces' &&
      store().activeWorkspaceName !== null &&
      store().layout !== null,
    execute: () => {
      const s = store();
      if (!s.layout || !s.activeWorkspaceName) return;
      const paneId = s.focusedPaneId ?? 'main';
      const pane = findPane(s.layout.root, paneId);
      if (!pane || pane.tabs.length === 0) return;
      const tab = pane.tabs.find((t) => t.id === pane.active_tab_id);
      if (!tab) return;
      s.removeTab(paneId, tab.id);
      if (tab.kind !== 'diff') {
        s.closePtySession(s.activeWorkspaceName, tab.terminal_id);
      }
    },
  },
  {
    id: 'terminal.prev-tab',
    label: 'Previous Tab',
    category: 'terminal',
    icon: ArrowLeft,
    shortcut: { modifiers: ['meta'], key: 'arrowleft' },
    when: () => {
      const s = store();
      if (s.activePage !== 'spaces' || !s.layout) return false;
      const paneId = s.focusedPaneId ?? 'main';
      const pane = findPane(s.layout.root, paneId);
      return pane !== null && pane.tabs.length > 1;
    },
    execute: () => {
      const s = store();
      if (!s.layout) return;
      const paneId = s.focusedPaneId ?? 'main';
      const pane = findPane(s.layout.root, paneId);
      if (!pane || pane.tabs.length === 0) return;
      const idx = pane.tabs.findIndex((t) => t.id === pane.active_tab_id);
      const next = (idx - 1 + pane.tabs.length) % pane.tabs.length;
      s.setActiveTab(paneId, pane.tabs[next].id);
    },
  },
  {
    id: 'terminal.next-tab',
    label: 'Next Tab',
    category: 'terminal',
    icon: ArrowRight,
    shortcut: { modifiers: ['meta'], key: 'arrowright' },
    when: () => {
      const s = store();
      if (s.activePage !== 'spaces' || !s.layout) return false;
      const paneId = s.focusedPaneId ?? 'main';
      const pane = findPane(s.layout.root, paneId);
      return pane !== null && pane.tabs.length > 1;
    },
    execute: () => {
      const s = store();
      if (!s.layout) return;
      const paneId = s.focusedPaneId ?? 'main';
      const pane = findPane(s.layout.root, paneId);
      if (!pane || pane.tabs.length === 0) return;
      const idx = pane.tabs.findIndex((t) => t.id === pane.active_tab_id);
      const next = (idx + 1) % pane.tabs.length;
      s.setActiveTab(paneId, pane.tabs[next].id);
    },
  },

  // ── App ─────────────────────────────────────────────────────
  {
    id: 'app.toggle-right-panel',
    label: 'Toggle Right Panel',
    category: 'app',
    icon: PanelRightClose,
    shortcut: { modifiers: ['meta'], key: 'b' },
    when: () => store().activePage === 'spaces',
    execute: () => store().toggleRightPanel(),
  },
  {
    id: 'app.command-palette',
    label: 'Command Palette',
    category: 'app',
    icon: Command,
    shortcut: { modifiers: ['meta'], key: 'k' },
    execute: () => {
      const s = store();
      if (s.activeModal?.type === 'command-palette') {
        s.closeModal();
      } else {
        s.openModal('command-palette');
      }
    },
  },
]);
