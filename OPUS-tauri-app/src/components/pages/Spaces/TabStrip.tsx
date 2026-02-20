import { useState, useEffect, useRef, useCallback } from 'react';
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

// ── Module-level drag state shared across all TabStrip instances ──

type DragInfo = { tabId: string; fromPaneId: string };
type DropTarget = { paneId: string; index: number };

let activeDrag: DragInfo | null = null;
let activeDropTarget: DropTarget | null = null;

type StripEntry = {
  element: HTMLDivElement;
  tabsContainer: HTMLDivElement;
  setDropIdx: (n: number | null) => void;
};
const stripRegistry = new Map<string, StripEntry>();

function getTabDropIndex(tabsEl: HTMLDivElement, x: number): number {
  const tabEls = Array.from(tabsEl.querySelectorAll('[role="tab"]')) as HTMLElement[];
  for (let i = 0; i < tabEls.length; i++) {
    const rect = tabEls[i].getBoundingClientRect();
    if (x < rect.left + rect.width / 2) return i;
  }
  return tabEls.length;
}

function findStripAtPoint(x: number, y: number): { paneId: string; entry: StripEntry } | null {
  for (const [paneId, entry] of stripRegistry) {
    const rect = entry.element.getBoundingClientRect();
    if (x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom) {
      return { paneId, entry };
    }
  }
  return null;
}

function clearAllDropIndicators() {
  stripRegistry.forEach(({ setDropIdx }) => setDropIdx(null));
}

// Floating ghost element that follows the cursor during drag
let ghostEl: HTMLDivElement | null = null;

function createGhost(sourceTab: HTMLElement) {
  const ghost = document.createElement('div');
  ghost.className = 'tab-strip__ghost';

  // Clone the tab content (icon + label) but not the close button
  const icon = sourceTab.querySelector('.tab-strip__icon');
  const label = sourceTab.querySelector('.tab-strip__label');
  if (icon) ghost.appendChild(icon.cloneNode(true));
  if (label) ghost.appendChild(label.cloneNode(true));

  document.body.appendChild(ghost);
  ghostEl = ghost;
}

function moveGhost(x: number, y: number) {
  if (!ghostEl) return;
  ghostEl.style.left = `${x + 12}px`;
  ghostEl.style.top = `${y - 14}px`;
}

function removeGhost() {
  if (ghostEl) {
    ghostEl.remove();
    ghostEl = null;
  }
}

// ── Component ──

