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

	const buildExtensions = (): Extension[] => [
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

	// Load language + create view when deps change
	$effect(() => {
		const el = container;
		const doc = content;
		const _ro = readOnly;
		const _ext = extraExtensions;
		void _ro;
		void _ext;
		if (!el) return;

		// Load language if path changed
		const path = filePath;
		if (path !== currentFilePath) {
			currentFilePath = path;
			if (path) {
				void loadLanguage(path).then((ext) => {
					if (filePath !== path) return;
					langExt = ext;
					createView(el, doc);
				});
			} else {
				langExt = null;
			}
		}

		createView(el, doc);

		return () => {
			if (view) {
				view.destroy();
				view = null;
			}
		};
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
