import { describe, expect, it } from 'vitest';
import type { Workspace } from '../types';
import { mapWorkspaceToSummary } from './worksetViewModel';

const baseWorkspace = (
	repos: Workspace['repos'],
	template = 'Template From Metadata',
): Workspace => ({
	id: 'ws-1',
	name: 'Workspace 1',
	path: '/tmp/ws-1',
	...(template ? { template } : {}),
	archived: false,
	repos,
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: '2026-02-09T00:00:00.000Z',
});

describe('worksetViewModel', () => {
	it('counts only open tracked pull requests', () => {
		const summary = mapWorkspaceToSummary(
			baseWorkspace([
				{
					id: 'r1',
					name: 'repo-1',
					path: '/tmp/ws-1/repo-1',
					dirty: false,
					missing: false,
					diff: { added: 0, removed: 0 },
					files: [],
					trackedPullRequest: {
						repo: 'repo-1',
						number: 11,
						url: 'https://github.com/example/repo-1/pull/11',
						title: 'Open PR',
						state: 'open',
						draft: false,
						baseRepo: 'example/repo-1',
						baseBranch: 'main',
						headRepo: 'example/repo-1',
						headBranch: 'feature/open',
					},
				},
				{
					id: 'r2',
					name: 'repo-2',
					path: '/tmp/ws-1/repo-2',
					dirty: false,
					missing: false,
					diff: { added: 0, removed: 0 },
					files: [],
					trackedPullRequest: {
						repo: 'repo-2',
						number: 12,
						url: 'https://github.com/example/repo-2/pull/12',
						title: 'Closed PR',
						state: 'closed',
						draft: false,
						merged: true,
						baseRepo: 'example/repo-2',
						baseBranch: 'main',
						headRepo: 'example/repo-2',
						headBranch: 'feature/closed',
					},
				},
			]),
		);

		expect(summary.openPrs).toBe(1);
		expect(summary.mergedPrs).toBe(1);
		expect(summary.template).toBe('Template From Metadata');
	});

	it('normalizes missing template metadata to Unassigned', () => {
		const summary = mapWorkspaceToSummary(
			baseWorkspace(
				[
					{
						id: 'r1',
						name: 'repo-1',
						path: '/tmp/ws-1/repo-1',
						dirty: false,
						missing: false,
						diff: { added: 0, removed: 0 },
						files: [],
					},
				],
				'',
			),
		);

		expect(summary.template).toBe('Unassigned');
	});
});
