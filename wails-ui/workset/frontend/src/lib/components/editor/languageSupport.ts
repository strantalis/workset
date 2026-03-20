import { StreamLanguage } from '@codemirror/language';
import type { Extension } from '@codemirror/state';

type LanguageLoader = () => Promise<Extension>;

/** Helper to wrap a legacy CM5 mode as a CM6 extension. */
const legacy = (
	importFn: () => Promise<Record<string, unknown>>,
	modeName: string,
): LanguageLoader => {
	return async () => {
		const mod = await importFn();
		const mode = mod[modeName];
		if (!mode) return StreamLanguage.define({} as never);
		return StreamLanguage.define(mode as Parameters<typeof StreamLanguage.define>[0]);
	};
};

const languageMap: Record<string, LanguageLoader> = {
	// ── Official CM6 language packages ────────────────────
	ts: () => import('@codemirror/lang-javascript').then((m) => m.javascript({ typescript: true })),
	tsx: () =>
		import('@codemirror/lang-javascript').then((m) =>
			m.javascript({ typescript: true, jsx: true }),
		),
	js: () => import('@codemirror/lang-javascript').then((m) => m.javascript()),
	jsx: () => import('@codemirror/lang-javascript').then((m) => m.javascript({ jsx: true })),
	mjs: () => import('@codemirror/lang-javascript').then((m) => m.javascript()),
	cjs: () => import('@codemirror/lang-javascript').then((m) => m.javascript()),
	go: () => import('@codemirror/lang-go').then((m) => m.go()),
	css: () => import('@codemirror/lang-css').then((m) => m.css()),
	html: () => import('@codemirror/lang-html').then((m) => m.html()),
	svelte: () => import('@codemirror/lang-html').then((m) => m.html()),
	json: () => import('@codemirror/lang-json').then((m) => m.json()),
	md: () => import('@codemirror/lang-markdown').then((m) => m.markdown()),
	markdown: () => import('@codemirror/lang-markdown').then((m) => m.markdown()),
	yaml: () => import('@codemirror/lang-yaml').then((m) => m.yaml()),
	yml: () => import('@codemirror/lang-yaml').then((m) => m.yaml()),
	py: () => import('@codemirror/lang-python').then((m) => m.python()),
	python: () => import('@codemirror/lang-python').then((m) => m.python()),

	// ── Legacy CM5 modes (via @codemirror/legacy-modes) ──
	sh: legacy(() => import('@codemirror/legacy-modes/mode/shell'), 'shell'),
	bash: legacy(() => import('@codemirror/legacy-modes/mode/shell'), 'shell'),
	zsh: legacy(() => import('@codemirror/legacy-modes/mode/shell'), 'shell'),
	toml: legacy(() => import('@codemirror/legacy-modes/mode/toml'), 'toml'),
	rs: legacy(() => import('@codemirror/legacy-modes/mode/rust'), 'rust'),
	rb: legacy(() => import('@codemirror/legacy-modes/mode/ruby'), 'ruby'),
	sql: legacy(() => import('@codemirror/legacy-modes/mode/sql'), 'sql'),
	lua: legacy(() => import('@codemirror/legacy-modes/mode/lua'), 'lua'),
	pl: legacy(() => import('@codemirror/legacy-modes/mode/perl'), 'perl'),
	r: legacy(() => import('@codemirror/legacy-modes/mode/r'), 'r'),
	swift: legacy(() => import('@codemirror/legacy-modes/mode/swift'), 'swift'),
	proto: legacy(() => import('@codemirror/legacy-modes/mode/protobuf'), 'protobuf'),
	diff: legacy(() => import('@codemirror/legacy-modes/mode/diff'), 'diff'),
};

/** Files matched by full filename rather than extension. */
const filenameMap: Record<string, LanguageLoader> = {
	dockerfile: legacy(() => import('@codemirror/legacy-modes/mode/dockerfile'), 'dockerFile'),
	makefile: legacy(() => import('@codemirror/legacy-modes/mode/cmake'), 'cmake'),
	cmakelists: legacy(() => import('@codemirror/legacy-modes/mode/cmake'), 'cmake'),
	'.gitignore': legacy(() => import('@codemirror/legacy-modes/mode/properties'), 'properties'),
	'.dockerignore': legacy(() => import('@codemirror/legacy-modes/mode/properties'), 'properties'),
	'.env': legacy(() => import('@codemirror/legacy-modes/mode/properties'), 'properties'),
	'.editorconfig': legacy(() => import('@codemirror/legacy-modes/mode/properties'), 'properties'),
	'nginx.conf': legacy(() => import('@codemirror/legacy-modes/mode/nginx'), 'nginx'),
};

/** Extract file extension from a path, lowercase. */
const extFromPath = (path: string): string => {
	const dot = path.lastIndexOf('.');
	return dot >= 0 ? path.slice(dot + 1).toLowerCase() : '';
};

/** Extract filename from a path, lowercase. */
const fileNameFromPath = (path: string): string => {
	const slash = path.lastIndexOf('/');
	return (slash >= 0 ? path.slice(slash + 1) : path).toLowerCase();
};

/**
 * Dynamically load the CodeMirror language extension for a file path.
 * Returns null if no language support is available.
 */
export const loadLanguage = async (filePath: string): Promise<Extension | null> => {
	const fileName = fileNameFromPath(filePath);

	// Check filename-based matches first (Dockerfile, Makefile, .gitignore, etc.)
	const fnLoader = filenameMap[fileName];
	if (fnLoader) {
		try {
			return await fnLoader();
		} catch {
			return null;
		}
	}

	// Fall back to extension-based matches
	const ext = extFromPath(filePath);
	// go.mod and go.sum → use Go highlighting
	if (fileName === 'go.mod' || fileName === 'go.sum' || fileName === 'go.work') {
		const loader = languageMap['go'];
		try {
			return await loader();
		} catch {
			return null;
		}
	}

	const loader = languageMap[ext];
	if (!loader) return null;
	try {
		return await loader();
	} catch {
		return null;
	}
};
