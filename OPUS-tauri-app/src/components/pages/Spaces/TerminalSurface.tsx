import { useEffect, useRef, useCallback } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { ClipboardAddon } from '@xterm/addon-clipboard';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { useAppStore } from '@/state/store';
import { useTerminalTheme } from '@/styles/ThemeProvider';
import { ptyBootstrap } from '@/api/pty';
import { onEvent } from '@/api/events';
import '@xterm/xterm/css/xterm.css';
import './TerminalSurface.css';

const ACK_BATCH_BYTES = 32 * 1024;
const ACK_FLUSH_DELAY_MS = 25;

type Props = {
  workspaceName: string;
  terminalId: string;
};

export function TerminalSurface({ workspaceName, terminalId }: Props) {
  const termTheme = useTerminalTheme();
  const containerRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const openedRef = useRef(false);
  const startedRef = useRef(false);
  const bootstrappedRef = useRef(false);
  const pendingAckRef = useRef(0);
  const ackTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const outputQueueRef = useRef<string[]>([]);
  const flushScheduledRef = useRef(false);

  const writePty = useAppStore((s) => s.writePty);
  const resizePty = useAppStore((s) => s.resizePty);
  const ackPty = useAppStore((s) => s.ackPty);
  const startPtySession = useAppStore((s) => s.startPtySession);
  const updatePtyStatus = useAppStore((s) => s.updatePtyStatus);
  const updatePtyModes = useAppStore((s) => s.updatePtyModes);
  const updateTabTitle = useAppStore((s) => s.updateTabTitle);

  const flushAck = useCallback(() => {
    if (pendingAckRef.current > 0) {
      const bytes = pendingAckRef.current;
      pendingAckRef.current = 0;
      ackPty(workspaceName, terminalId, bytes);
    }
  }, [workspaceName, terminalId, ackPty]);

  // Queue output and flush via requestAnimationFrame (like the Wails app)
  const enqueueOutput = useCallback((data: string, bytes: number) => {
    outputQueueRef.current.push(data);
    pendingAckRef.current += bytes;
    if (pendingAckRef.current >= ACK_BATCH_BYTES) {
      flushAck();
    }
    if (!flushScheduledRef.current) {
      flushScheduledRef.current = true;
      requestAnimationFrame(() => {
        flushScheduledRef.current = false;
        const term = terminalRef.current;
        if (!term || !openedRef.current) return;
        const chunks = outputQueueRef.current.splice(0);
        if (chunks.length > 0) {
          term.write(chunks.join(''));
        }
      });
    }
  }, [flushAck]);

  // Main initialization effect — sets up xterm, event listeners, starts PTY
  useEffect(() => {
    if (!containerRef.current) return;
    const container = containerRef.current;
    const unlisteners: (() => void)[] = [];
    let disposed = false;

    const term = new Terminal({
      fontSize: 13,
      fontFamily: "'JetBrains Mono', monospace",
      cursorBlink: true,
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
    startedRef.current = false;
    outputQueueRef.current = [];

    // Step 1: Register event listeners FIRST (before starting PTY)
    const tid = terminalId;

    onEvent<{ terminal_id: string; data: string; bytes: number }>('pty:data', (payload) => {
      if (disposed || payload.terminal_id !== tid) return;
      enqueueOutput(payload.data, payload.bytes);
    }).then((fn) => unlisteners.push(fn));

    onEvent<{ terminal_id: string; snapshot?: string }>('pty:bootstrap', (payload) => {
      if (disposed || payload.terminal_id !== tid) return;
      if (payload.snapshot && !bootstrappedRef.current) {
        bootstrappedRef.current = true;
        enqueueOutput(payload.snapshot, payload.snapshot.length);
      }
      updatePtyStatus(tid, 'connected');
    }).then((fn) => unlisteners.push(fn));

    onEvent<{ terminal_id: string; alt_screen: boolean; mouse: boolean }>('pty:modes', (payload) => {
      if (disposed || payload.terminal_id !== tid) return;
      updatePtyModes(tid, payload.alt_screen, payload.mouse);
    }).then((fn) => unlisteners.push(fn));

    onEvent<{ terminal_id: string; status: string }>('pty:lifecycle', (payload) => {
      if (disposed || payload.terminal_id !== tid) return;
      if (payload.status === 'started') updatePtyStatus(tid, 'connected');
      else if (payload.status === 'closed') updatePtyStatus(tid, 'disconnected');
      else if (payload.status === 'error') updatePtyStatus(tid, 'error');
    }).then((fn) => unlisteners.push(fn));

    // Step 2: Open xterm when container has dimensions
    const tryOpen = () => {
      if (disposed || openedRef.current) return;
      if (container.clientWidth <= 0 || container.clientHeight <= 0) {
        console.debug('[TerminalSurface] container has no dimensions yet:', container.clientWidth, container.clientHeight);
        return;
      }

      console.debug('[TerminalSurface] opening xterm in', container.clientWidth, 'x', container.clientHeight);
      term.open(container);
      term.unicode.activeVersion = '11';

      // Load WebGL renderer after open (like the Wails app)
      try {
        term.loadAddon(new WebglAddon());
      } catch (e) {
        console.warn('WebGL renderer unavailable, using canvas fallback:', e);
      }

      openedRef.current = true;

      // Fit after fonts are ready, then auto-focus
      const safeFit = () => {
        try { fitAddon.fit(); } catch (_) { /* renderer not ready */ }
      };
      const focusAfterSettle = () => {
        requestAnimationFrame(() => {
          if (!disposed && openedRef.current) term.focus();
        });
      };
      if (document.fonts?.ready) {
        document.fonts.ready.then(() => {
          if (!disposed && openedRef.current) safeFit();
          focusAfterSettle();
        });
      } else {
        safeFit();
        focusAfterSettle();
      }

      term.onData((data) => {
        writePty(workspaceName, tid, data);
      });
      term.onResize(({ cols, rows }) => {
        resizePty(workspaceName, tid, cols, rows);
      });
      term.onTitleChange((title) => {
        updateTabTitle(tid, title);
      });

      // Step 3: Start PTY session AFTER xterm is open and listeners are registered
      if (!startedRef.current) {
        startedRef.current = true;
        const ws = useAppStore.getState().workspaces.find((w) => w.name === workspaceName);
        const cwd = ws?.path ?? '/';
        console.debug('[TerminalSurface] starting PTY session', tid, 'cwd:', cwd);

        startPtySession(workspaceName, tid, 'terminal', cwd)
          .then(() => {
            console.debug('[TerminalSurface] PTY session started, fetching bootstrap in 200ms');
            // Give the streaming thread a moment, then fetch bootstrap as fallback
            setTimeout(() => {
              if (disposed || bootstrappedRef.current) return;
              ptyBootstrap(workspaceName, tid)
                .then((payload) => {
                  if (disposed || bootstrappedRef.current) return;
                  bootstrappedRef.current = true;
                  if (payload.snapshot) {
                    enqueueOutput(payload.snapshot, payload.snapshot.length);
                  }
                  updatePtyStatus(tid, 'connected');
                })
                .catch(() => {});
            }, 200);
          })
          .catch((err) => {
            console.error('PTY start failed:', err);
            updatePtyStatus(tid, 'error');
          });
      }
    };

    const observer = new ResizeObserver(() => {
      if (!openedRef.current) {
        tryOpen();
      } else if (!disposed) {
        try { fitAddon.fit(); } catch (_) { /* renderer not ready */ }
      }
    });
    observer.observe(container);
    requestAnimationFrame(tryOpen);

    // ACK flush timer
    ackTimerRef.current = setInterval(flushAck, ACK_FLUSH_DELAY_MS);

    return () => {
      disposed = true;
      observer.disconnect();
      if (ackTimerRef.current) clearInterval(ackTimerRef.current);
      for (const fn of unlisteners) fn();
      term.dispose();
      terminalRef.current = null;
      fitAddonRef.current = null;
      openedRef.current = false;
      startedRef.current = false;
      bootstrappedRef.current = false;
    };
  }, [workspaceName, terminalId, writePty, resizePty, startPtySession,
      updatePtyStatus, updatePtyModes, updateTabTitle, enqueueOutput, flushAck]);

  // Live-update terminal theme without recreating the PTY session
  useEffect(() => {
    const term = terminalRef.current;
    if (term && openedRef.current) {
      term.options.theme = termTheme;
    }
  }, [termTheme]);

  return <div ref={containerRef} className="terminal-surface" />;
}
