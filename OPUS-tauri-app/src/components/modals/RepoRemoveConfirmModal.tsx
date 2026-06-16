import { useState } from 'react';
import { useAppStore } from '@/state/store';
import { migrationStart } from '@/api/migrations';
import { listWorkspaces } from '@/api/workspaces';
import { Button } from '@/components/ui/Button';
import './Modal.css';

export function RepoRemoveConfirmModal() {
  const activeModal = useAppStore((s) => s.activeModal);
  const closeModal = useAppStore((s) => s.closeModal);
  const openModal = useAppStore((s) => s.openModal);
  const removeWorksetRepo = useAppStore((s) => s.removeWorksetRepo);

  const worksetId = (activeModal?.props?.worksetId as string) ?? '';
  const repoUrl = (activeModal?.props?.repoUrl as string) ?? '';

  const [deleteWorktrees, setDeleteWorktrees] = useState(true);
  const [deleteLocal, setDeleteLocal] = useState(false);
  const [removing, setRemoving] = useState(false);

  async function handleConfirm() {
    if (!worksetId || !repoUrl) return;
    setRemoving(true);
    try {
      await removeWorksetRepo(repoUrl);
      const freshWorkspaces = await listWorkspaces(worksetId);
      const wsNames = freshWorkspaces.map((ws) => ws.name);
      if (wsNames.length > 0) {
        const { job_id } = await migrationStart({
          worksetId,
          repoUrl,
          action: 'remove',
          workspaceNames: wsNames,
          deleteWorktrees,
          deleteLocal,
        });
        openModal('migration-status', {
          jobId: job_id,
          worksetId,
          repoUrl,
          action: 'remove',
          deleteWorktrees,
          deleteLocal,
        });
      } else {
        closeModal();
      }
    } finally {
      setRemoving(false);
    }
  }

  return (
    <div className="modal-overlay" onClick={closeModal}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">Remove Repository</div>
        <div className="modal-body">
          <p className="remove-confirm__description">
            Remove <strong>{repoUrl}</strong> from all workspaces?
          </p>
          <label className="remove-confirm__checkbox">
            <input
              type="checkbox"
              checked={deleteWorktrees}
              onChange={(e) => setDeleteWorktrees(e.target.checked)}
            />
            Delete worktrees
          </label>
          <label className="remove-confirm__checkbox">
            <input
              type="checkbox"
              checked={deleteLocal}
              onChange={(e) => setDeleteLocal(e.target.checked)}
            />
            Delete local branches
          </label>
        </div>
        <div className="modal-footer">
          <Button variant="ghost" onClick={closeModal}>Cancel</Button>
          <Button variant="danger" onClick={handleConfirm} disabled={removing}>
            {removing ? 'Removing...' : 'Remove'}
          </Button>
        </div>
      </div>
    </div>
  );
}
