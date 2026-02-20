import { useEffect } from 'react';
import { getCommands } from '@/commands/registry';
import type { KeyboardShortcut } from '@/commands/registry';

function matchesShortcut(e: KeyboardEvent, shortcut: KeyboardShortcut): boolean {
  const wantMeta = shortcut.modifiers.includes('meta');
  const wantShift = shortcut.modifiers.includes('shift');
  const wantAlt = shortcut.modifiers.includes('alt');
  const wantCtrl = shortcut.modifiers.includes('ctrl');

  if (e.metaKey !== wantMeta) return false;
  if (e.shiftKey !== wantShift) return false;
  if (e.altKey !== wantAlt) return false;
  if (e.ctrlKey !== wantCtrl) return false;

  return e.key.toLowerCase() === shortcut.key;
}

export function useGlobalShortcuts() {
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      // When focused on a text input, only intercept modifier-based
      // shortcuts (Cmd+...) and Escape — never plain keystrokes.
      if (!e.metaKey && e.key !== 'Escape') {
        const target = e.target as HTMLElement;
        if (
          target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.isContentEditable
        ) {
          return;
        }
      }

      const commands = getCommands();
      for (const cmd of commands) {
        if (!cmd.shortcut) continue;
        if (!matchesShortcut(e, cmd.shortcut)) continue;
        if (cmd.when && !cmd.when()) continue;
        e.preventDefault();
        cmd.execute();
        return;
      }
    }

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);
}
