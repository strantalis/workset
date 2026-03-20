import {
	EditorView,
	Decoration,
	WidgetType,
	type DecorationSet,
	gutter,
	GutterMarker,
} from '@codemirror/view';
import { StateField, StateEffect, RangeSetBuilder } from '@codemirror/state';

/**
 * A CI annotation from GitHub Actions or similar.
 */
export interface CIAnnotation {
	line: number;
	message: string;
	severity: 'error' | 'warning' | 'notice';
	title?: string;
	path?: string;
}

// ── Gutter Marker ─────────────────────────────────────────

class CIGutterMarker extends GutterMarker {
	constructor(readonly severity: CIAnnotation['severity']) {
		super();
	}

	toDOM(): HTMLElement {
		const el = document.createElement('span');
		el.className = `cm-ci-gutter-${this.severity}`;
		el.textContent = this.severity === 'error' ? '✕' : this.severity === 'warning' ? '⚠' : 'ℹ';
		return el;
	}
}

// ── Tooltip Widget ────────────────────────────────────────

class CITooltipWidget extends WidgetType {
	constructor(readonly annotations: CIAnnotation[]) {
		super();
	}

	toDOM(): HTMLElement {
		const wrap = document.createElement('div');
		wrap.className = 'cm-ci-annotation-block';

		for (const ann of this.annotations) {
			const row = document.createElement('div');
			row.className = `cm-ci-annotation cm-ci-${ann.severity}`;

			const icon = document.createElement('span');
			icon.className = 'cm-ci-icon';
			icon.textContent = ann.severity === 'error' ? '✕' : ann.severity === 'warning' ? '⚠' : 'ℹ';
			row.appendChild(icon);

			const msg = document.createElement('span');
			msg.className = 'cm-ci-message';
			msg.textContent = ann.title ? `${ann.title}: ${ann.message}` : ann.message;
			row.appendChild(msg);

			wrap.appendChild(row);
		}

		return wrap;
	}

	eq(other: CITooltipWidget): boolean {
		if (this.annotations.length !== other.annotations.length) return false;
		return this.annotations.every(
			(a, i) => a.line === other.annotations[i].line && a.message === other.annotations[i].message,
		);
	}
}

// ── State Effects ─────────────────────────────────────────

export const setCIAnnotations = StateEffect.define<CIAnnotation[]>();

// ── State Field (line decorations) ────────────────────────

export const ciAnnotationsField = StateField.define<DecorationSet>({
	create() {
		return Decoration.none;
	},
	update(decorations, tr) {
		for (const effect of tr.effects) {
			if (effect.is(setCIAnnotations)) {
				return buildDecorations(tr.state.doc, effect.value);
			}
		}
		return decorations.map(tr.changes);
	},
	provide: (field) => EditorView.decorations.from(field),
});

function buildDecorations(
	doc: { lineAt(pos: number): { from: number; to: number }; lines: number },
	annotations: CIAnnotation[],
): DecorationSet {
	const builder = new RangeSetBuilder<Decoration>();

	const byLine = new Map<number, CIAnnotation[]>();
	for (const a of annotations) {
		if (a.line < 1 || a.line > doc.lines) continue;
		const existing = byLine.get(a.line) ?? [];
		existing.push(a);
		byLine.set(a.line, existing);
	}

	const sortedLines = [...byLine.keys()].sort((a, b) => a - b);
	for (const lineNum of sortedLines) {
		const lineAnnotations = byLine.get(lineNum)!;
		const lineInfo = doc.lineAt(lineNum);
		// Determine worst severity for line highlight
		const worstSeverity = lineAnnotations.some((a) => a.severity === 'error')
			? 'error'
			: lineAnnotations.some((a) => a.severity === 'warning')
				? 'warning'
				: 'notice';
		builder.add(
			lineInfo.from,
			lineInfo.from,
			Decoration.line({ class: `cm-ci-line-${worstSeverity}` }),
		);
		// Add annotation widget after line
		builder.add(
			lineInfo.to,
			lineInfo.to,
			Decoration.widget({
				widget: new CITooltipWidget(lineAnnotations),
				side: 1,
				block: true,
			}),
		);
	}

	return builder.finish();
}

