import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPopoutManager } from './createPopoutManager.svelte';
import type { Workspace } from '../types';

const { openWorkspacePopout, closeWorkspacePopout, listWorkspacePopouts } = vi.hoisted(() => ({
	openWorkspacePopout: vi.fn(),
	closeWorkspacePopout: vi.fn(),
	listWorkspacePopouts: vi.fn(),
}));

vi.mock('../api/workspaces', () => ({
	openWorkspacePopout,
	closeWorkspacePopout,
	listWorkspacePopouts,
}));

function createWorkspace(id: string, workset: string): Workspace {
	return {
		id,
		name: id,
		path: `/tmp/${id}`,
		repos: [],
		workset,
		worksetKey: workset,
		worksetLabel: workset,
		placeholder: false,
		archived: false,
		archivedAt: '',
		archivedReason: '',
		pinned: false,
		pinOrder: 0,
		color: '',
		description: '',
		expanded: false,
		lastUsed: '',
	};
}

describe('createPopoutManager', () => {
	beforeEach(() => {
		openWorkspacePopout.mockReset();
		closeWorkspacePopout.mockReset();
		listWorkspacePopouts.mockReset();
	});

	it('hydrates popout state without requiring terminal release hooks', async () => {
		listWorkspacePopouts.mockResolvedValue([
			{ workspaceId: 'ws-1', windowName: 'workspace-ws-1-popout', open: true },
		]);

		const manager = createPopoutManager({
			popoutMode: false,
			getWorksetThreads: () => [],
		});

		await manager.loadState();

		expect(manager.openPopouts).toEqual({
			'ws-1': 'workspace-ws-1-popout',
		});
		expect(manager.isWorkspacePoppedOut('ws-1')).toBe(true);
	});

	it('opens and closes popouts while keeping open-state updates idempotent', async () => {
		openWorkspacePopout.mockResolvedValue({
			workspaceId: 'ws-1',
			windowName: 'workspace-ws-1-popout',
			open: true,
		});
		closeWorkspacePopout.mockResolvedValue(undefined);

		const manager = createPopoutManager({
			popoutMode: false,
			getWorksetThreads: () => [],
		});

		await manager.handlePopout('ws-1', true);
		manager.updateState('ws-1', 'workspace-ws-1-popout', true);

		expect(openWorkspacePopout).toHaveBeenCalledWith('ws-1');
		expect(manager.openPopouts).toEqual({
			'ws-1': 'workspace-ws-1-popout',
		});

		await manager.handlePopout('ws-1', false);

		expect(closeWorkspacePopout).toHaveBeenCalledWith('ws-1');
		expect(manager.openPopouts).toEqual({});
	});

	it('prefers the already-open thread when resolving a workset popout id', () => {
		const workspaces = [
			createWorkspace('thread-a', 'ask-gill'),
			createWorkspace('thread-b', 'ask-gill'),
		];
		const manager = createPopoutManager({
			popoutMode: false,
			getWorksetThreads: (workspaceId) => (workspaceId === 'ask-gill' ? workspaces : []),
		});

		manager.updateState('thread-b', 'workspace-thread-b-popout', true);

		expect(manager.resolvePopoutWorkspaceId('ask-gill')).toBe('thread-b');
	});
});
