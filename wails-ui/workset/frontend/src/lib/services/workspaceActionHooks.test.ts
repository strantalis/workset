import { describe, expect, it } from 'vitest';
import type { HookExecution, HookProgressEvent } from '../types';
import {
	applyHookProgress,
	applyRepoProgressFromHookEvent,
	finalizeRepoProgress,
	initRepoProgress,
} from './workspaceActionHooks';

const event = (overrides: Partial<HookProgressEvent> = {}): HookProgressEvent => ({
	repo: 'workset',
	event: 'worktree.created',
	hookId: 'bootstrap-frontend-deps',
	phase: 'started',
	...overrides,
});

describe('initRepoProgress', () => {
	it('marks every repo as preparing', () => {
		const progress = initRepoProgress(['workset', 'ghostty-web']);
		expect(progress).toEqual({ workset: 'preparing', 'ghostty-web': 'preparing' });
	});

	it('trims whitespace and drops empty entries', () => {
		const progress = initRepoProgress(['  workset  ', '', '  ']);
		expect(progress).toEqual({ workset: 'preparing' });
	});
});

describe('applyRepoProgressFromHookEvent', () => {
	it('promotes a preparing repo to cloning on clone-started', () => {
		const before = initRepoProgress(['workset', 'ghostty-web']);
		const after = applyRepoProgressFromHookEvent(
			before,
			event({ repo: 'workset', phase: 'clone-started' }),
		);
		expect(after).toEqual({ workset: 'cloning', 'ghostty-web': 'preparing' });
	});

	it('promotes cloning to running-hooks on clone-finished', () => {
		const before = { workset: 'cloning' as const };
		const after = applyRepoProgressFromHookEvent(
			before,
			event({ repo: 'workset', phase: 'clone-finished' }),
		);
		expect(after).toEqual({ workset: 'running-hooks' });
	});

	it('marks the repo failed when clone-finished carries an error', () => {
		const before = { workset: 'cloning' as const };
		const after = applyRepoProgressFromHookEvent(
			before,
			event({ repo: 'workset', phase: 'clone-finished', status: 'failed', error: 'boom' }),
		);
		expect(after).toEqual({ workset: 'failed' });
	});

	it('promotes a preparing repo to running-hooks on hook started', () => {
		const before = initRepoProgress(['workset', 'ghostty-web']);
		const after = applyRepoProgressFromHookEvent(before, event({ repo: 'workset' }));
		expect(after).toEqual({ workset: 'running-hooks', 'ghostty-web': 'preparing' });
		expect(before.workset).toBe('preparing');
	});

	it('is a no-op for hook finished phase', () => {
		const before = initRepoProgress(['workset']);
		const after = applyRepoProgressFromHookEvent(
			before,
			event({ phase: 'finished', status: 'ok' }),
		);
		expect(after).toBe(before);
	});

	it('ignores repos not in the progress map', () => {
		const before = initRepoProgress(['workset']);
		const after = applyRepoProgressFromHookEvent(before, event({ repo: 'other' }));
		expect(after).toBe(before);
	});

	it('does not downgrade a done or failed repo', () => {
		const before: Record<string, 'preparing' | 'cloning' | 'running-hooks' | 'done' | 'failed'> = {
			workset: 'done',
			other: 'failed',
		};
		expect(applyRepoProgressFromHookEvent(before, event({ repo: 'workset' }))).toBe(before);
		expect(applyRepoProgressFromHookEvent(before, event({ repo: 'other' }))).toBe(before);
	});

	it('does not regress running-hooks back to cloning', () => {
		const before = { workset: 'running-hooks' as const };
		expect(
			applyRepoProgressFromHookEvent(before, event({ repo: 'workset', phase: 'clone-started' })),
		).toBe(before);
	});
});

describe('applyHookProgress', () => {
	it('ignores clone-lifecycle events so they do not pollute hookRuns', () => {
		const before: HookExecution[] = [];
		const afterStart = applyHookProgress(
			before,
			event({ phase: 'clone-started', hookId: '', repo: 'workset' }),
		);
		expect(afterStart).toBe(before);
		const afterFinish = applyHookProgress(
			before,
			event({ phase: 'clone-finished', hookId: '', repo: 'workset' }),
		);
		expect(afterFinish).toBe(before);
	});

	it('ignores events without a hookId', () => {
		const before: HookExecution[] = [];
		const after = applyHookProgress(before, event({ hookId: '' }));
		expect(after).toBe(before);
	});

	it('records started hook events as running', () => {
		const before: HookExecution[] = [];
		const after = applyHookProgress(before, event({ phase: 'started' }));
		expect(after).toEqual([
			{
				event: 'worktree.created',
				repo: 'workset',
				id: 'bootstrap-frontend-deps',
				status: 'running',
				log_path: undefined,
			},
		]);
	});

	it('updates a running hook entry to ok on finished', () => {
		const before: HookExecution[] = [
			{
				event: 'worktree.created',
				repo: 'workset',
				id: 'bootstrap-frontend-deps',
				status: 'running',
			},
		];
		const after = applyHookProgress(before, event({ phase: 'finished', status: 'ok' }));
		expect(after[0].status).toBe('ok');
		expect(after).toHaveLength(1);
	});
});

describe('finalizeRepoProgress', () => {
	const runs = (overrides: Partial<HookExecution>[] = []): HookExecution[] =>
		overrides.map((o) => ({
			event: 'worktree.created',
			repo: 'workset',
			id: 'bootstrap-frontend-deps',
			status: 'ok',
			...o,
		}));

	it('promotes in-flight entries to done when no error is reported', () => {
		const before = { workset: 'running-hooks', 'ghostty-web': 'preparing' } as const;
		const after = finalizeRepoProgress(before, runs([{ status: 'ok' }]));
		expect(after).toEqual({ workset: 'done', 'ghostty-web': 'done' });
	});

	it('marks repos with a failed hookRun as failed', () => {
		const before = { workset: 'running-hooks', 'ghostty-web': 'preparing' } as const;
		const after = finalizeRepoProgress(before, runs([{ status: 'failed' }]));
		expect(after).toEqual({ workset: 'failed', 'ghostty-web': 'done' });
	});

	it('marks any non-done repo as failed when an error is passed', () => {
		const before = { workset: 'running-hooks', 'ghostty-web': 'done' } as const;
		const after = finalizeRepoProgress(before, [], { error: new Error('boom') });
		expect(after).toEqual({ workset: 'failed', 'ghostty-web': 'done' });
	});

	it('preserves already-done entries', () => {
		const before = { workset: 'done' } as const;
		const after = finalizeRepoProgress(before, []);
		expect(after).toEqual({ workset: 'done' });
	});
});
