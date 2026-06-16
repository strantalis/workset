import { useEffect } from 'react';
import { useAppStore } from '@/state/store';
import { getThemeById, DEFAULT_THEME_ID } from './themes';
import type { ITheme } from '@xterm/xterm';

export function ThemeProvider() {
  const activeThemeId = useAppStore((s) => s.activeThemeId);

  useEffect(() => {
    const theme = getThemeById(activeThemeId) ?? getThemeById(DEFAULT_THEME_ID)!;
    const root = document.documentElement;

    root.dataset.theme = theme.id;
    root.style.setProperty('color-scheme', theme.group);

    for (const [prop, value] of Object.entries(theme.tokens)) {
      root.style.setProperty(prop, value);
    }
  }, [activeThemeId]);

  return null;
}

export function useTerminalTheme(): ITheme {
  const activeThemeId = useAppStore((s) => s.activeThemeId);
  const theme = getThemeById(activeThemeId) ?? getThemeById(DEFAULT_THEME_ID)!;
  return theme.terminal;
}
