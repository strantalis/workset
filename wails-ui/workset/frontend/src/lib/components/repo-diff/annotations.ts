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
	if (reviews.length === 0) return [];

	const byID = new Map(reviews.map((comment) => [comment.id, comment]));
	const resolveThreadKey = (comment: PullRequestReviewComment): string => {
		if (comment.threadId) return comment.threadId;
		let cursor = comment;
		const visited = new Set<number>();
		while (cursor.inReplyTo && !visited.has(cursor.id)) {
			visited.add(cursor.id);
			const parent = byID.get(cursor.inReplyTo);
			if (!parent) break;
			if (parent.threadId) return parent.threadId;
			cursor = parent;
		}
		return `single-${cursor.id}`;
	};

	const threadMap = new Map<string, PullRequestReviewComment[]>();
	for (const comment of reviews) {
		const key = resolveThreadKey(comment);
		const comments = threadMap.get(key) ?? [];
		comments.push(comment);
		threadMap.set(key, comments);
	}

	const annotations: DiffLineAnnotation<ReviewAnnotation>[] = [];
	for (const thread of threadMap.values()) {
		const comments = [...thread].sort((a, b) => {
			const byTime = (a.createdAt ?? '').localeCompare(b.createdAt ?? '');
			return byTime !== 0 ? byTime : a.id - b.id;
		});

		const commentIds = new Set(comments.map((comment) => comment.id));
		const root =
			comments.find((comment) => !comment.inReplyTo || !commentIds.has(comment.inReplyTo)) ??
			comments[0];
		const anchor =
			comments.find((comment) => comment.line != null || comment.originalLine != null) ?? root;
		const lineNumber = anchor.line ?? anchor.originalLine;
		if (lineNumber == null || lineNumber <= 0) continue;

		annotations.push({
			side: (anchor.side?.toLowerCase() === 'left' ? 'deletions' : 'additions') as
				| 'deletions'
				| 'additions',
			lineNumber,
			metadata: {
				resolved: root.resolved ?? false,
				thread: comments.map((comment) => ({
					id: comment.id,
					nodeId: comment.nodeId,
					threadId: comment.threadId ?? root.threadId,
					author: comment.author ?? 'Reviewer',
					authorId: comment.authorId,
					body: comment.body,
					url: comment.url,
					isReply: comment.id !== root.id,
					resolved: root.resolved,
				})),
			},
		});
	}

	return annotations;
};
