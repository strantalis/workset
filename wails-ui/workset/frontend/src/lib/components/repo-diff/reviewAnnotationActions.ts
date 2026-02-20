import DOMPurify from 'dompurify';
import { marked } from 'marked';
import type { PullRequestReviewComment } from '../../types';
import type { DiffLineAnnotation, ReviewAnnotation } from './annotations';

type ReviewAnnotationActionsControllerOptions = {
	document: Document;
	workspaceId: () => string;
	repoId: () => string;
	prNumberInput: () => string;
	prBranchInput: () => string;
	parseNumber: (value: string) => number | undefined;
	getCurrentUserId: () => number | null;
	getPrReviews: () => PullRequestReviewComment[];
	setPrReviews: (value: PullRequestReviewComment[]) => void;
	replyToReviewComment: (
		workspaceId: string,
		repoId: string,
		commentId: number,
		body: string,
		prNumber?: number,
		prBranch?: string,
	) => Promise<PullRequestReviewComment>;
	editReviewComment: (
		workspaceId: string,
		repoId: string,
		commentId: number,
		body: string,
	) => Promise<PullRequestReviewComment>;
	handleDeleteComment: (commentId: number) => void | Promise<void>;
	handleResolveThread: (threadId: string, resolve: boolean) => void | Promise<void>;
	formatError: (error: unknown, fallback: string) => string;
	showAlert: (message: string) => void;
};

