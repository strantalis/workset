import { describe, expect, it } from 'vitest';
import {
	LAYOUT_VERSION,
	buildLeaf,
	buildTab,
	canSplit,
	closePane,
	collectLayoutTerminalIdsFromUnknown,
	collectLeaves,
	countLeaves,
	findAdjacentPane,
	findLeaf,
	moveTab,
	normalizeLayout,
	splitPane,
	updateNodeRatio,
	type TerminalLayout,
} from './terminalLayoutModel';

describe('terminalLayoutModel', () => {
	it('buildTab creates a tab with a single leaf root', () => {
		const tab = buildTab('term-1', 'Terminal 1');

		expect(tab.title).toBe('Terminal 1');
		expect(tab.root.kind).toBe('pane');
		expect(tab.root.kind === 'pane' && tab.root.terminalId).toBe('term-1');
		expect(tab.focusedPaneId).toBe(tab.root.id);
	});

	it('splitPane splits a leaf into a split node', () => {
		const tab = buildTab('term-1', 'Terminal 1');
		const newLeaf = buildLeaf('term-2');
		const result = splitPane(tab.root, tab.root.id, newLeaf, 'horizontal');

		expect(result).not.toBeNull();
		expect(result!.kind).toBe('split');
		if (result!.kind === 'split') {
			expect(result!.direction).toBe('horizontal');
			expect(result!.first).toBe(tab.root);
			expect(result!.second).toBe(newLeaf);
		}
	});

	it('splitPane respects max depth of 2', () => {
		const tab = buildTab('term-1', 'Terminal 1');
		const leaf2 = buildLeaf('term-2');
		const depth1 = splitPane(tab.root, tab.root.id, leaf2, 'vertical')!;

		// Split one of the children (depth 1 -> 2)
		const leaf3 = buildLeaf('term-3');
		const depth2 = splitPane(depth1, leaf2.id, leaf3, 'horizontal')!;
		expect(depth2).not.toBeNull();
		expect(countLeaves(depth2)).toBe(3);

		// Try to split at depth 2 — should return null
		const leaf4 = buildLeaf('term-4');
		const tooDeep = splitPane(depth2, leaf3.id, leaf4, 'vertical');
		expect(tooDeep).toBeNull();
	});

	it('splitPane allows max 4 panes with depth 2', () => {
		const tab = buildTab('term-1', 'Terminal 1');
		const leaf2 = buildLeaf('term-2');
		const root1 = splitPane(tab.root, tab.root.id, leaf2, 'vertical')!;

		// Split first child
		const leaf3 = buildLeaf('term-3');
		const root2 = splitPane(root1, tab.root.id, leaf3, 'horizontal')!;

		// Split second child
		const leaf4 = buildLeaf('term-4');
		const root3 = splitPane(root2, leaf2.id, leaf4, 'horizontal')!;

		expect(countLeaves(root3)).toBe(4);
		const leaves = collectLeaves(root3);
		expect(leaves.map((l) => l.terminalId)).toEqual(['term-1', 'term-3', 'term-2', 'term-4']);
	});

	it('closePane removes a leaf and returns sibling', () => {
		const tab = buildTab('term-1', 'Terminal 1');
		const leaf2 = buildLeaf('term-2');
		const split = splitPane(tab.root, tab.root.id, leaf2, 'vertical')!;

		const result = closePane(split, leaf2.id);
		expect(result).not.toBeNull();
		expect(result!.kind).toBe('pane');
		expect(result!.id).toBe(tab.root.id);
	});

	it('closePane in a 3-pane tree returns a 2-pane tree', () => {
		const tab = buildTab('term-1', 'Terminal 1');
		const leaf2 = buildLeaf('term-2');
		const leaf3 = buildLeaf('term-3');
		const root1 = splitPane(tab.root, tab.root.id, leaf2, 'vertical')!;
		const root2 = splitPane(root1, leaf2.id, leaf3, 'horizontal')!;

		const result = closePane(root2, leaf3.id);
		expect(result).not.toBeNull();
		expect(countLeaves(result!)).toBe(2);
	});

	it('canSplit returns false for panes at max depth', () => {
		const tab = buildTab('term-1', 'Terminal 1');
		// Single leaf at depth 0 — can split
		expect(canSplit(tab.root, tab.root.id)).toBe(true);

		const leaf2 = buildLeaf('term-2');
		const split = splitPane(tab.root, tab.root.id, leaf2, 'vertical')!;
		// Both leaves at depth 1 — can split
		expect(canSplit(split, tab.root.id)).toBe(true);
		expect(canSplit(split, leaf2.id)).toBe(true);

		const leaf3 = buildLeaf('term-3');
		const deep = splitPane(split, leaf2.id, leaf3, 'horizontal')!;
		// leaf2 and leaf3 are at depth 2 — can't split further
		expect(canSplit(deep, leaf2.id)).toBe(false);
		expect(canSplit(deep, leaf3.id)).toBe(false);
		// tab.root is at depth 1 — CAN still split (depth 1 < MAX_DEPTH 2)
		expect(canSplit(deep, tab.root.id)).toBe(true);
	});

	it('updateNodeRatio clamps and updates the correct split', () => {
		const tab = buildTab('term-1', 'Terminal 1');
		const leaf2 = buildLeaf('term-2');
		const split = splitPane(tab.root, tab.root.id, leaf2, 'vertical')!;

		const updated = updateNodeRatio(split, split.id, 0.7);
		expect(updated.kind === 'split' && updated.ratio).toBe(0.7);

		const clamped = updateNodeRatio(split, split.id, 0.95);
		expect(clamped.kind === 'split' && clamped.ratio).toBe(0.85);
	});

	it('findLeaf locates a leaf by pane ID', () => {
		const tab = buildTab('term-1', 'Terminal 1');
		const leaf2 = buildLeaf('term-2');
		const split = splitPane(tab.root, tab.root.id, leaf2, 'vertical')!;

		expect(findLeaf(split, tab.root.id)?.terminalId).toBe('term-1');
		expect(findLeaf(split, leaf2.id)?.terminalId).toBe('term-2');
		expect(findLeaf(split, 'nonexistent')).toBeNull();
	});

	it('findAdjacentPane navigates spatially', () => {
		const tab = buildTab('term-1', 'Terminal 1');
		const leaf2 = buildLeaf('term-2');
		const split = splitPane(tab.root, tab.root.id, leaf2, 'vertical')!;

		// Vertical split: term-1 is left, term-2 is right
		expect(findAdjacentPane(split, tab.root.id, 'right')).toBe(leaf2.id);
		expect(findAdjacentPane(split, leaf2.id, 'left')).toBe(tab.root.id);
		expect(findAdjacentPane(split, tab.root.id, 'up')).toBeNull();
		expect(findAdjacentPane(split, tab.root.id, 'down')).toBeNull();
	});

	it('normalizeLayout migrates v3 flat layouts to v4 tree', () => {
		const v3Layout = {
			version: 3,
			tabs: [
				{
					id: 'tab-1',
					title: 'Terminal',
					panes: [
						{ id: 'pane-1', terminalId: 'term-1' },
						{ id: 'pane-2', terminalId: 'term-2' },
					],
					splitDirection: 'vertical',
					splitRatio: 0.4,
					focusedPaneId: 'pane-1',
				},
			],
			activeTabId: 'tab-1',
		};

		const result = normalizeLayout(v3Layout);
		expect(result).not.toBeNull();
		expect(result!.version).toBe(LAYOUT_VERSION);
		expect(result!.tabs[0].root.kind).toBe('split');
		if (result!.tabs[0].root.kind === 'split') {
			expect(result!.tabs[0].root.direction).toBe('vertical');
			expect(result!.tabs[0].root.ratio).toBe(0.4);
			expect(result!.tabs[0].root.first.kind).toBe('pane');
			expect(result!.tabs[0].root.second.kind).toBe('pane');
		}
	});

	it('normalizeLayout handles single-pane v3 migration', () => {
		const v3Layout = {
			version: 3,
			tabs: [
				{
					id: 'tab-1',
					title: 'Terminal',
					panes: [{ id: 'pane-1', terminalId: 'term-1' }],
					focusedPaneId: 'pane-1',
				},
			],
			activeTabId: 'tab-1',
		};

		const result = normalizeLayout(v3Layout);
		expect(result).not.toBeNull();
		expect(result!.tabs[0].root.kind).toBe('pane');
	});

	it('collectLayoutTerminalIdsFromUnknown supports v3, v4, and legacy formats', () => {
		const v3Layout = {
			version: 3,
			tabs: [
				{
					id: 'tab-1',
					title: 'One',
					panes: [
						{ id: 'pane-1', terminalId: 'term-1' },
						{ id: 'pane-2', terminalId: 'term-2' },
					],
					splitDirection: 'vertical',
					focusedPaneId: 'pane-1',
				},
			],
			activeTabId: 'tab-1',
		};

		const v4Layout = {
			version: 4,
			tabs: [
				{
					id: 'tab-2',
					title: 'Two',
					root: {
						id: 'split-1',
						kind: 'split',
						first: { id: 'pane-left', kind: 'pane', terminalId: 'term-5' },
						second: { id: 'pane-right', kind: 'pane', terminalId: 'term-6' },
					},
					focusedPaneId: 'pane-left',
				},
			],
			activeTabId: 'tab-2',
		};

		const legacyLayout = {
			version: 1,
			root: {
				id: 'pane-legacy',
				kind: 'pane',
				terminalId: 'term-3',
			},
		};

		expect(collectLayoutTerminalIdsFromUnknown(v3Layout)).toEqual(['term-1', 'term-2']);
		expect(collectLayoutTerminalIdsFromUnknown(v4Layout)).toEqual(['term-5', 'term-6']);
		expect(collectLayoutTerminalIdsFromUnknown(legacyLayout)).toEqual(['term-3']);
	});

	it('moveTab reorders workspace tabs without changing the active tab id', () => {
		const first = buildTab('term-1', 'One');
		const second = buildTab('term-2', 'Two');
		const third = buildTab('term-3', 'Three');
		const layout: TerminalLayout = {
			version: LAYOUT_VERSION,
			tabs: [first, second, third],
			activeTabId: second.id,
		};

		const moved = moveTab(layout, 1, 0);

		expect(moved.tabs.map((tab) => tab.id)).toEqual([second.id, first.id, third.id]);
		expect(moved.activeTabId).toBe(second.id);
	});
});
