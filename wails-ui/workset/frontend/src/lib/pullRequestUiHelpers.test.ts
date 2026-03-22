import { describe, expect, test } from 'vitest';
import { buildReviewThreadCountsByFile } from './pullRequestUiHelpers';

describe('pullRequestUiHelpers', () => {
	test('counts unresolved review threads once per thread for matching files', () => {
		const counts = buildReviewThreadCountsByFile(
			[
				{
					id: 1,
					threadId: 'thread-1',
					body: 'first',
					path: 'README.md',
					outdated: false,
					resolved: false,
				},
				{
					id: 2,
					threadId: 'thread-1',
					body: 'reply',
					path: 'README.md',
					outdated: false,
					resolved: false,
				},
				{
					id: 3,
					threadId: 'thread-2',
					body: 'second thread',
					path: 'README.md',
					outdated: false,
					resolved: false,
				},
			],
			[{ path: 'README.md', added: 2, removed: 0, status: 'modified' }],
		);

		expect(counts.get('README.md')).toBe(2);
	});

	test('excludes resolved threads from file counts', () => {
		const counts = buildReviewThreadCountsByFile(
			[
				{
					id: 1,
					threadId: 'thread-1',
					body: 'resolved',
					path: 'README.md',
					outdated: false,
					resolved: true,
				},
			],
			[{ path: 'README.md', added: 2, removed: 0, status: 'modified' }],
		);

		expect(counts.get('README.md') ?? 0).toBe(0);
	});

	test('maps unresolved threads to renamed files via previous path', () => {
		const counts = buildReviewThreadCountsByFile(
			[
				{
					id: 1,
					threadId: 'thread-1',
					body: 'rename comment',
					path: 'docs/old-guide.md',
					outdated: false,
					resolved: false,
				},
			],
			[
				{
					path: 'docs/new-guide.md',
					prevPath: 'docs/old-guide.md',
					added: 4,
					removed: 1,
					status: 'renamed',
				},
			],
		);

		expect(counts.get('docs/new-guide.md')).toBe(1);
	});
});
