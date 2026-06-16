import type { LayoutNode, PaneNode, SplitNode } from '@/types/layout';

export function findPane(node: LayoutNode, paneId: string): PaneNode | null {
  if (node.kind === 'pane') return node.id === paneId ? node : null;
  return findPane(node.first, paneId) ?? findPane(node.second, paneId);
}

export function findSplit(node: LayoutNode, splitId: string): SplitNode | null {
  if (node.kind === 'pane') return null;
  if (node.id === splitId) return node;
  return findSplit(node.first, splitId) ?? findSplit(node.second, splitId);
}

export function findParentSplit(
  node: LayoutNode,
  childId: string,
): { parent: SplitNode; side: 'first' | 'second' } | null {
  if (node.kind === 'pane') return null;
  if (node.first.id === childId) return { parent: node, side: 'first' };
  if (node.second.id === childId) return { parent: node, side: 'second' };
  return findParentSplit(node.first, childId) ?? findParentSplit(node.second, childId);
}

export function replaceNodeInTree(
  node: LayoutNode,
  targetId: string,
  replacement: LayoutNode,
): LayoutNode {
  if (node.id === targetId) return replacement;
  if (node.kind === 'pane') return node;
  return {
    ...node,
    first: replaceNodeInTree(node.first, targetId, replacement),
    second: replaceNodeInTree(node.second, targetId, replacement),
  };
}

export function collectPaneIds(node: LayoutNode): string[] {
  if (node.kind === 'pane') return [node.id];
  return [...collectPaneIds(node.first), ...collectPaneIds(node.second)];
}
