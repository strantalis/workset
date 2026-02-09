import type { StateCreator } from 'zustand';

export type NavPage = 'command-center' | 'spaces' | 'settings';
export type CommandCenterSection = 'overview' | 'repositories' | 'diagnostics';
export type SettingsSection = 'app' | 'workset' | 'diagnostics';

export type ModalState = {
  type: string;
  props?: Record<string, unknown>;
} | null;

export type UiSlice = {
  activePage: NavPage;
  commandCenterSection: CommandCenterSection;
  settingsSection: SettingsSection;
  rightPanelCollapsed: boolean;
  activeModal: ModalState;
  setActivePage: (page: NavPage) => void;
  setCommandCenterSection: (section: CommandCenterSection) => void;
  setSettingsSection: (section: SettingsSection) => void;
  toggleRightPanel: () => void;
  openModal: (type: string, props?: Record<string, unknown>) => void;
  closeModal: () => void;
};

export const createUiSlice: StateCreator<UiSlice> = (set) => ({
  activePage: 'spaces',
  commandCenterSection: 'overview',
  settingsSection: 'app',
  rightPanelCollapsed: false,
  activeModal: null,

  setActivePage: (page) => set({ activePage: page }),
  setCommandCenterSection: (section) => set({ commandCenterSection: section }),
  setSettingsSection: (section) => set({ settingsSection: section }),
  toggleRightPanel: () => set((s) => ({ rightPanelCollapsed: !s.rightPanelCollapsed })),
  openModal: (type, props) => set({ activeModal: { type, props } }),
  closeModal: () => set({ activeModal: null }),
});
