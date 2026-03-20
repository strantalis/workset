import { EditorView } from '@codemirror/view';
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
import { tags } from '@lezer/highlight';

/**
 * Workset dark theme for CodeMirror 6.
 * Uses hardcoded values matching the CSS variables in style.css
 * since CM6 themes are built at JS level, not CSS variable level.
 */

const bg = '#0c1019';
const panel = '#101925';
const panelStrong = '#15202f';
const text = '#f2f6fb';
const muted = '#a3b5c9';
const subtle = '#8a9bb0';
const border = '#243244';
const accent = '#2d8cff';
const success = '#86c442';
const danger = '#ef4444';
const warning = '#f59e0b';
const purple = '#8b8aed';

const fontMono = "'JetBrains Mono', 'Fira Code', Menlo, Consolas, monospace";

export const worksetTheme = EditorView.theme(
	{
		'&': {
			color: text,
			backgroundColor: bg,
			fontFamily: fontMono,
			fontSize: '12px',
		},
		'.cm-content': {
			caretColor: accent,
			fontFamily: fontMono,
		},
		'.cm-cursor, .cm-dropCursor': {
			borderLeftColor: accent,
		},
		'&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
			backgroundColor: `color-mix(in srgb, ${accent} 25%, transparent)`,
		},
		'.cm-panels': {
			backgroundColor: panel,
			color: text,
			borderColor: border,
		},
		'.cm-panels.cm-panels-top': {
			borderBottom: `1px solid ${border}`,
		},
		'.cm-panels.cm-panels-bottom': {
			borderTop: `1px solid ${border}`,
		},
		'.cm-searchMatch': {
			backgroundColor: `color-mix(in srgb, ${warning} 30%, transparent)`,
			outline: `1px solid color-mix(in srgb, ${warning} 50%, transparent)`,
		},
		'.cm-searchMatch.cm-searchMatch-selected': {
			backgroundColor: `color-mix(in srgb, ${accent} 30%, transparent)`,
		},
		'.cm-activeLine': {
			backgroundColor: `color-mix(in srgb, ${panelStrong} 60%, transparent)`,
		},
		'.cm-selectionMatch': {
			backgroundColor: `color-mix(in srgb, ${accent} 18%, transparent)`,
		},
		'&.cm-focused .cm-matchingBracket, &.cm-focused .cm-nonmatchingBracket': {
			outline: `1px solid color-mix(in srgb, ${muted} 50%, transparent)`,
		},
		'.cm-gutters': {
			backgroundColor: panel,
			color: subtle,
			border: 'none',
			borderRight: `1px solid ${border}`,
		},
		'.cm-activeLineGutter': {
			backgroundColor: panelStrong,
			color: text,
		},
		'.cm-foldPlaceholder': {
			backgroundColor: panelStrong,
			color: muted,
			border: `1px solid ${border}`,
		},
		'.cm-tooltip': {
			backgroundColor: panel,
			color: text,
			border: `1px solid ${border}`,
		},
		'.cm-tooltip .cm-tooltip-arrow:before': {
			borderTopColor: 'transparent',
			borderBottomColor: 'transparent',
		},
		'.cm-tooltip .cm-tooltip-arrow:after': {
			borderTopColor: panel,
			borderBottomColor: panel,
		},
		'.cm-tooltip-autocomplete': {
			'& > ul > li[aria-selected]': {
				backgroundColor: panelStrong,
				color: text,
			},
		},
		// Merge view specific
		'.cm-mergeView': {
			backgroundColor: bg,
		},
		'.cm-changedLine': {
			backgroundColor: `color-mix(in srgb, ${accent} 8%, transparent)`,
		},
		'.cm-changedText': {
			backgroundColor: `color-mix(in srgb, ${accent} 20%, transparent)`,
		},
		'.cm-insertedLine, .cm-mergeView .cm-changedLine.cm-insertedLine': {
			backgroundColor: `color-mix(in srgb, ${success} 10%, transparent)`,
		},
		'.cm-deletedLine, .cm-mergeView .cm-changedLine.cm-deletedLine': {
			backgroundColor: `color-mix(in srgb, ${danger} 10%, transparent)`,
		},
		'.cm-insertedText': {
			backgroundColor: `color-mix(in srgb, ${success} 25%, transparent)`,
		},
		'.cm-deletedText': {
			backgroundColor: `color-mix(in srgb, ${danger} 25%, transparent)`,
		},
		'.cm-collapsedLines': {
			backgroundColor: panelStrong,
			color: muted,
			borderColor: border,
		},
	},
	{ dark: true },
);

export const worksetHighlightStyle = syntaxHighlighting(
	HighlightStyle.define([
		{ tag: tags.keyword, color: purple },
		{ tag: [tags.name, tags.deleted, tags.character, tags.macroName], color: text },
		{ tag: [tags.function(tags.variableName)], color: accent },
		{ tag: [tags.labelName], color: muted },
		{ tag: [tags.color, tags.constant(tags.name), tags.standard(tags.name)], color: warning },
		{ tag: [tags.definition(tags.name), tags.separator], color: text },
		{
			tag: [
				tags.typeName,
				tags.className,
				tags.number,
				tags.changed,
				tags.annotation,
				tags.modifier,
				tags.self,
				tags.namespace,
			],
			color: warning,
		},
		{
			tag: [
				tags.operator,
				tags.operatorKeyword,
				tags.url,
				tags.escape,
				tags.regexp,
				tags.link,
				tags.special(tags.string),
			],
			color: accent,
		},
		{ tag: [tags.meta, tags.comment], color: subtle },
		{ tag: tags.strong, fontWeight: 'bold' },
		{ tag: tags.emphasis, fontStyle: 'italic' },
		{ tag: tags.strikethrough, textDecoration: 'line-through' },
		{ tag: tags.link, color: accent, textDecoration: 'underline' },
		{ tag: tags.heading, fontWeight: 'bold', color: text },
		{ tag: [tags.atom, tags.bool, tags.special(tags.variableName)], color: warning },
		{ tag: [tags.processingInstruction, tags.string, tags.inserted], color: success },
		{ tag: tags.invalid, color: danger },
	]),
);

export const worksetExtensions = [worksetTheme, worksetHighlightStyle];
