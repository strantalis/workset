import { Browser } from '@wailsio/runtime';
import type { TerminalSessionDescriptor } from '../../../bindings/workset/models';
import {
	fetchTerminalServiceStatus,
	fetchSettings,
	type TerminalServiceStatusResponse,
} from '../api/settings';
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

export type TerminalSessionStartResult = Pick<
	TerminalSessionDescriptor,
	'workspaceId' | 'terminalId' | 'sessionId' | 'socketUrl' | 'socketToken'
>;

export type TerminalTransport = {
	start: (workspaceId: string, terminalId: string) => Promise<TerminalSessionStartResult>;
	stop: (workspaceId: string, terminalId: string) => Promise<void>;
	fetchTerminalServiceStatus: () => Promise<TerminalServiceStatusResponse>;
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
			source: 'terminal-service-bootstrap',
		});
		const descriptor = await fetchTerminalBootstrap(workspaceId, terminalId);
		const result: TerminalSessionStartResult = {
			workspaceId: descriptor.workspaceId,
			terminalId: descriptor.terminalId,
			sessionId: descriptor.sessionId,
			socketUrl: descriptor.socketUrl,
			socketToken: descriptor.socketToken,
		};
		await logWindowResolution(workspaceId, terminalId, 'transport_start_descriptor', {
			sessionId: result.sessionId,
			socketUrl: result.socketUrl,
			socketTokenPresent: Boolean(result.socketToken),
		});
		return result;
	},
	stop: stopWorkspaceTerminal,
	fetchTerminalServiceStatus,
	fetchSettings,
	logDebug: logTerminalDebug,
	openURL: async (url) => {
		await Browser.OpenURL(url);
	},
};
