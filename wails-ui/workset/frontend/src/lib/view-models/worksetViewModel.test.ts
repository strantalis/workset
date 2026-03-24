import { describe, expect, it } from 'vitest';
import type { Workspace } from '../types';
import {
	deriveWorksetIdentity,
	mapWorkspaceToSummary,
	mapWorkspaceToThreadShellSummary,
	mapWorkspacesToExplorerWorksets,
	mapWorkspacesToThreadGroups,
} from './worksetViewModel';

const baseWorkspace = (repos: Workspace['repos'], workset = 'Core Platform'): Workspace => ({
	id: 'ws-1',
	name: 'Workspace 1',
	path: '/tmp/ws-1',
	...(workset ? { workset } : {}),
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
		expect(summary.workset).toBe('Core Platform');
	});

	describe('deriveWorksetIdentity', () => {
		it('uses worksetKey when present', () => {
			const ws = baseWorkspace([], 'Core Platform');
			(ws as Record<string, unknown>).worksetKey = 'my-key';
			expect(deriveWorksetIdentity(ws).id).toBe('my-key');
		});

		it('uses worksetLabel when present', () => {
			const ws = baseWorkspace([], 'Core Platform');
			(ws as Record<string, unknown>).worksetLabel = 'My Label';
			expect(deriveWorksetIdentity(ws).label).toBe('My Label');
		});

		it('falls back to normalized workset for id', () => {
			const ws = baseWorkspace([], 'Core Platform');
			expect(deriveWorksetIdentity(ws).id).toBe('workset:core-platform');
		});

		it('falls back to workset for label', () => {
			const ws = baseWorkspace([], 'Core Platform');
			expect(deriveWorksetIdentity(ws).label).toBe('Core Platform');
		});

		it('falls back to workspace id and name when no workset', () => {
			const ws = baseWorkspace([], '');
			const identity = deriveWorksetIdentity(ws);
			expect(identity.id).toBe('workspace:ws-1');
			expect(identity.label).toBe('Workspace 1');
		});
	});

	it('normalizes missing workset metadata to Unassigned', () => {
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

		expect(summary.workset).toBe('Unassigned');
	});

	it('maps thread shell summaries with review feedback and status', () => {
		const summary = mapWorkspaceToThreadShellSummary(
			baseWorkspace([
				{
					id: 'r1',
					name: 'repo-1',
					path: '/tmp/ws-1/repo-1',
					dirty: true,
					missing: false,
					currentBranch: 'feature/shell-summary',
					diff: { added: 4, removed: 1 },
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
						reviewCommentsCount: 2,
					},
				},
			]),
		);

		expect(summary.branch).toBe('feature/shell-summary');
		expect(summary.dirtyRepos).toBe(1);
		expect(summary.openPrs).toBe(1);
		expect(summary.reviewCommentsCount).toBe(2);
		expect(summary.status).toBe('in-review');
	});

	it('groups explorer worksets from thread summaries', () => {
		const alpha = baseWorkspace(
			[
				{
					id: 'r1',
					name: 'repo-1',
					path: '/tmp/ws-1/repo-1',
					dirty: false,
					missing: false,
					diff: { added: 2, removed: 1 },
					files: [],
				},
			],
			'Core Platform',
		);
		const beta: Workspace = {
			...baseWorkspace(
				[
					{
						id: 'r2',
						name: 'repo-2',
						path: '/tmp/ws-2/repo-2',
						dirty: true,
						missing: false,
						diff: { added: 5, removed: 0 },
						files: [],
					},
				],
				'Core Platform',
			),
			id: 'ws-2',
			name: 'Workspace 2',
			path: '/tmp/ws-2',
			lastUsed: '2026-03-22T00:00:00.000Z',
		};

		const grouped = mapWorkspacesToExplorerWorksets(
			[alpha, beta],
			new Map([
				['ws-1', 2],
				['ws-2', 1],
			]),
		);

		expect(grouped).toHaveLength(1);
		expect(grouped[0].threads.map((thread) => thread.id)).toEqual(['ws-1', 'ws-2']);
		expect(grouped[0].dirtyRepos).toBe(1);
		expect(grouped[0].linesAdded).toBe(7);
		expect(grouped[0].shortcutNumber).toBe(1);
	});

	it('retains placeholder-only worksets in explorer grouping', () => {
		const alpha = {
			...baseWorkspace([], 'Alpha'),
			id: 'thread-alpha',
			name: 'Alpha Thread',
			worksetKey: 'workset:alpha',
			worksetLabel: 'Alpha',
		} as Workspace;
		const betaPlaceholder = {
			...baseWorkspace(
				[
					{
						id: 'r-beta',
						name: 'repo-beta',
						path: '',
						dirty: false,
						missing: false,
						diff: { added: 0, removed: 0 },
						files: [],
					},
				],
				'Beta',
			),
			id: 'placeholder-beta',
			name: 'Beta Placeholder',
			worksetKey: 'workset:beta',
			worksetLabel: 'Beta',
			placeholder: true,
		} as Workspace;

		const grouped = mapWorkspacesToExplorerWorksets(
			[alpha, betaPlaceholder],
			new Map([['thread-alpha', 1]]),
		);

		expect(grouped.map((group) => group.id)).toEqual(['workset:alpha', 'workset:beta']);
		expect(grouped[1].threads).toEqual([]);
		expect(grouped[1].repos).toEqual(['repo-beta']);
	});

	it('groups threads by workset and retains placeholder-only worksets', () => {
		const alpha = {
			...baseWorkspace([], 'Alpha'),
			id: 'thread-alpha',
			name: 'Alpha Thread',
			worksetKey: 'workset:alpha',
			worksetLabel: 'Alpha',
		};
		const betaPlaceholder = {
			...baseWorkspace([], 'Beta'),
			id: 'placeholder-beta',
			name: 'Beta Placeholder',
			worksetKey: 'workset:beta',
			worksetLabel: 'Beta',
			placeholder: true,
		} as Workspace;

		const groups = mapWorkspacesToThreadGroups([betaPlaceholder, alpha]);

		expect(groups.map((group) => group.id)).toEqual(['workset:alpha', 'workset:beta']);
		expect(groups[0].threads.map((thread) => thread.id)).toEqual(['thread-alpha']);
		expect(groups[1].threads).toEqual([]);
		expect(groups[1].repos).toEqual([]);
	});
});
