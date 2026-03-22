import { EditorView, gutter, GutterMarker, Decoration, type DecorationSet } from '@codemirror/view';
import { StateField, StateEffect, RangeSetBuilder } from '@codemirror/state';
import type { Extension } from '@codemirror/state';

/**
 * Blame data for a range of lines from the same commit.
 */
export interface BlameEntry {
	startLine: number;
	endLine: number;
	commitHash: string;
	author: string;
	authorDate: string;
	summary: string;
}

// ── State Effect & Field ─────────────────────────────────

export const setBlameData = StateEffect.define<BlameEntry[]>();

const blameDataField = StateField.define<BlameEntry[]>({
	create: () => [],
	update(value, tr) {
		for (const effect of tr.effects) {
			if (effect.is(setBlameData)) return effect.value;
		}
		return value;
	},
});

// ── Gutter Marker ────────────────────────────────────────

function relativeDate(iso: string): string {
	const now = Date.now();
	const then = new Date(iso).getTime();
	if (isNaN(then)) return '';
	const diffMs = now - then;
	const days = Math.floor(diffMs / 86_400_000);
	if (days < 1) return 'today';
	if (days === 1) return '1d ago';
	if (days < 30) return `${days}d ago`;
	const months = Math.floor(days / 30);
	if (months < 12) return `${months}mo ago`;
	const years = Math.floor(months / 12);
	return `${years}y ago`;
}

class BlameGutterMarker extends GutterMarker {
	constructor(
		readonly entry: BlameEntry,
		readonly isFirst: boolean,
	) {
		super();
	}

	toDOM(): HTMLElement {
		const el = document.createElement('div');
		el.className = 'cm-blame-gutter-line';
		if (this.isFirst) {
			const author = document.createElement('span');
			author.className = 'cm-blame-author';
			author.textContent = this.entry.author;
			el.appendChild(author);
			const date = document.createElement('span');
			date.className = 'cm-blame-date';
			date.textContent = relativeDate(this.entry.authorDate);
			el.appendChild(date);
		}
		el.title = `${this.entry.commitHash.slice(0, 8)} — ${this.entry.summary}`;
		return el;
	}
}

// ── Gutter Extension ─────────────────────────────────────

const blameGutterExt = gutter({
	class: 'cm-blame-gutter',
	lineMarker(view, line) {
		const entries = view.state.field(blameDataField);
		if (entries.length === 0) return null;
		const lineNo = view.state.doc.lineAt(line.from).number;
		for (const entry of entries) {
			if (lineNo >= entry.startLine && lineNo <= entry.endLine) {
				return new BlameGutterMarker(entry, lineNo === entry.startLine);
			}
		}
		return null;
	},
	lineMarkerChange(update) {
		return update.transactions.some((t) => t.effects.some((e) => e.is(setBlameData)));
	},
});

// ── Line Decorations (alternating bands) ──────────────────

const blameBandA = Decoration.line({ class: 'cm-blame-band-a' });
const blameBandB = Decoration.line({ class: 'cm-blame-band-b' });

const blameLineDecorations = StateField.define<DecorationSet>({
	create: () => Decoration.none,
	update(_, tr) {
		const entries = tr.state.field(blameDataField);
		if (entries.length === 0) return Decoration.none;
		const builder = new RangeSetBuilder<Decoration>();
		let bandIndex = 0;
		for (const entry of entries) {
			const deco = bandIndex % 2 === 0 ? blameBandA : blameBandB;
			for (let line = entry.startLine; line <= entry.endLine; line++) {
				if (line > tr.state.doc.lines) break;
				const lineObj = tr.state.doc.line(line);
				builder.add(lineObj.from, lineObj.from, deco);
			}
			bandIndex++;
		}
		return builder.finish();
	},
	provide: (f) => EditorView.decorations.from(f),
});

// ── Theme ────────────────────────────────────────────────

const blameTheme = EditorView.theme({
	'.cm-blame-gutter': {
		width: '180px',
		fontSize: '11px',
		fontFamily: 'var(--font-mono, monospace)',
		color: 'var(--muted, #6b7280)',
		borderRight: '1px solid var(--border, #2d333b)',
	},
	'.cm-blame-gutter-line': {
		display: 'flex',
		gap: '6px',
		paddingRight: '8px',
		overflow: 'hidden',
		whiteSpace: 'nowrap',
	},
	'.cm-blame-author': {
		flex: '1',
		minWidth: '0',
		overflow: 'hidden',
		textOverflow: 'ellipsis',
	},
	'.cm-blame-date': {
		flexShrink: '0',
		opacity: '0.7',
	},
	'.cm-blame-band-a': {
		background: 'color-mix(in srgb, var(--panel, #161b22) 50%, transparent)',
	},
	'.cm-blame-band-b': {
		background: 'transparent',
	},
});

// ── Public Bundle ────────────────────────────────────────

/**
 * Self-contained blame extension bundle.
 * Include in CodeEditor's `extensions` prop, then dispatch
 * `setBlameData` effect with blame entries.
 */
export function blameExtension(): Extension {
	return [blameDataField, blameGutterExt, blameLineDecorations, blameTheme];
}
