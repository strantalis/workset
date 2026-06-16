import { useAppStore } from '@/state/store';
import { DiffNavigator } from '@/components/diff/DiffNavigator';
import './RightPanel.css';

export function RightPanel() {
  const activeWorkspaceName = useAppStore((s) => s.activeWorkspaceName);

  if (!activeWorkspaceName) {
    return (
      <div className="right-panel">
        <div className="right-panel__header">
          <span className="right-panel__title">Diff</span>
        </div>
        <div className="right-panel__empty">
          <span>No workspace selected</span>
        </div>
      </div>
    );
  }

  return (
    <div className="right-panel">
      <DiffNavigator />
    </div>
  );
}
