import { describe, expect, it, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import type { Workspace } from './types';
import { applyRepoDiffSummary, applyRepoLocalStatus, workspaces } from './state';

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
});
