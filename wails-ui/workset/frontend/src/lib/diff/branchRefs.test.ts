import { describe, expect, it } from 'vitest';
import type { RemoteInfo } from '../types';
import { resolveBranchRefs, resolveRemoteRef } from './branchRefs';

const remotes: RemoteInfo[] = [
	{ name: 'origin', owner: 'octo', repo: 'repo' },
	{ name: 'upstream', owner: 'base', repo: 'repo' },
];

describe('resolveRemoteRef', () => {
	it('resolves a remote ref when owner/repo matches', () => {
		expect(resolveRemoteRef(remotes, 'octo/repo', 'feature')).toBe('origin/feature');
	});

	it('returns null when repo format is invalid', () => {
		expect(resolveRemoteRef(remotes, 'octo', 'feature')).toBeNull();
	});

	it('returns null when no remote matches', () => {
		expect(resolveRemoteRef(remotes, 'missing/repo', 'feature')).toBeNull();
	});
});

describe('resolveBranchRefs', () => {
	it('returns null when branches are missing', () => {
		expect(resolveBranchRefs(remotes, { baseRepo: 'octo/repo' })).toBeNull();
		expect(resolveBranchRefs(remotes, { headRepo: 'octo/repo' })).toBeNull();
	});

	it('resolves base/head refs via remotes when available', () => {
		const result = resolveBranchRefs(remotes, {
			baseRepo: 'base/repo',
			baseBranch: 'main',
			headRepo: 'octo/repo',
			headBranch: 'feature',
		});
		expect(result).toEqual({ base: 'upstream/main', head: 'origin/feature' });
	});

	it('falls back to branch names when remotes are missing or invalid', () => {
		const result = resolveBranchRefs([], {
			baseRepo: 'not/a/format',
			baseBranch: 'main',
			headRepo: 'missing/repo',
			headBranch: 'feature',
		});
		expect(result).toEqual({ base: 'main', head: 'feature' });
	});
});
