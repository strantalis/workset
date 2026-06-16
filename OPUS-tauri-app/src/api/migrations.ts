import { invoke } from './invoke';
import type { MigrationJobRef } from '@/types/jobs';

export type MigrationStartOptions = {
  worksetId: string;
  repoUrl: string;
  action: 'add' | 'remove';
  workspaceNames: string[];
  deleteWorktrees?: boolean;
  deleteLocal?: boolean;
};

export function migrationStart(opts: MigrationStartOptions): Promise<MigrationJobRef> {
  return invoke<MigrationJobRef>('migration_start', {
    worksetId: opts.worksetId,
    repoUrl: opts.repoUrl,
    action: opts.action,
    workspaceNames: opts.workspaceNames,
    deleteWorktrees: opts.deleteWorktrees,
    deleteLocal: opts.deleteLocal,
  });
}

export function migrationCancel(jobId: string): Promise<void> {
  return invoke<void>('migration_cancel', { jobId });
}
