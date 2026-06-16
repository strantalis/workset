import { type ReactNode } from 'react';
import { useAppStore } from '@/state/store';
import './AppShell.css';

type Props = {
  chrome: ReactNode;
  rail: ReactNode;
  sidebar: ReactNode;
  main: ReactNode;
  rightPanel?: ReactNode;
};

export function AppShell({ chrome, rail, sidebar, main, rightPanel }: Props) {
  const rightPanelCollapsed = useAppStore((s) => s.rightPanelCollapsed);
  const showRight = rightPanel && !rightPanelCollapsed;

  return (
    <div className={`app-shell ${showRight ? '' : 'app-shell--no-right'}`}>
      <div className="app-shell__chrome">{chrome}</div>
      <div className="app-shell__rail">{rail}</div>
      <div className="app-shell__sidebar">{sidebar}</div>
      <div className="app-shell__main">{main}</div>
      {showRight && <div className="app-shell__right-panel">{rightPanel}</div>}
    </div>
  );
}
