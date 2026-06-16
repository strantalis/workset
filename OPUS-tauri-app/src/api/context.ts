import { invoke } from './invoke';
import type { ActiveContext } from '@/types/context';

export function getContext(): Promise<ActiveContext> {
  return invoke<ActiveContext>('context_get');
}

export function setActiveWorkset(worksetId: string): Promise<void> {
  return invoke<void>('context_set_active_workset', { worksetId });
}

export function setActiveWorkspace(workspaceName: string): Promise<void> {
  return invoke<void>('context_set_active_workspace', { workspaceName });
}
