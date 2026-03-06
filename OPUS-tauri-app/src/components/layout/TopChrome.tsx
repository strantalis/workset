import { useState, useRef, useEffect } from 'react';
import { useAppStore } from '@/state/store';
import { ChevronDown } from 'lucide-react';
import './TopChrome.css';

export function TopChrome() {
  const worksets = useAppStore((s) => s.worksets);
  const activeWorksetId = useAppStore((s) => s.activeWorksetId);
  const setActiveWorkset = useAppStore((s) => s.setActiveWorkset);
  const deleteWorkset = useAppStore((s) => s.deleteWorkset);
  const openModal = useAppStore((s) => s.openModal);
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [menuOpenId, setMenuOpenId] = useState<string | null>(null);
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const activeWorkset = worksets.find((w) => w.id === activeWorksetId);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false);
        setMenuOpenId(null);
        setConfirmDeleteId(null);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  function handleMoreClick(e: React.MouseEvent, id: string) {
    e.stopPropagation();
    if (menuOpenId === id) {
      setMenuOpenId(null);
      setConfirmDeleteId(null);
    } else {
      setMenuOpenId(id);
      setConfirmDeleteId(null);
    }
  }

  function handleDelete(e: React.MouseEvent, id: string) {
    e.stopPropagation();
    if (confirmDeleteId !== id) {
      setConfirmDeleteId(id);
      return;
    }
    deleteWorkset(id);
    setMenuOpenId(null);
    setConfirmDeleteId(null);
    setDropdownOpen(false);
  }

  return (
    <div className="top-chrome" data-tauri-drag-region>
      <div className="top-chrome__spacer" data-tauri-drag-region />
      <div className="top-chrome__selector" ref={dropdownRef}>
        <button
          className="top-chrome__selector-btn"
          onClick={() => setDropdownOpen(!dropdownOpen)}
        >
          <span className="top-chrome__selector-label">{activeWorkset?.name || 'Select Workset'}</span>
          <ChevronDown size={14} className="top-chrome__selector-chevron" />
        </button>
        {dropdownOpen && (
          <div className="top-chrome__dropdown">
            {worksets.map((w) => (
              <div key={w.id} className="top-chrome__dropdown-row">
                <button
                  className={`top-chrome__dropdown-item ${w.id === activeWorksetId ? 'active' : ''}`}
                  onClick={() => {
                    setActiveWorkset(w.id);
                    setDropdownOpen(false);
                    setMenuOpenId(null);
                    setConfirmDeleteId(null);
                  }}
                >
                  {w.name}
                </button>
                <div className="top-chrome__more-anchor">
                  <button
                    className="top-chrome__more"
                    onClick={(e) => handleMoreClick(e, w.id)}
                    title="More actions"
                  >
                    ⋯
                  </button>
                  {menuOpenId === w.id && (
                    <div className="top-chrome__submenu">
                      <button
                        className={`top-chrome__submenu-item top-chrome__submenu-item--danger ${confirmDeleteId === w.id ? 'top-chrome__submenu-item--confirm' : ''}`}
                        onClick={(e) => handleDelete(e, w.id)}
                      >
                        {confirmDeleteId === w.id ? 'Confirm delete?' : 'Delete'}
                      </button>
                    </div>
                  )}
                </div>
              </div>
            ))}
            {worksets.length > 0 && <div className="top-chrome__dropdown-divider" />}
            <button
              className="top-chrome__dropdown-item top-chrome__dropdown-action"
              onClick={() => {
                openModal('create-workset');
                setDropdownOpen(false);
              }}
            >
              Create Workset...
            </button>
          </div>
        )}
      </div>
      <div className="top-chrome__spacer" data-tauri-drag-region />
    </div>
  );
}
