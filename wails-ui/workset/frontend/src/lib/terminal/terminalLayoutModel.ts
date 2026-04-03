import type {
	TerminalLayout as TerminalLayoutType,
	TerminalLayoutLeaf as TerminalLayoutLeafType,
	TerminalLayoutNode as TerminalLayoutNodeType,
	TerminalLayoutSplit as TerminalLayoutSplitType,
	TerminalLayoutTab as TerminalLayoutTabType,
	TerminalSplitDirection as TerminalSplitDirectionType,
} from '../types';
import type { TerminalSnapshotLike } from './terminalEmulatorContracts';

export type TerminalLayoutLeaf = TerminalLayoutLeafType;
export type TerminalLayoutSplit = TerminalLayoutSplitType;
export type TerminalLayoutNode = TerminalLayoutNodeType;
export type TerminalTab = TerminalLayoutTabType;
export type TerminalLayout = TerminalLayoutType;
export type TerminalSplitDirection = TerminalSplitDirectionType;

export const LAYOUT_VERSION = 4;
const MAX_DEPTH = 2;
const DEFAULT_SPLIT_RATIO = 0.5;
const DEFAULT_SPLIT_DIRECTION: TerminalSplitDirection = 'vertical';

export const newId = (): string => {
	if (typeof crypto !== 'undefined' && crypto.randomUUID) {
		return crypto.randomUUID();
	}
	return `term-${Math.random().toString(36).slice(2)}`;
};

const coerceId = (value: unknown): string => {
	if (typeof value === 'string' && value.trim()) {
		return value;
	}
	return newId();
};

// ── Snapshot normalization ──────��───────────────────────────────────

const normalizeSnapshot = (value: unknown): TerminalSnapshotLike | undefined => {
	if (!value || typeof value !== 'object') {
		return undefined;
	}
	const snapshot = value as Partial<TerminalSnapshotLike>;
	if (
		snapshot.version !== 1 ||
		typeof snapshot.nextOffset !== 'number' ||
		typeof snapshot.cols !== 'number' ||
		typeof snapshot.rows !== 'number' ||
		(snapshot.activeBuffer !== 'normal' && snapshot.activeBuffer !== 'alternate') ||
		typeof snapshot.normalViewportY !== 'number'
	) {
		return undefined;
	}
	if (
		!snapshot.cursor ||
		typeof snapshot.cursor.x !== 'number' ||
		typeof snapshot.cursor.y !== 'number' ||
		typeof snapshot.cursor.visible !== 'boolean'
	) {
		return undefined;
	}
	if (
		!snapshot.modes ||
		!Array.isArray(snapshot.modes.dec) ||
		!Array.isArray(snapshot.modes.ansi) ||
		!Array.isArray(snapshot.normalTail)
	) {
		return undefined;
	}
	return snapshot as TerminalSnapshotLike;
};

// ── Node normalization ──────���──────────────────��────────────────────

const normalizeSplitDirection = (value: unknown): TerminalSplitDirection => {
	if (value === 'horizontal' || value === 'vertical') {
		return value;
	}
	return DEFAULT_SPLIT_DIRECTION;
};

const normalizeSplitRatio = (value: unknown): number => {
	if (typeof value === 'number' && Number.isFinite(value) && value > 0.1 && value < 0.9) {
		return value;
	}
	return DEFAULT_SPLIT_RATIO;
};

const normalizeNode = (node: unknown, depth = 0): TerminalLayoutNode | null => {
	if (!node || typeof node !== 'object') return null;
	const candidate = node as Record<string, unknown>;

	if (candidate.kind === 'pane') {
		if (typeof candidate.terminalId !== 'string' || !candidate.terminalId.trim()) {
			return null;
		}
		return {
			kind: 'pane',
			id: coerceId(candidate.id),
			terminalId: candidate.terminalId as string,
			snapshot: normalizeSnapshot(candidate.snapshot),
		};
	}

	if (candidate.kind === 'split') {
		if (depth >= MAX_DEPTH) return null;
		const first = normalizeNode(candidate.first, depth + 1);
		const second = normalizeNode(candidate.second, depth + 1);
		if (!first || !second) return null;
		return {
			kind: 'split',
			id: coerceId(candidate.id),
			direction: normalizeSplitDirection(candidate.direction),
			ratio: normalizeSplitRatio(candidate.ratio),
			first,
			second,
		};
	}

	return null;
};

// ── Tab normalization ─────────────────��─────────────────────────────

