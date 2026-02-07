import type { PullRequestReviewComment } from '../../types';
import {
	DeleteReviewComment,
	EditReviewComment,
	ReplyToReviewComment,
	ResolveReviewThread,
} from '../../../../wailsjs/go/main/App';
import { mapCommentResponse } from './mappers';
import type { PullRequestReviewCommentResponse } from './types';

export async function replyToReviewComment(
	workspaceId: string,
	repoId: string,
	commentId: number,
	body: string,
	number?: number,
	branch?: string,
): Promise<PullRequestReviewComment> {
	const result = (await ReplyToReviewComment({
		workspaceId,
		repoId,
		commentId,
		body,
		number: number ?? 0,
		branch: branch ?? '',
	})) as PullRequestReviewCommentResponse;

	return mapCommentResponse(result);
}

export async function editReviewComment(
	workspaceId: string,
	repoId: string,
	commentId: number,
	body: string,
): Promise<PullRequestReviewComment> {
	const result = (await EditReviewComment({
		workspaceId,
		repoId,
		commentId,
		body,
	})) as PullRequestReviewCommentResponse;

	return mapCommentResponse(result);
}

export async function deleteReviewComment(
	workspaceId: string,
	repoId: string,
	commentId: number,
): Promise<void> {
	await DeleteReviewComment({
		workspaceId,
		repoId,
		commentId,
	});
}

export async function resolveReviewThread(
	workspaceId: string,
	repoId: string,
	threadId: string,
	resolve: boolean,
): Promise<boolean> {
	return (await ResolveReviewThread({
		workspaceId,
		repoId,
		threadId,
		resolve,
	})) as boolean;
}
