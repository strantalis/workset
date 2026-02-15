import { Browser } from '@wailsio/runtime';
import {
	ResizeWorkspaceTerminalForWindowName,
	StartWorkspaceTerminalForWindowName,
	WriteWorkspaceTerminalForWindowName,
} from '../../../bindings/workset/app';
import { fetchSessiondStatus, fetchSettings, type SessiondStatusResponse } from '../api/settings';
import { logTerminalDebug, stopWorkspaceTerminal } from '../api/terminal-layout';
import type { SettingsSnapshot } from '../types';
import { getCurrentWindowName } from '../windowContext';
import { subscribeWailsEvent } from '../wailsEventRegistry';

export type TerminalTransport = {
	onEvent: <T>(event: string, handler: (payload: T) => void) => () => void;
	start: (workspaceId: string, terminalId: string) => Promise<void>;
	write: (workspaceId: string, terminalId: string, data: string) => Promise<void>;
	resize: (workspaceId: string, terminalId: string, cols: number, rows: number) => Promise<void>;
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
	stop: stopWorkspaceTerminal,
	fetchSessiondStatus,
	fetchSettings,
	logDebug: logTerminalDebug,
	openURL: async (url) => {
		await Browser.OpenURL(url);
	},
};
