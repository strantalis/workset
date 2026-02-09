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

function MainContent() {
  const activePage = useAppStore((s) => s.activePage);

  switch (activePage) {
    case 'command-center':
      return <CommandCenterPage />;
    case 'spaces':
      return <SpacesPage />;
    case 'settings':
      return <SettingsPage />;
  }
}

export default function App() {
  const loadWorksets = useAppStore((s) => s.loadWorksets);
  const activeModal = useAppStore((s) => s.activeModal);
  const activePage = useAppStore((s) => s.activePage);

  useEffect(() => {
    loadWorksets();
  }, [loadWorksets]);

  return (
    <>
      <AppShell
        chrome={<TopChrome />}
        rail={<IconRail />}
        sidebar={<SecondarySidebar />}
        main={<MainContent />}
        rightPanel={activePage === 'spaces' ? <RightPanel /> : undefined}
      />
      {activeModal?.type === 'create-workset' && <WorksetCreateModal />}
      {activeModal?.type === 'create-workspace' && <CreateWorkspaceModal />}
      {activeModal?.type === 'migration-status' && <MigrationStatusModal />}
      {activeModal?.type === 'repo-remove-confirm' && <RepoRemoveConfirmModal />}
    </>
  );
}
