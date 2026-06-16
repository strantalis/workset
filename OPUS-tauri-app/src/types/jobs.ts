import type { ErrorEnvelope } from './errors';

export type MigrationJobRef = { job_id: string };

export type MigrationProgress = {
  job_id: string;
  state: 'queued' | 'running' | 'done' | 'failed' | 'canceled';
  workspaces: WorkspaceMigrationStatus[];
};

export type WorkspaceMigrationStatus = {
  workspace_name: string;
  state: 'pending' | 'running' | 'success' | 'failed';
  error?: ErrorEnvelope;
};
