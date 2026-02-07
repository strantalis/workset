import { beforeEach, describe, expect, test, vi } from 'vitest';
import {
	fetchWorkspaceTerminalLayout,
	persistWorkspaceTerminalLayout,
} from './api/terminal-layout';
import { fetchWorkspaces, removeWorkspace } from './api/workspaces';
import {
	GetWorkspaceTerminalLayout,
	ListWorkspaceSnapshots,
	RemoveWorkspace,
	SetWorkspaceTerminalLayout,
} from '../../wailsjs/go/main/App';
import type { TerminalLayout } from './types';

vi.mock('../../wailsjs/go/main/App', () => ({
	GetWorkspaceTerminalLayout: vi.fn(),
	ListWorkspaceSnapshots: vi.fn(),
	RemoveWorkspace: vi.fn(),
	SetWorkspaceTerminalLayout: vi.fn(),
}));

describe('workspace + terminal API compatibility exports', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	test('fetchWorkspaces maps snapshot payloads through compatibility export', async () => {
		const snapshots = [
			{
				id: 'ws-1',
				name: 'Workspace 1',
				path: '/tmp/ws-1',
				createdAt: '2026-01-01T00:00:00.000Z',
				lastUsed: '2026-01-02T00:00:00.000Z',
				archived: false,
				repos: [
					{
						id: 'repo-1',
						name: 'repo-1',
						path: '/tmp/ws-1/repo-1',
						remote: 'origin',
						defaultBranch: 'main',
						dirty: true,
						missing: false,
						statusKnown: true,
					},
				],
				pinned: true,
				pinOrder: 2,
				color: '#123456',
				expanded: true,
			},
		] as unknown as Awaited<ReturnType<typeof ListWorkspaceSnapshots>>;
		vi.mocked(ListWorkspaceSnapshots).mockResolvedValue(snapshots);

		const result = await fetchWorkspaces(true, true);

		expect(ListWorkspaceSnapshots).toHaveBeenCalledWith({
			includeArchived: true,
			includeStatus: true,
		});
		expect(result).toEqual([
			{
				id: 'ws-1',
				name: 'Workspace 1',
				path: '/tmp/ws-1',
				archived: false,
				archivedAt: undefined,
				archivedReason: undefined,
				repos: [
					{
						id: 'repo-1',
						name: 'repo-1',
						path: '/tmp/ws-1/repo-1',
						remote: 'origin',
						defaultBranch: 'main',
						ahead: 0,
						behind: 0,
						dirty: true,
						missing: false,
						statusKnown: true,
						diff: { added: 0, removed: 0 },
						files: [],
					},
				],
				pinned: true,
				pinOrder: 2,
				color: '#123456',
				expanded: true,
				lastUsed: '2026-01-02T00:00:00.000Z',
			},
		]);
	});

	test('removeWorkspace preserves fetchRemotes default behavior via barrel export', async () => {
		await removeWorkspace('ws-1', { deleteFiles: true });

		expect(RemoveWorkspace).toHaveBeenCalledWith({
			workspaceId: 'ws-1',
			deleteFiles: true,
			force: false,
			fetchRemotes: true,
		});
	});

	test('terminal layout compatibility exports pass through to wails API', async () => {
		const layout: TerminalLayout = {
			version: 1,
			root: {
				id: 'pane-1',
				kind: 'pane',
				tabs: [{ id: 'tab-1', terminalId: 'term-1', title: 'Terminal' }],
				activeTabId: 'tab-1',
			},
			focusedPaneId: 'pane-1',
		};
		const terminalLayoutPayload = {
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout,
		} as unknown as Awaited<ReturnType<typeof GetWorkspaceTerminalLayout>>;
		vi.mocked(GetWorkspaceTerminalLayout).mockResolvedValue(terminalLayoutPayload);

		const result = await fetchWorkspaceTerminalLayout('ws-1');
		await persistWorkspaceTerminalLayout('ws-1', layout);

		expect(GetWorkspaceTerminalLayout).toHaveBeenCalledWith('ws-1');
		expect(SetWorkspaceTerminalLayout).toHaveBeenCalledWith({
			workspaceId: 'ws-1',
			layout,
		});
		expect(result).toEqual({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout,
		});
	});
});
