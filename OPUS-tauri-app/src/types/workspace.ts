import type { ErrorEnvelope } from './errors';

export type WorkspaceSummary = {
  name: string;
  path: string;
  created_at?: string;
  last_used?: string;
  archived: boolean;
  pinned: boolean;
  pin_order: number;
  expanded: boolean;
};

export type WorkspaceCreateJobRef = { job_id: string };

export type WorkspaceCreateProgress = {
  job_id: string;
  state: 'queued' | 'running' | 'succeeded' | 'partial' | 'failed';
  repos: RepoProvisionStatus[];
  workspace_name?: string;
  workspace_path?: string;
};

export type RepoProvisionStatus = {
  name: string;
  state: 'pending' | 'running' | 'succeeded' | 'failed';
  step?: 'preflight' | 'fetch/clone' | 'worktree/checkout' | 'verify';
  error?: ErrorEnvelope;
};
