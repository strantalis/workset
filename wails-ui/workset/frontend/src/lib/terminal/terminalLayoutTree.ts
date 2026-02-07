import type {
	TerminalLayout as TerminalLayoutType,
	TerminalLayoutNode as TerminalLayoutNodeType,
	TerminalLayoutTab as TerminalLayoutTabType,
} from '../types';

export type TerminalTab = TerminalLayoutTabType;
export type PaneNode = Omit<TerminalLayoutNodeType, 'kind' | 'tabs' | 'activeTabId'> & {
	kind: 'pane';
	tabs: TerminalTab[];
	activeTabId: string;
};
export type SplitNode = Omit<
	TerminalLayoutNodeType,
	'kind' | 'first' | 'second' | 'direction' | 'ratio'
> & {
	kind: 'split';
	first: LayoutNode;
	second: LayoutNode;
	direction: 'row' | 'column';
	ratio: number;
};
export type LayoutNode = PaneNode | SplitNode;
export type TerminalLayout = Omit<TerminalLayoutType, 'root'> & {
	root: LayoutNode;
	focusedPaneId?: string;
};

export type PanePosition = {
	id: string;
	x: number;
	y: number;
	w: number;
	h: number;
};

export const LAYOUT_VERSION = 1;

export const newId = (): string => {
	if (typeof crypto !== 'undefined' && crypto.randomUUID) {
		return crypto.randomUUID();
	}
	return `term-${Math.random().toString(36).slice(2)}`;
};

const coerceId = (value: unknown): string => {
	if (typeof value === 'string' && value.trim()) return value;
	return newId();
};

const normalizeTab = (tab: TerminalTab | undefined | null): TerminalTab | null => {
	if (!tab) return null;
	if (typeof tab.id !== 'string' || typeof tab.terminalId !== 'string') return null;
	const title =
		typeof tab.title === 'string' && tab.title.trim().length > 0 ? tab.title : 'Terminal';
	return { id: tab.id, terminalId: tab.terminalId, title };
};

const normalizeNode = (node: TerminalLayoutNodeType | null | undefined): LayoutNode | null => {
	if (!node || typeof node !== 'object') return null;
	if (node.kind === 'pane') {
		const tabs = Array.isArray(node.tabs)
			? node.tabs.map(normalizeTab).filter((tab): tab is TerminalTab => tab !== null)
			: [];
		if (tabs.length === 0) return null;
		const activeTabId =
			typeof node.activeTabId === 'string' && tabs.some((tab) => tab.id === node.activeTabId)
				? node.activeTabId
				: tabs[0].id;
		return {
			id: coerceId(node.id),
			kind: 'pane',
			tabs,
			activeTabId,
		};
	}
	if (node.kind === 'split') {
		const first = normalizeNode(node.first);
		const second = normalizeNode(node.second);
		if (!first && !second) return null;
		if (!first) return second;
		if (!second) return first;
		const direction =
			node.direction === 'row' || node.direction === 'column' ? node.direction : 'row';
		const ratio =
			typeof node.ratio === 'number' &&
			Number.isFinite(node.ratio) &&
			node.ratio > 0 &&
			node.ratio < 1
				? node.ratio
				: 0.5;
		return {
			id: coerceId(node.id),
			kind: 'split',
			direction,
			ratio,
			first,
			second,
		};
	}
	return null;
};

export const normalizeLayout = (
	candidate: TerminalLayoutType | null | undefined,
): TerminalLayout | null => {
	if (!candidate || candidate.version !== LAYOUT_VERSION) return null;
	const root = normalizeNode(candidate.root);
	if (!root) return null;
	return {
		version: LAYOUT_VERSION,
		root,
		focusedPaneId: candidate.focusedPaneId,
	};
};

export const collectTabs = (node: LayoutNode, tabs: TerminalTab[] = []): TerminalTab[] => {
	if (node.kind === 'pane') {
		return tabs.concat(node.tabs);
	}
	collectTabs(node.first, tabs);
	collectTabs(node.second, tabs);
	return tabs;
};

export const collectPaneIds = (node: LayoutNode, ids: string[] = []): string[] => {
	if (node.kind === 'pane') {
		ids.push(node.id);
		return ids;
	}
	collectPaneIds(node.first, ids);
	collectPaneIds(node.second, ids);
	return ids;
};

export const findPane = (node: LayoutNode, paneId: string): PaneNode | null => {
	if (node.kind === 'pane') {
		return node.id === paneId ? node : null;
	}
	return findPane(node.first, paneId) || findPane(node.second, paneId);
};

export const updatePane = (
	node: LayoutNode,
	paneId: string,
	updater: (pane: PaneNode) => PaneNode,
): LayoutNode => {
	if (node.kind === 'pane') {
		return node.id === paneId ? updater(node) : node;
	}
	const first = updatePane(node.first, paneId, updater);
	const second = updatePane(node.second, paneId, updater);
	if (first === node.first && second === node.second) {
		return node;
	}
	return { ...node, first, second };
};

export const splitPane = (
	node: LayoutNode,
	paneId: string,
	direction: 'row' | 'column',
	pane: PaneNode,
): LayoutNode => {
	if (node.kind === 'pane') {
		if (node.id !== paneId) {
			return node;
		}
		return {
			id: newId(),
			kind: 'split',
			direction,
			ratio: 0.5,
			first: node,
			second: pane,
		};
	}
	const first = splitPane(node.first, paneId, direction, pane);
	const second = splitPane(node.second, paneId, direction, pane);
	if (first === node.first && second === node.second) {
		return node;
	}
	return { ...node, first, second };
};

