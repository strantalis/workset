import { invoke } from './invoke';
import type { WorksetProfile, WorksetDefaults } from '@/types/workset';

export function listWorksets(): Promise<WorksetProfile[]> {
  return invoke<WorksetProfile[]>('worksets_list');
}

export function createWorkset(name: string, defaults?: WorksetDefaults): Promise<WorksetProfile> {
  return invoke<WorksetProfile>('worksets_create', { name, defaults });
}

export function updateWorkset(id: string, name?: string, defaults?: WorksetDefaults): Promise<WorksetProfile> {
  return invoke<WorksetProfile>('worksets_update', { id, name, defaults });
}

export function deleteWorkset(id: string): Promise<void> {
  return invoke<void>('worksets_delete', { id });
}

export function addWorksetRepo(worksetId: string, source: string): Promise<WorksetProfile> {
  return invoke<WorksetProfile>('worksets_repos_add', { worksetId, source });
}

export function removeWorksetRepo(worksetId: string, source: string): Promise<WorksetProfile> {
  return invoke<WorksetProfile>('worksets_repos_remove', { worksetId, source });
}
