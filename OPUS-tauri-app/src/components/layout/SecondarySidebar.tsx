import { useAppStore } from '@/state/store';
import { WorkspaceList } from '@/components/pages/Spaces/WorkspaceList';
import { Plus } from 'lucide-react';
import './SecondarySidebar.css';

export function SecondarySidebar() {
  const activePage = useAppStore((s) => s.activePage);
  const openModal = useAppStore((s) => s.openModal);
  const ccSection = useAppStore((s) => s.commandCenterSection);
  const setCcSection = useAppStore((s) => s.setCommandCenterSection);
  const settingsSection = useAppStore((s) => s.settingsSection);
  const setSettingsSection = useAppStore((s) => s.setSettingsSection);

  if (activePage === 'command-center') {
    return (
      <div className="secondary-sidebar">
        <div className="secondary-sidebar__header">Command Center</div>
        <nav className="secondary-sidebar__nav">
          <button
            className={`secondary-sidebar__nav-item ${ccSection === 'overview' ? 'active' : ''}`}
            onClick={() => setCcSection('overview')}
          >Overview</button>
          <button
            className={`secondary-sidebar__nav-item ${ccSection === 'repositories' ? 'active' : ''}`}
            onClick={() => setCcSection('repositories')}
          >Repositories</button>
          <button
            className={`secondary-sidebar__nav-item ${ccSection === 'diagnostics' ? 'active' : ''}`}
            onClick={() => setCcSection('diagnostics')}
          >Diagnostics</button>
        </nav>
      </div>
    );
  }

  if (activePage === 'spaces') {
    return (
      <div className="secondary-sidebar">
        <div className="secondary-sidebar__header">
          <span>Workspaces</span>
          <button
            className="secondary-sidebar__action-btn"
            onClick={() => openModal('create-workspace')}
            title="New Workspace"
          >
            <Plus size={16} />
          </button>
        </div>
        <WorkspaceList />
      </div>
    );
  }

  return (
    <div className="secondary-sidebar">
      <div className="secondary-sidebar__header">Settings</div>
      <nav className="secondary-sidebar__nav">
        <button
          className={`secondary-sidebar__nav-item ${settingsSection === 'app' ? 'active' : ''}`}
          onClick={() => setSettingsSection('app')}
        >App Settings</button>
        <button
          className={`secondary-sidebar__nav-item ${settingsSection === 'workset' ? 'active' : ''}`}
          onClick={() => setSettingsSection('workset')}
        >Workset Settings</button>
        <button
          className={`secondary-sidebar__nav-item ${settingsSection === 'diagnostics' ? 'active' : ''}`}
          onClick={() => setSettingsSection('diagnostics')}
        >Diagnostics</button>
      </nav>
    </div>
  );
}