const normalizeTab = (tab: unknown): TerminalTab | null => {
	if (!tab || typeof tab !== 'object') return null;
	const candidate = tab as Record<string, unknown>;

	const root = normalizeNode(candidate.root);
	if (!root) return null;

	const title =
		typeof candidate.title === 'string' && candidate.title.trim().length > 0
			? candidate.title
			: 'Terminal';

	const leaves = collectLeaves(root);
	const focusedPaneId =
		typeof candidate.focusedPaneId === 'string' &&
		leaves.some((leaf) => leaf.id === candidate.focusedPaneId)
			? (candidate.focusedPaneId as string)
			: leaves[0].id;

	return {
		id: coerceId(candidate.id),
		title,
		root,
		focusedPaneId,
	};
};

// ── V3 migration ─────────────────��──────────────────────────────────

type V3Pane = { id?: unknown; terminalId?: unknown; snapshot?: unknown };
type V3Tab = {
	id?: unknown;
	title?: unknown;
	panes?: unknown;
	splitDirection?: unknown;
	splitRatio?: unknown;
	focusedPaneId?: unknown;
};

const migrateV3Pane = (pane: V3Pane): TerminalLayoutLeaf | null => {
	if (typeof pane.terminalId !== 'string' || !pane.terminalId.trim()) return null;
	return {
		kind: 'pane',
		id: coerceId(pane.id),
		terminalId: pane.terminalId,
		snapshot: normalizeSnapshot(pane.snapshot),
	};
};

const migrateV3Tab = (tab: V3Tab): TerminalTab | null => {
	if (!Array.isArray(tab.panes) || tab.panes.length === 0) return null;
	const leaves = tab.panes
		.map((p: unknown) => migrateV3Pane(p as V3Pane))
		.filter((leaf): leaf is TerminalLayoutLeaf => leaf !== null)
		.slice(0, 2);
	if (leaves.length === 0) return null;

	let root: TerminalLayoutNode;
	if (leaves.length === 1) {
		root = leaves[0];
	} else {
		root = {
			kind: 'split',
			id: newId(),
			direction: normalizeSplitDirection(tab.splitDirection),
			ratio: normalizeSplitRatio(tab.splitRatio),
			first: leaves[0],
			second: leaves[1],
		};
	}

	const title =
		typeof tab.title === 'string' && tab.title.trim().length > 0 ? tab.title : 'Terminal';

	const allLeaves = collectLeaves(root);
	const focusedPaneId =
		typeof tab.focusedPaneId === 'string' && allLeaves.some((leaf) => leaf.id === tab.focusedPaneId)
			? (tab.focusedPaneId as string)
			: allLeaves[0].id;

	return {
		id: coerceId(tab.id),
		title,
		root,
		focusedPaneId,
	};
};

// ── Layout normalization (entry point) ──────────────────────────────

export const normalizeLayout = (candidate: unknown): TerminalLayout | null => {
	if (!candidate || typeof candidate !== 'object') return null;
	const layout = candidate as Record<string, unknown>;

	// Handle v3 migration
	if (layout.version === 3) {
		const tabs = Array.isArray(layout.tabs)
			? layout.tabs
					.map((tab: unknown) => migrateV3Tab(tab as V3Tab))
					.filter((tab): tab is TerminalTab => tab !== null)
			: [];
		if (tabs.length === 0) return null;
		const activeTabId =
			typeof layout.activeTabId === 'string' && tabs.some((tab) => tab.id === layout.activeTabId)
				? (layout.activeTabId as string)
				: tabs[0].id;
		return { version: LAYOUT_VERSION, tabs, activeTabId };
	}

	if (layout.version !== LAYOUT_VERSION) return null;

	const tabs = Array.isArray(layout.tabs)
		? layout.tabs
				.map((tab: unknown) => normalizeTab(tab))
				.filter((tab): tab is TerminalTab => tab !== null)
		: [];
	if (tabs.length === 0) return null;

	const activeTabId =
		typeof layout.activeTabId === 'string' && tabs.some((tab) => tab.id === layout.activeTabId)
			? (layout.activeTabId as string)
			: tabs[0].id;

	return { version: LAYOUT_VERSION, tabs, activeTabId };
};

// ── Builders ───────────────────────���──────────────────────────��─────

export const buildLeaf = (terminalId: string): TerminalLayoutLeaf => ({
	kind: 'pane',
	id: newId(),
	terminalId,
});

export const buildTab = (terminalId: string, title: string): TerminalTab => {
	const leaf = buildLeaf(terminalId);
	return {
		id: newId(),
		title,
		root: leaf,
		focusedPaneId: leaf.id,
	};
};

// ── Tree queries ───────────────────────��────────────────────────────

export const collectLeaves = (node: TerminalLayoutNode): TerminalLayoutLeaf[] => {
	if (node.kind === 'pane') return [node];
	return [...collectLeaves(node.first), ...collectLeaves(node.second)];
};

