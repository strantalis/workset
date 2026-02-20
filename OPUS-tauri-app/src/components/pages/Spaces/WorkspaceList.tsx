import { useEffect } from 'react';
import { useAppStore } from '@/state/store';
import { WorkspaceItem } from './WorkspaceItem';
import './WorkspaceList.css';

export function WorkspaceList() {
  const workspaces = useAppStore((s) => s.workspaces);
  const activeWorksetId = useAppStore((s) => s.activeWorksetId);
  const activeWorkspaceName = useAppStore((s) => s.activeWorkspaceName);
  const loadWorkspaces = useAppStore((s) => s.loadWorkspaces);
  const setActiveWorkspace = useAppStore((s) => s.setActiveWorkspace);
  const deleteWorkspace = useAppStore((s) => s.deleteWorkspace);

  useEffect(() => {
    if (activeWorksetId) {
      loadWorkspaces(activeWorksetId);
    }
  }, [activeWorksetId, loadWorkspaces]);

  if (!activeWorksetId) return null;

  async function handleDelete(name: string) {
    await deleteWorkspace(activeWorksetId!, name);
    if (activeWorkspaceName === name) {
      // Clear active workspace since it was deleted
      const remaining = useAppStore.getState().workspaces;
      if (remaining.length > 0) {
        setActiveWorkspace(remaining[0].name);
      }
    }
  }

  return (
    <div className="workspace-list">
      {workspaces.map((ws) => (
        <WorkspaceItem
          key={ws.name}
          workspace={ws}
          active={ws.name === activeWorkspaceName}
          onClick={() => setActiveWorkspace(ws.name)}
          onDelete={() => handleDelete(ws.name)}
        />
      ))}
      {workspaces.length === 0 && (
        <div className="workspace-list__empty">No workspaces</div>
      )}
    </div>
  );
}
