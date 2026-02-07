import type { PullRequestReviewComment } from '../../types';

export function filterReviewsBySelectedPath(
	reviews: PullRequestReviewComment[],
	selectedPath: string | undefined,
): PullRequestReviewComment[] {
	return reviews.filter((comment) => (selectedPath ? comment.path === selectedPath : true));
}

export function buildReviewCountsByPath(reviews: PullRequestReviewComment[]): Map<string, number> {
	const counts = new Map<string, number>();
	for (const comment of reviews) {
		counts.set(comment.path, (counts.get(comment.path) ?? 0) + 1);
	}
	return counts;
}

export function getReviewCountForFile(
	reviewCountsByPath: Map<string, number>,
	path: string,
): number {
	return reviewCountsByPath.get(path) ?? 0;
}
