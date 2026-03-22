import { EditorView, ViewPlugin, type ViewUpdate } from '@codemirror/view';
import { StateEffect, StateField } from '@codemirror/state';
import type { Extension } from '@codemirror/state';

/**
 * State effect to mark the current document content as "clean" (saved).
 * Dispatch this after a successful save.
 */
export const setCleanDoc = StateEffect.define<string>();

/**
 * State field that tracks the last-saved document content.
 * When the current doc differs from this, the file is dirty.
 */
const cleanDocField = StateField.define<string>({
	create: (state) => state.doc.toString(),
	update(value, tr) {
		for (const effect of tr.effects) {
			if (effect.is(setCleanDoc)) return effect.value;
		}
		return value;
	},
});

/**
 * View plugin that toggles a `cm-dirty` CSS class on the editor root
 * when the document content differs from the clean snapshot.
 */
const dirtyClassPlugin = ViewPlugin.fromClass(
	class {
		private dirty = false;

		constructor(readonly view: EditorView) {
			this.sync();
		}

		update(update: ViewUpdate): void {
			if (
				update.docChanged ||
				update.transactions.some((t) => t.effects.some((e) => e.is(setCleanDoc)))
			) {
				this.sync();
			}
		}

		private sync(): void {
			const cleanDoc = this.view.state.field(cleanDocField);
			const currentDoc = this.view.state.doc.toString();
			const isDirty = currentDoc !== cleanDoc;
			if (isDirty !== this.dirty) {
				this.dirty = isDirty;
				this.view.dom.classList.toggle('cm-dirty', isDirty);
			}
		}
	},
);

/**
 * Bundle: tracks dirty state and toggles `cm-dirty` class on the editor.
 * Also includes a subtle border-top indicator when dirty.
 */
export function dirtyIndicator(): Extension {
	return [
		cleanDocField,
		dirtyClassPlugin,
		EditorView.theme({
			'&.cm-dirty': {
				borderTop: '2px solid var(--warning, #f59e0b)',
			},
		}),
	];
}
