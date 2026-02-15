import { describe, expect, it, vi } from 'vitest';
import type { HookExecution, RepoAddResponse, WorkspaceCreateResponse } from '../../types';
import {
	createWorkspaceActionMutationService,
	evaluateHookTransition,
	runArchiveWorkspaceMutation,
	runAddItemsMutation,
	runCreateWorkspaceMutation,
	runRemoveRepoMutation,
	runRemoveWorkspaceMutation,
	runRenameWorkspaceMutation,
} from '../../services/workspaceActionService';

const buildRepoAddResponse = (overrides: Partial<RepoAddResponse> = {}): RepoAddResponse => ({
	payload: {
		status: 'ok',
		workspace: 'ws-1',
		repo: 'repo-a',
		local_path: '/tmp/repo-a',
		managed: true,
	},
	...overrides,
});

describe('runCreateWorkspaceMutation', () => {
	it('runs create mutation and returns normalized hook state', async () => {
		const registerRepo = vi.fn(async () => undefined);
		const createWorkspace = vi.fn(
			async (): Promise<WorkspaceCreateResponse> => ({
				workspace: {
					name: 'alpha',
					path: '/tmp/alpha',
					workset: '/tmp/alpha/workset.yaml',
					branch: 'main',
					next: 'next',
				},
				warnings: ['warning-one', 'warning-one'],
				pendingHooks: [{ event: 'workspace.create', repo: 'repo-a', hooks: ['post-checkout'] }],
				hookRuns: [
					{
						event: 'workspace.create',
						repo: 'repo-a',
						id: 'hook-1',
						status: 'ok',
						log_path: '/tmp/hook-1.log',
					},
				],
			}),
		);

		const result = await runCreateWorkspaceMutation(
			{
				finalName: 'alpha',
				primaryInput: '/tmp/repos/pending',
				directRepos: [
					{ url: '/tmp/repos/first', register: true },
					{ url: 'git@github.com:acme/second.git', register: false },
				],
				selectedAliases: new Set(['alias-one']),
				selectedGroups: new Set(['group-one']),
			},
			{ registerRepo, createWorkspace },
		);

		expect(registerRepo).toHaveBeenCalledTimes(2);
		expect(registerRepo).toHaveBeenNthCalledWith(1, 'first', '/tmp/repos/first', '', '');
		expect(registerRepo).toHaveBeenNthCalledWith(2, 'pending', '/tmp/repos/pending', '', '');
		expect(createWorkspace).toHaveBeenCalledWith(
			'alpha',
			'',
			'group-one',
			['first', 'git@github.com:acme/second.git', 'pending', 'alias-one'],
			['group-one'],
		);
		expect(result).toEqual({
			workspaceName: 'alpha',
			warnings: ['warning-one'],
			pendingHooks: [{ event: 'workspace.create', repo: 'repo-a', hooks: ['post-checkout'] }],
			hookRuns: [
				{
					event: 'workspace.create',
					repo: 'repo-a',
					id: 'hook-1',
					status: 'ok',
					log_path: '/tmp/hook-1.log',
				},
			],
		});
	});

	it('propagates create mutation failures', async () => {
		const registerRepo = vi.fn(async () => undefined);
		const createWorkspace = vi.fn(async () => {
			throw new Error('create failed');
		});

		await expect(
			runCreateWorkspaceMutation(
				{
					finalName: 'alpha',
					primaryInput: '',
					directRepos: [],
					selectedAliases: new Set(),
					selectedGroups: new Set(),
				},
				{ registerRepo, createWorkspace },
			),
		).rejects.toThrow('create failed');
	});
});

