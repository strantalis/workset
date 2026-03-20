import { describe, expect, it } from 'vitest';
import { resolveWorkbenchLayout } from './spacesWorkbenchView.helpers';

describe('resolveWorkbenchLayout', () => {
	it('opens the unified code pane when pull-requests surface is active', () => {
		expect(resolveWorkbenchLayout('pull-requests')).toBe('terminal-with-prs');
	});

	it('returns terminal-only for the terminal surface', () => {
		expect(resolveWorkbenchLayout('terminal')).toBe('terminal');
	});
});
