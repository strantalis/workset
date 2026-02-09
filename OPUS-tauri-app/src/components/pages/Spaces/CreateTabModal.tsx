import { useAppStore } from '@/state/store';
import { Button } from '@/components/ui/Button';
import { Terminal, Bot } from 'lucide-react';
import '@/components/modals/Modal.css';
import './CreateTabModal.css';

export function CreateTabModal() {
  const activeModal = useAppStore((s) => s.activeModal);
  const closeModal = useAppStore((s) => s.closeModal);
  const createPtySession = useAppStore((s) => s.createPtySession);
  const addTab = useAppStore((s) => s.addTab);

  const paneId = (activeModal?.props?.paneId as string) ?? 'main';
  const workspaceName = (activeModal?.props?.workspaceName as string) ?? '';

  async function handleCreate(kind: 'terminal' | 'agent') {
    if (!workspaceName) return;
    // TODO: resolve actual workspace cwd from repos
    const cwd = '/';
    const terminalId = await createPtySession(workspaceName, kind, cwd);
    const tabId = `tab-${Date.now()}`;
    addTab(paneId, {
      id: tabId,
      terminal_id: terminalId,
      title: kind === 'agent' ? 'Agent' : 'Terminal',
      kind,
    });
    closeModal();
  }

  return (
    <div className="modal-overlay" onClick={closeModal}>
      <div className="modal-card modal-card--narrow" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">New Tab</div>
        <div className="modal-body">
          <div className="create-tab-options">
            <button className="create-tab-option" onClick={() => handleCreate('terminal')}>
              <Terminal size={20} />
              <div className="create-tab-option__text">
                <span className="create-tab-option__title">Terminal</span>
                <span className="create-tab-option__desc">Shell session in workspace</span>
              </div>
            </button>
            <button className="create-tab-option" onClick={() => handleCreate('agent')}>
              <Bot size={20} />
              <div className="create-tab-option__text">
                <span className="create-tab-option__title">Agent</span>
                <span className="create-tab-option__desc">AI coding assistant</span>
              </div>
            </button>
          </div>
        </div>
        <div className="modal-footer">
          <Button variant="ghost" onClick={closeModal}>Cancel</Button>
        </div>
      </div>
    </div>
  );
}
