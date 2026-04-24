import type { StateCreator } from 'zustand';
import type { WorkspaceSummary } from '@/types/workspace';
import * as api from '@/api/workspaces';

export type WorkspaceSlice = {
  workspaces: WorkspaceSummary[];
  workspacesLoading: boolean;
  loadWorkspaces: (worksetId: string) => Promise<void>;
  createWorkspace: (worksetId: string, name: string) => Promise<void>;
  deleteWorkspace: (worksetId: string, name: string) => Promise<void>;
};

export const createWorkspaceSlice: StateCreator<WorkspaceSlice> = (set) => ({
  workspaces: [],
  workspacesLoading: false,

  loadWorkspaces: async (worksetId) => {
    set({ workspacesLoading: true });
    try {
      const workspaces = await api.listWorkspaces(worksetId);
      set({ workspaces, workspacesLoading: false });
    } catch {
      set({ workspacesLoading: false });
    }
  },

  createWorkspace: async (worksetId, name) => {
    await api.createWorkspace(worksetId, name);
    try {
      const workspaces = await api.listWorkspaces(worksetId);
      set({ workspaces });
    } catch {}
  },

  deleteWorkspace: async (worksetId, name) => {
    await api.deleteWorkspace(worksetId, name, true);
    try {
      const workspaces = await api.listWorkspaces(worksetId);
      set({ workspaces });
    } catch {}
  },
});
