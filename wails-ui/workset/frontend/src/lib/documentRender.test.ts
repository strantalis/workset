import { describe, expect, test, vi } from 'vitest';

vi.mock('mermaid', () => ({
	default: {
		initialize: vi.fn(),
		parse: vi.fn(async (source: string) => {
			if (source.includes('participant ') && !source.includes('sequenceDiagram')) {
				throw new Error('invalid diagram');
			}
			return { diagramType: 'flowchart-v2' };
		}),
		render: vi.fn(async () => ({
			svg: '<svg><text>ok</text></svg>',
			bindFunctions: undefined,
		})),
	},
}));

import { renderCodeDocument, renderMarkdownDocument } from './documentRender';

describe('documentRender', () => {
	test('renders highlighted code documents with line wrappers', async () => {
		const rendered = await renderCodeDocument('const answer = 42;\n', 'src/example.ts');

		expect(rendered.containsMermaid).toBe(false);
		expect(rendered.html).toContain('class="shiki');
		expect(rendered.html).toContain('class="line"');
	});

	test('renders markdown code fences through the code renderer', async () => {
		const rendered = await renderMarkdownDocument('```ts\nconst answer = 42;\n```');

		expect(rendered.containsMermaid).toBe(false);
		expect(rendered.html).toContain('class="shiki');
	});

	test('renders mermaid blocks inline when parsing succeeds', async () => {
		const rendered = await renderMarkdownDocument(
			'```mermaid\nflowchart LR\n  A["One"] --> B["Two"]\n  B --> C["Three"]\n```',
		);

		expect(rendered.containsMermaid).toBe(true);
		expect(rendered.html).toContain('ws-mermaid-diagram');
		expect(rendered.html).toContain('<svg>');
		expect(rendered.html).not.toContain('flowchart LR');
		expect(rendered.html).not.toContain('A["One"] --> B["Two"]');
	});

	test('falls back to source when mermaid rendering fails', async () => {
		const rendered = await renderMarkdownDocument(
			'```mermaid\nparticipant UI as Browser/Wails\nparticipant A as worksetapi\n```',
		);

		expect(rendered.html).toContain('sequenceDiagram');
		expect(rendered.html).toContain('participant UI as Browser/Wails');
	});
});
