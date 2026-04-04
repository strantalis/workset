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
	fetchTerminalBootstrap: vi.fn().mockResolvedValue(null),
	fetchWorkspaceTerminalLayout: vi.fn(),
	persistWorkspaceTerminalLayout: vi.fn().mockResolvedValue(undefined),
	stopWorkspaceTerminal: vi.fn(),
}));

const settingsApiMocks = vi.hoisted(() => ({
	fetchSettings: vi.fn(),
	fetchTerminalServiceStatus: vi.fn(),
	setDefaultSetting: vi.fn(),
}));

const terminalServiceMocks = vi.hoisted(() => ({
	captureTerminalSnapshot: vi.fn().mockReturnValue(null),
	closeTerminal: vi.fn().mockResolvedValue(undefined),
	decreaseFontSize: vi.fn(),
	getCurrentFontSize: vi.fn().mockReturnValue(14),
	increaseFontSize: vi.fn(),
	resetFontSize: vi.fn(),
}));

vi.mock('../api/terminal-layout', () => {
	return {
		logTerminalDebug: vi.fn().mockResolvedValue(undefined),
		...terminalLayoutApiMocks,
	};
});
vi.mock('../api/settings', () => {
	return {
		...settingsApiMocks,
	};
});
vi.mock('../terminal/terminalService', () => {
	return {
		...terminalServiceMocks,
	};
});
vi.mock('./TerminalPane.svelte', async () => {
	const module = await import('./test-utils/MockTerminalPane.svelte');
	return { default: module.default };
});

import TerminalWorkspace from './TerminalWorkspace.svelte';

// V3 format — normalizeLayout will migrate to v4 tree on load
const buildTerminalLayout = (terminalId: string, title = 'Workspace-0') => ({
	version: 3,
	tabs: [
		{
			id: 'tab-1',
			title,
			panes: [{ id: 'pane-1', terminalId }],
			focusedPaneId: 'pane-1',
		},
	],
	activeTabId: 'tab-1',
});

const buildTerminalDescriptor = (workspaceId: string, terminalId: string) => ({
	workspaceId,
	terminalId,
	sessionId: `session-${workspaceId}-${terminalId}`,
	socketUrl: 'ws://localhost/socket',
	socketToken: 'token',
});

