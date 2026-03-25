import { describe, expect, it, vi } from 'vitest';
import { flushPendingRepoDefinitionTarget } from './unifiedRepoDefinition';
import type { EditorView } from '@codemirror/view';

function buildEditorView() {
	const line = { from: 10, length: 40 };
	return {
		state: {
			doc: {
				lines: 20,
				line: vi.fn(() => line),
			},
		},
		dispatch: vi.fn(),
		focus: vi.fn(),
	} as unknown as EditorView;
}

describe('unified repo definition helpers', () => {
	it('waits for the next animation frame before revealing a matching target', () => {
		const editorView = buildEditorView();
		const setPendingTarget = vi.fn();
		const callbacks: FrameRequestCallback[] = [];
		const requestAnimationFrame = vi
			.spyOn(globalThis, 'requestAnimationFrame')
			.mockImplementation((callback: FrameRequestCallback) => {
				callbacks.push(callback);
				return callbacks.length;
			});

		try {
			flushPendingRepoDefinitionTarget({
				target: {
					repoId: 'ws-1::repo-alpha',
					path: 'src/target.ts',
					line: 8,
					character: 4,
					endLine: 8,
					endCharacter: 10,
				},
				editorView,
				editorViewPath: 'src/target.ts',
				selectedRepoId: 'ws-1::repo-alpha',
				selectedFilePath: 'src/target.ts',
				isCurrent: () => true,
				setPendingTarget,
			});

			expect(editorView.dispatch).not.toHaveBeenCalled();
			expect(editorView.focus).not.toHaveBeenCalled();
			expect(setPendingTarget).not.toHaveBeenCalled();

			const firstCallback = callbacks.shift();
			expect(firstCallback).toBeTypeOf('function');
			firstCallback?.(0);

			expect(editorView.dispatch).not.toHaveBeenCalled();
			expect(editorView.focus).not.toHaveBeenCalled();
			expect(setPendingTarget).not.toHaveBeenCalled();

			const secondCallback = callbacks.shift();
			expect(secondCallback).toBeTypeOf('function');
			secondCallback?.(0);

			expect(editorView.dispatch).toHaveBeenCalledWith({
				selection: { anchor: 14 },
				effects: expect.anything(),
			});
			expect(editorView.focus).toHaveBeenCalledTimes(1);
			expect(setPendingTarget).toHaveBeenCalledWith(null);

			const thirdCallback = callbacks.shift();
			expect(thirdCallback).toBeTypeOf('function');
			thirdCallback?.(0);

			expect(editorView.focus).toHaveBeenCalledTimes(1);

			const fourthCallback = callbacks.shift();
			expect(fourthCallback).toBeTypeOf('function');
			fourthCallback?.(0);

			expect(editorView.focus).toHaveBeenCalledTimes(2);
		} finally {
			requestAnimationFrame.mockRestore();
		}
	});

	it('waits for the target file editor before revealing a pending definition jump', async () => {
		const editorView = buildEditorView();
		const setPendingTarget = vi.fn();

		flushPendingRepoDefinitionTarget({
			target: {
				repoId: 'ws-1::repo-alpha',
				path: 'src/target.ts',
				line: 8,
				character: 4,
				endLine: 8,
				endCharacter: 10,
			},
			editorView,
			editorViewPath: 'src/current.ts',
			selectedRepoId: 'ws-1::repo-alpha',
			selectedFilePath: 'src/target.ts',
			isCurrent: () => true,
			setPendingTarget,
		});

		expect(editorView.dispatch).not.toHaveBeenCalled();
		expect(editorView.focus).not.toHaveBeenCalled();
		expect(setPendingTarget).not.toHaveBeenCalled();
	});
});
