import { useEffect } from 'react';
import { useAppStore } from '@/state/store';
import { AppShell } from '@/components/layout/AppShell';
import { TopChrome } from '@/components/layout/TopChrome';
import { IconRail } from '@/components/layout/IconRail';
import { SecondarySidebar } from '@/components/layout/SecondarySidebar';
import { RightPanel } from '@/components/layout/RightPanel';
import { CommandCenterPage } from '@/components/pages/CommandCenter/CommandCenterPage';
import { SpacesPage } from '@/components/pages/Spaces/SpacesPage';
import { SettingsPage } from '@/components/pages/Settings/SettingsPage';
import { WorksetCreateModal } from '@/components/modals/WorksetCreateModal';
import { CreateWorkspaceModal } from '@/components/pages/Spaces/CreateWorkspaceModal';
import { MigrationStatusModal } from '@/components/modals/MigrationStatusModal';
import { RepoRemoveConfirmModal } from '@/components/modals/RepoRemoveConfirmModal';
import { CommandPalette } from '@/components/modals/CommandPalette';
import { ThemeProvider } from '@/styles/ThemeProvider';
import { useGlobalShortcuts } from '@/hooks/useGlobalShortcuts';
import '@/commands/appCommands';

function MainContent() {
  const activePage = useAppStore((s) => s.activePage);

  return (
    <>
      <div style={{ display: activePage === 'command-center' ? 'contents' : 'none' }}>
        <CommandCenterPage />
      </div>
      <div style={{ display: activePage === 'spaces' ? 'contents' : 'none' }}>
        <SpacesPage />
      </div>
      <div style={{ display: activePage === 'settings' ? 'contents' : 'none' }}>
        <SettingsPage />
      </div>
    </>
  );
}

export default function App() {
  const loadWorksets = useAppStore((s) => s.loadWorksets);
  const activeModal = useAppStore((s) => s.activeModal);
  const activePage = useAppStore((s) => s.activePage);

  useGlobalShortcuts();

  useEffect(() => {
    loadWorksets();
  }, [loadWorksets]);

  return (
    <>
      <ThemeProvider />
      <AppShell
        chrome={<TopChrome />}
        rail={<IconRail />}
        sidebar={<SecondarySidebar />}
        main={<MainContent />}
        rightPanel={activePage === 'spaces' ? <RightPanel /> : undefined}
      />
      {activeModal?.type === 'command-palette' && <CommandPalette />}
      {activeModal?.type === 'create-workset' && <WorksetCreateModal />}
      {activeModal?.type === 'create-workspace' && <CreateWorkspaceModal />}
      {activeModal?.type === 'migration-status' && <MigrationStatusModal />}
      {activeModal?.type === 'repo-remove-confirm' && <RepoRemoveConfirmModal />}
    </>
  );
}
