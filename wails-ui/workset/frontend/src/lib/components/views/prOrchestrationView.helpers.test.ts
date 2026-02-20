import { describe, expect, test } from 'vitest';
import type { PullRequestCreated, Workspace } from '../../types';
import { mergeTrackedPrMap, trackedPrMapsEqual } from './prOrchestrationView.helpers';

const openPr = (number: number, title = `PR ${number}`): PullRequestCreated => ({
	repo: 'octo/repo',
	number,
	url: `https://github.com/octo/repo/pull/${number}`,
	title,
	state: 'open',
	draft: false,
	baseRepo: 'octo/repo',
	baseBranch: 'main',
	headRepo: 'octo/repo',
	headBranch: `feature/${number}`,
});

const workspaceWithRepos = (
	repos: Array<{ id: string; trackedPullRequest?: PullRequestCreated }>,
): Workspace => ({
	id: 'ws-1',
	name: 'Workset One',
	path: '/tmp/ws-1',
	archived: false,
	repos: repos.map((repo) => ({
		id: repo.id,
		name: repo.id,
		path: `/tmp/ws-1/${repo.id}`,
		defaultBranch: 'main',
		currentBranch: 'feature',
		ahead: 0,
		behind: 0,
		dirty: false,
		missing: false,
		trackedPullRequest: repo.trackedPullRequest,
		diff: { added: 0, removed: 0 },
		files: [],
	})),
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: '2026-02-20T00:00:00Z',
});

describe('prOrchestrationView tracked PR map helpers', () => {
	test('keeps cached open PRs when workspace snapshot omits tracked data', () => {
		const currentMap = new Map<string, PullRequestCreated>([['repo-1', openPr(10)]]);
		const workspace = workspaceWithRepos([
			{ id: 'repo-1' },
			{ id: 'repo-2', trackedPullRequest: openPr(22) },
		]);

		const merged = mergeTrackedPrMap(workspace, currentMap);

		expect(Array.from(merged.keys())).toEqual(['repo-1', 'repo-2']);
		expect(merged.get('repo-1')?.number).toBe(10);
		expect(merged.get('repo-2')?.number).toBe(22);
	});

	test('drops cached PR when workspace explicitly reports non-open tracked state', () => {
		const currentMap = new Map<string, PullRequestCreated>([['repo-1', openPr(10)]]);
		const closedPr = { ...openPr(10), state: 'closed' };
		const workspace = workspaceWithRepos([{ id: 'repo-1', trackedPullRequest: closedPr }]);

		const merged = mergeTrackedPrMap(workspace, currentMap);

		expect(merged.size).toBe(0);
	});

	test('drops entries for repos no longer present in workspace', () => {
		const currentMap = new Map<string, PullRequestCreated>([
			['repo-1', openPr(10)],
			['repo-2', openPr(22)],
		]);
		const workspace = workspaceWithRepos([{ id: 'repo-2' }]);

		const merged = mergeTrackedPrMap(workspace, currentMap);

		expect(Array.from(merged.keys())).toEqual(['repo-2']);
		expect(merged.get('repo-2')?.number).toBe(22);
	});

	test('does not rehydrate repos that are temporarily suppressed after close/null confirmation', () => {
		const currentMap = new Map<string, PullRequestCreated>([['repo-1', openPr(10)]]);
		const workspace = workspaceWithRepos([{ id: 'repo-1', trackedPullRequest: openPr(10) }]);

		const merged = mergeTrackedPrMap(workspace, currentMap, new Set(['repo-1']));

		expect(merged.size).toBe(0);
	});

	test('compares tracked map payloads by repo key and PR identity fields', () => {
		const left = new Map<string, PullRequestCreated>([['repo-1', openPr(10)]]);
		const right = new Map<string, PullRequestCreated>([['repo-1', openPr(10)]]);
		const different = new Map<string, PullRequestCreated>([
			['repo-1', { ...openPr(10), headBranch: 'feature/other' }],
		]);

		expect(trackedPrMapsEqual(left, right)).toBe(true);
		expect(trackedPrMapsEqual(left, different)).toBe(false);
	});
});
