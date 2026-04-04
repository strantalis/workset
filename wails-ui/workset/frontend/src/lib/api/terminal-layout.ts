import type { TerminalLayout, TerminalLayoutPayload } from '../types';
import type {
	TerminalLayout as BoundTerminalLayout,
	TerminalLayoutRequest as BoundTerminalLayoutRequest,
	TerminalSessionDescriptor,
} from '../../../bindings/workset/models';
import {
	CreateWorkspaceTerminal,
	GetWorkspaceTerminalLayout,
	LogTerminalDebug,
	SetWorkspaceTerminalLayout,
	StartWorkspaceTerminalSessionForWindow,
	StopWorkspaceTerminalForWindow,
} from '../../../bindings/workset/app';

type TerminalDebugPreference = 'on' | 'off' | '';

const TERMINAL_DEBUG_LOG_STORAGE_KEY = 'worksetTerminalLifecycleDebug';
const INTERACTIVE_DEV_LOGGING = import.meta.env.DEV && import.meta.env.MODE !== 'test';

const hasLocalStorage = (): boolean =>
	typeof localStorage !== 'undefined' &&
	typeof localStorage.getItem === 'function' &&
	typeof localStorage.setItem === 'function' &&
	typeof localStorage.removeItem === 'function';

let terminalDebugLogPreference: TerminalDebugPreference = (() => {
	if (!hasLocalStorage()) return '';
	const stored = localStorage.getItem(TERMINAL_DEBUG_LOG_STORAGE_KEY);
	return stored === 'on' || stored === 'off' ? stored : '';
})();

const isTerminalDebugLoggingEnabled = (): boolean => {
	if (INTERACTIVE_DEV_LOGGING) return true;
	if (terminalDebugLogPreference === 'on') return true;
	if (terminalDebugLogPreference === 'off') return false;
	if (!hasLocalStorage()) return false;
	return localStorage.getItem('worksetTerminalDebug') === '1';
};

export function setTerminalDebugLogPreference(value: TerminalDebugPreference): void {
	terminalDebugLogPreference = value;
	if (!hasLocalStorage()) return;
	if (value === 'on' || value === 'off') {
		localStorage.setItem(TERMINAL_DEBUG_LOG_STORAGE_KEY, value);
		return;
	}
	localStorage.removeItem(TERMINAL_DEBUG_LOG_STORAGE_KEY);
}

export async function logTerminalDebug(
	workspaceId: string,
	terminalId: string,
	event: string,
	details = '',
): Promise<void> {
	if (!isTerminalDebugLoggingEnabled()) return;
	await LogTerminalDebug({ workspaceId, terminalId, event, details });
}

export async function createWorkspaceTerminal(
	workspaceId: string,
): Promise<{ workspaceId: string; terminalId: string }> {
	return CreateWorkspaceTerminal(workspaceId);
}

export async function fetchTerminalBootstrap(
	workspaceId: string,
	terminalId: string,
): Promise<TerminalSessionDescriptor> {
	return StartWorkspaceTerminalSessionForWindow(workspaceId, terminalId);
}

export async function stopWorkspaceTerminal(
	workspaceId: string,
	terminalId: string,
): Promise<void> {
	await StopWorkspaceTerminalForWindow(workspaceId, terminalId);
}

export async function fetchWorkspaceTerminalLayout(
	workspaceId: string,
): Promise<TerminalLayoutPayload> {
	return (await GetWorkspaceTerminalLayout(workspaceId)) as TerminalLayoutPayload;
}

export async function persistWorkspaceTerminalLayout(
	workspaceId: string,
	layout: TerminalLayout,
): Promise<void> {
	await SetWorkspaceTerminalLayout({
		workspaceId,
		layout: layout as unknown as BoundTerminalLayout,
	} as unknown as BoundTerminalLayoutRequest);
}
