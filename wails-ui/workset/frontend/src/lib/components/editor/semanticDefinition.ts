import type { RepoFileDefinitionResult, RepoFileDefinitionTarget } from '../../types';
import type { Extension } from '@codemirror/state';
import { keymap, ViewPlugin } from '@codemirror/view';
import { EditorView } from '@codemirror/view';
import { positionToLspLineCharacter, supportsSemanticHover } from './semanticHover';

export type SemanticDefinitionRequest = {
	filePath: string;
	content: string;
	line: number;
	character: number;
};

export type SemanticDefinitionOptions = {
	filePath: string;
	currentRepoId: string;
	fetchDefinition: (request: SemanticDefinitionRequest) => Promise<RepoFileDefinitionResult>;
	onNavigate: (target: RepoFileDefinitionTarget) => void;
};

export type DefinitionRequestLifecycle = {
	beginRequest: () => number;
	isCurrent: (requestId: number) => boolean;
	deactivate: () => void;
};

export function pickPreferredDefinitionTarget(
	currentRepoId: string,
	currentPath: string,
	targets: RepoFileDefinitionTarget[] | null | undefined,
): RepoFileDefinitionTarget | null {
	if (!targets || targets.length === 0) return null;
	return (
		targets.find((target) => target.repoId === currentRepoId && target.path === currentPath) ??
		targets[0] ??
		null
	);
}

export function semanticDefinitionExtension(options: SemanticDefinitionOptions): Extension {
	if (!supportsSemanticHover(options.filePath)) {
		return [];
	}
	const lifecycle = createDefinitionRequestLifecycle();

	const goToDefinition = (view: EditorView, pos: number): boolean => {
		void requestDefinition(options, lifecycle, view, pos, lifecycle.beginRequest());
		return true;
	};

	return [
		ViewPlugin.fromClass(
			class {
				destroy(): void {
					lifecycle.deactivate();
				}
			},
		),
		EditorView.domEventHandlers({
			mousedown(event, view) {
				if (!isDefinitionGesture(event)) return false;
				const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
				if (pos == null || !view.state.wordAt(pos)) return false;
				event.preventDefault();
				return goToDefinition(view, pos);
			},
		}),
		keymap.of([
			{
				key: 'F12',
				run(view) {
					return goToDefinition(view, view.state.selection.main.head);
				},
			},
		]),
	];
}

async function requestDefinition(
	options: SemanticDefinitionOptions,
	lifecycle: DefinitionRequestLifecycle,
	view: EditorView,
	pos: number,
	requestId: number,
): Promise<void> {
	let response: RepoFileDefinitionResult;
	try {
		response = await options.fetchDefinition({
			filePath: options.filePath,
			content: view.state.doc.toString(),
			...positionToLspLineCharacter(view, pos),
		});
	} catch {
		return;
	}

	if (!lifecycle.isCurrent(requestId)) return;
	if (!response.supported || !response.available || !response.found) return;
	const target = pickPreferredDefinitionTarget(
		options.currentRepoId,
		options.filePath,
		response.targets,
	);
	if (!target) return;
	options.onNavigate(target);
}

export function createDefinitionRequestLifecycle(): DefinitionRequestLifecycle {
	let currentRequestId = 0;
	let active = true;
	return {
		beginRequest() {
			currentRequestId += 1;
			return currentRequestId;
		},
		isCurrent(requestId) {
			return active && requestId === currentRequestId;
		},
		deactivate() {
			active = false;
			currentRequestId += 1;
		},
	};
}

function isDefinitionGesture(event: MouseEvent): boolean {
	if (event.button !== 0 || event.altKey || event.shiftKey) {
		return false;
	}
	return isApplePlatform() ? event.metaKey && !event.ctrlKey : event.ctrlKey;
}

function isApplePlatform(): boolean {
	return /Mac|iPhone|iPad|iPod/i.test(globalThis.navigator?.platform ?? '');
}
