<script lang="ts">
	import { EditorView, keymap, lineNumbers, highlightActiveLine } from '@codemirror/view';
	import { EditorState, type Extension } from '@codemirror/state';
	import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
	import { searchKeymap, highlightSelectionMatches } from '@codemirror/search';
	import { bracketMatching, foldGutter, foldKeymap } from '@codemirror/language';
	import { worksetExtensions } from './editorTheme';
	import { loadLanguage } from './languageSupport';

	interface Props {
		content: string;
		filePath?: string;
		readOnly?: boolean;
		extensions?: Extension[];
		onContentChange?: (content: string) => void;
		onViewReady?: (view: EditorView) => void;
	}

	const {
		content,
		filePath = '',
		readOnly = true,
		extensions: extraExtensions = [],
		onContentChange,
		onViewReady,
	}: Props = $props();

	let container: HTMLDivElement | null = $state(null);
	// Plain variables — NOT $state. CM views must not be proxied.
	let view: EditorView | null = null;
	let currentFilePath = '';
	let langExt: Extension | null = null;
	let languageRequestId = 0;

	const buildExtensions = (): Extension[] => [
		EditorView.lineWrapping,
		lineNumbers(),
		highlightActiveLine(),
		bracketMatching(),
		foldGutter(),
		highlightSelectionMatches(),
		history(),
		keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap, ...foldKeymap]),
		...worksetExtensions,
		...(langExt ? [langExt] : []),
		EditorState.readOnly.of(readOnly),
		...(readOnly
			? []
			: [
					EditorView.updateListener.of((update) => {
						if (update.docChanged) {
							onContentChange?.(update.state.doc.toString());
						}
					}),
				]),
		...extraExtensions,
	];

	const createView = (el: HTMLElement, doc: string): void => {
		if (view) {
			view.destroy();
			view = null;
		}
		const state = EditorState.create({ doc, extensions: buildExtensions() });
		view = new EditorView({ state, parent: el });
		onViewReady?.(view);
	};

	// Update doc content in-place when language hasn't changed
	const updateDoc = (doc: string): void => {
		if (!view) return;
		const currentDoc = view.state.doc.toString();
		if (currentDoc !== doc) {
			view.dispatch({
				changes: { from: 0, to: view.state.doc.length, insert: doc },
			});
		}
	};

	// Track whether extensions (readOnly, extraExtensions) changed.
	// Initialized to sentinel values so the first $effect run always creates the view.
	let prevReadOnly: boolean | null = null;
	let prevExtraExtensions: Extension[] | null = null;

	$effect(() => {
		const el = container;
		if (!el) return;

		return () => {
			languageRequestId += 1;
			if (view) {
				view.destroy();
				view = null;
			}
			currentFilePath = '';
			prevReadOnly = null;
			prevExtraExtensions = null;
		};
	});

	// Load language + create/update view when deps change
	$effect(() => {
		const el = container;
		const doc = content;
		const ro = readOnly;
		const ext = extraExtensions;
		if (!el) return;
		const requestId = ++languageRequestId;

		const path = filePath;
		const languageChanged = path !== currentFilePath;
		const extensionsChanged = ro !== prevReadOnly || ext !== prevExtraExtensions;

		if (languageChanged) {
			currentFilePath = path;
			prevReadOnly = ro;
			prevExtraExtensions = ext;
			if (path) {
				void loadLanguage(path).then((langResult) => {
					if (requestId !== languageRequestId || filePath !== path || container !== el) return;
					langExt = langResult;
					createView(el, doc);
				});
			} else {
				langExt = null;
				createView(el, doc);
			}
		} else if (extensionsChanged) {
			// readOnly or extra extensions changed — must recreate
			prevReadOnly = ro;
			prevExtraExtensions = ext;
			createView(el, doc);
		} else if (view) {
			// Same language, same extensions — update doc in place
			updateDoc(doc);
		} else {
			createView(el, doc);
		}
	});
</script>

<div class="code-editor" bind:this={container}></div>

<style>
	.code-editor {
		width: 100%;
		height: 100%;
		min-height: 0;
		overflow: hidden;
	}
	.code-editor :global(.cm-editor) {
		height: 100%;
	}
	.code-editor :global(.cm-scroller) {
		overflow: auto;
	}
</style>
