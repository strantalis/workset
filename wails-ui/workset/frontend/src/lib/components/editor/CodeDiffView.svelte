<script lang="ts">
	import { EditorView, keymap, lineNumbers } from '@codemirror/view';
	import { EditorState, type Extension, type StateEffect } from '@codemirror/state';
	import { defaultKeymap } from '@codemirror/commands';
	import { searchKeymap, highlightSelectionMatches } from '@codemirror/search';
	import { bracketMatching, foldGutter, foldKeymap } from '@codemirror/language';
	import { MergeView } from '@codemirror/merge';
	import { unifiedMergeView } from '@codemirror/merge';
	import { worksetExtensions } from './editorTheme';
	import { loadLanguage } from './languageSupport';
	import { parsePatch } from './patchParser';
	import {
		type ReviewComment,
		reviewDecorationsField,
		reviewDecorationsTheme,
		setReviewComments,
	} from './reviewDecorations';
	import {
		type CIAnnotation,
		ciAnnotationsField,
		ciAnnotationsTheme,
		ciGutterField,
		ciGutter,
		setCIAnnotations,
	} from './ciAnnotations';
	import { Loader2, FileCode } from '@lucide/svelte';

	interface Props {
		patch?: string | null;
		/** Full original file content (preferred over patch parsing). */
		originalContent?: string | null;
		/** Full modified file content (preferred over patch parsing). */
		modifiedContent?: string | null;
		filePath?: string;
		unified?: boolean;
		loading?: boolean;
		error?: string | null;
		binary?: boolean;
		truncated?: boolean;
		totalLines?: number;
		collapseUnchanged?: boolean;
		reviewComments?: ReviewComment[];
		ciAnnotations?: CIAnnotation[];
	}

	const {
		patch = null,
		originalContent = null,
		modifiedContent = null,
		filePath = '',
		unified = true,
		loading = false,
		error = null,
		binary = false,
		truncated = false,
		totalLines = 0,
		collapseUnchanged = true,
		reviewComments = [],
		ciAnnotations: ciAnns = [],
	}: Props = $props();

	const hasContent = $derived(
		(originalContent !== null && modifiedContent !== null) || patch !== null,
	);

	let container: HTMLDivElement | null = $state(null);
	// Plain variables — NOT $state. CM views must not be wrapped in Svelte proxies.
	let currentMergeView: MergeView | null = null;
	let currentUnifiedView: EditorView | null = null;
	let currentFilePath = '';
	let langExt: Extension | null = null;
	let epoch = 0;
	let collapseOverride = $state<boolean | null>(null);

	const effectiveCollapse = $derived(collapseOverride ?? collapseUnchanged);
	const collapseConfig = $derived(effectiveCollapse ? { margin: 3, minSize: 4 } : undefined);

	// Reset override when patch changes (new file)
	$effect(() => {
		void patch;
		collapseOverride = null;
	});

	const handleContainerClick = (event: MouseEvent): void => {
		const target = event.target;
		if (!(target instanceof Element)) return;
		if (target.closest('.cm-collapsedLines')) {
			collapseOverride = false;
		}
	};

	const baseExtensions = (): Extension[] => [
		lineNumbers(),
		bracketMatching(),
		foldGutter(),
		highlightSelectionMatches(),
		keymap.of([...defaultKeymap, ...searchKeymap, ...foldKeymap]),
		...worksetExtensions,
		...(langExt ? [langExt] : []),
		EditorState.readOnly.of(true),
		reviewDecorationsField,
		reviewDecorationsTheme,
		ciAnnotationsField,
		ciGutterField,
		ciGutter,
		ciAnnotationsTheme,
	];

	const destroyViews = (): void => {
		if (currentMergeView) {
			currentMergeView.destroy();
			currentMergeView = null;
		}
		if (currentUnifiedView) {
			currentUnifiedView.destroy();
			currentUnifiedView = null;
		}
	};

	// Create/recreate the diff view
	$effect(() => {
		const el = container;
		const origContent = originalContent;
		const modContent = modifiedContent;
		const currentPatch = patch;
		const isUnified = unified;
		const _collapse = collapseConfig;
		void _collapse;

		// Need either full contents or a patch
		const hasFullContent = origContent !== null && modContent !== null;
		if (!el || (!hasFullContent && !currentPatch)) {
			destroyViews();
			return;
		}

		const currentEpoch = ++epoch;

		// Load language if path changed
		const path = filePath;
		if (path !== currentFilePath) {
			currentFilePath = path;
			if (path) {
				void loadLanguage(path).then((ext) => {
					if (filePath !== path || epoch !== currentEpoch) return;
					langExt = ext;
				});
			} else {
				langExt = null;
			}
		}

		destroyViews();

		// Prefer full content, fall back to patch parsing
		const original = hasFullContent ? origContent! : parsePatch(currentPatch!).original;
		const modified = hasFullContent ? modContent! : parsePatch(currentPatch!).modified;

		if (currentEpoch !== epoch) return;

		if (isUnified) {
			const state = EditorState.create({
				doc: modified,
				extensions: [
					...baseExtensions(),
					unifiedMergeView({
						original,
						highlightChanges: true,
						gutter: true,
						syntaxHighlightDeletions: true,
						mergeControls: false,
						collapseUnchanged: collapseConfig,
					}),
				],
			});
			currentUnifiedView = new EditorView({ state, parent: el });
		} else {
			currentMergeView = new MergeView({
				a: { doc: original, extensions: baseExtensions() },
				b: { doc: modified, extensions: baseExtensions() },
				parent: el,
				highlightChanges: true,
				gutter: true,
				collapseUnchanged: collapseConfig,
			});
		}

		// Apply any pending annotations
		applyAnnotations();

		return () => {
			destroyViews();
		};
	});

	const applyAnnotations = (): void => {
		const targetView = currentUnifiedView ?? currentMergeView?.b;
		if (!targetView) return;
		const effects: StateEffect<unknown>[] = [];
		if (reviewComments.length > 0) effects.push(setReviewComments.of(reviewComments));
		if (ciAnns.length > 0) effects.push(setCIAnnotations.of(ciAnns));
		if (effects.length > 0) targetView.dispatch({ effects });
	};

	// Re-apply annotations when they change (without recreating the view)
	$effect(() => {
		const _comments = reviewComments;
		const _ci = ciAnns;
		void _comments;
		void _ci;
		applyAnnotations();
	});
