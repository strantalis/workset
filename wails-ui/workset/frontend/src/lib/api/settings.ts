import type { AgentCLIStatus, EnvSnapshotResult, RegisteredRepo, SettingsSnapshot } from '../types';
import { setTerminalDebugLogPreference } from './terminal-layout';
import {
	CheckAgentStatus,
	GetSettings,
	GetTerminalServiceStatus,
	ListRegisteredRepos,
	OpenDirectoryDialog,
	OpenFileDialog,
	RegisterRepo,
	ReloadLoginEnv,
	SetAgentCLIPath,
	SetDefaultSetting,
	UnregisterRepo,
	UpdateRegisteredRepo,
} from '../../../bindings/workset/app';

export type TerminalServiceStatusResponse = {
	available: boolean;
	error?: string;
};

export async function reloadLoginEnv(): Promise<EnvSnapshotResult> {
	return (await ReloadLoginEnv()) as EnvSnapshotResult;
}

export async function checkAgentStatus(agent: string): Promise<AgentCLIStatus> {
	return (await CheckAgentStatus({ agent })) as AgentCLIStatus;
}

export async function setAgentCLIPath(agent: string, path: string): Promise<AgentCLIStatus> {
	return (await SetAgentCLIPath({ agent, path })) as AgentCLIStatus;
}

export async function openFileDialog(title: string, defaultDirectory: string): Promise<string> {
	return (await OpenFileDialog(title, defaultDirectory)) as string;
}

export async function openDirectoryDialog(
	title: string,
	defaultDirectory: string,
): Promise<string> {
	return OpenDirectoryDialog(title, defaultDirectory);
}

export async function fetchTerminalServiceStatus(): Promise<TerminalServiceStatusResponse> {
	return GetTerminalServiceStatus();
}

export async function listRegisteredRepos(): Promise<RegisteredRepo[]> {
	return ListRegisteredRepos();
}

export async function registerRepo(
	name: string,
	source: string,
	remote: string,
	defaultBranch: string,
): Promise<void> {
	await RegisterRepo({
		name,
		source,
		remote,
		defaultBranch,
	} as Parameters<typeof RegisterRepo>[0]);
}

export async function updateRegisteredRepo(
	name: string,
	source: string,
	remote: string,
	defaultBranch: string,
): Promise<void> {
	await UpdateRegisteredRepo({
		name,
		source,
		remote,
		defaultBranch,
	} as Parameters<typeof UpdateRegisteredRepo>[0]);
}

export async function unregisterRepo(name: string): Promise<void> {
	await UnregisterRepo(name);
}

export async function fetchSettings(): Promise<SettingsSnapshot> {
	const settings = (await GetSettings()) as unknown as SettingsSnapshot;
	setTerminalDebugLogPreference(
		settings?.defaults?.terminalDebugLog === 'on' || settings?.defaults?.terminalDebugLog === 'off'
			? settings.defaults.terminalDebugLog
			: '',
	);
	return settings;
}

export async function setDefaultSetting(key: string, value: string): Promise<void> {
	await SetDefaultSetting(key, value);
	if (key === 'defaults.terminal_debug_log') {
		setTerminalDebugLogPreference(value === 'on' || value === 'off' ? value : '');
	}
}
