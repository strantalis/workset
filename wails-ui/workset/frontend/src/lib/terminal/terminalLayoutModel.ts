import type {
	TerminalLayout as TerminalLayoutType,
	TerminalLayoutPane as TerminalLayoutPaneType,
	TerminalLayoutTab as TerminalLayoutTabType,
	TerminalSplitDirection as TerminalSplitDirectionType,
} from '../types';
import type { TerminalSnapshotLike } from './terminalEmulatorContracts';

export type TerminalLayoutPane = TerminalLayoutPaneType;
export type TerminalTab = TerminalLayoutTabType;
export type TerminalLayout = TerminalLayoutType;
export type TerminalSplitDirection = TerminalSplitDirectionType;

export const LAYOUT_VERSION = 3;
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

const normalizePane = (pane: unknown): TerminalLayoutPane | null => {
	if (!pane || typeof pane !== 'object') {
		return null;
	}
	const candidate = pane as {
		id?: unknown;
		terminalId?: unknown;
	};
	if (typeof candidate.terminalId !== 'string' || !candidate.terminalId.trim()) {
		return null;
	}
	return {
		id: coerceId(candidate.id),
		terminalId: candidate.terminalId,
		snapshot: normalizeSnapshot((candidate as { snapshot?: unknown }).snapshot),
	};
};

const normalizePanes = (panes: unknown): TerminalLayoutPane[] => {
	if (!Array.isArray(panes)) {
		return [];
	}
	const normalized = panes
		.map((pane) => normalizePane(pane))
		.filter((pane): pane is TerminalLayoutPane => pane !== null);
	if (normalized.length === 0) {
		return [];
	}
	return normalized.slice(0, 2);
};

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

const normalizeTab = (tab: unknown): TerminalTab | null => {
	if (!tab || typeof tab !== 'object') {
		return null;
	}
	const candidate = tab as {
		id?: unknown;
		title?: unknown;
		panes?: unknown;
		splitDirection?: unknown;
		splitRatio?: unknown;
		focusedPaneId?: unknown;
	};
	const panes = normalizePanes(candidate.panes);
	if (panes.length === 0) {
		return null;
	}
	const title =
		typeof candidate.title === 'string' && candidate.title.trim().length > 0
			? candidate.title
			: 'Terminal';
	const focusedPaneId =
		typeof candidate.focusedPaneId === 'string' &&
		panes.some((pane) => pane.id === candidate.focusedPaneId)
			? candidate.focusedPaneId
			: panes[0].id;
	const normalized: TerminalTab = {
		id: coerceId(candidate.id),
		title,
		panes,
		focusedPaneId,
	};
	if (panes.length === 2) {
		normalized.splitDirection = normalizeSplitDirection(candidate.splitDirection);
		normalized.splitRatio = normalizeSplitRatio(candidate.splitRatio);
	}
	return normalized;
};

export const normalizeLayout = (candidate: unknown): TerminalLayout | null => {
	if (!candidate || typeof candidate !== 'object') {
		return null;
	}
	const layout = candidate as {
		version?: unknown;
		tabs?: unknown;
		activeTabId?: unknown;
	};
	if (layout.version !== LAYOUT_VERSION) {
		return null;
	}
	const tabs = Array.isArray(layout.tabs)
		? layout.tabs.map((tab) => normalizeTab(tab)).filter((tab): tab is TerminalTab => tab !== null)
		: [];
	if (tabs.length === 0) {
		return null;
	}
	const activeTabId =
		typeof layout.activeTabId === 'string' && tabs.some((tab) => tab.id === layout.activeTabId)
			? layout.activeTabId
			: tabs[0].id;
	return {
		version: LAYOUT_VERSION,
		tabs,
		activeTabId,
	};
};

export const buildPane = (terminalId: string): TerminalLayoutPane => ({
	id: newId(),
	terminalId,
});

