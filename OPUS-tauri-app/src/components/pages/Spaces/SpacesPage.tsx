import { useEffect, useCallback } from 'react';
import { useAppStore } from '@/state/store';
import type { LayoutNode, PaneNode } from '@/types/layout';
import { EmptyState } from '@/components/ui/EmptyState';
import { Button } from '@/components/ui/Button';
import { TerminalLayoutNodeView } from './TerminalLayoutNode';
import { Layers } from 'lucide-react';
import './SpacesPage.css';

function findPane(node: LayoutNode, paneId: string): PaneNode | null {
  if (node.kind === 'pane') return node.id === paneId ? node : null;
  return findPane(node.first, paneId) ?? findPane(node.second, paneId);
}

export function SpacesPage() {
  const activeWorksetId = useAppStore((s) => s.activeWorksetId);
  const activeWorkspaceName = useAppStore((s) => s.activeWorkspaceName);
  const workspaces = useAppStore((s) => s.workspaces);
  const layout = useAppStore((s) => s.layout);
  const loadLayout = useAppStore((s) => s.loadLayout);
  const saveLayout = useAppStore((s) => s.saveLayout);
  const openModal = useAppStore((s) => s.openModal);
  const worksets = useAppStore((s) => s.worksets);
  const setActiveTab = useAppStore((s) => s.setActiveTab);
  const focusedPaneId = useAppStore((s) => s.focusedPaneId);
  const addTab = useAppStore((s) => s.addTab);
  const removeTab = useAppStore((s) => s.removeTab);
  const allocatePtySession = useAppStore((s) => s.allocatePtySession);
  const closePtySession = useAppStore((s) => s.closePtySession);
  const setActiveWorkspace = useAppStore((s) => s.setActiveWorkspace);

  // Load layout when workspace changes
  useEffect(() => {
    if (activeWorkspaceName) {
      loadLayout(activeWorkspaceName);
    }
  }, [activeWorkspaceName, loadLayout]);

  // Auto-save layout on changes
  useEffect(() => {
    if (activeWorkspaceName && layout) {
      const timer = setTimeout(() => {
        saveLayout(activeWorkspaceName);
      }, 1000);
      return () => clearTimeout(timer);
    }
  }, [activeWorkspaceName, layout, saveLayout]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (!e.metaKey || !activeWorkspaceName) return;

      // Cmd+Up / Cmd+Down — switch workspaces
      if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
        if (workspaces.length < 2) return;
        e.preventDefault();
        const idx = workspaces.findIndex((ws) => ws.name === activeWorkspaceName);
        const next =
          e.key === 'ArrowUp'
            ? (idx - 1 + workspaces.length) % workspaces.length
            : (idx + 1) % workspaces.length;
        setActiveWorkspace(workspaces[next].name);
        return;
      }

      if (!layout) return;
      const paneId = focusedPaneId ?? 'main';
      const pane = findPane(layout.root, paneId);
      if (!pane || pane.tabs.length === 0) return;

      // Cmd+Left / Cmd+Right — switch tabs
      if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
        e.preventDefault();
        const idx = pane.tabs.findIndex((t) => t.id === pane.active_tab_id);
        const next =
          e.key === 'ArrowLeft'
            ? (idx - 1 + pane.tabs.length) % pane.tabs.length
            : (idx + 1) % pane.tabs.length;
        setActiveTab(paneId, pane.tabs[next].id);
        return;
      }

      // Cmd+T — new terminal
      if (e.key === 't') {
        e.preventDefault();
        allocatePtySession(activeWorkspaceName, 'terminal')
          .then((terminalId) => {
            addTab(paneId, {
              id: `tab-${Date.now()}`,
              terminal_id: terminalId,
              title: 'Terminal',
              kind: 'terminal',
            });
          })
          .catch(() => {});
        return;
      }

      // Cmd+W — close active tab
      if (e.key === 'w') {
        e.preventDefault();
        const tab = pane.tabs.find((t) => t.id === pane.active_tab_id);
        if (!tab) return;
        removeTab(paneId, tab.id);
        if (tab.kind !== 'diff') {
          closePtySession(activeWorkspaceName, tab.terminal_id);
        }
      }
    },
    [layout, activeWorkspaceName, workspaces, focusedPaneId, setActiveTab,
     setActiveWorkspace, allocatePtySession, addTab, removeTab, closePtySession],
  );

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);

  if (worksets.length === 0) {
    return (
      <EmptyState
        icon={<Layers size={32} />}
        title="Create your first workset"
        description="A workset groups repos together. Create one to get started."
        action={
          <Button variant="primary" onClick={() => openModal('create-workset')}>
            New Workset
          </Button>
        }
      />
    );
  }

  if (!activeWorksetId) {
    return (
      <EmptyState
        icon={<Layers size={32} />}
        title="Select a workset to get started"
        description="Use the workset selector in the top bar to choose or create a workset."
      />
    );
  }

  if (workspaces.length === 0) {
    return (
      <EmptyState
        icon={<Layers size={32} />}
        title="Create your first workspace"
        description="A workspace is a named work thread across all repos in this workset."
        action={
          <Button variant="primary" onClick={() => openModal('create-workspace')}>
            New Workspace
          </Button>
        }
      />
    );
  }

  if (!activeWorkspaceName) {
    return (
      <EmptyState
        icon={<Layers size={32} />}
        title="Select a workspace"
        description="Choose a workspace from the sidebar to begin."
      />
    );
  }

  if (!layout) {
    return (
      <div className="spaces-page spaces-page--loading">
        <span>Loading layout...</span>
      </div>
    );
  }

  return (
    <div className="spaces-page">
      <TerminalLayoutNodeView node={layout.root} workspaceName={activeWorkspaceName} />
    </div>
  );
}
