import { cleanup, render } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { writable } from 'svelte/store';

const terminalServiceMocks = vi.hoisted(() => ({
	detachTerminal: vi.fn(),
	focusTerminalInstance: vi.fn(),
	getTerminalStore: vi.fn(() =>
		writable({
			status: 'starting',
			message: '',
			health: 'ok',
			healthMessage: '',
			terminalServiceAvailable: true,
			terminalServiceChecked: true,
			debugEnabled: false,
			debugStats: {
				bytesIn: 0,
				bytesOut: 0,
				lastOutputAt: 0,
				lastCprAt: 0,
			},
		}),
	),
	isTerminalAtBottom: vi.fn(() => true),
	scrollTerminalToBottom: vi.fn(),
	syncTerminal: vi.fn(),
}));

vi.mock('../api/terminal-layout', async (importOriginal) => {
	const actual = await importOriginal<typeof import('../api/terminal-layout')>();
	return {
		...actual,
		logTerminalDebug: vi.fn(),
	};
});

vi.mock('./terminalService', async (importOriginal) => {
	const actual = await importOriginal<typeof import('./terminalService')>();
	return {
		...actual,
		...terminalServiceMocks,
	};
});

import TerminalController from './TerminalController.svelte';

describe('TerminalController', () => {
	afterEach(() => {
		cleanup();
		vi.clearAllMocks();
	});

	it('force detaches the terminal when the controller is destroyed', () => {
		const container = document.createElement('div');
		document.body.appendChild(container);

		const view = render(TerminalController, {
			props: {
				workspaceId: 'demo',
				workspaceName: 'Demo',
				terminalId: 'term-1',
				terminalContainer: container,
			},
		});

		view.unmount();

		expect(terminalServiceMocks.detachTerminal).toHaveBeenCalledWith('demo', 'term-1', {
			force: true,
		});

		container.remove();
	});
});
