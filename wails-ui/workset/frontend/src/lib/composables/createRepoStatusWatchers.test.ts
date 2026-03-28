import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Workspace } from '../types';

const repoDiffApiMocks = vi.hoisted(() => ({
	startRepoDiffWatch: vi.fn(async () => true),
	startRepoStatusWatch: vi.fn(async () => true),
	stopRepoDiffWatch: vi.fn(async () => true),
	stopRepoStatusWatch: vi.fn(async () => true),
	updateRepoDiffWatch: vi.fn(async () => true),
}));

vi.mock('../api/repo-diff', () => repoDiffApiMocks);

import { createRepoStatusWatchers } from './createRepoStatusWatchers';

const buildWorkspace = (
	trackedPullRequest?: Workspace['repos'][number]['trackedPullRequest'],
): Workspace => ({
	id: 'ws-1',
	name: 'Workset One',
	path: '/tmp/ws-1',
	archived: false,
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: '2026-03-21T00:00:00Z',
	repos: [
		{
			id: 'ws-1::repo-1',
			name: 'repo-1',
			path: '/tmp/ws-1/repo-1',
			dirty: false,
			missing: false,
			statusKnown: true,
			ahead: 0,
			behind: 0,
			trackedPullRequest,
			diff: { added: 0, removed: 0 },
			files: [],
		},
	],
});

describe('createRepoStatusWatchers', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('starts local watches for repos without tracked pull requests', () => {
		const manager = createRepoStatusWatchers();

		manager.sync([buildWorkspace()]);

		expect(repoDiffApiMocks.startRepoStatusWatch).toHaveBeenCalledWith('ws-1', 'ws-1::repo-1');
		expect(repoDiffApiMocks.startRepoDiffWatch).not.toHaveBeenCalled();
	});

	it('starts full watches for repos with open tracked pull requests', () => {
		const manager = createRepoStatusWatchers();

		manager.sync([
			buildWorkspace({
				repo: 'octo/repo-1',
				number: 42,
				url: 'https://github.com/octo/repo-1/pull/42',
				title: 'Warm PR cache',
				state: 'open',
				draft: false,
				merged: false,
				baseRepo: 'octo/repo-1',
				baseBranch: 'main',
				headRepo: 'octo/repo-1',
				headBranch: 'feature/pr-cache',
			}),
		]);

		expect(repoDiffApiMocks.startRepoDiffWatch).toHaveBeenCalledWith(
			'ws-1',
			'ws-1::repo-1',
			42,
			'feature/pr-cache',
		);
		expect(repoDiffApiMocks.startRepoStatusWatch).not.toHaveBeenCalled();
	});

	it('skips duplicate sync work when the watch scope is unchanged', () => {
		const manager = createRepoStatusWatchers();
		const workspace = buildWorkspace();

		manager.sync([workspace]);
		manager.sync([workspace]);

		expect(repoDiffApiMocks.startRepoStatusWatch).toHaveBeenCalledTimes(1);
		expect(repoDiffApiMocks.stopRepoStatusWatch).not.toHaveBeenCalled();
		expect(repoDiffApiMocks.startRepoDiffWatch).not.toHaveBeenCalled();
		expect(repoDiffApiMocks.updateRepoDiffWatch).not.toHaveBeenCalled();
	});

	it('upgrades local watches to full PR watches and keeps PR refs updated', async () => {
		const manager = createRepoStatusWatchers();

		manager.sync([buildWorkspace()]);
		manager.sync([
			buildWorkspace({
				repo: 'octo/repo-1',
				number: 42,
				url: 'https://github.com/octo/repo-1/pull/42',
				title: 'Warm PR cache',
				state: 'open',
				draft: false,
				merged: false,
				baseRepo: 'octo/repo-1',
				baseBranch: 'main',
				headRepo: 'octo/repo-1',
				headBranch: 'feature/pr-cache',
			}),
		]);

		await Promise.resolve();
		await Promise.resolve();

		expect(repoDiffApiMocks.stopRepoStatusWatch).toHaveBeenCalledWith('ws-1', 'ws-1::repo-1');
		expect(repoDiffApiMocks.startRepoDiffWatch).toHaveBeenCalledWith(
			'ws-1',
			'ws-1::repo-1',
			42,
			'feature/pr-cache',
		);

		manager.sync([
			buildWorkspace({
				repo: 'octo/repo-1',
				number: 43,
				url: 'https://github.com/octo/repo-1/pull/43',
				title: 'Warm PR cache v2',
				state: 'open',
				draft: false,
				merged: false,
				baseRepo: 'octo/repo-1',
				baseBranch: 'main',
				headRepo: 'octo/repo-1',
				headBranch: 'feature/pr-cache-v2',
			}),
		]);

		expect(repoDiffApiMocks.updateRepoDiffWatch).toHaveBeenCalledWith(
			'ws-1',
			'ws-1::repo-1',
			43,
			'feature/pr-cache-v2',
		);
	});
});
