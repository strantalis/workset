<style>
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
		font-size: var(--text-xs);
		font-weight: 600;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	:global(.diff-annotation-reply .diff-annotation-avatar) {
		width: 20px;
		height: 20px;
		font-size: var(--text-xs);
	}

	:global(.diff-annotation-author) {
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--text);
	}

	:global(.diff-annotation-body) {
		font-size: var(--text-base);
		line-height: 1.5;
		color: var(--text);
		white-space: pre-wrap;
		word-break: break-word;
	}

	:global(.diff-annotation-reply .diff-annotation-body) {
		font-size: var(--text-sm);
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
		font-size: var(--text-xs);
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
		font-size: var(--text-base);
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
		font-size: var(--text-sm);
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
		font-size: var(--text-sm);
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
		font-size: var(--text-xs);
		color: #3fb950;
		transition: transform 0.15s ease;
	}

	:global(.diff-annotation-thread:not(.diff-annotation-collapsed) .diff-annotation-collapsed-icon) {
		transform: rotate(90deg);
	}

	:global(.diff-annotation-collapsed-badge) {
		font-size: var(--text-xs);
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
		font-size: var(--text-sm);
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	:global(.diff-annotation-collapsed-count) {
		font-size: var(--text-xs);
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

	/* Line highlight animation for annotation navigation */
	:global(.highlight-line) {
		animation: line-highlight 2s ease-out;
	}

	:global(.highlight-line td) {
		background: rgba(210, 153, 34, 0.2) !important;
	}

	@keyframes line-highlight {
		0% {
			background: rgba(210, 153, 34, 0.4);
		}
		100% {
			background: transparent;
		}
	}
</style>
