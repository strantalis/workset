import { beforeEach, describe, expect, test, vi } from 'vitest';
import {
	fetchWorkspaceTerminalLayout,
	persistWorkspaceTerminalLayout,
} from './api/terminal-layout';
import { fetchWorkspaces, previewRepoHooks, removeWorkspace } from './api/workspaces';
import {
	CloseWorkspacePopout,
	GetWorkspaceTerminalLayout,
	ListWorkspaceSnapshots,
	OpenWorkspacePopout,
	PreviewRepoHooks,
	RemoveWorkspace,
	SetWorkspaceTerminalLayout,
} from '../../bindings/workset/app';
import type { TerminalLayout } from './types';

vi.mock('./terminal/terminalService', () => ({
	flushWorkspaceTerminalSnapshots: vi.fn(),
}));

vi.mock('../../bindings/workset/app', () => ({
	GetWorkspaceTerminalLayout: vi.fn(),
	ListWorkspaceSnapshots: vi.fn(),
	CloseWorkspacePopout: vi.fn(),
	OpenWorkspacePopout: vi.fn(),
	PreviewRepoHooks: vi.fn(),
	RemoveWorkspace: vi.fn(),
	SetWorkspaceTerminalLayout: vi.fn(),
}));

import { flushWorkspaceTerminalSnapshots } from './terminal/terminalService';

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
						currentBranch: 'feature/redesign',
						ahead: 3,
						behind: 1,
						dirty: true,
						missing: false,
						statusKnown: true,
						diff: { added: 2, removed: 1 },
						files: [{ path: 'README.md', added: 2, removed: 1 }],
						trackedPullRequest: {
							repo: 'repo-1',
							number: 7,
							url: 'https://github.com/example/repo-1/pull/7',
							title: 'Integrate redesign API data',
							state: 'open',
							draft: false,
							baseRepo: 'example/repo-1',
							baseBranch: 'main',
							headRepo: 'example/repo-1',
							headBranch: 'feature/redesign',
							updatedAt: '2026-02-09T10:00:00.000Z',
							reviewCommentsCount: 2,
						},
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
				description: undefined,
				archived: false,
				archivedAt: undefined,
				archivedReason: undefined,
				workset: undefined,
				worksetKey: 'ws-1',
				worksetLabel: 'Workspace 1',
				placeholder: false,
				repos: [
					{
						id: 'repo-1',
						name: 'repo-1',
						path: '/tmp/ws-1/repo-1',
						remote: 'origin',
						defaultBranch: 'main',
						currentBranch: 'feature/redesign',
						ahead: 3,
						behind: 1,
						dirty: true,
						missing: false,
						statusKnown: true,
						trackedPullRequest: {
							repo: 'repo-1',
							number: 7,
							url: 'https://github.com/example/repo-1/pull/7',
							title: 'Integrate redesign API data',
							body: undefined,
							state: 'open',
							draft: false,
							baseRepo: 'example/repo-1',
							baseBranch: 'main',
							headRepo: 'example/repo-1',
							headBranch: 'feature/redesign',
							updatedAt: '2026-02-09T10:00:00.000Z',
							merged: false,
							commentsCount: 0,
							reviewCommentsCount: 2,
						},
						diff: { added: 2, removed: 1 },
						files: [{ path: 'README.md', added: 2, removed: 1, hunks: [] }],
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

	test('previewRepoHooks maps hook preview payload to hook IDs', async () => {
		vi.mocked(PreviewRepoHooks).mockResolvedValue({
			source: 'git@github.com:example/repo.git',
			exists: true,
			hooks: [
				{ id: 'bootstrap', on: ['workspace.create'], run: ['npm install'] },
				{ id: '', on: ['workspace.create'], run: ['npm run build', 'npm test'] },
				{ id: 'bootstrap', on: ['workspace.create'], run: ['npm install'] },
			],
		} as Awaited<ReturnType<typeof PreviewRepoHooks>>);

		const preview = await previewRepoHooks('git@github.com:example/repo.git');

		expect(PreviewRepoHooks).toHaveBeenCalledWith({
			source: 'git@github.com:example/repo.git',
			ref: undefined,
		});
		expect(preview).toEqual({
			hooks: ['bootstrap', 'npm run build && npm test'],
			previewUnavailableReason: null,
		});
	});

	test('previewRepoHooks preserves auth-required soft miss state', async () => {
		vi.mocked(PreviewRepoHooks).mockResolvedValue({
			source: 'git@github.com:example/private-repo.git',
			exists: false,
			preview_unavailable_reason: 'auth_required',
		} as Awaited<ReturnType<typeof PreviewRepoHooks>>);

		const preview = await previewRepoHooks('git@github.com:example/private-repo.git');

		expect(preview).toEqual({
			hooks: [],
			previewUnavailableReason: 'auth_required',
		});
	});

	test('terminal layout compatibility exports pass through to wails API', async () => {
		const layout: TerminalLayout = {
			version: 2,
			tabs: [
				{
					id: 'tab-1',
					title: 'Terminal',
					root: {
						id: 'pane-1',
						kind: 'pane',
						terminalId: 'term-1',
					},
					focusedPaneId: 'pane-1',
				},
			],
			activeTabId: 'tab-1',
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

	test('openWorkspacePopout flushes terminal snapshots before opening the popout window', async () => {
		vi.mocked(OpenWorkspacePopout).mockResolvedValue({
			workspaceId: 'ws-1',
			windowName: 'workspace-ws-1-popout',
			open: true,
		} as Awaited<ReturnType<typeof OpenWorkspacePopout>>);

		const { openWorkspacePopout } = await import('./api/workspaces');
		await openWorkspacePopout('ws-1');

		expect(flushWorkspaceTerminalSnapshots).toHaveBeenCalledWith('ws-1');
		expect(OpenWorkspacePopout).toHaveBeenCalledWith('ws-1');
		expect(vi.mocked(flushWorkspaceTerminalSnapshots).mock.invocationCallOrder[0]).toBeLessThan(
			vi.mocked(OpenWorkspacePopout).mock.invocationCallOrder[0],
		);
	});

	test('closeWorkspacePopout flushes terminal snapshots before returning the popout window', async () => {
		const { closeWorkspacePopout } = await import('./api/workspaces');
		await closeWorkspacePopout('ws-1');

		expect(flushWorkspaceTerminalSnapshots).toHaveBeenCalledWith('ws-1');
		expect(CloseWorkspacePopout).toHaveBeenCalledWith('ws-1');
		expect(vi.mocked(flushWorkspaceTerminalSnapshots).mock.invocationCallOrder[0]).toBeLessThan(
			vi.mocked(CloseWorkspacePopout).mock.invocationCallOrder[0],
		);
	});
});
