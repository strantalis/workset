import { useState } from 'react';
import { useAppStore } from '@/state/store';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import '@/components/modals/Modal.css';

export function CreateWorkspaceModal() {
  const activeWorksetId = useAppStore((s) => s.activeWorksetId);
  const worksets = useAppStore((s) => s.worksets);
  const createWorkspace = useAppStore((s) => s.createWorkspace);
  const setActiveWorkspace = useAppStore((s) => s.setActiveWorkspace);
  const loadLayout = useAppStore((s) => s.loadLayout);
  const allocatePtySession = useAppStore((s) => s.allocatePtySession);
  const addTab = useAppStore((s) => s.addTab);
  const closeModal = useAppStore((s) => s.closeModal);
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const activeWorkset = worksets.find((w) => w.id === activeWorksetId);

  async function handleCreate() {
    if (!name.trim() || !activeWorksetId) return;
    setLoading(true);
    setError(null);
    try {
      const wsName = name.trim();
      await createWorkspace(activeWorksetId, wsName);
      await setActiveWorkspace(wsName);
      await loadLayout(wsName);

      // Open an initial terminal tab
      try {
        const terminalId = allocatePtySession(wsName, 'terminal');
        addTab('main', {
          id: `tab-${Date.now()}`,
          terminal_id: terminalId,
          title: 'Terminal',
          kind: 'terminal',
        });
      } catch {
        // Non-fatal — workspace is created even if terminal setup fails
      }

      closeModal();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message
        : typeof err === 'object' && err !== null && 'message' in err ? String((err as { message: string }).message)
        : 'Failed to create workspace';
      setError(msg);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="modal-overlay" onClick={closeModal}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">Create Workspace</div>
        <div className="modal-body">
          <label className="modal-label">Workspace Name</label>
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. feature/search-ranking"
            autoFocus
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
          />
          {activeWorkset && activeWorkset.repos.length > 0 && (
            <div style={{ marginTop: 12 }}>
              <label className="modal-label">Repos to include</label>
              <div style={{ fontSize: 12, color: 'var(--muted)', marginTop: 4 }}>
                {activeWorkset.repos.map((r) => (
                  <div key={r} style={{ fontFamily: 'var(--font-mono)', fontSize: 11, padding: '2px 0' }}>
                    {r}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
        {error && (
          <div className="modal-error" style={{ color: 'var(--danger, #ef4444)', padding: '0 20px', fontSize: 13 }}>
            {error}
          </div>
        )}
        <div className="modal-footer">
          <Button variant="ghost" onClick={closeModal}>Cancel</Button>
          <Button variant="primary" onClick={handleCreate} disabled={!name.trim() || loading}>
            {loading ? 'Creating...' : 'Create'}
          </Button>
        </div>
      </div>
    </div>
  );
}