</script>

{#if error}
	<div class="diff-placeholder">
		<FileCode size={24} />
		<p>{error}</p>
	</div>
{:else if binary}
	<div class="diff-placeholder">
		<FileCode size={24} />
		<p>Binary file</p>
	</div>
{:else if hasContent}
	<div class="diff-view-wrap">
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div class="diff-view" bind:this={container} onclick={handleContainerClick}></div>
		{#if loading}
			<div class="diff-loading-overlay">
				<Loader2 size={18} class="spin" />
				<p>Refreshing diff...</p>
			</div>
		{/if}
	</div>
	{#if truncated}
		<div class="diff-truncated">
			Diff truncated ({totalLines} total lines)
		</div>
	{/if}
{:else if loading}
	<div class="diff-placeholder">
		<Loader2 size={20} class="spin" />
		<p>Loading diff...</p>
	</div>
{:else}
	<div class="diff-placeholder">
		<FileCode size={24} />
		<p>No diff content</p>
	</div>
{/if}

<style>
	.diff-view-wrap {
		position: relative;
		width: 100%;
		height: 100%;
		min-height: 0;
		overflow: hidden;
		display: flex;
		flex-direction: column;
	}
	.diff-view {
		flex: 1;
		min-height: 0;
		width: 100%;
		height: 100%;
		overflow: hidden;
	}
	.diff-view :global(.cm-editor) {
		height: 100%;
	}
	.diff-view :global(.cm-scroller) {
		overflow: auto;
	}
	.diff-view :global(.cm-mergeView) {
		height: 100%;
	}
	.diff-view :global(.cm-mergeViewEditors) {
		height: 100%;
	}
	.diff-view :global(.cm-mergeViewEditor) {
		overflow: hidden;
	}
	.diff-view :global(.cm-collapsedLines) {
		cursor: pointer;
		transition: background 120ms ease;
	}
	.diff-view :global(.cm-collapsedLines:hover) {
		background: color-mix(in srgb, var(--accent) 12%, var(--panel-strong));
	}
	.diff-loading-overlay {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		background: color-mix(in srgb, var(--bg) 78%, transparent);
		color: var(--text);
		font-size: var(--text-sm);
		pointer-events: none;
	}
	.diff-truncated {
		padding: 12px;
		text-align: center;
		font-size: var(--text-xs);
		color: var(--yellow);
		background: color-mix(in srgb, var(--yellow) 5%, transparent);
		border-top: 1px solid var(--border);
	}
	.diff-placeholder {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 48px;
		color: var(--subtle);
		font-size: var(--text-base);
		text-align: center;
		flex: 1;
		height: 100%;
	}
</style>
