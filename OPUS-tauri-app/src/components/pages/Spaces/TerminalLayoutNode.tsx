import { useRef, useCallback, useState } from 'react';
import type { LayoutNode, SplitNode } from '@/types/layout';
import { TabStrip } from './TabStrip';
import { TabContent } from './TabContent';
import { useAppStore } from '@/state/store';
import './TerminalLayoutNode.css';

type Props = {
  node: LayoutNode;
  workspaceName: string;
};

function SplitDivider({ split }: { split: SplitNode }) {
  const setSplitRatio = useAppStore((s) => s.setSplitRatio);
  const containerRef = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState(false);
  const isRow = split.direction === 'row';

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      setDragging(true);

      const parent = containerRef.current?.parentElement;
      if (!parent) return;

      const onMouseMove = (ev: MouseEvent) => {
        const rect = parent.getBoundingClientRect();
        const pos = isRow ? ev.clientX - rect.left : ev.clientY - rect.top;
        const total = isRow ? rect.width : rect.height;
        if (total <= 0) return;
        const ratio = pos / total;
        setSplitRatio(split.id, ratio);
      };

      const onMouseUp = () => {
        setDragging(false);
        document.removeEventListener('mousemove', onMouseMove);
        document.removeEventListener('mouseup', onMouseUp);
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
      };

      document.body.style.cursor = isRow ? 'col-resize' : 'row-resize';
      document.body.style.userSelect = 'none';
      document.addEventListener('mousemove', onMouseMove);
      document.addEventListener('mouseup', onMouseUp);
    },
    [split.id, isRow, setSplitRatio],
  );

  return (
    <>
      {dragging && <div className="layout-split__drag-overlay" />}
      <div
        ref={containerRef}
        className={`layout-split__divider ${isRow ? 'layout-split__divider--row' : 'layout-split__divider--col'}`}
        onMouseDown={handleMouseDown}
      />
    </>
  );
}

export function TerminalLayoutNodeView({ node, workspaceName }: Props) {
  const focusedPaneId = useAppStore((s) => s.focusedPaneId);
  const setFocusedPane = useAppStore((s) => s.setFocusedPane);

  if (node.kind === 'pane') {
    const activeTab = node.tabs.find((t) => t.id === node.active_tab_id) ?? node.tabs[0];
    const isFocused = node.id === focusedPaneId;

    return (
      <div
        className={`layout-pane ${isFocused ? 'focused' : ''}`}
        data-pane-id={node.id}
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
  const firstSize = `calc(${(node.ratio * 100).toFixed(1)}% - 3px)`;
  const secondSize = `calc(${((1 - node.ratio) * 100).toFixed(1)}% - 3px)`;

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
      <SplitDivider split={node} />
      <div style={{ [isRow ? 'width' : 'height']: secondSize, overflow: 'hidden', display: 'flex' }}>
        <TerminalLayoutNodeView node={node.second} workspaceName={workspaceName} />
      </div>
    </div>
  );
}
