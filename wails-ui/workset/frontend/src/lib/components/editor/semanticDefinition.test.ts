import { afterEach, describe, expect, it, vi } from 'vitest';
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import type { RepoFileDefinitionResult } from '../../types';
import {
	createDefinitionRequestLifecycle,
	pickPreferredDefinitionTarget,
	semanticDefinitionExtension,
} from './semanticDefinition';

describe('semantic definition helpers', () => {
	let view: EditorView | null = null;

	afterEach(() => {
		view?.destroy();
		view = null;
	});

	it('prefers same-file definition targets before falling back to the first target', () => {
		expect(
			pickPreferredDefinitionTarget('ws-1::app', 'src/example.ts', [
				{
					repoId: 'ws-1::app',
					path: 'src/lib.ts',
					line: 1,
					character: 0,
					endLine: 1,
					endCharacter: 6,
				},
				{
					repoId: 'ws-1::app',
					path: 'src/example.ts',
					line: 8,
					character: 4,
					endLine: 8,
					endCharacter: 10,
				},
			]),
		).toEqual({
			repoId: 'ws-1::app',
			path: 'src/example.ts',
			line: 8,
			character: 4,
			endLine: 8,
			endCharacter: 10,
		});
	});

	it('returns null when there are no definition targets', () => {
		expect(pickPreferredDefinitionTarget('ws-1::app', 'src/example.ts', [])).toBeNull();
	});

	it('invalidates older requests when a newer definition lookup starts', () => {
		const lifecycle = createDefinitionRequestLifecycle();
		const first = lifecycle.beginRequest();
		const second = lifecycle.beginRequest();

		expect(lifecycle.isCurrent(first)).toBe(false);
		expect(lifecycle.isCurrent(second)).toBe(true);
	});

	it('invalidates pending requests when the extension is destroyed', () => {
		const lifecycle = createDefinitionRequestLifecycle();
		const requestId = lifecycle.beginRequest();

		lifecycle.deactivate();

		expect(lifecycle.isCurrent(requestId)).toBe(false);
	});

	it('ignores stale definition responses after a newer request wins', async () => {
		let resolveFirst: ((value: RepoFileDefinitionResult) => void) | null = null;
		let resolveSecond: ((value: RepoFileDefinitionResult) => void) | null = null;
		let callCount = 0;
		const onNavigate = vi.fn();

		view = new EditorView({
			state: EditorState.create({
				doc: 'const alpha = beta;\n',
				extensions: [
					semanticDefinitionExtension({
						filePath: 'src/example.ts',
						currentRepoId: 'ws-1::app',
						fetchDefinition: () =>
							new Promise((resolve) => {
								callCount += 1;
								if (callCount === 1) {
									resolveFirst = resolve;
									return;
								}
								resolveSecond = resolve;
							}),
						onNavigate,
					}),
				],
			}),
			parent: document.body,
		});

		const firstHandled = view.contentDOM.dispatchEvent(
			new KeyboardEvent('keydown', {
				bubbles: true,
				cancelable: true,
				key: 'F12',
			}),
		);
		void firstHandled;
		const secondHandled = view.contentDOM.dispatchEvent(
			new KeyboardEvent('keydown', {
				bubbles: true,
				cancelable: true,
				key: 'F12',
			}),
		);
		void secondHandled;

		resolveFirst!({
			supported: true,
			available: true,
			found: true,
			targets: [
				{
					repoId: 'ws-1::app',
					path: 'src/first.ts',
					line: 1,
					character: 0,
					endLine: 1,
					endCharacter: 5,
				},
			],
		});
		await Promise.resolve();

		expect(onNavigate).not.toHaveBeenCalled();

		resolveSecond!({
			supported: true,
			available: true,
			found: true,
			targets: [
				{
					repoId: 'ws-1::app',
					path: 'src/second.ts',
					line: 2,
					character: 1,
					endLine: 2,
					endCharacter: 6,
				},
			],
		});
		await Promise.resolve();

		expect(onNavigate).toHaveBeenCalledTimes(1);
		expect(onNavigate).toHaveBeenCalledWith({
			repoId: 'ws-1::app',
			path: 'src/second.ts',
			line: 2,
			character: 1,
			endLine: 2,
			endCharacter: 6,
		});
	});
});
