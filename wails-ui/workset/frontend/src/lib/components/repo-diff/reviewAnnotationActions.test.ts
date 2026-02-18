import { afterEach, describe, expect, it, vi } from 'vitest';
import type { PullRequestReviewComment } from '../../types';
import type { DiffLineAnnotation, ReviewAnnotation } from './annotations';
import { createReviewAnnotationActionsController } from './reviewAnnotationActions';

const baseComment = {
	id: 1,
	nodeId: 'PRRC_1',
	threadId: 'THREAD_1',
	author: 'octocat',
	authorId: 7,
	body: 'Original comment',
	path: 'src/file.ts',
	outdated: false,
} satisfies PullRequestReviewComment;

const baseAnnotation: DiffLineAnnotation<ReviewAnnotation> = {
	side: 'additions',
	lineNumber: 12,
	metadata: {
		resolved: false,
		thread: [
			{
				id: 1,
				nodeId: 'PRRC_1',
				threadId: 'THREAD_1',
				author: 'octocat',
				authorId: 7,
				body: 'Original comment',
				isReply: false,
			},
		],
	},
};

const flushMicrotasks = async (): Promise<void> => {
	await Promise.resolve();
	await Promise.resolve();
};

const createSetup = (overrides?: Partial<{ currentUserId: number | null }>) => {
	const state: { currentUserId: number | null; prReviews: PullRequestReviewComment[] } = {
		currentUserId: overrides?.currentUserId ?? 7,
		prReviews: [{ ...baseComment }],
	};

	const replyToReviewComment = vi.fn(async () => ({
		id: 2,
		nodeId: 'PRRC_2',
		threadId: 'THREAD_1',
		author: 'reviewer',
		authorId: 99,
		body: 'Reply posted',
		path: 'src/file.ts',
		outdated: false,
		inReplyTo: 1,
	}));
	const editReviewComment = vi.fn(async () => ({
		id: 1,
		body: 'Edited body',
		path: 'src/file.ts',
		outdated: false,
	}));
	const handleDeleteComment = vi.fn();
	const handleResolveThread = vi.fn();
	const showAlert = vi.fn();

	const controller = createReviewAnnotationActionsController({
		document,
		workspaceId: () => 'ws-1',
		repoId: () => 'repo-1',
		prNumberInput: () => '42',
		prBranchInput: () => 'feature/review-actions',
		parseNumber: (value) => Number.parseInt(value, 10),
		getCurrentUserId: () => state.currentUserId,
		getPrReviews: () => state.prReviews,
		setPrReviews: (value) => {
			state.prReviews = value;
		},
		replyToReviewComment,
		editReviewComment,
		handleDeleteComment,
		handleResolveThread,
		formatError: (error, fallback) => (error instanceof Error ? error.message : fallback),
		showAlert,
	});

	const render = (
		annotation: DiffLineAnnotation<ReviewAnnotation> = baseAnnotation,
	): HTMLElement => {
		const el = controller.renderAnnotation(annotation);
		if (!el) throw new Error('Expected annotation element');
		document.body.appendChild(el);
		return el;
	};

	return {
		state,
		replyToReviewComment,
		editReviewComment,
		handleDeleteComment,
		handleResolveThread,
		showAlert,
		render,
	};
};

describe('reviewAnnotationActions', () => {
	afterEach(() => {
		document.body.innerHTML = '';
		vi.clearAllMocks();
	});

	it('opens and cancels edit form while restoring comment body visibility', () => {
		const setup = createSetup();
		const threadEl = setup.render();

		(threadEl.querySelector('[data-action="edit"]') as HTMLElement).click();

		const bodyEl = threadEl.querySelector('.diff-annotation-body') as HTMLElement;
		expect(bodyEl.style.display).toBe('none');
		expect(threadEl.querySelector('.diff-annotation-inline-form')).toBeInTheDocument();

		(threadEl.querySelector('[data-action="cancel-edit"]') as HTMLElement).click();

		expect(threadEl.querySelector('.diff-annotation-inline-form')).toBeNull();
		expect(bodyEl.style.display).toBe('');
	});

	it('submits reply actions and appends the new review comment', async () => {
		const setup = createSetup();
		const threadEl = setup.render();

		(threadEl.querySelector('[data-action="reply"]') as HTMLElement).click();
		const textarea = threadEl.querySelector('.diff-inline-textarea') as HTMLTextAreaElement;
		textarea.value = '  Looks good to me  ';

		(threadEl.querySelector('[data-action="submit-reply"]') as HTMLElement).click();
		await flushMicrotasks();

		expect(setup.replyToReviewComment).toHaveBeenCalledWith(
			'ws-1',
			'repo-1',
			1,
			'Looks good to me',
			42,
			'feature/review-actions',
		);
		expect(setup.state.prReviews.map((comment) => comment.id)).toEqual([1, 2]);
		expect(threadEl.querySelector('.diff-annotation-inline-form')).toBeNull();
		expect(setup.showAlert).not.toHaveBeenCalled();
	});

	it('submits edit actions and preserves existing thread id when API omits it', async () => {
		const setup = createSetup();
		const threadEl = setup.render();

		(threadEl.querySelector('[data-action="edit"]') as HTMLElement).click();
		const textarea = threadEl.querySelector('.diff-inline-textarea') as HTMLTextAreaElement;
		textarea.value = '  Updated copy  ';

		(threadEl.querySelector('[data-action="submit-edit"]') as HTMLElement).click();
		await flushMicrotasks();

		expect(setup.editReviewComment).toHaveBeenCalledWith('ws-1', 'repo-1', 1, 'Updated copy');
		expect(setup.state.prReviews[0].body).toBe('Edited body');
		expect(setup.state.prReviews[0].threadId).toBe('THREAD_1');
		expect(threadEl.querySelector('.diff-annotation-inline-form')).toBeNull();
	});

	it('dispatches delete and resolve actions to the provided callbacks', () => {
		const setup = createSetup();
		const threadEl = setup.render();

		(threadEl.querySelector('[data-action="delete"]') as HTMLElement).click();
		(threadEl.querySelector('[data-action="resolve"]') as HTMLElement).click();

		expect(setup.handleDeleteComment).toHaveBeenCalledWith(1);
		expect(setup.handleResolveThread).toHaveBeenCalledWith('THREAD_1', true);
	});
});
