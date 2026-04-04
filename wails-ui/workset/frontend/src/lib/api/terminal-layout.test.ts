import { beforeEach, describe, expect, test, vi } from 'vitest';

type MockStorage = {
	getItem: (key: string) => string | null;
	setItem: (key: string, value: string) => void;
	removeItem: (key: string) => void;
	clear: () => void;
};

const localStorageMock = vi.hoisted<MockStorage>(() => {
	const store = new Map<string, string>();
	return {
		getItem: (key: string) => store.get(key) ?? null,
		setItem: (key: string, value: string) => {
			store.set(key, value);
		},
		removeItem: (key: string) => {
			store.delete(key);
		},
		clear: () => {
			store.clear();
		},
	};
});

Object.defineProperty(globalThis, 'localStorage', {
	value: localStorageMock,
	configurable: true,
});

const makeCancellablePromise = <T>(value: T) => {
	const promise = Promise.resolve(value);
	return {
		then: promise.then.bind(promise),
		catch: promise.catch.bind(promise),
		finally: promise.finally.bind(promise),
		cancel: vi.fn(),
		cancelOn: vi.fn(),
	} as unknown as Promise<T>;
};

const bindingsMocks = vi.hoisted(() => ({
	LogTerminalDebug: vi.fn(() => makeCancellablePromise(undefined)),
	GetSettings: vi.fn(() => makeCancellablePromise({ defaults: {} })),
	SetDefaultSetting: vi.fn(() => makeCancellablePromise(undefined)),
}));

vi.mock('../../../bindings/workset/app', () => ({
	CreateWorkspaceTerminal: vi.fn(),
	GetWorkspaceTerminalLayout: vi.fn(),
	LogTerminalDebug: bindingsMocks.LogTerminalDebug,
	SetWorkspaceTerminalLayout: vi.fn(),
	StartWorkspaceTerminalSessionForWindow: vi.fn(),
	StopWorkspaceTerminalForWindow: vi.fn(),
	GetSettings: bindingsMocks.GetSettings,
	SetDefaultSetting: bindingsMocks.SetDefaultSetting,
}));

import { fetchSettings, setDefaultSetting } from './settings';
import { logTerminalDebug, setTerminalDebugLogPreference } from './terminal-layout';

describe('terminal debug logging gate', () => {
	beforeEach(() => {
		localStorageMock.clear();
		setTerminalDebugLogPreference('');
		bindingsMocks.LogTerminalDebug.mockClear();
		bindingsMocks.GetSettings.mockReset();
		bindingsMocks.SetDefaultSetting.mockReset();
		bindingsMocks.GetSettings.mockImplementation(() => makeCancellablePromise({ defaults: {} }));
		bindingsMocks.SetDefaultSetting.mockImplementation(() => makeCancellablePromise(undefined));
	});

	test('skips bridge logging when lifecycle debug is disabled', async () => {
		await logTerminalDebug('ws-1', 'term-1', 'event', '{}');

		expect(bindingsMocks.LogTerminalDebug).not.toHaveBeenCalled();
	});

	test('sends bridge logging when lifecycle debug is enabled', async () => {
		setTerminalDebugLogPreference('on');

		await logTerminalDebug('ws-1', 'term-1', 'event', '{}');

		expect(bindingsMocks.LogTerminalDebug).toHaveBeenCalledWith({
			workspaceId: 'ws-1',
			terminalId: 'term-1',
			event: 'event',
			details: '{}',
		});
	});

	test('mirrors fetched settings into the lifecycle debug preference', async () => {
		bindingsMocks.GetSettings.mockImplementation(() =>
			makeCancellablePromise({
				defaults: {
					terminalDebugLog: 'on',
				},
			}),
		);

		await fetchSettings();
		await logTerminalDebug('ws-1', 'term-1', 'event', '{}');

		expect(bindingsMocks.LogTerminalDebug).toHaveBeenCalledTimes(1);
	});

	test('mirrors setting updates into the lifecycle debug preference', async () => {
		await setDefaultSetting('defaults.terminal_debug_log', 'on');
		await logTerminalDebug('ws-1', 'term-1', 'event', '{}');

		expect(bindingsMocks.SetDefaultSetting).toHaveBeenCalledWith(
			'defaults.terminal_debug_log',
			'on',
		);
		expect(bindingsMocks.LogTerminalDebug).toHaveBeenCalledTimes(1);

		bindingsMocks.LogTerminalDebug.mockClear();

		await setDefaultSetting('defaults.terminal_debug_log', 'off');
		await logTerminalDebug('ws-1', 'term-1', 'event', '{}');

		expect(bindingsMocks.SetDefaultSetting).toHaveBeenCalledWith(
			'defaults.terminal_debug_log',
			'off',
		);
		expect(bindingsMocks.LogTerminalDebug).not.toHaveBeenCalled();
	});
});
