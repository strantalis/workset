import { Browser } from '@wailsio/runtime';
import {
	AckWorkspaceTerminalForWindowName,
	ResizeWorkspaceTerminalForWindowName,
	StartWorkspaceTerminalForWindowName,
	WriteWorkspaceTerminalForWindowName,
} from '../../../bindings/workset/app';
import { fetchSessiondStatus, fetchSettings, type SessiondStatusResponse } from '../api/settings';
import {
	fetchTerminalBootstrap,
	fetchWorkspaceTerminalStatus,
	logTerminalDebug,
	stopWorkspaceTerminal,
	type TerminalBootstrapResponse,
	type WorkspaceTerminalStatusResponse,
} from '../api/terminal-layout';
import type { SettingsSnapshot } from '../types';
import { getCurrentWindowName } from '../windowContext';
import { subscribeWailsEvent } from '../wailsEventRegistry';

export type TerminalTransport = {
	onEvent: <T>(event: string, handler: (payload: T) => void) => () => void;
	start: (workspaceId: string, terminalId: string) => Promise<void>;
	write: (workspaceId: string, terminalId: string, data: string) => Promise<void>;
	resize: (workspaceId: string, terminalId: string, cols: number, rows: number) => Promise<void>;
	ack: (workspaceId: string, terminalId: string, bytes: number) => Promise<void>;
	stop: (workspaceId: string, terminalId: string) => Promise<void>;
	fetchStatus: (
		workspaceId: string,
		terminalId: string,
	) => Promise<WorkspaceTerminalStatusResponse>;
	fetchBootstrap: (workspaceId: string, terminalId: string) => Promise<TerminalBootstrapResponse>;
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
	onEvent: (event, handler) => subscribeWailsEvent(event, handler),
	start: async (workspaceId, terminalId) => {
		const windowName = await getCurrentWindowName();
		await StartWorkspaceTerminalForWindowName(workspaceId, terminalId, windowName);
	},
	write: async (workspaceId, terminalId, data) => {
		const windowName = await getCurrentWindowName();
		await WriteWorkspaceTerminalForWindowName(workspaceId, terminalId, data, windowName);
	},
	resize: async (workspaceId, terminalId, cols, rows) => {
		const windowName = await getCurrentWindowName();
		await ResizeWorkspaceTerminalForWindowName(workspaceId, terminalId, cols, rows, windowName);
	},
	ack: async (workspaceId, terminalId, bytes) => {
		const windowName = await getCurrentWindowName();
		await AckWorkspaceTerminalForWindowName(workspaceId, terminalId, bytes, windowName);
	},
	stop: stopWorkspaceTerminal,
	fetchStatus: fetchWorkspaceTerminalStatus,
	fetchBootstrap: fetchTerminalBootstrap,
	fetchSessiondStatus,
	fetchSettings,
	logDebug: logTerminalDebug,
	openURL: async (url) => {
		await Browser.OpenURL(url);
	},
};
