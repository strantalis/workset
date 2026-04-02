import { describe, expect, it } from 'vitest';
import {
	LAYOUT_VERSION,
	buildTab,
	collectLayoutTerminalIdsFromUnknown,
	moveTab,
	withSplit,
	type TerminalLayout,
} from './terminalLayoutModel';

describe('terminalLayoutModel', () => {
	it('buildTab creates a flat tab with a single focused pane', () => {
		const tab = buildTab('term-1', 'Terminal 1');

		expect(tab.title).toBe('Terminal 1');
		expect(tab.panes).toHaveLength(1);
		expect(tab.panes[0].terminalId).toBe('term-1');
		expect(tab.focusedPaneId).toBe(tab.panes[0].id);
	});

	it('withSplit adds at most one second pane and updates split metadata', () => {
		const tab = buildTab('term-1', 'Terminal 1');
		const split = withSplit(tab, { id: 'pane-2', terminalId: 'term-2' }, 'horizontal');

		expect(split.panes.map((pane) => pane.terminalId)).toEqual(['term-1', 'term-2']);
		expect(split.splitDirection).toBe('horizontal');
		expect(split.focusedPaneId).toBe('pane-2');

		const flipped = withSplit(split, { id: 'pane-3', terminalId: 'term-3' }, 'vertical');
		expect(flipped.panes.map((pane) => pane.terminalId)).toEqual(['term-1', 'term-2']);
		expect(flipped.splitDirection).toBe('vertical');
	});

	it('collectLayoutTerminalIdsFromUnknown supports flat layouts and one-time legacy cleanup', () => {
		const flatLayout = {
			version: LAYOUT_VERSION,
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
		const legacyLayout = {
			version: 1,
			root: {
				id: 'pane-legacy',
				kind: 'pane',
				tabs: [
					{ id: 'legacy-1', terminalId: 'term-3', title: 'Legacy 1' },
					{ id: 'legacy-2', terminalId: 'term-4', title: 'Legacy 2' },
				],
				activeTabId: 'legacy-1',
			},
		};
		const recursiveTabLayout = {
			version: 2,
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

		expect(collectLayoutTerminalIdsFromUnknown(flatLayout)).toEqual(['term-1', 'term-2']);
		expect(collectLayoutTerminalIdsFromUnknown(legacyLayout)).toEqual(['term-3', 'term-4']);
		expect(collectLayoutTerminalIdsFromUnknown(recursiveTabLayout)).toEqual(['term-5', 'term-6']);
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
