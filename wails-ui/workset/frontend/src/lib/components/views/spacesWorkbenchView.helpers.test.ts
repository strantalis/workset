import { describe, expect, it } from 'vitest';
import { resolveWorkbenchLayout } from './spacesWorkbenchView.helpers';

describe('resolveWorkbenchLayout', () => {
	it('keeps the terminal visible and opens PRs as a right pane when the PR surface is active', () => {
		expect(resolveWorkbenchLayout('pull-requests', false)).toBe('terminal-with-prs');
	});

	it('prioritizes the PR pane over the document viewer when both states are present', () => {
		expect(resolveWorkbenchLayout('pull-requests', true)).toBe('terminal-with-prs');
	});

	it('shows the document viewer only for the terminal surface', () => {
		expect(resolveWorkbenchLayout('terminal', true)).toBe('terminal-with-document');
		expect(resolveWorkbenchLayout('terminal', false)).toBe('terminal');
	});
});
