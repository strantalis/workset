import {
	checkForUpdates as checkForUpdatesApi,
	fetchAppVersion as fetchAppVersionApi,
	fetchUpdatePreferences as fetchUpdatePreferencesApi,
	fetchUpdateState as fetchUpdateStateApi,
	setUpdatePreferences as setUpdatePreferencesApi,
	startAppUpdate as startAppUpdateApi,
} from '../../api/updates';
import {
	createWorkspaceTerminal as createWorkspaceTerminalApi,
	fetchWorkspaceTerminalLayout as fetchWorkspaceTerminalLayoutApi,
	persistWorkspaceTerminalLayout as persistWorkspaceTerminalLayoutApi,
	stopWorkspaceTerminal as stopWorkspaceTerminalApi,
} from '../../api/terminal-layout';
import {
	restartSessiond as restartSessiondApi,
	type SessiondStatusResponse,
} from '../../api/settings';
import { toErrorMessage as toErrorMessageApi } from '../../errors';
import { generateTerminalName as generateTerminalNameApi } from '../../names';
import type {
	AppVersion,
	TerminalLayout,
	TerminalLayoutNode,
	UpdateCheckResult,
	UpdatePreferences,
	UpdateState,
	Workspace,
} from '../../types';

const SESSIOND_RESTART_TIMEOUT_MS = 20000;
const LAYOUT_VERSION = 1;

export const DEFAULT_UPDATE_PREFERENCES: UpdatePreferences = { channel: 'stable', autoCheck: true };

export type SettingsPanelActionResult = {
	success?: string;
	error?: string;
};

export type UpdateChannelResult = {
	updatePreferences?: UpdatePreferences;
	error?: string;
};

export type UpdateCheckActionResult = {
	updateCheck?: UpdateCheckResult;
	updateState?: UpdateState;
	error?: string;
};

export type StartUpdateActionResult = {
	updateState?: UpdateState;
	error?: string;
};

export type UpdateBootstrapResult = {
	updatePreferences: UpdatePreferences;
	updateState: UpdateState | null;
};

export type SettingsPanelSideEffects = {
	restartSessiond: () => Promise<SettingsPanelActionResult>;
	resetTerminalLayout: (workspace: Workspace) => Promise<SettingsPanelActionResult>;
	setUpdateChannel: (channel: string) => Promise<UpdateChannelResult>;
	checkForUpdates: (channel: UpdatePreferences['channel']) => Promise<UpdateCheckActionResult>;
	startUpdate: (channel: UpdatePreferences['channel']) => Promise<StartUpdateActionResult>;
	loadAppVersion: () => Promise<AppVersion | null>;
	loadUpdateBootstrap: () => Promise<UpdateBootstrapResult>;
};

export type SettingsPanelSideEffectDeps = {
	restartSessiond: (reason?: string) => Promise<SessiondStatusResponse>;
	setTimeout: (handler: () => void, timeoutMs: number) => number;
	fetchWorkspaceTerminalLayout: (
		workspaceId: string,
	) => Promise<{ workspaceId: string; workspacePath: string; layout?: TerminalLayout }>;
	createWorkspaceTerminal: (
		workspaceId: string,
	) => Promise<{ workspaceId: string; terminalId: string }>;
	persistWorkspaceTerminalLayout: (workspaceId: string, layout: TerminalLayout) => Promise<void>;
	stopWorkspaceTerminal: (workspaceId: string, terminalId: string) => Promise<void>;
	generateTerminalName: (workspaceName: string, index: number) => string;
	checkForUpdates: (channel?: string) => Promise<UpdateCheckResult>;
	fetchUpdateState: () => Promise<UpdateState>;
	setUpdatePreferences: (
		input: Partial<UpdatePreferences> & { channel?: string },
	) => Promise<UpdatePreferences>;
	startAppUpdate: (channel?: string) => Promise<{ state: UpdateState }>;
	fetchAppVersion: () => Promise<AppVersion>;
	fetchUpdatePreferences: () => Promise<UpdatePreferences>;
	toErrorMessage: (error: unknown, fallback: string) => string;
	dispatchLayoutReset: (workspaceId: string) => void;
	randomUUID?: () => string;
};

const defaultDispatchLayoutReset = (workspaceId: string): void => {
	if (typeof window === 'undefined' || typeof CustomEvent === 'undefined') {
		return;
	}
	window.dispatchEvent(
		new CustomEvent('workset:terminal-layout-reset', {
			detail: { workspaceId },
		}),
	);
};

const resolveId = (randomUUID?: () => string): string => {
	if (randomUUID) {
		return randomUUID();
	}
	if (typeof crypto !== 'undefined' && crypto.randomUUID) {
		return crypto.randomUUID();
	}
	return `term-${Math.random().toString(36).slice(2)}`;
};

export const collectTerminalIds = (node: TerminalLayoutNode | null | undefined): string[] => {
	if (!node) {
		return [];
	}
	if (node.kind === 'pane') {
		return (node.tabs ?? []).map((tab) => tab.terminalId).filter(Boolean);
	}
	return [...collectTerminalIds(node.first), ...collectTerminalIds(node.second)];
};

const stopSessionsForLayout = async (
	workspaceId: string,
	layout: TerminalLayout | null,
	stopWorkspaceTerminal: (workspaceId: string, terminalId: string) => Promise<void>,
): Promise<void> => {
	if (!layout) {
		return;
	}
	const terminalIds = Array.from(new Set(collectTerminalIds(layout.root)));
	if (terminalIds.length === 0) {
		return;
	}
	await Promise.allSettled(
		terminalIds.map((terminalId) => stopWorkspaceTerminal(workspaceId, terminalId)),
	);
};

