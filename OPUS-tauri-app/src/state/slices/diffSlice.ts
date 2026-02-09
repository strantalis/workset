import type { StateCreator } from 'zustand';
import type { DiffSummary, FilePatch } from '@/types/diff';
import { diffSummary, diffFilePatch } from '@/api/diff';

export type RepoDiffState = {
  repo: string;
  repoPath: string;
  summary: DiffSummary | null;
  loading: boolean;
};

export type DiffSlice = {
  repoDiffs: Record<string, RepoDiffState>;
  loadDiffSummary: (workspaceName: string, repo: string, repoPath: string) => Promise<void>;
  updateDiffSummary: (repo: string, summary: DiffSummary) => void;
  fetchFilePatch: (repoPath: string, path: string, prevPath: string | undefined, status: string) => Promise<FilePatch>;
};

export const createDiffSlice: StateCreator<DiffSlice, [], [], DiffSlice> = (set, get) => ({
  repoDiffs: {},

  loadDiffSummary: async (workspaceName, repo, repoPath) => {
    set((state) => ({
      repoDiffs: {
        ...state.repoDiffs,
        [repo]: { repo, repoPath, summary: state.repoDiffs[repo]?.summary ?? null, loading: true },
      },
    }));
    try {
      const summary = await diffSummary(workspaceName, repo, repoPath);
      set((state) => ({
        repoDiffs: {
          ...state.repoDiffs,
          [repo]: { repo, repoPath, summary, loading: false },
        },
      }));
    } catch {
      set((state) => ({
        repoDiffs: {
          ...state.repoDiffs,
          [repo]: { ...state.repoDiffs[repo], loading: false },
        },
      }));
    }
  },

  updateDiffSummary: (repo, summary) => {
    set((state) => {
      const existing = state.repoDiffs[repo];
      return {
        repoDiffs: {
          ...state.repoDiffs,
          [repo]: { repo, repoPath: existing?.repoPath ?? '', summary, loading: false },
        },
      };
    });
  },

  fetchFilePatch: async (repoPath, path, prevPath, status) => {
    return diffFilePatch(repoPath, path, prevPath, status);
  },
});