export function TabStrip({ paneId, tabs, activeTabId, workspaceName }: Props) {
  const setActiveTab = useAppStore((s) => s.setActiveTab);
  const removeTab = useAppStore((s) => s.removeTab);
  const closePtySession = useAppStore((s) => s.closePtySession);
  const allocatePtySession = useAppStore((s) => s.allocatePtySession);
  const addTab = useAppStore((s) => s.addTab);
  const moveTab = useAppStore((s) => s.moveTab);
  const reorderTab = useAppStore((s) => s.reorderTab);
  const renameTab = useAppStore((s) => s.renameTab);
  const splitPane = useAppStore((s) => s.splitPane);

  const [dropIdx, setDropIdx] = useState<number | null>(null);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; tab: LayoutTab } | null>(null);
  const [renamingTabId, setRenamingTabId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const renameInputRef = useRef<HTMLInputElement>(null);
  const stripRef = useRef<HTMLDivElement>(null);
  const tabsRef = useRef<HTMLDivElement>(null);

  // Register this TabStrip so other instances can find it during drag
  useEffect(() => {
    if (!stripRef.current || !tabsRef.current) return;
    stripRegistry.set(paneId, {
      element: stripRef.current,
      tabsContainer: tabsRef.current,
      setDropIdx,
    });
    return () => { stripRegistry.delete(paneId); };
  }, [paneId]);

  // Close context menu on outside click
  useEffect(() => {
    if (!contextMenu) return;
    const handler = () => setContextMenu(null);
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [contextMenu]);

  // Focus rename input when entering rename mode
  useEffect(() => {
    if (renamingTabId) renameInputRef.current?.focus();
  }, [renamingTabId]);

  function handleContextMenu(e: React.MouseEvent, tab: LayoutTab) {
    e.preventDefault();
    setContextMenu({ x: e.clientX, y: e.clientY, tab });
  }

  function startRename(tab: LayoutTab) {
    setRenamingTabId(tab.id);
    setRenameValue(tab.title);
    setContextMenu(null);
  }

  function commitRename() {
    if (renamingTabId && renameValue.trim()) {
      renameTab(paneId, renamingTabId, renameValue.trim());
    }
    setRenamingTabId(null);
  }

  function cancelRename() {
    setRenamingTabId(null);
  }

  const handleTabClick = useCallback((tab: LayoutTab) => {
    setActiveTab(paneId, tab.id);
  }, [paneId, setActiveTab]);

  function handleClose(e: React.MouseEvent, tab: LayoutTab) {
    e.stopPropagation();
    removeTab(paneId, tab.id);
    if (tab.kind !== 'diff') {
      closePtySession(tab.terminal_id);
    }
  }

  function handleAddTerminal() {
    if (!workspaceName) return;
    const terminalId = allocatePtySession(workspaceName, 'terminal');
    const tabId = `tab-${Date.now()}`;
    addTab(paneId, {
      id: tabId,
      terminal_id: terminalId,
      title: 'Terminal',
      kind: 'terminal',
    });
  }

  function handleTabMouseDown(e: React.MouseEvent, tab: LayoutTab) {
    // Only left-click
    if (e.button !== 0) return;
    // Don't start drag from close button
    if ((e.target as HTMLElement).closest('.tab-strip__close')) return;

    e.preventDefault();
    const startX = e.clientX;
    const startY = e.clientY;
    const sourceEl = (e.target as HTMLElement).closest('.tab-strip__tab') as HTMLElement | null;
    let dragStarted = false;

    const onMouseMove = (ev: MouseEvent) => {
      if (!dragStarted) {
        const dx = Math.abs(ev.clientX - startX);
        const dy = Math.abs(ev.clientY - startY);
        if (dx + dy < 5) return;
        dragStarted = true;
        activeDrag = { tabId: tab.id, fromPaneId: paneId };
        document.body.classList.add('tab-dragging');
        if (sourceEl) createGhost(sourceEl);
      }

      moveGhost(ev.clientX, ev.clientY);

      // Clear all indicators, then set the one we're hovering
      clearAllDropIndicators();
      const hit = findStripAtPoint(ev.clientX, ev.clientY);
      if (hit) {
        const idx = getTabDropIndex(hit.entry.tabsContainer, ev.clientX);
        hit.entry.setDropIdx(idx);
        activeDropTarget = { paneId: hit.paneId, index: idx };
      } else {
        activeDropTarget = null;
      }
    };

    const onMouseUp = () => {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);

      if (dragStarted && activeDrag && activeDropTarget) {
        const { tabId: dragTabId, fromPaneId: dragFromPane } = activeDrag;
        const { paneId: targetPane, index: targetIdx } = activeDropTarget;

        if (dragFromPane === targetPane) {
          reorderTab(targetPane, dragTabId, targetIdx);
        } else {
          moveTab(dragFromPane, targetPane, dragTabId);
          reorderTab(targetPane, dragTabId, targetIdx);
        }
      } else if (!dragStarted) {
        // No drag occurred — treat as a click
        handleTabClick(tab);
      }

      activeDrag = null;
      activeDropTarget = null;
      document.body.classList.remove('tab-dragging');
      clearAllDropIndicators();
      removeGhost();
    };

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  }

  return (
    <div className="tab-strip" ref={stripRef}>
      <div className="tab-strip__tabs" ref={tabsRef}>
        {tabs.map((tab, i) => {
          const Icon = tabIcons[tab.kind] ?? Terminal;
          const isActive = tab.id === activeTabId;
          const isRenaming = renamingTabId === tab.id;
          return (
            <div
              key={tab.id}
              className={`tab-strip__tab ${isActive ? 'active' : ''}`}
              role="tab"
              onMouseDown={(e) => handleTabMouseDown(e, tab)}
              onContextMenu={(e) => handleContextMenu(e, tab)}
            >
              {dropIdx === i && <div className="tab-strip__drop-line" />}
              <Icon size={13} className="tab-strip__icon" />
              {isRenaming ? (
                <input
                  ref={renameInputRef}
                  className="tab-strip__rename-input"
                  value={renameValue}
                  onChange={(e) => setRenameValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') commitRename();
                    if (e.key === 'Escape') cancelRename();
                  }}
                  onBlur={commitRename}
                  onMouseDown={(e) => e.stopPropagation()}
                />
              ) : (
                <span
                  className="tab-strip__label"
                  onDoubleClick={() => startRename(tab)}
                >
                  {tab.title}
                </span>
              )}
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
        {dropIdx === tabs.length && <div className="tab-strip__drop-line tab-strip__drop-line--end" />}
      </div>
      <button
        className="tab-strip__add"
        onClick={handleAddTerminal}
        title="New Terminal"
      >
        <Plus size={14} />
      </button>

      {contextMenu && (
        <div
          className="tab-context-menu"
          style={{ left: contextMenu.x, top: contextMenu.y }}
          onMouseDown={(e) => e.stopPropagation()}
        >
          <button onClick={() => startRename(contextMenu.tab)}>Rename</button>
          <button onClick={() => {
            setContextMenu(null);
            const s = useAppStore.getState();
            if (!workspaceName) return;
            const newPaneId = splitPane(paneId, 'row');
            const terminalId = s.allocatePtySession(workspaceName, 'terminal');
            addTab(newPaneId, { id: `tab-${Date.now()}`, terminal_id: terminalId, title: 'Terminal', kind: 'terminal' });
          }}>Split Right</button>
          <button onClick={() => {
            setContextMenu(null);
            const s = useAppStore.getState();
            if (!workspaceName) return;
            const newPaneId = splitPane(paneId, 'column');
            const terminalId = s.allocatePtySession(workspaceName, 'terminal');
            addTab(newPaneId, { id: `tab-${Date.now()}`, terminal_id: terminalId, title: 'Terminal', kind: 'terminal' });
          }}>Split Down</button>
          <div className="tab-context-menu__separator" />
          <button onClick={() => {
            setContextMenu(null);
            // Close all other tabs in this pane
            tabs.forEach((t) => {
              if (t.id !== contextMenu.tab.id) {
                removeTab(paneId, t.id);
                if (t.kind !== 'diff') closePtySession(t.terminal_id);
              }
            });
          }}>Close Other Tabs</button>
          <button onClick={() => {
            setContextMenu(null);
            handleClose({ stopPropagation: () => {} } as React.MouseEvent, contextMenu.tab);
          }}>Close Tab</button>
        </div>
      )}
    </div>
  );
}