export const buildFreshLayout = (
	workspaceName: string,
	terminalId: string,
	generateTerminalName: (workspaceName: string, index: number) => string,
	nextId: () => string,
): TerminalLayout => {
	const tabId = nextId();
	const paneId = nextId();
	return {
		version: LAYOUT_VERSION,
		root: {
			id: paneId,
			kind: 'pane',
			tabs: [
				{
					id: tabId,
					terminalId,
					title: generateTerminalName(workspaceName, 0),
				},
			],
			activeTabId: tabId,
		},
		focusedPaneId: paneId,
	};
};

export const createSettingsPanelSideEffects = (
	overrides: Partial<SettingsPanelSideEffectDeps> = {},
): SettingsPanelSideEffects => {
	const deps: SettingsPanelSideEffectDeps = {
		restartSessiond: restartSessiondApi,
		setTimeout: (handler, timeoutMs) => window.setTimeout(handler, timeoutMs),
		fetchWorkspaceTerminalLayout: fetchWorkspaceTerminalLayoutApi,
		createWorkspaceTerminal: createWorkspaceTerminalApi,
		persistWorkspaceTerminalLayout: persistWorkspaceTerminalLayoutApi,
		stopWorkspaceTerminal: stopWorkspaceTerminalApi,
		generateTerminalName: generateTerminalNameApi,
		checkForUpdates: checkForUpdatesApi,
		fetchUpdateState: fetchUpdateStateApi,
		setUpdatePreferences: setUpdatePreferencesApi,
		startAppUpdate: startAppUpdateApi,
		fetchAppVersion: fetchAppVersionApi,
		fetchUpdatePreferences: fetchUpdatePreferencesApi,
		toErrorMessage: toErrorMessageApi,
		dispatchLayoutReset: defaultDispatchLayoutReset,
		...overrides,
	};

	return {
		restartSessiond: async () => {
			try {
				const status = await Promise.race([
					deps.restartSessiond('settings_panel'),
					new Promise<SessiondStatusResponse>((_, reject) => {
						deps.setTimeout(() => {
							reject(new Error('Session daemon restart timed out.'));
						}, SESSIOND_RESTART_TIMEOUT_MS);
					}),
				]);
				if (status?.available) {
					return {
						success: status.warning
							? `Session daemon restarted. ${status.warning}`
							: 'Session daemon restarted.',
					};
				}
				const warning = status?.warning ? ` ${status.warning}` : '';
				return {
					error: status?.error
						? `Failed to restart: ${status.error}${warning}`
						: `Failed to restart session daemon.${warning}`,
				};
			} catch (error) {
				return {
					error: `Failed to restart: ${deps.toErrorMessage(error, 'Failed to update settings.')}`,
				};
			}
		},

		resetTerminalLayout: async (workspace) => {
			try {
				let layoutToStop: TerminalLayout | null = null;
				try {
					const payload = await deps.fetchWorkspaceTerminalLayout(workspace.id);
					layoutToStop = payload?.layout ?? null;
				} catch {
					layoutToStop = null;
				}

				await stopSessionsForLayout(workspace.id, layoutToStop, deps.stopWorkspaceTerminal);
				const created = await deps.createWorkspaceTerminal(workspace.id);
				const layout = buildFreshLayout(
					workspace.name,
					created.terminalId,
					deps.generateTerminalName,
					() => resolveId(deps.randomUUID),
				);

				await deps.persistWorkspaceTerminalLayout(workspace.id, layout);
				deps.dispatchLayoutReset(workspace.id);

				return { success: `Terminal layout reset for ${workspace.name}.` };
			} catch (error) {
				return {
					error: `Failed to reset terminal layout: ${deps.toErrorMessage(error, 'Failed to update settings.')}`,
				};
			}
		},

		setUpdateChannel: async (channel) => {
			const nextChannel = channel === 'alpha' ? 'alpha' : 'stable';
			try {
				return {
					updatePreferences: await deps.setUpdatePreferences({ channel: nextChannel }),
				};
			} catch (error) {
				return {
					error: deps.toErrorMessage(error, 'Failed to update channel preference.'),
				};
			}
		},

		checkForUpdates: async (channel) => {
			try {
				const updateCheck = await deps.checkForUpdates(channel);
				const updateState = await deps.fetchUpdateState();
				return { updateCheck, updateState };
			} catch (error) {
				return {
					error: deps.toErrorMessage(error, 'Failed to check for updates.'),
				};
			}
		},

		startUpdate: async (channel) => {
			try {
				const result = await deps.startAppUpdate(channel);
				return { updateState: result.state };
			} catch (error) {
				return {
					error: deps.toErrorMessage(error, 'Failed to start update.'),
				};
			}
		},

		loadAppVersion: async () => {
			try {
				return await deps.fetchAppVersion();
			} catch {
				return null;
			}
		},

		loadUpdateBootstrap: async () => {
			let updatePreferences = DEFAULT_UPDATE_PREFERENCES;
			try {
				updatePreferences = await deps.fetchUpdatePreferences();
			} catch {
				updatePreferences = DEFAULT_UPDATE_PREFERENCES;
			}

			let updateState: UpdateState | null = null;
			try {
				updateState = await deps.fetchUpdateState();
			} catch {
				updateState = null;
			}

			return { updatePreferences, updateState };
		},
	};
};
