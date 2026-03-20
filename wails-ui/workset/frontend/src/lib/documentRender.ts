import DOMPurify from 'dompurify';
import { marked } from 'marked';
import { codeToHtml } from 'shiki';

export type DocumentRenderResult = {
	html: string;
	containsMermaid: boolean;
};

const SHIKI_THEME = 'github-dark';

const PATH_LANGUAGE_MAP: Record<string, string> = {
	astro: 'astro',
	bash: 'bash',
	cjs: 'js',
	css: 'css',
	diff: 'diff',
	go: 'go',
	html: 'html',
	java: 'java',
	js: 'js',
	json: 'json',
	jsx: 'jsx',
	md: 'markdown',
	mdx: 'mdx',
	mjs: 'js',
	py: 'python',
	rs: 'rust',
	sh: 'bash',
	sql: 'sql',
	svelte: 'svelte',
	swift: 'swift',
	toml: 'toml',
	ts: 'ts',
	tsx: 'tsx',
	txt: 'text',
	xml: 'xml',
	yaml: 'yaml',
	yml: 'yaml',
	zsh: 'bash',
};

const MARKED_LANGUAGE_MAP: Record<string, string> = {
	bash: 'bash',
	console: 'bash',
	golang: 'go',
	js: 'js',
	jsonc: 'json',
	md: 'markdown',
	rs: 'rust',
	sh: 'bash',
	shell: 'bash',
	text: 'text',
	ts: 'ts',
	yml: 'yaml',
	zsh: 'bash',
};

const FENCED_CODE_BLOCK_PATTERN =
	/^ {0,3}(```+|~~~+)([^\n]*)\n([\s\S]*?)(?:\n {0,3}\1[~`]*[ \t]*(?=\n|$)|(?![\s\S]))/gm;

const escapeHtml = (value: string): string =>
	value
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;')
		.replaceAll("'", '&#39;');

const sanitizeHtml = (html: string): string =>
	DOMPurify.sanitize(html, {
		ADD_ATTR: [
			'class',
			'style',
			'tabindex',
			'target',
			'rel',
			'data-mermaid-pending',
			'data-mermaid-rendered',
		],
	});

const normalizeMarkedLanguage = (value: string | null | undefined): string | null => {
	const trimmed = value?.trim().toLowerCase() ?? '';
	if (!trimmed) return null;
	const first = trimmed.split(/\s+/, 1)[0];
	return MARKED_LANGUAGE_MAP[first] ?? first;
};

export const inferCodeLanguage = (path: string): string | null => {
	const trimmed = path.trim();
	if (!trimmed) return null;
	const fileName = trimmed.split('/').pop()?.toLowerCase() ?? '';
	if (fileName === 'dockerfile') return 'dockerfile';
	if (fileName === 'makefile') return 'make';
	if (fileName === '.gitignore') return 'gitignore';
	const dot = fileName.lastIndexOf('.');
	if (dot < 0 || dot === fileName.length - 1) return null;
	const ext = fileName.slice(dot + 1);
	return PATH_LANGUAGE_MAP[ext] ?? null;
};

const renderPlainTextBlock = (source: string): string => {
	const lines = source.replaceAll('\r\n', '\n').split('\n');
	return `<pre class="shiki ws-plain-text" style="background-color:#111827;color:#e5e7eb" tabindex="0"><code>${lines
		.map((line) => `<span class="line">${line.length > 0 ? escapeHtml(line) : '&#8203;'}</span>`)
		.join('\n')}</code></pre>`;
};

const renderMermaidHtml = (svg: string): string =>
	`<div class="ws-mermaid-block" data-mermaid-rendered="true"><div class="ws-mermaid-diagram">${svg}</div></div>`;

const renderMermaidError = (source: string, message: string): string =>
	`<div class="ws-mermaid-block" data-mermaid-rendered="error">${renderPlainTextBlock(source)}<div class="ws-mermaid-error">${escapeHtml(message)}</div></div>`;

const withMermaidContainer = async <T>(run: (container: HTMLElement) => Promise<T>): Promise<T> => {
	if (typeof document === 'undefined' || !document.body) {
		throw new Error('document body unavailable for Mermaid rendering');
	}
	const container = document.createElement('div');
	container.style.position = 'absolute';
	container.style.left = '-10000px';
	container.style.top = '0';
	container.style.width = '1600px';
	container.style.visibility = 'hidden';
	container.style.pointerEvents = 'none';
	container.setAttribute('aria-hidden', 'true');
	document.body.appendChild(container);
	try {
		return await run(container);
	} finally {
		container.remove();
	}
};

