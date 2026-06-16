import { useAppStore } from '@/state/store';
import type { NavPage } from '@/state/slices/uiSlice';
import { LayoutGrid, Layers, Settings } from 'lucide-react';
import './IconRail.css';

const navItems: { page: NavPage; icon: typeof LayoutGrid; label: string }[] = [
  { page: 'command-center', icon: LayoutGrid, label: 'Command Center' },
  { page: 'spaces', icon: Layers, label: 'Spaces' },
  { page: 'settings', icon: Settings, label: 'Settings' },
];

export function IconRail() {
  const activePage = useAppStore((s) => s.activePage);
  const setActivePage = useAppStore((s) => s.setActivePage);

  return (
    <nav className="icon-rail">
      {navItems.map(({ page, icon: Icon, label }) => (
        <button
          key={page}
          className={`icon-rail__btn ${activePage === page ? 'active' : ''}`}
          onClick={() => setActivePage(page)}
          title={label}
        >
          <Icon size={20} />
        </button>
      ))}
    </nav>
  );
}
