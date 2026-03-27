import type { RepoFileHoverRange, RepoFileHoverResult } from '../../types';
import { renderCodeDocument, renderMarkdownDocument } from '../../documentRender';
import type { Extension } from '@codemirror/state';
import { EditorView, hoverTooltip, tooltips } from '@codemirror/view';

export type SemanticHoverRequest = {
	filePath: string;
	content: string;
	line: number;
	character: number;
};

export type SemanticHoverOptions = {
	filePath: string;
	fetchHover: (request: SemanticHoverRequest) => Promise<RepoFileHoverResult>;
};

const supportedExtensions = new Set([
	'.ts',
	'.tsx',
	'.js',
	'.jsx',
	'.mjs',
	'.cjs',
	'.mts',
	'.cts',
	'.svelte',
	'.go',
	'.py',
	'.rs',
	'.tf',
	'.tfvars',
]);

const supportedSuffixes = ['.tfcomponent.hcl', '.tfdeploy.hcl', '.tfquery.hcl'] as const;

export function supportsSemanticHover(filePath: string): boolean {
	const normalized = filePath.toLowerCase();
	for (const suffix of supportedSuffixes) {
		if (normalized.endsWith(suffix)) {
			return true;
		}
	}
	for (const ext of supportedExtensions) {
		if (normalized.endsWith(ext)) {
			return true;
		}
	}
	return false;
}

export function positionToLspLineCharacter(
	view: EditorView,
	pos: number,
): {
	line: number;
	character: number;
} {
	const lineInfo = view.state.doc.lineAt(pos);
	return {
		line: lineInfo.number - 1,
		character: pos - lineInfo.from,
	};
}

export function lspRangeToOffsets(
	view: EditorView,
	range: RepoFileHoverRange | null | undefined,
	fallbackPos: number,
): { from: number; to: number } {
	if (!range) {
		const word = hoverWordRange(view, fallbackPos);
		return {
			from: word?.from ?? fallbackPos,
			to: word?.to ?? fallbackPos,
		};
	}
	const startLine = view.state.doc.line(
		Math.min(view.state.doc.lines, Math.max(1, range.startLine + 1)),
	);
	const endLine = view.state.doc.line(
		Math.min(view.state.doc.lines, Math.max(1, range.endLine + 1)),
	);
	return {
		from: startLine.from + Math.min(range.startCharacter, startLine.length),
		to: endLine.from + Math.min(range.endCharacter, endLine.length),
	};
}

export async function renderSemanticHoverHeader(result: RepoFileHoverResult): Promise<string> {
	if (!result.header) return '';

	const rendered = await renderCodeDocument(
		result.header,
		`hover.${semanticHoverCodeLanguage(result.language) ?? 'txt'}`,
		semanticHoverCodeLanguage(result.language),
	);
	return rendered.html;
}

export async function renderSemanticHoverDocumentation(
	result: RepoFileHoverResult,
): Promise<string> {
	const sections: string[] = [];
	if (result.documentation) {
		if (result.documentationKind === 'plaintext') {
			sections.push(`<p>${escapeHtml(result.documentation).replace(/\n/g, '<br />')}</p>`);
		} else {
			const rendered = await renderMarkdownDocument(result.documentation);
			sections.push(rendered.html);
		}
	}
	if (result.installHint) {
		sections.push(`<p class="cm-semantic-hover-hint">${escapeHtml(result.installHint)}</p>`);
	}
	return sections.join('');
}

export function semanticHoverAccent(language?: string | null): string {
	switch (language?.toLowerCase()) {
		case 'typescript':
			return 'typescript';
		case 'svelte':
			return 'svelte';
		case 'go':
			return 'go';
		case 'python':
			return 'python';
		case 'rust':
			return 'rust';
		case 'terraform':
			return 'terraform';
		default:
			return 'default';
	}
}

export function semanticHoverCodeLanguage(language?: string | null): string | null {
	switch (language?.toLowerCase()) {
		case 'typescript':
			return 'ts';
		case 'svelte':
			return 'svelte';
		case 'go':
			return 'go';
		case 'python':
			return 'python';
		case 'rust':
			return 'rust';
		case 'terraform':
			return 'terraform';
		default:
			return null;
	}
}

export function semanticHoverExtension(options: SemanticHoverOptions): Extension {
	if (!supportsSemanticHover(options.filePath)) {
		return [];
	}

	return [
		tooltips({ parent: document.body }),
		hoverTooltip(async (view, pos) => {
			const target = hoverWordRange(view, pos);
			if (!target) return null;

			let response: RepoFileHoverResult;
			try {
				response = await options.fetchHover({
					filePath: options.filePath,
					content: view.state.doc.toString(),
					...positionToLspLineCharacter(view, pos),
				});
			} catch {
				return null;
			}

			if (!response.supported) return null;
			if (!response.available) {
				return buildTooltip(target.from, target.to, response, true);
			}
			if (!response.found) return null;

			const offsets = lspRangeToOffsets(view, response.range, pos);
			return buildTooltip(offsets.from, offsets.to, response, false);
		}),
		semanticHoverTheme,
	];
}

