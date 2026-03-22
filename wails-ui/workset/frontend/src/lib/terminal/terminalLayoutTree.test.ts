import { describe, expect, it } from 'vitest';
import {
	LAYOUT_VERSION,
	buildTab,
	collectTerminalIdsFromUnknownLayout,
	moveTab,
	type TerminalLayout,
} from './terminalLayoutTree';

describe('terminalLayoutTree', () => {
	it('buildTab creates a top-level tab with a single focused pane', () => {
		const tab = buildTab('term-1', 'Terminal 1');

		expect(tab.title).toBe('Terminal 1');
		expect(tab.root.kind).toBe('pane');
		if (tab.root.kind !== 'pane') {
			throw new Error('expected pane root');
		}
		expect(tab.root.terminalId).toBe('term-1');
		expect(tab.focusedPaneId).toBe(tab.root.id);
	});

	it('collectTerminalIdsFromUnknownLayout supports both legacy and v2 layouts', () => {
		const v2Layout = {
			version: LAYOUT_VERSION,
			tabs: [
				{
					id: 'tab-1',
					title: 'One',
					root: {
						id: 'split-1',
						kind: 'split',
						direction: 'row',
						ratio: 0.5,
						first: { id: 'pane-1', kind: 'pane', terminalId: 'term-1' },
						second: { id: 'pane-2', kind: 'pane', terminalId: 'term-2' },
					},
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
			focusedPaneId: 'pane-legacy',
		};

		expect(collectTerminalIdsFromUnknownLayout(v2Layout)).toEqual(['term-1', 'term-2']);
		expect(collectTerminalIdsFromUnknownLayout(legacyLayout)).toEqual(['term-3', 'term-4']);
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
