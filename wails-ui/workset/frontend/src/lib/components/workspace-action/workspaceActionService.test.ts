import { describe, expect, it, vi } from 'vitest';
import type { HookExecution, RepoAddResponse, WorkspaceCreateResponse } from '../../types';
import {
	evaluateHookTransition,
	runAddItemsMutation,
	runCreateWorkspaceMutation,
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