describe('runAddItemsMutation', () => {
	it('runs add mutation and dedupes warnings/pending hooks', async () => {
		const addRepo = vi
			.fn()
			.mockResolvedValueOnce(
				buildRepoAddResponse({
					warnings: ['warning-a'],
					pendingHooks: [
						{ event: 'repo.add', repo: 'repo-a', hooks: ['post-checkout'], reason: 'needs trust' },
					],
					hookRuns: [{ event: 'repo.add', repo: 'repo-a', id: 'hook-1', status: 'ok' }],
				}),
			)
			.mockResolvedValueOnce(
				buildRepoAddResponse({
					warnings: ['warning-a', 'warning-b'],
					pendingHooks: [
						{ event: 'repo.add', repo: 'repo-a', hooks: ['post-checkout'], reason: 'updated' },
						{ event: 'repo.add', repo: 'repo-b', hooks: ['post-merge'] },
					],
					hookRuns: [{ event: 'repo.add', repo: 'repo-b', id: 'hook-2', status: 'skipped' }],
				}),
			);
		const applyGroup = vi.fn(async () => undefined);

		const result = await runAddItemsMutation(
			{
				workspaceId: 'ws-1',
				source: '/tmp/repos/source',
				selectedAliases: new Set(['alias-one']),
				selectedGroups: new Set(['group-one']),
			},
			{ addRepo, applyGroup },
		);

		expect(addRepo).toHaveBeenCalledTimes(2);
		expect(addRepo).toHaveBeenNthCalledWith(1, 'ws-1', '/tmp/repos/source', '', '');
		expect(addRepo).toHaveBeenNthCalledWith(2, 'ws-1', 'alias-one', '', '');
		expect(applyGroup).toHaveBeenCalledTimes(1);
		expect(applyGroup).toHaveBeenCalledWith('ws-1', 'group-one');
		expect(result.itemCount).toBe(3);
		expect(result.warnings).toEqual(['warning-a', 'warning-b']);
		expect(result.pendingHooks).toEqual([
			{ event: 'repo.add', repo: 'repo-a', hooks: ['post-checkout'], reason: 'updated' },
			{ event: 'repo.add', repo: 'repo-b', hooks: ['post-merge'] },
		]);
		expect(result.hookRuns).toEqual([
			{ event: 'repo.add', repo: 'repo-a', id: 'hook-1', status: 'ok' },
			{ event: 'repo.add', repo: 'repo-b', id: 'hook-2', status: 'skipped' },
		]);
	});

	it('propagates add failures and does not apply groups after failure', async () => {
		const addRepo = vi
			.fn()
			.mockResolvedValueOnce(buildRepoAddResponse())
			.mockRejectedValueOnce(new Error('add failed'));
		const applyGroup = vi.fn(async () => undefined);

		await expect(
			runAddItemsMutation(
				{
					workspaceId: 'ws-1',
					source: '/tmp/repos/source',
					selectedAliases: new Set(['alias-one']),
					selectedGroups: new Set(['group-one']),
				},
				{ addRepo, applyGroup },
			),
		).rejects.toThrow('add failed');
		expect(applyGroup).not.toHaveBeenCalled();
	});
});

describe('evaluateHookTransition', () => {
	it('returns auto-close only for clean hook activity', () => {
		const cleanRuns: HookExecution[] = [
			{ event: 'repo.add', repo: 'repo-a', id: 'hook-1', status: 'ok' },
		];
		expect(
			evaluateHookTransition({
				warnings: [],
				pendingHooks: [],
				hookRuns: cleanRuns,
			}),
		).toEqual({ hasHookActivity: true, shouldAutoClose: true });

		expect(
			evaluateHookTransition({
				warnings: ['warning'],
				pendingHooks: [],
				hookRuns: cleanRuns,
			}),
		).toEqual({ hasHookActivity: true, shouldAutoClose: false });

		expect(
			evaluateHookTransition({
				warnings: [],
				pendingHooks: [],
				hookRuns: [{ event: 'repo.add', repo: 'repo-a', id: 'hook-1', status: 'failed' }],
			}),
		).toEqual({ hasHookActivity: true, shouldAutoClose: false });
	});
});

describe('runRenameWorkspaceMutation', () => {
	it('renames workspace and returns mutation context', async () => {
		const renameWorkspace = vi.fn(async () => undefined);

		const result = await runRenameWorkspaceMutation(
			{
				workspaceId: 'ws-1',
				workspaceName: 'renamed',
			},
			{ renameWorkspace },
		);

		expect(renameWorkspace).toHaveBeenCalledTimes(1);
		expect(renameWorkspace).toHaveBeenCalledWith('ws-1', 'renamed');
		expect(result).toEqual({ workspaceId: 'ws-1', workspaceName: 'renamed' });
	});

	it('propagates rename failures', async () => {
		const renameWorkspace = vi.fn(async () => {
			throw new Error('rename failed');
		});

		await expect(
			runRenameWorkspaceMutation(
				{
					workspaceId: 'ws-1',
					workspaceName: 'renamed',
				},
				{ renameWorkspace },
			),
		).rejects.toThrow('rename failed');
	});
});

describe('runArchiveWorkspaceMutation', () => {
	it('archives workspace and returns mutation context', async () => {
		const archiveWorkspace = vi.fn(async () => undefined);

		const result = await runArchiveWorkspaceMutation(
			{
				workspaceId: 'ws-1',
				reason: 'completed',
			},
			{ archiveWorkspace },
		);

		expect(archiveWorkspace).toHaveBeenCalledTimes(1);
		expect(archiveWorkspace).toHaveBeenCalledWith('ws-1', 'completed');
		expect(result).toEqual({ workspaceId: 'ws-1' });
	});

	it('propagates archive failures', async () => {
		const archiveWorkspace = vi.fn(async () => {
			throw new Error('archive failed');
		});

		await expect(
			runArchiveWorkspaceMutation(
				{
					workspaceId: 'ws-1',
					reason: '',
				},
				{ archiveWorkspace },
			),
		).rejects.toThrow('archive failed');
	});
});

