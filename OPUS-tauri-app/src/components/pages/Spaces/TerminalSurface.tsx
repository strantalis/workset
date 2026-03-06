import { useEffect, useRef } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { ClipboardAddon } from '@xterm/addon-clipboard';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { useAppStore } from '@/state/store';
import { useTerminalTheme } from '@/styles/ThemeProvider';
import {
  terminalSpawn,
  terminalAttach,
  terminalDetach,
  terminalWrite,
  terminalResize,
} from '@/api/pty';
import type { PtyEvent } from '@/api/pty';
import '@xterm/xterm/css/xterm.css';
import './TerminalSurface.css';

type Props = {
  workspaceName: string;
  terminalId: string;
};

export function TerminalSurface({ workspaceName, terminalId }: Props) {
  const termTheme = useTerminalTheme();
  const terminalStyle = useAppStore((s) => s.terminalStyle);
  const containerRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const openedRef = useRef(false);

  // Use a ref for the PTY event handler so the main effect never re-runs
  // due to callback identity changes (e.g. during HMR).
  const ptyEventRef = useRef<(event: PtyEvent) => void>(() => {});
  ptyEventRef.current = (event: PtyEvent) => {
    const term = terminalRef.current;
    if (!term || !openedRef.current) return;

    switch (event.type) {
      case 'Data':
        term.write(event.data);
        break;
      case 'Closed':
        useAppStore.getState().closePtySession(terminalId);
        useAppStore.getState().closeTabByTerminalId(terminalId);
        break;
      case 'Error':
        console.error('[TerminalSurface] PTY error:', event.message);
        useAppStore.getState().updatePtyStatus(terminalId, 'error');
        break;
    }
  };

  // Stable handler that delegates to the ref — identity never changes.
  const stableHandler = useRef((event: PtyEvent) => {
    ptyEventRef.current(event);
  }).current;

  // Main terminal lifecycle — only re-runs when identity changes.
  useEffect(() => {
    if (!containerRef.current) return;
    const container = containerRef.current;
    let disposed = false;

    const style = useAppStore.getState().terminalStyle;
    const term = new Terminal({
      fontSize: style.fontSize,
      fontFamily: style.fontFamily,
      lineHeight: style.lineHeight,
      cursorBlink: style.cursorBlink,
      scrollback: 10000,
      allowProposedApi: true,
      theme: termTheme,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.loadAddon(new WebLinksAddon());
    term.loadAddon(new ClipboardAddon());
    term.loadAddon(new Unicode11Addon());

    terminalRef.current = term;
    fitAddonRef.current = fitAddon;
    openedRef.current = false;

    const tryOpen = () => {
      if (disposed || openedRef.current) return;
      if (container.clientWidth <= 0 || container.clientHeight <= 0) return;

      term.open(container);
      term.unicode.activeVersion = '11';

      try {
        term.loadAddon(new WebglAddon());
      } catch (e) {
        console.warn('WebGL renderer unavailable, using canvas fallback:', e);
      }

      openedRef.current = true;

      const safeFit = () => {
        try { fitAddon.fit(); } catch { /* renderer not ready */ }
      };
      if (document.fonts?.ready) {
        document.fonts.ready.then(() => {
          if (!disposed && openedRef.current) safeFit();
          requestAnimationFrame(() => {
            if (!disposed && openedRef.current) term.focus();
          });
        });
      } else {
        safeFit();
        requestAnimationFrame(() => {
          if (!disposed && openedRef.current) term.focus();
        });
      }

      // Wire up input, resize, and title change
      term.onData((data) => {
        terminalWrite(terminalId, data);
      });
      term.onResize(({ cols, rows }) => {
        terminalResize(terminalId, cols, rows);
      });
      term.onTitleChange((title) => {
        useAppStore.getState().updateTabTitle(terminalId, title);
      });

      // Spawn or reattach the PTY
      const isSpawned = useAppStore.getState().ptySessions[terminalId]?.spawned;

      if (isSpawned) {
        // Reattach — ring buffer replay restores screen content
        terminalAttach(terminalId, stableHandler)
          .then(() => {
            if (!disposed) useAppStore.getState().updatePtyStatus(terminalId, 'connected');
          })
          .catch((err) => {
            console.error('[TerminalSurface] attach failed, trying spawn:', err);
            spawnFallback();
          });
      } else {
        spawnFallback();
      }

      function spawnFallback() {
        const ws = useAppStore.getState().workspaces.find((w) => w.name === workspaceName);
        const cwd = ws?.path ?? '/';
        terminalSpawn(terminalId, cwd, stableHandler)
          .then(() => {
            if (!disposed) useAppStore.getState().markPtySpawned(terminalId);
          })
          .catch((e) => {
            console.error('[TerminalSurface] spawn failed:', e);
            if (!disposed) useAppStore.getState().updatePtyStatus(terminalId, 'error');
          });
      }
    };

    const observer = new ResizeObserver(() => {
      if (!openedRef.current) {
        tryOpen();
      } else if (!disposed) {
        try { fitAddon.fit(); } catch { /* renderer not ready */ }
      }
    });
    observer.observe(container);
    requestAnimationFrame(tryOpen);

    return () => {
      disposed = true;
      observer.disconnect();
      // Detach only — PTY keeps running in the backend
      terminalDetach(terminalId).catch(() => {});
      term.dispose();
      terminalRef.current = null;
      fitAddonRef.current = null;
      openedRef.current = false;
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspaceName, terminalId]);

  // Live-update terminal theme without recreating the PTY session
  useEffect(() => {
    const term = terminalRef.current;
    if (term && openedRef.current) {
      term.options.theme = termTheme;
    }
  }, [termTheme]);

  // Live-update terminal font/style
  useEffect(() => {
    const term = terminalRef.current;
    if (term && openedRef.current) {
      term.options.fontSize = terminalStyle.fontSize;
      term.options.fontFamily = terminalStyle.fontFamily;
      term.options.lineHeight = terminalStyle.lineHeight;
      term.options.cursorBlink = terminalStyle.cursorBlink;
      try { fitAddonRef.current?.fit(); } catch { /* renderer not ready */ }
    }
  }, [terminalStyle]);

  return <div ref={containerRef} className="terminal-surface" />;
}
