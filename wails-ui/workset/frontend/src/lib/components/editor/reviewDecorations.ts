import { EditorView, Decoration, WidgetType, type DecorationSet } from '@codemirror/view';
import { StateField, StateEffect } from '@codemirror/state';
import { RangeSetBuilder } from '@codemirror/state';

/**
 * Review comment data for a single line annotation.
 */
export interface ReviewComment {
	id: number;
	author: string;
	body: string;
	line: number;
	path: string;
	createdAt?: string;
	resolved?: boolean;
	threadId?: string;
}

/**
 * Widget that renders an inline review comment block between lines.
 */
class ReviewCommentWidget extends WidgetType {
	constructor(readonly comments: ReviewComment[]) {
		super();
	}

	toDOM(): HTMLElement {
		const container = document.createElement('div');
		container.className = 'cm-review-thread';

		for (const comment of this.comments) {
			const block = document.createElement('div');
			block.className = `cm-review-comment${comment.resolved ? ' cm-review-resolved' : ''}`;

			const header = document.createElement('div');
			header.className = 'cm-review-header';

			const author = document.createElement('span');
			author.className = 'cm-review-author';
			author.textContent = comment.author || 'unknown';
			header.appendChild(author);

			if (comment.createdAt) {
				const time = document.createElement('span');
				time.className = 'cm-review-time';
				time.textContent = comment.createdAt;
				header.appendChild(time);
			}

			if (comment.resolved) {
				const badge = document.createElement('span');
				badge.className = 'cm-review-resolved-badge';
				badge.textContent = 'Resolved';
				header.appendChild(badge);
			}

			block.appendChild(header);

			const body = document.createElement('div');
			body.className = 'cm-review-body';
			body.textContent = comment.body;
			block.appendChild(body);

			container.appendChild(block);
		}

		return container;
	}

	eq(other: ReviewCommentWidget): boolean {
		if (this.comments.length !== other.comments.length) return false;
		return this.comments.every(
			(c, i) => c.id === other.comments[i].id && c.body === other.comments[i].body,
		);
	}

	ignoreEvent(): boolean {
		return false;
	}
}

// ── State Effects ─────────────────────────────────────────

export const setReviewComments = StateEffect.define<ReviewComment[]>();

// ── State Field ───────────────────────────────────────────

export const reviewDecorationsField = StateField.define<DecorationSet>({
	create() {
		return Decoration.none;
	},
	update(decorations, tr) {
		for (const effect of tr.effects) {
			if (effect.is(setReviewComments)) {
				return buildDecorations(tr.state.doc, effect.value);
			}
		}
		return decorations.map(tr.changes);
	},
	provide: (field) => EditorView.decorations.from(field),
});

function buildDecorations(
	doc: { lineAt(pos: number): { from: number; to: number }; lines: number },
	comments: ReviewComment[],
): DecorationSet {
	const builder = new RangeSetBuilder<Decoration>();

	// Group comments by line
	const byLine = new Map<number, ReviewComment[]>();
	for (const c of comments) {
		if (c.line == null || c.line < 1 || c.line > doc.lines) continue;
		const existing = byLine.get(c.line) ?? [];
		existing.push(c);
		byLine.set(c.line, existing);
	}

	// Sort by line number and add decorations
	const sortedLines = [...byLine.keys()].sort((a, b) => a - b);
	for (const lineNum of sortedLines) {
		const lineComments = byLine.get(lineNum)!;
		const lineInfo = doc.lineAt(lineNum);
		// Add line highlight
		builder.add(
			lineInfo.from,
			lineInfo.from,
			Decoration.line({ class: 'cm-review-highlighted-line' }),
		);
		// Add widget after the line
		builder.add(
			lineInfo.to,
			lineInfo.to,
			Decoration.widget({
				widget: new ReviewCommentWidget(lineComments),
				side: 1,
				block: true,
			}),
		);
	}

	return builder.finish();
}

// ── CSS Theme ─────────────────────────────────────────────

export const reviewDecorationsTheme = EditorView.baseTheme({
	'.cm-review-highlighted-line': {
		backgroundColor: 'color-mix(in srgb, #2d8cff 8%, transparent)',
		borderLeft: '2px solid #2d8cff',
	},
	'.cm-review-thread': {
		padding: '4px 12px 4px 24px',
		borderLeft: '2px solid #2d8cff',
		backgroundColor: 'color-mix(in srgb, #101925 90%, transparent)',
	},
	'.cm-review-comment': {
		padding: '8px 12px',
		borderRadius: '6px',
		backgroundColor: '#15202f',
		border: '1px solid #243244',
		marginBottom: '4px',
	},
	'.cm-review-comment.cm-review-resolved': {
		opacity: '0.5',
	},
	'.cm-review-header': {
		display: 'flex',
		alignItems: 'center',
		gap: '8px',
		marginBottom: '4px',
		fontSize: '11px',
	},
	'.cm-review-author': {
		fontWeight: '600',
		color: '#f2f6fb',
	},
	'.cm-review-time': {
		color: '#8a9bb0',
		fontSize: '10px',
	},
	'.cm-review-resolved-badge': {
		padding: '1px 5px',
		borderRadius: '4px',
		border: '1px solid color-mix(in srgb, #86c442 30%, transparent)',
		backgroundColor: 'color-mix(in srgb, #86c442 10%, transparent)',
		color: '#86c442',
		fontSize: '9px',
	},
	'.cm-review-body': {
		fontSize: '12px',
		lineHeight: '1.5',
		color: '#a3b5c9',
		whiteSpace: 'pre-wrap',
	},
});
