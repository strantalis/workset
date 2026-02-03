/**
 * @vitest-environment jsdom
 */
import { beforeEach, describe, expect, test, vi } from 'vitest';
import { render, waitFor, cleanup } from '@testing-library/svelte';
import TerminalWorkspace from './TerminalWorkspace.svelte';
import * as api from '../api';
import type { TerminalLayout } from '../types';

vi.mock('../api', () => ({
	createWorkspaceTerminal: vi.fn(),
	fetchWorkspaceTerminalStatus: vi.fn(),
	fetchWorkspaceTerminalLayout: vi.fn(),
	persistWorkspaceTerminalLayout: vi.fn(),
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

		vi.mocked(api.fetchWorkspaceTerminalLayout).mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout,
		});
		vi.mocked(api.fetchWorkspaceTerminalStatus).mockResolvedValue({
			workspaceId: 'ws-1',
			terminalId: 'term-legacy',
			active: false,
		});
		vi.mocked(api.createWorkspaceTerminal).mockResolvedValue({
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
			expect(api.persistWorkspaceTerminalLayout).toHaveBeenCalledTimes(1);
		});

		expect(api.fetchWorkspaceTerminalStatus).toHaveBeenCalledWith('ws-1', 'term-legacy');
		expect(api.createWorkspaceTerminal).toHaveBeenCalledTimes(1);
		expect(api.persistWorkspaceTerminalLayout).toHaveBeenCalledWith(
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

		vi.mocked(api.fetchWorkspaceTerminalLayout).mockResolvedValue({
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
			expect(api.fetchWorkspaceTerminalLayout).toHaveBeenCalledTimes(2);
		});
		expect(api.createWorkspaceTerminal).toHaveBeenCalledTimes(1);
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

		vi.mocked(api.fetchWorkspaceTerminalLayout).mockResolvedValue({
			workspaceId: 'ws-1',
			workspacePath: '/tmp/ws-1',
			layout,
		});
		vi.mocked(api.fetchWorkspaceTerminalStatus).mockResolvedValue({
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
			expect(api.fetchWorkspaceTerminalStatus).toHaveBeenCalledTimes(1);
		});

		expect(api.createWorkspaceTerminal).not.toHaveBeenCalled();
		expect(api.persistWorkspaceTerminalLayout).not.toHaveBeenCalled();
	});
});
