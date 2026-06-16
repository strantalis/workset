import type { StateCreator } from 'zustand';
import type { WorksetProfile, WorksetDefaults } from '@/types/workset';
import * as api from '@/api/worksets';
import * as contextApi from '@/api/context';

export type WorksetSlice = {
  worksets: WorksetProfile[];
  activeWorksetId: string | null;
  activeWorkspaceName: string | null;
  worksetsLoading: boolean;
  loadWorksets: () => Promise<void>;
  createWorkset: (name: string, defaults?: WorksetDefaults) => Promise<WorksetProfile>;
  updateWorkset: (id: string, name?: string, defaults?: WorksetDefaults) => Promise<void>;
  deleteWorkset: (id: string) => Promise<void>;
  setActiveWorkset: (id: string) => Promise<void>;
  setActiveWorkspace: (name: string) => Promise<void>;
  addWorksetRepo: (source: string) => Promise<void>;
  removeWorksetRepo: (source: string) => Promise<void>;
};

export const createWorksetSlice: StateCreator<WorksetSlice> = (set, get) => ({
  worksets: [],
  activeWorksetId: null,
  activeWorkspaceName: null,
  worksetsLoading: false,

  loadWorksets: async () => {
    set({ worksetsLoading: true });
    try {
      const [worksets, context] = await Promise.all([
        api.listWorksets(),
        contextApi.getContext(),
      ]);
      const validWorksetId = worksets.some((w) => w.id === context.active_workset_id)
        ? context.active_workset_id
        : null;
      set({
        worksets,
        activeWorksetId: validWorksetId,
        activeWorkspaceName: validWorksetId ? (context.active_workspace || null) : null,
        worksetsLoading: false,
      });
    } catch {
      set({ worksetsLoading: false });
    }
  },

  createWorkset: async (name, defaults) => {
    const profile = await api.createWorkset(name, defaults);
    set((s) => ({ worksets: [...s.worksets, profile] }));
    return profile;
  },

  updateWorkset: async (id, name, defaults) => {
    const updated = await api.updateWorkset(id, name, defaults);
    set((s) => ({ worksets: s.worksets.map((w) => (w.id === id ? updated : w)) }));
  },

  deleteWorkset: async (id) => {
    await api.deleteWorkset(id);
    set((s) => ({
      worksets: s.worksets.filter((w) => w.id !== id),
      activeWorksetId: s.activeWorksetId === id ? null : s.activeWorksetId,
    }));
  },

  setActiveWorkset: async (id) => {
    await contextApi.setActiveWorkset(id);
    set({ activeWorksetId: id, activeWorkspaceName: null });
  },

  setActiveWorkspace: async (name) => {
    await contextApi.setActiveWorkspace(name);
    set({ activeWorkspaceName: name });
  },

  addWorksetRepo: async (source) => {
    const { activeWorksetId } = get();
    if (!activeWorksetId) return;
    const updated = await api.addWorksetRepo(activeWorksetId, source);
    set((s) => ({ worksets: s.worksets.map((w) => (w.id === activeWorksetId ? updated : w)) }));
  },

  removeWorksetRepo: async (source) => {
    const { activeWorksetId } = get();
    if (!activeWorksetId) return;
    const updated = await api.removeWorksetRepo(activeWorksetId, source);
    set((s) => ({ worksets: s.worksets.map((w) => (w.id === activeWorksetId ? updated : w)) }));
  },
});
