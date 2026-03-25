import { afterEach, describe, expect, it } from 'vitest';
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import {
	lspRangeToOffsets,
	positionToLspLineCharacter,
	renderSemanticHoverHeader,
	renderSemanticHoverDocumentation,
	semanticHoverAccent,
	semanticHoverCodeLanguage,
	supportsSemanticHover,
} from './semanticHover';

describe('semantic hover helpers', () => {
	let view: EditorView | null = null;

	afterEach(() => {
		view?.destroy();
		view = null;
	});

	it('recognizes supported top-language file types', () => {
		expect(supportsSemanticHover('src/main.ts')).toBe(true);
		expect(supportsSemanticHover('src/routes/+page.svelte')).toBe(true);
		expect(supportsSemanticHover('pkg/server.go')).toBe(true);
		expect(supportsSemanticHover('script.py')).toBe(true);
		expect(supportsSemanticHover('src/lib.rs')).toBe(true);
		expect(supportsSemanticHover('README.md')).toBe(false);
	});

	it('maps editor positions to lsp line and character coordinates', () => {
		view = new EditorView({
			state: EditorState.create({
				doc: 'first line\nsecond line',
			}),
			parent: document.body,
		});

		expect(positionToLspLineCharacter(view, 13)).toEqual({
			line: 1,
			character: 2,
		});
	});

	it('maps lsp ranges back to editor offsets', () => {
		view = new EditorView({
			state: EditorState.create({
				doc: 'alpha\nbravo\ncharlie',
			}),
			parent: document.body,
		});

		expect(
			lspRangeToOffsets(
				view,
				{
					startLine: 1,
					startCharacter: 1,
					endLine: 1,
					endCharacter: 4,
				},
				0,
			),
		).toEqual({
			from: 7,
			to: 10,
		});
	});

	it('renders plaintext documentation safely', async () => {
		const html = await renderSemanticHoverDocumentation({
			supported: true,
			available: true,
			found: true,
			documentation: '<unsafe>\nsecond line',
			documentationKind: 'plaintext',
		});

		expect(html).toContain('&lt;unsafe&gt;');
		expect(html).toContain('second line');
	});

	it('maps supported languages to stable accent tokens', () => {
		expect(semanticHoverAccent('typescript')).toBe('typescript');
		expect(semanticHoverAccent('svelte')).toBe('svelte');
		expect(semanticHoverAccent('go')).toBe('go');
		expect(semanticHoverAccent('python')).toBe('python');
		expect(semanticHoverAccent('rust')).toBe('rust');
		expect(semanticHoverAccent('unknown')).toBe('default');
		expect(semanticHoverAccent(null)).toBe('default');
	});

	it('maps supported languages to code highlighter languages', () => {
		expect(semanticHoverCodeLanguage('typescript')).toBe('ts');
		expect(semanticHoverCodeLanguage('svelte')).toBe('svelte');
		expect(semanticHoverCodeLanguage('go')).toBe('go');
		expect(semanticHoverCodeLanguage('python')).toBe('python');
		expect(semanticHoverCodeLanguage('rust')).toBe('rust');
		expect(semanticHoverCodeLanguage('unknown')).toBeNull();
	});

	it('renders syntax-highlighted hover headers', async () => {
		const html = await renderSemanticHoverHeader({
			supported: true,
			available: true,
			found: true,
			language: 'typescript',
			header: 'map<T, U>(value: T): U',
		});

		expect(html).toContain('class="shiki');
		expect(html).toContain('map');
	});
});
