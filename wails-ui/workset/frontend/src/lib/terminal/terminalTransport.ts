import { Browser } from '@wailsio/runtime';
import type { TerminalSessionDescriptor } from '../../../bindings/workset/models';
import { fetchSessiondStatus, fetchSettings, type SessiondStatusResponse } from '../api/settings';
import {
	fetchTerminalBootstrap,
	logTerminalDebug,
	stopWorkspaceTerminal,
} from '../api/terminal-layout';
import type { SettingsSnapshot } from '../types';

const shouldLogTransportDebug = (): boolean => {
	if (typeof localStorage === 'undefined') return false;
	try {
		return localStorage.getItem('worksetTerminalDebug') === '1';
	} catch {
		return false;
	}
};

const logWindowResolution = async (
	workspaceId: string,
	terminalId: string,
	event: string,
	details: Record<string, unknown>,
): Promise<void> => {
	if (!shouldLogTransportDebug()) return;
	try {
		await logTerminalDebug(workspaceId, terminalId, event, JSON.stringify(details));
	} catch {
		// Ignore debug logging failures.
	}
};

export type TerminalSessionStartResult = TerminalSessionDescriptor;

export type TerminalTransport = {
	start: (workspaceId: string, terminalId: string) => Promise<TerminalSessionStartResult>;
	stop: (workspaceId: string, terminalId: string) => Promise<void>;
	fetchSessiondStatus: () => Promise<SessiondStatusResponse>;
	fetchSettings: () => Promise<SettingsSnapshot>;
	logDebug: (
		workspaceId: string,
		terminalId: string,
		event: string,
		details: string,
	) => Promise<void>;
	openURL: (url: string) => Promise<void>;
};

export const terminalTransport: TerminalTransport = {
	start: async (workspaceId, terminalId) => {
		await logWindowResolution(workspaceId, terminalId, 'transport_start_request', {
			source: 'sessiond-bootstrap',
		});
		const descriptor = await fetchTerminalBootstrap(workspaceId, terminalId);
		await logWindowResolution(workspaceId, terminalId, 'transport_start_descriptor', {
			windowName: descriptor.windowName ?? '',
			sessionId: descriptor.sessionId,
			owner: descriptor.owner ?? '',
			canWrite: descriptor.canWrite,
			running: descriptor.running,
			currentOffset: descriptor.currentOffset,
			transport: descriptor.transport,
		});
		return descriptor;
	},
	stop: stopWorkspaceTerminal,
	fetchSessiondStatus,
	fetchSettings,
	logDebug: logTerminalDebug,
	openURL: async (url) => {
		await Browser.OpenURL(url);
	},
};
