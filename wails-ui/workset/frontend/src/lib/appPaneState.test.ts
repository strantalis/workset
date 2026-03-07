import { describe, expect, it } from 'vitest';
import { resolveCockpitPaneState } from './appPaneState';

describe('resolveCockpitPaneState', () => {
	it('opens pull requests as the only active cockpit pane', () => {
		expect(
			resolveCockpitPaneState({
				surface: 'terminal',
				filesOpen: true,
				intent: 'pull-requests',
			}),
		).toEqual({ surface: 'pull-requests', filesOpen: false });
	});

	it('toggles pull requests back to terminal when already active', () => {
		expect(
			resolveCockpitPaneState({
				surface: 'pull-requests',
				filesOpen: false,
				intent: 'pull-requests',
			}),
		).toEqual({ surface: 'terminal', filesOpen: false });
	});

	it('opens files in terminal mode and clears the PR pane', () => {
		expect(
			resolveCockpitPaneState({
				surface: 'pull-requests',
				filesOpen: false,
				intent: 'files',
			}),
		).toEqual({ surface: 'terminal', filesOpen: true });
	});
});
