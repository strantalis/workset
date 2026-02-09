import type { LayoutNode } from '@/types/layout';
import { TabStrip } from './TabStrip';
import { TabContent } from './TabContent';
import { useAppStore } from '@/state/store';
import './TerminalLayoutNode.css';

type Props = {
  node: LayoutNode;
  workspaceName: string;
};

export function TerminalLayoutNodeView({ node, workspaceName }: Props) {
  const focusedPaneId = useAppStore((s) => s.focusedPaneId);
  const setFocusedPane = useAppStore((s) => s.setFocusedPane);

  if (node.kind === 'pane') {
    const activeTab = node.tabs.find((t) => t.id === node.active_tab_id) ?? node.tabs[0];
    const isFocused = node.id === focusedPaneId;

    return (
      <div
        className={`layout-pane ${isFocused ? 'focused' : ''}`}
        onClick={() => setFocusedPane(node.id)}
      >
        <TabStrip
          paneId={node.id}
          tabs={node.tabs}
          activeTabId={node.active_tab_id}
          workspaceName={workspaceName}
        />
        <TabContent tab={activeTab} workspaceName={workspaceName} />
      </div>
    );
  }

  // Split node
  const isRow = node.direction === 'row';
  const firstSize = `${(node.ratio * 100).toFixed(1)}%`;
  const secondSize = `${((1 - node.ratio) * 100).toFixed(1)}%`;

  return (
    <div
      className="layout-split"
      style={{
        flexDirection: isRow ? 'row' : 'column',
      }}
    >
      <div style={{ [isRow ? 'width' : 'height']: firstSize, overflow: 'hidden', display: 'flex' }}>
        <TerminalLayoutNodeView node={node.first} workspaceName={workspaceName} />
      </div>
      <div className="layout-split__divider" style={{ [isRow ? 'width' : 'height']: '1px' }} />
      <div style={{ [isRow ? 'width' : 'height']: secondSize, overflow: 'hidden', display: 'flex' }}>
        <TerminalLayoutNodeView node={node.second} workspaceName={workspaceName} />
      </div>
    </div>
  );
}
