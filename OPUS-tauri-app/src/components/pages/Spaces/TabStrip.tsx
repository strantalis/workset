import type { LayoutTab } from '@/types/layout';
import { useAppStore } from '@/state/store';
import { Terminal, Bot, GitCompare, Plus } from 'lucide-react';
import './TabStrip.css';

type Props = {
  paneId: string;
  tabs: LayoutTab[];
  activeTabId?: string;
  workspaceName: string;
};

const tabIcons: Record<string, typeof Terminal> = {
  terminal: Terminal,
  agent: Bot,
  diff: GitCompare,
};

export function TabStrip({ paneId, tabs, activeTabId, workspaceName }: Props) {
  const setActiveTab = useAppStore((s) => s.setActiveTab);
  const removeTab = useAppStore((s) => s.removeTab);
  const closePtySession = useAppStore((s) => s.closePtySession);
  const allocatePtySession = useAppStore((s) => s.allocatePtySession);
  const addTab = useAppStore((s) => s.addTab);

  function handleClose(e: React.MouseEvent, tab: LayoutTab) {
    e.stopPropagation();
    removeTab(paneId, tab.id);
    if (tab.kind !== 'diff') {
      closePtySession(workspaceName, tab.terminal_id);
    }
  }

  async function handleAddTerminal() {
    if (!workspaceName) return;
    try {
      const terminalId = await allocatePtySession(workspaceName, 'terminal');
      const tabId = `tab-${Date.now()}`;
      addTab(paneId, {
        id: tabId,
        terminal_id: terminalId,
        title: 'Terminal',
        kind: 'terminal',
      });
      // TerminalSurface handles starting the PTY session after it mounts
    } catch (err: unknown) {
      const msg = typeof err === 'object' && err !== null && 'message' in err
        ? String((err as { message: string }).message)
        : 'Failed to create terminal session';
      console.error('Terminal creation failed:', msg);
    }
  }

  return (
    <div className="tab-strip">
      <div className="tab-strip__tabs">
        {tabs.map((tab) => {
          const Icon = tabIcons[tab.kind] ?? Terminal;
          const isActive = tab.id === activeTabId;
          return (
            <div
              key={tab.id}
              className={`tab-strip__tab ${isActive ? 'active' : ''}`}
              role="tab"
              onClick={() => setActiveTab(paneId, tab.id)}
            >
              <Icon size={13} className="tab-strip__icon" />
              <span className="tab-strip__label">{tab.title}</span>
              <button
                className="tab-strip__close"
                onClick={(e) => handleClose(e, tab)}
                aria-label="Close tab"
              >
                ×
              </button>
            </div>
          );
        })}
      </div>
      <button
        className="tab-strip__add"
        onClick={handleAddTerminal}
        title="New Terminal"
      >
        <Plus size={14} />
      </button>
    </div>
  );
}