export const createReviewAnnotationActionsController = (
	options: ReviewAnnotationActionsControllerOptions,
) => {
	const escapeHtml = (text: string): string => {
		const div = options.document.createElement('div');
		div.textContent = text;
		return div.innerHTML;
	};

	const renderMarkdown = (text: string): string => {
		try {
			const rendered = marked.parse(text, { async: false, breaks: true }) as string;
			return DOMPurify.sanitize(rendered);
		} catch {
			return escapeHtml(text);
		}
	};

	const removeInlineForm = (threadEl: HTMLElement): void => {
		const existingForm = threadEl.querySelector('.diff-annotation-inline-form');
		if (existingForm) {
			// If editing, restore the hidden body
			const editingId = (existingForm as HTMLElement).dataset.editingId;
			if (editingId) {
				const commentEl = threadEl.querySelector(`[data-comment-id="${editingId}"]`);
				const bodyEl = commentEl?.querySelector('.diff-annotation-body') as HTMLElement;
				if (bodyEl) bodyEl.style.display = '';
			}
			existingForm.remove();
		}
	};

	const injectReplyForm = (threadEl: HTMLElement, commentId: number): void => {
		removeInlineForm(threadEl);

		const formEl = options.document.createElement('div');
		formEl.className = 'diff-annotation-inline-form';
		formEl.innerHTML = `
      <textarea class="diff-inline-textarea" placeholder="Write your reply..." rows="3"></textarea>
      <div class="diff-inline-form-actions">
        <button class="btn-ghost" data-action="cancel-reply" type="button">Cancel</button>
        <button class="btn-primary" data-action="submit-reply" data-comment-id="${commentId}" type="button">Reply</button>
      </div>
    `;
		threadEl.appendChild(formEl);

		const textarea = formEl.querySelector('textarea');
		textarea?.focus();
	};

	const injectEditForm = (threadEl: HTMLElement, comment: PullRequestReviewComment): void => {
		removeInlineForm(threadEl);

		const commentEl = threadEl.querySelector(`[data-comment-id="${comment.id}"]`);
		const bodyEl = commentEl?.querySelector('.diff-annotation-body') as HTMLElement;
		if (bodyEl) bodyEl.style.display = 'none';

		const formEl = options.document.createElement('div');
		formEl.className = 'diff-annotation-inline-form';
		formEl.dataset.editingId = String(comment.id);
		formEl.innerHTML = `
      <textarea class="diff-inline-textarea" rows="3">${escapeHtml(comment.body)}</textarea>
      <div class="diff-inline-form-actions">
        <button class="btn-ghost" data-action="cancel-edit" type="button">Cancel</button>
        <button class="btn-primary" data-action="submit-edit" data-comment-id="${comment.id}" type="button">Save</button>
      </div>
    `;

		if (commentEl) {
			commentEl.appendChild(formEl);
		} else {
			threadEl.appendChild(formEl);
		}

		const textarea = formEl.querySelector('textarea');
		textarea?.focus();
	};

	const submitInlineReply = async (threadEl: HTMLElement, commentId: number): Promise<void> => {
		const formEl = threadEl.querySelector('.diff-annotation-inline-form');
		const textarea = formEl?.querySelector('textarea') as HTMLTextAreaElement;
		const submitBtn = formEl?.querySelector('[data-action="submit-reply"]') as HTMLButtonElement;

		if (!textarea || !textarea.value.trim()) return;

		textarea.disabled = true;
		if (submitBtn) {
			submitBtn.disabled = true;
			submitBtn.textContent = 'Posting...';
		}

		try {
			const newComment = await options.replyToReviewComment(
				options.workspaceId(),
				options.repoId(),
				commentId,
				textarea.value.trim(),
				options.parseNumber(options.prNumberInput()),
				options.prBranchInput().trim() || undefined,
			);
			options.setPrReviews([...options.getPrReviews(), newComment]);
			removeInlineForm(threadEl);
		} catch (error) {
			textarea.disabled = false;
			if (submitBtn) {
				submitBtn.disabled = false;
				submitBtn.textContent = 'Reply';
			}
			options.showAlert(options.formatError(error, 'Failed to post reply.'));
		}
	};

	const submitInlineEdit = async (threadEl: HTMLElement, commentId: number): Promise<void> => {
		const formEl = threadEl.querySelector('.diff-annotation-inline-form');
		const textarea = formEl?.querySelector('textarea') as HTMLTextAreaElement;
		const submitBtn = formEl?.querySelector('[data-action="submit-edit"]') as HTMLButtonElement;

		if (!textarea || !textarea.value.trim()) return;

		textarea.disabled = true;
		if (submitBtn) {
			submitBtn.disabled = true;
			submitBtn.textContent = 'Saving...';
		}

		try {
			const updated = await options.editReviewComment(
				options.workspaceId(),
				options.repoId(),
				commentId,
				textarea.value.trim(),
			);
			options.setPrReviews(
				options
					.getPrReviews()
					.map((comment) =>
						comment.id === updated.id
							? { ...comment, ...updated, threadId: updated.threadId ?? comment.threadId }
							: comment,
					),
			);
			removeInlineForm(threadEl);
		} catch (error) {
			textarea.disabled = false;
			if (submitBtn) {
				submitBtn.disabled = false;
				submitBtn.textContent = 'Save';
			}
			options.showAlert(options.formatError(error, 'Failed to save edit.'));
		}
	};

	const renderAnnotation = (
		annotation: DiffLineAnnotation<ReviewAnnotation>,
	): HTMLElement | undefined => {
		if (!annotation.metadata || annotation.metadata.thread.length === 0) return undefined;
		const threadEl = options.document.createElement('div');
		threadEl.className = 'diff-annotation-thread';

		const lastComment = annotation.metadata.thread[annotation.metadata.thread.length - 1];
		const rootComment = annotation.metadata.thread[0];
		const isResolved = annotation.metadata.resolved;
		const resolvedThreadId =
			rootComment.threadId ??
			(rootComment.nodeId && rootComment.nodeId.startsWith('PRRT_')
				? rootComment.nodeId
				: undefined);

		if (isResolved) {
			threadEl.classList.add('diff-annotation-resolved', 'diff-annotation-collapsed');
		}

		const collapsedHeader = options.document.createElement('div');
		collapsedHeader.className = 'diff-annotation-collapsed-header';
		collapsedHeader.innerHTML = `
        <span class="diff-annotation-collapsed-icon">▸</span>
        <span class="diff-annotation-collapsed-badge">Resolved</span>
        <span class="diff-annotation-collapsed-preview">${escapeHtml(rootComment.body.substring(0, 60))}${rootComment.body.length > 60 ? '...' : ''}</span>
        <span class="diff-annotation-collapsed-count">${annotation.metadata.thread.length} comment${annotation.metadata.thread.length > 1 ? 's' : ''}</span>
      `;
		threadEl.appendChild(collapsedHeader);

		const contentWrapper = options.document.createElement('div');
		contentWrapper.className = 'diff-annotation-content';
		contentWrapper.innerHTML = annotation.metadata.thread
			.map((comment, idx) => {
				const currentUserId = options.getCurrentUserId();
				const isOwn = currentUserId && comment.authorId && comment.authorId === currentUserId;

				return `
        <div class="diff-annotation${idx > 0 ? ' diff-annotation-reply' : ''}" data-comment-id="${comment.id}">
          <div class="diff-annotation-header">
            <span class="diff-annotation-avatar">${comment.author[0].toUpperCase()}</span>
            <span class="diff-annotation-author">${comment.author}</span>
            <div class="diff-annotation-actions">
              ${
								isOwn
									? `
                <button class="diff-action-btn" data-action="edit" data-comment-id="${comment.id}" title="Edit">✎</button>
                <button class="diff-action-btn diff-action-delete" data-action="delete" data-comment-id="${comment.id}" title="Delete">×</button>
              `
									: ''
							}
            </div>
          </div>
          <div class="diff-annotation-body">${renderMarkdown(comment.body)}</div>
        </div>
      `;
			})
			.join('');
		threadEl.appendChild(contentWrapper);

		const footerEl = options.document.createElement('div');
		footerEl.className = 'diff-annotation-footer';
		footerEl.innerHTML = `
        <button class="diff-action-btn diff-action-reply" data-action="reply" data-comment-id="${lastComment.id}" title="Reply">↩ Reply</button>
        ${
					resolvedThreadId
						? `
          <button class="diff-action-btn ${isResolved ? 'diff-action-unresolve' : 'diff-action-resolve'}" data-action="${isResolved ? 'unresolve' : 'resolve'}" data-thread-id="${resolvedThreadId}" title="${isResolved ? 'Unresolve thread' : 'Resolve thread'}">${isResolved ? '↺ Unresolve' : '✓ Resolve'}</button>
        `
						: ''
				}
      `;
		threadEl.appendChild(footerEl);

		threadEl.addEventListener('click', async (event) => {
			const target = event.target as HTMLElement;

			if (target.closest('.diff-annotation-collapsed-header')) {
				threadEl.classList.remove('diff-annotation-collapsed');
				return;
			}

			const btn = target.closest('[data-action]') as HTMLElement;
			if (!btn) return;

			const action = btn.dataset.action;
			const commentId = btn.dataset.commentId ? Number.parseInt(btn.dataset.commentId, 10) : null;
			const threadId = btn.dataset.threadId;

			if (action === 'reply' && commentId) {
				injectReplyForm(threadEl, commentId);
				return;
			}
			if (action === 'edit' && commentId) {
				const comment = options.getPrReviews().find((item) => item.id === commentId);
				if (comment) injectEditForm(threadEl, comment);
				return;
			}
			if (action === 'delete' && commentId) {
				void options.handleDeleteComment(commentId);
				return;
			}
			if (action === 'resolve' && threadId) {
				void options.handleResolveThread(threadId, true);
				return;
			}
			if (action === 'unresolve' && threadId) {
				void options.handleResolveThread(threadId, false);
				return;
			}
			if (action === 'cancel-reply' || action === 'cancel-edit') {
				removeInlineForm(threadEl);
				return;
			}
			if (action === 'submit-reply' && commentId) {
				await submitInlineReply(threadEl, commentId);
				return;
			}
			if (action === 'submit-edit' && commentId) {
				await submitInlineEdit(threadEl, commentId);
			}
		});

		return threadEl;
	};

	return {
		renderAnnotation,
		injectReplyForm,
		injectEditForm,
		removeInlineForm,
		submitInlineReply,
		submitInlineEdit,
	};
};
