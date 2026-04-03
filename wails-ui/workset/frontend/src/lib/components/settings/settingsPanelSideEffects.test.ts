import { describe, expect, it, vi } from 'vitest';
import type { TerminalLayout, Workspace } from '../../types';
import {
	buildFreshLayout,
	collectTerminalIds,
	createSettingsPanelSideEffects,
	DEFAULT_UPDATE_PREFERENCES,
} from './settingsPanelSideEffects';

describe('settingsPanelSideEffects', () => {
	it('collectTerminalIds walks flat panes and legacy layouts during cleanup', () => {
		const ids = collectTerminalIds({
			version: 3,
			tabs: [
				{
					id: 'tab-1',
					title: 'One',
					panes: [
						{ id: 'left', terminalId: 'term-1' },
						{ id: 'right', terminalId: 'term-2' },
					],
					splitDirection: 'vertical',
					focusedPaneId: 'left',
				},
				{
					id: 'tab-2',
					title: 'Two',
					panes: [{ id: 'solo', terminalId: 'term-3' }],
					focusedPaneId: 'solo',
				},
			],
			activeTabId: 'tab-1',
		});

		expect(ids).toEqual(['term-1', 'term-2', 'term-3']);
	});

	it('buildFreshLayout creates a single-pane layout with generated IDs', () => {
		const idSequence = ['tab-abc', 'pane-def'];
		const layout = buildFreshLayout(
			'Workspace',
			'terminal-new',
			(name, index) => `${name}-${index}`,
			() => idSequence.shift() ?? 'fallback-id',
		);

		expect(layout.version).toBe(4);
		expect(layout.activeTabId).toBe('tab-abc');
		expect(layout.tabs).toEqual([
			{
				id: 'tab-abc',
				title: 'Workspace-0',
				root: {
					kind: 'pane',
					id: 'pane-def',
					terminalId: 'terminal-new',
				},
				focusedPaneId: 'pane-def',
			},
		]);
	});

	it('resetTerminalLayout stops existing sessions and persists a fresh layout', async () => {
		const stopWorkspaceTerminal = vi.fn().mockResolvedValue(undefined);
		const persistWorkspaceTerminalLayout = vi.fn().mockResolvedValue(undefined);
		const dispatchLayoutReset = vi.fn();

		const sideEffects = createSettingsPanelSideEffects({
			fetchWorkspaceTerminalLayout: vi.fn().mockResolvedValue({
				workspaceId: 'ws-1',
				workspacePath: '/tmp/ws-1',
				layout: {
					version: 1,
					root: {
						id: 'pane-existing',
						kind: 'pane',
						tabs: [
							{ id: 'tab-a', terminalId: 'term-a', title: 'A' },
							{ id: 'tab-b', terminalId: 'term-b', title: 'B' },
						],
						activeTabId: 'tab-a',
					},
					focusedPaneId: 'pane-existing',
				} as unknown as TerminalLayout,
			}),
			stopWorkspaceTerminal,
			createWorkspaceTerminal: vi.fn().mockResolvedValue({
				workspaceId: 'ws-1',
				terminalId: 'term-new',
			}),
			persistWorkspaceTerminalLayout,
			generateTerminalName: (workspaceName, index) => `${workspaceName}-${index}`,
			randomUUID: vi.fn().mockReturnValueOnce('tab-new').mockReturnValueOnce('pane-new'),
			dispatchLayoutReset,
		});

		const workspace = { id: 'ws-1', name: 'Workspace One' } as Workspace;
		const result = await sideEffects.resetTerminalLayout(workspace);

		expect(result.error).toBeUndefined();
		expect(result.success).toContain('Workspace One');
		expect(stopWorkspaceTerminal).toHaveBeenCalledTimes(2);
		expect(stopWorkspaceTerminal).toHaveBeenCalledWith('ws-1', 'term-a');
		expect(stopWorkspaceTerminal).toHaveBeenCalledWith('ws-1', 'term-b');
		expect(persistWorkspaceTerminalLayout).toHaveBeenCalledWith('ws-1', {
			version: 4,
			tabs: [
				{
					id: 'tab-new',
					title: 'Workspace One-0',
					root: {
						kind: 'pane',
						id: 'pane-new',
						terminalId: 'term-new',
					},
					focusedPaneId: 'pane-new',
				},
			],
			activeTabId: 'tab-new',
		});
		expect(dispatchLayoutReset).toHaveBeenCalledWith('ws-1');
	});

	it('setUpdateChannel returns a friendly error when update preference write fails', async () => {
		const sideEffects = createSettingsPanelSideEffects({
			setUpdatePreferences: vi.fn().mockRejectedValue(new Error('write failed')),
		});

		const result = await sideEffects.setUpdateChannel('alpha');
		expect(result.updatePreferences).toBeUndefined();
		expect(result.error).toBeTruthy();
	});

	it('setAutoCheck writes the auto-check preference', async () => {
		const setUpdatePreferences = vi.fn().mockResolvedValue({
			channel: 'stable',
			autoCheck: false,
			dismissedVersion: '',
		});
		const sideEffects = createSettingsPanelSideEffects({
			setUpdatePreferences,
		});

		const result = await sideEffects.setAutoCheck(false);
		expect(setUpdatePreferences).toHaveBeenCalledWith({ autoCheck: false });
		expect(result.updatePreferences).toEqual({
			channel: 'stable',
			autoCheck: false,
			dismissedVersion: '',
		});
	});

	it('loadUpdateBootstrap falls back to defaults on fetch failures', async () => {
		const sideEffects = createSettingsPanelSideEffects({
			fetchUpdatePreferences: vi.fn().mockRejectedValue(new Error('boom')),
			fetchUpdateState: vi.fn().mockRejectedValue(new Error('boom')),
		});

		const result = await sideEffects.loadUpdateBootstrap();
		expect(result.updatePreferences).toEqual(DEFAULT_UPDATE_PREFERENCES);
		expect(result.updateState).toBeNull();
	});
});