describe('TerminalWorkspace', () => {
	afterEach(() => {
		cleanup();
		vi.useRealTimers();
		vi.clearAllMocks();
	});

	it('does not prewarm terminals while the workspace surface is inactive', async () => {
		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout.mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout: {
				version: 3,
				tabs: [
					{
						id: 'tab-1',
						title: 'Workspace-0',
						panes: [{ id: 'pane-1', terminalId: 'term-1' }],
						focusedPaneId: 'pane-1',
					},
				],
				activeTabId: 'tab-1',
			},
		});
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchTerminalServiceStatus.mockResolvedValue({ available: true });

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

	it('keeps the existing layout visible during a reset reload', async () => {
		const initialLayout = buildTerminalLayout('term-1');

		const reloadedLayout = {
			...initialLayout,
			activeTabId: 'tab-2',
			tabs: [
				...initialLayout.tabs,
				{
					id: 'tab-2',
					title: 'Workspace-1',
					panes: [{ id: 'pane-2', terminalId: 'term-2' }],
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
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchTerminalServiceStatus.mockResolvedValue({ available: true });

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				active: true,
			},
		});

		await screen.findByRole('tab', { name: /workspace-0/i });

		window.dispatchEvent(
			new CustomEvent('workset:terminal-layout-reset', {
				detail: { workspaceId: 'ws-1' },
			}),
		);

		expect(screen.queryByText('Starting thread terminals…')).not.toBeInTheDocument();
		expect(screen.getByRole('tab', { name: /workspace-0/i })).toBeInTheDocument();

		reloadDeferred.resolve({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout: reloadedLayout,
		});

		await waitFor(() => {
			expect(screen.getByRole('tab', { name: /workspace-1/i })).toBeInTheDocument();
		});
	});

	it('keeps split panes visible during a reset reload', async () => {
		const initialLayout = {
			version: 3,
			tabs: [
				{
					id: 'tab-1',
					title: 'Workspace-0',
					panes: [
						{ id: 'pane-1', terminalId: 'term-1' },
						{ id: 'pane-2', terminalId: 'term-2' },
					],
					splitDirection: 'vertical',
					splitRatio: 0.5,
					focusedPaneId: 'pane-1',
				},
			],
			activeTabId: 'tab-1',
		};

		const reloadDeferred = createDeferred<{
			workspaceId: string;
			workspacePath: string;
			layout: typeof initialLayout;
		}>();

		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout
			.mockResolvedValueOnce({
				workspaceId: 'ws-1',
				workspacePath: '/tmp/ws-1',
				layout: initialLayout,
			})
			.mockImplementationOnce(() => reloadDeferred.promise);
		terminalLayoutApiMocks.fetchTerminalBootstrap.mockImplementation(
			async (_workspaceId, terminalId) => buildTerminalDescriptor('ws-1', String(terminalId)),
		);
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchTerminalServiceStatus.mockResolvedValue({ available: true });

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				active: true,
			},
		});

		await waitFor(() => {
			expect(screen.getAllByTestId('mock-terminal-pane')).toHaveLength(2);
		});

		window.dispatchEvent(
			new CustomEvent('workset:terminal-layout-reset', {
				detail: { workspaceId: 'ws-1' },
			}),
		);

		expect(screen.queryByText('Starting thread terminals…')).not.toBeInTheDocument();
		expect(screen.getAllByTestId('mock-terminal-pane')).toHaveLength(2);

		reloadDeferred.resolve({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout: initialLayout,
		});

		await waitFor(() => {
			expect(screen.getAllByTestId('mock-terminal-pane')).toHaveLength(2);
		});
	});

	it('does not reinitialize the same workspace after the initial layout load settles', async () => {
		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout.mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout: buildTerminalLayout('term-1'),
		});
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchTerminalServiceStatus.mockResolvedValue({ available: true });

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				active: true,
			},
		});

		await screen.findByRole('tab', { name: /workspace-0/i });

		await waitFor(() => {
			expect(terminalLayoutApiMocks.fetchWorkspaceTerminalLayout).toHaveBeenCalledTimes(1);
		});
		expect(screen.queryByText('Starting thread terminals…')).not.toBeInTheDocument();
	});

	it('does not bootstrap stale terminal ids after switching workspaces', async () => {
		const workspaceTwoDeferred = createDeferred<{
			workspaceId: string;
			workspacePath: string;
			layout: ReturnType<typeof buildTerminalLayout>;
		}>();

		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout
			.mockResolvedValueOnce({
				workspaceId: 'ws-1',
				workspacePath: '/tmp/ws-1',
				layout: buildTerminalLayout('term-1'),
			})
			.mockImplementationOnce(() => workspaceTwoDeferred.promise);
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchTerminalServiceStatus.mockResolvedValue({ available: true });

		const view = render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace One',
				active: true,
			},
		});

		await screen.findByRole('tab', { name: /workspace-0/i });

		await view.rerender({
			workspaceId: 'ws-2',
			workspaceName: 'Workspace Two',
			active: true,
		});

		expect(await screen.findByText('Starting thread terminals…')).toBeInTheDocument();
		await waitFor(() => {
			expect(terminalLayoutApiMocks.fetchWorkspaceTerminalLayout).toHaveBeenCalledWith('ws-2');
		});

		workspaceTwoDeferred.resolve({
			workspaceId: 'ws-2',
			workspacePath: '/tmp/ws-2',
			layout: buildTerminalLayout('term-2', 'Workspace-1'),
		});

		await waitFor(() => {
			expect(screen.getByRole('tab', { name: /workspace-1/i })).toBeInTheDocument();
		});
	});

	it('passes stored pane snapshots through to terminal panes and persists fresh snapshots on workspace switch', async () => {
		const storedSnapshot = {
			version: 1,
			nextOffset: 12,
			cols: 80,
			rows: 24,
			activeBuffer: 'normal' as const,
			normalViewportY: 0,
			cursor: { x: 0, y: 0, visible: true },
			modes: { dec: [], ansi: [] },
			normalTail: ['stored'],
			normalScreen: ['stored'],
		};
		const freshSnapshot = {
			...storedSnapshot,
			nextOffset: 24,
			normalTail: ['fresh'],
			normalScreen: ['fresh'],
		};

		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout
			.mockResolvedValueOnce({
				workspaceId: 'ws-1',
				workspacePath: '/tmp/ws-1',
				layout: {
					version: 3,
					tabs: [
						{
							id: 'tab-1',
							title: 'Workspace-0',
							panes: [{ id: 'pane-1', terminalId: 'term-1', snapshot: storedSnapshot }],
							focusedPaneId: 'pane-1',
						},
					],
					activeTabId: 'tab-1',
				},
			})
			.mockResolvedValueOnce({
				workspaceId: 'ws-2',
				workspacePath: '/tmp/ws-2',
				layout: buildTerminalLayout('term-2', 'Workspace-1'),
			});
		terminalServiceMocks.captureTerminalSnapshot.mockReturnValue(freshSnapshot);
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchTerminalServiceStatus.mockResolvedValue({ available: true });

		const view = render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace One',
				active: true,
			},
		});

		const pane = await screen.findByTestId('mock-terminal-pane');
		expect(pane).toHaveAttribute('data-has-snapshot', 'true');

		await view.rerender({
			workspaceId: 'ws-2',
			workspaceName: 'Workspace Two',
			active: true,
		});

		await waitFor(() => {
			expect(terminalLayoutApiMocks.persistWorkspaceTerminalLayout).toHaveBeenCalledWith(
				'ws-1',
				expect.objectContaining({
					version: 4,
					tabs: [
						expect.objectContaining({
							root: expect.objectContaining({
								kind: 'pane',
								terminalId: 'term-1',
								snapshot: freshSnapshot,
							}),
						}),
					],
				}),
			);
		});
	});

	it('captures the outgoing tab snapshot before switching tabs', async () => {
		const capturedSnapshot = {
			version: 1,
			nextOffset: 44,
			cols: 80,
			rows: 24,
			activeBuffer: 'alternate' as const,
			normalViewportY: 0,
			cursor: { x: 4, y: 8, visible: true },
			modes: { dec: [1049], ansi: [] },
			normalTail: ['prompt'],
			alternateScreen: ['full-screen'],
		};

		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout.mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout: {
				version: 3,
				tabs: [
					{
						id: 'tab-1',
						title: 'Workspace-0',
						panes: [{ id: 'pane-1', terminalId: 'term-1' }],
						focusedPaneId: 'pane-1',
					},
					{
						id: 'tab-2',
						title: 'Workspace-1',
						panes: [{ id: 'pane-2', terminalId: 'term-2' }],
						focusedPaneId: 'pane-2',
					},
				],
				activeTabId: 'tab-1',
			},
		});
		terminalServiceMocks.captureTerminalSnapshot.mockReturnValue(capturedSnapshot);
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchTerminalServiceStatus.mockResolvedValue({ available: true });

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				active: true,
			},
		});

		const initialPane = await screen.findByTestId('mock-terminal-pane');
		expect(initialPane).toHaveTextContent('term-1');
		expect(initialPane).toHaveAttribute('data-has-snapshot', 'false');

		await fireEvent.click(screen.getByRole('tab', { name: /workspace-1/i }));
		await waitFor(() => {
			expect(screen.getByTestId('mock-terminal-pane')).toHaveTextContent('term-2');
		});
		expect(screen.getByTestId('mock-terminal-pane')).toHaveAttribute('data-has-snapshot', 'false');

		await fireEvent.click(screen.getByRole('tab', { name: /workspace-0/i }));
		await waitFor(() => {
			expect(screen.getByTestId('mock-terminal-pane')).toHaveTextContent('term-1');
		});
		expect(screen.getByTestId('mock-terminal-pane')).toHaveAttribute('data-has-snapshot', 'true');
	});

	it('collapses a split when the focused pane is closed', async () => {
		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout.mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout: {
				version: 3,
				tabs: [
					{
						id: 'tab-1',
						title: 'Workspace-0',
						panes: [
							{ id: 'pane-1', terminalId: 'term-1' },
							{ id: 'pane-2', terminalId: 'term-2' },
						],
						splitDirection: 'vertical',
						splitRatio: 0.5,
						focusedPaneId: 'pane-1',
					},
				],
				activeTabId: 'tab-1',
			},
		});
		terminalLayoutApiMocks.fetchTerminalBootstrap.mockImplementation(
			async (_workspaceId, terminalId) => buildTerminalDescriptor('ws-1', String(terminalId)),
		);
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchTerminalServiceStatus.mockResolvedValue({ available: true });

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				active: true,
			},
		});

		await screen.findByRole('tab', { name: /workspace-0/i });

		const closePane = await screen.findByRole('button', { name: /close pane/i });
		await fireEvent.click(closePane);

		await waitFor(() => {
			expect(terminalServiceMocks.closeTerminal).toHaveBeenCalledWith('ws-1', 'term-1');
		});
		await waitFor(() => {
			expect(screen.getAllByTestId('mock-terminal-pane')).toHaveLength(1);
		});
		expect(screen.getByTestId('mock-terminal-pane')).toHaveTextContent('term-2');
	});

	it('creates a new terminal when splitting and the split is stored as a tree', async () => {
		vi.useFakeTimers();
		terminalLayoutApiMocks.fetchWorkspaceTerminalLayout.mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout: buildTerminalLayout('term-1'),
		});
		terminalLayoutApiMocks.fetchTerminalBootstrap.mockImplementation(
			async (_workspaceId, terminalId) => buildTerminalDescriptor('ws-1', String(terminalId)),
		);
		terminalLayoutApiMocks.createWorkspaceTerminal.mockResolvedValue({
			workspaceId: 'ws-1',
			terminalId: 'term-2',
		});
		settingsApiMocks.fetchSettings.mockResolvedValue({});
		settingsApiMocks.fetchTerminalServiceStatus.mockResolvedValue({ available: true });

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'Workspace',
				active: true,
			},
		});

		await screen.findByRole('tab', { name: /workspace-0/i });

		await fireEvent.click(screen.getByRole('button', { name: /split vertical/i }));
		await vi.runAllTimersAsync();

		expect(terminalLayoutApiMocks.createWorkspaceTerminal).toHaveBeenCalledTimes(1);
		const lastPersistCall = terminalLayoutApiMocks.persistWorkspaceTerminalLayout.mock.calls.at(-1);
		expect(lastPersistCall?.[0]).toBe('ws-1');
		const persisted = lastPersistCall?.[1];
		expect(persisted.version).toBe(4);
		expect(persisted.tabs[0].root.kind).toBe('split');
		expect(persisted.tabs[0].root.direction).toBe('vertical');
		expect(persisted.tabs[0].root.first.terminalId).toBe('term-1');
		expect(persisted.tabs[0].root.second.terminalId).toBe('term-2');
	});
});
