<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime';
	import type {
		FileDiff as FileDiffBase,
		FileDiffMetadata,
		FileDiffOptions as FileDiffOptionsBase,
		ParsedPatch,
	} from '@pierre/diffs';
	import type {
		PullRequestCreated,
		PullRequestReviewComment,
		PullRequestStatusResult,
		RemoteInfo,
		Repo,
		RepoDiffFileSummary,
		RepoDiffSummary,
		RepoFileDiff,
	} from '../types';
	import { resolveBranchRefs } from '../diff/branchRefs';
	import {
		commitAndPush,
		createPullRequest,
		deleteReviewComment,
		editReviewComment,
		fetchCurrentGitHubUser,
		fetchTrackedPullRequest,
		fetchPullRequestReviews,
		fetchPullRequestStatus,
		fetchRepoLocalStatus,
		fetchRepoDiffSummary,
		fetchRepoFileDiff,
		fetchBranchDiffSummary,
		fetchBranchFileDiff,
		generatePullRequestText,
		listRemotes,
		replyToReviewComment,
		resolveReviewThread,
	} from '../api';
	import type { RepoLocalStatus } from '../api';
	import GitHubLoginModal from './GitHubLoginModal.svelte';
	import { formatPath } from '../pathUtils';
	import { getPrCreateStageCopy } from '../prCreateProgress';
	import type { PrCreateStage } from '../prCreateProgress';

	/**
	 * Validates and opens URL only if it belongs to trusted GitHub domains.
	 */
	function validateAndOpenURL(url: string | undefined | null): void {
		if (!url) return;

		try {
			const parsed = new URL(url);
			const hostname = parsed.hostname.toLowerCase();

			// Allow github.com and subdomains (for GitHub Enterprise)
			if (hostname === 'github.com' || hostname.endsWith('.github.com')) {
				BrowserOpenURL(url);
			}
		} catch {
			// Invalid URL - silently ignore
		}
	}

	// Local type definitions for @pierre/diffs generic types
	// (The library exports these but TypeScript doesn't resolve the generics correctly)
	type AnnotationSide = 'deletions' | 'additions';

	type DiffLineAnnotation<T = undefined> = {
		side: AnnotationSide;
		lineNumber: number;
	} & (T extends undefined ? { metadata?: undefined } : { metadata: T });

	type FileDiffOptions<T = undefined> = FileDiffOptionsBase & {
		renderAnnotation?: (annotation: DiffLineAnnotation<T>) => HTMLElement | undefined;
	};

	// FileDiff instance type with annotation support
	type FileDiffType<T = undefined> = FileDiffBase & {
		setOptions(options: FileDiffOptions<T> | undefined): void;
		setLineAnnotations(lineAnnotations: DiffLineAnnotation<T>[]): void;
		render(props: {
			fileDiff?: FileDiffMetadata;
			oldFile?: unknown;
			newFile?: unknown;
			forceRender?: boolean;
			fileContainer?: HTMLElement;
			containerWrapper?: HTMLElement;
			lineAnnotations?: DiffLineAnnotation<T>[];
		}): void;
		cleanUp(): void;
	};

	interface Props {
		repo: Repo;
		workspaceId: string;
		onClose: () => void;
	}

	const { repo, workspaceId, onClose }: Props = $props();

	type DiffsModule = {
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		FileDiff: new (options?: FileDiffOptionsBase) => any;
		parsePatchFiles: (patch: string) => ParsedPatch[];
	};

	let summary: RepoDiffSummary | null = $state(null);
	let summaryLoading = $state(true);
	let summaryError: string | null = $state(null);

	let selected: RepoDiffFileSummary | null = $state(null);
	let selectedDiff: FileDiffMetadata | null = $state(null);
	let fileMeta: RepoFileDiff | null = $state(null);
	let fileLoading = $state(false);
	let fileError: string | null = $state(null);

	let diffMode: 'split' | 'unified' = $state('split');
	let diffContainer: HTMLElement | null = $state(null);
	let diffInstance: FileDiffType<ReviewAnnotation> | null = null;
	let diffModule: DiffsModule | null = null;
	let rendererLoading = $state(false);
	let rendererError: string | null = $state(null);

	let prBase = $state('');
	let prBaseRemote = $state('');
	let prDraft = $state(false);
	let prPanelExpanded = $state(false);
	let prCreateError: string | null = $state(null);
	let prCreateSuccess: PullRequestCreated | null = $state(null);
	let prTracked: PullRequestCreated | null = $state(null);
	let prCreating = $state(false);
	let prCreatingStage: PrCreateStage | null = $state(null);

	const prCreateStageCopy = $derived.by(() => getPrCreateStageCopy(prCreatingStage));

	// Remotes list for base remote dropdown
	let remotes: RemoteInfo[] = $state([]);
	let remotesLoading = $state(false);

	// PR panel mode state
	let forceMode: 'create' | 'status' | null = $state(null);

	let prNumberInput = $state('');
	let prBranchInput = $state('');
	let prStatus: PullRequestStatusResult | null = $state(null);
	let prStatusError: string | null = $state(null);
	let prStatusLoading = $state(false);

	let prReviews: PullRequestReviewComment[] = $state([]);
	let prReviewsLoading = $state(false);
	let prReviewsSent = $state(false);

	// Comment management state
	let currentUserId: number | null = $state(null);
	let authModalOpen = $state(false);
	let authModalMessage: string | null = $state(null);
	let authPendingAction: (() => Promise<void>) | null = null;

	let localStatus: RepoLocalStatus | null = $state(null);
	let commitPushLoading = $state(false);
	let commitPushError: string | null = $state(null);
	let commitPushSuccess = $state(false);

	// Local uncommitted changes summary (separate from PR branch diff)
	let localSummary: RepoDiffSummary | null = $state(null);

	// Track which source the selected file is from
	let selectedSource: 'pr' | 'local' = $state('pr');

	// Sidebar tab: 'files' or 'checks'
	let sidebarTab: 'files' | 'checks' = $state('files');

	// Sidebar resize state
	const SIDEBAR_WIDTH_KEY = 'workset:repoDiff:sidebarWidth';
	const MIN_SIDEBAR_WIDTH = 200;
	const DEFAULT_SIDEBAR_WIDTH = 280;
	let sidebarWidth = $state(DEFAULT_SIDEBAR_WIDTH);
	let isResizing = $state(false);

	// Load persisted sidebar width on mount
	onMount(() => {
		try {
			const saved = localStorage.getItem(SIDEBAR_WIDTH_KEY);
			if (saved) {
				const parsed = parseInt(saved, 10);
				if (!isNaN(parsed) && parsed >= MIN_SIDEBAR_WIDTH) {
					sidebarWidth = parsed;
				}
			}
		} catch {
			// localStorage unavailable, use default width
		}
	});

	let summaryRequest = 0;
	let fileRequest = 0;

	// Auto-polling constants
	const POLL_INTERVAL = 30_000;

	// Derived mode: status when PR exists, create otherwise
	const effectiveMode = $derived(forceMode ?? (prTracked ? 'status' : 'create'));

	// Annotation metadata type for review comment threads
	type ReviewCommentItem = {
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

	type ReviewAnnotation = {
		thread: ReviewCommentItem[];
		resolved: boolean;
	};

	const buildOptions = (): FileDiffOptions<ReviewAnnotation> => ({
		theme: 'pierre-dark',
		themeType: 'dark',
		diffStyle: diffMode,
		diffIndicators: 'bars',
		hunkSeparators: 'line-info',
		lineDiffType: 'word',
		overflow: 'scroll',
		disableFileHeader: true,
		renderAnnotation: (annotation: DiffLineAnnotation<ReviewAnnotation>) => {
			if (!annotation.metadata || annotation.metadata.thread.length === 0) return undefined;
			const el = document.createElement('div');
			el.className = 'diff-annotation-thread';

			const lastComment = annotation.metadata.thread[annotation.metadata.thread.length - 1];
			const rootComment = annotation.metadata.thread[0];
			const isResolved = annotation.metadata.resolved;
			const resolvedThreadId =
				rootComment.threadId ??
				(rootComment.nodeId && rootComment.nodeId.startsWith('PRRT_')
					? rootComment.nodeId
					: undefined);

			// If resolved, show collapsed view by default
			if (isResolved) {
				el.classList.add('diff-annotation-resolved', 'diff-annotation-collapsed');
			}

			// Collapsed header for resolved threads
			const collapsedHeader = document.createElement('div');
			collapsedHeader.className = 'diff-annotation-collapsed-header';
			collapsedHeader.innerHTML = `
        <span class="diff-annotation-collapsed-icon">▸</span>
        <span class="diff-annotation-collapsed-badge">Resolved</span>
        <span class="diff-annotation-collapsed-preview">${escapeHtml(rootComment.body.substring(0, 60))}${rootComment.body.length > 60 ? '...' : ''}</span>
        <span class="diff-annotation-collapsed-count">${annotation.metadata.thread.length} comment${annotation.metadata.thread.length > 1 ? 's' : ''}</span>
      `;
			el.appendChild(collapsedHeader);

			// Full thread content wrapper
			const contentWrapper = document.createElement('div');
			contentWrapper.className = 'diff-annotation-content';
			contentWrapper.innerHTML = annotation.metadata.thread
				.map((comment, idx) => {
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
          <div class="diff-annotation-body">${escapeHtml(comment.body)}</div>
        </div>
      `;
				})
				.join('');
			el.appendChild(contentWrapper);

			// Add thread footer with reply and resolve buttons
			const footerEl = document.createElement('div');
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
			el.appendChild(footerEl);

			// Add event listeners
			el.addEventListener('click', async (e) => {
				const target = e.target as HTMLElement;

				// Handle collapsed header click to expand
				if (target.closest('.diff-annotation-collapsed-header')) {
					el.classList.remove('diff-annotation-collapsed');
					return;
				}

				const btn = target.closest('[data-action]') as HTMLElement;
				if (!btn) return;

				const action = btn.dataset.action;
				const commentId = btn.dataset.commentId ? parseInt(btn.dataset.commentId, 10) : null;
				const threadId = btn.dataset.threadId;

				if (action === 'reply' && commentId) {
					injectReplyForm(el, commentId);
				} else if (action === 'edit' && commentId) {
					const comment = prReviews.find((c) => c.id === commentId);
					if (comment) injectEditForm(el, comment);
				} else if (action === 'delete' && commentId) {
					handleDeleteComment(commentId);
				} else if (action === 'resolve' && threadId) {
					handleResolveThread(threadId, true);
				} else if (action === 'unresolve' && threadId) {
					handleResolveThread(threadId, false);
				} else if (action === 'cancel-reply') {
					removeInlineForm(el);
				} else if (action === 'submit-reply' && commentId) {
					await submitInlineReply(el, commentId);
				} else if (action === 'cancel-edit') {
					removeInlineForm(el);
				} else if (action === 'submit-edit' && commentId) {
					await submitInlineEdit(el, commentId);
				}
			});

			return el;
		},
	});

	const injectReplyForm = (threadEl: HTMLElement, commentId: number): void => {
		// Remove any existing form
		removeInlineForm(threadEl);

		const formEl = document.createElement('div');
		formEl.className = 'diff-annotation-inline-form';
		formEl.innerHTML = `
      <textarea class="diff-inline-textarea" placeholder="Write your reply..." rows="3"></textarea>
      <div class="diff-inline-form-actions">
        <button class="btn-ghost" data-action="cancel-reply" type="button">Cancel</button>
        <button class="btn-primary" data-action="submit-reply" data-comment-id="${commentId}" type="button">Reply</button>
      </div>
    `;
		threadEl.appendChild(formEl);

		// Focus the textarea
		const textarea = formEl.querySelector('textarea');
		textarea?.focus();
	};

	const injectEditForm = (threadEl: HTMLElement, comment: PullRequestReviewComment): void => {
		// Remove any existing form
		removeInlineForm(threadEl);

		// Find and hide the original comment body
		const commentEl = threadEl.querySelector(`[data-comment-id="${comment.id}"]`);
		const bodyEl = commentEl?.querySelector('.diff-annotation-body') as HTMLElement;
		if (bodyEl) bodyEl.style.display = 'none';

		const formEl = document.createElement('div');
		formEl.className = 'diff-annotation-inline-form';
		formEl.dataset.editingId = String(comment.id);
		formEl.innerHTML = `
      <textarea class="diff-inline-textarea" rows="3">${escapeHtml(comment.body)}</textarea>
      <div class="diff-inline-form-actions">
        <button class="btn-ghost" data-action="cancel-edit" type="button">Cancel</button>
        <button class="btn-primary" data-action="submit-edit" data-comment-id="${comment.id}" type="button">Save</button>
      </div>
    `;

		// Insert after the comment header
		if (commentEl) {
			commentEl.appendChild(formEl);
		} else {
			threadEl.appendChild(formEl);
		}

		// Focus the textarea
		const textarea = formEl.querySelector('textarea');
		textarea?.focus();
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

	const submitInlineReply = async (threadEl: HTMLElement, commentId: number): Promise<void> => {
		const formEl = threadEl.querySelector('.diff-annotation-inline-form');
		const textarea = formEl?.querySelector('textarea') as HTMLTextAreaElement;
		const submitBtn = formEl?.querySelector('[data-action="submit-reply"]') as HTMLButtonElement;

		if (!textarea || !textarea.value.trim()) return;

		// Disable form while submitting
		textarea.disabled = true;
		if (submitBtn) {
			submitBtn.disabled = true;
			submitBtn.textContent = 'Posting...';
		}

		try {
			const newComment = await replyToReviewComment(
				workspaceId,
				repo.id,
				commentId,
				textarea.value.trim(),
				parseNumber(prNumberInput),
				prBranchInput.trim() || undefined,
			);
			prReviews = [...prReviews, newComment];
			removeInlineForm(threadEl);
			// Re-render the diff to show the new comment
			renderDiff();
		} catch (err) {
			// Re-enable form on error
			textarea.disabled = false;
			if (submitBtn) {
				submitBtn.disabled = false;
				submitBtn.textContent = 'Reply';
			}
			const errorMsg = formatError(err, 'Failed to post reply.');
			alert(errorMsg);
		}
	};

	const submitInlineEdit = async (threadEl: HTMLElement, commentId: number): Promise<void> => {
		const formEl = threadEl.querySelector('.diff-annotation-inline-form');
		const textarea = formEl?.querySelector('textarea') as HTMLTextAreaElement;
		const submitBtn = formEl?.querySelector('[data-action="submit-edit"]') as HTMLButtonElement;

		if (!textarea || !textarea.value.trim()) return;

		// Disable form while submitting
		textarea.disabled = true;
		if (submitBtn) {
			submitBtn.disabled = true;
			submitBtn.textContent = 'Saving...';
		}

		try {
			const updated = await editReviewComment(
				workspaceId,
				repo.id,
				commentId,
				textarea.value.trim(),
			);
			prReviews = prReviews.map((c) =>
				c.id === updated.id ? { ...c, ...updated, threadId: updated.threadId ?? c.threadId } : c,
			);
			removeInlineForm(threadEl);
			// Re-render the diff to show the updated comment
			renderDiff();
		} catch (err) {
			// Re-enable form on error
			textarea.disabled = false;
			if (submitBtn) {
				submitBtn.disabled = false;
				submitBtn.textContent = 'Save';
			}
			const errorMsg = formatError(err, 'Failed to save edit.');
			alert(errorMsg);
		}
	};

	const escapeHtml = (text: string): string => {
		const div = document.createElement('div');
		div.textContent = text;
		return div.innerHTML;
	};

	// Group comments into threads and convert to diff annotations
	// AnnotationSide in @pierre/diffs is 'deletions' | 'additions'
	const buildLineAnnotations = (): DiffLineAnnotation<ReviewAnnotation>[] => {
		const withLine = filteredReviews.filter((r) => r.line != null);
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

	const statusLabel = (status: string): string => {
		switch (status) {
			case 'added':
				return 'added';
			case 'deleted':
				return 'deleted';
			case 'renamed':
				return 'renamed';
			case 'untracked':
				return 'untracked';
			case 'binary':
				return 'binary';
			default:
				return 'modified';
		}
	};

	// Sidebar resize handlers
	const startResize = (e: MouseEvent) => {
		e.preventDefault();
		isResizing = true;
		const startX = e.clientX;
		const startWidth = sidebarWidth;

		const handleMouseMove = (e: MouseEvent) => {
			const diff = e.clientX - startX;
			const newWidth = Math.max(MIN_SIDEBAR_WIDTH, startWidth + diff);
			sidebarWidth = newWidth;
		};

		const handleMouseUp = () => {
			isResizing = false;
			try {
				localStorage.setItem(SIDEBAR_WIDTH_KEY, String(sidebarWidth));
			} catch {
				// localStorage unavailable, width won't persist
			}
			cleanupListeners();
		};

		const handleBlur = () => {
			isResizing = false;
			cleanupListeners();
		};

		const cleanupListeners = () => {
			document.removeEventListener('mousemove', handleMouseMove);
			document.removeEventListener('mouseup', handleMouseUp);
			window.removeEventListener('blur', handleBlur);
		};

		document.addEventListener('mousemove', handleMouseMove);
		document.addEventListener('mouseup', handleMouseUp);
		window.addEventListener('blur', handleBlur);
	};

	const formatError = (err: unknown, fallback: string): string => {
		if (err instanceof Error) return err.message;
		if (typeof err === 'string') return err;
		if (err && typeof err === 'object' && 'message' in err) {
			const message = (err as { message?: string }).message;
			if (typeof message === 'string') return message;
		}
		return fallback;
	};

	const authRequiredPrefix = 'AUTH_REQUIRED:';

	const isAuthRequiredMessage = (message: string): boolean =>
		message.startsWith(authRequiredPrefix);

	const stripAuthPrefix = (message: string): string =>
		message.replace(/^AUTH_REQUIRED:\s*/, '') || 'GitHub authentication required.';

	const runGitHubAction = async (
		action: () => Promise<void>,
		onError: (message: string) => void,
		fallback: string,
	): Promise<void> => {
		if (authModalOpen) {
			authPendingAction = () => runGitHubAction(action, onError, fallback);
			return;
		}
		try {
			await action();
		} catch (err) {
			const message = formatError(err, fallback);
			if (isAuthRequiredMessage(message)) {
				authModalMessage = stripAuthPrefix(message);
				authPendingAction = () => runGitHubAction(action, onError, fallback);
				authModalOpen = true;
				return;
			}
			onError(message);
		}
	};

	const handleAuthSuccess = async (): Promise<void> => {
		authModalOpen = false;
		authModalMessage = null;
		const pending = authPendingAction;
		authPendingAction = null;
		if (pending) {
			await pending();
		}
	};

	const handleAuthClose = (): void => {
		authModalOpen = false;
		authPendingAction = null;
	};

	const parseNumber = (value: string): number | undefined => {
		const parsed = Number.parseInt(value.trim(), 10);
		return Number.isFinite(parsed) ? parsed : undefined;
	};

	const loadPrStatus = async (): Promise<void> => {
		prStatusLoading = true;
		prStatusError = null;
		try {
			await runGitHubAction(
				async () => {
					prStatus = await fetchPullRequestStatus(
						workspaceId,
						repo.id,
						parseNumber(prNumberInput),
						prBranchInput.trim() || undefined,
					);
				},
				(message) => {
					prStatusError = message;
					prStatus = null;
				},
				'Failed to load pull request status.',
			);
		} finally {
			prStatusLoading = false;
		}
	};

	const loadPrReviews = async (): Promise<void> => {
		prReviewsLoading = true;
		// Error handling is done via prReviewsLoading and prReviews state
		prReviewsSent = false;
		try {
			await runGitHubAction(
				async () => {
					prReviews = await fetchPullRequestReviews(
						workspaceId,
						repo.id,
						parseNumber(prNumberInput),
						prBranchInput.trim() || undefined,
					);
					// Also fetch current user for edit/delete permissions
					if (currentUserId === null) {
						loadCurrentUser();
					}
				},
				(_message) => {
					// Error displayed via UI state, clear reviews on error
					prReviews = [];
				},
				'Failed to load review comments.',
			);
		} finally {
			prReviewsLoading = false;
		}
	};

	const loadCurrentUser = async (): Promise<void> => {
		try {
			const user = await fetchCurrentGitHubUser(workspaceId, repo.id);
			currentUserId = user.id;
		} catch {
			// Non-fatal: if we can't get user, edit/delete buttons won't show
			currentUserId = null;
		}
	};

	const loadLocalStatus = async (): Promise<void> => {
		try {
			localStatus = await fetchRepoLocalStatus(workspaceId, repo.id);
		} catch {
			// Non-fatal: local status is optional
			localStatus = null;
		}
	};

	const loadRemotes = async (): Promise<void> => {
		remotesLoading = true;
		try {
			remotes = await listRemotes(workspaceId, repo.id);
		} catch {
			// Non-fatal: remotes loading is optional
			remotes = [];
		} finally {
			remotesLoading = false;
		}
	};

	const handleCommitAndPush = async (): Promise<void> => {
		if (commitPushLoading) return;
		commitPushLoading = true;
		commitPushError = null;
		commitPushSuccess = false;
		try {
			await commitAndPush(workspaceId, repo.id);
			commitPushSuccess = true;
			// Refresh everything
			await handleRefresh();
		} catch (err) {
			commitPushError = formatError(err, 'Failed to commit and push.');
		} finally {
			commitPushLoading = false;
		}
	};

	const handleRefresh = async (): Promise<void> => {
		// Refresh diff summary
		await loadSummary();
		// In status mode, also refresh PR status, reviews, and local status
		if (effectiveMode === 'status') {
			await loadPrStatus();
			await loadPrReviews();
			await loadLocalStatus();
			await loadLocalSummary();
		}
	};

	const handleCreatePR = async (): Promise<void> => {
		if (prCreating) return;
		prPanelExpanded = true;
		prCreating = true;
		prCreatingStage = 'generating';
		prCreateError = null;
		prCreateSuccess = null;
		try {
			await runGitHubAction(
				async () => {
					// Auto-generate title/body
					const generated = await generatePullRequestText(workspaceId, repo.id);

					// Create PR with generated content
					prCreatingStage = 'creating';
					const created = await createPullRequest(workspaceId, repo.id, {
						title: generated.title,
						body: generated.body,
						base: prBase.trim() || undefined,
						baseRemote: prBaseRemote || undefined,
						draft: prDraft,
						autoCommit: true,
						autoPush: true,
					});
					prCreateSuccess = created;
					prTracked = created;
					forceMode = null; // Auto-switch to status mode (polling starts)
					prNumberInput = `${created.number}`;
					prStatus = {
						pullRequest: created,
						checks: [],
					};
				},
				(message) => {
					prCreateError = message;
				},
				'Failed to create pull request.',
			);
		} finally {
			prCreating = false;
			prCreatingStage = null;
		}
	};

	const handleDeleteComment = async (commentId: number): Promise<void> => {
		// Note: native confirm() doesn't work in Wails WebView on macOS
		// The delete button click is explicit enough user intent
		await runGitHubAction(
			async () => {
				await deleteReviewComment(workspaceId, repo.id, commentId);
				await loadPrReviews();
				renderDiff();
			},
			(message) => {
				alert(message);
			},
			'Failed to delete comment.',
		);
	};

	let resolvingThread = $state(false);

	const handleResolveThread = async (threadId: string, resolve: boolean): Promise<void> => {
		if (!threadId) {
			alert('No thread ID found for this comment');
			return;
		}
		if (resolvingThread) return;
		resolvingThread = true;
		await runGitHubAction(
			async () => {
				await resolveReviewThread(workspaceId, repo.id, threadId, resolve);
				// Refresh reviews to get updated state
				await loadPrReviews();
				// Re-render diff with updated data
				renderDiff();
			},
			(message) => {
				alert(message);
			},
			resolve ? 'Failed to resolve thread.' : 'Failed to unresolve thread.',
		);
		resolvingThread = false;
	};

	const filteredReviews = $derived(
		prReviews.filter((comment) => (selected?.path ? comment.path === selected.path : true)),
	);

	// Check stats for compact display
	const checkStats = $derived.by(() => {
		const checks = prStatus?.checks ?? [];
		const passed = checks.filter((c) => c.conclusion === 'success').length;
		const failed = checks.filter((c) => c.conclusion === 'failure').length;
		const pending = checks.filter(
			(c) => !c.conclusion || c.status === 'in_progress' || c.status === 'queued',
		).length;
		return { total: checks.length, passed, failed, pending };
	});

	// Count reviews for a specific file path
	const reviewCountForFile = (path: string): number => {
		return prReviews.filter((comment) => comment.path === path).length;
	};

	const ensureRenderer = async (): Promise<void> => {
		if (diffModule || rendererLoading) return;
		rendererLoading = true;
		rendererError = null;
		try {
			diffModule = (await import('@pierre/diffs')) as DiffsModule;
		} catch (err) {
			rendererError = formatError(err, 'Diff renderer failed to load.');
		} finally {
			rendererLoading = false;
		}
	};

	const loadTrackedPR = async (): Promise<void> => {
		try {
			const tracked = await fetchTrackedPullRequest(workspaceId, repo.id);
			if (!tracked) {
				return;
			}
			prTracked = tracked;
			if (!prNumberInput) {
				prNumberInput = `${tracked.number}`;
			}
			if (!prBranchInput && tracked.headBranch) {
				prBranchInput = tracked.headBranch;
			}
		} catch {
			// ignore tracking failures
		}
	};

	const renderDiff = (): void => {
		if (!diffModule || !selectedDiff || !diffContainer) return;
		if (!diffInstance) {
			diffInstance = new diffModule.FileDiff(buildOptions()) as FileDiffType<ReviewAnnotation>;
		} else {
			diffInstance.setOptions(buildOptions());
		}
		const annotations = buildLineAnnotations();
		diffInstance.render({
			fileDiff: selectedDiff,
			fileContainer: diffContainer,
			forceRender: true,
			lineAnnotations: annotations,
		});
	};

	const selectFile = (file: RepoDiffFileSummary, source: 'pr' | 'local' = 'pr'): void => {
		selected = file;
		selectedSource = source;
		void loadFileDiff(file);
	};

	const loadLocalSummary = async (): Promise<void> => {
		if (!localStatus?.hasUncommitted) {
			localSummary = null;
			return;
		}
		try {
			localSummary = await fetchRepoDiffSummary(workspaceId, repo.id);
		} catch {
			localSummary = null;
		}
	};

	// Check if we should use branch diff (when PR exists with branches)
	const useBranchDiff = (): { base: string; head: string } | null => {
		return resolveBranchRefs(remotes, prStatus?.pullRequest ?? prTracked);
	};

	const loadSummary = async (): Promise<void> => {
		summaryLoading = true;
		summaryError = null;
		summary = null;
		selected = null;
		selectedDiff = null;
		fileMeta = null;
		fileError = null;
		if (repo.statusKnown !== false && repo.missing) {
			summaryError = 'Repo is missing on disk. Restore it to view the diff.';
			summaryLoading = false;
			return;
		}
		const requestId = ++summaryRequest;
		try {
			const branchRefs = useBranchDiff();
			const data = branchRefs
				? await fetchBranchDiffSummary(workspaceId, repo.id, branchRefs.base, branchRefs.head)
				: await fetchRepoDiffSummary(workspaceId, repo.id);
			if (requestId !== summaryRequest) return;
			summary = data;
			if (summary.files.length > 0) {
				selectFile(summary.files[0]);
			}
		} catch (err) {
			if (requestId !== summaryRequest) return;
			summaryError = formatError(err, 'Failed to load diff summary.');
		} finally {
			if (requestId === summaryRequest) {
				summaryLoading = false;
			}
		}
	};

	const loadFileDiff = async (file: RepoDiffFileSummary): Promise<void> => {
		fileLoading = true;
		fileError = null;
		fileMeta = null;
		selectedDiff = null;
		const requestId = ++fileRequest;

		if (file.binary) {
			fileError = 'Binary files are not rendered yet.';
			fileLoading = false;
			return;
		}
		try {
			// Use local repo diff for local files, branch diff for PR files
			const branchRefs = selectedSource === 'local' ? null : useBranchDiff();
			const response = branchRefs
				? await fetchBranchFileDiff(
						workspaceId,
						repo.id,
						branchRefs.base,
						branchRefs.head,
						file.path,
						file.prevPath ?? '',
					)
				: await fetchRepoFileDiff(
						workspaceId,
						repo.id,
						file.path,
						file.prevPath ?? '',
						file.status,
					);
			if (requestId !== fileRequest) return;
			fileMeta = response;
			if (response.truncated) {
				const kb = Math.max(1, Math.round(response.totalBytes / 1024));
				fileError = `Diff too large (${response.totalLines} lines, ${kb} KB).`;
				return;
			}
			if (!response.patch.trim()) {
				fileError = 'No diff available for this file.';
				return;
			}
			await ensureRenderer();
			if (!diffModule) {
				fileError = rendererError ?? 'Diff renderer unavailable.';
				return;
			}
			const parsed = diffModule.parsePatchFiles(response.patch);
			const fileDiff = parsed[0]?.files?.[0] ?? null;
			if (!fileDiff) {
				fileError = 'Unable to parse diff content.';
				return;
			}
			selectedDiff = fileDiff;
			renderDiff();
		} catch (err) {
			if (requestId !== fileRequest) return;
			fileError = formatError(err, 'Failed to load file diff.');
		} finally {
			if (requestId === fileRequest) {
				fileLoading = false;
			}
		}
	};

	onMount(() => {
		void loadSummary();
		void loadTrackedPR();
		void loadRemotes();
	});

	onDestroy(() => {
		diffInstance?.cleanUp();
	});

	$effect(() => {
		if (selectedDiff && diffContainer) {
			renderDiff();
		}
	});

	// Reload diff when PR branch info becomes available (switch from local to branch diff)
	let lastBranchKey: string | null = null;
	$effect(() => {
		const branchRefs = useBranchDiff();
		const newKey = branchRefs ? `${branchRefs.base}..${branchRefs.head}` : null;
		if (newKey !== lastBranchKey && newKey !== null) {
			lastBranchKey = newKey;
			// Reload summary with branch diff
			void loadSummary();
		}
	});

	// Re-render diff with updated annotations when reviews change
	$effect(() => {
		// Track filteredReviews to trigger re-render when they change
		const reviewCount = filteredReviews.length;
		if (diffContainer && selectedDiff && reviewCount >= 0) {
			// Re-render to update annotations
			renderDiff();
		}
	});

	// Auto-poll PR status, reviews, local status, and local summary when in status mode
	$effect(() => {
		if (effectiveMode !== 'status') return;

		// Initial load
		void loadPrStatus();
		void loadPrReviews();
		void loadLocalStatus().then(() => loadLocalSummary());

		// Set up polling
		const interval = setInterval(async () => {
			await loadPrStatus();
			await loadPrReviews();
			await loadLocalStatus();
			await loadLocalSummary();
		}, POLL_INTERVAL);

		// Cleanup on mode change
		return () => clearInterval(interval);
	});
</script>

<section class="diff">
	<header class="diff-header">
		<div class="title">
			<div class="repo-name">{repo.name}</div>
			<div class="meta">
				{#if repo.defaultBranch}
					<span>Default branch: {repo.defaultBranch}</span>
				{/if}
				{#if repo.statusKnown === false}
					<span class="status unknown">unknown</span>
				{:else if repo.missing}
					<span class="status missing">missing</span>
				{:else if repo.dirty}
					<span class="status dirty">dirty</span>
				{:else}
					<span class="status clean">clean</span>
				{/if}
				{#if summary}
					<span>Files: {summary.files.length}</span>
					<span class="diffstat"
						><span class="add">+{summary.totalAdded}</span><span class="sep">/</span><span
							class="del">-{summary.totalRemoved}</span
						></span
					>
				{/if}
				<!-- PR status badge (inline in header when in status mode) -->
				{#if effectiveMode === 'status' && prStatus}
					<button
						class="pr-badge"
						type="button"
						onclick={() => validateAndOpenURL(prStatus?.pullRequest?.url)}
						title="Open PR #{prStatus.pullRequest.number} on GitHub"
					>
						<span class="pr-badge-number">PR #{prStatus.pullRequest.number}</span>
						<span class="pr-badge-state pr-badge-state-{prStatus.pullRequest.state.toLowerCase()}"
							>{prStatus.pullRequest.state}</span
						>
						<span class="pr-badge-divider">·</span>
						{#if checkStats.total === 0}
							<span class="pr-badge-checks muted">No checks</span>
						{:else if checkStats.failed > 0}
							<span class="pr-badge-checks failed">✗ {checkStats.failed}</span>
						{:else if checkStats.pending > 0}
							<span class="pr-badge-checks pending">● {checkStats.pending}</span>
						{:else}
							<span class="pr-badge-checks passed">✓ {checkStats.passed}</span>
						{/if}
						{#if prStatusLoading || prReviewsLoading}
							<span class="pr-badge-sync"></span>
						{/if}
					</button>
				{:else if effectiveMode === 'status' && prStatusLoading}
					<span class="pr-badge-loading">PR...</span>
				{/if}
			</div>
		</div>
		<div class="controls">
			<div class="toggle">
				<button
					class:active={diffMode === 'split'}
					onclick={() => {
						diffMode = 'split';
						renderDiff();
					}}
					type="button"
				>
					Split
				</button>
				<button
					class:active={diffMode === 'unified'}
					onclick={() => {
						diffMode = 'unified';
						renderDiff();
					}}
					type="button"
				>
					Unified
				</button>
			</div>
			<button class="ghost" type="button" onclick={handleRefresh}>Refresh</button>
			<button class="close" onclick={onClose} type="button">Back to terminal</button>
		</div>
	</header>

	<!-- PR Create form (only shown in create mode) -->
	{#if effectiveMode === 'create'}
		<section class="pr-panel">
			<button
				class="pr-panel-toggle"
				type="button"
				onclick={() => (prPanelExpanded = !prPanelExpanded)}
			>
				<span class="pr-panel-toggle-icon">{prPanelExpanded ? '▾' : '▸'}</span>
				<span class="pr-title">Create Pull Request</span>
			</button>

			<div class="pr-panel-content" class:expanded={prPanelExpanded}>
				<div class="pr-panel-inner">
					<div class="pr-form-row">
						<label class="field-inline">
							<span>Target</span>
							<select
								bind:value={prBaseRemote}
								disabled={remotesLoading}
								title="Base remote (defaults to upstream if available)"
							>
								<option value="">Auto</option>
								{#each remotes as remote (remote.name)}
									<option value={remote.name}>{remote.name}</option>
								{/each}
							</select>
						</label>
						<span class="field-separator">/</span>
						<label class="field-inline">
							<input
								class="branch-input"
								type="text"
								bind:value={prBase}
								placeholder="main"
								autocapitalize="off"
								autocorrect="off"
								spellcheck="false"
							/>
						</label>
						<label class="checkbox-inline">
							<input type="checkbox" bind:checked={prDraft} />
							Draft
						</label>
						<button
							class="pr-create-btn"
							type="button"
							onclick={handleCreatePR}
							disabled={prCreating}
						>
							{#if prCreating}
								<span class="pr-create-spinner" aria-hidden="true">
									<svg
										class="pr-create-spinner-icon"
										viewBox="0 0 24 24"
										fill="none"
										stroke="currentColor"
										stroke-width="2"
									>
										<circle cx="12" cy="12" r="9" opacity="0.25" />
										<path d="M21 12a9 9 0 0 0-9-9" stroke-linecap="round" />
									</svg>
								</span>
							{/if}
							<span class="pr-create-label"
								>{prCreating ? prCreateStageCopy?.button ?? 'Creating PR...' : 'Create PR'}</span
							>
						</button>
					</div>

					{#if prCreating && prCreateStageCopy}
						<div class="pr-create-progress" role="status" aria-live="polite">
							{prCreateStageCopy.detail}
						</div>
					{/if}

					{#if prCreateError}
						<div class="error">{prCreateError}</div>
					{/if}

					{#if prTracked && !prCreateSuccess}
						<div class="info-banner">
							Existing PR #{prTracked.number} found.
							<button class="mode-link" type="button" onclick={() => (forceMode = 'status')}>
								View status →
							</button>
						</div>
					{/if}
				</div>
			</div>
		</section>
	{/if}

	<!-- Status errors/success banners (only in status mode) -->
	{#if effectiveMode === 'status'}
		{#if prStatusError}
			<div class="error-banner compact">{prStatusError}</div>
		{/if}
		{#if prReviewsSent}
			<div class="success-banner compact">Sent to terminal</div>
		{/if}
	{/if}

	<!-- Local uncommitted changes banner (in status mode when PR exists) -->
	{#if effectiveMode === 'status' && localStatus?.hasUncommitted}
		<section class="local-changes-banner">
			<span class="local-changes-text">You have uncommitted local changes</span>
			<button
				class="commit-push-btn"
				type="button"
				onclick={handleCommitAndPush}
				disabled={commitPushLoading}
			>
				{commitPushLoading ? 'Committing...' : 'Commit & Push'}
			</button>
		</section>
		{#if commitPushError}
			<div class="error-banner compact">{commitPushError}</div>
		{/if}
		{#if commitPushSuccess}
			<div class="success-banner compact">Changes committed and pushed</div>
		{/if}
	{/if}

	{#if summaryLoading}
		<div class="state">Loading diff summary...</div>
	{:else if summaryError}
		<div class="state error">
			<div class="message">{summaryError}</div>
			<button class="ghost" type="button" onclick={loadSummary}>Retry</button>
		</div>
	{:else if !summary || summary.files.length === 0}
		<div class="state">No changes detected in this repo.</div>
	{:else}
		<div class="diff-body" style="--sidebar-width: {sidebarWidth}px">
			<aside class="file-list">
				<!-- Sidebar tabs (only show when in status mode with checks) -->
				{#if effectiveMode === 'status' && prStatus && prStatus.checks.length > 0}
					<div class="sidebar-tabs">
						<button
							class="sidebar-tab"
							class:active={sidebarTab === 'files'}
							type="button"
							onclick={() => (sidebarTab = 'files')}
						>
							Files
							<span class="tab-count"
								>{summary.files.length + (localSummary?.files.length ?? 0)}</span
							>
						</button>
						<button
							class="sidebar-tab"
							class:active={sidebarTab === 'checks'}
							type="button"
							onclick={() => (sidebarTab = 'checks')}
						>
							Checks
							{#if checkStats.failed > 0}
								<span class="tab-count failed">✗ {checkStats.failed}</span>
							{:else if checkStats.pending > 0}
								<span class="tab-count pending">● {checkStats.pending}</span>
							{:else}
								<span class="tab-count passed">✓ {checkStats.passed}</span>
							{/if}
						</button>
					</div>
				{/if}

				<!-- Files tab content -->
				{#if sidebarTab === 'files'}
					<!-- Local uncommitted changes section (yellow) -->
					{#if localSummary && localSummary.files.length > 0}
						<div class="section-title local-section-title">Uncommitted changes</div>
						{#each localSummary.files as file (file.path)}
							<button
								class:selected={file.path === selected?.path &&
									file.prevPath === selected?.prevPath &&
									selectedSource === 'local'}
								class="file-row local-file"
								onclick={() => selectFile(file, 'local')}
								type="button"
							>
									<div class="file-meta">
										<span class="path" title={file.path}>{formatPath(file.path)}</span>
										{#if file.prevPath}
											<span class="rename">from {file.prevPath}</span>
										{/if}
									</div>
									<div class="stats">
										<span class="tag local-tag">{statusLabel(file.status)}</span>
									<span class="diffstat local-diffstat"
										><span class="add">+{file.added}</span><span class="sep">/</span><span
											class="del">-{file.removed}</span
										></span
									>
								</div>
							</button>
						{/each}
					{/if}

					<!-- PR changed files section -->
					{#if !(effectiveMode === 'status' && prStatus && prStatus.checks.length > 0)}
						<div class="section-title">
							{localSummary && localSummary.files.length > 0 ? 'PR files' : 'Changed files'}
						</div>
					{:else if localSummary && localSummary.files.length > 0}
						<div class="section-title">PR files</div>
					{/if}
					{#each summary.files as file (file.path)}
						{@const reviewCount = reviewCountForFile(file.path)}
						<button
							class:selected={file.path === selected?.path &&
								file.prevPath === selected?.prevPath &&
								selectedSource === 'pr'}
							class="file-row"
							onclick={() => selectFile(file, 'pr')}
							type="button"
						>
							<div class="file-meta">
								<span class="path" title={file.path}>{formatPath(file.path)}</span>
								{#if file.prevPath}
									<span class="rename">from {file.prevPath}</span>
								{/if}
							</div>
							<div class="stats">
								{#if reviewCount > 0}
									<span
										class="review-badge"
										title="{reviewCount} review comment{reviewCount > 1 ? 's' : ''}"
									>
										💬 {reviewCount}
									</span>
								{/if}
								<span class="tag {file.status}">{statusLabel(file.status)}</span>
								<span class="diffstat"
									><span class="add">+{file.added}</span><span class="sep">/</span><span class="del"
										>-{file.removed}</span
									></span
								>
							</div>
						</button>
					{/each}
				{/if}

				<!-- Checks tab content -->
				{#if sidebarTab === 'checks' && prStatus}
					<div class="checks-tab-content">
						{#each prStatus.checks as check (check.name)}
							<div class="check-row check-{check.conclusion || check.status}">
								<span class="check-indicator">
									{#if check.conclusion === 'success'}
										✓
									{:else if check.conclusion === 'failure'}
										✗
									{:else if check.status === 'in_progress' || check.status === 'queued'}
										<span class="spinner"></span>
									{:else}
										●
									{/if}
								</span>
								<span class="check-name">{check.name}</span>
								{#if check.detailsUrl}
									<button
										class="check-link"
										type="button"
										onclick={() => check.detailsUrl && BrowserOpenURL(check.detailsUrl)}
										title="View on GitHub"
									>
										↗
									</button>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</aside>
			<div
				class="resize-handle"
				class:resizing={isResizing}
				onmousedown={startResize}
				role="separator"
				aria-label="Resize sidebar"
			></div>
			<div class="diff-view">
				<div class="file-header">
					<div class="file-title">
						<span>{selected?.path}</span>
						{#if selected?.prevPath}
							<span class="rename">from {selected.prevPath}</span>
						{/if}
					</div>
					<span class="diffstat">
						<span class="add">+{selected?.added ?? 0}</span><span class="sep">/</span><span
							class="del">-{selected?.removed ?? 0}</span
						>
						{#if fileMeta && !fileMeta.truncated && fileMeta.totalLines > 0}
							<span class="line-count">{fileMeta.totalLines} lines</span>
						{/if}
					</span>
				</div>
				{#if fileLoading || rendererLoading}
					<div class="state compact">Loading file diff...</div>
				{:else if fileError}
					<div class="state compact">{fileError}</div>
				{:else if rendererError}
					<div class="state compact">{rendererError}</div>
				{:else}
					<div class="diff-renderer">
						<diffs-container bind:this={diffContainer}></diffs-container>
					</div>
				{/if}
			</div>
		</div>
	{/if}
</section>

{#if authModalOpen}
	<div
		class="overlay"
		role="button"
		tabindex="0"
		onclick={handleAuthClose}
		onkeydown={(event) => {
			if (event.key === 'Escape') handleAuthClose();
		}}
	>
		<div
			class="overlay-panel"
			role="presentation"
			onclick={(event) => event.stopPropagation()}
			onkeydown={(event) => event.stopPropagation()}
		>
			<GitHubLoginModal
				notice={authModalMessage}
				onClose={handleAuthClose}
				onSuccess={handleAuthSuccess}
			/>
		</div>
	</div>
{/if}

<style>
	/* Sidebar tabs */
	.sidebar-tabs {
		display: flex;
		gap: 0;
		margin-bottom: 16px;
		border-bottom: 1px solid var(--border);
	}

	.sidebar-tab {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 10px 12px;
		border: none;
		border-bottom: 2px solid transparent;
		margin-bottom: -1px;
		background: transparent;
		color: var(--muted);
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.sidebar-tab:hover:not(.active) {
		color: var(--text);
	}

	.sidebar-tab.active {
		color: var(--text);
		border-bottom-color: var(--accent);
	}

	.tab-count {
		font-size: 11px;
		font-weight: 600;
		padding: 2px 6px;
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.08);
	}

	.tab-count.passed {
		color: #3fb950;
		background: rgba(46, 160, 67, 0.15);
	}
	.tab-count.failed {
		color: #f85149;
		background: rgba(248, 81, 73, 0.15);
	}
	.tab-count.pending {
		color: #d29922;
		background: rgba(210, 153, 34, 0.15);
	}

	/* Checks tab content */
	.checks-tab-content {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.check-row {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px;
		border-radius: 10px;
		font-size: 13px;
		transition: background 0.15s ease;
		border-left: 3px solid transparent;
	}

	.check-row:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.check-row.check-success {
		background: rgba(46, 160, 67, 0.1);
		border-left-color: #3fb950;
	}

	.check-row.check-failure {
		background: rgba(248, 81, 73, 0.1);
		border-left-color: #f85149;
	}

	.check-row.check-in_progress,
	.check-row.check-queued,
	.check-row.check-pending {
		background: rgba(210, 153, 34, 0.1);
		border-left-color: #d29922;
	}

	.check-row .check-indicator {
		width: 22px;
		height: 22px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 12px;
		font-weight: 700;
		flex-shrink: 0;
	}

	.check-row.check-success .check-indicator {
		background: #3fb950;
		color: #fff;
	}

	.check-row.check-failure .check-indicator {
		background: #f85149;
		color: #fff;
	}

	.check-row.check-in_progress .check-indicator,
	.check-row.check-queued .check-indicator,
	.check-row.check-pending .check-indicator {
		background: #d29922;
		color: #fff;
	}

	.check-row .check-name {
		color: var(--text);
		font-weight: 500;
		flex: 1;
	}

	.check-row .check-link {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		font-size: 14px;
		cursor: pointer;
		opacity: 0;
		transition: all 0.15s ease;
	}

	.check-row:hover .check-link {
		opacity: 1;
	}

	.check-row .check-link:hover {
		background: rgba(255, 255, 255, 0.1);
		color: var(--text);
	}

	.check-row .spinner {
		width: 12px;
		height: 12px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top-color: #fff;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	/* Local changes warning banner */
	.local-changes-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 12px 16px;
		border-radius: 10px;
		background: rgba(210, 153, 34, 0.12);
		border: 1px solid rgba(210, 153, 34, 0.35);
	}

	.local-changes-text {
		font-size: 13px;
		font-weight: 500;
		color: #d29922;
	}

	/* Local files section styling (yellow) */
	.local-section-title {
		color: #d29922 !important;
	}

	.file-row.local-file {
		border-left: 2px solid rgba(210, 153, 34, 0.5);
		background: rgba(210, 153, 34, 0.06);
	}

	.file-row.local-file:hover:not(.selected) {
		border-color: rgba(210, 153, 34, 0.4);
		background: rgba(210, 153, 34, 0.12);
		border-left-color: #d29922;
	}

	.file-row.local-file.selected {
		background: rgba(210, 153, 34, 0.18);
		border-color: rgba(210, 153, 34, 0.5);
		border-left-color: #d29922;
	}

	.local-tag {
		color: #d29922 !important;
	}

	.local-diffstat .add {
		color: #d29922 !important;
	}

	.local-diffstat .del {
		color: #d29922 !important;
		opacity: 0.7;
	}

	.commit-push-btn {
		padding: 8px 16px;
		border-radius: 8px;
		border: none;
		background: linear-gradient(135deg, #d29922 0%, #b8860b 100%);
		color: #1a1a1a;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		transition:
			transform 0.15s ease,
			box-shadow 0.15s ease,
			opacity 0.15s ease;
	}

	.commit-push-btn:hover:not(:disabled) {
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(210, 153, 34, 0.3);
	}

	.commit-push-btn:active:not(:disabled) {
		transform: translateY(0);
	}

	.commit-push-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.pr-panel {
		border-radius: 14px;
		background: var(--panel);
		border: 1px solid var(--border);
		overflow: hidden;
	}

	.pr-panel-toggle {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 14px 16px;
		background: transparent;
		border: none;
		cursor: pointer;
		text-align: left;
		transition: background 0.15s ease;
	}

	.pr-panel-toggle:hover {
		background: rgba(255, 255, 255, 0.03);
	}

	.pr-panel-toggle-icon {
		font-size: 12px;
		color: var(--muted);
		width: 12px;
	}

	.pr-title {
		font-weight: 600;
		font-size: 14px;
		color: var(--text);
	}

	.pr-panel-content {
		display: grid;
		grid-template-rows: 0fr;
		transition: grid-template-rows 0.2s ease;
	}

	.pr-panel-content.expanded {
		grid-template-rows: 1fr;
	}

	.pr-panel-inner {
		overflow: hidden;
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 0 16px 14px;
	}

	.pr-form-row {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.field-inline {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--muted);
	}

	.field-inline span {
		white-space: nowrap;
	}

	.field-inline input,
	.field-inline select {
		background: var(--panel-soft);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 6px 10px;
		color: var(--text);
		font-size: 13px;
		font-family: inherit;
	}

	.field-inline select {
		cursor: pointer;
		appearance: none;
		background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%238b949e' d='M3 4.5L6 7.5L9 4.5'/%3E%3C/svg%3E");
		background-repeat: no-repeat;
		background-position: right 8px center;
		padding-right: 26px;
		min-width: 80px;
	}

	.field-inline .branch-input {
		width: 120px;
	}

	.field-separator {
		color: var(--muted);
		font-size: 14px;
	}

	.checkbox-inline {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--muted);
		white-space: nowrap;
	}

	.pr-create-btn {
		padding: 6px 14px;
		border-radius: 8px;
		border: none;
		background: var(--accent);
		color: var(--text);
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
		white-space: nowrap;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		transition: opacity 0.15s ease;
	}

	.pr-create-btn:hover:not(:disabled) {
		opacity: 0.9;
	}

	.pr-create-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.pr-create-spinner {
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}

	.pr-create-spinner-icon {
		width: 12px;
		height: 12px;
		animation: pr-create-spin 0.8s linear infinite;
	}

	.pr-create-progress {
		font-size: 12px;
		color: var(--muted);
		display: flex;
		align-items: center;
		gap: 8px;
	}

	@keyframes pr-create-spin {
		to {
			transform: rotate(360deg);
		}
	}

	.poll-time {
		margin-left: auto;
		font-size: 11px;
		color: var(--muted);
	}

	.info-banner {
		font-size: 12px;
		color: var(--muted);
		padding: 8px 10px;
		background: var(--panel-soft);
		border-radius: 8px;
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.mode-link {
		font-size: 12px;
		color: var(--muted);
		cursor: pointer;
		background: none;
		border: none;
		padding: 0;
	}

	.mode-link:hover {
		color: var(--text);
	}

	.row {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
		gap: 10px;
	}

	.checkbox {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--text);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}

	.actions button {
		border-radius: 10px;
		border: 1px solid var(--border);
		background: var(--accent);
		color: var(--text);
		padding: 8px 12px;
		font-size: 12px;
		cursor: pointer;
	}

	/* Inline PR Badge (shown in header meta) */
	.pr-badge {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 5px 12px;
		border-radius: 999px;
		background: rgba(99, 102, 241, 0.1);
		border: 1px solid rgba(99, 102, 241, 0.25);
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.pr-badge:hover {
		background: rgba(99, 102, 241, 0.18);
		border-color: rgba(99, 102, 241, 0.4);
	}

	.pr-badge-number {
		color: var(--text);
		font-weight: 600;
	}

	.pr-badge-state {
		text-transform: uppercase;
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.04em;
	}

	.pr-badge-state-open {
		color: #3fb950;
	}
	.pr-badge-state-closed {
		color: #a78bfa;
	}
	.pr-badge-state-merged {
		color: #a78bfa;
	}

	.pr-badge-divider {
		color: var(--muted);
		opacity: 0.5;
	}

	.pr-badge-checks {
		font-size: 12px;
		font-weight: 500;
	}

	.pr-badge-checks.passed {
		color: #3fb950;
	}
	.pr-badge-checks.failed {
		color: #f85149;
	}
	.pr-badge-checks.pending {
		color: #d29922;
	}
	.pr-badge-checks.muted {
		color: var(--muted);
	}

	.pr-badge-sync {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--accent);
		animation: pulse 1.5s ease infinite;
	}

	.pr-badge-loading {
		font-size: 13px;
		color: var(--muted);
	}

	@keyframes pulse {
		0%,
		100% {
			opacity: 0.4;
		}
		50% {
			opacity: 1;
		}
	}

	.btn-text {
		background: none;
		border: none;
		padding: 0;
		font-size: 12px;
		color: var(--accent);
		cursor: pointer;
		transition: color 0.15s ease;
	}

	.btn-text:hover:not(:disabled) {
		color: var(--text);
	}

	.btn-text:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-text.muted {
		color: var(--muted);
	}

	.loading-skeleton.horizontal {
		display: flex;
		gap: 12px;
		padding: 8px 0;
	}

	.skeleton-line.short {
		width: 60px;
	}

	/* Colored Badges */
	.badge {
		padding: 3px 8px;
		border-radius: 999px;
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}

	.badge-open {
		background: rgba(46, 160, 67, 0.15);
		color: #3fb950;
		border: 1px solid rgba(46, 160, 67, 0.3);
	}

	.badge-closed {
		background: rgba(139, 92, 246, 0.15);
		color: #a78bfa;
		border: 1px solid rgba(139, 92, 246, 0.3);
	}

	.badge-merged {
		background: rgba(139, 92, 246, 0.15);
		color: #a78bfa;
		border: 1px solid rgba(139, 92, 246, 0.3);
	}

	.badge-draft {
		background: rgba(139, 148, 158, 0.15);
		color: #8b949e;
		border: 1px solid rgba(139, 148, 158, 0.3);
	}

	.badge-mergeable {
		background: rgba(46, 160, 67, 0.15);
		color: #3fb950;
		border: 1px solid rgba(46, 160, 67, 0.3);
	}

	.badge-conflicting {
		background: rgba(248, 81, 73, 0.15);
		color: #f85149;
		border: 1px solid rgba(248, 81, 73, 0.3);
	}

	.badge-unknown {
		background: rgba(139, 148, 158, 0.15);
		color: #8b949e;
		border: 1px solid rgba(139, 148, 158, 0.3);
	}

	/* Live Indicator */
	.live-indicator {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 11px;
		color: var(--accent);
	}

	.live-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--accent);
		animation: pulse 1.5s ease-in-out infinite;
	}

	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
			transform: scale(1);
		}
		50% {
			opacity: 0.5;
			transform: scale(0.8);
		}
	}

	.poll-time {
		font-size: 11px;
		color: var(--muted);
	}

	/* Loading Skeleton */
	.loading-skeleton {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 12px;
	}

	.skeleton-line {
		height: 12px;
		border-radius: 6px;
		background: linear-gradient(
			90deg,
			var(--panel-soft) 25%,
			var(--border) 50%,
			var(--panel-soft) 75%
		);
		background-size: 200% 100%;
		animation: shimmer 1.5s infinite;
	}

	.skeleton-line.wide {
		width: 80%;
	}
	.skeleton-line.medium {
		width: 50%;
	}

	@keyframes shimmer {
		0% {
			background-position: 200% 0;
		}
		100% {
			background-position: -200% 0;
		}
	}

	@keyframes fadeIn {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.branch {
		padding: 2px 6px;
		border-radius: 4px;
		background: rgba(56, 139, 253, 0.1);
		color: #58a6ff;
		font-family: var(--font-mono);
		font-size: 11px;
	}

	.arrow {
		color: var(--muted);
		font-size: 10px;
	}

	.pr-link {
		font-size: 11px;
		color: var(--accent);
		background: none;
		border: none;
		padding: 0;
		cursor: pointer;
		transition: color 0.15s ease;
	}

	.pr-link:hover {
		color: var(--text);
	}

	/* Sections */
	.section {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.section-header {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.section-title {
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--muted);
	}

	.section-count {
		font-size: 10px;
		padding: 2px 6px;
		border-radius: 999px;
		background: var(--panel-soft);
		color: var(--muted);
	}

	.empty-state {
		font-size: 12px;
		color: var(--muted);
		padding: 12px;
		text-align: center;
		border: 1px dashed var(--border);
		border-radius: 8px;
	}

	/* Checks List */
	.checks-list {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.check-item {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		border-radius: 8px;
		background: var(--panel-soft);
		font-size: 12px;
		animation: fadeIn 0.2s ease;
	}

	.check-indicator {
		width: 18px;
		height: 18px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 10px;
		font-weight: 700;
		flex-shrink: 0;
	}

	.check-indicator.check-success {
		background: rgba(46, 160, 67, 0.2);
		color: #3fb950;
	}

	.check-indicator.check-failure {
		background: rgba(248, 81, 73, 0.2);
		color: #f85149;
	}

	.check-indicator.check-in_progress,
	.check-indicator.check-queued,
	.check-indicator.check-pending {
		background: rgba(210, 153, 34, 0.2);
		color: #d29922;
	}

	.check-indicator.check-skipped,
	.check-indicator.check-neutral {
		background: rgba(139, 148, 158, 0.2);
		color: #8b949e;
	}

	.spinner {
		width: 10px;
		height: 10px;
		border: 2px solid currentColor;
		border-top-color: transparent;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.check-name {
		flex: 1;
		color: var(--text);
		font-weight: 500;
	}

	.check-status-label {
		font-size: 11px;
		font-weight: 500;
		text-transform: capitalize;
	}

	.check-status-label.check-success {
		color: #3fb950;
	}
	.check-status-label.check-failure {
		color: #f85149;
	}
	.check-status-label.check-in_progress,
	.check-status-label.check-queued,
	.check-status-label.check-pending {
		color: #d29922;
	}
	.check-status-label.check-skipped,
	.check-status-label.check-neutral {
		color: #8b949e;
	}

	/* Error & Success Banners */
	.error-banner {
		padding: 10px 12px;
		border-radius: 8px;
		background: rgba(248, 81, 73, 0.1);
		border: 1px solid rgba(248, 81, 73, 0.3);
		color: #f85149;
		font-size: 12px;
	}

	.error-banner.compact,
	.success-banner.compact {
		padding: 6px 10px;
		font-size: 11px;
	}

	.success-banner {
		padding: 10px 12px;
		border-radius: 8px;
		background: rgba(46, 160, 67, 0.1);
		border: 1px solid rgba(46, 160, 67, 0.3);
		color: #3fb950;
		font-size: 12px;
		animation: fadeIn 0.2s ease;
	}

	.empty-state.compact {
		padding: 8px;
		font-size: 11px;
	}

	.btn-primary {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 10px 16px;
		border-radius: 10px;
		border: none;
		background: linear-gradient(135deg, var(--accent) 0%, #6366f1 100%);
		color: white;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		transition:
			transform 0.15s ease,
			box-shadow 0.15s ease;
	}

	.btn-primary:hover:not(:disabled) {
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
	}

	.btn-primary:active:not(:disabled) {
		transform: translateY(0);
	}

	.btn-primary:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.btn-icon {
		font-size: 14px;
	}

	.btn-ghost {
		padding: 10px 16px;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: transparent;
		color: var(--muted);
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
		transition:
			border-color 0.15s ease,
			color 0.15s ease;
	}

	.btn-ghost:hover {
		border-color: var(--accent);
		color: var(--text);
	}

	/* Legacy support */
	.error {
		color: var(--danger);
		font-size: 12px;
	}

	.success {
		color: var(--success);
		font-size: 12px;
	}

	.diff {
		display: flex;
		flex-direction: column;
		gap: 16px;
		height: 100%;
		padding: 16px;
	}

	.diff-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 16px;
	}

	.title {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.repo-name {
		font-size: 20px;
		font-weight: 600;
	}

	.meta {
		display: flex;
		align-items: center;
		gap: 12px;
		color: var(--muted);
		font-size: 12px;
		flex-wrap: wrap;
	}

	.diffstat {
		font-weight: 600;
		display: inline-flex;
		gap: 8px;
		align-items: center;
	}

	.diffstat .add {
		color: var(--success);
	}

	.diffstat .del {
		color: var(--danger);
	}

	.diffstat .sep {
		color: var(--muted);
		margin: 0 -6px;
	}

	.line-count {
		font-size: 11px;
		color: var(--muted);
		font-weight: 500;
	}

	.status {
		font-weight: 600;
	}

	.dirty {
		color: var(--warning);
	}

	.missing {
		color: var(--danger);
	}

	.clean {
		color: var(--success);
	}

	.unknown {
		color: var(--muted);
	}

	.controls {
		display: flex;
		gap: 12px;
		align-items: center;
	}

	.toggle {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: 10px;
		overflow: hidden;
		background: var(--panel);
	}

	.toggle button {
		background: transparent;
		border: none;
		color: var(--muted);
		padding: 6px 12px;
		cursor: pointer;
		font-size: 12px;
		transition:
			background var(--transition-fast),
			color var(--transition-fast);
	}

	.toggle button:hover:not(.active) {
		background: rgba(255, 255, 255, 0.04);
	}

	.toggle button.active {
		color: var(--text);
		background: var(--accent-subtle);
	}

	.close {
		background: var(--panel);
		border: 1px solid var(--border);
		color: var(--text);
		border-radius: var(--radius-sm);
		padding: 8px 12px;
		cursor: pointer;
		transition:
			border-color var(--transition-fast),
			background var(--transition-fast);
	}

	.close:hover {
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.04);
	}

	.state {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 20px;
		color: var(--muted);
	}

	.state.compact {
		padding: 16px;
		border-radius: 12px;
		background: var(--panel-soft);
		border: 1px dashed var(--border);
		text-align: center;
	}

	.state.error {
		color: var(--warning);
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.diff-body {
		display: grid;
		grid-template-columns: var(--sidebar-width, 280px) 1fr;
		gap: 8px;
		flex: 1;
		min-height: 0;
		position: relative;
	}

	.resize-handle {
		position: absolute;
		left: calc(var(--sidebar-width, 280px) + 2px);
		top: 0;
		bottom: 0;
		width: 4px;
		background: transparent;
		cursor: col-resize;
		transition: background var(--transition-fast);
		z-index: 10;
		border-radius: 2px;
	}

	.resize-handle:hover,
	.resize-handle.resizing {
		background: var(--accent);
	}

	.resize-handle::after {
		content: '';
		position: absolute;
		inset: 0;
		width: 12px;
		transform: translateX(-4px);
	}

	.file-list {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		overflow: auto;
		scrollbar-width: thin;
		scrollbar-color: var(--border) transparent;
	}

	.file-list::-webkit-scrollbar {
		width: 6px;
	}

	.file-list::-webkit-scrollbar-track {
		background: transparent;
	}

	.file-list::-webkit-scrollbar-thumb {
		background: var(--border);
		border-radius: 3px;
	}

	.file-list::-webkit-scrollbar-thumb:hover {
		background: var(--accent);
	}

	.section-title {
		font-size: 11px;
		font-weight: 500;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		height: 24px;
		display: flex;
		align-items: center;
	}

	.file-row {
		display: flex;
		flex-direction: column;
		gap: 6px;
		background: transparent;
		outline: 1px solid transparent;
		outline-offset: -1px;
		color: var(--text);
		text-align: left;
		padding: 10px;
		border-radius: var(--radius-md);
		cursor: pointer;
		transition:
			outline-color var(--transition-fast),
			background var(--transition-fast);
	}

	.file-row:hover:not(.selected) {
		outline-color: var(--border);
		background: rgba(255, 255, 255, 0.02);
	}

	.file-row.selected {
		background: var(--accent-subtle);
		outline-color: var(--accent);
	}

	.file-meta {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.path {
		font-size: 13px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.rename {
		font-size: 11px;
		color: var(--muted);
	}

	.stats {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--muted);
	}

	.review-badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 6px;
		border-radius: 999px;
		background: linear-gradient(135deg, rgba(99, 102, 241, 0.15) 0%, rgba(139, 92, 246, 0.15) 100%);
		border: 1px solid rgba(99, 102, 241, 0.3);
		font-size: 10px;
		font-weight: 600;
		color: #a78bfa;
	}

	.tag {
		text-transform: uppercase;
		letter-spacing: 0.08em;
		font-size: 10px;
		font-weight: 600;
	}

	.tag.added {
		color: var(--success);
	}

	.tag.deleted {
		color: var(--danger);
	}

	.tag.renamed {
		color: var(--accent);
	}

	.tag.untracked {
		color: var(--warning);
	}

	.tag.binary {
		color: var(--muted);
	}

	.diff-view {
		background: var(--panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		overflow: hidden;
	}

	.file-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		font-size: 13px;
		color: var(--muted);
		height: 24px;
	}

	.file-title {
		display: flex;
		gap: 8px;
		align-items: center;
		color: var(--text);
		font-weight: 500;
	}

	.diff-renderer {
		flex: 1;
		min-height: 0;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: var(--panel-soft);
		padding: 8px;
		overflow: hidden;
		--diffs-dark-bg: var(--panel-soft);
		--diffs-dark: var(--text);
		--diffs-dark-addition-color: var(--success);
		--diffs-dark-deletion-color: var(--danger);
		--diffs-dark-modified-color: var(--accent);
		--diffs-font-family: var(--font-mono);
		--diffs-font-size: 12px;
		--diffs-header-font-family: var(--font-body);
		--diffs-gap-block: 8px;
		--diffs-gap-inline: 10px;
	}

	diffs-container {
		display: block;
		height: 100%;
		width: 100%;
		overflow: auto;
	}

	.ghost {
		background: rgba(255, 255, 255, 0.02);
		border: 1px solid var(--border);
		color: var(--text);
		padding: 8px 12px;
		border-radius: var(--radius-md);
		cursor: pointer;
		font-size: 12px;
		transition:
			border-color var(--transition-fast),
			background var(--transition-fast);
	}

	.ghost:hover:not(:disabled) {
		border-color: var(--accent);
		background: rgba(255, 255, 255, 0.04);
	}

	.ghost:active:not(:disabled) {
		transform: scale(0.98);
	}

	.link {
		color: var(--accent);
		text-decoration: none;
	}

	.link:hover {
		text-decoration: underline;
	}

	/* Inline Review Annotations via @pierre/diffs renderAnnotation callback */
	:global(.diff-annotation-thread) {
		margin: 8px 0;
		max-width: 720px;
		border-radius: 10px;
		overflow: hidden;
		border: 1px solid rgba(99, 102, 241, 0.25);
		border-left: 3px solid #6366f1;
		animation: fadeSlideIn 0.2s ease;
	}

	:global(.diff-annotation) {
		padding: 12px 14px;
		background: linear-gradient(135deg, rgba(99, 102, 241, 0.08) 0%, rgba(139, 92, 246, 0.08) 100%);
	}

	:global(.diff-annotation-reply) {
		background: linear-gradient(135deg, rgba(99, 102, 241, 0.04) 0%, rgba(139, 92, 246, 0.04) 100%);
		border-top: 1px solid rgba(99, 102, 241, 0.15);
		padding-left: 28px;
		position: relative;
	}

	:global(.diff-annotation-reply::before) {
		content: '';
		position: absolute;
		left: 14px;
		top: 12px;
		bottom: 12px;
		width: 2px;
		background: rgba(99, 102, 241, 0.3);
		border-radius: 1px;
	}

	@keyframes fadeSlideIn {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	:global(.diff-annotation-header) {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 8px;
	}

	:global(.diff-annotation-avatar) {
		width: 24px;
		height: 24px;
		border-radius: 50%;
		background: linear-gradient(135deg, #6366f1 0%, #a78bfa 100%);
		color: white;
		font-size: 11px;
		font-weight: 600;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	:global(.diff-annotation-reply .diff-annotation-avatar) {
		width: 20px;
		height: 20px;
		font-size: 10px;
	}

	:global(.diff-annotation-author) {
		font-size: 12px;
		font-weight: 600;
		color: var(--text);
	}

	:global(.diff-annotation-body) {
		font-size: 13px;
		line-height: 1.5;
		color: var(--text);
		white-space: pre-wrap;
		word-break: break-word;
	}

	:global(.diff-annotation-reply .diff-annotation-body) {
		font-size: 12px;
	}

	/* Comment action buttons */
	:global(.diff-annotation-actions) {
		display: flex;
		gap: 4px;
		margin-left: auto;
		opacity: 0;
		transition: opacity 0.15s ease;
	}

	:global(.diff-annotation:hover .diff-annotation-actions) {
		opacity: 1;
	}

	:global(.diff-action-btn) {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 4px;
		padding: 4px 8px;
		border: none;
		border-radius: 6px;
		background: rgba(255, 255, 255, 0.08);
		color: var(--muted);
		font-size: 11px;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	:global(.diff-action-btn:hover) {
		background: rgba(255, 255, 255, 0.15);
		color: var(--text);
	}

	:global(.diff-action-delete:hover) {
		background: rgba(248, 81, 73, 0.2);
		color: #f85149;
	}

	:global(.diff-annotation-footer) {
		display: flex;
		gap: 8px;
		padding: 10px 14px;
		background: rgba(99, 102, 241, 0.04);
		border-top: 1px solid rgba(99, 102, 241, 0.15);
	}

	:global(.diff-action-reply) {
		color: #6366f1;
	}

	:global(.diff-action-reply:hover) {
		background: rgba(99, 102, 241, 0.15);
		color: #818cf8;
	}

	:global(.diff-action-resolve) {
		color: #3fb950;
	}

	:global(.diff-action-resolve:hover) {
		background: rgba(46, 160, 67, 0.15);
		color: #4ade80;
	}

	/* Inline comment form */
	:global(.diff-annotation-inline-form) {
		padding: 12px 14px;
		background: rgba(99, 102, 241, 0.06);
		border-top: 1px solid rgba(99, 102, 241, 0.15);
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	:global(.diff-inline-textarea) {
		width: 100%;
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--panel);
		color: var(--text);
		font-family: inherit;
		font-size: 13px;
		line-height: 1.5;
		resize: vertical;
		min-height: 80px;
	}

	:global(.diff-inline-textarea:focus) {
		outline: none;
		border-color: var(--accent);
	}

	:global(.diff-inline-textarea:disabled) {
		opacity: 0.6;
		cursor: not-allowed;
	}

	:global(.diff-inline-form-actions) {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}

	:global(.diff-inline-form-actions .btn-ghost) {
		padding: 6px 12px;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		font-size: 12px;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	:global(.diff-inline-form-actions .btn-ghost:hover:not(:disabled)) {
		background: rgba(255, 255, 255, 0.05);
		color: var(--text);
	}

	:global(.diff-inline-form-actions .btn-primary) {
		padding: 6px 14px;
		border: none;
		border-radius: 6px;
		background: var(--accent);
		color: white;
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	:global(.diff-inline-form-actions .btn-primary:hover:not(:disabled)) {
		opacity: 0.9;
	}

	:global(.diff-inline-form-actions .btn-primary:disabled),
	:global(.diff-inline-form-actions .btn-ghost:disabled) {
		opacity: 0.5;
		cursor: not-allowed;
	}

	/* Resolved thread styles */
	:global(.diff-annotation-thread.diff-annotation-resolved) {
		border-color: rgba(46, 160, 67, 0.25);
		border-left-color: #3fb950;
	}

	:global(.diff-annotation-thread.diff-annotation-resolved .diff-annotation) {
		background: linear-gradient(135deg, rgba(46, 160, 67, 0.06) 0%, rgba(46, 160, 67, 0.04) 100%);
	}

	:global(.diff-annotation-thread.diff-annotation-resolved .diff-annotation-reply) {
		background: linear-gradient(135deg, rgba(46, 160, 67, 0.03) 0%, rgba(46, 160, 67, 0.02) 100%);
		border-top-color: rgba(46, 160, 67, 0.15);
	}

	:global(.diff-annotation-thread.diff-annotation-resolved .diff-annotation-footer) {
		background: rgba(46, 160, 67, 0.04);
		border-top-color: rgba(46, 160, 67, 0.15);
	}

	/* Collapsed header for resolved threads */
	:global(.diff-annotation-collapsed-header) {
		display: none;
		align-items: center;
		gap: 10px;
		padding: 10px 14px;
		background: linear-gradient(135deg, rgba(46, 160, 67, 0.08) 0%, rgba(46, 160, 67, 0.05) 100%);
		cursor: pointer;
		transition: background 0.15s ease;
	}

	:global(.diff-annotation-collapsed-header:hover) {
		background: linear-gradient(135deg, rgba(46, 160, 67, 0.12) 0%, rgba(46, 160, 67, 0.08) 100%);
	}

	:global(.diff-annotation-collapsed .diff-annotation-collapsed-header) {
		display: flex;
	}

	:global(.diff-annotation-collapsed .diff-annotation-content),
	:global(.diff-annotation-collapsed .diff-annotation-footer) {
		display: none;
	}

	:global(.diff-annotation-collapsed-icon) {
		font-size: 10px;
		color: #3fb950;
		transition: transform 0.15s ease;
	}

	:global(.diff-annotation-thread:not(.diff-annotation-collapsed) .diff-annotation-collapsed-icon) {
		transform: rotate(90deg);
	}

	:global(.diff-annotation-collapsed-badge) {
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		padding: 2px 8px;
		border-radius: 999px;
		background: rgba(46, 160, 67, 0.2);
		color: #3fb950;
	}

	:global(.diff-annotation-collapsed-preview) {
		flex: 1;
		font-size: 12px;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	:global(.diff-annotation-collapsed-count) {
		font-size: 11px;
		color: var(--muted);
		opacity: 0.7;
	}

	/* Unresolve button style */
	:global(.diff-action-unresolve) {
		color: #d29922;
	}

	:global(.diff-action-unresolve:hover) {
		background: rgba(210, 153, 34, 0.15);
		color: #f0b429;
	}

	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(6, 9, 14, 0.78);
		display: grid;
		place-items: center;
		z-index: 30;
		padding: 24px;
		animation: overlayFadeIn var(--transition-normal) ease-out;
	}

	.overlay-panel {
		width: 100%;
		display: flex;
		justify-content: center;
		animation: modalSlideIn 200ms ease-out;
	}

	@keyframes overlayFadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	@keyframes modalSlideIn {
		from {
			opacity: 0;
			transform: translateY(-8px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

	@media (max-width: 720px) {
		.overlay {
			padding: 0;
		}
	}
</style>
