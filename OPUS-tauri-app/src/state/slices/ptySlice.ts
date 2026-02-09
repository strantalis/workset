import type { StateCreator } from 'zustand';
import { ptyCreate, ptyStart, ptyWrite, ptyResize, ptyAck, ptyStop } from '@/api/pty';

export type PtySessionState = {
  terminalId: string;
  workspaceName: string;
  kind: 'terminal' | 'agent';
  status: 'starting' | 'connected' | 'disconnected' | 'error';
  altScreen: boolean;
  mouse: boolean;
};

export type PtySlice = {
  ptySessions: Record<string, PtySessionState>;
  allocatePtySession: (workspaceName: string, kind: 'terminal' | 'agent') => Promise<string>;
  startPtySession: (workspaceName: string, terminalId: string, kind: 'terminal' | 'agent', cwd: string) => Promise<void>;
  createPtySession: (workspaceName: string, kind: 'terminal' | 'agent', cwd: string) => Promise<string>;
  closePtySession: (workspaceName: string, terminalId: string) => Promise<void>;
  writePty: (workspaceName: string, terminalId: string, data: string) => Promise<void>;
  resizePty: (workspaceName: string, terminalId: string, cols: number, rows: number) => Promise<void>;
  ackPty: (workspaceName: string, terminalId: string, bytes: number) => Promise<void>;
  updatePtyStatus: (terminalId: string, status: PtySessionState['status']) => void;
  updatePtyModes: (terminalId: string, altScreen: boolean, mouse: boolean) => void;
};

export const createPtySlice: StateCreator<PtySlice, [], [], PtySlice> = (set, _get) => ({
  ptySessions: {},

  allocatePtySession: async (workspaceName, kind) => {
    const { terminal_id } = await ptyCreate();
    set((state) => ({
      ptySessions: {
        ...state.ptySessions,
        [terminal_id]: {
          terminalId: terminal_id,
          workspaceName,
          kind,
          status: 'starting',
          altScreen: false,
          mouse: false,
        },
      },
    }));
    return terminal_id;
  },

  startPtySession: async (workspaceName, terminalId, kind, cwd) => {
    await ptyStart(workspaceName, terminalId, kind, cwd);
  },

  createPtySession: async (workspaceName, kind, cwd) => {
    const { terminal_id } = await ptyCreate();
    set((state) => ({
      ptySessions: {
        ...state.ptySessions,
        [terminal_id]: {
          terminalId: terminal_id,
          workspaceName,
          kind,
          status: 'starting',
          altScreen: false,
          mouse: false,
        },
      },
    }));
    await ptyStart(workspaceName, terminal_id, kind, cwd);
    return terminal_id;
  },

  closePtySession: async (workspaceName, terminalId) => {
    await ptyStop(workspaceName, terminalId).catch(() => {});
    set((state) => {
      const { [terminalId]: _, ...rest } = state.ptySessions;
      return { ptySessions: rest };
    });
  },

  writePty: async (workspaceName, terminalId, data) => {
    await ptyWrite(workspaceName, terminalId, data);
  },

  resizePty: async (workspaceName, terminalId, cols, rows) => {
    await ptyResize(workspaceName, terminalId, cols, rows);
  },

  ackPty: async (workspaceName, terminalId, bytes) => {
    await ptyAck(workspaceName, terminalId, bytes);
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

  updatePtyModes: (terminalId, altScreen, mouse) => {
    set((state) => {
      const session = state.ptySessions[terminalId];
      if (!session) return state;
      return {
        ptySessions: {
          ...state.ptySessions,
          [terminalId]: { ...session, altScreen, mouse },
        },
      };
    });
  },
});
