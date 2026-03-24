import { describe, expect, test } from 'vitest';
import {
	buildRepoTreeFromDirectories,
	computeRepoTreeDirectoryCounts,
	createRepoDirEntriesKey,
} from './tree';
import type { RepoDirectoryEntry } from '../../api/repo-files';

describe('repo tree helpers', () => {
	test('preserve repo ids containing double colons when computing counts and building nodes', () => {
		const repoId = 'ws-1::repo-alpha';
		const dirEntries = new Map<string, RepoDirectoryEntry[]>([
			[
				createRepoDirEntriesKey(repoId, ''),
				[
					{
						name: 'src',
						path: 'src',
						isDir: true,
						sizeBytes: 0,
						isMarkdown: false,
						childCount: 1,
					},
				],
			],
			[
				createRepoDirEntriesKey(repoId, 'src'),
				[
					{
						name: 'main.ts',
						path: 'src/main.ts',
						isDir: false,
						sizeBytes: 42,
						isMarkdown: false,
						childCount: 0,
					},
				],
			],
		]);

		const nodes = buildRepoTreeFromDirectories(
			[{ id: repoId, name: 'repo-alpha' }],
			dirEntries,
			new Set([`repo:${repoId}`, `dir:${repoId}:src`]),
		);
		const counts = computeRepoTreeDirectoryCounts(dirEntries);

		expect(nodes).toEqual([
			{ kind: 'repo', key: `repo:${repoId}`, label: 'repo-alpha', repoId, depth: 0 },
			{
				kind: 'dir',
				key: `dir:${repoId}:src`,
				label: 'src',
				depth: 1,
				repoId,
				path: 'src',
			},
			{
				kind: 'file',
				key: `file:${repoId}:src/main.ts`,
				label: 'main.ts',
				depth: 2,
				path: 'src/main.ts',
				repoId,
				isMarkdown: false,
			},
		]);
		expect(counts.get(`repo:${repoId}`)).toBe(1);
		expect(counts.get(`dir:${repoId}:src`)).toBe(1);
	});
});
