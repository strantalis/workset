import type {
	Alias,
	AgentCLIStatus,
	EnvSnapshotResult,
	Group,
	GroupSummary,
	SettingsSnapshot,
} from '../types';
import {
	AddGroupMember,
	ApplyGroup,
	CheckAgentStatus,
	CreateAlias,
	CreateGroup,
	DeleteAlias,
	DeleteGroup,
	GetGroup,
	GetSettings,
	GetSessiondStatus,
	ListAliases,
	ListGroups,
	OpenDirectoryDialog,
	OpenFileDialog,
	ReloadLoginEnv,
	RemoveGroupMember,
	RestartSessiond,
	RestartSessiondWithReason,
	SetAgentCLIPath,
	SetDefaultSetting,
	UpdateAlias,
	UpdateGroup,
} from '../../../wailsjs/go/main/App';

export type SessiondStatusResponse = {
	available: boolean;
	error?: string;
	warning?: string;
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

export async function fetchSessiondStatus(): Promise<SessiondStatusResponse> {
	return GetSessiondStatus();
}

export async function restartSessiond(reason?: string): Promise<SessiondStatusResponse> {
	const trimmed = reason?.trim();
	if (trimmed) {
		return RestartSessiondWithReason(trimmed);
	}
	return RestartSessiond();
}

export async function listRegisteredRepos(): Promise<Alias[]> {
	return ListAliases();
}

export async function registerRepo(
	name: string,
	source: string,
	remote: string,
	defaultBranch: string,
): Promise<void> {
	await CreateAlias({ name, source, remote, defaultBranch });
}

export async function updateRegisteredRepo(
	name: string,
	source: string,
	remote: string,
	defaultBranch: string,
): Promise<void> {
	await UpdateAlias({ name, source, remote, defaultBranch });
}

export async function unregisterRepo(name: string): Promise<void> {
	await DeleteAlias(name);
}

/** @deprecated Use listRegisteredRepos instead */
export const listAliases = listRegisteredRepos;

/** @deprecated Use registerRepo instead */
export const createAlias = registerRepo;

/** @deprecated Use updateRegisteredRepo instead */
export const updateAlias = updateRegisteredRepo;

/** @deprecated Use unregisterRepo instead */
export const deleteAlias = unregisterRepo;

export async function listGroups(): Promise<GroupSummary[]> {
	return ListGroups();
}

export async function getGroup(name: string): Promise<Group> {
	return GetGroup(name);
}

export async function createGroup(name: string, description: string): Promise<void> {
	await CreateGroup({ name, description });
}

export async function updateGroup(name: string, description: string): Promise<void> {
	await UpdateGroup({ name, description });
}

export async function deleteGroup(name: string): Promise<void> {
	await DeleteGroup(name);
}

export async function addGroupMember(groupName: string, repoName: string): Promise<void> {
	await AddGroupMember({
		groupName,
		repoName,
	});
}

export async function removeGroupMember(groupName: string, repoName: string): Promise<void> {
	await RemoveGroupMember({
		groupName,
		repoName,
	});
}

export async function applyGroup(workspaceId: string, groupName: string): Promise<void> {
	await ApplyGroup(workspaceId, groupName);
}

export async function fetchSettings(): Promise<SettingsSnapshot> {
	return (await GetSettings()) as unknown as SettingsSnapshot;
}

export async function setDefaultSetting(key: string, value: string): Promise<void> {
	await SetDefaultSetting(key, value);
}
