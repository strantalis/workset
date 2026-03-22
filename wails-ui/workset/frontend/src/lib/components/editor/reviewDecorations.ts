import { EditorView, Decoration, WidgetType, type DecorationSet } from '@codemirror/view';
import { StateField, StateEffect } from '@codemirror/state';
import { RangeSetBuilder } from '@codemirror/state';
import { mount, unmount } from 'svelte';
import ReviewThread from './ReviewThread.svelte';

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
 * Widget that creates a mount point for a Svelte ReviewThread component.
 * CM6 handles positioning; Svelte handles rendering.
 */
class ReviewCommentWidget extends WidgetType {
	private mounted: Record<string, never> | null = null;

	constructor(readonly comments: ReviewComment[]) {
		super();
	}

	toDOM(): HTMLElement {
		const container = document.createElement('div');
		container.className = 'cm-review-thread';
		this.mounted = mount(ReviewThread, {
			target: container,
			props: { comments: this.comments },
		});
		return container;
	}

	destroy(): void {
		if (this.mounted) {
			try {
				unmount(this.mounted);
			} catch {
				// Component may already be detached
			}
			this.mounted = null;
		}
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

// ── CSS Theme (minimal — just the line highlight and thread container) ──

export const reviewDecorationsTheme = EditorView.baseTheme({
	'.cm-review-highlighted-line': {
		backgroundColor: 'color-mix(in srgb, #2d8cff 8%, transparent)',
		borderLeft: '2px solid #2d8cff',
	},
	'.cm-review-thread': {
		borderLeft: '2px solid #2d8cff',
		backgroundColor: 'color-mix(in srgb, #101925 90%, transparent)',
	},
});
