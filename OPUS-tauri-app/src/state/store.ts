import { create } from 'zustand';
import { createWorksetSlice, type WorksetSlice } from './slices/worksetSlice';
import { createWorkspaceSlice, type WorkspaceSlice } from './slices/workspaceSlice';
import { createLayoutSlice, type LayoutSlice } from './slices/layoutSlice';
import { createPtySlice, type PtySlice } from './slices/ptySlice';
import { createDiffSlice, type DiffSlice } from './slices/diffSlice';
import { createUiSlice, type UiSlice } from './slices/uiSlice';

export type AppStore = WorksetSlice & WorkspaceSlice & LayoutSlice & PtySlice & DiffSlice & UiSlice;

export const useAppStore = create<AppStore>()((...args) => ({
  ...createWorksetSlice(...args),
  ...createWorkspaceSlice(...args),
  ...createLayoutSlice(...args),
  ...createPtySlice(...args),
  ...createDiffSlice(...args),
  ...createUiSlice(...args),
}));
