import { useState, useRef, useEffect } from 'react';
import type { WorkspaceSummary } from '@/types/workspace';
import './WorkspaceItem.css';

type Props = {
  workspace: WorkspaceSummary;
  active: boolean;
  onClick: () => void;
  onDelete: () => Promise<void>;
};

export function WorkspaceItem({ workspace, active, onClick, onDelete }: Props) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!menuOpen) return;
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false);
        setConfirming(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [menuOpen]);

  function handleMenuToggle(e: React.MouseEvent) {
    e.stopPropagation();
    setMenuOpen(!menuOpen);
    setConfirming(false);
  }

  function handleDelete(e: React.MouseEvent) {
    e.stopPropagation();
    if (!confirming) {
      setConfirming(true);
      return;
    }
    setDeleting(true);
    onDelete().catch(() => {
      setDeleting(false);
      setMenuOpen(false);
    });
  }

  return (
    <div
      className={`workspace-item ${active ? 'active' : ''}`}
      onClick={onClick}
      role="button"
      tabIndex={0}
    >
      <span className="workspace-item__dot" style={{ background: 'var(--success)' }} />
      <span className="workspace-item__name">{workspace.name}</span>
      <div className="workspace-item__menu-anchor" ref={menuRef}>
        <button
          className="workspace-item__more"
          onClick={handleMenuToggle}
          title="More actions"
        >
          ⋯
        </button>
        {menuOpen && (
          <div className="workspace-item__menu">
            <button
              className={`workspace-item__menu-item workspace-item__menu-item--danger ${confirming ? 'workspace-item__menu-item--confirm' : ''}`}
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? 'Deleting...' : confirming ? 'Confirm delete?' : 'Delete'}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
