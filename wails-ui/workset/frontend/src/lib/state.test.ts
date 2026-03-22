import { describe, expect, it, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import type { Workspace } from './types';
import {
	applyRepoDiffSummary,
	applyRepoLocalStatus,
	applyTrackedPullRequest,
	applyTrackedPullRequestReviewComments,
	workspaces,
} from './state';

const baseWorkspaces: Workspace[] = [
	{
		id: 'ws-1',
		name: 'Alpha',
		path: '/tmp/alpha',
		archived: false,
		pinned: false,
		pinOrder: 0,
		expanded: false,
		lastUsed: '2024-01-01T00:00:00Z',
		repos: [
			{
				id: 'repo-1',
				name: 'frontend',
				path: '/tmp/alpha/frontend',
				dirty: false,
				missing: false,
				statusKnown: false,
				ahead: 0,
				behind: 0,
				diff: { added: 0, removed: 0 },
				files: [],
			},
			{
				id: 'repo-2',
				name: 'backend',
				path: '/tmp/alpha/backend',
				dirty: false,
				missing: false,
				statusKnown: true,
				ahead: 1,
				behind: 2,
				diff: { added: 0, removed: 0 },
				files: [],
			},
		],
	},
	{
		id: 'ws-2',
		name: 'Beta',
		path: '/tmp/beta',
		archived: false,
		pinned: false,
		pinOrder: 0,
		expanded: false,
		lastUsed: '2024-01-02T00:00:00Z',
		repos: [
			{
				id: 'repo-3',
				name: 'api',
				path: '/tmp/beta/api',
				dirty: false,
				missing: false,
				statusKnown: true,
				ahead: 0,
				behind: 0,
				diff: { added: 0, removed: 0 },
				files: [],
			},
		],
	},
];

const cloneWorkspaces = (): Workspace[] => JSON.parse(JSON.stringify(baseWorkspaces));

beforeEach(() => {
	workspaces.set(cloneWorkspaces());
});

describe('state repo diff updates', () => {
	it('updates diff totals from summary payloads', () => {
		applyRepoDiffSummary('ws-1', 'repo-1', { files: [], totalAdded: 5, totalRemoved: 2 });

		const repo = get(workspaces)[0].repos[0];
		expect(repo.diff).toEqual({ added: 5, removed: 2 });

		const untouched = get(workspaces)[0].repos[1];
		expect(untouched.diff).toEqual({ added: 0, removed: 0 });
	});

	it('updates dirty status and clears diff when clean', () => {
		applyRepoDiffSummary('ws-1', 'repo-1', { files: [], totalAdded: 4, totalRemoved: 1 });

		applyRepoLocalStatus('ws-1', 'repo-1', {
			hasUncommitted: false,
			ahead: 2,
			behind: 1,
			currentBranch: 'main',
		});

		const repo = get(workspaces)[0].repos[0];
		expect(repo.dirty).toBe(false);
		expect(repo.statusKnown).toBe(true);
		expect(repo.ahead).toBe(2);
		expect(repo.behind).toBe(1);
		expect(repo.diff).toEqual({ added: 0, removed: 0 });
	});

	it('preserves diff totals when dirty', () => {
		applyRepoDiffSummary('ws-1', 'repo-1', { files: [], totalAdded: 3, totalRemoved: 7 });

		applyRepoLocalStatus('ws-1', 'repo-1', {
			hasUncommitted: true,
			ahead: 0,
			behind: 0,
			currentBranch: 'main',
		});

		const repo = get(workspaces)[0].repos[0];
		expect(repo.dirty).toBe(true);
		expect(repo.diff).toEqual({ added: 3, removed: 7 });
	});

	it('updates tracked pull request metadata from live status payloads', () => {
		applyTrackedPullRequest('ws-1', 'repo-1', {
			repo: 'octo/frontend',
			number: 42,
			url: 'https://github.com/octo/frontend/pull/42',
			title: 'Speed up PR cache',
			state: 'open',
			draft: false,
			merged: false,
			baseRepo: 'octo/frontend',
			baseBranch: 'main',
			headRepo: 'octo/frontend',
			headBranch: 'feature/pr-cache',
			commentsCount: 5,
			reviewCommentsCount: 3,
		});

		const repo = get(workspaces)[0].repos[0];
		expect(repo.trackedPullRequest?.number).toBe(42);
		expect(repo.trackedPullRequest?.reviewCommentsCount).toBe(3);
	});

	it('clears closed, unmerged tracked pull requests', () => {
		applyTrackedPullRequest('ws-1', 'repo-1', {
			repo: 'octo/frontend',
			number: 42,
			url: 'https://github.com/octo/frontend/pull/42',
			title: 'Speed up PR cache',
			state: 'open',
			draft: false,
			merged: false,
			baseRepo: 'octo/frontend',
			baseBranch: 'main',
			headRepo: 'octo/frontend',
			headBranch: 'feature/pr-cache',
		});

		applyTrackedPullRequest('ws-1', 'repo-1', {
			repo: 'octo/frontend',
			number: 42,
			url: 'https://github.com/octo/frontend/pull/42',
			title: 'Speed up PR cache',
			state: 'closed',
			draft: false,
			merged: false,
			baseRepo: 'octo/frontend',
			baseBranch: 'main',
			headRepo: 'octo/frontend',
			headBranch: 'feature/pr-cache',
		});

		expect(get(workspaces)[0].repos[0].trackedPullRequest).toBeUndefined();
	});

	it('refreshes tracked review comment counts without a workspace reload', () => {
		applyTrackedPullRequest('ws-1', 'repo-1', {
			repo: 'octo/frontend',
			number: 42,
			url: 'https://github.com/octo/frontend/pull/42',
			title: 'Speed up PR cache',
			state: 'open',
			draft: false,
			merged: false,
			baseRepo: 'octo/frontend',
			baseBranch: 'main',
			headRepo: 'octo/frontend',
			headBranch: 'feature/pr-cache',
			commentsCount: 4,
			reviewCommentsCount: 1,
		});

		applyTrackedPullRequestReviewComments('ws-1', 'repo-1', [
			{ id: 1, body: 'one', path: 'src/main.ts', outdated: false },
			{ id: 2, body: 'two', path: 'src/main.ts', outdated: false },
			{ id: 3, body: 'three', path: 'src/main.ts', outdated: false },
		]);

		const repo = get(workspaces)[0].repos[0];
		expect(repo.trackedPullRequest?.reviewCommentsCount).toBe(3);
		expect(repo.trackedPullRequest?.commentsCount).toBe(6);
	});
});
