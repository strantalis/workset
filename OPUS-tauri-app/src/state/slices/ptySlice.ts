import type { StateCreator } from 'zustand';
import { terminalWrite, terminalResize, terminalKill } from '@/api/pty';

export type PtySessionState = {
  terminalId: string;
  workspaceName: string;
  kind: 'terminal' | 'agent';
  status: 'starting' | 'connected' | 'disconnected' | 'error';
  spawned: boolean;
};

export type PtySlice = {
  ptySessions: Record<string, PtySessionState>;
  allocatePtySession: (workspaceName: string, kind: 'terminal' | 'agent') => string;
  markPtySpawned: (terminalId: string) => void;
  closePtySession: (terminalId: string) => Promise<void>;
  writePty: (terminalId: string, data: string) => Promise<void>;
  resizePty: (terminalId: string, cols: number, rows: number) => Promise<void>;
  updatePtyStatus: (terminalId: string, status: PtySessionState['status']) => void;
};

export const createPtySlice: StateCreator<PtySlice, [], [], PtySlice> = (set) => ({
  ptySessions: {},

  allocatePtySession: (workspaceName, kind) => {
    const terminalId = crypto.randomUUID();
    set((state) => ({
      ptySessions: {
        ...state.ptySessions,
        [terminalId]: {
          terminalId,
          workspaceName,
          kind,
          status: 'starting',
          spawned: false,
        },
      },
    }));
    return terminalId;
  },

  markPtySpawned: (terminalId) => {
    set((state) => {
      const session = state.ptySessions[terminalId];
      if (!session) return state;
      return {
        ptySessions: {
          ...state.ptySessions,
          [terminalId]: { ...session, spawned: true, status: 'connected' },
        },
      };
    });
  },

  closePtySession: async (terminalId) => {
    await terminalKill(terminalId).catch(() => {});
    set((state) => {
      const { [terminalId]: _, ...rest } = state.ptySessions;
      return { ptySessions: rest };
    });
  },

  writePty: async (terminalId, data) => {
    await terminalWrite(terminalId, data);
  },

  resizePty: async (terminalId, cols, rows) => {
    await terminalResize(terminalId, cols, rows);
  },

  updatePtyStatus: (terminalId, status) => {
    set((state) => {
      const session = state.ptySessions[terminalId];
      if (!session) return state;
      return {
        ptySessions: {
          ...state.ptySessions,
          [terminalId]: { ...session, status },
        },
      };
    });
  },
});
