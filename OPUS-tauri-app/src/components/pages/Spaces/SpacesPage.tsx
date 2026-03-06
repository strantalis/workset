import { useEffect } from 'react';
import { useAppStore } from '@/state/store';
import { EmptyState } from '@/components/ui/EmptyState';
import { Button } from '@/components/ui/Button';
import { TerminalLayoutNodeView } from './TerminalLayoutNode';
import { Layers } from 'lucide-react';
import './SpacesPage.css';

export function SpacesPage() {
  const activeWorksetId = useAppStore((s) => s.activeWorksetId);
  const activeWorkspaceName = useAppStore((s) => s.activeWorkspaceName);
  const workspaces = useAppStore((s) => s.workspaces);
  const layout = useAppStore((s) => s.layout);
  const loadLayout = useAppStore((s) => s.loadLayout);
  const saveLayout = useAppStore((s) => s.saveLayout);
  const openModal = useAppStore((s) => s.openModal);
  const worksets = useAppStore((s) => s.worksets);

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
