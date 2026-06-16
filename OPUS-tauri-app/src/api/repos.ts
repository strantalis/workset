import { invoke } from './invoke';
import type { RepoInstance } from '@/types/repo';

export function listWorkspaceRepos(workspaceName: string): Promise<RepoInstance[]> {
  return invoke<RepoInstance[]>('workspace_repos_list', { workspaceName });
}
