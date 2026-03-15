import { describe, expect, it } from 'vitest';
import { resolveWorkbenchPaneState } from './appPaneState';

describe('resolveWorkbenchPaneState', () => {
	it('opens pull requests as the only active workbench pane', () => {
		expect(
			resolveWorkbenchPaneState({
				surface: 'terminal',
				filesOpen: true,
				intent: 'pull-requests',
			}),
		).toEqual({ surface: 'pull-requests', filesOpen: false });
	});

	it('toggles pull requests back to terminal when already active', () => {
		expect(
			resolveWorkbenchPaneState({
				surface: 'pull-requests',
				filesOpen: false,
				intent: 'pull-requests',
			}),
		).toEqual({ surface: 'terminal', filesOpen: false });
	});

	it('opens files in terminal mode and clears the PR pane', () => {
		expect(
			resolveWorkbenchPaneState({
				surface: 'pull-requests',
				filesOpen: false,
				intent: 'files',
			}),
		).toEqual({ surface: 'terminal', filesOpen: true });
	});
});