export const buildTab = (terminalId: string, title: string): TerminalTab => {
	const pane = buildPane(terminalId);
	return {
		id: newId(),
		title,
		panes: [pane],
		focusedPaneId: pane.id,
	};
};

export const ensureFocusedPane = (tab: TerminalTab): TerminalTab => {
	if (tab.panes.length === 0) {
		return tab;
	}
	if (tab.panes.some((pane) => pane.id === tab.focusedPaneId)) {
		return tab;
	}
	return {
		...tab,
		focusedPaneId: tab.panes[0].id,
	};
};

export const ensureActiveTab = (layout: TerminalLayout): TerminalLayout => {
	const tabs = layout.tabs.map(ensureFocusedPane);
	const activeTabId = tabs.some((tab) => tab.id === layout.activeTabId)
		? layout.activeTabId
		: (tabs[0]?.id ?? '');
	return {
		...layout,
		tabs,
		activeTabId,
	};
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
	if (sourceIndex === targetIndex) {
		return layout;
	}
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
	if (!tab) {
		return layout;
	}
	tabs.splice(targetIndex, 0, tab);
	return {
		...layout,
		tabs,
	};
};

export const collectTabTerminalIds = (tab: TerminalTab): string[] =>
	tab.panes.map((pane) => pane.terminalId);

export const collectLayoutTerminalIds = (layout: TerminalLayout): string[] =>
	layout.tabs.flatMap((tab) => collectTabTerminalIds(tab));

export const collectLayoutTerminalIdsFromUnknown = (layout: unknown): string[] => {
	if (!layout || typeof layout !== 'object') {
		return [];
	}
	const terminalIds = new Set<string>();

	const collectLegacyNode = (node: unknown): void => {
		if (!node || typeof node !== 'object') return;
		const candidate = node as {
			kind?: unknown;
			terminalId?: unknown;
			first?: unknown;
			second?: unknown;
			tabs?: Array<{ terminalId?: unknown }>;
		};
		if (typeof candidate.terminalId === 'string' && candidate.terminalId.trim()) {
			terminalIds.add(candidate.terminalId);
		}
		if (Array.isArray(candidate.tabs)) {
			for (const tab of candidate.tabs) {
				if (typeof tab?.terminalId === 'string' && tab.terminalId.trim()) {
					terminalIds.add(tab.terminalId);
				}
			}
		}
		collectLegacyNode(candidate.first);
		collectLegacyNode(candidate.second);
	};

	const candidate = layout as {
		tabs?: Array<{
			panes?: Array<{ terminalId?: unknown }>;
			root?: unknown;
		}>;
		root?: unknown;
	};
	if (Array.isArray(candidate.tabs)) {
		for (const tab of candidate.tabs) {
			if (Array.isArray(tab?.panes)) {
				for (const pane of tab.panes) {
					if (typeof pane?.terminalId === 'string' && pane.terminalId.trim()) {
						terminalIds.add(pane.terminalId);
					}
				}
			}
			collectLegacyNode(tab?.root);
		}
	}
	collectLegacyNode(candidate.root);
	return Array.from(terminalIds);
};

export const collapseTabToPane = (tab: TerminalTab, paneIdToKeep: string): TerminalTab => {
	const pane = tab.panes.find((candidate) => candidate.id === paneIdToKeep) ?? tab.panes[0];
	return {
		id: tab.id,
		title: tab.title,
		panes: pane ? [pane] : [],
		focusedPaneId: pane?.id,
	};
};

export const withSplit = (
	tab: TerminalTab,
	pane: TerminalLayoutPane,
	direction: TerminalSplitDirection,
	ratio = DEFAULT_SPLIT_RATIO,
): TerminalTab => {
	if (tab.panes.length >= 2) {
		return {
			...tab,
			splitDirection: direction,
			splitRatio: ratio,
		};
	}
	return {
		...tab,
		panes: [...tab.panes, pane],
		splitDirection: direction,
		splitRatio: ratio,
		focusedPaneId: pane.id,
	};
};