// ── Gutter ────────────────────────────────────────────────

export const ciGutterField = StateField.define<Map<number, CIAnnotation['severity']>>({
	create() {
		return new Map();
	},
	update(value, tr) {
		for (const effect of tr.effects) {
			if (effect.is(setCIAnnotations)) {
				const map = new Map<number, CIAnnotation['severity']>();
				for (const a of effect.value) {
					const existing = map.get(a.line);
					if (!existing || severityWeight(a.severity) > severityWeight(existing)) {
						map.set(a.line, a.severity);
					}
				}
				return map;
			}
		}
		return value;
	},
});

const severityWeight = (s: CIAnnotation['severity']): number =>
	s === 'error' ? 3 : s === 'warning' ? 2 : 1;

export const ciGutter = gutter({
	class: 'cm-ci-gutter',
	markers: (view) => {
		const map = view.state.field(ciGutterField, false);
		if (!map) return [];
		const builder = new RangeSetBuilder<GutterMarker>();
		const sortedLines = [...map.entries()].sort(([a], [b]) => a - b);
		for (const [lineNum, severity] of sortedLines) {
			if (lineNum >= 1 && lineNum <= view.state.doc.lines) {
				const lineInfo = view.state.doc.line(lineNum);
				builder.add(lineInfo.from, lineInfo.from, new CIGutterMarker(severity));
			}
		}
		return builder.finish();
	},
});

// ── Theme ─────────────────────────────────────────────────

export const ciAnnotationsTheme = EditorView.baseTheme({
	'.cm-ci-gutter': {
		width: '16px',
	},
	'.cm-ci-gutter-error': {
		color: '#ef4444',
		fontWeight: 'bold',
		fontSize: '11px',
		textAlign: 'center',
	},
	'.cm-ci-gutter-warning': {
		color: '#f59e0b',
		fontSize: '11px',
		textAlign: 'center',
	},
	'.cm-ci-gutter-notice': {
		color: '#2d8cff',
		fontSize: '11px',
		textAlign: 'center',
	},
	'.cm-ci-line-error': {
		backgroundColor: 'color-mix(in srgb, #ef4444 8%, transparent)',
	},
	'.cm-ci-line-warning': {
		backgroundColor: 'color-mix(in srgb, #f59e0b 6%, transparent)',
	},
	'.cm-ci-line-notice': {
		backgroundColor: 'color-mix(in srgb, #2d8cff 6%, transparent)',
	},
	'.cm-ci-annotation-block': {
		padding: '2px 12px 2px 24px',
	},
	'.cm-ci-annotation': {
		display: 'flex',
		alignItems: 'flex-start',
		gap: '6px',
		padding: '4px 8px',
		fontSize: '11px',
		lineHeight: '1.4',
		borderRadius: '4px',
		marginBottom: '2px',
	},
	'.cm-ci-error': {
		backgroundColor: 'color-mix(in srgb, #ef4444 10%, #15202f)',
		border: '1px solid color-mix(in srgb, #ef4444 25%, transparent)',
		color: '#f2f6fb',
	},
	'.cm-ci-warning': {
		backgroundColor: 'color-mix(in srgb, #f59e0b 8%, #15202f)',
		border: '1px solid color-mix(in srgb, #f59e0b 20%, transparent)',
		color: '#f2f6fb',
	},
	'.cm-ci-notice': {
		backgroundColor: 'color-mix(in srgb, #2d8cff 8%, #15202f)',
		border: '1px solid color-mix(in srgb, #2d8cff 20%, transparent)',
		color: '#f2f6fb',
	},
	'.cm-ci-icon': {
		flexShrink: '0',
		fontWeight: 'bold',
	},
	'.cm-ci-message': {
		whiteSpace: 'pre-wrap',
	},
});