const renderHighlightedBlock = async (source: string, language: string | null): Promise<string> => {
	if (!language) {
		return renderPlainTextBlock(source);
	}
	try {
		return await codeToHtml(source, {
			lang: language,
			theme: SHIKI_THEME,
		});
	} catch {
		return renderPlainTextBlock(source);
	}
};

export async function renderCodeDocument(
	source: string,
	path: string,
	languageOverride?: string | null,
): Promise<DocumentRenderResult> {
	return {
		html: sanitizeHtml(
			await renderHighlightedBlock(source, languageOverride ?? inferCodeLanguage(path)),
		),
		containsMermaid: false,
	};
}

const MD_RENDER_CACHE_MAX = 20;
const mdRenderCache = new Map<string, DocumentRenderResult>();

const contentHash = (s: string): string => {
	let h = 5381;
	for (let i = 0; i < s.length; i++) {
		h = ((h << 5) + h + s.charCodeAt(i)) | 0;
	}
	return s.length + ':' + h.toString(36);
};

export async function renderMarkdownDocument(source: string): Promise<DocumentRenderResult> {
	const hash = contentHash(source);
	const cached = mdRenderCache.get(hash);
	if (cached) return cached;

	let containsMermaid = false;
	let processed = '';
	let cursor = 0;
	const mermaidSources: string[] = [];
	for (const match of source.matchAll(FENCED_CODE_BLOCK_PATTERN)) {
		const index = match.index ?? 0;
		processed += source.slice(cursor, index);
		const language = normalizeMarkedLanguage(match[2]);
		const blockSource = match[3] ?? '';
		if (language === 'mermaid') {
			containsMermaid = true;
			const mermaidIndex = mermaidSources.push(blockSource) - 1;
			processed += `<div data-mermaid-slot="m${mermaidIndex}"></div>`;
		} else {
			processed += await renderHighlightedBlock(blockSource, language);
		}
		cursor = index + match[0].length;
	}
	processed += source.slice(cursor);
	let html = marked.parse(processed, {
		gfm: true,
		breaks: true,
	}) as string;
	html = sanitizeHtml(html);
	if (!containsMermaid) {
		return {
			html,
			containsMermaid,
		};
	}
	const mermaidModule = await loadMermaid();
	const mermaid = mermaidModule.default;
	mermaid.initialize({
		startOnLoad: false,
		securityLevel: 'strict',
		theme: 'dark',
		fontFamily: 'inherit',
		suppressErrorRendering: true,
	});
	for (let index = 0; index < mermaidSources.length; index += 1) {
		const source = mermaidSources[index];
		let replacement: string;
		try {
			await mermaid.parse(source, { suppressErrors: false });
			const { svg } = await withMermaidContainer((container) =>
				mermaid.render(`ws-mermaid-${Date.now()}-${index}`, source, container),
			);
			replacement = renderMermaidHtml(svg);
		} catch (error) {
			const rawMessage =
				error instanceof Error ? error.message.trim() : 'Unable to render Mermaid diagram.';
			const hint =
				source.includes('participant ') &&
				!source.includes('sequenceDiagram') &&
				!source.includes('sequenceDiagram-v2')
					? 'Mermaid parse error: add a `sequenceDiagram` header before participant lines.'
					: `Mermaid parse error: ${rawMessage}`;
			replacement = renderMermaidError(source, hint);
		}
		html = html.replace(`<div data-mermaid-slot="m${index}"></div>`, replacement);
	}
	const result: DocumentRenderResult = { html, containsMermaid };
	if (mdRenderCache.size >= MD_RENDER_CACHE_MAX) {
		const firstKey = mdRenderCache.keys().next().value as string | undefined;
		if (firstKey !== undefined) mdRenderCache.delete(firstKey);
	}
	mdRenderCache.set(hash, result);
	return result;
}

let mermaidLoaded: Promise<typeof import('mermaid')> | null = null;

const loadMermaid = async (): Promise<typeof import('mermaid')> => {
	if (!mermaidLoaded) {
		mermaidLoaded = import('mermaid');
	}
	return mermaidLoaded;
};

export async function renderMermaidBlocks(container: HTMLElement): Promise<void> {
	const blocks = Array.from(
		container.querySelectorAll<HTMLElement>('.ws-mermaid-block[data-mermaid-pending="true"]'),
	);
	if (blocks.length === 0) return;
	const mermaidModule = await loadMermaid();
	const mermaid = mermaidModule.default;
	mermaid.initialize({
		startOnLoad: false,
		securityLevel: 'strict',
		theme: 'dark',
		fontFamily: 'inherit',
		suppressErrorRendering: true,
	});
}
