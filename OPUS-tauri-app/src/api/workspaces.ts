import { invoke } from './invoke';
import type { WorkspaceSummary, WorkspaceCreateJobRef, WorkspaceCreateProgress } from '@/types/workspace';

export function listWorkspaces(worksetId: string): Promise<WorkspaceSummary[]> {
  return invoke<WorkspaceSummary[]>('workspaces_list', { worksetId });
}

export function createWorkspace(worksetId: string, name: string, path?: string): Promise<WorkspaceCreateJobRef> {
  return invoke<WorkspaceCreateJobRef>('workspaces_create', { worksetId, name, path });
}

export function getCreateStatus(jobId: string): Promise<WorkspaceCreateProgress> {
  return invoke<WorkspaceCreateProgress>('workspaces_create_status', { jobId });
}

export function deleteWorkspace(worksetId: string, workspaceName: string, del?: boolean): Promise<void> {
  return invoke<void>('workspaces_delete', { worksetId, workspaceName, delete: del });
}
