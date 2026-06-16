import { useState, useCallback } from 'react';
import type { MigrationProgress } from '@/types/jobs';
import { useAppStore } from '@/state/store';
import { useTauriEvent } from '@/hooks/useTauriEvent';
import { migrationCancel, migrationStart } from '@/api/migrations';
import { Button } from '@/components/ui/Button';
import { CheckCircle, XCircle, Loader, Circle, RotateCcw } from 'lucide-react';
import './Modal.css';
import './MigrationStatusModal.css';

export function MigrationStatusModal() {
  const activeModal = useAppStore((s) => s.activeModal);
  const closeModal = useAppStore((s) => s.closeModal);
  const openModal = useAppStore((s) => s.openModal);

  const jobId = (activeModal?.props?.jobId as string) ?? '';
  const worksetId = (activeModal?.props?.worksetId as string) ?? '';
  const repoUrl = (activeModal?.props?.repoUrl as string) ?? '';
  const action = (activeModal?.props?.action as 'add' | 'remove') ?? 'add';
  const deleteWorktrees = activeModal?.props?.deleteWorktrees as boolean | undefined;
  const deleteLocal = activeModal?.props?.deleteLocal as boolean | undefined;

  const [progress, setProgress] = useState<MigrationProgress | null>(null);
  const [retrying, setRetrying] = useState(false);

  const handleProgress = useCallback(
    (payload: MigrationProgress) => {
      if (payload.job_id === jobId) {
        setProgress(payload);
      }
    },
    [jobId],
  );
  useTauriEvent<MigrationProgress>('migration:progress', handleProgress);

  const isDone = progress?.state === 'done' || progress?.state === 'failed' || progress?.state === 'canceled';
  const hasFailures = progress?.workspaces.some((ws) => ws.state === 'failed') ?? false;

  async function handleRetryFailed() {
    if (!progress || !worksetId || !repoUrl) return;
    const failedNames = progress.workspaces
      .filter((ws) => ws.state === 'failed')
      .map((ws) => ws.workspace_name);
    if (failedNames.length === 0) return;

    setRetrying(true);
    try {
      const { job_id } = await migrationStart({
        worksetId,
        repoUrl,
        action,
        workspaceNames: failedNames,
        deleteWorktrees,
        deleteLocal,
      });
      // Reopen modal with the new job
      openModal('migration-status', {
        jobId: job_id,
        worksetId,
        repoUrl,
        action,
        deleteWorktrees,
        deleteLocal,
      });
    } finally {
      setRetrying(false);
    }
  }

  async function handleRetryWorkspace(wsName: string) {
    if (!worksetId || !repoUrl) return;
    setRetrying(true);
    try {
      const { job_id } = await migrationStart({
        worksetId,
        repoUrl,
        action,
        workspaceNames: [wsName],
        deleteWorktrees,
        deleteLocal,
      });
      openModal('migration-status', {
        jobId: job_id,
        worksetId,
        repoUrl,
        action,
        deleteWorktrees,
        deleteLocal,
      });
    } finally {
      setRetrying(false);
    }
  }

  function statusIcon(state: string) {
    switch (state) {
      case 'success': return <CheckCircle size={14} className="migration-icon--success" />;
      case 'failed': return <XCircle size={14} className="migration-icon--failed" />;
      case 'running': return <Loader size={14} className="migration-icon--running" />;
      default: return <Circle size={14} className="migration-icon--pending" />;
    }
  }

  return (
    <div className="modal-overlay" onClick={closeModal}>
      <div className="modal-card modal-card--wide" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">Migration Progress</div>
        <div className="modal-body">
          {!progress ? (
            <div className="migration-loading">Waiting for progress...</div>
          ) : (
            <div className="migration-list">
              {progress.workspaces.map((ws) => (
                <div key={ws.workspace_name} className={`migration-item ${ws.state === 'failed' ? 'migration-item--failed' : ''}`}>
                  {statusIcon(ws.state)}
                  <span className="migration-item__name">{ws.workspace_name}</span>
                  <span className="migration-item__state">{ws.state}</span>
                  {ws.state === 'failed' && isDone && (
                    <button
                      className="migration-item__retry"
                      onClick={() => handleRetryWorkspace(ws.workspace_name)}
                      disabled={retrying}
                      title="Retry this workspace"
                    >
                      <RotateCcw size={12} />
                    </button>
                  )}
                  {ws.error && (
                    <div className="migration-item__error">{ws.error.message}</div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="modal-footer">
          {!isDone && (
            <Button variant="danger" onClick={() => migrationCancel(jobId)}>
              Cancel
            </Button>
          )}
          {isDone && hasFailures && (
            <Button variant="secondary" onClick={handleRetryFailed} disabled={retrying}>
              {retrying ? 'Retrying...' : 'Retry Failed'}
            </Button>
          )}
          <Button variant={isDone ? 'primary' : 'ghost'} onClick={closeModal}>
            {isDone ? 'Done' : 'Close'}
          </Button>
        </div>
      </div>
    </div>
  );
}