export const countLeaves = (node: TerminalLayoutNode): number => {
	if (node.kind === 'pane') return 1;
	return countLeaves(node.first) + countLeaves(node.second);
};

export const findLeaf = (node: TerminalLayoutNode, paneId: string): TerminalLayoutLeaf | null => {
	if (node.kind === 'pane') return node.id === paneId ? node : null;
	return findLeaf(node.first, paneId) ?? findLeaf(node.second, paneId);
};

const depthOf = (node: TerminalLayoutNode, paneId: string, depth = 0): number => {
	if (node.kind === 'pane') return node.id === paneId ? depth : -1;
	const left = depthOf(node.first, paneId, depth + 1);
	if (left >= 0) return left;
	return depthOf(node.second, paneId, depth + 1);
};

export const canSplit = (root: TerminalLayoutNode, paneId: string): boolean => {
	const d = depthOf(root, paneId);
	return d >= 0 && d < MAX_DEPTH;
};

// ── Tree mutations (immutable) ──────────────────────────────────────

export const splitPane = (
	node: TerminalLayoutNode,
	paneId: string,
	newLeaf: TerminalLayoutLeaf,
	direction: TerminalSplitDirection,
	depth = 0,
): TerminalLayoutNode | null => {
	if (node.kind === 'pane') {
		if (node.id !== paneId) return null;
		if (depth >= MAX_DEPTH) return null;
		return {
			kind: 'split',
			id: newId(),
			direction,
			ratio: DEFAULT_SPLIT_RATIO,
			first: node,
			second: newLeaf,
		};
	}
	const firstResult = splitPane(node.first, paneId, newLeaf, direction, depth + 1);
	if (firstResult) return { ...node, first: firstResult };
	const secondResult = splitPane(node.second, paneId, newLeaf, direction, depth + 1);
	if (secondResult) return { ...node, second: secondResult };
	return null;
};

export const closePane = (node: TerminalLayoutNode, paneId: string): TerminalLayoutNode | null => {
	if (node.kind === 'pane') return null; // can't close root leaf this way
	// Direct child match — return sibling
	if (node.first.kind === 'pane' && node.first.id === paneId) return node.second;
	if (node.second.kind === 'pane' && node.second.id === paneId) return node.first;
	// Recurse into children
	const firstResult = closePane(node.first, paneId);
	if (firstResult) return { ...node, first: firstResult };
	const secondResult = closePane(node.second, paneId);
	if (secondResult) return { ...node, second: secondResult };
	return null;
};

export const updateNodeRatio = (
	node: TerminalLayoutNode,
	nodeId: string,
	ratio: number,
): TerminalLayoutNode => {
	if (node.kind === 'pane') return node;
	if (node.id === nodeId) {
		return { ...node, ratio: Math.max(0.15, Math.min(0.85, ratio)) };
	}
	const firstResult = updateNodeRatio(node.first, nodeId, ratio);
	if (firstResult !== node.first) return { ...node, first: firstResult };
	const secondResult = updateNodeRatio(node.second, nodeId, ratio);
	if (secondResult !== node.second) return { ...node, second: secondResult };
	return node;
};

export const updateLeafSnapshot = (
	node: TerminalLayoutNode,
	paneId: string,
	snapshot: TerminalSnapshotLike | undefined,
): TerminalLayoutNode => {
	if (node.kind === 'pane') {
		return node.id === paneId ? { ...node, snapshot } : node;
	}
	const first = updateLeafSnapshot(node.first, paneId, snapshot);
	const second = updateLeafSnapshot(node.second, paneId, snapshot);
	if (first === node.first && second === node.second) return node;
	return { ...node, first, second };
};

// ── Terminal ID collection ───────────────���──────────────────────────

export const collectTabTerminalIds = (tab: TerminalTab): string[] =>
	collectLeaves(tab.root).map((leaf) => leaf.terminalId);

export const collectLayoutTerminalIds = (layout: TerminalLayout): string[] =>
	layout.tabs.flatMap((tab) => collectTabTerminalIds(tab));

const collectNodeTerminalIds = (node: unknown): string[] => {
	if (!node || typeof node !== 'object') return [];
	const candidate = node as Record<string, unknown>;
	const ids: string[] = [];
	if (typeof candidate.terminalId === 'string' && candidate.terminalId.trim()) {
		ids.push(candidate.terminalId);
	}
	// Legacy: node may contain a nested tabs array
	if (Array.isArray(candidate.tabs)) {
		for (const tab of candidate.tabs) {
			if (
				tab &&
				typeof tab === 'object' &&
				typeof (tab as Record<string, unknown>).terminalId === 'string'
			) {
				ids.push((tab as Record<string, unknown>).terminalId as string);
			}
		}
	}
	if (candidate.first) ids.push(...collectNodeTerminalIds(candidate.first));
	if (candidate.second) ids.push(...collectNodeTerminalIds(candidate.second));
	return ids;
};

