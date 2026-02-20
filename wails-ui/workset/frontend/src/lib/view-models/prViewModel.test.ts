import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Workspace } from '../types';
import { mapWorkspaceToPrItems } from './prViewModel';

const buildWorkspace = (): Workspace => ({
	id: 'ws-1',
	name: 'Workspace 1',
	path: '/tmp/ws-1',
	archived: false,
	pinned: false,
	pinOrder: 0,
	expanded: false,
	lastUsed: '2026-02-09T10:00:00.000Z',
	repos: [
		{
			id: 'repo-open',
			name: 'repo-open',
			path: '/tmp/ws-1/repo-open',
			defaultBranch: 'main',
			dirty: false,
			missing: false,
			diff: { added: 0, removed: 0 },
			files: [],
			trackedPullRequest: {
				repo: 'repo-open',
				number: 5,
				url: 'https://github.com/example/repo-open/pull/5',
				title: 'Use API snapshot for PR list',
				state: 'open',
				draft: false,
				baseRepo: 'example/repo-open',
				baseBranch: 'main',
				headRepo: 'example/repo-open',
				headBranch: 'feature/snapshot',
				updatedAt: '2026-02-09T11:59:40.000Z',
			},
		},
		{
			id: 'repo-merged',
			name: 'repo-merged',
			path: '/tmp/ws-1/repo-merged',
			defaultBranch: 'main',
			dirty: false,
			missing: false,
			diff: { added: 0, removed: 0 },
			files: [],
			trackedPullRequest: {
				repo: 'repo-merged',
				number: 6,
				url: 'https://github.com/example/repo-merged/pull/6',
				title: 'Merged PR',
				state: 'closed',
				draft: false,
				merged: true,
				baseRepo: 'example/repo-merged',
				baseBranch: 'main',
				headRepo: 'example/repo-merged',
				headBranch: 'feature/merged',
				updatedAt: '2026-02-09T11:20:00.000Z',
			},
		},
		{
			id: 'repo-running',
			name: 'repo-running',
			path: '/tmp/ws-1/repo-running',
			defaultBranch: 'main',
			dirty: true,
			missing: false,
			diff: { added: 0, removed: 0 },
			files: [{ path: 'README.md', added: 1, removed: 0, hunks: [] }],
		},
		{
			id: 'repo-blocked',
			name: 'repo-blocked',
			path: '/tmp/ws-1/repo-blocked',
			defaultBranch: 'main',
			dirty: false,
			missing: true,
			diff: { added: 0, removed: 0 },
			files: [],
		},
	],
});

describe('prViewModel', () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it('maps tracked PR metadata without synthesized review titles', () => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-02-09T12:00:00.000Z'));

		const items = mapWorkspaceToPrItems(buildWorkspace());
		const open = items.find((item) => item.repoId === 'repo-open');
		const merged = items.find((item) => item.repoId === 'repo-merged');
		const running = items.find((item) => item.repoId === 'repo-running');
		const blocked = items.find((item) => item.repoId === 'repo-blocked');

		expect(open?.title).toBe('Use API snapshot for PR list');
		expect(open?.branch).toBe('feature/snapshot');
		expect(open?.status).toBe('open');
		expect(open?.updatedAtLabel).toBe('just now');
		expect(merged?.status).toBe('merged');

		expect(running?.title).toBe('repo-running');
		expect(running?.status).toBe('running');

		expect(blocked?.status).toBe('blocked');
	});
});