async function buildTooltip(
	from: number,
	to: number,
	response: RepoFileHoverResult,
	unavailable: boolean,
) {
	const headerHtml = unavailable ? '' : await renderSemanticHoverHeader(response);
	const bodyHtml = unavailable
		? renderUnavailableHover(response)
		: await renderSemanticHoverDocumentation(response);

	return {
		pos: from,
		end: Math.max(from, to),
		create() {
			const dom = document.createElement('div');
			dom.className = unavailable
				? 'cm-semantic-hover cm-semantic-hover-unavailable'
				: 'cm-semantic-hover';
			dom.dataset.language = semanticHoverAccent(response.language);

			if (headerHtml) {
				const header = document.createElement('div');
				header.className = 'cm-semantic-hover-header';
				header.innerHTML = headerHtml;
				dom.append(header);
			}

			if (bodyHtml) {
				const body = document.createElement('div');
				body.className = 'cm-semantic-hover-body';
				body.innerHTML = bodyHtml;
				dom.append(body);
			}

			if (response.provider || response.language) {
				const footer = document.createElement('div');
				footer.className = 'cm-semantic-hover-footer';
				footer.textContent = [response.provider, response.language].filter(Boolean).join(' · ');
				dom.append(footer);
			}

			return { dom };
		},
	};
}

function renderUnavailableHover(response: RepoFileHoverResult): string {
	const parts: string[] = [];
	if (response.unavailableReason) {
		parts.push(`<p>${escapeHtml(response.unavailableReason)}</p>`);
	}
	if (response.installHint) {
		parts.push(`<p class="cm-semantic-hover-hint">${escapeHtml(response.installHint)}</p>`);
	}
	return parts.join('');
}

function hoverWordRange(view: EditorView, pos: number): { from: number; to: number } | null {
	return view.state.wordAt(pos) ?? view.state.wordAt(Math.max(0, pos - 1));
}

function escapeHtml(input: string): string {
	return input.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
}

const semanticHoverTheme = EditorView.baseTheme({
	'.cm-tooltip-hover': {
		overflow: 'visible',
	},
	'.cm-semantic-hover': {
		minWidth: 'min(18rem, 80vw)',
		maxWidth: 'min(40rem, calc(100vw - 2rem))',
		width: 'max-content',
		maxHeight: '24rem',
		overflowY: 'auto',
		padding: '0.35rem 0.45rem',
		fontSize: '0.82rem',
		lineHeight: '1.4',
		borderLeft:
			'3px solid var(--cm-semantic-hover-accent, color-mix(in srgb, var(--border-color) 70%, transparent))',
		background:
			'linear-gradient(135deg, color-mix(in srgb, var(--cm-semantic-hover-accent, var(--panel)) 9%, var(--panel)) 0%, var(--panel) 48%)',
		boxShadow: '0 4px 16px rgba(0, 0, 0, 0.28), 0 0 0 1px rgba(255, 255, 255, 0.05)',
	},
	'.cm-semantic-hover-header': {
		margin: '0 0 0.45rem',
		padding: '0 0 0.45rem',
		borderBottom:
			'1px solid color-mix(in srgb, var(--cm-semantic-hover-accent, var(--border-color)) 32%, transparent)',
		overflow: 'hidden',
	},
	'.cm-semantic-hover-header .shiki': {
		margin: 0,
		borderRadius: '0.4rem',
		padding: '0.45rem 0.55rem',
		background:
			'color-mix(in srgb, var(--cm-semantic-hover-accent, var(--panel-strong)) 12%, var(--panel-strong)) !important',
		whiteSpace: 'pre-wrap !important',
		overflowX: 'hidden',
	},
	'.cm-semantic-hover-header .shiki code': {
		display: 'block',
		whiteSpace: 'inherit',
	},
	'.cm-semantic-hover-header .shiki .line': {
		whiteSpace: 'pre-wrap',
		overflowWrap: 'anywhere',
	},
	'.cm-semantic-hover-body': {
		display: 'grid',
		gap: '0.35rem',
	},
	'.cm-semantic-hover-body .shiki': {
		margin: 0,
		borderRadius: '0.4rem',
		padding: '0.45rem 0.55rem',
		overflowX: 'auto',
	},
	'.cm-semantic-hover-body p': {
		margin: 0,
	},
	'.cm-semantic-hover-body pre': {
		margin: '0.2rem 0',
		padding: '0.35rem 0.45rem',
		borderRadius: '0.35rem',
		background: 'color-mix(in srgb, var(--panel-strong) 80%, transparent)',
		overflowX: 'auto',
	},
	'.cm-semantic-hover-footer': {
		marginTop: '0.45rem',
		paddingTop: '0.35rem',
		borderTop:
			'1px solid color-mix(in srgb, var(--cm-semantic-hover-accent, var(--border-color)) 22%, transparent)',
		fontSize: '0.72rem',
		color:
			'color-mix(in srgb, var(--cm-semantic-hover-accent, var(--text-muted)) 62%, var(--text-muted))',
		fontWeight: '600',
	},
	'.cm-semantic-hover-hint': {
		color: 'var(--text-muted)',
	},
	'.cm-semantic-hover-unavailable .cm-semantic-hover-header': {
		display: 'none',
	},
	'.cm-semantic-hover[data-language="default"]': {
		'--cm-semantic-hover-accent': 'var(--accent-primary, #6b7280)',
	},
	'.cm-semantic-hover[data-language="typescript"]': {
		'--cm-semantic-hover-accent': '#3178c6',
	},
	'.cm-semantic-hover[data-language="svelte"]': {
		'--cm-semantic-hover-accent': '#ff5d01',
	},
	'.cm-semantic-hover[data-language="go"]': {
		'--cm-semantic-hover-accent': '#00add8',
	},
	'.cm-semantic-hover[data-language="python"]': {
		'--cm-semantic-hover-accent': '#ffd43b',
	},
	'.cm-semantic-hover[data-language="rust"]': {
		'--cm-semantic-hover-accent': '#ce7e00',
	},
	'.cm-semantic-hover[data-language="terraform"]': {
		'--cm-semantic-hover-accent': '#7b42bc',
	},
});