export const removePane = (node: LayoutNode, paneId: string): LayoutNode | null => {
	if (node.kind === 'pane') {
		return node.id === paneId ? null : node;
	}
	const first = removePane(node.first, paneId);
	const second = removePane(node.second, paneId);
	if (!first && !second) return null;
	if (!first) return second;
	if (!second) return first;
	return { ...node, first, second };
};

export const updateSplitRatio = (node: LayoutNode, splitId: string, ratio: number): LayoutNode => {
	if (node.kind === 'pane') return node;
	if (node.id === splitId) {
		return { ...node, ratio };
	}
	const first = updateSplitRatio(node.first, splitId, ratio);
	const second = updateSplitRatio(node.second, splitId, ratio);
	if (first === node.first && second === node.second) return node;
	return { ...node, first, second };
};

export const moveTab = (
	node: LayoutNode,
	sourcePaneId: string,
	targetPaneId: string,
	tabId: string,
	targetIndex: number,
): LayoutNode => {
	const sourcePane = findPane(node, sourcePaneId);
	if (!sourcePane) return node;
	const tab = sourcePane.tabs.find((t) => t.id === tabId);
	if (!tab) return node;

	if (sourcePaneId === targetPaneId) {
		return updatePane(node, sourcePaneId, (pane) => {
			const tabs = pane.tabs.filter((t) => t.id !== tabId);
			tabs.splice(targetIndex, 0, tab);
			return { ...pane, tabs };
		});
	}

	let updated = updatePane(node, sourcePaneId, (pane) => ({
		...pane,
		tabs: pane.tabs.filter((t) => t.id !== tabId),
		activeTabId:
			pane.activeTabId === tabId
				? (pane.tabs.find((t) => t.id !== tabId)?.id ?? pane.activeTabId)
				: pane.activeTabId,
	}));

	updated = updatePane(updated, targetPaneId, (pane) => {
		const tabs = [...pane.tabs];
		tabs.splice(targetIndex, 0, tab);
		return { ...pane, tabs, activeTabId: tab.id };
	});

	return updated;
};

export const firstPaneId = (node: LayoutNode): string => {
	if (node.kind === 'pane') return node.id;
	return firstPaneId(node.first);
};

export const buildTab = (terminalId: string, title: string): TerminalTab => ({
	id: newId(),
	terminalId,
	title,
});

export const buildPane = (tab: TerminalTab): PaneNode => ({
	id: newId(),
	kind: 'pane',
	tabs: [tab],
	activeTabId: tab.id,
});

export const ensureFocusedPane = (next: TerminalLayout): TerminalLayout => {
	if (!next.focusedPaneId) {
		return { ...next, focusedPaneId: firstPaneId(next.root) };
	}
	if (collectPaneIds(next.root).includes(next.focusedPaneId)) {
		return next;
	}
	return { ...next, focusedPaneId: firstPaneId(next.root) };
};

export const applyTabFixes = (
	node: LayoutNode,
	fixes: Map<string, { terminalId?: string; drop?: boolean }>,
): LayoutNode | null => {
	if (node.kind === 'pane') {
		const tabs = node.tabs
			.map((tab) => {
				const fix = fixes.get(tab.id);
				if (fix?.drop) return null;
				if (fix?.terminalId) {
					return { ...tab, terminalId: fix.terminalId };
				}
				return tab;
			})
			.filter((tab): tab is TerminalTab => tab !== null);
		if (tabs.length === 0) return null;
		const activeTabId = tabs.some((tab) => tab.id === node.activeTabId)
			? node.activeTabId
			: tabs[0].id;
		return { ...node, tabs, activeTabId };
	}
	const first = applyTabFixes(node.first, fixes);
	const second = applyTabFixes(node.second, fixes);
	if (!first && !second) return null;
	if (!first) return second;
	if (!second) return first;
	return { ...node, first, second };
};

export const buildPanePositions = (
	node: LayoutNode,
	x = 0,
	y = 0,
	w = 1,
	h = 1,
	positions: PanePosition[] = [],
): PanePosition[] => {
	if (node.kind === 'pane') {
		positions.push({ id: node.id, x, y, w, h });
		return positions;
	}
	const { direction, ratio, first, second } = node;
	if (direction === 'row') {
		buildPanePositions(first, x, y, w * ratio, h, positions);
		buildPanePositions(second, x + w * ratio, y, w * (1 - ratio), h, positions);
	} else {
		buildPanePositions(first, x, y, w, h * ratio, positions);
		buildPanePositions(second, x, y + h * ratio, w, h * (1 - ratio), positions);
	}
	return positions;
};

export const findAdjacentPane = (
	currentId: string,
	direction: 'up' | 'down' | 'left' | 'right',
	positions: PanePosition[],
): string | null => {
	const current = positions.find((p) => p.id === currentId);
	if (!current) return null;

	const cx = current.x + current.w / 2;
	const cy = current.y + current.h / 2;

	let candidates = positions.filter((p) => p.id !== currentId);

	if (direction === 'left') {
		candidates = candidates.filter((p) => p.x + p.w <= current.x + 0.01);
	} else if (direction === 'right') {
		candidates = candidates.filter((p) => p.x >= current.x + current.w - 0.01);
	} else if (direction === 'up') {
		candidates = candidates.filter((p) => p.y + p.h <= current.y + 0.01);
	} else {
		candidates = candidates.filter((p) => p.y >= current.y + current.h - 0.01);
	}

	if (candidates.length === 0) return null;

	const axis = direction === 'left' || direction === 'right' ? 'y' : 'x';
	const center = axis === 'x' ? cx : cy;
	candidates.sort((a, b) => {
		const aCtr = axis === 'x' ? a.x + a.w / 2 : a.y + a.h / 2;
		const bCtr = axis === 'x' ? b.x + b.w / 2 : b.y + b.h / 2;
		return Math.abs(aCtr - center) - Math.abs(bCtr - center);
	});

	return candidates[0].id;
};
