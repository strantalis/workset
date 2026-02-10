import type { GitHubAuthInfo, GitHubAuthStatus } from '../../types';
import {
	DisconnectGitHub,
	GetGitHubAuthInfo,
	GetGitHubAuthStatus,
	SetGitHubAuthMode,
	SetGitHubCLIPath,
	SetGitHubToken,
} from '../../../../bindings/workset/app';

export async function fetchGitHubAuthStatus(): Promise<GitHubAuthStatus> {
	return (await GetGitHubAuthStatus()) as GitHubAuthStatus;
}

export async function fetchGitHubAuthInfo(): Promise<GitHubAuthInfo> {
	return (await GetGitHubAuthInfo()) as GitHubAuthInfo;
}

export async function setGitHubToken(token: string, source = 'pat'): Promise<GitHubAuthStatus> {
	return (await SetGitHubToken({ token, source })) as GitHubAuthStatus;
}

export async function setGitHubAuthMode(mode: string): Promise<GitHubAuthInfo> {
	return (await SetGitHubAuthMode({ mode })) as GitHubAuthInfo;
}

export async function disconnectGitHub(): Promise<void> {
	await DisconnectGitHub();
}

export async function setGitHubCLIPath(path: string): Promise<GitHubAuthInfo> {
	return (await SetGitHubCLIPath({ path })) as GitHubAuthInfo;
}
