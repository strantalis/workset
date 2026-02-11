import type { StateCreator } from 'zustand';

export type NavPage = 'command-center' | 'spaces' | 'settings';
export type CommandCenterSection = 'repositories' | 'diagnostics';
export type SettingsSection = 'app' | 'appearance' | 'workset' | 'diagnostics';

export type ModalState = {
  type: string;
  props?: Record<string, unknown>;
} | null;

export type TerminalStyle = {
  fontFamily: string;
  fontSize: number;
  lineHeight: number;
  cursorBlink: boolean;
};

export type UiSlice = {
  activePage: NavPage;
  commandCenterSection: CommandCenterSection;
  settingsSection: SettingsSection;
  rightPanelCollapsed: boolean;
  activeModal: ModalState;
  activeThemeId: string;
  terminalStyle: TerminalStyle;
  setActivePage: (page: NavPage) => void;
  setCommandCenterSection: (section: CommandCenterSection) => void;
  setSettingsSection: (section: SettingsSection) => void;
  toggleRightPanel: () => void;
  openModal: (type: string, props?: Record<string, unknown>) => void;
  closeModal: () => void;
  setTheme: (id: string) => void;
  setTerminalStyle: (style: Partial<TerminalStyle>) => void;
};

export const createUiSlice: StateCreator<UiSlice> = (set) => ({
  activePage: 'spaces',
  commandCenterSection: 'repositories',
  settingsSection: 'app',
  rightPanelCollapsed: false,
  activeModal: null,
  activeThemeId: 'dark',
  terminalStyle: {
    fontFamily: "'JetBrains Mono', monospace",
    fontSize: 13,
    lineHeight: 1.2,
    cursorBlink: true,
  },

  setActivePage: (page) => set({ activePage: page }),
  setCommandCenterSection: (section) => set({ commandCenterSection: section }),
  setSettingsSection: (section) => set({ settingsSection: section }),
  toggleRightPanel: () => set((s) => ({ rightPanelCollapsed: !s.rightPanelCollapsed })),
  openModal: (type, props) => set({ activeModal: { type, props } }),
  closeModal: () => set({ activeModal: null }),
  setTheme: (id) => set({ activeThemeId: id }),
  setTerminalStyle: (patch) => set((s) => ({
    terminalStyle: { ...s.terminalStyle, ...patch },
  })),
});