describe('runRemoveWorkspaceMutation', () => {
	it('removes workspace and returns mutation context', async () => {
		const removeWorkspace = vi.fn(async () => undefined);

		const result = await runRemoveWorkspaceMutation(
			{
				workspaceId: 'ws-1',
				deleteFiles: true,
				force: false,
			},
			{ removeWorkspace },
		);

		expect(removeWorkspace).toHaveBeenCalledTimes(1);
		expect(removeWorkspace).toHaveBeenCalledWith('ws-1', {
			deleteFiles: true,
			force: false,
		});
		expect(result).toEqual({ workspaceId: 'ws-1' });
	});

	it('propagates remove workspace failures', async () => {
		const removeWorkspace = vi.fn(async () => {
			throw new Error('remove workspace failed');
		});

		await expect(
			runRemoveWorkspaceMutation(
				{
					workspaceId: 'ws-1',
					deleteFiles: false,
					force: true,
				},
				{ removeWorkspace },
			),
		).rejects.toThrow('remove workspace failed');
	});
});

describe('runRemoveRepoMutation', () => {
	it('removes repo and returns mutation context', async () => {
		const removeRepo = vi.fn(async () => undefined);

		const result = await runRemoveRepoMutation(
			{
				workspaceId: 'ws-1',
				repoName: 'repo-a',
				deleteWorktree: true,
			},
			{ removeRepo },
		);

		expect(removeRepo).toHaveBeenCalledTimes(1);
		expect(removeRepo).toHaveBeenCalledWith('ws-1', 'repo-a', true, false);
		expect(result).toEqual({ workspaceId: 'ws-1', repoName: 'repo-a' });
	});

	it('propagates remove repo failures', async () => {
		const removeRepo = vi.fn(async () => {
			throw new Error('remove repo failed');
		});

		await expect(
			runRemoveRepoMutation(
				{
					workspaceId: 'ws-1',
					repoName: 'repo-a',
					deleteWorktree: false,
				},
				{ removeRepo },
			),
		).rejects.toThrow('remove repo failed');
	});
});

describe('createWorkspaceActionMutationService', () => {
	it('routes mutation calls through the dedicated gateway', async () => {
		const registerRepo = vi.fn(async () => undefined);
		const createWorkspace = vi.fn(
			async (): Promise<WorkspaceCreateResponse> => ({
				workspace: {
					name: 'alpha',
					path: '/tmp/alpha',
					workset: '/tmp/alpha/workset.yaml',
					branch: 'main',
					next: 'next',
				},
				warnings: [],
				pendingHooks: [],
				hookRuns: [],
			}),
		);
		const addRepo = vi.fn(async () => buildRepoAddResponse());
		const applyGroup = vi.fn(async () => undefined);
		const renameWorkspace = vi.fn(async () => undefined);
		const archiveWorkspace = vi.fn(async () => undefined);
		const removeWorkspace = vi.fn(async () => undefined);
		const removeRepo = vi.fn(async () => undefined);

		const service = createWorkspaceActionMutationService({
			registerRepo,
			createWorkspace,
			addRepo,
			applyGroup,
			renameWorkspace,
			archiveWorkspace,
			removeWorkspace,
			removeRepo,
		});

		await service.createWorkspace({
			finalName: 'alpha',
			primaryInput: '/tmp/repos/alpha',
			directRepos: [],
			selectedAliases: new Set(),
			selectedGroups: new Set(),
		});
		await service.addItems({
			workspaceId: 'ws-1',
			source: '/tmp/repos/beta',
			selectedAliases: new Set(['alias-one']),
			selectedGroups: new Set(['group-one']),
		});
		await service.renameWorkspace({
			workspaceId: 'ws-1',
			workspaceName: 'renamed',
		});
		await service.archiveWorkspace({
			workspaceId: 'ws-1',
			reason: 'done',
		});
		await service.removeWorkspace({
			workspaceId: 'ws-1',
			deleteFiles: false,
			force: false,
		});
		await service.removeRepo({
			workspaceId: 'ws-1',
			repoName: 'repo-a',
			deleteWorktree: false,
		});

		expect(registerRepo).toHaveBeenCalledTimes(1);
		expect(createWorkspace).toHaveBeenCalledTimes(1);
		expect(addRepo).toHaveBeenCalledTimes(2);
		expect(applyGroup).toHaveBeenCalledTimes(1);
		expect(renameWorkspace).toHaveBeenCalledTimes(1);
		expect(archiveWorkspace).toHaveBeenCalledTimes(1);
		expect(removeWorkspace).toHaveBeenCalledTimes(1);
		expect(removeRepo).toHaveBeenCalledTimes(1);
	});
});
