import type { LayoutNode, PaneNode } from '@/types/layout';

export function findPane(node: LayoutNode, paneId: string): PaneNode | null {
  if (node.kind === 'pane') return node.id === paneId ? node : null;
  return findPane(node.first, paneId) ?? findPane(node.second, paneId);
}
