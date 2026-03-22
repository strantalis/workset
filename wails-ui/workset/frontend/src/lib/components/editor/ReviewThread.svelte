<script lang="ts">
	import DOMPurify from 'dompurify';
	import { marked } from 'marked';
	import type { ReviewComment } from './reviewDecorations';

	interface Props {
		comments: ReviewComment[];
	}

	const { comments }: Props = $props();

	const renderBody = (body: string): string => {
		try {
			return DOMPurify.sanitize(marked.parse(body, { gfm: true, breaks: true }) as string, {
				FORBID_TAGS: ['sub', 'sup'],
			});
		} catch {
			return body;
		}
	};

	const formatTime = (iso?: string): string => {
		if (!iso) return '';
		const d = new Date(iso);
		if (isNaN(d.getTime())) return iso;
		const now = Date.now();
		const diffMs = now - d.getTime();
		const days = Math.floor(diffMs / 86_400_000);
		if (days < 1) return 'today';
		if (days === 1) return 'yesterday';
		if (days < 30) return `${days}d ago`;
		const months = Math.floor(days / 30);
		if (months < 12) return `${months}mo ago`;
		return `${Math.floor(months / 12)}y ago`;
	};
</script>

<div class="review-thread">
	{#each comments as comment (comment.id)}
		<div class="review-comment" class:resolved={comment.resolved}>
			<div class="review-header">
				<span class="review-author">{comment.author || 'unknown'}</span>
				{#if comment.createdAt}
					<span class="review-time">{formatTime(comment.createdAt)}</span>
				{/if}
				{#if comment.resolved}
					<span class="review-resolved-badge">Resolved</span>
				{/if}
			</div>
			<div class="review-body">
				<!-- eslint-disable-next-line svelte/no-at-html-tags -->
				{@html renderBody(comment.body)}
			</div>
		</div>
	{/each}
</div>

<style>
	.review-thread {
		padding: 4px 12px 4px 24px;
		border-left: 2px solid #2d8cff;
		background: color-mix(in srgb, #101925 90%, transparent);
	}
	.review-comment {
		padding: 8px 12px;
		border-radius: 6px;
		background: #15202f;
		border: 1px solid #243244;
		margin-bottom: 4px;
	}
	.review-comment:last-child {
		margin-bottom: 0;
	}
	.review-comment.resolved {
		opacity: 0.5;
	}
	.review-header {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 4px;
		font-size: 11px;
	}
	.review-author {
		font-weight: 600;
		color: #f2f6fb;
	}
	.review-time {
		color: #8a9bb0;
		font-size: 10px;
	}
	.review-resolved-badge {
		padding: 1px 5px;
		border-radius: 4px;
		border: 1px solid color-mix(in srgb, #86c442 30%, transparent);
		background: color-mix(in srgb, #86c442 10%, transparent);
		color: #86c442;
		font-size: 9px;
	}

	/* ── Markdown body ──────────────────────────────── */
	.review-body {
		font-size: 12px;
		line-height: 1.6;
		color: #a3b5c9;
		word-break: break-word;
		overflow-wrap: anywhere;
	}
	.review-body :global(p) {
		margin: 0 0 6px;
	}
	.review-body :global(p:last-child) {
		margin-bottom: 0;
	}
	.review-body :global(strong) {
		color: #d0dae6;
	}
	.review-body :global(code) {
		padding: 1px 5px;
		border-radius: 4px;
		background: color-mix(in srgb, #2d8cff 12%, #0c1019);
		font-family: var(--font-mono, monospace);
		font-size: 11px;
		color: #c9d6e3;
	}
	.review-body :global(pre) {
		margin: 6px 0;
		padding: 8px 10px;
		border-radius: 6px;
		background: #0c1019;
		border: 1px solid #1e2a3a;
		overflow: auto;
	}
	.review-body :global(pre code) {
		padding: 0;
		background: transparent;
		font-size: 11px;
	}
	.review-body :global(a) {
		color: #2d8cff;
		text-decoration: none;
	}
	.review-body :global(a:hover) {
		text-decoration: underline;
	}
	.review-body :global(blockquote) {
		margin: 6px 0;
		padding-left: 10px;
		border-left: 2px solid #2a3545;
		color: #7a8da0;
	}
	.review-body :global(ul),
	.review-body :global(ol) {
		margin: 4px 0;
		padding-left: 18px;
	}
	.review-body :global(li) {
		margin-bottom: 2px;
	}
	.review-body :global(img) {
		display: inline !important;
		max-width: 100%;
		border-radius: 4px;
		vertical-align: middle;
	}
</style>
