import { describe, expect, it, vi } from 'vitest';
import type { HookExecution, HookProgressEvent } from '../../types';
import {
	applyHookProgress,
	appendHookRuns,
	beginHookTracking,
	clearHookTracking,
	handleRunPendingHookCore,
	handleTrustPendingHookCore,
	shouldTrackHookEvent,
	type WorkspaceActionPendingHook,
} from '../../services/workspaceActionHooks';

const pendingHook = (repo: string): WorkspaceActionPendingHook => ({
	event: 'repo.add',
	repo,
	hooks: ['post-checkout'],
});

describe('workspaceActionHooks', () => {
	it('begins and clears hook tracking state', () => {
		expect(beginHookTracking('repo.add', 'alpha')).toEqual({
			activeHookOperation: 'repo.add',
			activeHookWorkspace: 'alpha',
			hookRuns: [],
			pendingHooks: [],
		});
		expect(clearHookTracking()).toEqual({
			activeHookOperation: null,
			activeHookWorkspace: null,
		});
	});

	it('appends and dedupes hook runs by repo/event/id', () => {
		const initial: HookExecution[] = [
			{ event: 'repo.add', repo: 'repo-a', id: 'hook-1', status: 'running' },
		];
		const next = appendHookRuns(initial, [
			{ event: 'repo.add', repo: 'repo-a', id: 'hook-1', status: 'ok' },
			{ event: 'repo.add', repo: 'repo-b', id: 'hook-2', status: 'skipped' },
		]);

		expect(next).toEqual([
			{ event: 'repo.add', repo: 'repo-a', id: 'hook-1', status: 'ok' },
			{ event: 'repo.add', repo: 'repo-b', id: 'hook-2', status: 'skipped' },
		]);
		expect(appendHookRuns(next, undefined)).toBe(next);
	});

	it('tracks hook events only for matching operation/context', () => {
		const payload: HookProgressEvent = {
			operation: 'repo.add',
			workspace: 'alpha',
			repo: 'repo-a',
			event: 'repo.add',
			hookId: 'hook-1',
			phase: 'started',
		};

		expect(
			shouldTrackHookEvent(payload, {
				activeHookOperation: null,
				activeHookWorkspace: 'alpha',
				loading: true,
			}),
		).toBe(false);

		expect(
			shouldTrackHookEvent(payload, {
				activeHookOperation: 'repo.add',
				activeHookWorkspace: 'alpha',
				loading: false,
			}),
		).toBe(false);

		expect(
			shouldTrackHookEvent(payload, {
				activeHookOperation: 'workspace.create',
				activeHookWorkspace: 'alpha',
				loading: true,
			}),
		).toBe(false);

		expect(
			shouldTrackHookEvent(
				{
					...payload,
					workspace: 'beta',
				},
				{
					activeHookOperation: 'repo.add',
					activeHookWorkspace: 'alpha',
					loading: true,
				},
			),
		).toBe(false);

		expect(
			shouldTrackHookEvent(
				{
					...payload,
					workspace: '',
				},
				{
					activeHookOperation: 'repo.add',
					activeHookWorkspace: 'alpha',
					loading: true,
				},
			),
		).toBe(true);
	});

	it('applies hook progress by inserting and updating entries', () => {
		const started: HookProgressEvent = {
			operation: 'repo.add',
			repo: 'repo-a',
			event: 'repo.add',
			hookId: 'hook-1',
			phase: 'started',
			logPath: '/tmp/hook-1.log',
		};
		const withStarted = applyHookProgress([], started);
		expect(withStarted).toEqual([
			{
				repo: 'repo-a',
				event: 'repo.add',
				id: 'hook-1',
				status: 'running',
				log_path: '/tmp/hook-1.log',
			},
		]);

		const finished = applyHookProgress(withStarted, {
			...started,
			phase: 'finished',
			error: 'failed',
		});
		expect(finished).toEqual([
			{
				repo: 'repo-a',
				event: 'repo.add',
				id: 'hook-1',
				status: 'failed',
				log_path: '/tmp/hook-1.log',
			},
		]);
	});

	it('handles run pending hook core with missing workspace reference', async () => {
		const runRepoHooks = vi.fn();
		const initialPending = [pendingHook('repo-a')];

		const state = handleRunPendingHookCore(
			{
				pending: initialPending[0],
				pendingHooks: initialPending,
				hookRuns: [],
				workspaceReferences: [null, '', undefined],
				activeHookOperation: 'repo.add',
			},
			{
				runRepoHooks,
				formatError: (err, fallback) => (err ? String(err) : fallback),
			},
		);

		expect(runRepoHooks).not.toHaveBeenCalled();
		expect(state.pendingHooks).toEqual([
			{
				...pendingHook('repo-a'),
				running: false,
				runError: 'Workspace reference unavailable for hook run.',
			},
		]);
		await expect(state.completion).resolves.toEqual({
			pendingHooks: state.pendingHooks,
			hookRuns: [],
		});
	});

	it('handles run pending hook core success and uses latest state accessors', async () => {
		const runRepoHooks = vi.fn(async () => ({
			event: 'repo.add',
			repo: 'repo-a',
			results: [{ id: 'hook-1', status: 'ok', log_path: '/tmp/hook-1.log' }],
		}));

		const pendingA = pendingHook('repo-a');
		const pendingB = pendingHook('repo-b');
		const pendingC = pendingHook('repo-c');

		let currentPendingHooks: WorkspaceActionPendingHook[] = [pendingA, pendingB];
		let currentHookRuns: HookExecution[] = [
			{ event: 'repo.add', repo: 'repo-z', id: 'hook-z', status: 'running' },
		];

		const state = handleRunPendingHookCore(
			{
				pending: pendingA,
				pendingHooks: currentPendingHooks,
				hookRuns: currentHookRuns,
				workspaceReferences: ['ws-1'],
				activeHookOperation: 'repo.add',
				getPendingHooks: () => currentPendingHooks,
				getHookRuns: () => currentHookRuns,
			},
			{
				runRepoHooks,
				formatError: (err, fallback) => (err ? String(err) : fallback),
			},
		);

		expect(state.pendingHooks).toEqual([
			{ ...pendingHook('repo-a'), running: true, runError: undefined },
			pendingB,
		]);

		currentPendingHooks = [...state.pendingHooks, pendingC];
		currentHookRuns = [...state.hookRuns];

		const completed = await state.completion;

		expect(runRepoHooks).toHaveBeenCalledTimes(1);
		expect(runRepoHooks).toHaveBeenCalledWith('ws-1', 'repo-a', 'repo.add', 'repo.add.ui');
		expect(completed.pendingHooks).toEqual([pendingB, pendingC]);
		expect(completed.hookRuns).toEqual([
			{ event: 'repo.add', repo: 'repo-z', id: 'hook-z', status: 'running' },
			{
				event: 'repo.add',
				repo: 'repo-a',
				id: 'hook-1',
				status: 'ok',
				log_path: '/tmp/hook-1.log',
			},
		]);
	});

	it('handles run pending hook core failure', async () => {
		const runRepoHooks = vi.fn(async () => {
			throw new Error('boom');
		});
		const pendingA = pendingHook('repo-a');
		let currentPendingHooks: WorkspaceActionPendingHook[] = [pendingA];
		let currentHookRuns: HookExecution[] = [
			{ event: 'repo.add', repo: 'repo-z', id: 'hook-z', status: 'running' },
		];

		const state = handleRunPendingHookCore(
			{
				pending: pendingA,
				pendingHooks: currentPendingHooks,
				hookRuns: currentHookRuns,
				workspaceReferences: ['ws-1'],
				activeHookOperation: null,
				getPendingHooks: () => currentPendingHooks,
				getHookRuns: () => currentHookRuns,
			},
			{
				runRepoHooks,
				formatError: (_err, fallback) => `${fallback} (formatted)`,
			},
		);

		currentPendingHooks = state.pendingHooks;
		currentHookRuns = state.hookRuns;

		await expect(state.completion).resolves.toEqual({
			pendingHooks: [
				{
					...pendingHook('repo-a'),
					running: false,
					runError: 'Failed to run hooks for repo-a. (formatted)',
				},
			],
			hookRuns: currentHookRuns,
		});
	});

	it('handles trust pending hook core success and failure', async () => {
		const pendingA = pendingHook('repo-a');
		const pendingB = pendingHook('repo-b');

		let currentPendingHooks: WorkspaceActionPendingHook[] = [pendingA, pendingB];
		const trustRepoHooks = vi.fn(async () => undefined);

		const trustState = handleTrustPendingHookCore(
			{
				pending: pendingA,
				pendingHooks: currentPendingHooks,
				getPendingHooks: () => currentPendingHooks,
			},
			{
				trustRepoHooks,
				formatError: (_err, fallback) => fallback,
			},
		);

		expect(trustState.pendingHooks).toEqual([
			{ ...pendingHook('repo-a'), trusting: true, runError: undefined },
			pendingB,
		]);
		currentPendingHooks = trustState.pendingHooks;
		await expect(trustState.completion).resolves.toEqual([
			{ ...pendingHook('repo-a'), trusting: false, trusted: true },
			pendingB,
		]);

		const failingTrustState = handleTrustPendingHookCore(
			{
				pending: pendingB,
				pendingHooks: [pendingB],
			},
			{
				trustRepoHooks: vi.fn(async () => {
					throw new Error('denied');
				}),
				formatError: (_err, fallback) => `${fallback} (formatted)`,
			},
		);
		await expect(failingTrustState.completion).resolves.toEqual([
			{
				...pendingHook('repo-b'),
				trusting: false,
				runError: 'Failed to trust hooks for repo-b. (formatted)',
			},
		]);
	});
});
