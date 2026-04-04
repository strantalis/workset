import { cleanup, render } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

const performanceMocks = vi.hoisted(() => ({
	createTerminalPerformanceSampler: vi.fn(() => ({
		sampleFrame: () => ({ fps: 0, frameTimeMs: 0 }),
		reset: () => undefined,
	})),
}));

vi.mock('../terminal/terminalPerformance', () => performanceMocks);
vi.mock('../terminal/TerminalController.svelte', async () => {
	const module = await import('./test-utils/MockTerminalController.svelte');
	return { default: module.default };
});

import TerminalPane from './TerminalPane.svelte';
import {
	mockTerminalControllerTracker,
	resetMockTerminalControllerTracker,
} from './test-utils/mockTerminalControllerTracker';

describe('TerminalPane', () => {
	afterEach(() => {
		cleanup();
		resetMockTerminalControllerTracker();
		vi.clearAllMocks();
	});

	it('remounts the terminal controller when the terminal id changes', async () => {
		const view = render(TerminalPane, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				terminalId: 'term-1',
				active: true,
				compact: true,
			},
		});

		expect(mockTerminalControllerTracker.mounts).toEqual([
			{ workspaceId: 'ws-1', terminalId: 'term-1' },
		]);

		await view.rerender({
			workspaceId: 'ws-1',
			workspaceName: 'Workspace',
			terminalId: 'term-2',
			active: true,
			compact: true,
		});

		expect(mockTerminalControllerTracker.destroys).toEqual([
			{ workspaceId: 'ws-1', terminalId: 'term-1' },
		]);
		expect(mockTerminalControllerTracker.mounts).toEqual([
			{ workspaceId: 'ws-1', terminalId: 'term-1' },
			{ workspaceId: 'ws-1', terminalId: 'term-2' },
		]);
	});
});
