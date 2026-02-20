import { describe, expect, it } from 'vitest';
import type { PullRequestReviewComment } from '../../types';
import { buildLineAnnotations } from './annotations';

describe('buildLineAnnotations', () => {
	it('groups replies under the root comment', () => {
		const comments: PullRequestReviewComment[] = [
			{
				id: 1,
				body: 'Root',
				path: 'file.ts',
				line: 10,
				side: 'RIGHT',
				outdated: false,
			},
			{
				id: 2,
				body: 'Reply',
				path: 'file.ts',
				line: 10,
				side: 'RIGHT',
				inReplyTo: 1,
				outdated: false,
			},
		];

		const annotations = buildLineAnnotations(comments);
		expect(annotations).toHaveLength(1);
		expect(annotations[0].lineNumber).toBe(10);
		expect(annotations[0].metadata?.thread).toHaveLength(2);
		expect(annotations[0].metadata?.thread[0].isReply).toBe(false);
		expect(annotations[0].metadata?.thread[1].isReply).toBe(true);
	});

	it('anchors outdated threads using originalLine when line is missing', () => {
		const comments: PullRequestReviewComment[] = [
			{
				id: 10,
				threadId: 'THREAD_1',
				body: 'Outdated root',
				path: 'file.ts',
				originalLine: 58,
				side: 'RIGHT',
				outdated: true,
			},
			{
				id: 11,
				threadId: 'THREAD_1',
				body: 'Reply without line',
				path: 'file.ts',
				inReplyTo: 10,
				outdated: true,
			},
		];

		const annotations = buildLineAnnotations(comments);
		expect(annotations).toHaveLength(1);
		expect(annotations[0].lineNumber).toBe(58);
		expect(annotations[0].metadata?.thread).toHaveLength(2);
		expect(annotations[0].metadata?.thread[0].isReply).toBe(false);
		expect(annotations[0].metadata?.thread[1].isReply).toBe(true);
	});

	it('uses threadId grouping when reply metadata is missing', () => {
		const comments: PullRequestReviewComment[] = [
			{
				id: 20,
				threadId: 'THREAD_2',
				body: 'Root',
				path: 'file.ts',
				line: 12,
				side: 'RIGHT',
				outdated: false,
			},
			{
				id: 21,
				threadId: 'THREAD_2',
				body: 'Reply without inReplyTo',
				path: 'file.ts',
				line: 12,
				side: 'RIGHT',
				outdated: false,
			},
		];

		const annotations = buildLineAnnotations(comments);
		expect(annotations).toHaveLength(1);
		expect(annotations[0].metadata?.thread).toHaveLength(2);
	});
});
