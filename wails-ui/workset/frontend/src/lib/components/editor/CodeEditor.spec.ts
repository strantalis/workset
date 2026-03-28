import { cleanup, render } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import CodeEditor from './CodeEditor.svelte';
import type { Extension } from '@codemirror/state';

const createDoc = (text: string) => ({
	length: text.length,
	toString: () => text,
});

const codeMirrorHarness = vi.hoisted(() => ({
	instances: [] as Array<{
		dispatch: ReturnType<typeof vi.fn>;
		destroy: ReturnType<typeof vi.fn>;
		state: { doc: ReturnType<typeof createDoc> };
	}>,
}));

const languageSupportMocks = vi.hoisted(() => ({
	loadLanguage: vi.fn(async () => null),
}));

vi.mock('@codemirror/view', () => {
	class MockEditorView {
		static lineWrapping = Symbol('lineWrapping');
		static updateListener = {
			of: (value: unknown) => value,
		};
		static theme = (value: unknown) => value;

		readonly dom = document.createElement('div');
		readonly dispatch: ReturnType<typeof vi.fn>;
		readonly destroy: ReturnType<typeof vi.fn>;
		state: { doc: ReturnType<typeof createDoc> };

		constructor({
			state,
			parent,
		}: {
			state: { doc: ReturnType<typeof createDoc> };
			parent: HTMLElement;
		}) {
			this.state = state;
			this.dispatch = vi.fn((transaction: { changes?: { insert: string } }) => {
				if (transaction.changes) {
					this.state.doc = createDoc(transaction.changes.insert);
				}
			});
			this.destroy = vi.fn(() => {
				this.dom.remove();
			});
			parent.appendChild(this.dom);
			codeMirrorHarness.instances.push(this);
		}
	}

	return {
		EditorView: MockEditorView,
		keymap: { of: (value: unknown) => value },
		lineNumbers: () => ({ type: 'lineNumbers' }),
		highlightActiveLine: () => ({ type: 'highlightActiveLine' }),
	};
});

vi.mock('@codemirror/state', () => ({
	EditorState: {
		create: ({ doc, extensions }: { doc: string; extensions: unknown[] }) => ({
			doc: createDoc(doc),
			extensions,
		}),
		readOnly: {
			of: (value: boolean) => value,
		},
	},
}));

vi.mock('@codemirror/commands', () => ({
	defaultKeymap: [],
	history: () => ({ type: 'history' }),
	historyKeymap: [],
}));

vi.mock('@codemirror/search', () => ({
	searchKeymap: [],
	highlightSelectionMatches: () => ({ type: 'highlightSelectionMatches' }),
}));

vi.mock('@codemirror/language', () => ({
	bracketMatching: () => ({ type: 'bracketMatching' }),
	foldGutter: () => ({ type: 'foldGutter' }),
	foldKeymap: [],
}));

vi.mock('./editorTheme', () => ({
	worksetExtensions: [],
}));

vi.mock('./languageSupport', () => languageSupportMocks);

describe('CodeEditor', () => {
	beforeEach(() => {
		codeMirrorHarness.instances.length = 0;
		vi.clearAllMocks();
		languageSupportMocks.loadLanguage.mockResolvedValue(null);
	});

	afterEach(() => {
		cleanup();
	});

	test('updates the document in place when saved content changes for the same file', async () => {
		const onViewReady = vi.fn();
		const extensions: Extension[] = [];
		const { rerender } = render(CodeEditor, {
			props: {
				content: 'first line',
				filePath: 'src/main.ts',
				readOnly: false,
				extensions,
				onViewReady,
			},
		});

		await Promise.resolve();
		await Promise.resolve();

		expect(languageSupportMocks.loadLanguage).toHaveBeenCalledWith('src/main.ts');
		expect(codeMirrorHarness.instances).toHaveLength(1);
		expect(onViewReady).toHaveBeenCalledTimes(1);

		const [instance] = codeMirrorHarness.instances;

		await rerender({
			content: 'saved line',
			filePath: 'src/main.ts',
			readOnly: false,
			extensions,
			onViewReady,
		});

		await Promise.resolve();

		expect(codeMirrorHarness.instances).toHaveLength(1);
		expect(onViewReady).toHaveBeenCalledTimes(1);
		expect(instance.destroy).not.toHaveBeenCalled();
		expect(instance.dispatch).toHaveBeenCalledWith({
			changes: { from: 0, to: 'first line'.length, insert: 'saved line' },
		});
		expect(instance.state.doc.toString()).toBe('saved line');
	});
});
