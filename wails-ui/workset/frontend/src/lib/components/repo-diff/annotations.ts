import type { PullRequestReviewComment } from '../../types';

export type AnnotationSide = 'deletions' | 'additions';

export type DiffLineAnnotation<T = undefined> = {
	side: AnnotationSide;
	lineNumber: number;
} & (T extends undefined ? { metadata?: undefined } : { metadata: T });

export type ReviewCommentItem = {
	id: number;
	nodeId?: string;
	threadId?: string;
	author: string;
	authorId?: number;
	body: string;
	url?: string;
	isReply: boolean;
	resolved?: boolean;
};

export type ReviewAnnotation = {
	thread: ReviewCommentItem[];
	resolved: boolean;
};

// Group comments into threads and convert to diff annotations
// AnnotationSide in @pierre/diffs is 'deletions' | 'additions'
export const buildLineAnnotations = (
	reviews: PullRequestReviewComment[],
): DiffLineAnnotation<ReviewAnnotation>[] => {
	const withLine = reviews.filter((r) => r.line != null);
	if (withLine.length === 0) return [];

	// Build map of comment ID to comment for quick lookup
	const byId = new Map(withLine.map((r) => [r.id, r]));

	// Find root comments (not replies, or reply target not in our filtered set)
	const roots = withLine.filter((r) => !r.inReplyTo || !byId.has(r.inReplyTo));

	// Build threads: for each root, gather all replies
	const threads: Array<{
		root: PullRequestReviewComment;
		replies: PullRequestReviewComment[];
	}> = [];

	const usedIds = new Set<number>();

	for (const root of roots) {
		if (usedIds.has(root.id)) continue;
		usedIds.add(root.id);

		// Find all comments that reply to this root (direct or chained)
		const replies: PullRequestReviewComment[] = [];
		const findReplies = (parentId: number) => {
			for (const r of withLine) {
				if (r.inReplyTo === parentId && !usedIds.has(r.id)) {
					usedIds.add(r.id);
					replies.push(r);
					findReplies(r.id); // Find nested replies
				}
			}
		};
		findReplies(root.id);

		threads.push({ root, replies });
	}

	// Convert threads to annotations
	return threads.map(({ root, replies }) => ({
		side: (root.side?.toLowerCase() === 'left' ? 'deletions' : 'additions') as
			| 'deletions'
			| 'additions',
		lineNumber: root.line!,
		metadata: {
			resolved: root.resolved ?? false,
			thread: [
				{
					id: root.id,
					nodeId: root.nodeId,
					threadId: root.threadId,
					author: root.author ?? 'Reviewer',
					authorId: root.authorId,
					body: root.body,
					url: root.url,
					isReply: false,
					resolved: root.resolved,
				},
				...replies.map((r) => ({
					id: r.id,
					nodeId: r.nodeId,
					threadId: r.threadId ?? root.threadId,
					author: r.author ?? 'Reviewer',
					authorId: r.authorId,
					body: r.body,
					url: r.url,
					isReply: true,
					resolved: root.resolved,
				})),
			],
		},
	}));
};
