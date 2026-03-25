import type { RepoFileDefinitionTarget } from '../../types';
import { EditorView } from '@codemirror/view';

type DefinitionNavigationOptions = {
	target: RepoFileDefinitionTarget;
	editorView: EditorView | null;
	selectedRepoId: string | null;
	selectedFilePath: string | null;
	setPendingTarget: (target: RepoFileDefinitionTarget | null) => void;
	selectTreeFile: (path: string, repoId: string) => void;
};

type DefinitionNavigateHandlerOptions = {
	getEditorView: () => EditorView | null;
	getSelectedRepoId: () => string | null;
	getSelectedFilePath: () => string | null;
	getSelectTreeFile: () => (path: string, repoId: string) => void;
	setPendingTarget: (target: RepoFileDefinitionTarget | null) => void;
};

type PendingDefinitionOptions = {
	target: RepoFileDefinitionTarget | null;
	editorView: EditorView | null;
	editorViewPath: string | null;
	selectedRepoId: string | null;
	selectedFilePath: string | null;
	isCurrent: () => boolean;
	setPendingTarget: (target: RepoFileDefinitionTarget | null) => void;
};

export function navigateRepoDefinitionTarget(options: DefinitionNavigationOptions): void {
	const { target, editorView, selectedRepoId, selectedFilePath, setPendingTarget, selectTreeFile } =
		options;
	setPendingTarget(target);
	if (target.repoId === selectedRepoId && target.path === selectedFilePath && editorView) {
		revealRepoDefinitionTarget(editorView, target);
		setPendingTarget(null);
		return;
	}
	selectTreeFile(target.path, target.repoId);
}

export function createRepoDefinitionNavigateHandler(
	options: DefinitionNavigateHandlerOptions,
): (target: RepoFileDefinitionTarget) => void {
	return (target) =>
		navigateRepoDefinitionTarget({
			target,
			editorView: options.getEditorView(),
			selectedRepoId: options.getSelectedRepoId(),
			selectedFilePath: options.getSelectedFilePath(),
			setPendingTarget: options.setPendingTarget,
			selectTreeFile: options.getSelectTreeFile(),
		});
}

export function flushPendingRepoDefinitionTarget(options: PendingDefinitionOptions): void {
	const {
		target,
		editorView,
		editorViewPath,
		selectedRepoId,
		selectedFilePath,
		isCurrent,
		setPendingTarget,
	} = options;
	if (!target || !editorView) return;
	if (target.repoId !== selectedRepoId || target.path !== selectedFilePath) return;
	if (editorViewPath !== target.path) return;
	scheduleDefinitionReveal(() => {
		if (!isCurrent()) return;
		revealRepoDefinitionTarget(editorView, target);
		setPendingTarget(null);
	});
}

export function revealRepoDefinitionTarget(
	editorView: EditorView,
	target: RepoFileDefinitionTarget,
): void {
	const line = editorView.state.doc.line(
		Math.min(editorView.state.doc.lines, Math.max(1, target.line + 1)),
	);
	const pos = line.from + Math.min(target.character, line.length);
	editorView.focus();
	editorView.dispatch({
		selection: { anchor: pos },
		effects: EditorView.scrollIntoView(pos, { y: 'center' }),
	});
	scheduleDefinitionReveal(() => editorView.focus());
}

function scheduleDefinitionReveal(callback: () => void): void {
	if (typeof globalThis.requestAnimationFrame === 'function') {
		globalThis.requestAnimationFrame(() => {
			globalThis.requestAnimationFrame(() => callback());
		});
		return;
	}
	globalThis.setTimeout(callback, 16);
}
