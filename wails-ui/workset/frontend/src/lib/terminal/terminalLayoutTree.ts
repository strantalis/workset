import type {
	TerminalLayout as TerminalLayoutType,
	TerminalLayoutNode as TerminalLayoutNodeType,
	TerminalLayoutTab as TerminalLayoutTabType,
} from '../types';

export type PaneNode = Omit<
	TerminalLayoutNodeType,
	'kind' | 'direction' | 'ratio' | 'first' | 'second'
> & {
	kind: 'pane';
	terminalId: string;
};
export type SplitNode = Omit<
	TerminalLayoutNodeType,
	'kind' | 'terminalId' | 'first' | 'second' | 'direction' | 'ratio'
> & {
	kind: 'split';
	first: LayoutNode;
	second: LayoutNode;
	direction: 'row' | 'column';
	ratio: number;
};
export type LayoutNode = PaneNode | SplitNode;
export type TerminalTab = Omit<TerminalLayoutTabType, 'root'> & {
	root: LayoutNode;
	focusedPaneId?: string;
};
export type TerminalLayout = Omit<TerminalLayoutType, 'tabs'> & {
	tabs: TerminalTab[];
};

export type PanePosition = {
	id: string;
	x: number;
	y: number;
	w: number;
	h: number;
};

export const LAYOUT_VERSION = 2;

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

const normalizeTab = (tab: TerminalLayoutTabType | undefined | null): TerminalTab | null => {
	if (!tab) return null;
	if (typeof tab.id !== 'string') return null;
	if (!tab.id.trim()) return null;
	const title =
		typeof tab.title === 'string' && tab.title.trim().length > 0 ? tab.title : 'Terminal';
	const root = normalizeNode(tab.root);
	if (!root) return null;
	const focusedPaneId =
		typeof tab.focusedPaneId === 'string' && collectPaneIds(root).includes(tab.focusedPaneId)
			? tab.focusedPaneId
			: firstPaneId(root);
	return { id: tab.id, title, root, focusedPaneId };
};

const normalizeNode = (node: TerminalLayoutNodeType | null | undefined): LayoutNode | null => {
	if (!node || typeof node !== 'object') return null;
	if (node.kind === 'pane') {
		if (typeof node.terminalId !== 'string' || !node.terminalId.trim()) return null;
		return {
			id: coerceId(node.id),
			kind: 'pane',
			terminalId: node.terminalId,
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
	const tabs = Array.isArray(candidate.tabs)
		? candidate.tabs
				.map((tab) => normalizeTab(tab))
				.filter((tab): tab is TerminalTab => tab !== null)
		: [];
	if (tabs.length === 0) return null;
	const activeTabId =
		typeof candidate.activeTabId === 'string' &&
		tabs.some((tab) => tab.id === candidate.activeTabId)
			? candidate.activeTabId
			: tabs[0].id;
	return {
		version: LAYOUT_VERSION,
		tabs,
		activeTabId,
	};
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
	layout: TerminalLayout,
	sourceIndex: number,
	targetIndex: number,
): TerminalLayout => {
	if (sourceIndex === targetIndex) return layout;
	if (
		sourceIndex < 0 ||
		targetIndex < 0 ||
		sourceIndex >= layout.tabs.length ||
		targetIndex >= layout.tabs.length
	) {
		return layout;
	}

	const tabs = [...layout.tabs];
	const [tab] = tabs.splice(sourceIndex, 1);
	if (!tab) return layout;
	tabs.splice(targetIndex, 0, tab);
	return { ...layout, tabs };
};

export const firstPaneId = (node: LayoutNode): string => {
	if (node.kind === 'pane') return node.id;
	return firstPaneId(node.first);
};

export const buildTab = (terminalId: string, title: string): TerminalTab => {
	const root = buildPane(terminalId);
	return {
		id: newId(),
		title,
		root,
		focusedPaneId: root.id,
	};
};

export const buildPane = (terminalId: string): PaneNode => ({
	id: newId(),
	kind: 'pane',
	terminalId,
});

export const ensureFocusedPane = (tab: TerminalTab): TerminalTab => {
	if (collectPaneIds(tab.root).includes(tab.focusedPaneId ?? '')) {
		return tab;
	}
	return { ...tab, focusedPaneId: firstPaneId(tab.root) };
};

export const ensureActiveTab = (layout: TerminalLayout): TerminalLayout => {
	const tabs = layout.tabs.map(ensureFocusedPane);
	const activeTabId = tabs.some((tab) => tab.id === layout.activeTabId)
		? layout.activeTabId
		: (tabs[0]?.id ?? '');
	return { ...layout, tabs, activeTabId };
};

export const findTab = (layout: TerminalLayout, tabId: string): TerminalTab | null =>
	layout.tabs.find((tab) => tab.id === tabId) ?? null;

export const updateTab = (
	layout: TerminalLayout,
	tabId: string,
	updater: (tab: TerminalTab) => TerminalTab,
): TerminalLayout => {
	const tabs = layout.tabs.map((tab) => (tab.id === tabId ? updater(tab) : tab));
	return ensureActiveTab({ ...layout, tabs });
};

export const activeTab = (layout: TerminalLayout): TerminalTab | null =>
	findTab(layout, layout.activeTabId);

export const collectTerminalIds = (node: LayoutNode, ids: string[] = []): string[] => {
	if (node.kind === 'pane') {
		ids.push(node.terminalId);
		return ids;
	}
	collectTerminalIds(node.first, ids);
	collectTerminalIds(node.second, ids);
	return ids;
};

type LegacyLayoutNode = {
	kind?: string;
	terminalId?: string;
	tabs?: Array<{ terminalId?: string }>;
	first?: LegacyLayoutNode | null;
	second?: LegacyLayoutNode | null;
};

const collectLegacyNodeTerminalIds = (
	node: LegacyLayoutNode | null | undefined,
	ids: string[],
): void => {
	if (!node) return;
	if (node.kind === 'pane') {
		if (typeof node.terminalId === 'string' && node.terminalId.trim()) {
			ids.push(node.terminalId);
		}
		if (Array.isArray(node.tabs)) {
			for (const tab of node.tabs) {
				if (typeof tab?.terminalId === 'string' && tab.terminalId.trim()) {
					ids.push(tab.terminalId);
				}
			}
		}
		return;
	}
	collectLegacyNodeTerminalIds(node.first ?? null, ids);
	collectLegacyNodeTerminalIds(node.second ?? null, ids);
};

export const collectTerminalIdsFromUnknownLayout = (layout: unknown): string[] => {
	if (!layout || typeof layout !== 'object') return [];
	const ids: string[] = [];
	const candidate = layout as {
		tabs?: Array<{ root?: LayoutNode | LegacyLayoutNode | null }>;
		root?: LegacyLayoutNode | null;
	};

	if (Array.isArray(candidate.tabs)) {
		for (const tab of candidate.tabs) {
			const root = tab?.root;
			if (!root) continue;
			if (
				(root as LayoutNode).kind === 'pane' &&
				typeof (root as LayoutNode & { terminalId?: string }).terminalId === 'string'
			) {
				collectTerminalIds(root as LayoutNode, ids);
				continue;
			}
			collectLegacyNodeTerminalIds(root as LegacyLayoutNode, ids);
		}
		return Array.from(new Set(ids));
	}

	collectLegacyNodeTerminalIds(candidate.root ?? null, ids);
	return Array.from(new Set(ids));
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
