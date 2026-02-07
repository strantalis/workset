/**
 * @vitest-environment jsdom
 */
import { beforeEach, describe, expect, test, vi } from 'vitest';
import { render, waitFor, cleanup } from '@testing-library/svelte';
import TerminalWorkspace from './TerminalWorkspace.svelte';
import * as terminalApi from '../api/terminal-layout';
import type { TerminalLayout } from '../types';

vi.mock('../api/terminal-layout', () => ({
	createWorkspaceTerminal: vi.fn(),
	fetchWorkspaceTerminalStatus: vi.fn(),
	fetchWorkspaceTerminalLayout: vi.fn(),
	persistWorkspaceTerminalLayout: vi.fn(),
}));

vi.mock('../api/settings', () => ({
	fetchSettings: vi.fn().mockResolvedValue({
		defaults: {},
	}),
}));

vi.mock('../terminal/terminalService', async () => {
	const { writable } = await import('svelte/store');
	const store = writable({
		status: '',
		message: '',
		health: 'unknown',
		healthMessage: '',
		renderer: 'unknown',
		rendererMode: 'webgl',
		sessiondAvailable: null,
		sessiondChecked: false,
		debugEnabled: false,
		debugStats: {
			bytesIn: 0,
			bytesOut: 0,
			backlog: 0,
			lastOutputAt: 0,
			lastCprAt: 0,
		},
	});
	return {
		getTerminalStore: () => store,
		syncTerminal: vi.fn(),
		detachTerminal: vi.fn(),
		restartTerminal: vi.fn(),
		retryHealthCheck: vi.fn(),
		focusTerminalInstance: vi.fn(),
	};
});

describe('TerminalWorkspace migration', () => {
	const ensureLocalStorage = (): void => {
		if (typeof localStorage !== 'undefined' && typeof localStorage.clear === 'function') {
			localStorage.clear();
			return;
		}
		const store = new Map<string, string>();
		Object.defineProperty(globalThis, 'localStorage', {
			value: {
				getItem: (key: string) => store.get(key) ?? null,
				setItem: (key: string, value: string) => {
					store.set(key, String(value));
				},
				removeItem: (key: string) => {
					store.delete(key);
				},
				clear: () => {
					store.clear();
				},
			},
			configurable: true,
		});
	};

	beforeEach(() => {
		cleanup();
		ensureLocalStorage();
		vi.clearAllMocks();
	});

	test('migrates invalid terminal ids once and persists layout', async () => {
		const layout: TerminalLayout = {
			version: 1,
			root: {
				id: 'pane-1',
				kind: 'pane',
				tabs: [
					{
						id: 'tab-1',
						terminalId: 'term-legacy',
						title: 'Legacy',
					},
				],
				activeTabId: 'tab-1',
			},
			focusedPaneId: 'pane-1',
		};

		vi.mocked(terminalApi.fetchWorkspaceTerminalLayout).mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout,
		});
		vi.mocked(terminalApi.fetchWorkspaceTerminalStatus).mockResolvedValue({
			workspaceId: 'ws-1',
			terminalId: 'term-legacy',
			active: false,
		});
		vi.mocked(terminalApi.createWorkspaceTerminal).mockResolvedValue({
			workspaceId: 'ws-1',
			terminalId: 'term-new',
		});

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'demo',
				active: false,
			},
		});

		await waitFor(() => {
			expect(terminalApi.persistWorkspaceTerminalLayout).toHaveBeenCalledTimes(1);
		});

		expect(terminalApi.fetchWorkspaceTerminalStatus).toHaveBeenCalledWith('ws-1', 'term-legacy');
		expect(terminalApi.createWorkspaceTerminal).toHaveBeenCalledTimes(1);
		expect(terminalApi.persistWorkspaceTerminalLayout).toHaveBeenCalledWith(
			'ws-1',
			expect.objectContaining({
				root: expect.objectContaining({
					kind: 'pane',
					tabs: [
						expect.objectContaining({
							terminalId: 'term-new',
						}),
					],
				}),
			}),
		);
		expect(localStorage.getItem('workset:terminal-layout:migrated:v1:ws-1')).toBe('1');

		cleanup();

		vi.mocked(terminalApi.fetchWorkspaceTerminalLayout).mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout,
		});

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'demo',
				active: false,
			},
		});

		await waitFor(() => {
			expect(terminalApi.fetchWorkspaceTerminalLayout).toHaveBeenCalledTimes(2);
		});
		expect(terminalApi.createWorkspaceTerminal).toHaveBeenCalledTimes(1);
	});

	test('skips migration when status returns an error', async () => {
		const layout: TerminalLayout = {
			version: 1,
			root: {
				id: 'pane-1',
				kind: 'pane',
				tabs: [
					{
						id: 'tab-1',
						terminalId: 'term-legacy',
						title: 'Legacy',
					},
				],
				activeTabId: 'tab-1',
			},
			focusedPaneId: 'pane-1',
		};

		vi.mocked(terminalApi.fetchWorkspaceTerminalLayout).mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout,
		});
		vi.mocked(terminalApi.fetchWorkspaceTerminalStatus).mockResolvedValue({
			workspaceId: 'ws-1',
			terminalId: 'term-legacy',
			active: false,
			error: 'sessiond unavailable',
		});

		render(TerminalWorkspace, {
			props: {
				workspaceId: 'ws-1',
				workspaceName: 'demo',
				active: false,
			},
		});

		await waitFor(() => {
			expect(terminalApi.fetchWorkspaceTerminalStatus).toHaveBeenCalledTimes(1);
		});

		expect(terminalApi.createWorkspaceTerminal).not.toHaveBeenCalled();
		expect(terminalApi.persistWorkspaceTerminalLayout).not.toHaveBeenCalled();
	});
});
