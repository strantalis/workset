import { invoke } from './invoke';
import type { GitHubRepo, GitHubAuthStatus, GitHubAccount } from '@/types/github';

export function listGitHubRepos(): Promise<GitHubRepo[]> {
  return invoke<GitHubRepo[]>('github_list_repos');
}

export function githubAuthStatus(): Promise<GitHubAuthStatus> {
  return invoke<GitHubAuthStatus>('github_auth_status');
}

export function listGitHubAccounts(): Promise<GitHubAccount[]> {
  return invoke<GitHubAccount[]>('github_list_accounts');
}

export function switchGitHubAccount(user: string): Promise<void> {
  return invoke<void>('github_switch_account', { user });
}
