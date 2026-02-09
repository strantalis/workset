import { invoke } from './invoke';
import type { DiffSummary, FilePatch } from '@/types/diff';

export function diffSummary(
  workspaceName: string,
  repo: string,
  repoPath: string,
): Promise<DiffSummary> {
  return invoke<DiffSummary>('diff_summary', { workspaceName, repo, repoPath });
}

export function diffFilePatch(
  repoPath: string,
  path: string,
  prevPath: string | undefined,
  status: string,
): Promise<FilePatch> {
  return invoke<FilePatch>('diff_file_patch', { repoPath, path, prevPath, status });
}

export function diffWatchStart(
  workspaceName: string,
  repo: string,
  repoPath: string,
): Promise<void> {
  return invoke<void>('diff_watch_start', { workspaceName, repo, repoPath });
}

export function diffWatchStop(workspaceName: string, repo: string): Promise<void> {
  return invoke<void>('diff_watch_stop', { workspaceName, repo });
}