export const collectLayoutTerminalIdsFromUnknown = (layout: unknown): string[] => {
	if (!layout || typeof layout !== 'object') return [];
	const terminalIds = new Set<string>();

	const candidate = layout as Record<string, unknown>;
	if (Array.isArray(candidate.tabs)) {
		for (const tab of candidate.tabs) {
			if (!tab || typeof tab !== 'object') continue;
			const t = tab as Record<string, unknown>;
			// V4: tree root
			if (t.root) {
				for (const id of collectNodeTerminalIds(t.root)) terminalIds.add(id);
			}
			// V3: flat panes array
			if (Array.isArray(t.panes)) {
				for (const pane of t.panes) {
					if (
						pane &&
						typeof pane === 'object' &&
						typeof (pane as Record<string, unknown>).terminalId === 'string'
					) {
						terminalIds.add((pane as Record<string, unknown>).terminalId as string);
					}
				}
			}
		}
	}

	// Legacy: root-level node
	if (candidate.root) {
		for (const id of collectNodeTerminalIds(candidate.root)) terminalIds.add(id);
	}

	return Array.from(terminalIds);
};

// ── Tab-level operations ──────────────────���─────────────────────────

export const ensureFocusedPane = (tab: TerminalTab): TerminalTab => {
	const leaves = collectLeaves(tab.root);
	if (leaves.length === 0) return tab;
	if (leaves.some((leaf) => leaf.id === tab.focusedPaneId)) return tab;
	return { ...tab, focusedPaneId: leaves[0].id };
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

export const activeTab = (layout: TerminalLayout): TerminalTab | null =>
	findTab(layout, layout.activeTabId);

export const updateTab = (
	layout: TerminalLayout,
	tabId: string,
	updater: (tab: TerminalTab) => TerminalTab,
): TerminalLayout => {
	const tabs = layout.tabs.map((tab) => (tab.id === tabId ? ensureFocusedPane(updater(tab)) : tab));
	return ensureActiveTab({ ...layout, tabs });
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

// ── Focus navigation ────────────────────────────────────────────────

type Rect = { x: number; y: number; w: number; h: number };
type PaneBounds = { paneId: string; bounds: Rect };

export const collectPaneBounds = (
	node: TerminalLayoutNode,
	bounds: Rect = { x: 0, y: 0, w: 1, h: 1 },
): PaneBounds[] => {
	if (node.kind === 'pane') return [{ paneId: node.id, bounds }];
	const { direction, ratio } = node;
	let firstBounds: Rect;
	let secondBounds: Rect;
	if (direction === 'vertical') {
		firstBounds = { x: bounds.x, y: bounds.y, w: bounds.w * ratio, h: bounds.h };
		secondBounds = {
			x: bounds.x + bounds.w * ratio,
			y: bounds.y,
			w: bounds.w * (1 - ratio),
			h: bounds.h,
		};
	} else {
		firstBounds = { x: bounds.x, y: bounds.y, w: bounds.w, h: bounds.h * ratio };
		secondBounds = {
			x: bounds.x,
			y: bounds.y + bounds.h * ratio,
			w: bounds.w,
			h: bounds.h * (1 - ratio),
		};
	}
	return [
		...collectPaneBounds(node.first, firstBounds),
		...collectPaneBounds(node.second, secondBounds),
	];
};

const center = (r: Rect): { cx: number; cy: number } => ({
	cx: r.x + r.w / 2,
	cy: r.y + r.h / 2,
});

export const findAdjacentPane = (
	root: TerminalLayoutNode,
	currentPaneId: string,
	direction: 'up' | 'down' | 'left' | 'right',
): string | null => {
	const allBounds = collectPaneBounds(root);
	const current = allBounds.find((b) => b.paneId === currentPaneId);
	if (!current) return null;
	const cc = center(current.bounds);

	let best: PaneBounds | null = null;
	let bestDist = Infinity;

	for (const candidate of allBounds) {
		if (candidate.paneId === currentPaneId) continue;
		const tc = center(candidate.bounds);
		let valid = false;
		switch (direction) {
			case 'left':
				valid = tc.cx < cc.cx;
				break;
			case 'right':
				valid = tc.cx > cc.cx;
				break;
			case 'up':
				valid = tc.cy < cc.cy;
				break;
			case 'down':
				valid = tc.cy > cc.cy;
				break;
		}
		if (!valid) continue;
		const dist = Math.abs(tc.cx - cc.cx) + Math.abs(tc.cy - cc.cy);
		if (dist < bestDist) {
			bestDist = dist;
			best = candidate;
		}
	}

	return best?.paneId ?? null;
};
