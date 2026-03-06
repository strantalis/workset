import { describe, expect, test } from 'vitest';
import { shouldClearPreviousWorkspaceTerminalActivity } from './terminalActivity';

describe('shouldClearPreviousWorkspaceTerminalActivity', () => {
	test('clears the previous workspace when switching threads in the main window', () => {
		expect(
			shouldClearPreviousWorkspaceTerminalActivity({
				previousWorkspaceId: 'thread-alpha',
				nextWorkspaceId: 'thread-beta',
				previousWorkspacePoppedOut: false,
			}),
		).toBe(true);
	});

	test('preserves activity when reselecting the same workspace', () => {
		expect(
			shouldClearPreviousWorkspaceTerminalActivity({
				previousWorkspaceId: 'thread-alpha',
				nextWorkspaceId: 'thread-alpha',
				previousWorkspacePoppedOut: false,
			}),
		).toBe(false);
	});

	test('preserves activity for popped out workspaces', () => {
		expect(
			shouldClearPreviousWorkspaceTerminalActivity({
				previousWorkspaceId: 'thread-alpha',
				nextWorkspaceId: 'thread-beta',
				previousWorkspacePoppedOut: true,
			}),
		).toBe(false);
	});
});
