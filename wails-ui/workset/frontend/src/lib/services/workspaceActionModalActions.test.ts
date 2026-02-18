import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { HookExecution } from '../types';
import type { WorkspaceActionPendingHook } from './workspaceActionHooks';
import { trustWorkspaceActionPendingHook } from './workspaceActionModalActions';
import { runRepoHooks, trustRepoHooks } from '../api/workspaces';

vi.mock('../api/workspaces', async (importOriginal) => {
	const actual = await importOriginal<typeof import('../api/workspaces')>();
	return {
		...actual,
		runRepoHooks: vi.fn(),
		trustRepoHooks: vi.fn(),
	};
});

const pendingHook = (repo: string): WorkspaceActionPendingHook => ({
	event: 'workspace.create',
	repo,
	hooks: ['bootstrap'],
});

describe('workspaceActionModalActions trust flow', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('trusts and then immediately runs pending hooks', async () => {
		vi.mocked(trustRepoHooks).mockResolvedValue(undefined);
		vi.mocked(runRepoHooks).mockResolvedValue({
			event: 'workspace.create',
			repo: 'repo-a',
			results: [{ id: 'bootstrap', status: 'ok', log_path: '/tmp/hooks/bootstrap.log' }],
		});

		let pendingHooks: WorkspaceActionPendingHook[] = [pendingHook('repo-a'), pendingHook('repo-b')];
		let hookRuns: HookExecution[] = [];

		await trustWorkspaceActionPendingHook({
			pending: pendingHooks[0],
			pendingHooks,
			hookRuns,
			workspaceReferences: ['', 'workspace-alpha'],
			activeHookOperation: 'workspace.create',
			getPendingHooks: () => pendingHooks,
			getHookRuns: () => hookRuns,
			setPendingHooks: (next) => (pendingHooks = next),
			setHookRuns: (next) => (hookRuns = next),
		});

		expect(trustRepoHooks).toHaveBeenCalledTimes(1);
		expect(trustRepoHooks).toHaveBeenCalledWith('repo-a');
		expect(runRepoHooks).toHaveBeenCalledTimes(1);
		expect(runRepoHooks).toHaveBeenCalledWith(
			'workspace-alpha',
			'repo-a',
			'workspace.create',
			'workspace.create.ui',
		);
		expect(pendingHooks).toEqual([pendingHook('repo-b')]);
		expect(hookRuns).toEqual([
			{
				event: 'workspace.create',
				repo: 'repo-a',
				id: 'bootstrap',
				status: 'ok',
				log_path: '/tmp/hooks/bootstrap.log',
			},
		]);
	});

	it('does not run hooks when trust fails', async () => {
		vi.mocked(trustRepoHooks).mockRejectedValue(new Error('trust denied'));
		vi.mocked(runRepoHooks).mockResolvedValue({
			event: 'workspace.create',
			repo: 'repo-a',
			results: [{ id: 'bootstrap', status: 'ok', log_path: '/tmp/hooks/bootstrap.log' }],
		});

		let pendingHooks: WorkspaceActionPendingHook[] = [pendingHook('repo-a')];
		let hookRuns: HookExecution[] = [];

		await trustWorkspaceActionPendingHook({
			pending: pendingHooks[0],
			pendingHooks,
			hookRuns,
			workspaceReferences: ['workspace-alpha'],
			activeHookOperation: 'workspace.create',
			getPendingHooks: () => pendingHooks,
			getHookRuns: () => hookRuns,
			setPendingHooks: (next) => (pendingHooks = next),
			setHookRuns: (next) => (hookRuns = next),
		});

		expect(runRepoHooks).not.toHaveBeenCalled();
		expect(pendingHooks).toEqual([
			{
				...pendingHook('repo-a'),
				trusting: false,
				runError: 'trust denied',
			},
		]);
		expect(hookRuns).toEqual([]);
	});
});
