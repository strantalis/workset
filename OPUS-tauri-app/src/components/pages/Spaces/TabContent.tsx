import type { LayoutTab } from '@/types/layout';
import { TerminalSurface } from './TerminalSurface';
import { DiffTabView } from '@/components/diff/DiffTabView';
import './TabContent.css';

type Props = {
  tab: LayoutTab | undefined;
  workspaceName: string;
};

export function TabContent({ tab, workspaceName }: Props) {
  if (!tab) {
    return (
      <div className="tab-content tab-content--empty">
        <span>No tabs open</span>
      </div>
    );
  }

  if (tab.kind === 'diff') {
    if (!tab.diff_repo_path || !tab.diff_file_path || !tab.diff_status) {
      return (
        <div className="tab-content tab-content--diff">
          <span>Missing diff metadata</span>
        </div>
      );
    }
    return (
      <div className="tab-content">
        <DiffTabView
          repoPath={tab.diff_repo_path}
          filePath={tab.diff_file_path}
          prevPath={tab.diff_prev_path}
          status={tab.diff_status}
        />
      </div>
    );
  }

  // Both 'terminal' and 'agent' kinds use the terminal surface
  return (
    <div className="tab-content">
      <TerminalSurface
        key={tab.terminal_id}
        workspaceName={workspaceName}
        terminalId={tab.terminal_id}
      />
    </div>
  );
}
