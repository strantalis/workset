import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';

type Deferred<T> = {
	promise: Promise<T>;
	resolve: (value: T) => void;
	reject: (reason?: unknown) => void;
};

const createDeferred = <T>(): Deferred<T> => {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
};

const terminalLayoutApiMocks = vi.hoisted(() => ({
	createWorkspaceTerminal: vi.fn(),
	fetchTerminalBootstrap: vi.fn(),
	fetchWorkspaceTerminalLayout: vi.fn(),
	persistWorkspaceTerminalLayout: vi.fn(),
	stopWorkspaceTerminal: vi.fn(),
}));

const settingsApiMocks = vi.hoisted(() => ({
	fetchSettings: vi.fn(),
	fetchSessiondStatus: vi.fn(),
	setDefaultSetting: vi.fn(),
}));

const terminalServiceMocks = vi.hoisted(() => ({
	closeTerminal: vi.fn().mockResolvedValue(undefined),
	decreaseFontSize: vi.fn(),
	getCurrentFontSize: vi.fn().mockReturnValue(14),
	increaseFontSize: vi.fn(),
	resetFontSize: vi.fn(),
}));

vi.mock('../api/terminal-layout', async (importOriginal) => {
	const actual = await importOriginal<typeof import('../api/terminal-layout')>();
	return {
		...actual,
		...terminalLayoutApiMocks,
	};
});
vi.mock('../api/settings', async (importOriginal) => {
	const actual = await importOriginal<typeof import('../api/settings')>();
	return {
		...actual,
		...settingsApiMocks,
	};
});
vi.mock('../terminal/terminalService', async (importOriginal) => {
	const actual = await importOriginal<typeof import('../terminal/terminalService')>();
	return {
		...actual,
		...terminalServiceMocks,
	};
});
vi.mock('./TerminalLayoutNode.svelte', async () => {
	const module = await import('./test-utils/MockTerminalLayoutNode.svelte');
	return { default: module.default };
});

import TerminalWorkspace from './TerminalWorkspace.svelte';

describe('TerminalWorkspace', () => {
	afterEach(() => {
		cleanup();
		vi.clearAllMocks();
	});

	it('does not prewarm terminals while the workspace surface is inactive', async () => {
		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout.mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout: {
				version: 2,
				tabs: [
					{
						id: 'tab-1',
						title: 'Workspace-0',
						root: {
							id: 'pane-1',
							kind: 'pane',
							terminalId: 'term-1',
						},
						focusedPaneId: 'pane-1',
					},
				],
				activeTabId: 'tab-1',
			},
		});
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchSessiondStatus.mockResolvedValue({ available: true });

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				active: false,
			},
		});

		await screen.findByRole('tab', { name: /workspace-0/i });

		await waitFor(() => {
			expect(terminalLayoutApiMocks.fetchWorkspaceTerminalLayout).toHaveBeenCalled();
		});
		expect(terminalLayoutApiMocks.fetchTerminalBootstrap).not.toHaveBeenCalled();
	});

	it('stays stable when init clears layout during a reset reload', async () => {
		const initialLayout = {
			version: 2,
			tabs: [
				{
					id: 'tab-1',
					title: 'Workspace-0',
					root: {
						id: 'pane-1',
						kind: 'pane',
						terminalId: 'term-1',
					},
					focusedPaneId: 'pane-1',
				},
			],
			activeTabId: 'tab-1',
		};

		const reloadedLayout = {
			...initialLayout,
			activeTabId: 'tab-2',
			tabs: [
				...initialLayout.tabs,
				{
					id: 'tab-2',
					title: 'Workspace-1',
					root: {
						id: 'pane-2',
						kind: 'pane',
						terminalId: 'term-2',
					},
					focusedPaneId: 'pane-2',
				},
			],
		};

		const reloadDeferred = createDeferred<{
			workspaceId: string;
			workspacePath: string;
			layout: typeof reloadedLayout;
		}>();

		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout
			.mockResolvedValueOnce({
				workspaceId: 'ws-1',
				workspacePath: '/tmp/ws-1',
				layout: initialLayout,
			})
			.mockImplementationOnce(() => reloadDeferred.promise);
		terminalLayoutApiMocks.fetchTerminalBootstrap.mockResolvedValue({
			workspaceId: 'ws-1',
			terminalId: 'term-1',
			sessionId: 'session-1',
			windowName: 'main',
			owner: 'main',
			canWrite: true,
			running: true,
			currentOffset: 0,
			socketUrl: 'ws://localhost/socket',
			socketToken: 'token',
			transport: 'sessiond-websocket',
		});
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchSessiondStatus.mockResolvedValue({ available: true });

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				active: true,
			},
		});

		await screen.findByRole('tab', { name: /workspace-0/i });
		await waitFor(() => {
			expect(terminalLayoutApiMocks.fetchTerminalBootstrap).toHaveBeenCalledWith('ws-1', 'term-1');
		});

		window.dispatchEvent(
			new CustomEvent('workset:terminal-layout-reset', {
				detail: { workspaceId: 'ws-1' },
			}),
		);

		expect(await screen.findByText('Starting thread terminals…')).toBeInTheDocument();

		reloadDeferred.resolve({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout: reloadedLayout,
		});

		await waitFor(() => {
			expect(screen.getByRole('tab', { name: /workspace-1/i })).toBeInTheDocument();
		});
		await waitFor(() => {
			expect(terminalLayoutApiMocks.fetchTerminalBootstrap).toHaveBeenCalledWith('ws-1', 'term-2');
		});
	});

	it('collapses a split when the focused pane is closed', async () => {
		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout.mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout: {
				version: 2,
				tabs: [
					{
						id: 'tab-1',
						title: 'Workspace-0',
						root: {
							id: 'split-1',
							kind: 'split',
							direction: 'row',
							ratio: 0.5,
							first: {
								id: 'pane-1',
								kind: 'pane',
								terminalId: 'term-1',
							},
							second: {
								id: 'pane-2',
								kind: 'pane',
								terminalId: 'term-2',
							},
						},
						focusedPaneId: 'pane-1',
					},
				],
				activeTabId: 'tab-1',
			},
		});
		terminalLayoutApiMocks.fetchTerminalBootstrap.mockResolvedValue({
			workspaceId: 'ws-1',
			terminalId: 'term-1',
			sessionId: 'session-1',
			windowName: 'main',
			owner: 'main',
			canWrite: true,
			running: true,
			currentOffset: 0,
			socketUrl: 'ws://localhost/socket',
			socketToken: 'token',
			transport: 'sessiond-websocket',
		});
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchSessiondStatus.mockResolvedValue({ available: true });

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				active: true,
			},
		});

		await screen.findByRole('tab', { name: /workspace-0/i });

		const closePane = await screen.findByTestId('mock-layout-close-pane');
		await fireEvent.click(closePane);

		await waitFor(() => {
			expect(terminalServiceMocks.closeTerminal).toHaveBeenCalledWith('ws-1', 'term-1');
		});
		await waitFor(() => {
			expect(screen.getByTestId('mock-terminal-layout-node')).toHaveTextContent('term-2');
		});
	});
});
